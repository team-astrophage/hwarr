"""Tests for heartbeat/ping-pong mechanism and connection lifecycle.

Covers:
- Heartbeat recording and staleness detection
- Stale connection reaper
- Reconnection with room restoration
- Engine.IO ping/pong config
- End-to-end heartbeat event handler
"""

import asyncio
import sys
import time
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest
import socketio

# Add server root to path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from sio.connection_manager import (
    HEARTBEAT_INTERVAL_SEC,
    HEARTBEAT_TIMEOUT_SEC,
    ConnectionInfo,
    ConnectionManager,
)


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
def sio_server():
    """Create an AsyncServer with mocked methods for testing."""
    server = socketio.AsyncServer(async_mode="asgi")
    server.enter_room = AsyncMock()
    server.leave_room = AsyncMock()
    server.emit = AsyncMock()
    server.disconnect = AsyncMock()
    return server


@pytest.fixture
def manager(sio_server):
    mgr = ConnectionManager(sio_server)
    yield mgr
    # Clean up reaper if running
    if mgr._reaper_task and not mgr._reaper_task.done():
        mgr._reaper_task.cancel()


# ---------------------------------------------------------------------------
# ConnectionInfo tests
# ---------------------------------------------------------------------------


class TestConnectionInfo:
    def test_last_heartbeat_defaults_to_connected_at(self):
        now = time.time()
        info = ConnectionInfo(sid="s1", connected_at=now)
        assert info.last_heartbeat == now

    def test_user_id_defaults_to_none(self):
        info = ConnectionInfo(sid="s1", connected_at=time.time())
        assert info.user_id is None

    def test_reconnect_count_defaults_to_zero(self):
        info = ConnectionInfo(sid="s1", connected_at=time.time())
        assert info.reconnect_count == 0


# ---------------------------------------------------------------------------
# Heartbeat recording tests
# ---------------------------------------------------------------------------


class TestHeartbeatRecording:
    @pytest.mark.asyncio
    async def test_record_heartbeat_updates_timestamp(self, manager):
        info = await manager.add("sid-1")
        original_ts = info.last_heartbeat

        # Simulate time passing
        await asyncio.sleep(0.01)
        result = manager.record_heartbeat("sid-1")

        assert result is True
        assert info.last_heartbeat > original_ts

    @pytest.mark.asyncio
    async def test_record_heartbeat_unknown_sid_returns_false(self, manager):
        result = manager.record_heartbeat("unknown")
        assert result is False


# ---------------------------------------------------------------------------
# Staleness detection tests
# ---------------------------------------------------------------------------


class TestStalenessDetection:
    @pytest.mark.asyncio
    async def test_fresh_connection_not_stale(self, manager):
        await manager.add("sid-1")
        assert manager.is_stale("sid-1") is False

    @pytest.mark.asyncio
    async def test_old_connection_is_stale(self, manager):
        info = await manager.add("sid-1")
        # Fake old heartbeat
        info.last_heartbeat = time.time() - HEARTBEAT_TIMEOUT_SEC - 1
        assert manager.is_stale("sid-1") is True

    @pytest.mark.asyncio
    async def test_unknown_sid_is_stale(self, manager):
        assert manager.is_stale("unknown") is True

    @pytest.mark.asyncio
    async def test_get_stale_sids(self, manager):
        await manager.add("fresh-1")
        info2 = await manager.add("stale-1")
        info3 = await manager.add("stale-2")
        info4 = await manager.add("fresh-2")

        now = time.time()
        # Make two connections stale
        info2.last_heartbeat = now - HEARTBEAT_TIMEOUT_SEC - 10
        info3.last_heartbeat = now - HEARTBEAT_TIMEOUT_SEC - 5

        stale = manager.get_stale_sids(now)
        assert set(stale) == {"stale-1", "stale-2"}

    @pytest.mark.asyncio
    async def test_heartbeat_resets_staleness(self, manager):
        info = await manager.add("sid-1")
        info.last_heartbeat = time.time() - HEARTBEAT_TIMEOUT_SEC - 1
        assert manager.is_stale("sid-1") is True

        manager.record_heartbeat("sid-1")
        assert manager.is_stale("sid-1") is False


# ---------------------------------------------------------------------------
# Reconnection tests
# ---------------------------------------------------------------------------


class TestReconnection:
    @pytest.mark.asyncio
    async def test_reconnect_increments_count(self, manager):
        info1 = await manager.add("sid-1", user_id="user-A")
        assert info1.reconnect_count == 0

        info2 = await manager.add("sid-2", user_id="user-A")
        assert info2.reconnect_count == 1

        info3 = await manager.add("sid-3", user_id="user-A")
        assert info3.reconnect_count == 2

    @pytest.mark.asyncio
    async def test_get_previous_session(self, manager):
        info1 = await manager.add("sid-1", user_id="user-A")
        info1.rooms = {"grid:1", "grid:2"}

        prev = manager.get_previous_session("user-A")
        assert prev is not None
        assert prev.rooms == {"grid:1", "grid:2"}

    @pytest.mark.asyncio
    async def test_get_previous_session_unknown_user(self, manager):
        assert manager.get_previous_session("unknown") is None

    @pytest.mark.asyncio
    async def test_restore_rooms(self, manager):
        await manager.add("sid-1")
        await manager.restore_rooms("sid-1", {"grid:1", "grid:2"})

        info = manager.get("sid-1")
        assert info is not None
        assert info.rooms == {"grid:1", "grid:2"}

    @pytest.mark.asyncio
    async def test_restore_empty_rooms(self, manager):
        await manager.add("sid-1")
        await manager.restore_rooms("sid-1", set())
        info = manager.get("sid-1")
        assert info is not None
        assert info.rooms == set()

    @pytest.mark.asyncio
    async def test_full_reconnect_flow(self, manager):
        """Simulate: connect → join rooms → disconnect → reconnect → rooms restored."""
        # First connection
        info1 = await manager.add("sid-1", user_id="user-B")
        await manager.join_rooms("sid-1", ["grid:A", "grid:B"])
        assert info1.rooms == {"grid:A", "grid:B"}

        # Disconnect (but user_sessions preserved)
        await manager.remove("sid-1")
        assert manager.get("sid-1") is None

        # Previous session still accessible
        prev = manager.get_previous_session("user-B")
        assert prev is not None
        prev_rooms = prev.rooms.copy()
        assert prev_rooms == {"grid:A", "grid:B"}

        # Reconnect with new sid
        info2 = await manager.add("sid-2", user_id="user-B")
        assert info2.reconnect_count == 1
        await manager.restore_rooms("sid-2", prev_rooms)
        assert info2.rooms == {"grid:A", "grid:B"}


# ---------------------------------------------------------------------------
# Reaper tests
# ---------------------------------------------------------------------------


class TestReaper:
    @pytest.mark.asyncio
    async def test_start_stop_reaper(self, manager):
        manager.start_reaper()
        assert manager._reaper_task is not None
        assert not manager._reaper_task.done()

        await manager.stop_reaper()
        assert manager._reaper_task is None

    @pytest.mark.asyncio
    async def test_start_reaper_idempotent(self, manager):
        manager.start_reaper()
        task1 = manager._reaper_task
        manager.start_reaper()
        assert manager._reaper_task is task1
        await manager.stop_reaper()

    @pytest.mark.asyncio
    async def test_stop_reaper_when_not_started(self, manager):
        """Should not raise."""
        await manager.stop_reaper()


# ---------------------------------------------------------------------------
# Heartbeat event handler tests (main.py)
# ---------------------------------------------------------------------------


class TestHeartbeatEventHandler:
    @pytest.fixture(autouse=True)
    def clean_manager(self):
        from main import manager as app_manager
        app_manager._connections.clear()
        app_manager._user_sessions.clear()
        yield
        app_manager._connections.clear()
        app_manager._user_sessions.clear()

    @pytest.mark.asyncio
    async def test_heartbeat_handler_returns_ack(self):
        from main import handle_heartbeat, manager as app_manager
        await app_manager.add("test-hb-sid")

        result = await handle_heartbeat("test-hb-sid", {"ts": 1000.0})
        assert result["status"] == "ok"
        assert "server_ts" in result
        assert result["client_ts"] == 1000.0

    @pytest.mark.asyncio
    async def test_heartbeat_handler_unknown_sid(self):
        from main import handle_heartbeat
        result = await handle_heartbeat("unknown-sid", {})
        assert result["error"] == "unknown_sid"

    @pytest.mark.asyncio
    async def test_heartbeat_handler_no_data(self):
        from main import handle_heartbeat, manager as app_manager
        await app_manager.add("test-hb-sid-2")

        result = await handle_heartbeat("test-hb-sid-2", None)
        assert result["status"] == "ok"
        assert result["client_ts"] is None


# ---------------------------------------------------------------------------
# Connect handler reconnection tests
# ---------------------------------------------------------------------------


class TestConnectHandlerReconnection:
    @pytest.fixture(autouse=True)
    def clean_manager(self):
        from main import manager as app_manager, sio as app_sio
        app_manager._connections.clear()
        app_manager._user_sessions.clear()
        self._orig_emit = app_sio.emit
        yield
        app_manager._connections.clear()
        app_manager._user_sessions.clear()

    @pytest.mark.asyncio
    async def test_connect_with_user_id(self):
        from main import connect, manager as app_manager, sio as app_sio
        with patch.object(app_sio, "emit", AsyncMock()):
            await connect("sid-A", {}, auth={"user_id": "user-1"})

        info = app_manager.get("sid-A")
        assert info is not None
        assert info.user_id == "user-1"
        assert info.reconnect_count == 0

    @pytest.mark.asyncio
    async def test_connect_reconnect_restores_rooms(self):
        from main import connect, disconnect, manager as app_manager, sio as app_sio

        with patch.object(app_sio, "emit", AsyncMock()):
            with patch.object(app_sio, "enter_room", AsyncMock()):
                # First connect
                await connect("sid-A", {}, auth={"user_id": "user-2"})
                await app_manager.join_rooms("sid-A", ["grid:X", "grid:Y"])

                # Disconnect
                await disconnect("sid-A")

                # Reconnect with new sid
                await connect("sid-B", {}, auth={"user_id": "user-2"})

        info = app_manager.get("sid-B")
        assert info is not None
        assert info.reconnect_count == 1
        assert info.rooms == {"grid:X", "grid:Y"}

    @pytest.mark.asyncio
    async def test_connect_sends_heartbeat_interval(self):
        from main import connect, sio as app_sio
        mock_emit = AsyncMock()

        with patch.object(app_sio, "emit", mock_emit):
            await connect("sid-C", {}, auth=None)

        call_args = mock_emit.call_args
        payload = call_args[0][1]
        assert "heartbeat_interval" in payload
        assert payload["heartbeat_interval"] == HEARTBEAT_INTERVAL_SEC


# ---------------------------------------------------------------------------
# Engine.IO configuration tests
# ---------------------------------------------------------------------------


class TestEngineIOConfig:
    def test_ping_interval_configured(self):
        from main import sio as app_sio
        # python-socketio stores engine.io config on the eio attribute
        eio = app_sio.eio
        assert eio.ping_interval == 10
        assert eio.ping_timeout == 5
