"""Daily region ranking endpoint.

GET /api/ranking/today?limit=10 — returns today's (KST) top arson regions.

Response shape:
  { "date": "YYYY-MM-DD", "items": [ { rank, region, count }, ... ] }

Returns an empty items list when the admin resolver is disabled or no fires
have been registered yet today.
"""

from __future__ import annotations

import logging
from datetime import datetime

from fastapi import APIRouter, HTTPException, Query

from config import KST, STATS_DAILY_RANKING_PREFIX

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api", tags=["ranking"])


def _get_engine():
    from main import engine
    if engine is None:
        raise HTTPException(
            status_code=503,
            detail="Fire engine not available — Redis may be disconnected",
        )
    return engine


@router.get("/ranking/today", summary="Today's top arson regions (KST)")
async def get_today_ranking(
    limit: int = Query(10, ge=1, le=50, description="Max number of regions to return"),
) -> dict:
    engine = _get_engine()
    redis = engine._redis

    today_str = datetime.now(KST).strftime("%Y-%m-%d")
    key = f"{STATS_DAILY_RANKING_PREFIX}{today_str}"

    # ZREVRANGE with scores → [(member_bytes, score_float), ...]
    raw = await redis.zrevrange(key, 0, limit - 1, withscores=True)

    items = []
    for rank, (member, score) in enumerate(raw, start=1):
        region = member.decode("utf-8") if isinstance(member, (bytes, bytearray)) else str(member)
        items.append({
            "rank": rank,
            "region": region,
            "count": int(score),
        })

    return {"date": today_str, "items": items}
