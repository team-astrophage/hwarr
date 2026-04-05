"""Demo mode fallback endpoint for GPS-unavailable clients.

When a client accesses the app with ?demo=true, GPS geolocation may not
be available (e.g., desktop browser, denied permissions, indoor venue).
This module provides:

- GET /api/demo/location       — Returns a random predefined Korean landmark
- GET /api/demo/locations      — Returns all available demo locations
- POST /api/demo/fire          — Ignite fire at a random demo location (no GPS needed)

The frontend should use these endpoints when:
1. navigator.geolocation is unavailable
2. User denied GPS permission
3. ?demo=true query parameter is present in the URL
"""

from __future__ import annotations

import logging
import random
import time
import uuid
from typing import Optional

from fastapi import APIRouter, HTTPException, Query
from pydantic import BaseModel, Field

from config import FIRE_TTL_SEC
from grid import grid_id_to_center, to_grid_id
from models.fire import build_grid_state
from routes.map_config import _PREDEFINED_LOCATIONS, FIRE_LOCATIONS, FireLocation

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/demo", tags=["demo"])


# ---------------------------------------------------------------------------
# Response models
# ---------------------------------------------------------------------------


class DemoLocationResponse(BaseModel):
    """A single demo location returned as GPS fallback."""

    id: str = Field(description="Location identifier")
    name: str = Field(description="Display name (Korean)")
    lat: float = Field(description="Latitude")
    lng: float = Field(description="Longitude")
    grid_id: str = Field(description="Pre-computed grid cell ID")
    description: str = Field(default="", description="Location description")
    demo: bool = Field(default=True, description="Indicates this is a demo coordinate")


class DemoLocationsListResponse(BaseModel):
    """List of all available demo locations."""

    locations: list[DemoLocationResponse]
    total: int = Field(description="Total number of demo locations")
    demo: bool = Field(default=True, description="Indicates demo mode")


class DemoFireResponse(BaseModel):
    """Response for demo fire ignition (no GPS required)."""

    grid_id: str
    event_id: str
    active_count: int
    stage: int
    stage_info: dict
    location: DemoLocationResponse = Field(
        description="The demo location where fire was ignited"
    )
    demo: bool = Field(default=True, description="Indicates demo mode fire")


# ---------------------------------------------------------------------------
# Engine accessor — same pattern as routes/fire.py
# ---------------------------------------------------------------------------

def _get_engine():
    """Get the FireProgressionEngine from the main module."""
    from main import engine
    if engine is None:
        raise HTTPException(
            status_code=503,
            detail="Fire engine not available — Redis may be disconnected",
        )
    return engine


def _get_manager():
    """Get the ConnectionManager from the main module for broadcasting."""
    from main import manager
    return manager


# ---------------------------------------------------------------------------
# Helper: pick a demo location
# ---------------------------------------------------------------------------

def _pick_random_location(location_id: Optional[str] = None) -> dict:
    """Pick a demo location — specific by ID or random.

    Args:
        location_id: Optional specific location ID. If None, picks random.

    Returns:
        Location dict from _PREDEFINED_LOCATIONS.

    Raises:
        HTTPException: If location_id is provided but not found.
    """
    if location_id:
        for loc in _PREDEFINED_LOCATIONS:
            if loc["id"] == location_id:
                return loc
        raise HTTPException(
            status_code=404,
            detail=f"Demo location '{location_id}' not found. "
                   f"Available: {[l['id'] for l in _PREDEFINED_LOCATIONS]}",
        )
    return random.choice(_PREDEFINED_LOCATIONS)


def _location_to_response(loc: dict) -> DemoLocationResponse:
    """Convert a raw location dict to DemoLocationResponse."""
    return DemoLocationResponse(
        id=loc["id"],
        name=loc["name"],
        lat=loc["lat"],
        lng=loc["lng"],
        grid_id=to_grid_id(loc["lat"], loc["lng"]),
        description=loc.get("description", ""),
        demo=True,
    )


# ---------------------------------------------------------------------------
# GET /api/demo/location — Random demo coordinate fallback
# ---------------------------------------------------------------------------


@router.get(
    "/location",
    response_model=DemoLocationResponse,
    summary="Get a random demo location (GPS fallback)",
)
async def get_demo_location(
    location_id: Optional[str] = Query(
        None,
        description="Specific location ID (optional — random if omitted)",
    ),
) -> DemoLocationResponse:
    """Return a demo GPS location for clients without GPS access.

    When the frontend detects ?demo=true or GPS is unavailable, it calls
    this endpoint to get a valid coordinate for fire ignition.

    Query params:
        location_id: Optional specific location ID. If omitted, returns random.

    Returns:
        DemoLocationResponse with lat/lng and grid_id ready for fire ignition.
    """
    loc = _pick_random_location(location_id)
    response = _location_to_response(loc)

    logger.info(
        "Demo location served: %s (%s) at [%f, %f]",
        loc["id"], loc["name"], loc["lat"], loc["lng"],
    )

    return response


# ---------------------------------------------------------------------------
# GET /api/demo/locations — All available demo locations
# ---------------------------------------------------------------------------


@router.get(
    "/locations",
    response_model=DemoLocationsListResponse,
    summary="List all available demo locations",
)
async def list_demo_locations() -> DemoLocationsListResponse:
    """Return all available demo locations for the GPS fallback picker.

    The frontend can display these as clickable options when GPS is unavailable,
    letting users pick a Korean landmark to "be at" during the demo.
    """
    locations = [_location_to_response(loc) for loc in _PREDEFINED_LOCATIONS]
    return DemoLocationsListResponse(
        locations=locations,
        total=len(locations),
        demo=True,
    )


# ---------------------------------------------------------------------------
# POST /api/demo/fire — Ignite fire without GPS
# ---------------------------------------------------------------------------


class DemoFireRequest(BaseModel):
    """Request body for demo fire ignition."""

    location_id: Optional[str] = Field(
        None,
        description="Specific location ID. If omitted, picks a random location.",
    )


@router.post(
    "/fire",
    response_model=DemoFireResponse,
    summary="Ignite fire at a demo location (no GPS needed)",
    status_code=201,
)
async def ignite_demo_fire(
    body: Optional[DemoFireRequest] = None,
) -> DemoFireResponse:
    """Ignite a fire at a random demo location — GPS not required.

    This is the demo-mode equivalent of POST /api/fire. Instead of requiring
    real GPS coordinates, it picks a predefined Korean landmark and ignites
    a fire there.

    Process:
      1. Pick a demo location (random or by location_id)
      2. Convert to grid cell
      3. Register fire event in Redis
      4. Broadcast to all connected clients
      5. Return fire state + demo location info

    Use when: ?demo=true is set and GPS is unavailable.
    """
    engine = _get_engine()
    mgr = _get_manager()

    location_id = body.location_id if body else None
    loc = _pick_random_location(location_id)
    lat = loc["lat"]
    lng = loc["lng"]

    # GPS → Grid ID
    grid_id = to_grid_id(lat, lng)

    # Generate fire event
    event_id = f"fire-demo-{uuid.uuid4().hex[:12]}"
    expire_at = time.time() + FIRE_TTL_SEC

    # Register in Redis (landing grid may differ if source reached spread threshold)
    registration = await engine.register_fire(grid_id, event_id, expire_at)
    if registration.grid_id != grid_id:
        lat, lng = grid_id_to_center(registration.grid_id)
    grid_id = registration.grid_id
    active_count = registration.active_count

    # Build state
    state = build_grid_state(
        grid_id=grid_id,
        active_count=active_count,
        lat=lat,
        lng=lng,
    )

    # Broadcast to clients
    ignite_payload = {
        "grid_id": grid_id,
        "event_id": event_id,
        "lat": lat,
        "lng": lng,
        "active_count": active_count,
        "stage": state.stage,
        "stage_info": state.stage_info.model_dump(),
        "ignited_by": "demo_mode",
        "demo": True,
        "location_name": loc["name"],
        "timestamp": time.time(),
    }
    await mgr.broadcast_to_room("fire:ignite", ignite_payload, room=grid_id)
    await mgr.broadcast("fire:global_update", {
        "grid_id": grid_id,
        "active_count": active_count,
        "stage": state.stage,
        "stage_info": state.stage_info.model_dump(),
        "demo": True,
        "timestamp": time.time(),
    })

    demo_loc = _location_to_response(loc)

    logger.info(
        "Demo fire ignited: grid=%s location=%s count=%d stage=%d",
        grid_id, loc["id"], active_count, state.stage,
    )

    return DemoFireResponse(
        grid_id=grid_id,
        event_id=event_id,
        active_count=active_count,
        stage=state.stage,
        stage_info=state.stage_info.model_dump(),
        location=demo_loc,
        demo=True,
    )
