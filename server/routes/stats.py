"""Landing page statistics endpoint (mock server compatibility).

GET /api/stats — returns activeGrids, totalFires, onlineUsers in camelCase
to match the contract expected by the frontend (built against server/).
"""

from __future__ import annotations

import logging
import time

from fastapi import APIRouter, HTTPException

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api", tags=["stats"])


def _get_engine():
    from main import engine
    if engine is None:
        raise HTTPException(
            status_code=503,
            detail="Fire engine not available — Redis may be disconnected",
        )
    return engine


def _get_manager():
    from main import manager
    return manager


@router.get("/stats", summary="Get global fire statistics for landing page")
async def get_stats() -> dict:
    """Return aggregated fire statistics in camelCase (mock server compat).

    Response: { activeGrids, totalFires, onlineUsers }
    """
    engine = _get_engine()
    mgr = _get_manager()
    now = time.time()

    grid_ids = await engine._get_active_grid_ids()

    active_grids = 0
    total_fires = 0
    for grid_id in grid_ids:
        count = await engine._get_active_count(grid_id, now)
        if count > 0:
            active_grids += 1
            total_fires += count

    return {
        "activeGrids": active_grids,
        "totalFires": total_fires,
        "onlineUsers": mgr.active_count,
    }
