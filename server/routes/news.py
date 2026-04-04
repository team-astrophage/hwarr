"""Breaking news (속보) endpoint.

GET /api/news — combines news templates with live Redis fire data
to produce a feed of breaking-news style items for the landing page.
"""

from __future__ import annotations

import hashlib
import logging
import time

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from grid import grid_id_to_center
from models.fire import FireStage, get_stage, STAGE_CONFIGS

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api", tags=["news"])

FIRE_TTL_SEC = 1800
MAX_NEWS_ITEMS = 5

# ---------------------------------------------------------------------------
# Location mapping — lat/lng → Korean place name
# ---------------------------------------------------------------------------

_AREAS: list[dict] = [
    {"name": "강남 테헤란로", "lat": (37.49, 37.52), "lng": (127.02, 127.07)},
    {"name": "서초 교대역", "lat": (37.47, 37.50), "lng": (126.98, 127.02)},
    {"name": "송파 잠실", "lat": (37.49, 37.52), "lng": (127.07, 127.13)},
    {"name": "종로 광화문", "lat": (37.57, 37.60), "lng": (126.97, 127.01)},
    {"name": "마포 홍대입구", "lat": (37.54, 37.57), "lng": (126.89, 126.95)},
    {"name": "용산 이태원", "lat": (37.52, 37.55), "lng": (126.96, 127.00)},
    {"name": "영등포 여의도", "lat": (37.51, 37.54), "lng": (126.89, 126.93)},
    {"name": "구로 디지털단지", "lat": (37.48, 37.51), "lng": (126.85, 126.90)},
    {"name": "성동 성수동", "lat": (37.54, 37.57), "lng": (127.02, 127.07)},
    {"name": "관악 신림동", "lat": (37.46, 37.49), "lng": (126.92, 126.97)},
    {"name": "동작 노량진", "lat": (37.49, 37.51), "lng": (126.93, 126.97)},
    {"name": "강동 천호동", "lat": (37.53, 37.56), "lng": (127.12, 127.17)},
    {"name": "중구 명동", "lat": (37.55, 37.57), "lng": (126.97, 127.02)},
    {"name": "노원 상계동", "lat": (37.63, 37.67), "lng": (127.05, 127.10)},
    {"name": "강서 마곡", "lat": (37.55, 37.57), "lng": (126.82, 126.86)},
    {"name": "은평 연신내", "lat": (37.60, 37.64), "lng": (126.91, 126.95)},
    {"name": "판교 테크노밸리", "lat": (37.38, 37.42), "lng": (127.08, 127.12)},
    {"name": "수원 광교", "lat": (37.28, 37.32), "lng": (127.04, 127.08)},
    {"name": "인천 송도", "lat": (37.37, 37.41), "lng": (126.64, 126.68)},
    {"name": "부산 해운대", "lat": (35.15, 35.18), "lng": (129.16, 129.20)},
]

_FALLBACK_NAMES = [
    "미지의 구역", "화재 지점 A", "격전지 B", "불꽃 지대",
    "방화 현장", "도심 외곽", "한적한 골목", "번화가 뒷편",
]


def _get_location_name(grid_id: str) -> str:
    """Resolve a grid ID to a human-readable Korean location name."""
    lat, lng = grid_id_to_center(grid_id)

    for area in _AREAS:
        lat_lo, lat_hi = area["lat"]
        lng_lo, lng_hi = area["lng"]
        if lat_lo <= lat <= lat_hi and lng_lo <= lng <= lng_hi:
            return area["name"]

    idx = int(hashlib.md5(grid_id.encode()).hexdigest(), 16) % len(_FALLBACK_NAMES)
    return _FALLBACK_NAMES[idx]


# ---------------------------------------------------------------------------
# Pydantic models
# ---------------------------------------------------------------------------

class HeadlinePart(BaseModel):
    text: str
    highlight: str | None = None


class NewsItem(BaseModel):
    id: str
    icon: str
    icon_bg: str
    headline_parts: list[HeadlinePart]
    time: str
    detail: str


# ---------------------------------------------------------------------------
# News template builders
# ---------------------------------------------------------------------------

def _format_time_ago(seconds: float) -> str:
    """Convert elapsed seconds to a human-friendly Korean string."""
    if seconds < 0:
        seconds = 0
    minutes = int(seconds / 60)
    if minutes < 1:
        return "방금 전"
    if minutes < 60:
        return f"{minutes}분 전"
    hours = minutes // 60
    return f"{hours}시간 전"


def _build_big_fire(location: str, stage_label: str, count: int, time_ago: str, grid_id: str) -> NewsItem:
    return NewsItem(
        id=f"news-bf-{grid_id}",
        icon="🔥",
        icon_bg="rgba(255,68,68,0.15)",
        headline_parts=[
            HeadlinePart(text=location, highlight="accent"),
            HeadlinePart(text=" 일대 "),
            HeadlinePart(text=stage_label, highlight="warning"),
            HeadlinePart(text=" 발생"),
        ],
        time=time_ago,
        detail=f"활성 방화범 {count}명 · {STAGE_CONFIGS[get_stage(count)].stage.value}단계 지속 중",
    )


def _build_fire_active(location: str, stage_label: str, count: int, time_ago: str, grid_id: str) -> NewsItem:
    return NewsItem(
        id=f"news-fa-{grid_id}",
        icon="🔥",
        icon_bg="rgba(255,140,0,0.2)",
        headline_parts=[
            HeadlinePart(text=location, highlight="accent"),
            HeadlinePart(text=" 반경 "),
            HeadlinePart(text=stage_label, highlight="warning"),
            HeadlinePart(text=" 확산 중"),
        ],
        time=time_ago,
        detail=f"활성 방화범 {count}명",
    )


def _build_firefighter(location: str, count: int, time_ago: str, grid_id: str) -> NewsItem:
    return NewsItem(
        id=f"news-ff-{grid_id}",
        icon="🧯",
        icon_bg="rgba(30,215,96,0.15)",
        headline_parts=[
            HeadlinePart(text=location, highlight="accent"),
            HeadlinePart(text=" "),
            HeadlinePart(text="소방관 출동", highlight="warning"),
        ],
        time=time_ago,
        detail=f"화재 진압 중 — 잔여 {count}건",
    )


def _build_small_fire(location: str, count: int, time_ago: str, grid_id: str) -> NewsItem:
    return NewsItem(
        id=f"news-sf-{grid_id}",
        icon="🔥",
        icon_bg="rgba(255,140,0,0.12)",
        headline_parts=[
            HeadlinePart(text=location, highlight="accent"),
            HeadlinePart(text=" 부근 "),
            HeadlinePart(text="불씨", highlight="warning"),
            HeadlinePart(text=" 감지"),
        ],
        time=time_ago,
        detail=f"활성 방화범 {count}명",
    )


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _get_engine():
    from main import engine
    if engine is None:
        raise HTTPException(
            status_code=503,
            detail="Fire engine not available — Redis may be disconnected",
        )
    return engine


async def _get_latest_ignite_ts(redis, grid_id: str) -> float | None:
    """Estimate the most recent ignite timestamp for a grid."""
    key = f"fire:{grid_id}"
    latest = await redis.zrevrange(key, 0, 0, withscores=True)
    if not latest:
        return None
    _, expiry_score = latest[0]
    return float(expiry_score) - FIRE_TTL_SEC


# ---------------------------------------------------------------------------
# Endpoint
# ---------------------------------------------------------------------------

@router.get("/news", summary="Get breaking-news feed for landing page")
async def get_news() -> list[NewsItem]:
    """Build a news feed from live Redis fire data + templates.

    Scans active grids, ranks by fire stage/count, selects an appropriate
    template for each, and returns up to MAX_NEWS_ITEMS items.
    """
    engine = _get_engine()
    redis = engine._redis
    now = time.time()

    grid_ids = await engine._get_active_grid_ids()

    grids: list[dict] = []
    for grid_id in grid_ids:
        count = await engine._get_active_count(grid_id, now)
        if count <= 0:
            continue
        stage = get_stage(count)
        has_firefighter = await redis.sismember("firefighter_grids", grid_id)
        ignite_ts = await _get_latest_ignite_ts(redis, grid_id)
        grids.append({
            "grid_id": grid_id,
            "count": count,
            "stage": stage,
            "has_firefighter": bool(has_firefighter),
            "ignite_ts": ignite_ts or now,
        })

    grids.sort(key=lambda g: (g["stage"], g["count"]), reverse=True)
    grids = grids[:MAX_NEWS_ITEMS]

    items: list[NewsItem] = []
    for g in grids:
        grid_id = g["grid_id"]
        count = g["count"]
        stage: FireStage = g["stage"]
        has_ff = g["has_firefighter"]
        time_ago = _format_time_ago(now - g["ignite_ts"])
        location = _get_location_name(grid_id)
        stage_label = STAGE_CONFIGS[stage].label_ko

        if has_ff:
            items.append(_build_firefighter(location, count, time_ago, grid_id))
        elif stage.value >= 4:
            items.append(_build_big_fire(location, stage_label, count, time_ago, grid_id))
        elif stage.value >= 2:
            items.append(_build_fire_active(location, stage_label, count, time_ago, grid_id))
        else:
            items.append(_build_small_fire(location, count, time_ago, grid_id))

    return items
