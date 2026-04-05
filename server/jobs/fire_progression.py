"""Fire stage progression engine — background scheduler.

Runs as an asyncio background task that periodically:
1. Scans all active fire grid keys in Redis
2. Removes expired fire events (TTL-based via sorted set scores)
3. Recalculates fire stage for each grid
4. Broadcasts stage changes to subscribed clients via Socket.IO
5. Broadcasts firefighter:spawn event at stage ≥ 4 (visual effect only)

The scheduler uses Redis Sorted Sets where:
- Key:    fire:{gridId}
- Score:  expiration timestamp (now + FIRE_TTL_SEC at creation)
- Member: unique event ID

Active fires = members with score > current time.
"""

from __future__ import annotations

import asyncio
import logging
import time
from datetime import datetime
from typing import TYPE_CHECKING, Any, Protocol

from config import (
    KST,
    STATS_DAILY_FIRES_PREFIX,
    STATS_DAILY_FIRES_TTL_SEC,
    STATS_DAILY_RANKING_PREFIX,
    STATS_DAILY_RANKING_TTL_SEC,
    STATS_TOTAL_FIRES_KEY,
)
from grid import grid_id_to_center
from models.fire import (
    FireStage,
    build_grid_state,
    get_firefighter_remove_count,
    get_stage,
    should_dispatch_firefighter,
)
from models.firefighter import (
    FirefighterNPC,
    create_firefighter_npc,
    should_spawn_firefighter,
)

if TYPE_CHECKING:
    from redis.asyncio import Redis

    from services.admin_region import AdminRegionResolver

logger = logging.getLogger(__name__)

# Redis key prefix for fire grids
FIRE_KEY_PREFIX = "fire:"

# Key tracking all active grid IDs (a Redis Set)
ACTIVE_GRIDS_KEY = "active_grids"


class Broadcaster(Protocol):
    """Protocol for broadcasting events to Socket.IO rooms."""

    async def broadcast_to_room(self, event: str, data: Any, room: str) -> None: ...
    async def broadcast(self, event: str, data: Any) -> None: ...


class FireProgressionEngine:
    """Time-based auto-escalation scheduler for fire stage progression.

    Manages the lifecycle of fires across all grid cells:
    - Cleans expired fire events from Redis sorted sets
    - Tracks previous stage per grid to detect transitions
    - Broadcasts fire:update on stage change
    - Broadcasts firefighter:spawn event at stage ≥ 4 (visual effect only)

    Args:
        redis: Async Redis client instance.
        broadcaster: Object implementing broadcast_to_room and broadcast.
        scan_interval: Seconds between each progression scan (default: 2s).
        cleanup_interval: Seconds between deep cleanup runs (default: 60s).
    """

    def __init__(
        self,
        redis: Redis,
        broadcaster: Broadcaster,
        scan_interval: float = 2.0,
        cleanup_interval: float = 60.0,
    ) -> None:
        self._redis = redis
        self._broadcaster = broadcaster
        self._scan_interval = scan_interval
        self._cleanup_interval = cleanup_interval

        # Track previous stage per grid to detect transitions
        self._grid_stages: dict[str, FireStage] = {}

        # Admin region resolver for daily ranking (injected at startup; optional)
        self._resolver: "AdminRegionResolver | None" = None

        # Background task references
        self._progression_task: asyncio.Task | None = None
        self._cleanup_task: asyncio.Task | None = None
        self._running = False

    @property
    def is_running(self) -> bool:
        return self._running

    @property
    def grid_stages(self) -> dict[str, FireStage]:
        """Current tracked stage per grid (read-only copy)."""
        return dict(self._grid_stages)

    def set_region_resolver(self, resolver: "AdminRegionResolver | None") -> None:
        """Inject the admin region resolver used for daily ranking.

        Accepts None to disable ranking writes without failing the pipeline.
        """
        self._resolver = resolver

    def start(self) -> None:
        """Start all background loops as asyncio tasks."""
        if self._running:
            logger.warning("FireProgressionEngine already running")
            return

        self._running = True
        self._progression_task = asyncio.create_task(
            self._progression_loop(), name="fire-progression"
        )
        self._cleanup_task = asyncio.create_task(
            self._cleanup_loop(), name="fire-cleanup"
        )
        logger.info(
            "FireProgressionEngine started (scan=%.1fs, cleanup=%.1fs)",
            self._scan_interval,
            self._cleanup_interval,
        )

    async def stop(self) -> None:
        """Gracefully stop all background loops."""
        self._running = False
        tasks = [
            t
            for t in (self._progression_task, self._cleanup_task)
            if t is not None
        ]
        for task in tasks:
            task.cancel()
        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)

        self._progression_task = None
        self._cleanup_task = None
        logger.info("FireProgressionEngine stopped")

    # ------------------------------------------------------------------
    # Core: Stage progression scan
    # ------------------------------------------------------------------

    async def _progression_loop(self) -> None:
        """Main loop: scan grids, detect stage changes, broadcast updates."""
        while self._running:
            try:
                await self._scan_and_update_stages()
            except asyncio.CancelledError:
                break
            except Exception:
                logger.exception("Error in progression loop")
            await asyncio.sleep(self._scan_interval)

    async def _scan_and_update_stages(self) -> None:
        """Scan all active grids and broadcast any stage transitions."""
        now = time.time()
        grid_ids = await self._get_active_grid_ids()

        for grid_id in grid_ids:
            active_count = await self._get_active_count(grid_id, now)
            new_stage = get_stage(active_count)
            prev_stage = self._grid_stages.get(grid_id, FireStage.NONE)

            if new_stage != prev_stage:
                self._grid_stages[grid_id] = new_stage
                await self._broadcast_stage_change(
                    grid_id, active_count, new_stage, prev_stage=prev_stage,
                )

                # Trigger firefighter NPC spawn on escalation to stage 4+
                if new_stage >= FireStage.DAEHWAJAE:
                    await self.check_and_spawn_firefighter(grid_id, active_count)

            # Clean up tracking for empty grids (even if stage didn't change)
            if new_stage == FireStage.NONE:
                self._grid_stages.pop(grid_id, None)
                await self._remove_active_grid(grid_id)

    async def _get_active_count(self, grid_id: str, now: float) -> int:
        """Count active (non-expired) fires in a grid cell.

        Args:
            grid_id: The grid cell identifier.
            now: Current timestamp for expiry comparison.

        Returns:
            Number of active fire events.
        """
        key = f"{FIRE_KEY_PREFIX}{grid_id}"
        # ZCOUNT with score range [now, +inf] = active fires
        count = await self._redis.zcount(key, now, "+inf")
        return count

    async def _get_active_grid_ids(self) -> list[str]:
        """Get all grid IDs that have (or recently had) active fires.

        Uses the active_grids Redis Set to track which grids have fires,
        avoiding expensive SCAN operations.
        """
        members = await self._redis.smembers(ACTIVE_GRIDS_KEY)
        return [m.decode() if isinstance(m, bytes) else m for m in members]

    async def _remove_active_grid(self, grid_id: str) -> None:
        """Remove a grid from the active tracking set."""
        await self._redis.srem(ACTIVE_GRIDS_KEY, grid_id)

    async def _broadcast_stage_change(
        self, grid_id: str, active_count: int, new_stage: FireStage,
        prev_stage: FireStage | None = None,
    ) -> None:
        """Broadcast fire:update and fire:stage_transition events on stage change.

        Sends updates to both the grid room (for viewport subscribers) and
        globally (for map overview clients). The fire:stage_transition event
        carries prev/new stage info so clients can animate transitions.

        Args:
            grid_id: Grid cell that changed.
            active_count: Current active fire count.
            new_stage: The new fire stage.
            prev_stage: The previous fire stage (for transition payload).
        """
        import time as _time

        state = build_grid_state(grid_id=grid_id, active_count=active_count)
        payload = state.model_dump()
        payload["timestamp"] = _time.time()
        # camelCase aliases for mock server compat
        payload["gridId"] = grid_id
        payload["activeCount"] = active_count

        # 1) Room-scoped update — reaches viewport subscribers
        await self._broadcaster.broadcast_to_room("fire:update", payload, room=grid_id)

        # 2) Global update — reaches map overview clients
        await self._broadcaster.broadcast("fire:update", payload)

        # 3) Dedicated stage transition event with prev/new for animations
        if prev_stage is None:
            prev_stage = FireStage.NONE
        transition_payload = {
            "grid_id": grid_id,
            "active_count": active_count,
            "prev_stage": int(prev_stage),
            "new_stage": int(new_stage),
            "stage_info": payload.get("stage_info"),
            "timestamp": payload["timestamp"],
        }
        await self._broadcaster.broadcast_to_room(
            "fire:stage_transition", transition_payload, room=grid_id,
        )
        await self._broadcaster.broadcast("fire:stage_transition", transition_payload)

        logger.info(
            "Stage change: grid=%s stage=%s→%s count=%d",
            grid_id,
            prev_stage.name,
            new_stage.name,
            active_count,
        )

    async def _broadcast_fire_count_update(
        self, grid_id: str, active_count: int, current_stage: FireStage,
    ) -> None:
        """Broadcast fire:update with current count (even within same stage).

        Used after firefighter suppression so clients see the count decreasing
        even when the stage hasn't changed yet.
        """
        import time as _time

        state = build_grid_state(grid_id=grid_id, active_count=active_count)
        payload = state.model_dump()
        payload["timestamp"] = _time.time()
        # camelCase aliases for mock server compat
        payload["gridId"] = grid_id
        payload["activeCount"] = active_count

        await self._broadcaster.broadcast_to_room("fire:update", payload, room=grid_id)
        await self._broadcaster.broadcast("fire:update", payload)

    # ------------------------------------------------------------------
    # Firefighter spawn (visual effect only)
    # ------------------------------------------------------------------

    async def check_and_spawn_firefighter(self, grid_id: str, active_count: int) -> FirefighterNPC | None:
        """Broadcast firefighter:spawn event when a grid reaches stage 4+.

        Visual-effect only — no Redis state is persisted and no suppression
        is applied. Callers already gate by stage transition, so dedup is
        handled at the call site.

        Args:
            grid_id: Grid cell to check.
            active_count: Current active fire count.

        Returns:
            The spawned FirefighterNPC (payload holder), or None if no spawn needed.
        """
        if not should_spawn_firefighter(active_count):
            return None

        npc = create_firefighter_npc(grid_id, active_count)
        await self._broadcast_firefighter_spawn(npc)

        logger.info(
            "Firefighter spawn broadcast: npc_id=%s grid=%s stage=%d",
            npc.npc_id,
            grid_id,
            npc.target_stage,
        )

        return npc

    async def _broadcast_firefighter_spawn(self, npc: FirefighterNPC) -> None:
        """Broadcast firefighter:spawn event when a new NPC is dispatched."""
        payload = npc.to_broadcast_payload()
        # Broadcast to the specific grid room and also globally
        await self._broadcaster.broadcast_to_room(
            "firefighter:spawn", payload, room=npc.grid_id
        )
        await self._broadcaster.broadcast("firefighter:spawn", payload)

    # ------------------------------------------------------------------
    # Cleanup: remove expired entries
    # ------------------------------------------------------------------

    async def _cleanup_loop(self) -> None:
        """Periodically remove expired fire events from all grid keys."""
        while self._running:
            try:
                await self._run_cleanup()
            except asyncio.CancelledError:
                break
            except Exception:
                logger.exception("Error in cleanup loop")
            await asyncio.sleep(self._cleanup_interval)

    async def _run_cleanup(self) -> None:
        """Remove expired events and clean up empty grid keys."""
        now = time.time()
        grid_ids = await self._get_active_grid_ids()
        cleaned = 0

        for grid_id in grid_ids:
            key = f"{FIRE_KEY_PREFIX}{grid_id}"

            # Remove members with score < now (expired)
            removed = await self._redis.zremrangebyscore(key, "-inf", now)
            if removed > 0:
                cleaned += removed

            # Check if key is now empty
            remaining = await self._redis.zcard(key)
            if remaining == 0:
                await self._redis.delete(key)
                await self._remove_active_grid(grid_id)
                self._grid_stages.pop(grid_id, None)

        if cleaned > 0:
            logger.info("Cleanup: removed %d expired fire events", cleaned)

    # ------------------------------------------------------------------
    # Public helpers (for use by fire API when adding new fires)
    # ------------------------------------------------------------------

    async def register_fire(self, grid_id: str, event_id: str, expire_at: float) -> int:
        """Register a new fire event and return updated active count.

        Called by the fire API when a user ignites a fire.

        Args:
            grid_id: Grid cell where the fire was ignited.
            event_id: Unique fire event identifier.
            expire_at: Expiration timestamp (score in sorted set).

        Returns:
            Current active fire count after registration.
        """
        key = f"{FIRE_KEY_PREFIX}{grid_id}"
        now = time.time()

        # Add fire event to sorted set
        await self._redis.zadd(key, {event_id: expire_at})

        # Track this grid as active
        await self._redis.sadd(ACTIVE_GRIDS_KEY, grid_id)

        # Increment cumulative fire counter
        await self._redis.incr(STATS_TOTAL_FIRES_KEY)

        # Increment today's fire counter (KST-based key, 48h TTL for safe rollover)
        today_str = datetime.now(KST).strftime("%Y-%m-%d")
        today_key = f"{STATS_DAILY_FIRES_PREFIX}{today_str}"
        await self._redis.incr(today_key)
        await self._redis.expire(today_key, STATS_DAILY_FIRES_TTL_SEC)

        # Increment today's region ranking (skip if resolver absent or point
        # falls outside any admin polygon — cumulative/daily counters stay intact)
        if self._resolver is not None:
            lat, lng = grid_id_to_center(grid_id)
            region = self._resolver.resolve(lat, lng)
            if region:
                rank_key = f"{STATS_DAILY_RANKING_PREFIX}{today_str}"
                await self._redis.zincrby(rank_key, 1, region)
                await self._redis.expire(rank_key, STATS_DAILY_RANKING_TTL_SEC)

        # Get current active count
        active_count = await self._redis.zcount(key, now, "+inf")

        # Update stage tracking
        new_stage = get_stage(active_count)
        prev_stage = self._grid_stages.get(grid_id, FireStage.NONE)
        self._grid_stages[grid_id] = new_stage

        # Broadcast immediately on stage change (don't wait for scan)
        if new_stage != prev_stage:
            await self._broadcast_stage_change(
                grid_id, active_count, new_stage, prev_stage=prev_stage,
            )

            # Spawn firefighter NPC on immediate escalation to stage 4+
            if new_stage >= FireStage.DAEHWAJAE:
                await self.check_and_spawn_firefighter(grid_id, active_count)

        return active_count
