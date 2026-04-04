"""Breaking news (속보) endpoint.

GET /api/news — combines news templates with live Redis fire data
to produce a feed of breaking-news style items for the landing page.
"""

from __future__ import annotations

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
# Location mapping — lat/lng → Korean place name (3-tier)
#   Tier 1: Landmarks (좁은 범위, 구체적 장소명)
#   Tier 2: Districts (서울 25개 구 + 주요 도시 구/군)
#   Tier 3: Provinces (시/도 단위)
#   Fallback: 좌표 기반 "N37.5° E127.0° 부근"
# ---------------------------------------------------------------------------

_LANDMARKS: list[dict] = [
    {"name": "강남 테헤란로", "lat": (37.49, 37.52), "lng": (127.02, 127.07)},
    {"name": "서초 교대역", "lat": (37.47, 37.50), "lng": (126.98, 127.02)},
    {"name": "송파 잠실", "lat": (37.505, 37.52), "lng": (127.07, 127.10)},
    {"name": "종로 광화문", "lat": (37.57, 37.58), "lng": (126.97, 126.98)},
    {"name": "홍대입구", "lat": (37.556, 37.562), "lng": (126.92, 126.93)},
    {"name": "이태원", "lat": (37.533, 37.537), "lng": (126.99, 127.00)},
    {"name": "여의도", "lat": (37.52, 37.53), "lng": (126.92, 126.94)},
    {"name": "디지털단지", "lat": (37.48, 37.49), "lng": (126.89, 126.91)},
    {"name": "성수동", "lat": (37.54, 37.55), "lng": (127.05, 127.06)},
    {"name": "판교 테크노밸리", "lat": (37.39, 37.41), "lng": (127.09, 127.11)},
]

_DISTRICTS: list[dict] = [
    # 서울 25개 구
    {"name": "서울 종로구", "lat": (37.57, 37.61), "lng": (126.96, 127.02)},
    {"name": "서울 중구", "lat": (37.55, 37.57), "lng": (126.96, 127.01)},
    {"name": "서울 용산구", "lat": (37.52, 37.55), "lng": (126.95, 127.01)},
    {"name": "서울 성동구", "lat": (37.54, 37.57), "lng": (127.01, 127.07)},
    {"name": "서울 광진구", "lat": (37.53, 37.56), "lng": (127.07, 127.11)},
    {"name": "서울 동대문구", "lat": (37.57, 37.60), "lng": (127.02, 127.07)},
    {"name": "서울 중랑구", "lat": (37.59, 37.62), "lng": (127.07, 127.11)},
    {"name": "서울 성북구", "lat": (37.58, 37.62), "lng": (126.99, 127.04)},
    {"name": "서울 강북구", "lat": (37.61, 37.65), "lng": (126.99, 127.04)},
    {"name": "서울 도봉구", "lat": (37.65, 37.70), "lng": (127.02, 127.06)},
    {"name": "서울 노원구", "lat": (37.62, 37.67), "lng": (127.04, 127.10)},
    {"name": "서울 은평구", "lat": (37.59, 37.64), "lng": (126.90, 126.96)},
    {"name": "서울 서대문구", "lat": (37.56, 37.60), "lng": (126.92, 126.97)},
    {"name": "서울 마포구", "lat": (37.54, 37.57), "lng": (126.88, 126.96)},
    {"name": "서울 양천구", "lat": (37.51, 37.54), "lng": (126.85, 126.89)},
    {"name": "서울 강서구", "lat": (37.54, 37.58), "lng": (126.81, 126.86)},
    {"name": "서울 구로구", "lat": (37.48, 37.51), "lng": (126.85, 126.90)},
    {"name": "서울 금천구", "lat": (37.44, 37.48), "lng": (126.89, 126.92)},
    {"name": "서울 영등포구", "lat": (37.51, 37.54), "lng": (126.88, 126.93)},
    {"name": "서울 동작구", "lat": (37.49, 37.52), "lng": (126.93, 126.98)},
    {"name": "서울 관악구", "lat": (37.46, 37.49), "lng": (126.92, 126.97)},
    {"name": "서울 서초구", "lat": (37.46, 37.50), "lng": (126.97, 127.06)},
    {"name": "서울 강남구", "lat": (37.49, 37.53), "lng": (127.01, 127.10)},
    {"name": "서울 송파구", "lat": (37.49, 37.53), "lng": (127.07, 127.14)},
    {"name": "서울 강동구", "lat": (37.52, 37.56), "lng": (127.11, 127.17)},
    # 수도권
    {"name": "성남 분당구", "lat": (37.35, 37.41), "lng": (127.05, 127.15)},
    {"name": "수원 영통구", "lat": (37.25, 37.30), "lng": (127.03, 127.09)},
    {"name": "수원 권선구", "lat": (37.24, 37.29), "lng": (126.96, 127.02)},
    {"name": "용인 수지구", "lat": (37.30, 37.35), "lng": (127.06, 127.12)},
    {"name": "고양 일산", "lat": (37.64, 37.70), "lng": (126.72, 126.82)},
    {"name": "안양시", "lat": (37.38, 37.42), "lng": (126.91, 126.97)},
    {"name": "부천시", "lat": (37.48, 37.52), "lng": (126.76, 126.83)},
    {"name": "안산시", "lat": (37.30, 37.35), "lng": (126.80, 126.88)},
    {"name": "화성 동탄", "lat": (37.18, 37.23), "lng": (127.05, 127.10)},
    {"name": "인천 남동구", "lat": (37.39, 37.43), "lng": (126.68, 126.75)},
    {"name": "인천 연수구", "lat": (37.37, 37.41), "lng": (126.63, 126.70)},
    {"name": "인천 부평구", "lat": (37.49, 37.52), "lng": (126.70, 126.75)},
    # 광역시
    {"name": "부산 해운대구", "lat": (35.15, 35.20), "lng": (129.13, 129.21)},
    {"name": "부산 부산진구", "lat": (35.14, 35.18), "lng": (129.03, 129.08)},
    {"name": "부산 남구", "lat": (35.11, 35.15), "lng": (129.07, 129.12)},
    {"name": "대구 중구", "lat": (35.86, 35.88), "lng": (128.58, 128.61)},
    {"name": "대구 수성구", "lat": (35.83, 35.87), "lng": (128.61, 128.67)},
    {"name": "대전 유성구", "lat": (36.33, 36.40), "lng": (127.30, 127.40)},
    {"name": "대전 서구", "lat": (36.33, 36.38), "lng": (127.37, 127.42)},
    {"name": "광주 서구", "lat": (35.13, 35.17), "lng": (126.86, 126.91)},
    {"name": "울산 남구", "lat": (35.52, 35.56), "lng": (129.32, 129.37)},
]

_PROVINCES: list[dict] = [
    {"name": "서울", "lat": (37.42, 37.70), "lng": (126.77, 127.18)},
    {"name": "인천", "lat": (37.35, 37.60), "lng": (126.35, 126.80)},
    {"name": "경기 북부", "lat": (37.70, 38.30), "lng": (126.40, 127.80)},
    {"name": "경기 남부", "lat": (36.90, 37.42), "lng": (126.60, 127.60)},
    {"name": "부산", "lat": (35.05, 35.25), "lng": (128.85, 129.25)},
    {"name": "대구", "lat": (35.75, 36.05), "lng": (128.45, 128.80)},
    {"name": "대전", "lat": (36.25, 36.50), "lng": (127.30, 127.55)},
    {"name": "광주", "lat": (35.05, 35.25), "lng": (126.75, 127.00)},
    {"name": "울산", "lat": (35.45, 35.65), "lng": (129.20, 129.50)},
    {"name": "세종", "lat": (36.45, 36.65), "lng": (126.85, 127.10)},
    {"name": "강원", "lat": (37.00, 38.60), "lng": (127.50, 129.40)},
    {"name": "충북", "lat": (36.30, 37.20), "lng": (127.30, 128.30)},
    {"name": "충남", "lat": (36.00, 37.00), "lng": (126.00, 127.30)},
    {"name": "전북", "lat": (35.30, 36.20), "lng": (126.30, 127.50)},
    {"name": "전남", "lat": (34.00, 35.50), "lng": (126.00, 127.50)},
    {"name": "경북", "lat": (35.50, 37.10), "lng": (128.30, 130.00)},
    {"name": "경남", "lat": (34.50, 35.80), "lng": (127.50, 129.40)},
    {"name": "제주", "lat": (33.10, 33.60), "lng": (126.10, 127.00)},
]


def _get_location_name(grid_id: str) -> str:
    """Resolve a grid ID to a human-readable Korean location name.

    Checks 3 tiers (landmarks → districts → provinces), then falls
    back to a coordinate-based description.
    """
    lat, lng = grid_id_to_center(grid_id)

    for tier in (_LANDMARKS, _DISTRICTS, _PROVINCES):
        for area in tier:
            lat_lo, lat_hi = area["lat"]
            lng_lo, lng_hi = area["lng"]
            if lat_lo <= lat <= lat_hi and lng_lo <= lng <= lng_hi:
                return area["name"]

    return f"N{lat:.1f}° E{lng:.1f}° 부근"


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
