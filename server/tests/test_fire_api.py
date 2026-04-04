"""Tests for the fire ignition REST API (POST /api/fire, GET /api/grid/*).

Tests the fire API endpoints using mocked Redis/engine to verify:
- Fire ignition creates events and returns correct grid state
- Grid state queries return accurate active counts
- Viewport queries return only active grids
- Error handling for missing/invalid inputs
- Broadcasting to Socket.IO clients on ignition
"""

import sys
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import pytest_asyncio
from httpx import ASGITransport, AsyncClient

# Add server root to path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

import main as main_module
from main import app


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


class MockEngine:
    """Minimal mock of FireProgressionEngine for API tests."""

    def __init__(self):
        self._fire_counts: dict[str, int] = {}
        self.register_fire = AsyncMock(side_effect=self._register)
        self._get_active_count = AsyncMock(side_effect=self._active_count)
        self._get_active_grid_ids = AsyncMock(return_value=[])
        self.is_running = True

    async def _register(self, grid_id: str, event_id: str, expire_at: float) -> int:
        self._fire_counts[grid_id] = self._fire_counts.get(grid_id, 0) + 1
        return self._fire_counts[grid_id]

    async def _active_count(self, grid_id: str, now: float) -> int:
        return self._fire_counts.get(grid_id, 0)


@pytest.fixture
def mock_engine():
    """Provide a mock engine and patch it into main module."""
    engine = MockEngine()
    original = main_module.engine
    main_module.engine = engine
    yield engine
    main_module.engine = original


@pytest.fixture
def mock_manager():
    """Patch the manager's broadcast methods."""
    mgr = main_module.manager
    original_broadcast = mgr.broadcast
    original_broadcast_to_room = mgr.broadcast_to_room
    mgr.broadcast = AsyncMock()
    mgr.broadcast_to_room = AsyncMock()
    yield mgr
    mgr.broadcast = original_broadcast
    mgr.broadcast_to_room = original_broadcast_to_room


@pytest_asyncio.fixture
async def client():
    """Async HTTP client for testing FastAPI endpoints."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as ac:
        yield ac


# ---------------------------------------------------------------------------
# POST /api/fire — Fire ignition
# ---------------------------------------------------------------------------


class TestFireIgniteEndpoint:
    """Tests for POST /api/fire."""

    @pytest.mark.asyncio
    async def test_ignite_success(self, client, mock_engine, mock_manager):
        """POST /api/fire with valid coords returns 201 with grid state."""
        response = await client.post(
            "/api/fire",
            json={"lat": 37.5760, "lng": 126.9769},
        )
        assert response.status_code == 201
        data = response.json()
        assert "grid_id" in data
        assert "event_id" in data
        assert data["active_count"] == 1
        assert data["stage"] == 1  # 불씨 (1 fire)
        assert "stage_info" in data

    @pytest.mark.asyncio
    async def test_ignite_increments_count(self, client, mock_engine, mock_manager):
        """Multiple ignitions at same location increment the count."""
        coords = {"lat": 37.5760, "lng": 126.9769}

        r1 = await client.post("/api/fire", json=coords)
        assert r1.json()["active_count"] == 1

        r2 = await client.post("/api/fire", json=coords)
        assert r2.json()["active_count"] == 2

        r3 = await client.post("/api/fire", json=coords)
        assert r3.json()["active_count"] == 3

    @pytest.mark.asyncio
    async def test_ignite_broadcasts_to_socketio(self, client, mock_engine, mock_manager):
        """Fire ignition should broadcast events to Socket.IO clients."""
        await client.post(
            "/api/fire",
            json={"lat": 37.5760, "lng": 126.9769},
        )

        # Should broadcast fire:ignite to grid room
        mock_manager.broadcast_to_room.assert_called_once()
        call_args = mock_manager.broadcast_to_room.call_args
        assert call_args[0][0] == "fire:ignite"

        # Should broadcast fire:global_update globally
        mock_manager.broadcast.assert_called_once()
        global_call = mock_manager.broadcast.call_args
        assert global_call[0][0] == "fire:global_update"

    @pytest.mark.asyncio
    async def test_ignite_returns_event_id(self, client, mock_engine, mock_manager):
        """Each ignition returns a unique event_id."""
        r1 = await client.post("/api/fire", json={"lat": 37.0, "lng": 127.0})
        r2 = await client.post("/api/fire", json={"lat": 37.0, "lng": 127.0})
        assert r1.json()["event_id"] != r2.json()["event_id"]

    @pytest.mark.asyncio
    async def test_ignite_missing_lat(self, client, mock_engine, mock_manager):
        """POST /api/fire without lat returns 422."""
        response = await client.post("/api/fire", json={"lng": 126.9769})
        assert response.status_code == 422

    @pytest.mark.asyncio
    async def test_ignite_missing_lng(self, client, mock_engine, mock_manager):
        """POST /api/fire without lng returns 422."""
        response = await client.post("/api/fire", json={"lat": 37.5760})
        assert response.status_code == 422

    @pytest.mark.asyncio
    async def test_ignite_invalid_body(self, client, mock_engine, mock_manager):
        """POST /api/fire with non-numeric coords returns 422."""
        response = await client.post(
            "/api/fire",
            json={"lat": "not-a-number", "lng": 126.9769},
        )
        assert response.status_code == 422

    @pytest.mark.asyncio
    async def test_ignite_no_engine_returns_503(self, client, mock_manager):
        """POST /api/fire with no engine (Redis down) returns 503."""
        original = main_module.engine
        main_module.engine = None
        try:
            response = await client.post(
                "/api/fire",
                json={"lat": 37.0, "lng": 127.0},
            )
            assert response.status_code == 503
        finally:
            main_module.engine = original

    @pytest.mark.asyncio
    async def test_ignite_grid_id_consistency(self, client, mock_engine, mock_manager):
        """Same GPS coordinates should always produce the same grid_id."""
        coords = {"lat": 35.1587, "lng": 129.1604}
        r1 = await client.post("/api/fire", json=coords)
        r2 = await client.post("/api/fire", json=coords)
        assert r1.json()["grid_id"] == r2.json()["grid_id"]


# ---------------------------------------------------------------------------
# GET /api/grid/{grid_id} — Single grid state
# ---------------------------------------------------------------------------


class TestGridStateEndpoint:
    """Tests for GET /api/grid/{grid_id}."""

    @pytest.mark.asyncio
    async def test_get_grid_empty(self, client, mock_engine):
        """GET /api/grid/{id} for empty grid returns stage 0."""
        response = await client.get("/api/grid/12345:67890")
        assert response.status_code == 200
        data = response.json()
        assert data["active_count"] == 0
        assert data["stage"] == 0

    @pytest.mark.asyncio
    async def test_get_grid_with_fires(self, client, mock_engine, mock_manager):
        """GET /api/grid/{id} after ignition returns correct count."""
        # Ignite a fire first
        r = await client.post(
            "/api/fire",
            json={"lat": 37.5760, "lng": 126.9769},
        )
        grid_id = r.json()["grid_id"]

        # Query the grid state
        response = await client.get(f"/api/grid/{grid_id}")
        assert response.status_code == 200
        data = response.json()
        assert data["active_count"] == 1
        assert data["stage"] == 1

    @pytest.mark.asyncio
    async def test_get_grid_no_engine_returns_503(self, client):
        """GET /api/grid/{id} with no engine returns 503."""
        original = main_module.engine
        main_module.engine = None
        try:
            response = await client.get("/api/grid/123:456")
            assert response.status_code == 503
        finally:
            main_module.engine = original


# ---------------------------------------------------------------------------
# GET /api/grid/viewport — Viewport bulk query
# ---------------------------------------------------------------------------


class TestViewportEndpoint:
    """Tests for GET /api/grid/viewport."""

    @pytest.mark.asyncio
    async def test_viewport_empty(self, client, mock_engine):
        """GET /api/grid/viewport with no fires returns empty list."""
        response = await client.get(
            "/api/grid/viewport",
            params={
                "ne_lat": 37.6, "ne_lng": 127.1,
                "sw_lat": 37.5, "sw_lng": 126.9,
            },
        )
        assert response.status_code == 200
        data = response.json()
        assert data["grids"] == []
        assert data["total_active_grids"] == 0

    @pytest.mark.asyncio
    async def test_viewport_missing_params(self, client, mock_engine):
        """GET /api/grid/viewport without required params returns 422."""
        response = await client.get(
            "/api/grid/viewport",
            params={"ne_lat": 37.6},
        )
        assert response.status_code == 422

    @pytest.mark.asyncio
    async def test_viewport_with_fires(self, client, mock_engine, mock_manager):
        """GET /api/grid/viewport after ignition includes the active grid."""
        # Ignite fire at Gwanghwamun
        r = await client.post(
            "/api/fire",
            json={"lat": 37.5760, "lng": 126.9769},
        )
        grid_id = r.json()["grid_id"]

        # Query viewport that includes Gwanghwamun
        response = await client.get(
            "/api/grid/viewport",
            params={
                "ne_lat": 37.58, "ne_lng": 126.98,
                "sw_lat": 37.57, "sw_lng": 126.97,
            },
        )
        assert response.status_code == 200
        data = response.json()
        assert data["total_active_grids"] >= 1

        # The ignited grid should be in the results
        grid_ids_in_response = [g["grid_id"] for g in data["grids"]]
        assert grid_id in grid_ids_in_response
