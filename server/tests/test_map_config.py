"""Tests for the map configuration endpoint."""

from __future__ import annotations

import pytest
from httpx import ASGITransport, AsyncClient

from grid import to_grid_id
from routes.map_config import (
    FIRE_LOCATIONS,
    MAP_CONFIG,
    FireLocation,
    MapConfig,
    _PREDEFINED_LOCATIONS,
    _build_fire_locations,
)


# ---------------------------------------------------------------------------
# Unit tests for data integrity
# ---------------------------------------------------------------------------


class TestPredefinedLocations:
    """Verify predefined fire locations are well-formed."""

    def test_locations_not_empty(self):
        assert len(_PREDEFINED_LOCATIONS) > 0

    def test_all_locations_have_required_fields(self):
        for loc in _PREDEFINED_LOCATIONS:
            assert "id" in loc
            assert "name" in loc
            assert "lat" in loc
            assert "lng" in loc

    def test_all_ids_unique(self):
        ids = [loc["id"] for loc in _PREDEFINED_LOCATIONS]
        assert len(ids) == len(set(ids)), "Duplicate location IDs found"

    def test_coordinates_in_korea_range(self):
        """All locations should be within South Korea's approximate bounds."""
        for loc in _PREDEFINED_LOCATIONS:
            assert 33.0 <= loc["lat"] <= 39.0, (
                f"{loc['id']} lat {loc['lat']} out of Korea range"
            )
            assert 124.0 <= loc["lng"] <= 132.0, (
                f"{loc['id']} lng {loc['lng']} out of Korea range"
            )

    def test_has_seoul_locations(self):
        """Should include at least some Seoul landmarks."""
        ids = {loc["id"] for loc in _PREDEFINED_LOCATIONS}
        assert "gwanghwamun" in ids
        assert "gangnam" in ids

    def test_has_major_cities(self):
        """Should include locations outside Seoul."""
        ids = {loc["id"] for loc in _PREDEFINED_LOCATIONS}
        assert "busan_haeundae" in ids
        assert "jeju_hallasan" in ids


class TestBuildFireLocations:
    """Verify the location builder produces correct FireLocation objects."""

    def test_returns_list_of_fire_locations(self):
        locations = _build_fire_locations()
        assert all(isinstance(loc, FireLocation) for loc in locations)

    def test_grid_ids_are_computed(self):
        """Each location should have a grid_id matching to_grid_id()."""
        locations = _build_fire_locations()
        for loc in locations:
            expected_grid_id = to_grid_id(loc.lat, loc.lng)
            assert loc.grid_id == expected_grid_id, (
                f"{loc.id}: grid_id mismatch — got {loc.grid_id}, "
                f"expected {expected_grid_id}"
            )

    def test_count_matches_predefined(self):
        locations = _build_fire_locations()
        assert len(locations) == len(_PREDEFINED_LOCATIONS)


class TestMapConfig:
    """Verify the static MapConfig object is well-formed."""

    def test_is_valid_model(self):
        assert isinstance(MAP_CONFIG, MapConfig)

    def test_center_in_korea(self):
        assert 33.0 <= MAP_CONFIG.center_lat <= 39.0
        assert 124.0 <= MAP_CONFIG.center_lng <= 132.0

    def test_zoom_levels(self):
        assert MAP_CONFIG.min_zoom < MAP_CONFIG.default_zoom < MAP_CONFIG.max_zoom

    def test_bounds_are_valid(self):
        b = MAP_CONFIG.bounds
        assert b.sw_lat < b.ne_lat
        assert b.sw_lng < b.ne_lng

    def test_grid_config(self):
        assert MAP_CONFIG.grid.cell_size_meters == 100
        assert MAP_CONFIG.grid.lat_unit > 0
        assert MAP_CONFIG.grid.lng_unit > 0

    def test_locations_populated(self):
        assert len(MAP_CONFIG.locations) == len(FIRE_LOCATIONS)


# ---------------------------------------------------------------------------
# Integration test for the endpoint
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_map_config_endpoint_returns_config():
    """GET /api/map/config should return full map configuration."""
    from main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/map/config")

    assert resp.status_code == 200
    data = resp.json()

    # Top-level fields
    assert "center_lat" in data
    assert "center_lng" in data
    assert "default_zoom" in data
    assert "bounds" in data
    assert "grid" in data
    assert "locations" in data

    # Locations should be a non-empty list
    assert isinstance(data["locations"], list)
    assert len(data["locations"]) > 0

    # Each location should have expected fields
    loc = data["locations"][0]
    assert "id" in loc
    assert "name" in loc
    assert "lat" in loc
    assert "lng" in loc
    assert "grid_id" in loc


@pytest.mark.asyncio
async def test_map_config_endpoint_grid_ids_present():
    """Every location in the response should have a valid grid_id."""
    from main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/map/config")

    data = resp.json()
    for loc in data["locations"]:
        assert ":" in loc["grid_id"], (
            f"Location {loc['id']} has invalid grid_id: {loc['grid_id']}"
        )
