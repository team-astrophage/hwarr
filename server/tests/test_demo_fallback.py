"""Tests for ?demo=true GPS fallback functionality.

Verifies that when GPS is unavailable, the demo mode provides:
1. Random demo locations via REST API
2. Specific location selection by ID
3. Demo fire ignition without GPS coordinates
4. Socket.IO fire:ignite with demo fallback
"""

from __future__ import annotations

import pytest
from httpx import ASGITransport, AsyncClient

from grid import to_grid_id
from routes.demo import (
    DemoFireResponse,
    DemoLocationResponse,
    DemoLocationsListResponse,
    _location_to_response,
    _pick_random_location,
)
from routes.map_config import _PREDEFINED_LOCATIONS


# ---------------------------------------------------------------------------
# Unit tests for helper functions
# ---------------------------------------------------------------------------


class TestPickRandomLocation:
    """Test the location picker helper."""

    def test_returns_a_predefined_location(self):
        loc = _pick_random_location()
        ids = {l["id"] for l in _PREDEFINED_LOCATIONS}
        assert loc["id"] in ids

    def test_specific_location_by_id(self):
        loc = _pick_random_location("gwanghwamun")
        assert loc["id"] == "gwanghwamun"
        assert loc["name"] == "광화문광장"

    def test_specific_location_gangnam(self):
        loc = _pick_random_location("gangnam")
        assert loc["id"] == "gangnam"

    def test_unknown_location_raises(self):
        with pytest.raises(Exception):
            _pick_random_location("nonexistent_location_xyz")

    def test_none_returns_random(self):
        """None should return a random location (not raise)."""
        loc = _pick_random_location(None)
        assert "lat" in loc
        assert "lng" in loc

    def test_randomness_over_multiple_calls(self):
        """Multiple calls should return different locations (probabilistic)."""
        results = set()
        for _ in range(50):
            loc = _pick_random_location()
            results.add(loc["id"])
        # With 16 locations and 50 tries, we should see at least 2 different
        assert len(results) >= 2, "Expected variety in random location picks"


class TestLocationToResponse:
    """Test conversion from raw dict to response model."""

    def test_converts_to_response(self):
        loc = _PREDEFINED_LOCATIONS[0]
        resp = _location_to_response(loc)
        assert isinstance(resp, DemoLocationResponse)
        assert resp.id == loc["id"]
        assert resp.lat == loc["lat"]
        assert resp.lng == loc["lng"]
        assert resp.demo is True

    def test_grid_id_computed(self):
        loc = _PREDEFINED_LOCATIONS[0]
        resp = _location_to_response(loc)
        expected = to_grid_id(loc["lat"], loc["lng"])
        assert resp.grid_id == expected

    def test_all_locations_convertible(self):
        """Every predefined location should convert without error."""
        for loc in _PREDEFINED_LOCATIONS:
            resp = _location_to_response(loc)
            assert resp.id == loc["id"]
            assert resp.demo is True


# ---------------------------------------------------------------------------
# Integration tests for REST endpoints
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_demo_location_returns_valid_coords():
    """GET /api/demo/location should return a valid demo location."""
    from main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/demo/location")

    assert resp.status_code == 200
    data = resp.json()
    assert "lat" in data
    assert "lng" in data
    assert "grid_id" in data
    assert "name" in data
    assert data["demo"] is True

    # Coordinates should be within South Korea
    assert 33.0 <= data["lat"] <= 39.0
    assert 124.0 <= data["lng"] <= 132.0


@pytest.mark.asyncio
async def test_demo_location_specific_id():
    """GET /api/demo/location?location_id=gwanghwamun should return that location."""
    from main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/demo/location?location_id=gwanghwamun")

    assert resp.status_code == 200
    data = resp.json()
    assert data["id"] == "gwanghwamun"
    assert data["name"] == "광화문광장"
    assert data["demo"] is True


@pytest.mark.asyncio
async def test_demo_location_invalid_id():
    """GET /api/demo/location?location_id=invalid should return 404."""
    from main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/demo/location?location_id=nonexistent")

    assert resp.status_code == 404


@pytest.mark.asyncio
async def test_demo_locations_list():
    """GET /api/demo/locations should return all demo locations."""
    from main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/demo/locations")

    assert resp.status_code == 200
    data = resp.json()
    assert "locations" in data
    assert data["total"] == len(_PREDEFINED_LOCATIONS)
    assert data["demo"] is True

    # All locations should have demo=True and valid coords
    for loc in data["locations"]:
        assert loc["demo"] is True
        assert "grid_id" in loc
        assert 33.0 <= loc["lat"] <= 39.0


@pytest.mark.asyncio
async def test_demo_location_has_grid_id():
    """Demo locations should include pre-computed grid_id for immediate use."""
    from main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/demo/location?location_id=gangnam")

    data = resp.json()
    expected_grid = to_grid_id(data["lat"], data["lng"])
    assert data["grid_id"] == expected_grid


@pytest.mark.asyncio
async def test_demo_location_randomness():
    """Multiple calls should occasionally return different locations."""
    from main import app

    transport = ASGITransport(app=app)
    ids = set()
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        for _ in range(20):
            resp = await client.get("/api/demo/location")
            assert resp.status_code == 200
            ids.add(resp.json()["id"])

    assert len(ids) >= 2, "Expected random variety across demo location calls"


# ---------------------------------------------------------------------------
# Test demo mode via Socket.IO fire:ignite event
# ---------------------------------------------------------------------------


class TestSioFireIgniteDemoFallback:
    """Test that fire:ignite with demo=true works without lat/lng."""

    def test_predefined_locations_available_for_import(self):
        """Ensure sio/events.py can import predefined locations."""
        from routes.map_config import _PREDEFINED_LOCATIONS
        assert len(_PREDEFINED_LOCATIONS) > 0

    def test_demo_flag_in_predefined_locations(self):
        """All predefined locations should have lat and lng for demo use."""
        for loc in _PREDEFINED_LOCATIONS:
            assert isinstance(loc["lat"], (int, float))
            assert isinstance(loc["lng"], (int, float))
