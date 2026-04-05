"""Tests for fire stage progression engine."""

from __future__ import annotations

import asyncio
import time
from typing import Any
from unittest.mock import AsyncMock, MagicMock

import pytest

from jobs.fire_progression import (
    ACTIVE_GRIDS_KEY,
    FIRE_KEY_PREFIX,
    FireProgressionEngine,
)
from models.fire import FireStage, get_stage


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


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


class FakeRedis:
    """In-memory fake Redis for testing without a real Redis server.

    Supports: zadd, zcount, zcard, zpopmin, zremrangebyscore,
              smembers, sadd, srem, sismember, hset, expire, delete.
    """

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

    async def srem(self, key: str, *members: str) -> int:
        s = self._sets.get(key, set())
        removed = 0
        for m in members:
            if m in s:
                s.discard(m)
                removed += 1
        return removed

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
        cleanup_interval=0.3,
    )


# ---------------------------------------------------------------------------
# Tests: register_fire
# ---------------------------------------------------------------------------


class TestRegisterFire:
    """Test fire registration and immediate stage detection."""

    @pytest.mark.asyncio
    async def test_register_first_fire(self, engine, redis):
        """Registering first fire should return count=1 and track grid."""
        expire_at = time.time() + 1800  # 30 min from now
        count = await engine.register_fire("10:20", "evt-001", expire_at)

        assert count == 1
        assert "10:20" in await redis.smembers(ACTIVE_GRIDS_KEY)

    @pytest.mark.asyncio
    async def test_register_multiple_fires(self, engine, redis):
        """Multiple fires in same grid should accumulate count."""
        expire_at = time.time() + 1800
        await engine.register_fire("10:20", "evt-001", expire_at)
        await engine.register_fire("10:20", "evt-002", expire_at)
        count = await engine.register_fire("10:20", "evt-003", expire_at)

        assert count == 3

    @pytest.mark.asyncio
    async def test_stage_change_broadcasts(self, engine, broadcaster):
        """Stage transition should trigger a fire:update broadcast."""
        expire_at = time.time() + 1800

        # First fire: NONE → BULSSSI (stage 0→1)
        await engine.register_fire("5:5", "evt-001", expire_at)
        updates = broadcaster.get_events("fire:update")
        assert len(updates) == 2  # room + global broadcast
        assert updates[0][2] == "5:5"  # room = grid_id
        assert updates[1][2] is None   # global broadcast

    @pytest.mark.asyncio
    async def test_no_broadcast_same_stage(self, engine, broadcaster):
        """No broadcast when count changes but stage stays the same."""
        expire_at = time.time() + 1800

        await engine.register_fire("5:5", "evt-001", expire_at)
        broadcaster.clear()

        # Second fire: still stage 1 (BULSSSI is 1-4)
        await engine.register_fire("5:5", "evt-002", expire_at)
        updates = broadcaster.get_events("fire:update")
        assert len(updates) == 0

    @pytest.mark.asyncio
    async def test_stage_escalation_sequence(self, engine, broadcaster):
        """Fires should escalate through stages as count increases."""
        expire_at = time.time() + 1800

        # Add fires to cross stage boundaries
        stages_seen = []
        for i in range(1, 51):
            broadcaster.clear()
            await engine.register_fire("1:1", f"evt-{i:03d}", expire_at)
            updates = broadcaster.get_events("fire:update")
            if updates:
                stage = updates[0][1]["stage"]
                stages_seen.append(stage)

        # Should have seen stages 1, 2, 3, 4, 5
        assert 1 in stages_seen
        assert 2 in stages_seen
        assert 3 in stages_seen
        assert 4 in stages_seen
        assert 5 in stages_seen


# ---------------------------------------------------------------------------
# Tests: _scan_and_update_stages
# ---------------------------------------------------------------------------


class TestScanAndUpdateStages:
    """Test the periodic stage scan logic."""

    @pytest.mark.asyncio
    async def test_detects_stage_change_from_expiry(self, engine, redis, broadcaster):
        """When fires expire, scan should detect stage decrease."""
        now = time.time()

        # Add 5 fires expiring soon
        grid_id = "7:7"
        for i in range(5):
            await engine.register_fire(grid_id, f"evt-{i}", now + 0.1)

        # Wait for expiry
        await asyncio.sleep(0.15)
        broadcaster.clear()

        # Scan should detect all fires expired → stage drops to NONE
        await engine._scan_and_update_stages()

        updates = broadcaster.get_events("fire:update")
        # Should broadcast the stage change
        assert len(updates) >= 1
        assert updates[-1][1]["stage"] == 0  # NONE

    @pytest.mark.asyncio
    async def test_empty_grid_removed_from_tracking(self, engine, redis, broadcaster):
        """Grids with no active fires should be removed from active_grids."""
        now = time.time()
        grid_id = "8:8"

        # Add a fire that expires immediately
        await engine.register_fire(grid_id, "evt-temp", now - 1)
        broadcaster.clear()

        await engine._scan_and_update_stages()

        # Grid should be removed from active tracking
        active = await redis.smembers(ACTIVE_GRIDS_KEY)
        assert grid_id not in active
        assert grid_id not in engine.grid_stages

    @pytest.mark.asyncio
    async def test_scan_with_no_grids(self, engine, redis):
        """Scan with no active grids should complete without error."""
        await engine._scan_and_update_stages()
        # No exception = pass


# ---------------------------------------------------------------------------
# Tests: cleanup
# ---------------------------------------------------------------------------


class TestCleanup:
    """Test expired event cleanup."""

    @pytest.mark.asyncio
    async def test_cleanup_removes_expired_events(self, engine, redis):
        """Cleanup should remove events whose score < now."""
        now = time.time()

        # Add mix of expired and active fires
        await redis.zadd(f"{FIRE_KEY_PREFIX}9:9", {
            "expired-1": now - 100,
            "expired-2": now - 50,
            "active-1": now + 1800,
        })
        await redis.sadd(ACTIVE_GRIDS_KEY, "9:9")

        await engine._run_cleanup()

        remaining = await redis.zcard(f"{FIRE_KEY_PREFIX}9:9")
        assert remaining == 1  # Only active-1 remains

    @pytest.mark.asyncio
    async def test_cleanup_deletes_empty_keys(self, engine, redis):
        """Cleanup should delete keys with no remaining members."""
        now = time.time()

        await redis.zadd(f"{FIRE_KEY_PREFIX}11:11", {"expired-1": now - 100})
        await redis.sadd(ACTIVE_GRIDS_KEY, "11:11")

        await engine._run_cleanup()

        remaining = await redis.zcard(f"{FIRE_KEY_PREFIX}11:11")
        assert remaining == 0
        active = await redis.smembers(ACTIVE_GRIDS_KEY)
        assert "11:11" not in active


# ---------------------------------------------------------------------------
# Tests: start/stop lifecycle
# ---------------------------------------------------------------------------


class TestLifecycle:
    """Test engine start/stop."""

    @pytest.mark.asyncio
    async def test_start_creates_tasks(self, engine):
        """Starting the engine should create background tasks."""
        engine.start()
        assert engine.is_running
        assert engine._progression_task is not None
        assert engine._cleanup_task is not None

        await engine.stop()
        assert not engine.is_running

    @pytest.mark.asyncio
    async def test_double_start_is_safe(self, engine):
        """Starting twice should not create duplicate tasks."""
        engine.start()
        engine.start()  # Should log warning, not crash
        assert engine.is_running
        await engine.stop()

    @pytest.mark.asyncio
    async def test_stop_cancels_tasks(self, engine):
        """Stopping should cancel all running tasks."""
        engine.start()
        await engine.stop()
        assert engine._progression_task is None
        assert engine._cleanup_task is None

    @pytest.mark.asyncio
    async def test_background_loops_run(self, engine, redis, broadcaster):
        """Background loops should execute scans periodically."""
        expire_at = time.time() + 1800

        # Register fire before starting engine
        await engine.register_fire("bg:test", "evt-bg", expire_at)
        broadcaster.clear()

        engine.start()
        # Let it run a couple of scan intervals
        await asyncio.sleep(0.3)
        await engine.stop()

        # Engine should have run without crashing
        # (the actual scan results depend on timing, but no exceptions)


# ---------------------------------------------------------------------------
# Tests: grid_stages property
# ---------------------------------------------------------------------------


class TestGridStages:
    """Test stage tracking state."""

    @pytest.mark.asyncio
    async def test_tracks_stages_after_registration(self, engine):
        """grid_stages should reflect registered fires."""
        expire_at = time.time() + 1800
        await engine.register_fire("t:1", "evt-1", expire_at)

        stages = engine.grid_stages
        assert "t:1" in stages
        assert stages["t:1"] == FireStage.BULSSSI

    @pytest.mark.asyncio
    async def test_grid_stages_is_copy(self, engine):
        """grid_stages should return a copy, not the internal dict."""
        expire_at = time.time() + 1800
        await engine.register_fire("t:2", "evt-1", expire_at)

        stages = engine.grid_stages
        stages["t:2"] = FireStage.JEONSO  # mutate copy

        assert engine.grid_stages["t:2"] == FireStage.BULSSSI  # original unchanged
