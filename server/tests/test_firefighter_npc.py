"""Tests for firefighter NPC trigger detection and spawn logic.

Verifies that:
- Firefighter NPCs are spawned when a cell reaches stage 4+ (대화재)
- NPCs are NOT spawned below stage 4
- Duplicate NPCs are not spawned for the same grid cell
- NPC model serialization works correctly
- Integration with FireProgressionEngine spawn detection
"""

from __future__ import annotations

import asyncio
import time
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from models.fire import FireStage, get_stage, should_dispatch_firefighter
from models.firefighter import (
    FIREFIGHTER_KEY_PREFIX,
    FIREFIGHTER_GRID_INDEX_KEY,
    ACTIVE_FIREFIGHTERS_KEY,
    FirefighterNPC,
    FirefighterStatus,
    create_firefighter_npc,
    should_spawn_firefighter,
)


# ---------------------------------------------------------------------------
# Unit tests: firefighter model and spawn detection
# ---------------------------------------------------------------------------


class TestShouldSpawnFirefighter:
    """Trigger detection: when should a firefighter NPC be spawned?"""

    @pytest.mark.parametrize(
        "count,expected",
        [
            (0, False),
            (1, False),
            (4, False),
            (5, False),
            (14, False),
            (15, False),
            (29, False),
            (30, True),   # Stage 4 threshold (대화재)
            (40, True),
            (49, True),
            (50, True),   # Stage 5 threshold (전소)
            (100, True),
        ],
    )
    def test_spawn_threshold(self, count: int, expected: bool):
        assert should_spawn_firefighter(count) == expected

    def test_consistent_with_should_dispatch(self):
        """should_spawn_firefighter must agree with should_dispatch_firefighter."""
        for count in range(0, 60):
            assert should_spawn_firefighter(count) == should_dispatch_firefighter(count)


class TestCreateFirefighterNPC:
    """NPC factory function."""

    def test_creates_npc_for_stage_4(self):
        npc = create_firefighter_npc("100:200", active_count=35)
        assert npc.grid_id == "100:200"
        assert npc.status == FirefighterStatus.DISPATCHED
        assert npc.target_stage == FireStage.DAEHWAJAE.value
        assert npc.remove_per_sweep == 2  # stage 4 removes 2 per sweep
        assert npc.fires_removed == 0
        assert npc.npc_id.startswith("ff-")

    def test_creates_npc_for_stage_5(self):
        npc = create_firefighter_npc("300:400", active_count=55)
        assert npc.target_stage == FireStage.JEONSO.value
        assert npc.remove_per_sweep == 3  # stage 5 removes 3 per sweep

    def test_npc_id_is_unique(self):
        npc1 = create_firefighter_npc("1:1", active_count=30)
        npc2 = create_firefighter_npc("1:1", active_count=30)
        assert npc1.npc_id != npc2.npc_id

    def test_dispatched_at_is_recent(self):
        before = time.time()
        npc = create_firefighter_npc("1:1", active_count=30)
        after = time.time()
        assert before <= npc.dispatched_at <= after


class TestFirefighterNPCModel:
    """NPC model and serialization."""

    def test_broadcast_payload(self):
        npc = FirefighterNPC(
            npc_id="ff-test1234",
            grid_id="10:20",
            status=FirefighterStatus.ACTIVE,
            dispatched_at=1000.0,
            fires_removed=5,
            remove_per_sweep=2,
            target_stage=4,
        )
        payload = npc.to_broadcast_payload()
        assert payload["npc_id"] == "ff-test1234"
        assert payload["grid_id"] == "10:20"
        assert payload["status"] == "active"
        assert payload["dispatched_at"] == 1000.0
        assert payload["fires_removed"] == 5
        assert payload["remove_per_sweep"] == 2
        assert payload["target_stage"] == 4

    def test_default_status_is_dispatched(self):
        npc = FirefighterNPC(
            npc_id="ff-x",
            grid_id="1:1",
            dispatched_at=1000.0,
            remove_per_sweep=2,
            target_stage=4,
        )
        assert npc.status == FirefighterStatus.DISPATCHED

    def test_status_enum_values(self):
        assert FirefighterStatus.DISPATCHED == "dispatched"
        assert FirefighterStatus.ACTIVE == "active"
        assert FirefighterStatus.DONE == "done"


# ---------------------------------------------------------------------------
# Integration tests: FireProgressionEngine spawn detection
# ---------------------------------------------------------------------------


class MockRedis:
    """Minimal async Redis mock for testing firefighter spawn logic."""

    def __init__(self):
        self._data: dict = {}
        self._sets: dict[str, set] = {}
        self._hashes: dict[str, dict] = {}
        self._ttls: dict[str, int] = {}

    async def sismember(self, key: str, member: str) -> bool:
        return member in self._sets.get(key, set())

    async def sadd(self, key: str, *members: str) -> int:
        if key not in self._sets:
            self._sets[key] = set()
        added = 0
        for m in members:
            if m not in self._sets[key]:
                self._sets[key].add(m)
                added += 1
        return added

    async def srem(self, key: str, *members: str) -> int:
        if key not in self._sets:
            return 0
        removed = 0
        for m in members:
            if m in self._sets[key]:
                self._sets[key].discard(m)
                removed += 1
        return removed

    async def smembers(self, key: str) -> set:
        return self._sets.get(key, set())

    async def hset(self, key: str, mapping: dict = None, **kwargs) -> int:
        if key not in self._hashes:
            self._hashes[key] = {}
        if mapping:
            self._hashes[key].update(mapping)
        self._hashes[key].update(kwargs)
        return len(mapping or kwargs)

    async def hget(self, key: str, field: str):
        return self._hashes.get(key, {}).get(field)

    async def expire(self, key: str, seconds: int) -> bool:
        self._ttls[key] = seconds
        return True

    async def delete(self, *keys: str) -> int:
        count = 0
        for key in keys:
            if key in self._hashes:
                del self._hashes[key]
                count += 1
            if key in self._data:
                del self._data[key]
                count += 1
        return count

    async def zcount(self, key: str, min_score, max_score) -> int:
        return 0

    async def zadd(self, key: str, mapping: dict) -> int:
        return len(mapping)

    async def zpopmin(self, key: str, count: int) -> list:
        return []

    async def zremrangebyscore(self, key: str, min_score, max_score) -> int:
        return 0

    async def zcard(self, key: str) -> int:
        return 0


class MockBroadcaster:
    """Minimal broadcaster mock that records calls."""

    def __init__(self):
        self.events: list[tuple[str, dict, str | None]] = []

    async def broadcast_to_room(self, event: str, data, room: str) -> None:
        self.events.append((event, data, room))

    async def broadcast(self, event: str, data) -> None:
        self.events.append((event, data, None))


class TestFireProgressionEngineSpawn:
    """Integration: firefighter NPC spawn via FireProgressionEngine."""

    def _make_engine(self):
        from jobs.fire_progression import FireProgressionEngine

        redis = MockRedis()
        broadcaster = MockBroadcaster()
        engine = FireProgressionEngine(redis, broadcaster)
        return engine, redis, broadcaster

    @pytest.mark.asyncio
    async def test_spawn_at_stage_4(self):
        """A firefighter NPC is spawned when active_count reaches stage 4."""
        engine, redis, broadcaster = self._make_engine()

        npc = await engine.check_and_spawn_firefighter("50:60", active_count=35)

        assert npc is not None
        assert npc.grid_id == "50:60"
        assert npc.status == FirefighterStatus.DISPATCHED
        assert npc.target_stage == FireStage.DAEHWAJAE.value

        # Verify it was stored in Redis
        assert "50:60" in redis._sets.get(FIREFIGHTER_GRID_INDEX_KEY, set())
        assert npc.npc_id in redis._sets.get(ACTIVE_FIREFIGHTERS_KEY, set())

        # Verify broadcast was emitted
        spawn_events = [e for e in broadcaster.events if e[0] == "firefighter:spawn"]
        assert len(spawn_events) == 2  # room + global broadcast

    @pytest.mark.asyncio
    async def test_spawn_at_stage_5(self):
        """A firefighter NPC is spawned when active_count reaches stage 5."""
        engine, redis, broadcaster = self._make_engine()

        npc = await engine.check_and_spawn_firefighter("10:20", active_count=55)

        assert npc is not None
        assert npc.target_stage == FireStage.JEONSO.value
        assert npc.remove_per_sweep == 3

    @pytest.mark.asyncio
    async def test_no_spawn_below_stage_4(self):
        """No firefighter NPC for stages 1-3."""
        engine, redis, broadcaster = self._make_engine()

        for count in [0, 1, 5, 15, 29]:
            npc = await engine.check_and_spawn_firefighter("1:1", active_count=count)
            assert npc is None

        # No firefighter tracking should exist
        assert len(redis._sets.get(FIREFIGHTER_GRID_INDEX_KEY, set())) == 0

    @pytest.mark.asyncio
    async def test_no_duplicate_spawn(self):
        """Only one firefighter per grid cell at a time."""
        engine, redis, broadcaster = self._make_engine()

        npc1 = await engine.check_and_spawn_firefighter("5:5", active_count=35)
        assert npc1 is not None

        # Second spawn attempt for same grid should return None
        npc2 = await engine.check_and_spawn_firefighter("5:5", active_count=40)
        assert npc2 is None

    @pytest.mark.asyncio
    async def test_different_grids_get_separate_npcs(self):
        """Different grid cells can each have their own firefighter."""
        engine, redis, broadcaster = self._make_engine()

        npc1 = await engine.check_and_spawn_firefighter("1:1", active_count=30)
        npc2 = await engine.check_and_spawn_firefighter("2:2", active_count=50)

        assert npc1 is not None
        assert npc2 is not None
        assert npc1.npc_id != npc2.npc_id
        assert npc1.grid_id == "1:1"
        assert npc2.grid_id == "2:2"

    @pytest.mark.asyncio
    async def test_spawn_broadcast_payload(self):
        """Firefighter spawn broadcasts contain correct payload."""
        engine, redis, broadcaster = self._make_engine()

        npc = await engine.check_and_spawn_firefighter("7:8", active_count=30)

        spawn_events = [e for e in broadcaster.events if e[0] == "firefighter:spawn"]
        # Room broadcast
        room_event = [e for e in spawn_events if e[2] == "7:8"]
        assert len(room_event) == 1
        payload = room_event[0][1]
        assert payload["grid_id"] == "7:8"
        assert payload["npc_id"] == npc.npc_id
        assert payload["status"] == "dispatched"

    @pytest.mark.asyncio
    async def test_register_fire_triggers_spawn(self):
        """register_fire should trigger firefighter spawn when crossing stage 4."""
        engine, redis, broadcaster = self._make_engine()

        # Mock active count to return stage 4 threshold
        async def mock_zcount(key, min_score, max_score):
            return 30

        redis.zcount = mock_zcount

        # Set previous stage to stage 3 so we detect transition
        engine._grid_stages["99:99"] = FireStage.HWAJAE

        await engine.register_fire("99:99", "event-1", time.time() + 1800)

        # Should have spawned a firefighter
        assert "99:99" in redis._sets.get(FIREFIGHTER_GRID_INDEX_KEY, set())
        spawn_events = [e for e in broadcaster.events if e[0] == "firefighter:spawn"]
        assert len(spawn_events) >= 1
