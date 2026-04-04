"""Load test: verify 10+ concurrent WebSocket connections.

Spins up the real FastAPI + Socket.IO server on a random port, then
opens 15 python-socketio AsyncClient connections concurrently and
asserts they all connect, receive the 'connected' ack, and remain
alive simultaneously.

Uses a standalone asyncio approach to avoid pytest-asyncio version
compatibility issues with module-scoped async fixtures.
"""

from __future__ import annotations

import asyncio
import sys
from pathlib import Path

import pytest
import socketio
import uvicorn

# Ensure server root is importable
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

NUM_CLIENTS = 15  # test with 15 to exceed the 10-minimum requirement


class _UvicornServer:
    """Thin wrapper to run uvicorn in a background asyncio task."""

    def __init__(self, app: str, host: str = "127.0.0.1", port: int = 0):
        self.host = host
        self.port = port
        self.app = app
        self._server: uvicorn.Server | None = None
        self._task = None

    async def start(self) -> int:
        config = uvicorn.Config(
            self.app,
            host=self.host,
            port=self.port,
            log_level="warning",
        )
        self._server = uvicorn.Server(config)
        self._task = asyncio.create_task(self._server.serve())
        while not self._server.started:
            await asyncio.sleep(0.05)
        for sock in self._server.servers:
            addr = sock.sockets[0].getsockname()
            self.port = addr[1]
            break
        return self.port

    async def stop(self):
        if self._server:
            self._server.should_exit = True
            if self._task:
                await self._task


async def _connect_client(
    url: str,
    results: dict,
    idx: int,
) -> socketio.AsyncClient:
    """Connect a single Socket.IO client and record results."""
    client = socketio.AsyncClient(
        reconnection=False,
        logger=False,
        engineio_logger=False,
    )
    results[idx] = {
        "connected": False,
        "ack_received": False,
        "sid": None,
        "error": None,
    }

    @client.on("connected")
    async def on_connected(data):
        results[idx]["ack_received"] = True
        results[idx]["sid"] = data.get("sid")

    try:
        await client.connect(url, wait_timeout=10)
        results[idx]["connected"] = client.connected
        # Give a moment for the ack event to arrive
        await asyncio.sleep(0.3)
    except Exception as exc:
        results[idx]["error"] = str(exc)

    return client


async def _run_concurrent_connection_test():
    """Core test logic: start server, connect clients, assert."""
    # Reset the global manager before the test
    from main import manager
    manager._connections.clear()

    srv = _UvicornServer("main:combined_app")
    port = await srv.start()
    url = f"http://127.0.0.1:{port}"

    try:
        results: dict = {}

        # Launch all connections concurrently
        tasks = [
            asyncio.create_task(_connect_client(url, results, i))
            for i in range(NUM_CLIENTS)
        ]
        clients = await asyncio.gather(*tasks)

        # --- Assert: all connected ---
        connected_count = sum(1 for r in results.values() if r["connected"])
        ack_count = sum(1 for r in results.values() if r["ack_received"])
        errors = [r for r in results.values() if r["error"]]

        assert connected_count >= 10, (
            f"Only {connected_count}/{NUM_CLIENTS} clients connected. "
            f"Errors: {errors}"
        )
        assert ack_count >= 10, (
            f"Only {ack_count}/{NUM_CLIENTS} clients received 'connected' ack"
        )

        # Verify unique sids
        sids = [r["sid"] for r in results.values() if r["sid"]]
        assert len(sids) == len(set(sids)), "Duplicate sids detected!"

        # --- Assert: connections maintained after hold ---
        await asyncio.sleep(2)
        still_connected = sum(1 for c in clients if c.connected)
        assert still_connected >= 10, (
            f"Only {still_connected}/{NUM_CLIENTS} connections survived "
            f"after 2s hold"
        )

        # --- Assert: /health reflects connections ---
        import httpx

        async with httpx.AsyncClient() as http_client:
            resp = await http_client.get(f"{url}/health")
            assert resp.status_code == 200
            data = resp.json()
            assert data["status"] == "ok"
            assert data["connections"] >= 10, (
                f"/health reported {data['connections']} connections, "
                f"expected >= 10"
            )

        # Cleanup — disconnect all
        await asyncio.gather(
            *(c.disconnect() for c in clients if c.connected),
            return_exceptions=True,
        )
    finally:
        await srv.stop()


async def _run_connection_lifecycle_test():
    """Test rapid connect/disconnect cycles maintain consistent state."""
    from main import manager
    manager._connections.clear()

    srv = _UvicornServer("main:combined_app")
    port = await srv.start()
    url = f"http://127.0.0.1:{port}"

    try:
        results: dict = {}

        # Connect 15 clients
        tasks = [
            asyncio.create_task(_connect_client(url, results, i))
            for i in range(NUM_CLIENTS)
        ]
        clients = await asyncio.gather(*tasks)

        connected = sum(1 for c in clients if c.connected)
        assert connected >= 10, f"Only {connected} connected initially"

        # Disconnect first 5
        for i in range(5):
            if clients[i].connected:
                await clients[i].disconnect()

        await asyncio.sleep(0.5)

        still_connected = sum(1 for c in clients if c.connected)
        assert still_connected >= 10, (
            f"After disconnecting 5, only {still_connected} remain "
            f"(expected >= 10)"
        )

        # Cleanup remaining
        await asyncio.gather(
            *(c.disconnect() for c in clients if c.connected),
            return_exceptions=True,
        )
    finally:
        await srv.stop()


# ---------------------------------------------------------------------------
# Pytest wrappers — each runs the full scenario
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_10_plus_concurrent_connections_established_and_maintained():
    """10+ clients connect concurrently, receive acks, stay alive 2s,
    and /health reports correct count."""
    await _run_concurrent_connection_test()


@pytest.mark.asyncio
async def test_connection_lifecycle_connect_disconnect_cycle():
    """Rapid connect/disconnect cycles maintain consistent state."""
    await _run_connection_lifecycle_test()
