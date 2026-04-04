"""Fire stage enum, models, and stage transition rules.

Stages: 불씨(1-4) → 모닥불(5-14) → 화재(15-29) → 대화재(30-49) → 전소(50+)
Each stage has:
  - threshold: minimum active fire count to reach this stage
  - duration_sec: how long fires persist at this stage (contributes to TTL)
  - label_ko / label_en: display names
  - triggers_firefighter: whether this stage dispatches firefighter NPCs
"""

from __future__ import annotations

from enum import IntEnum
from typing import Optional

from pydantic import BaseModel, Field


class FireStage(IntEnum):
    """Fire intensity stages, ordered by severity."""

    NONE = 0       # 없음 — no active fires
    BULSSSI = 1    # 불씨 — ember/spark (1-4 clicks)
    MODAKBUL = 2   # 모닥불 — campfire (5-14 clicks)
    HWAJAE = 3     # 화재 — fire (15-29 clicks)
    DAEHWAJAE = 4  # 대화재 — big fire (30-49 clicks)
    JEONSO = 5     # 전소 — total burn (50+ clicks)


class StageConfig(BaseModel):
    """Configuration for a single fire stage."""

    stage: FireStage
    label_ko: str
    label_en: str
    threshold: int = Field(
        description="Minimum active fire count to reach this stage"
    )
    duration_sec: int = Field(
        default=10800,
        description="Base TTL for fires at this stage (seconds). Default 3 hours.",
    )
    triggers_firefighter: bool = Field(
        default=False,
        description="Whether reaching this stage dispatches firefighter NPCs",
    )
    firefighter_remove_count: int = Field(
        default=0,
        description="Number of fires a firefighter removes per sweep at this stage",
    )


# ---------------------------------------------------------------------------
# Stage transition rule table
# ---------------------------------------------------------------------------
# Thresholds based on active fire count in a single grid cell.
# Firefighters activate at stage 4 (대형화재) and above.
# ---------------------------------------------------------------------------

STAGE_CONFIGS: dict[FireStage, StageConfig] = {
    FireStage.NONE: StageConfig(
        stage=FireStage.NONE,
        label_ko="없음",
        label_en="none",
        threshold=0,
        duration_sec=0,
        triggers_firefighter=False,
        firefighter_remove_count=0,
    ),
    FireStage.BULSSSI: StageConfig(
        stage=FireStage.BULSSSI,
        label_ko="불씨",
        label_en="ember",
        threshold=1,           # 1-49 clicks
        duration_sec=10800,
        triggers_firefighter=False,
        firefighter_remove_count=0,
    ),
    FireStage.MODAKBUL: StageConfig(
        stage=FireStage.MODAKBUL,
        label_ko="모닥불",
        label_en="campfire",
        threshold=50,          # 50-99 clicks
        duration_sec=10800,
        triggers_firefighter=False,
        firefighter_remove_count=0,
    ),
    FireStage.HWAJAE: StageConfig(
        stage=FireStage.HWAJAE,
        label_ko="화재",
        label_en="fire",
        threshold=100,         # 100-149 clicks
        duration_sec=10800,
        triggers_firefighter=False,
        firefighter_remove_count=0,
    ),
    FireStage.DAEHWAJAE: StageConfig(
        stage=FireStage.DAEHWAJAE,
        label_ko="대화재",
        label_en="big fire",
        threshold=150,         # 150-199 clicks — firefighter triggers here
        duration_sec=10800,
        triggers_firefighter=True,
        firefighter_remove_count=2,
    ),
    FireStage.JEONSO: StageConfig(
        stage=FireStage.JEONSO,
        label_ko="전소",
        label_en="total burn",
        threshold=200,         # 200+ clicks
        duration_sec=10800,
        triggers_firefighter=True,
        firefighter_remove_count=3,
    ),
}

# Pre-sorted thresholds (descending) for efficient stage lookup
_STAGE_THRESHOLDS: list[tuple[int, FireStage]] = sorted(
    [(cfg.threshold, cfg.stage) for cfg in STAGE_CONFIGS.values()],
    key=lambda x: x[0],
    reverse=True,
)


def get_stage(active_count: int) -> FireStage:
    """Determine fire stage from active fire count in a grid cell.

    >>> get_stage(0)
    <FireStage.NONE: 0>
    >>> get_stage(1)
    <FireStage.BULSSSI: 1>
    >>> get_stage(5)
    <FireStage.MODAKBUL: 2>
    >>> get_stage(15)
    <FireStage.HWAJAE: 3>
    >>> get_stage(30)
    <FireStage.DAEHWAJAE: 4>
    >>> get_stage(100)
    <FireStage.JEONSO: 5>
    """
    for threshold, stage in _STAGE_THRESHOLDS:
        if active_count >= threshold:
            return stage
    return FireStage.NONE


def get_stage_config(stage: FireStage) -> StageConfig:
    """Get the full configuration for a given stage."""
    return STAGE_CONFIGS[stage]


def should_dispatch_firefighter(active_count: int) -> bool:
    """Check if the current fire count warrants firefighter dispatch."""
    stage = get_stage(active_count)
    return STAGE_CONFIGS[stage].triggers_firefighter


def get_firefighter_remove_count(active_count: int) -> int:
    """Get how many fires a firefighter should remove per sweep."""
    stage = get_stage(active_count)
    return STAGE_CONFIGS[stage].firefighter_remove_count


# ---------------------------------------------------------------------------
# Pydantic response models (for API serialization)
# ---------------------------------------------------------------------------

class FireStageInfo(BaseModel):
    """Stage info returned in API responses."""

    stage: int
    label_ko: str
    label_en: str
    triggers_firefighter: bool


class GridState(BaseModel):
    """State of a single grid cell, returned by API."""

    grid_id: str
    lat: Optional[float] = None
    lng: Optional[float] = None
    active_count: int
    stage: int
    stage_info: FireStageInfo


def build_grid_state(
    grid_id: str,
    active_count: int,
    lat: Optional[float] = None,
    lng: Optional[float] = None,
) -> GridState:
    """Convenience builder for GridState from raw count."""
    stage = get_stage(active_count)
    cfg = STAGE_CONFIGS[stage]
    return GridState(
        grid_id=grid_id,
        lat=lat,
        lng=lng,
        active_count=active_count,
        stage=stage.value,
        stage_info=FireStageInfo(
            stage=stage.value,
            label_ko=cfg.label_ko,
            label_en=cfg.label_en,
            triggers_firefighter=cfg.triggers_firefighter,
        ),
    )
