"""Fire stage progression engine — background scheduler.

Runs as an asyncio background task that periodically:
1. Scans all active fire grid keys in Redis
2. Removes expired fire events (TTL-based via sorted set scores)
3. Recalculates fire stage for each grid
4. Broadcasts stage changes to subscribed clients via Socket.IO
5. Dispatches firefighter NPCs to suppress fires at stage ≥ 4

The scheduler uses Redis Sorted Sets where:
- Key:    fire:{gridId}
- Score:  expiration timestamp (now + 30min at creation)
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
    STATS_TOTAL_FIRES_KEY,
)
from models.fire import (
    FireStage,
    build_grid_state,
    get_firefighter_remove_count,
    get_stage,
    should_dispatch_firefighter,
)
from models.firefighter import (
    ACTIVE_FIREFIGHTERS_KEY,
    FIREFIGHTER_GRID_INDEX_KEY,
    FIREFIGHTER_KEY_PREFIX,
    FIREFIGHTER_TTL_SEC,
    FirefighterNPC,
    FirefighterStatus,
    create_firefighter_npc,
    should_spawn_firefighter,
)

if TYPE_CHECKING:
    from redis.asyncio import Redis

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
    - Triggers firefighter:alert and suppression at stage ≥ 4

    Args:
        redis: Async Redis client instance.
        broadcaster: Object implementing broadcast_to_room and broadcast.
        scan_interval: Seconds between each progression scan (default: 2s).
        firefighter_interval: Seconds between firefighter sweeps (default: 30s).
        cleanup_interval: Seconds between deep cleanup runs (default: 60s).
    """

    def __init__(
        self,
        redis: Redis,
        broadcaster: Broadcaster,
        scan_interval: float = 2.0,
        firefighter_interval: float = 30.0,
        cleanup_interval: float = 60.0,
    ) -> None:
        self._redis = redis
        self._broadcaster = broadcaster
        self._scan_interval = scan_interval
        self._firefighter_interval = firefighter_interval
        self._cleanup_interval = cleanup_interval

        # Track previous stage per grid to detect transitions
        self._grid_stages: dict[str, FireStage] = {}

        # Background task references
        self._progression_task: asyncio.Task | None = None
        self._firefighter_task: asyncio.Task | None = None
        self._cleanup_task: asyncio.Task | None = None
        self._running = False

    @property
    def is_running(self) -> bool:
        return self._running

    @property
    def grid_stages(self) -> dict[str, FireStage]:
        """Current tracked stage per grid (read-only copy)."""
        return dict(self._grid_stages)

    def start(self) -> None:
        """Start all background loops as asyncio tasks."""
        if self._running:
            logger.warning("FireProgressionEngine already running")
            return

        self._running = True
        self._progression_task = asyncio.create_task(
            self._progression_loop(), name="fire-progression"
        )
        self._firefighter_task = asyncio.create_task(
            self._firefighter_loop(), name="fire-firefighter"
        )
        self._cleanup_task = asyncio.create_task(
            self._cleanup_loop(), name="fire-cleanup"
        )
        logger.info(
            "FireProgressionEngine started (scan=%.1fs, firefighter=%.1fs, cleanup=%.1fs)",
            self._scan_interval,
            self._firefighter_interval,
            self._cleanup_interval,
        )

    async def stop(self) -> None:
        """Gracefully stop all background loops."""
        self._running = False
        tasks = [
            t
            for t in (self._progression_task, self._firefighter_task, self._cleanup_task)
            if t is not None
        ]
        for task in tasks:
            task.cancel()
        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)

        self._progression_task = None
        self._firefighter_task = None
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
    # Firefighter suppression
    # ------------------------------------------------------------------

    async def _firefighter_loop(self) -> None:
        """Periodically dispatch firefighters to suppress high-stage fires."""
        while self._running:
            try:
                await self._run_firefighter_sweep()
            except asyncio.CancelledError:
                break
            except Exception:
                logger.exception("Error in firefighter loop")
            await asyncio.sleep(self._firefighter_interval)

    async def _run_firefighter_sweep(self) -> None:
        """Sweep all active firefighter NPCs and apply suppression.

        Iterates over every active firefighter NPC (not just grids):
        - Each NPC decrements its assigned cell's fire count by remove_per_sweep
        - NPC status transitions: DISPATCHED → ACTIVE → DONE
        - NPC is removed only when the cell's fire count reaches 0
        """
        npc_ids_raw = await self._redis.smembers(ACTIVE_FIREFIGHTERS_KEY)
        if not npc_ids_raw:
            return

        for npc_id_raw in npc_ids_raw:
            npc_id = npc_id_raw.decode() if isinstance(npc_id_raw, bytes) else npc_id_raw
            npc = await self._load_firefighter(npc_id)
            if npc is None:
                # Orphaned entry — clean up
                await self._redis.srem(ACTIVE_FIREFIGHTERS_KEY, npc_id)
                continue

            grid_id = npc.grid_id
            now = time.time()
            active_count = await self._get_active_count(grid_id, now)

            # If fire is already at 0, retire the NPC
            if active_count <= 0:
                await self._retire_firefighter(npc)
                continue

            # Transition from DISPATCHED → ACTIVE on first sweep
            if npc.status == FirefighterStatus.DISPATCHED:
                npc.status = FirefighterStatus.ACTIVE
                await self._update_firefighter_status(npc)

            # Suppress fires: remove `remove_per_sweep` fires (default 2)
            removed = await self._suppress_fires(grid_id, npc.remove_per_sweep)
            if removed > 0:
                npc.fires_removed += removed
                await self._update_firefighter_fires_removed(npc)

                # Re-count after suppression
                new_count = await self._get_active_count(grid_id, time.time())
                new_stage = get_stage(new_count)
                prev_ff_stage = self._grid_stages.get(grid_id, FireStage.NONE)
                self._grid_stages[grid_id] = new_stage

                await self._broadcast_firefighter_alert(grid_id, removed, new_count)

                # Always broadcast fire:update after suppression (count changed)
                # so clients see the updated fire count even within same stage
                await self._broadcast_fire_count_update(grid_id, new_count, new_stage)

                # Additionally broadcast stage transition if stage changed
                if new_stage != prev_ff_stage:
                    await self._broadcast_stage_change(
                        grid_id, new_count, new_stage, prev_stage=prev_ff_stage,
                    )

                logger.info(
                    "Firefighter sweep: npc=%s grid=%s removed=%d remaining=%d",
                    npc.npc_id,
                    grid_id,
                    removed,
                    new_count,
                )

                # Retire firefighter if fire count reached 0
                if new_count <= 0:
                    await self._retire_firefighter(npc)

    async def _load_firefighter(self, npc_id: str) -> FirefighterNPC | None:
        """Load a firefighter NPC from Redis hash.

        Args:
            npc_id: The firefighter NPC identifier.

        Returns:
            FirefighterNPC if found, None otherwise.
        """
        key = f"{FIREFIGHTER_KEY_PREFIX}{npc_id}"
        data = {}
        for field in ("npc_id", "grid_id", "status", "dispatched_at",
                       "fires_removed", "remove_per_sweep", "target_stage"):
            val = await self._redis.hget(key, field)
            if val is None:
                return None
            data[field] = val.decode() if isinstance(val, bytes) else val

        return FirefighterNPC(
            npc_id=data["npc_id"],
            grid_id=data["grid_id"],
            status=FirefighterStatus(data["status"]),
            dispatched_at=float(data["dispatched_at"]),
            fires_removed=int(data["fires_removed"]),
            remove_per_sweep=int(data["remove_per_sweep"]),
            target_stage=int(data["target_stage"]),
        )

    async def _update_firefighter_status(self, npc: FirefighterNPC) -> None:
        """Update the status field of a firefighter NPC in Redis."""
        key = f"{FIREFIGHTER_KEY_PREFIX}{npc.npc_id}"
        await self._redis.hset(key, mapping={"status": npc.status.value})

    async def _update_firefighter_fires_removed(self, npc: FirefighterNPC) -> None:
        """Update the fires_removed field of a firefighter NPC in Redis."""
        key = f"{FIREFIGHTER_KEY_PREFIX}{npc.npc_id}"
        await self._redis.hset(key, mapping={"fires_removed": str(npc.fires_removed)})

    async def _retire_firefighter(self, npc: FirefighterNPC) -> None:
        """Retire a firefighter NPC: mark as DONE, broadcast, and clean up.

        Args:
            npc: The firefighter NPC to retire.
        """
        npc.status = FirefighterStatus.DONE
        await self._broadcast_firefighter_retire(npc)
        await self._remove_firefighter(npc.npc_id, npc.grid_id)
        logger.info(
            "Firefighter retired: npc_id=%s grid=%s fires_removed=%d",
            npc.npc_id,
            npc.grid_id,
            npc.fires_removed,
        )

    async def _broadcast_firefighter_retire(self, npc: FirefighterNPC) -> None:
        """Broadcast firefighter:retire event when an NPC finishes suppression."""
        payload = npc.to_broadcast_payload()
        await self._broadcaster.broadcast_to_room(
            "firefighter:retire", payload, room=npc.grid_id
        )
        await self._broadcaster.broadcast("firefighter:retire", payload)

    async def _retire_firefighter_for_grid(self, grid_id: str) -> None:
        """Retire and clean up the firefighter NPC assigned to a grid."""
        # Find the NPC for this grid
        npc_ids = await self._redis.smembers(ACTIVE_FIREFIGHTERS_KEY)
        for npc_id_raw in npc_ids:
            npc_id = npc_id_raw.decode() if isinstance(npc_id_raw, bytes) else npc_id_raw
            npc = await self._load_firefighter(npc_id)
            if npc is not None and npc.grid_id == grid_id:
                await self._retire_firefighter(npc)
                break

    async def _suppress_fires(self, grid_id: str, count: int) -> int:
        """Remove the oldest fire events from a grid (firefighter action).

        Uses ZPOPMIN to remove the fires with the earliest expiry
        (i.e., the oldest fires).

        Args:
            grid_id: Grid cell to suppress.
            count: Number of fires to remove.

        Returns:
            Actual number of fires removed.
        """
        key = f"{FIRE_KEY_PREFIX}{grid_id}"
        removed = await self._redis.zpopmin(key, count)
        return len(removed)

    async def _broadcast_firefighter_alert(
        self, grid_id: str, removed_count: int, remaining_count: int
    ) -> None:
        """Broadcast firefighter:alert event to grid room and globally."""
        import time as _time

        payload = {
            "grid_id": grid_id,
            "removed_count": removed_count,
            "remaining_count": remaining_count,
            "stage": int(get_stage(remaining_count)),
            "timestamp": _time.time(),
        }
        # Room-scoped for viewport subscribers
        await self._broadcaster.broadcast_to_room(
            "firefighter:alert", payload, room=grid_id
        )
        # Global for map overview clients
        await self._broadcaster.broadcast("firefighter:alert", payload)

    # ------------------------------------------------------------------
    # Firefighter NPC spawning
    # ------------------------------------------------------------------

    async def check_and_spawn_firefighter(self, grid_id: str, active_count: int) -> FirefighterNPC | None:
        """Check if a grid cell needs a firefighter NPC and spawn one if so.

        This is the main trigger detection entry point. Called whenever a
        fire event is registered or a stage change is detected. If the cell
        has reached stage 4+ (대화재) and doesn't already have an active
        firefighter, a new NPC is spawned and tracked in Redis.

        Args:
            grid_id: Grid cell to check.
            active_count: Current active fire count.

        Returns:
            The spawned FirefighterNPC, or None if no spawn needed.
        """
        if not should_spawn_firefighter(active_count):
            return None

        # Check if this grid already has an active firefighter
        if await self._grid_has_firefighter(grid_id):
            return None

        # Spawn a new firefighter NPC
        npc = create_firefighter_npc(grid_id, active_count)
        await self._store_firefighter(npc)
        await self._broadcast_firefighter_spawn(npc)

        logger.info(
            "Firefighter NPC spawned: npc_id=%s grid=%s stage=%d remove_per_sweep=%d",
            npc.npc_id,
            grid_id,
            npc.target_stage,
            npc.remove_per_sweep,
        )

        return npc

    async def _grid_has_firefighter(self, grid_id: str) -> bool:
        """Check if a grid cell already has an assigned firefighter NPC."""
        return await self._redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)

    async def _store_firefighter(self, npc: FirefighterNPC) -> None:
        """Store a firefighter NPC in Redis with TTL.

        Stores NPC data as a Redis hash and adds to tracking sets.
        """
        key = f"{FIREFIGHTER_KEY_PREFIX}{npc.npc_id}"
        data = npc.to_broadcast_payload()
        # Store as hash
        await self._redis.hset(key, mapping={k: str(v) for k, v in data.items()})
        await self._redis.expire(key, FIREFIGHTER_TTL_SEC)
        # Track in active sets
        await self._redis.sadd(ACTIVE_FIREFIGHTERS_KEY, npc.npc_id)
        await self._redis.sadd(FIREFIGHTER_GRID_INDEX_KEY, npc.grid_id)

    async def _remove_firefighter(self, npc_id: str, grid_id: str) -> None:
        """Remove a firefighter NPC from Redis tracking."""
        key = f"{FIREFIGHTER_KEY_PREFIX}{npc_id}"
        await self._redis.delete(key)
        await self._redis.srem(ACTIVE_FIREFIGHTERS_KEY, npc_id)
        await self._redis.srem(FIREFIGHTER_GRID_INDEX_KEY, grid_id)

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
        today_key = f"{STATS_DAILY_FIRES_PREFIX}{datetime.now(KST).strftime('%Y-%m-%d')}"
        await self._redis.incr(today_key)
        await self._redis.expire(today_key, STATS_DAILY_FIRES_TTL_SEC)

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
