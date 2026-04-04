"""Tests for state-change event broadcasting via Socket.IO.

Verifies that fire ignition, stage escalation, and firefighter actions
publish updates to all connected clients via the connection manager.

Tests cover:
1. fire:ignite — broadcasts ignition event to grid room + global
2. subscribe:viewport — joins rooms and returns current state
3. fire:state — returns current fire state on request
4. Stage transitions trigger fire:update broadcasts
5. Firefighter actions trigger firefighter:spawn/alert/retire broadcasts
6. Multiple clients receive broadcasts simultaneously
"""

from __future__ import annotations

import sys
import time
from pathlib import Path
from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

# Add server root to path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from grid import to_grid_id
from jobs.fire_progression import (
    ACTIVE_GRIDS_KEY,
    FIRE_KEY_PREFIX,
    FireProgressionEngine,
)
from models.fire import FireStage, build_grid_state, get_stage
from sio.connection_manager import ConnectionManager, ConnectionInfo
from sio.events import register_fire_events


# ---------------------------------------------------------------------------
# Shared test fakes
# ---------------------------------------------------------------------------


class FakeRedis:
    """In-memory fake Redis for testing."""

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


class RecordingBroadcaster:
    """Wraps ConnectionManager to record all broadcast calls for assertion."""

    def __init__(self) -> None:
        self.events: list[tuple[str, Any, str | None]] = []

    async def broadcast_to_room(self, event: str, data: Any, room: str) -> None:
        self.events.append((event, data, room))

    async def broadcast(self, event: str, data: Any) -> None:
        self.events.append((event, data, None))

    async def send_to(self, event: str, data: Any, sid: str) -> None:
        self.events.append((event, data, f"sid:{sid}"))

    def get_events(self, event_name: str) -> list[tuple[str, Any, str | None]]:
        return [e for e in self.events if e[0] == event_name]

    def clear(self) -> None:
        self.events.clear()


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
def redis():
    return FakeRedis()


@pytest.fixture
def fake_sio():
    """Create a mock Socket.IO server that records registered event handlers."""
    sio = MagicMock()
    sio._handlers = {}

    def on_decorator(event_name):
        def decorator(handler):
            sio._handlers[event_name] = handler
            return handler
        return decorator

    sio.on = on_decorator
    sio.emit = AsyncMock()
    sio.enter_room = AsyncMock()
    sio.leave_room = AsyncMock()
    return sio


@pytest.fixture
def manager_with_sio(fake_sio):
    """ConnectionManager backed by the fake sio."""
    return ConnectionManager(fake_sio)


@pytest.fixture
def recording_broadcaster():
    return RecordingBroadcaster()


@pytest.fixture
def engine(redis, recording_broadcaster):
    return FireProgressionEngine(
        redis=redis,
        broadcaster=recording_broadcaster,
        scan_interval=0.1,
        firefighter_interval=0.2,
        cleanup_interval=0.3,
    )


@pytest.fixture
def handlers(fake_sio, manager_with_sio, engine):
    """Register event handlers and return them for direct invocation."""
    register_fire_events(fake_sio, manager_with_sio, engine)
    return fake_sio._handlers


# ===========================================================================
# 1. fire:ignite event handler
# ===========================================================================


class TestFireIgniteHandler:
    """Test fire:ignite Socket.IO event handler."""

    @pytest.mark.asyncio
    async def test_ignite_returns_ack(self, handlers, engine):
        """fire:ignite should return an ack with grid state."""
        handler = handlers["fire:ignite"]
        result = await handler("client-1", {"lat": 37.5665, "lng": 126.9780})

        assert result["status"] == "ok"
        assert "grid_id" in result
        assert "event_id" in result
        assert result["active_count"] == 1
        assert result["stage"] == 1  # BULSSSI

    @pytest.mark.asyncio
    async def test_ignite_missing_lat(self, handlers):
        """fire:ignite with missing lat should return error."""
        handler = handlers["fire:ignite"]
        result = await handler("client-1", {"lng": 126.0})

        assert "error" in result

    @pytest.mark.asyncio
    async def test_ignite_missing_lng(self, handlers):
        """fire:ignite with missing lng should return error."""
        handler = handlers["fire:ignite"]
        result = await handler("client-1", {"lat": 37.0})

        assert "error" in result

    @pytest.mark.asyncio
    async def test_ignite_invalid_coords(self, handlers):
        """fire:ignite with non-numeric coords should return error."""
        handler = handlers["fire:ignite"]
        result = await handler("client-1", {"lat": "abc", "lng": "def"})

        assert "error" in result

    @pytest.mark.asyncio
    async def test_ignite_registers_fire_in_redis(self, handlers, engine, redis):
        """fire:ignite should register the fire event in Redis."""
        handler = handlers["fire:ignite"]
        result = await handler("client-1", {"lat": 37.5665, "lng": 126.9780})

        grid_id = result["grid_id"]
        key = f"{FIRE_KEY_PREFIX}{grid_id}"
        count = await redis.zcard(key)
        assert count == 1

    @pytest.mark.asyncio
    async def test_ignite_broadcasts_to_grid_room(
        self, handlers, engine, recording_broadcaster
    ):
        """fire:ignite should broadcast fire:update to the grid room and globally."""
        handler = handlers["fire:ignite"]
        result = await handler("client-1", {"lat": 37.5665, "lng": 126.9780})

        grid_id = result["grid_id"]

        # The engine broadcasts fire:update on stage change (room + global)
        fire_updates = recording_broadcaster.get_events("fire:update")
        assert len(fire_updates) >= 2  # room + global
        assert fire_updates[0][2] == grid_id  # room = grid_id
        assert fire_updates[1][2] is None     # global broadcast

    @pytest.mark.asyncio
    async def test_ignite_broadcasts_global_update(
        self, handlers, fake_sio, engine
    ):
        """fire:ignite should broadcast fire:global_update to all clients."""
        handler = handlers["fire:ignite"]
        await handler("client-1", {"lat": 37.5665, "lng": 126.9780})

        # Check that sio.emit was called for fire:global_update (broadcast)
        # The manager.broadcast calls sio.emit without room
        emit_calls = fake_sio.emit.call_args_list
        global_updates = [c for c in emit_calls if c[0][0] == "fire:global_update"]
        assert len(global_updates) >= 1

    @pytest.mark.asyncio
    async def test_multiple_ignitions_same_grid(self, handlers, engine):
        """Multiple ignitions in the same grid should accumulate count."""
        handler = handlers["fire:ignite"]

        for i in range(5):
            result = await handler(f"client-{i}", {"lat": 37.5665, "lng": 126.9780})

        assert result["active_count"] == 5
        assert result["stage"] == 2  # MODAKBUL (5-14)

    @pytest.mark.asyncio
    async def test_ignite_triggers_stage_change_broadcast(
        self, handlers, engine, recording_broadcaster
    ):
        """Crossing a stage threshold should broadcast fire:update (room + global)."""
        handler = handlers["fire:ignite"]

        # Add 4 fires (stage 1: BULSSSI)
        for i in range(4):
            await handler(f"client-{i}", {"lat": 37.5665, "lng": 126.9780})

        recording_broadcaster.clear()

        # 5th fire crosses to stage 2 (MODAKBUL)
        result = await handler("client-5", {"lat": 37.5665, "lng": 126.9780})

        fire_updates = recording_broadcaster.get_events("fire:update")
        assert len(fire_updates) >= 2  # room + global
        assert fire_updates[0][1]["stage"] == 2  # MODAKBUL

    @pytest.mark.asyncio
    async def test_ignite_triggers_firefighter_at_stage_4(
        self, handlers, engine, recording_broadcaster
    ):
        """Reaching stage 4 (30 fires) should trigger firefighter:spawn."""
        handler = handlers["fire:ignite"]

        # Add 29 fires
        for i in range(29):
            await handler(f"client-{i}", {"lat": 37.5665, "lng": 126.9780})

        recording_broadcaster.clear()

        # 30th fire crosses to stage 4 → firefighter spawns
        await handler("client-30", {"lat": 37.5665, "lng": 126.9780})

        spawn_events = recording_broadcaster.get_events("firefighter:spawn")
        assert len(spawn_events) >= 1
        assert spawn_events[0][1]["status"] == "dispatched"


# ===========================================================================
# 2. subscribe:viewport event handler
# ===========================================================================


class TestSubscribeViewportHandler:
    """Test subscribe:viewport Socket.IO event handler."""

    @pytest.mark.asyncio
    async def test_subscribe_returns_ack(self, handlers):
        """subscribe:viewport should return ack with grid count."""
        handler = handlers["subscribe:viewport"]
        result = await handler("client-1", {
            "ne_lat": 37.57,
            "ne_lng": 126.98,
            "sw_lat": 37.56,
            "sw_lng": 126.97,
        })

        assert result["status"] == "ok"
        assert "subscribed_grids" in result
        assert result["subscribed_grids"] > 0

    @pytest.mark.asyncio
    async def test_subscribe_missing_fields(self, handlers):
        """subscribe:viewport with missing fields should return error."""
        handler = handlers["subscribe:viewport"]
        result = await handler("client-1", {"ne_lat": 37.57})

        assert "error" in result

    @pytest.mark.asyncio
    async def test_subscribe_joins_rooms(self, handlers, fake_sio, manager_with_sio):
        """subscribe:viewport should join the client to grid rooms."""
        # Add client to manager first
        await manager_with_sio.add("client-1")

        handler = handlers["subscribe:viewport"]
        result = await handler("client-1", {
            "ne_lat": 37.57,
            "ne_lng": 126.98,
            "sw_lat": 37.56,
            "sw_lng": 126.97,
        })

        # Check that enter_room was called
        assert fake_sio.enter_room.call_count > 0

    @pytest.mark.asyncio
    async def test_subscribe_returns_active_fires(
        self, handlers, engine, recording_broadcaster
    ):
        """subscribe:viewport should return current fire states in viewport."""
        # First ignite a fire within the viewport
        ignite_handler = handlers["fire:ignite"]
        await ignite_handler("igniter", {"lat": 37.565, "lng": 126.975})

        # Now subscribe to a viewport covering that fire
        handler = handlers["subscribe:viewport"]
        result = await handler("viewer", {
            "ne_lat": 37.57,
            "ne_lng": 126.98,
            "sw_lat": 37.56,
            "sw_lng": 126.97,
        })

        assert result["status"] == "ok"
        assert len(result["active_fires"]) >= 1
        assert result["active_fires"][0]["active_count"] > 0

    @pytest.mark.asyncio
    async def test_subscribe_empty_viewport(self, handlers):
        """subscribe:viewport with no fires should return empty active list."""
        handler = handlers["subscribe:viewport"]
        result = await handler("client-1", {
            "ne_lat": 10.0,
            "ne_lng": 10.0,
            "sw_lat": 9.0,
            "sw_lng": 9.0,
        })

        assert result["status"] == "ok"
        assert result["active_fires"] == []


# ===========================================================================
# 3. fire:state event handler
# ===========================================================================


class TestFireStateHandler:
    """Test fire:state Socket.IO event handler."""

    @pytest.mark.asyncio
    async def test_state_returns_all_active(self, handlers, engine):
        """fire:state without grid_ids should return all active grids."""
        ignite = handlers["fire:ignite"]
        await ignite("c1", {"lat": 37.5665, "lng": 126.9780})
        await ignite("c2", {"lat": 38.0, "lng": 127.0})

        handler = handlers["fire:state"]
        result = await handler("viewer", {})

        assert result["status"] == "ok"
        assert result["total_active_grids"] == 2

    @pytest.mark.asyncio
    async def test_state_with_specific_grids(self, handlers, engine):
        """fire:state with grid_ids should return only those grids."""
        ignite = handlers["fire:ignite"]
        r1 = await ignite("c1", {"lat": 37.5665, "lng": 126.9780})
        grid_id = r1["grid_id"]

        handler = handlers["fire:state"]
        result = await handler("viewer", {"grid_ids": [grid_id]})

        assert result["status"] == "ok"
        assert result["total_active_grids"] == 1
        assert result["grids"][0]["grid_id"] == grid_id

    @pytest.mark.asyncio
    async def test_state_empty_when_no_fires(self, handlers):
        """fire:state should return empty when no fires exist."""
        handler = handlers["fire:state"]
        result = await handler("viewer", {})

        assert result["status"] == "ok"
        assert result["total_active_grids"] == 0
        assert result["grids"] == []


# ===========================================================================
# 4. End-to-end broadcast flow
# ===========================================================================


class TestBroadcastFlow:
    """Test end-to-end state-change broadcasting."""

    @pytest.mark.asyncio
    async def test_ignition_broadcasts_stage_change(
        self, handlers, engine, recording_broadcaster
    ):
        """Full flow: ignite → stage change → fire:update broadcast (room + global)."""
        ignite = handlers["fire:ignite"]

        # First fire: NONE → BULSSSI
        await ignite("c1", {"lat": 37.5665, "lng": 126.9780})

        updates = recording_broadcaster.get_events("fire:update")
        # Should have at least 2: one room-scoped, one global
        assert len(updates) >= 2
        assert updates[0][1]["stage"] == 1  # BULSSSI (room)
        assert updates[1][1]["stage"] == 1  # BULSSSI (global)

    @pytest.mark.asyncio
    async def test_stage_escalation_broadcasts_sequence(
        self, handlers, engine, recording_broadcaster
    ):
        """Escalating through stages should produce broadcasts at each boundary."""
        ignite = handlers["fire:ignite"]

        stages_broadcast = []
        for i in range(1, 16):
            recording_broadcaster.clear()
            await ignite(f"c-{i}", {"lat": 37.5665, "lng": 126.9780})
            updates = recording_broadcaster.get_events("fire:update")
            if updates:
                stages_broadcast.append(updates[0][1]["stage"])

        # Should see stage 1 (at count 1), stage 2 (at count 5), stage 3 (at count 15)
        assert 1 in stages_broadcast
        assert 2 in stages_broadcast
        assert 3 in stages_broadcast

    @pytest.mark.asyncio
    async def test_stage_transition_events_carry_prev_and_new(
        self, handlers, engine, recording_broadcaster
    ):
        """fire:stage_transition should carry prev_stage and new_stage."""
        ignite = handlers["fire:ignite"]

        # First fire: NONE(0) → BULSSSI(1)
        await ignite("c1", {"lat": 37.5665, "lng": 126.9780})

        transitions = recording_broadcaster.get_events("fire:stage_transition")
        assert len(transitions) >= 1
        t = transitions[0][1]
        assert t["prev_stage"] == 0  # NONE
        assert t["new_stage"] == 1   # BULSSSI
        assert "timestamp" in t

    @pytest.mark.asyncio
    async def test_stage_transition_on_escalation(
        self, handlers, engine, recording_broadcaster
    ):
        """Crossing from BULSSSI to MODAKBUL emits correct transition."""
        ignite = handlers["fire:ignite"]

        # 4 fires → stage 1 (BULSSSI)
        for i in range(4):
            await ignite(f"c-{i}", {"lat": 37.5665, "lng": 126.9780})

        recording_broadcaster.clear()

        # 5th fire → stage 2 (MODAKBUL)
        await ignite("c-5", {"lat": 37.5665, "lng": 126.9780})

        transitions = recording_broadcaster.get_events("fire:stage_transition")
        assert len(transitions) >= 1
        t = transitions[0][1]
        assert t["prev_stage"] == 1  # BULSSSI
        assert t["new_stage"] == 2   # MODAKBUL

    @pytest.mark.asyncio
    async def test_firefighter_lifecycle_broadcasts(
        self, handlers, engine, recording_broadcaster
    ):
        """Full firefighter lifecycle should produce spawn, alert, and retire broadcasts."""
        ignite = handlers["fire:ignite"]

        # Escalate to stage 4 (30 fires)
        for i in range(30):
            await ignite(f"c-{i}", {"lat": 37.5665, "lng": 126.9780})

        # Should have firefighter:spawn
        spawn_events = recording_broadcaster.get_events("firefighter:spawn")
        assert len(spawn_events) >= 1

        recording_broadcaster.clear()

        # Run a firefighter sweep — should produce firefighter:alert
        await engine._run_firefighter_sweep()

        alerts = recording_broadcaster.get_events("firefighter:alert")
        assert len(alerts) >= 1
        assert alerts[0][1]["removed_count"] > 0

    @pytest.mark.asyncio
    async def test_firefighter_alert_broadcasts_globally(
        self, handlers, engine, recording_broadcaster
    ):
        """Firefighter alert should broadcast to room AND globally."""
        ignite = handlers["fire:ignite"]

        # Escalate to stage 4 (30 fires)
        for i in range(30):
            await ignite(f"c-{i}", {"lat": 37.5665, "lng": 126.9780})

        recording_broadcaster.clear()

        # Run firefighter sweep
        await engine._run_firefighter_sweep()

        alerts = recording_broadcaster.get_events("firefighter:alert")
        # Should have at least 2: one room-scoped, one global
        assert len(alerts) >= 2
        room_alerts = [a for a in alerts if a[2] is not None]
        global_alerts = [a for a in alerts if a[2] is None]
        assert len(room_alerts) >= 1
        assert len(global_alerts) >= 1

    @pytest.mark.asyncio
    async def test_no_duplicate_broadcasts_same_stage(
        self, handlers, engine, recording_broadcaster
    ):
        """Adding fires within the same stage should NOT broadcast fire:update."""
        ignite = handlers["fire:ignite"]

        # First fire: triggers broadcast (NONE → BULSSSI)
        await ignite("c1", {"lat": 37.5665, "lng": 126.9780})
        recording_broadcaster.clear()

        # 2nd, 3rd, 4th fires: same stage, no broadcast
        for i in range(2, 5):
            await ignite(f"c{i}", {"lat": 37.5665, "lng": 126.9780})

        updates = recording_broadcaster.get_events("fire:update")
        assert len(updates) == 0, "No fire:update expected within same stage"

    @pytest.mark.asyncio
    async def test_global_update_on_every_ignition(
        self, handlers, fake_sio, engine
    ):
        """fire:global_update should be broadcast on EVERY ignition (not just stage changes)."""
        ignite = handlers["fire:ignite"]

        await ignite("c1", {"lat": 37.5665, "lng": 126.9780})
        await ignite("c2", {"lat": 37.5665, "lng": 126.9780})
        await ignite("c3", {"lat": 37.5665, "lng": 126.9780})

        emit_calls = fake_sio.emit.call_args_list
        global_updates = [c for c in emit_calls if c[0][0] == "fire:global_update"]
        assert len(global_updates) == 3, "Each ignition should broadcast fire:global_update"


# ===========================================================================
# 5. Multiple simultaneous clients
# ===========================================================================


class TestMultiClientBroadcasting:
    """Test that broadcasts reach multiple connected clients."""

    @pytest.mark.asyncio
    async def test_10_clients_all_receive_broadcasts(
        self, handlers, fake_sio, manager_with_sio, engine
    ):
        """10 connected clients should all be in the broadcast scope."""
        # Connect 10 clients
        for i in range(10):
            await manager_with_sio.add(f"client-{i}")

        assert manager_with_sio.active_count == 10

        # When fire:update is broadcast, sio.emit is called without room-filtering
        # for global broadcasts, so all 10 clients receive it
        ignite = handlers["fire:ignite"]
        await ignite("client-0", {"lat": 37.5665, "lng": 126.9780})

        # Check that global broadcast was sent
        emit_calls = fake_sio.emit.call_args_list
        global_updates = [c for c in emit_calls if c[0][0] == "fire:global_update"]
        assert len(global_updates) >= 1

    @pytest.mark.asyncio
    async def test_room_subscribers_receive_grid_updates(
        self, handlers, fake_sio, manager_with_sio, engine
    ):
        """Clients subscribed to a viewport receive grid-specific updates."""
        # Connect and subscribe two clients to the same viewport
        await manager_with_sio.add("viewer-1")
        await manager_with_sio.add("viewer-2")

        subscribe = handlers["subscribe:viewport"]
        await subscribe("viewer-1", {
            "ne_lat": 37.57, "ne_lng": 126.98,
            "sw_lat": 37.56, "sw_lng": 126.97,
        })
        await subscribe("viewer-2", {
            "ne_lat": 37.57, "ne_lng": 126.98,
            "sw_lat": 37.56, "sw_lng": 126.97,
        })

        # Both viewers should have joined the same rooms
        info1 = manager_with_sio.get("viewer-1")
        info2 = manager_with_sio.get("viewer-2")
        assert info1 is not None
        assert info2 is not None
        assert info1.rooms == info2.rooms
        assert len(info1.rooms) > 0


# ===========================================================================
# 6. Event handler registration
# ===========================================================================


class TestEventRegistration:
    """Test that register_fire_events properly registers handlers."""

    def test_all_handlers_registered(self, handlers):
        """All expected event handlers should be registered."""
        assert "fire:ignite" in handlers
        assert "subscribe:viewport" in handlers
        assert "fire:state" in handlers

    def test_handler_count(self, handlers):
        """Should register exactly 3 event handlers."""
        assert len(handlers) == 3
