"""Fire ignition and grid state REST API endpoints.

POST /api/fire       — Ignite a fire at GPS coordinates
GET  /api/grid/{id}  — Get state of a single grid cell
GET  /api/grid/viewport — Get all active fires in a viewport bounding box

These REST endpoints complement the Socket.IO fire:ignite event,
providing an HTTP alternative for fire ignition and state queries.
"""

from __future__ import annotations

import logging
import time
import uuid
from typing import Any, Optional

from fastapi import APIRouter, HTTPException, Query
from pydantic import BaseModel, Field

from config import FIRE_TTL_SEC
from grid import to_grid_id, grid_id_to_center, get_grids_in_viewport
from models.fire import build_grid_state, get_stage, GridState

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api", tags=["fire"])


# ---------------------------------------------------------------------------
# Request / Response models
# ---------------------------------------------------------------------------


class FireIgniteRequest(BaseModel):
    """Request body for POST /api/fire."""

    lat: float = Field(description="Latitude in decimal degrees")
    lng: float = Field(description="Longitude in decimal degrees")


class FireIgniteResponse(BaseModel):
    """Response body for POST /api/fire."""

    grid_id: str = Field(description="Grid cell ID where the fire was placed")
    event_id: str = Field(description="Unique fire event identifier")
    active_count: int = Field(description="Current active fire count in the grid")
    stage: int = Field(description="Current fire stage (0-5)")
    stage_info: dict = Field(description="Stage metadata (label, firefighter trigger)")


class GridStateResponse(BaseModel):
    """Response body for GET /api/grid/{grid_id}."""

    grid_id: str
    active_count: int
    stage: int
    stage_info: dict


class ViewportResponse(BaseModel):
    """Response body for GET /api/grid/viewport."""

    grids: list[dict] = Field(description="Active grid states in viewport")
    total_active_grids: int = Field(description="Number of grids with active fires")


# ---------------------------------------------------------------------------
# Engine accessor — lazy-loaded from main module
# ---------------------------------------------------------------------------

def _get_engine():
    """Get the FireProgressionEngine from the main module.

    The engine is initialized at startup in main.py. This function
    provides late-binding access so the router module doesn't need
    a direct import-time dependency on the engine instance.

    Raises:
        HTTPException: If the engine is not available (Redis down).
    """
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
# POST /api/fire — Fire ignition
# ---------------------------------------------------------------------------


@router.post(
    "/fire",
    response_model=FireIgniteResponse,
    summary="Ignite a fire at GPS coordinates",
    status_code=201,
)
async def ignite_fire(body: FireIgniteRequest) -> FireIgniteResponse:
    """Ignite a fire at the given GPS coordinates.

    Process:
      1. Convert GPS (lat, lng) → grid cell ID
      2. Register fire event in Redis via FireProgressionEngine
      3. Broadcast fire:ignite + fire:global_update to Socket.IO clients
      4. Return grid state to the caller

    The fire event has a 30-minute TTL and will auto-expire.
    If the grid reaches stage 4+ (대화재), a firefighter NPC is
    automatically dispatched by the engine.
    """
    engine = _get_engine()
    mgr = _get_manager()

    lat = body.lat
    lng = body.lng

    # 1. GPS → Grid ID
    grid_id = to_grid_id(lat, lng)

    # 2. Generate unique event ID and expiration
    event_id = f"fire-{uuid.uuid4().hex[:12]}"
    expire_at = time.time() + FIRE_TTL_SEC

    # 3. Register in Redis — engine handles neighbor-spreading, stage detection,
    #    and firefighter spawn. The landing grid may differ if the source was
    #    at/over the 500 spread threshold.
    registration = await engine.register_fire(grid_id, event_id, expire_at)
    # If the fire spread to a neighbor, use that grid's center for lat/lng
    if registration.grid_id != grid_id:
        lat, lng = grid_id_to_center(registration.grid_id)
    grid_id = registration.grid_id
    active_count = registration.active_count

    # 4. Build grid state for response and broadcast
    state = build_grid_state(
        grid_id=grid_id,
        active_count=active_count,
        lat=lat,
        lng=lng,
    )

    # 5. Broadcast to Socket.IO clients (same as sio/events.py fire:ignite)
    ignite_payload = {
        "grid_id": grid_id,
        "event_id": event_id,
        "lat": lat,
        "lng": lng,
        "active_count": active_count,
        "stage": state.stage,
        "stage_info": state.stage_info.model_dump(),
        "ignited_by": "rest_api",
        "timestamp": time.time(),
    }
    await mgr.broadcast_to_room("fire:ignite", ignite_payload, room=grid_id)
    await mgr.broadcast("fire:global_update", {
        "grid_id": grid_id,
        "active_count": active_count,
        "stage": state.stage,
        "stage_info": state.stage_info.model_dump(),
        "timestamp": time.time(),
    })

    logger.info(
        "POST /api/fire grid=%s count=%d stage=%d",
        grid_id, active_count, state.stage,
    )

    return FireIgniteResponse(
        grid_id=grid_id,
        event_id=event_id,
        active_count=active_count,
        stage=state.stage,
        stage_info=state.stage_info.model_dump(),
    )


# ---------------------------------------------------------------------------
# GET /api/grid/viewport — Viewport bulk query
# NOTE: This route MUST be defined before /grid/{grid_id} to avoid
# "viewport" being captured as a grid_id path parameter.
# ---------------------------------------------------------------------------


@router.get(
    "/grid/viewport",
    response_model=ViewportResponse,
    summary="Get all active fires in a map viewport",
)
async def get_viewport_fires(
    ne_lat: float = Query(..., description="Northeast corner latitude"),
    ne_lng: float = Query(..., description="Northeast corner longitude"),
    sw_lat: float = Query(..., description="Southwest corner latitude"),
    sw_lng: float = Query(..., description="Southwest corner longitude"),
) -> ViewportResponse:
    """Return fire states for all grid cells within a map viewport bounding box.

    Process:
      1. Compute all grid IDs within the bounding box
      2. Query Redis for active fire counts via pipeline
      3. Return only grids with active fires
    """
    engine = _get_engine()
    now = time.time()

    grid_ids = get_grids_in_viewport(ne_lat, ne_lng, sw_lat, sw_lng)

    grid_states = []
    for grid_id in grid_ids:
        active_count = await engine._get_active_count(grid_id, now)
        if active_count > 0:
            center_lat, center_lng = grid_id_to_center(grid_id)
            state = build_grid_state(
                grid_id=grid_id,
                active_count=active_count,
                lat=center_lat,
                lng=center_lng,
            )
            grid_states.append(state.model_dump())

    return ViewportResponse(
        grids=grid_states,
        total_active_grids=len(grid_states),
    )


# ---------------------------------------------------------------------------
# GET /api/grid/{grid_id} — Single grid state
# NOTE: Must be after /grid/viewport to avoid path parameter capturing "viewport"
# ---------------------------------------------------------------------------


@router.get(
    "/grid/{grid_id}",
    response_model=GridStateResponse,
    summary="Get fire state of a single grid cell",
)
async def get_grid_state(grid_id: str) -> GridStateResponse:
    """Return the current fire state for a specific grid cell.

    Process:
      1. ZCOUNT(fire:{gridId}, now, +inf) for active count
      2. Calculate fire stage from active count
    """
    engine = _get_engine()
    now = time.time()

    active_count = await engine._get_active_count(grid_id, now)
    state = build_grid_state(grid_id=grid_id, active_count=active_count)

    return GridStateResponse(
        grid_id=grid_id,
        active_count=active_count,
        stage=state.stage,
        stage_info=state.stage_info.model_dump(),
    )
