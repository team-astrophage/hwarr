"""Tests for the WebSocket (/ws) endpoint — connect/disconnect lifecycle.

Tests Socket.IO connect/disconnect event handlers and their integration
with ConnectionManager.
"""

import sys
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest

# Add server root to path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from main import app, manager, sio, sio_asgi_app
from sio.connection_manager import ConnectionManager


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(autouse=True)
def clean_manager():
    """Ensure ConnectionManager is clean before/after each test."""
    # Sync clear — directly manipulate the dict (test-only)
    manager._connections.clear()
    yield
    manager._connections.clear()


# ---------------------------------------------------------------------------
# Unit tests — event handler logic (mocked transport)
# ---------------------------------------------------------------------------


class TestConnectHandler:
    """Test the connect event handler registers connections."""

    @pytest.mark.asyncio
    async def test_connect_registers_with_manager(self):
        """connect event should add the sid to ConnectionManager."""
        mock_environ = {"REMOTE_ADDR": "127.0.0.1"}
        mock_emit = AsyncMock()

        with patch.object(sio, "emit", mock_emit):
            from main import connect
            await connect("test-sid-1", mock_environ, auth=None)

        assert manager.active_count >= 1
        info = manager.get("test-sid-1")
        assert info is not None
        assert info.sid == "test-sid-1"

    @pytest.mark.asyncio
    async def test_connect_sends_ack(self):
        """connect event should emit a 'connected' ack to the client."""
        mock_environ = {"REMOTE_ADDR": "127.0.0.1"}
        mock_emit = AsyncMock()

        with patch.object(sio, "emit", mock_emit):
            from main import connect
            await connect("test-sid-2", mock_environ, auth=None)

        # Check that 'connected' event was emitted to the sid
        mock_emit.assert_called_once()
        call_args = mock_emit.call_args
        assert call_args[0][0] == "connected"  # event name
        payload = call_args[0][1]
        assert payload["sid"] == "test-sid-2"
        assert "connected_at" in payload
        assert "active_connections" in payload
        assert call_args[1]["to"] == "test-sid-2"


class TestDisconnectHandler:
    """Test the disconnect event handler cleans up connections."""

    @pytest.mark.asyncio
    async def test_disconnect_removes_from_manager(self):
        """disconnect event should remove the sid from ConnectionManager."""
        await manager.add("test-sid-3")
        assert manager.get("test-sid-3") is not None

        from main import disconnect
        await disconnect("test-sid-3")

        assert manager.get("test-sid-3") is None
        assert manager.active_count == 0

    @pytest.mark.asyncio
    async def test_disconnect_unknown_sid_no_crash(self):
        """disconnect for unknown sid should not raise."""
        from main import disconnect
        await disconnect("never-connected-sid")  # should not raise


class TestDisconnectCleansRooms:
    """Test that disconnect cleans up room subscriptions."""

    @pytest.mark.asyncio
    async def test_disconnect_leaves_rooms(self):
        """When a client disconnects, all its room memberships are cleaned up."""
        await manager.add("test-sid-4")

        with patch.object(sio, "enter_room", AsyncMock()):
            await manager.join_rooms("test-sid-4", ["grid:1", "grid:2"])

        info = manager.get("test-sid-4")
        assert info is not None
        assert len(info.rooms) == 2

        with patch.object(sio, "leave_room", AsyncMock()) as mock_leave:
            from main import disconnect
            await disconnect("test-sid-4")
            assert mock_leave.call_count == 2

        assert manager.get("test-sid-4") is None


class TestMultipleConnections:
    """Test handling multiple simultaneous connections."""

    @pytest.mark.asyncio
    async def test_multiple_connects(self):
        """Multiple clients connecting should all be tracked."""
        mock_emit = AsyncMock()
        mock_environ = {}

        with patch.object(sio, "emit", mock_emit):
            from main import connect
            for i in range(10):
                await connect(f"sid-{i}", mock_environ, auth=None)

        assert manager.active_count == 10
        for i in range(10):
            assert manager.get(f"sid-{i}") is not None

    @pytest.mark.asyncio
    async def test_connect_disconnect_cycle(self):
        """Rapid connect/disconnect cycles should maintain consistent state."""
        mock_emit = AsyncMock()
        mock_environ = {}

        with patch.object(sio, "emit", mock_emit):
            from main import connect, disconnect

            # Connect 10 clients
            for i in range(10):
                await connect(f"cycle-sid-{i}", mock_environ, auth=None)

            assert manager.active_count == 10

            # Disconnect first 5
            for i in range(5):
                await disconnect(f"cycle-sid-{i}")

            assert manager.active_count == 5

            for i in range(5, 10):
                assert manager.get(f"cycle-sid-{i}") is not None
            for i in range(5):
                assert manager.get(f"cycle-sid-{i}") is None


class TestHealthEndpoint:
    """Test the /health endpoint reports connection count."""

    @pytest.mark.asyncio
    async def test_health_returns_ok(self):
        from main import health_check
        result = await health_check()
        assert result["status"] == "ok"
        assert "connections" in result

    @pytest.mark.asyncio
    async def test_health_reflects_connection_count(self):
        await manager.add("health-sid-1")
        await manager.add("health-sid-2")

        from main import health_check
        result = await health_check()
        assert result["connections"] == 2
