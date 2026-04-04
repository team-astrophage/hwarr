"""
GPS → Grid ID conversion and viewport utilities.

Grid system: 100m × 100m cells based on GPS coordinates.
Fire stage logic is defined in models.fire — this module re-exports for convenience.
"""

import math

from models.fire import (  # noqa: F401 — re-exports
    FireStage,
    GridState,
    STAGE_CONFIGS,
    build_grid_state,
    get_firefighter_remove_count,
    get_stage,
    get_stage_config,
    should_dispatch_firefighter,
)

# 100m ≈ 위도 0.0009도, 경도 0.0011도 (한국 기준, ~37°N)
LAT_UNIT = 0.0009
LNG_UNIT = 0.0011


def get_stage_name(stage: int) -> str:
    """Get the Korean display name for a fire stage."""
    try:
        cfg = get_stage_config(FireStage(stage))
        return cfg.label_ko
    except (ValueError, KeyError):
        return "알 수 없음"


def get_stage_info(active_count: int) -> dict:
    """
    Get complete stage information for a given active count.

    Returns:
        Dict with stage number, name, and whether firefighter is triggered.
    """
    stage = get_stage(active_count)
    cfg = STAGE_CONFIGS[stage]
    return {
        "stage": int(stage),
        "stage_name": cfg.label_ko,
        "active_count": active_count,
        "firefighter_triggered": cfg.triggers_firefighter,
    }


def to_grid_id(lat: float, lng: float) -> str:
    """
    Convert GPS coordinates to a grid cell ID.

    Uses a simple floor-division scheme to map lat/lng to 100m grid cells.

    Args:
        lat: Latitude in decimal degrees.
        lng: Longitude in decimal degrees.

    Returns:
        Grid ID string in format "grid_lat:grid_lng".
    """
    grid_lat = math.floor(lat / LAT_UNIT)
    grid_lng = math.floor(lng / LNG_UNIT)
    return f"{grid_lat}:{grid_lng}"


def grid_id_to_center(grid_id: str) -> tuple[float, float]:
    """
    Convert a grid ID back to the center coordinates of the grid cell.

    Args:
        grid_id: Grid ID in format "grid_lat:grid_lng".

    Returns:
        Tuple of (lat, lng) at the center of the grid cell.
    """
    parts = grid_id.split(":")
    grid_lat = int(parts[0])
    grid_lng = int(parts[1])
    center_lat = (grid_lat + 0.5) * LAT_UNIT
    center_lng = (grid_lng + 0.5) * LNG_UNIT
    return (center_lat, center_lng)


def get_grids_in_viewport(
    ne_lat: float, ne_lng: float, sw_lat: float, sw_lng: float
) -> list[str]:
    """
    Get all grid IDs within a map viewport bounding box.

    Args:
        ne_lat: Northeast corner latitude.
        ne_lng: Northeast corner longitude.
        sw_lat: Southwest corner latitude.
        sw_lng: Southwest corner longitude.

    Returns:
        List of grid ID strings within the viewport.
    """
    grids = []
    lat = math.floor(sw_lat / LAT_UNIT)
    lat_max = math.floor(ne_lat / LAT_UNIT)
    lng_min = math.floor(sw_lng / LNG_UNIT)
    lng_max = math.floor(ne_lng / LNG_UNIT)

    while lat <= lat_max:
        lng = lng_min
        while lng <= lng_max:
            grids.append(f"{lat}:{lng}")
            lng += 1
        lat += 1
    return grids
