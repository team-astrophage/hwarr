"""Tests for ConnectionManager."""

import sys
from pathlib import Path
from unittest.mock import AsyncMock

import pytest
import socketio

# Add server root to path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from sio.connection_manager import ConnectionManager


@pytest.fixture
def sio():
    """Create an AsyncServer with mocked room methods for testing."""
    server = socketio.AsyncServer(async_mode="asgi")
    # Mock enter_room/leave_room so tests don't need real connections
    server.enter_room = AsyncMock()
    server.leave_room = AsyncMock()
    return server


@pytest.fixture
def manager(sio):
    return ConnectionManager(sio)


@pytest.mark.asyncio
async def test_add_connection(manager):
    info = await manager.add("sid-1")
    assert info.sid == "sid-1"
    assert manager.active_count == 1
    assert "sid-1" in manager.active_sids


@pytest.mark.asyncio
async def test_add_multiple_connections(manager):
    await manager.add("sid-1")
    await manager.add("sid-2")
    await manager.add("sid-3")
    assert manager.active_count == 3


@pytest.mark.asyncio
async def test_remove_connection(manager):
    await manager.add("sid-1")
    removed = await manager.remove("sid-1")
    assert removed is not None
    assert removed.sid == "sid-1"
    assert manager.active_count == 0


@pytest.mark.asyncio
async def test_remove_unknown_sid(manager):
    result = await manager.remove("unknown")
    assert result is None


@pytest.mark.asyncio
async def test_get_connection(manager):
    await manager.add("sid-1")
    info = manager.get("sid-1")
    assert info is not None
    assert info.sid == "sid-1"


@pytest.mark.asyncio
async def test_get_unknown_sid(manager):
    assert manager.get("unknown") is None


@pytest.mark.asyncio
async def test_join_rooms(manager, sio):
    await manager.add("sid-1")
    await manager.join_rooms("sid-1", ["grid:1", "grid:2"])
    info = manager.get("sid-1")
    assert info is not None
    assert info.rooms == {"grid:1", "grid:2"}
    assert sio.enter_room.call_count == 2


@pytest.mark.asyncio
async def test_join_rooms_replaces_old(manager, sio):
    """Joining new rooms should leave old rooms and join new ones."""
    await manager.add("sid-1")
    await manager.join_rooms("sid-1", ["grid:1", "grid:2"])
    await manager.join_rooms("sid-1", ["grid:3"])
    info = manager.get("sid-1")
    assert info is not None
    assert info.rooms == {"grid:3"}
    # leave_room called for grid:1 and grid:2
    assert sio.leave_room.call_count == 2


@pytest.mark.asyncio
async def test_join_rooms_unknown_sid(manager):
    """Should not raise for unknown sid."""
    await manager.join_rooms("unknown", ["grid:1"])


@pytest.mark.asyncio
async def test_broadcast(manager, sio):
    sio.emit = AsyncMock()
    await manager.add("sid-1")
    await manager.broadcast("fire:update", {"gridId": "1:2", "stage": 1})
    sio.emit.assert_called_once_with("fire:update", {"gridId": "1:2", "stage": 1})


@pytest.mark.asyncio
async def test_broadcast_to_room(manager, sio):
    sio.emit = AsyncMock()
    await manager.add("sid-1")
    await manager.broadcast_to_room(
        "fire:update", {"gridId": "1:2", "stage": 2}, room="1:2"
    )
    sio.emit.assert_called_once_with(
        "fire:update", {"gridId": "1:2", "stage": 2}, room="1:2"
    )


@pytest.mark.asyncio
async def test_send_to(manager, sio):
    sio.emit = AsyncMock()
    await manager.add("sid-1")
    await manager.send_to("fire:update", {"data": "test"}, sid="sid-1")
    sio.emit.assert_called_once_with("fire:update", {"data": "test"}, to="sid-1")


@pytest.mark.asyncio
async def test_connection_info_has_timestamp(manager):
    info = await manager.add("sid-1")
    assert info.connected_at > 0


@pytest.mark.asyncio
async def test_remove_cleans_up_rooms(manager, sio):
    """After remove, leave_room is called for each room."""
    await manager.add("sid-1")
    await manager.join_rooms("sid-1", ["grid:1", "grid:2"])
    sio.leave_room.reset_mock()  # reset from join_rooms (no old rooms to leave)

    removed = await manager.remove("sid-1")
    assert removed is not None
    assert removed.rooms == {"grid:1", "grid:2"}  # still in the returned object
    assert manager.get("sid-1") is None  # but removed from manager
    assert sio.leave_room.call_count == 2


@pytest.mark.asyncio
async def test_concurrent_add_remove(manager):
    """Simulate rapid add/remove cycles."""
    for i in range(10):
        await manager.add(f"sid-{i}")
    assert manager.active_count == 10

    for i in range(5):
        await manager.remove(f"sid-{i}")
    assert manager.active_count == 5
    assert "sid-5" in manager.active_sids
    assert "sid-0" not in manager.active_sids
