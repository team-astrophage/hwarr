"""Reset REST API endpoint.

POST /api/reset — Clear all Redis data and reset engine state
"""

from __future__ import annotations

import logging

from fastapi import APIRouter, HTTPException

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api", tags=["reset"])


def _get_engine():
    from main import engine
    if engine is None:
        raise HTTPException(
            status_code=503,
            detail="Fire engine not available — Redis may be disconnected",
        )
    return engine


@router.post("/reset", summary="Reset all fire data")
async def reset_all():
    """Clear all fire events, firefighters, and active grid tracking from Redis."""
    engine = _get_engine()
    redis = engine._redis
    deleted = 0

    # Delete fire:* keys via SCAN
    async for key in redis.scan_iter(match="fire:*"):
        await redis.delete(key)
        deleted += 1

    # Delete firefighter:* keys via SCAN
    async for key in redis.scan_iter(match="firefighter:*"):
        await redis.delete(key)
        deleted += 1

    # Delete tracking sets
    for key in ["active_grids", "active_firefighters", "firefighter_grids"]:
        result = await redis.delete(key)
        deleted += result

    # Clear in-memory engine state
    engine._grid_stages.clear()

    logger.info("Reset complete: deleted %d Redis keys", deleted)

    return {"message": "reset complete", "deleted_keys": deleted}
