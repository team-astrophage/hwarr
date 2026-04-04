"""Map configuration endpoint.

Serves predefined clickable fire location coordinates for the frontend map.
These locations are landmark points across South Korea that users can click
to ignite fires during the hackathon demo.

The map config includes:
- Map center and default zoom level
- Bounding box for the map view
- Predefined fire locations with names and GPS coordinates
- Grid size metadata
"""

from __future__ import annotations

from pydantic import BaseModel, Field

from fastapi import APIRouter

from grid import LAT_UNIT, LNG_UNIT, to_grid_id

router = APIRouter(prefix="/api", tags=["map"])


# ---------------------------------------------------------------------------
# Response models
# ---------------------------------------------------------------------------


class FireLocation(BaseModel):
    """A predefined clickable fire location on the map."""

    id: str = Field(description="Unique location identifier")
    name: str = Field(description="Display name (Korean)")
    lat: float = Field(description="Latitude")
    lng: float = Field(description="Longitude")
    grid_id: str = Field(description="Pre-computed grid cell ID for this location")
    description: str = Field(default="", description="Optional description")


class MapBounds(BaseModel):
    """Bounding box for the map view."""

    ne_lat: float = Field(description="Northeast corner latitude")
    ne_lng: float = Field(description="Northeast corner longitude")
    sw_lat: float = Field(description="Southwest corner latitude")
    sw_lng: float = Field(description="Southwest corner longitude")


class GridConfig(BaseModel):
    """Grid system configuration metadata."""

    lat_unit: float = Field(description="Latitude unit per grid cell (degrees)")
    lng_unit: float = Field(description="Longitude unit per grid cell (degrees)")
    cell_size_meters: int = Field(description="Approximate cell size in meters")


class MapConfig(BaseModel):
    """Complete map configuration returned by the API."""

    center_lat: float = Field(description="Default map center latitude")
    center_lng: float = Field(description="Default map center longitude")
    default_zoom: int = Field(description="Default map zoom level")
    min_zoom: int = Field(description="Minimum allowed zoom level")
    max_zoom: int = Field(description="Maximum allowed zoom level")
    bounds: MapBounds = Field(description="Map bounding box")
    grid: GridConfig = Field(description="Grid system metadata")
    locations: list[FireLocation] = Field(
        description="Predefined clickable fire locations"
    )


# ---------------------------------------------------------------------------
# Predefined fire locations — Korean landmarks for the hackathon demo
# ---------------------------------------------------------------------------

_PREDEFINED_LOCATIONS: list[dict] = [
    # Seoul landmarks
    {
        "id": "gwanghwamun",
        "name": "광화문광장",
        "lat": 37.5760,
        "lng": 126.9769,
        "description": "서울 광화문광장",
    },
    {
        "id": "gangnam",
        "name": "강남역",
        "lat": 37.4979,
        "lng": 127.0276,
        "description": "서울 강남역 사거리",
    },
    {
        "id": "hongdae",
        "name": "홍대입구",
        "lat": 37.5563,
        "lng": 126.9236,
        "description": "서울 홍대입구역",
    },
    {
        "id": "yeouido",
        "name": "여의도공원",
        "lat": 37.5284,
        "lng": 126.9344,
        "description": "서울 여의도공원",
    },
    {
        "id": "jamsil",
        "name": "잠실종합운동장",
        "lat": 37.5153,
        "lng": 127.0728,
        "description": "서울 잠실종합운동장",
    },
    {
        "id": "namsan",
        "name": "남산타워",
        "lat": 37.5512,
        "lng": 126.9882,
        "description": "서울 남산서울타워",
    },
    {
        "id": "itaewon",
        "name": "이태원",
        "lat": 37.5345,
        "lng": 126.9946,
        "description": "서울 이태원거리",
    },
    # Major cities
    {
        "id": "busan_haeundae",
        "name": "해운대해수욕장",
        "lat": 35.1587,
        "lng": 129.1604,
        "description": "부산 해운대해수욕장",
    },
    {
        "id": "daegu_dongseong",
        "name": "동성로",
        "lat": 35.8691,
        "lng": 128.5958,
        "description": "대구 동성로",
    },
    {
        "id": "incheon_songdo",
        "name": "송도센트럴파크",
        "lat": 37.3925,
        "lng": 126.6632,
        "description": "인천 송도센트럴파크",
    },
    {
        "id": "gwangju_chungjang",
        "name": "충장로",
        "lat": 35.1488,
        "lng": 126.9156,
        "description": "광주 충장로",
    },
    {
        "id": "daejeon_dunsan",
        "name": "둔산동",
        "lat": 36.3511,
        "lng": 127.3782,
        "description": "대전 둔산동",
    },
    {
        "id": "jeju_hallasan",
        "name": "한라산",
        "lat": 33.3617,
        "lng": 126.5292,
        "description": "제주 한라산",
    },
    {
        "id": "suwon_hwaseong",
        "name": "수원화성",
        "lat": 37.2870,
        "lng": 127.0095,
        "description": "수원 화성행궁",
    },
    {
        "id": "gyeongju_bulguksa",
        "name": "불국사",
        "lat": 35.7900,
        "lng": 129.3322,
        "description": "경주 불국사",
    },
    # Demo / hackathon venue fallback (Gangnam COEX area)
    {
        "id": "coex",
        "name": "코엑스",
        "lat": 37.5126,
        "lng": 127.0590,
        "description": "서울 코엑스 (해커톤 데모 장소)",
    },
]


def _build_fire_locations() -> list[FireLocation]:
    """Build FireLocation objects with pre-computed grid IDs."""
    return [
        FireLocation(
            id=loc["id"],
            name=loc["name"],
            lat=loc["lat"],
            lng=loc["lng"],
            grid_id=to_grid_id(loc["lat"], loc["lng"]),
            description=loc.get("description", ""),
        )
        for loc in _PREDEFINED_LOCATIONS
    ]


# Pre-build at module load — these are static
FIRE_LOCATIONS: list[FireLocation] = _build_fire_locations()

# South Korea bounding box (approximate)
_KOREA_BOUNDS = MapBounds(
    ne_lat=38.6,
    ne_lng=131.9,
    sw_lat=33.0,
    sw_lng=124.5,
)

_GRID_CONFIG = GridConfig(
    lat_unit=LAT_UNIT,
    lng_unit=LNG_UNIT,
    cell_size_meters=100,
)

# Default map config — centered on South Korea
MAP_CONFIG = MapConfig(
    center_lat=36.5,
    center_lng=127.8,
    default_zoom=7,
    min_zoom=6,
    max_zoom=18,
    bounds=_KOREA_BOUNDS,
    grid=_GRID_CONFIG,
    locations=FIRE_LOCATIONS,
)


# ---------------------------------------------------------------------------
# Endpoint
# ---------------------------------------------------------------------------


@router.get(
    "/map/config",
    response_model=MapConfig,
    summary="Get map configuration with predefined fire locations",
)
async def get_map_config():
    """Return the map configuration including predefined clickable fire locations.

    The frontend uses this to:
    1. Initialize the map view (center, zoom, bounds)
    2. Render clickable fire location markers on the map
    3. Know grid cell size for overlay rendering

    Each location includes a pre-computed grid_id so the frontend can
    immediately associate clicks with the correct grid cell.
    """
    return MAP_CONFIG
