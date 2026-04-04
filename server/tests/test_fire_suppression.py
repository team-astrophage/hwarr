"""Tests for fire suppression logic — firefighter NPC trigger conditions
and flame suppression/rollback mechanics.

Verifies the complete firefighter lifecycle:
1. Trigger: Stage 4+ (대화재) triggers firefighter NPC spawn
2. Suppression: Firefighter removes oldest fires via ZPOPMIN
3. Rollback: Fire stage decreases as count drops below thresholds
4. Retirement: Firefighter is retired when grid drops below stage 4
5. Edge cases: concurrent grids, re-escalation, empty grids
"""

from __future__ import annotations

import asyncio
import time
from typing import Any

import pytest

from jobs.fire_progression import (
    ACTIVE_GRIDS_KEY,
    FIRE_KEY_PREFIX,
    FireProgressionEngine,
)
from models.fire import (
    FireStage,
    STAGE_CONFIGS,
    get_stage,
    get_firefighter_remove_count,
    should_dispatch_firefighter,
)
from models.firefighter import (
    ACTIVE_FIREFIGHTERS_KEY,
    FIREFIGHTER_GRID_INDEX_KEY,
    FIREFIGHTER_KEY_PREFIX,
    FirefighterNPC,
    FirefighterStatus,
    create_firefighter_npc,
    should_spawn_firefighter,
)


# ---------------------------------------------------------------------------
# Shared test fixtures (in-memory fakes)
# ---------------------------------------------------------------------------


class FakeRedis:
    """In-memory fake Redis for testing without a real Redis server."""

    def __init__(self) -> None:
        self._sorted_sets: dict[str, dict[str, float]] = {}
        self._sets: dict[str, set[str]] = {}
        self._hashes: dict[str, dict[str, str]] = {}
        self._ttls: dict[str, float] = {}

    async def zadd(self, key: str, mapping: dict[str, float]) -> int:
        if key not in self._sorted_sets:
            self._sorted_sets[key] = {}
        added = 0
        for member, score in mapping.items():
            if member not in self._sorted_sets[key]:
                added += 1
            self._sorted_sets[key][member] = score
        return added

    async def zcount(self, key: str, min_score: float | str, max_score: float | str) -> int:
        ss = self._sorted_sets.get(key, {})
        mn = float("-inf") if min_score == "-inf" else float(min_score)
        mx = float("inf") if max_score == "+inf" else float(max_score)
        return sum(1 for score in ss.values() if mn <= score <= mx)

    async def zcard(self, key: str) -> int:
        return len(self._sorted_sets.get(key, {}))

    async def zpopmin(self, key: str, count: int = 1) -> list[tuple[str, float]]:
        ss = self._sorted_sets.get(key, {})
        sorted_items = sorted(ss.items(), key=lambda x: x[1])
        to_remove = sorted_items[:count]
        for member, _ in to_remove:
            del ss[member]
        return to_remove

    async def zremrangebyscore(self, key: str, min_score: float | str, max_score: float | str) -> int:
        ss = self._sorted_sets.get(key, {})
        mn = float("-inf") if min_score == "-inf" else float(min_score)
        mx = float("inf") if max_score == "+inf" else float(max_score)
        to_remove = [m for m, s in ss.items() if mn <= s <= mx]
        for m in to_remove:
            del ss[m]
        return len(to_remove)

    async def smembers(self, key: str) -> set[str]:
        return set(self._sets.get(key, set()))

    async def sadd(self, key: str, *members: str) -> int:
        if key not in self._sets:
            self._sets[key] = set()
        added = 0
        for m in members:
            if m not in self._sets[key]:
                added += 1
                self._sets[key].add(m)
        return added

    async def srem(self, key: str, *members: str) -> int:
        s = self._sets.get(key, set())
        removed = 0
        for m in members:
            if m in s:
                s.discard(m)
                removed += 1
        return removed

    async def sismember(self, key: str, member: str) -> bool:
        return member in self._sets.get(key, set())

    async def hset(self, key: str, mapping: dict[str, str] | None = None, **kwargs) -> int:
        if key not in self._hashes:
            self._hashes[key] = {}
        if mapping:
            self._hashes[key].update(mapping)
        self._hashes[key].update(kwargs)
        return len(mapping or kwargs)

    async def hget(self, key: str, field: str) -> str | None:
        return self._hashes.get(key, {}).get(field)

    async def expire(self, key: str, seconds: int) -> bool:
        self._ttls[key] = seconds
        return True

    async def delete(self, *keys: str) -> int:
        deleted = 0
        for key in keys:
            found = False
            if key in self._sorted_sets:
                del self._sorted_sets[key]
                found = True
            if key in self._hashes:
                del self._hashes[key]
                found = True
            if found:
                deleted += 1
        return deleted


class FakeBroadcaster:
    """Records all broadcast calls for assertion."""

    def __init__(self) -> None:
        self.events: list[tuple[str, Any, str | None]] = []

    async def broadcast_to_room(self, event: str, data: Any, room: str) -> None:
        self.events.append((event, data, room))

    async def broadcast(self, event: str, data: Any) -> None:
        self.events.append((event, data, None))

    def get_events(self, event_name: str) -> list[tuple[str, Any, str | None]]:
        return [e for e in self.events if e[0] == event_name]

    def clear(self) -> None:
        self.events.clear()


@pytest.fixture
def redis():
    return FakeRedis()


@pytest.fixture
def broadcaster():
    return FakeBroadcaster()


@pytest.fixture
def engine(redis, broadcaster):
    return FireProgressionEngine(
        redis=redis,
        broadcaster=broadcaster,
        scan_interval=0.1,
        firefighter_interval=0.2,
        cleanup_interval=0.3,
    )


# ---------------------------------------------------------------------------
# Helper: populate a grid with N fires
# ---------------------------------------------------------------------------


async def add_fires(engine, grid_id: str, count: int, ttl: float = 1800) -> None:
    """Register `count` fires into a grid cell."""
    expire_at = time.time() + ttl
    for i in range(count):
        await engine.register_fire(grid_id, f"evt-{grid_id}-{i:04d}", expire_at)


# ===========================================================================
# 1. Trigger conditions — when should a firefighter spawn?
# ===========================================================================


class TestFirefighterTriggerConditions:
    """Verify that firefighter NPCs are triggered exactly at stage 4+."""

    @pytest.mark.asyncio
    async def test_stage_3_does_not_trigger(self, engine, redis):
        """29 fires (stage 3 = 화재) should NOT trigger a firefighter."""
        await add_fires(engine, "trigger:a", 29)
        has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, "trigger:a")
        assert has_ff is False

    @pytest.mark.asyncio
    async def test_stage_4_triggers_on_threshold_cross(self, engine, redis, broadcaster):
        """Crossing from 29→30 fires (stage 3→4) triggers firefighter spawn."""
        # Add 29 fires (stage 3, no firefighter)
        await add_fires(engine, "trigger:b", 29)
        broadcaster.clear()

        # Add the 30th fire — crosses into stage 4
        expire_at = time.time() + 1800
        await engine.register_fire("trigger:b", "evt-trigger-30", expire_at)

        # Firefighter should now be assigned to this grid
        has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, "trigger:b")
        assert has_ff is True

        # Should have broadcast firefighter:spawn
        spawn_events = broadcaster.get_events("firefighter:spawn")
        assert len(spawn_events) >= 1

    @pytest.mark.asyncio
    async def test_stage_5_also_triggers(self, engine, redis):
        """50+ fires (stage 5 = 전소) should also trigger a firefighter."""
        await add_fires(engine, "trigger:c", 55)
        has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, "trigger:c")
        assert has_ff is True

    @pytest.mark.asyncio
    async def test_trigger_threshold_boundary_values(self):
        """Verify exact boundary values for firefighter trigger."""
        assert should_dispatch_firefighter(29) is False
        assert should_dispatch_firefighter(30) is True
        assert should_dispatch_firefighter(49) is True
        assert should_dispatch_firefighter(50) is True


# ===========================================================================
# 2. Suppression mechanics — how firefighters remove fires
# ===========================================================================


class TestSuppressionMechanics:
    """Verify fire removal logic during firefighter sweeps."""

    @pytest.mark.asyncio
    async def test_zpopmin_removes_oldest_fires(self, engine, redis):
        """Firefighter uses ZPOPMIN to remove the OLDEST fires first."""
        grid_id = "suppress:a"
        now = time.time()

        # Add fires with staggered expiry times
        for i in range(35):
            expire_at = now + 1800 + i  # slight offset so order is deterministic
            await engine.register_fire(grid_id, f"evt-{i:03d}", expire_at)

        initial_count = await redis.zcard(f"{FIRE_KEY_PREFIX}{grid_id}")
        assert initial_count == 35

        # Run one firefighter sweep
        await engine._run_firefighter_sweep()

        after_count = await redis.zcard(f"{FIRE_KEY_PREFIX}{grid_id}")
        # Stage 4 remove_per_sweep = 2, so 2 fires should be removed
        assert after_count == 33

    @pytest.mark.asyncio
    async def test_stage_4_removes_2_per_sweep(self):
        """Stage 4 (대화재) firefighter removes 2 fires per sweep."""
        assert get_firefighter_remove_count(30) == 2
        assert get_firefighter_remove_count(35) == 2
        assert get_firefighter_remove_count(49) == 2

    @pytest.mark.asyncio
    async def test_stage_5_removes_3_per_sweep(self):
        """Stage 5 (전소) firefighter removes 3 fires per sweep."""
        assert get_firefighter_remove_count(50) == 3
        assert get_firefighter_remove_count(100) == 3

    @pytest.mark.asyncio
    async def test_suppression_broadcasts_alert(self, engine, redis, broadcaster):
        """Firefighter sweep should broadcast firefighter:alert."""
        await add_fires(engine, "suppress:b", 35)
        broadcaster.clear()

        await engine._run_firefighter_sweep()

        alerts = broadcaster.get_events("firefighter:alert")
        assert len(alerts) >= 1
        payload = alerts[0][1]
        assert payload["grid_id"] == "suppress:b"
        assert payload["removed_count"] > 0
        assert payload["remaining_count"] < 35

    @pytest.mark.asyncio
    async def test_suppression_broadcasts_fire_update(self, engine, redis, broadcaster):
        """After suppression, fire:update should reflect the new stage."""
        await add_fires(engine, "suppress:c", 35)
        broadcaster.clear()

        await engine._run_firefighter_sweep()

        updates = broadcaster.get_events("fire:update")
        assert len(updates) >= 2  # room + global broadcast
        # After removing 2 from 35, still 33 → still stage 4
        update_payload = updates[0][1]
        assert update_payload["grid_id"] == "suppress:c"
        assert update_payload["active_count"] == 33


# ===========================================================================
# 3. Rollback mechanics — stage decreases after suppression
# ===========================================================================


class TestStageRollback:
    """Verify that fire stage properly decreases after firefighter suppression."""

    @pytest.mark.asyncio
    async def test_rollback_from_stage_4_to_3(self, engine, redis, broadcaster):
        """Repeated sweeps should bring stage 4 back to stage 3."""
        grid_id = "rollback:a"
        # Add exactly 30 fires (threshold for stage 4)
        await add_fires(engine, grid_id, 30)

        # Run sweeps until count drops below 30
        for _ in range(10):  # safety limit
            await engine._run_firefighter_sweep()
            count = await redis.zcard(f"{FIRE_KEY_PREFIX}{grid_id}")
            if count < 30:
                break

        # After enough sweeps, should be below stage 4 threshold
        final_count = await redis.zcard(f"{FIRE_KEY_PREFIX}{grid_id}")
        final_stage = get_stage(final_count)
        assert final_stage < FireStage.DAEHWAJAE
        assert final_count < 30

    @pytest.mark.asyncio
    async def test_rollback_broadcasts_decreasing_stages(self, engine, redis, broadcaster):
        """As suppression progresses, broadcasts should show decreasing stages."""
        grid_id = "rollback:b"
        await add_fires(engine, grid_id, 32)
        broadcaster.clear()

        # Run sweeps — stage 4 removes 2 per sweep
        # 32 → 30 → 28 (drops to stage 3)
        await engine._run_firefighter_sweep()  # 32 → 30
        await engine._run_firefighter_sweep()  # 30 → 28 (below threshold)

        # Check that we saw a fire:update with stage 3 or lower
        updates = broadcaster.get_events("fire:update")
        stages_seen = [u[1]["stage"] for u in updates]

        # Should have seen at least one update with stage < 4
        assert any(s < 4 for s in stages_seen), f"Expected stage < 4 in broadcasts, got {stages_seen}"

    @pytest.mark.asyncio
    async def test_firefighter_continues_below_stage_4(self, engine, redis, broadcaster):
        """Firefighter NPC should continue suppressing below stage 4 until fire reaches 0."""
        grid_id = "rollback:c"
        # Add exactly 31 fires — one sweep of 2 → 29 (below threshold)
        await add_fires(engine, grid_id, 31)

        # Verify firefighter exists
        has_ff_before = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
        assert has_ff_before is True

        # First sweep: 31 → 29 (below stage 4 threshold of 30)
        await engine._run_firefighter_sweep()

        # Firefighter should still be active (continues until fire = 0)
        has_ff_after = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
        assert has_ff_after is True

    @pytest.mark.asyncio
    async def test_firefighter_retired_only_at_zero(self, engine, redis, broadcaster):
        """Firefighter is only retired when fire count reaches 0, not at stage boundary."""
        grid_id = "rollback:d"
        await add_fires(engine, grid_id, 4)  # 4 fires, stage 1

        # Manually spawn a firefighter for this grid (simulating prior stage 4)
        from models.firefighter import create_firefighter_npc
        npc = create_firefighter_npc(grid_id, active_count=30)  # pretend it was stage 4
        await engine._store_firefighter(npc)

        # Run sweeps until fire reaches 0
        await engine._run_firefighter_sweep()  # 4 → 2
        has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
        assert has_ff is True  # still active

        await engine._run_firefighter_sweep()  # 2 → 0
        has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
        assert has_ff is False  # now retired


# ===========================================================================
# 4. Full lifecycle — spawn → suppress → rollback → retire
# ===========================================================================


class TestFullFirefighterLifecycle:
    """End-to-end test of the complete firefighter lifecycle."""

    @pytest.mark.asyncio
    async def test_complete_lifecycle(self, engine, redis, broadcaster):
        """Full lifecycle: escalate → spawn → suppress → retire at zero."""
        grid_id = "lifecycle:a"

        # Phase 1: Escalate to stage 4 (triggers firefighter)
        await add_fires(engine, grid_id, 30)
        has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
        assert has_ff is True, "Firefighter should be spawned at stage 4"

        spawn_events = broadcaster.get_events("firefighter:spawn")
        assert len(spawn_events) >= 1, "Should broadcast firefighter:spawn"

        # Phase 2: Firefighter suppresses fires
        broadcaster.clear()
        count_before = await redis.zcard(f"{FIRE_KEY_PREFIX}{grid_id}")

        await engine._run_firefighter_sweep()

        count_after = await redis.zcard(f"{FIRE_KEY_PREFIX}{grid_id}")
        assert count_after < count_before, "Sweep should remove fires"

        alerts = broadcaster.get_events("firefighter:alert")
        assert len(alerts) >= 1, "Should broadcast firefighter:alert"

        # Phase 3: Keep sweeping until firefighter retires (at fire count = 0)
        max_sweeps = 20
        for i in range(max_sweeps):
            await engine._run_firefighter_sweep()
            has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
            if not has_ff:
                break

        assert not has_ff, "Firefighter should be retired after enough sweeps"

        # Phase 4: Verify final state — all fires suppressed
        final_count = await redis.zcard(f"{FIRE_KEY_PREFIX}{grid_id}")
        assert final_count == 0, "All fires should be suppressed"

    @pytest.mark.asyncio
    async def test_re_escalation_spawns_new_firefighter(self, engine, redis, broadcaster):
        """If fires re-escalate to stage 4 after full suppression, a new firefighter spawns."""
        grid_id = "lifecycle:b"

        # Phase 1: Escalate and suppress to 0
        await add_fires(engine, grid_id, 4)
        # Manually spawn firefighter (as if it was at stage 4 before)
        from models.firefighter import create_firefighter_npc
        npc = create_firefighter_npc(grid_id, active_count=30)
        await engine._store_firefighter(npc)

        # Sweep until retired
        for _ in range(10):
            count = await redis.zcard(f"{FIRE_KEY_PREFIX}{grid_id}")
            if count <= 0:
                break
            await engine._run_firefighter_sweep()

        has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
        assert has_ff is False, "Firefighter should be retired at 0 fires"

        # Phase 2: Re-escalate by adding fires to stage 4
        broadcaster.clear()
        expire_at = time.time() + 1800
        for i in range(30):
            await engine.register_fire(grid_id, f"re-escalate-{i}", expire_at)

        # Should have spawned a new firefighter
        has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
        assert has_ff is True, "New firefighter should spawn on re-escalation"


# ===========================================================================
# 5. Concurrent grid suppression
# ===========================================================================


class TestConcurrentGridSuppression:
    """Verify that firefighters work correctly across multiple grids."""

    @pytest.mark.asyncio
    async def test_multiple_grids_suppressed_independently(self, engine, redis, broadcaster):
        """Each grid gets its own firefighter and independent suppression."""
        grid_a = "multi:a"
        grid_b = "multi:b"

        await add_fires(engine, grid_a, 35)
        await add_fires(engine, grid_b, 50)

        # Both should have firefighters
        assert await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_a)
        assert await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_b)

        broadcaster.clear()
        await engine._run_firefighter_sweep()

        alerts = broadcaster.get_events("firefighter:alert")
        grid_ids_alerted = {a[1]["grid_id"] for a in alerts}
        assert grid_a in grid_ids_alerted
        assert grid_b in grid_ids_alerted

    @pytest.mark.asyncio
    async def test_one_grid_retires_other_continues(self, engine, redis):
        """One grid's firefighter retiring doesn't affect another."""
        grid_a = "multi:c"  # small fire count, will reach 0 faster
        grid_b = "multi:d"  # deeply stage 5, many sweeps needed

        await add_fires(engine, grid_a, 30)
        await add_fires(engine, grid_b, 60)

        # Sweep until grid_a's fire reaches 0 (15 sweeps of 2)
        for _ in range(20):
            count_a = await redis.zcard(f"{FIRE_KEY_PREFIX}{grid_a}")
            if count_a <= 0:
                break
            await engine._run_firefighter_sweep()

        # Grid A's firefighter should be retired (fire count = 0)
        assert not await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_a)

        # Grid B's firefighter should still be active
        assert await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_b)


# ===========================================================================
# 6. Edge cases
# ===========================================================================


class TestSuppressionEdgeCases:
    """Edge cases in fire suppression logic."""

    @pytest.mark.asyncio
    async def test_suppress_more_than_available(self, engine, redis):
        """If remove count exceeds remaining fires, just remove all."""
        grid_id = "edge:a"
        # Add exactly 30 fires — at stage 4, remove 2 per sweep
        # But let's manually test direct suppression
        expire_at = time.time() + 1800
        key = f"{FIRE_KEY_PREFIX}{grid_id}"

        # Add just 1 fire
        await redis.zadd(key, {"single-fire": expire_at})
        await redis.sadd(ACTIVE_GRIDS_KEY, grid_id)

        # Try to remove 5 — should only remove 1
        removed = await engine._suppress_fires(grid_id, 5)
        assert removed == 1

        remaining = await redis.zcard(key)
        assert remaining == 0

    @pytest.mark.asyncio
    async def test_suppress_empty_grid(self, engine, redis):
        """Suppressing an empty grid should not crash."""
        removed = await engine._suppress_fires("edge:empty", 3)
        assert removed == 0

    @pytest.mark.asyncio
    async def test_firefighter_alert_payload_structure(self, engine, redis, broadcaster):
        """Verify the firefighter:alert payload has all required fields."""
        await add_fires(engine, "edge:payload", 35)
        broadcaster.clear()

        await engine._run_firefighter_sweep()

        alerts = broadcaster.get_events("firefighter:alert")
        assert len(alerts) >= 1
        payload = alerts[0][1]
        assert "grid_id" in payload
        assert "removed_count" in payload
        assert "remaining_count" in payload
        assert "stage" in payload
        assert isinstance(payload["stage"], int)

    @pytest.mark.asyncio
    async def test_stage_config_consistency(self):
        """Verify all stage configs are internally consistent."""
        for stage, cfg in STAGE_CONFIGS.items():
            if cfg.triggers_firefighter:
                assert cfg.firefighter_remove_count > 0, (
                    f"Stage {stage.name} triggers firefighter but has 0 remove count"
                )
            if not cfg.triggers_firefighter:
                assert cfg.firefighter_remove_count == 0, (
                    f"Stage {stage.name} has remove count but doesn't trigger firefighter"
                )

    @pytest.mark.asyncio
    async def test_scan_triggers_firefighter_for_existing_stage4(self, engine, redis, broadcaster):
        """The periodic scan should also trigger firefighter spawn (not just register_fire)."""
        grid_id = "edge:scan"
        expire_at = time.time() + 1800

        # Manually add 35 fires to Redis without going through register_fire
        key = f"{FIRE_KEY_PREFIX}{grid_id}"
        for i in range(35):
            await redis.zadd(key, {f"manual-evt-{i}": expire_at})
        await redis.sadd(ACTIVE_GRIDS_KEY, grid_id)

        # No firefighter yet (we bypassed register_fire)
        has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
        assert has_ff is False

        # Run the scan — should detect stage 4 and spawn firefighter
        await engine._scan_and_update_stages()

        has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
        assert has_ff is True


# ===========================================================================
# 7. NPC status transitions & broadcast lifecycle
# ===========================================================================


class TestNPCStatusTransitions:
    """Verify NPC status transitions: DISPATCHED → ACTIVE → DONE."""

    @pytest.mark.asyncio
    async def test_dispatched_to_active_on_first_sweep(self, engine, redis, broadcaster):
        """NPC status transitions from DISPATCHED to ACTIVE on first sweep."""
        grid_id = "status:a"
        await add_fires(engine, grid_id, 35)

        # Get the NPC ID
        npc_ids = await redis.smembers(ACTIVE_FIREFIGHTERS_KEY)
        assert len(npc_ids) >= 1
        npc_id = list(npc_ids)[0]

        # Check initial status
        status_before = await redis.hget(f"{FIREFIGHTER_KEY_PREFIX}{npc_id}", "status")
        assert status_before == "dispatched"

        # First sweep transitions to active
        await engine._run_firefighter_sweep()

        status_after = await redis.hget(f"{FIREFIGHTER_KEY_PREFIX}{npc_id}", "status")
        assert status_after == "active"

    @pytest.mark.asyncio
    async def test_retire_broadcasts_done_event(self, engine, redis, broadcaster):
        """When firefighter retires, firefighter:retire event is broadcast."""
        grid_id = "status:b"
        await add_fires(engine, grid_id, 4)

        # Manually spawn firefighter
        from models.firefighter import create_firefighter_npc
        npc = create_firefighter_npc(grid_id, active_count=30)
        await engine._store_firefighter(npc)

        broadcaster.clear()

        # Sweep until retired (4 → 2 → 0)
        for _ in range(5):
            await engine._run_firefighter_sweep()
            has_ff = await redis.sismember(FIREFIGHTER_GRID_INDEX_KEY, grid_id)
            if not has_ff:
                break

        retire_events = broadcaster.get_events("firefighter:retire")
        assert len(retire_events) >= 1
        payload = retire_events[0][1]
        assert payload["grid_id"] == grid_id
        assert payload["status"] == "done"

    @pytest.mark.asyncio
    async def test_fires_removed_counter_accumulates(self, engine, redis, broadcaster):
        """The fires_removed counter on the NPC should accumulate across sweeps."""
        grid_id = "status:c"
        await add_fires(engine, grid_id, 35)

        npc_ids = list(await redis.smembers(ACTIVE_FIREFIGHTERS_KEY))
        npc_id = npc_ids[0]

        # First sweep removes 2
        await engine._run_firefighter_sweep()
        removed_1 = await redis.hget(f"{FIREFIGHTER_KEY_PREFIX}{npc_id}", "fires_removed")

        # Second sweep removes 2 more
        await engine._run_firefighter_sweep()
        removed_2 = await redis.hget(f"{FIREFIGHTER_KEY_PREFIX}{npc_id}", "fires_removed")

        assert int(removed_1) == 2
        assert int(removed_2) == 4
