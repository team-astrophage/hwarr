"""
Tests for fire stage progression and grid utilities.

Verifies AC 2: Fire stage progression visible:
  불씨(1-4 clicks) → 모닥불(5-14) → 화재(15-29) → 대화재(30-49) → 전소(50+)
"""

import sys
import os
import pytest

# Add server root to path so imports work
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from models.fire import (
    FireStage,
    STAGE_CONFIGS,
    get_stage,
    get_stage_config,
    should_dispatch_firefighter,
    get_firefighter_remove_count,
    build_grid_state,
)
from grid import (
    get_stage_info,
    get_stage_name,
    grid_id_to_center,
    to_grid_id,
    get_grids_in_viewport,
)


# ─── Stage progression tests ───────────────────────────────────────


class TestFireStageProgression:
    """Test the complete fire stage progression from 불씨 to 전소."""

    def test_stage_0_no_fire(self):
        """Stage 0 (없음): 0 clicks — no fire."""
        assert get_stage(0) == 0

    def test_stage_1_bulssee_lower_bound(self):
        """Stage 1 (불씨): starts at 1 click."""
        assert get_stage(1) == 1

    def test_stage_1_bulssee_upper_bound(self):
        """Stage 1 (불씨): ends at 4 clicks."""
        assert get_stage(4) == 1

    def test_stage_2_modakbul_lower_bound(self):
        """Stage 2 (모닥불): starts at 5 clicks."""
        assert get_stage(5) == 2

    def test_stage_2_modakbul_upper_bound(self):
        """Stage 2 (모닥불): ends at 14 clicks."""
        assert get_stage(14) == 2

    def test_stage_3_hwajae_lower_bound(self):
        """Stage 3 (화재): starts at 15 clicks."""
        assert get_stage(15) == 3

    def test_stage_3_hwajae_upper_bound(self):
        """Stage 3 (화재): ends at 29 clicks."""
        assert get_stage(29) == 3

    def test_stage_4_daehwajae_lower_bound(self):
        """Stage 4 (대화재): starts at 30 clicks."""
        assert get_stage(30) == 4

    def test_stage_4_daehwajae_upper_bound(self):
        """Stage 4 (대화재): ends at 49 clicks."""
        assert get_stage(49) == 4

    def test_stage_5_jeonso_lower_bound(self):
        """Stage 5 (전소): starts at 50 clicks."""
        assert get_stage(50) == 5

    def test_stage_5_jeonso_high_count(self):
        """Stage 5 (전소): stays at 5 even with very high counts."""
        assert get_stage(100) == 5
        assert get_stage(999) == 5

    def test_negative_count_returns_none(self):
        """Negative counts should return stage 0."""
        assert get_stage(-1) == 0


class TestStageTransitions:
    """Test exact boundary transitions between stages."""

    @pytest.mark.parametrize(
        "count,expected_stage",
        [
            (0, 0),   # none
            (1, 1),   # → 불씨
            (4, 1),   # still 불씨
            (5, 2),   # → 모닥불
            (14, 2),  # still 모닥불
            (15, 3),  # → 화재
            (29, 3),  # still 화재
            (30, 4),  # → 대화재
            (49, 4),  # still 대화재
            (50, 5),  # → 전소
        ],
    )
    def test_stage_boundaries(self, count: int, expected_stage: int):
        """Verify every stage boundary transition."""
        assert get_stage(count) == expected_stage

    def test_full_progression_sequence(self):
        """Simulate a full fire lifecycle from 0 to 50+ clicks."""
        stages_seen = []
        prev_stage = -1
        for count in range(60):
            stage = get_stage(count)
            if stage != prev_stage:
                stages_seen.append((count, int(stage)))
                prev_stage = stage

        # Should see: 0→0, 1→1, 5→2, 15→3, 30→4, 50→5
        assert stages_seen == [
            (0, 0),
            (1, 1),
            (5, 2),
            (15, 3),
            (30, 4),
            (50, 5),
        ]


# ─── Stage info / name tests ──────────────────────────────────────


class TestStageInfo:
    """Test stage metadata functions."""

    def test_stage_names_korean(self):
        """All stages should have Korean names."""
        assert get_stage_name(0) == "없음"
        assert get_stage_name(1) == "불씨"
        assert get_stage_name(2) == "모닥불"
        assert get_stage_name(3) == "화재"
        assert get_stage_name(4) == "대화재"
        assert get_stage_name(5) == "전소"

    def test_unknown_stage_name(self):
        """Unknown stage values should return a fallback."""
        assert get_stage_name(99) == "알 수 없음"

    def test_get_stage_info_complete(self):
        """get_stage_info returns complete info dict."""
        info = get_stage_info(35)
        assert info["stage"] == 4
        assert info["stage_name"] == "대화재"
        assert info["active_count"] == 35
        assert info["firefighter_triggered"] is True

    def test_firefighter_not_triggered_below_stage_4(self):
        """Firefighter should NOT trigger below stage 4 (대화재)."""
        for count in [0, 1, 5, 15, 29]:
            info = get_stage_info(count)
            assert info["firefighter_triggered"] is False, (
                f"Firefighter should not trigger at count={count}"
            )

    def test_firefighter_triggered_at_stage_4_plus(self):
        """Firefighter SHOULD trigger at stage 4+ (대화재, 전소)."""
        for count in [30, 49, 50, 100]:
            info = get_stage_info(count)
            assert info["firefighter_triggered"] is True, (
                f"Firefighter should trigger at count={count}"
            )


class TestFirefighterDispatch:
    """Test firefighter-related functions from models.fire."""

    def test_should_dispatch_below_threshold(self):
        """Firefighter should NOT dispatch below 30 clicks."""
        assert should_dispatch_firefighter(0) is False
        assert should_dispatch_firefighter(1) is False
        assert should_dispatch_firefighter(14) is False
        assert should_dispatch_firefighter(29) is False

    def test_should_dispatch_at_stage_4(self):
        """Firefighter should dispatch at 30+ clicks (대화재)."""
        assert should_dispatch_firefighter(30) is True
        assert should_dispatch_firefighter(49) is True

    def test_should_dispatch_at_stage_5(self):
        """Firefighter should dispatch at 50+ clicks (전소)."""
        assert should_dispatch_firefighter(50) is True
        assert should_dispatch_firefighter(100) is True

    def test_remove_count_at_stage_4(self):
        """Firefighter removes 2 fires per sweep at 대화재."""
        assert get_firefighter_remove_count(30) == 2
        assert get_firefighter_remove_count(49) == 2

    def test_remove_count_at_stage_5(self):
        """Firefighter removes 3 fires per sweep at 전소."""
        assert get_firefighter_remove_count(50) == 3

    def test_remove_count_below_threshold(self):
        """No removal below firefighter threshold."""
        assert get_firefighter_remove_count(0) == 0
        assert get_firefighter_remove_count(29) == 0


class TestBuildGridState:
    """Test GridState builder from models.fire."""

    def test_build_grid_state_basic(self):
        """build_grid_state creates correct GridState."""
        state = build_grid_state("123:456", 35)
        assert state.grid_id == "123:456"
        assert state.active_count == 35
        assert state.stage == 4
        assert state.stage_info.label_ko == "대화재"
        assert state.stage_info.triggers_firefighter is True

    def test_build_grid_state_with_coords(self):
        """build_grid_state includes lat/lng when provided."""
        state = build_grid_state("123:456", 5, lat=37.5, lng=127.0)
        assert state.lat == 37.5
        assert state.lng == 127.0
        assert state.stage == 2
        assert state.stage_info.label_ko == "모닥불"

    def test_build_grid_state_empty(self):
        """build_grid_state handles zero count."""
        state = build_grid_state("0:0", 0)
        assert state.stage == 0
        assert state.stage_info.label_ko == "없음"


# ─── Grid coordinate tests ────────────────────────────────────────


class TestGridConversion:
    """Test GPS to grid ID conversion."""

    def test_to_grid_id_basic(self):
        """Basic grid ID generation."""
        grid_id = to_grid_id(37.5665, 126.9780)
        assert ":" in grid_id
        parts = grid_id.split(":")
        assert len(parts) == 2
        assert all(p.lstrip("-").isdigit() for p in parts)

    def test_same_cell_returns_same_id(self):
        """Coordinates within same 100m cell should return same grid ID."""
        id1 = to_grid_id(37.5665, 126.9780)
        id2 = to_grid_id(37.5666, 126.9781)
        assert id1 == id2

    def test_different_cells_return_different_ids(self):
        """Coordinates far apart should return different grid IDs."""
        id1 = to_grid_id(37.5665, 126.9780)
        id2 = to_grid_id(37.5700, 126.9800)
        assert id1 != id2

    def test_grid_id_roundtrip(self):
        """Grid ID → center coordinates should be within the cell."""
        lat, lng = 37.5665, 126.9780
        grid_id = to_grid_id(lat, lng)
        center_lat, center_lng = grid_id_to_center(grid_id)
        # Center should be within ~100m of original
        assert abs(center_lat - lat) < 0.001
        assert abs(center_lng - lng) < 0.001


class TestViewportGrids:
    """Test viewport to grid list conversion."""

    def test_single_grid_viewport(self):
        """Very small viewport should return at least one grid."""
        grids = get_grids_in_viewport(37.567, 126.979, 37.566, 126.978)
        assert len(grids) >= 1

    def test_viewport_grid_count(self):
        """Larger viewport should return multiple grids."""
        # ~500m x 500m viewport
        grids = get_grids_in_viewport(37.570, 126.985, 37.565, 126.980)
        assert len(grids) > 1

    def test_viewport_contains_center_grid(self):
        """Viewport should contain the grid for its center point."""
        ne_lat, ne_lng = 37.570, 126.985
        sw_lat, sw_lng = 37.565, 126.980
        center_lat = (ne_lat + sw_lat) / 2
        center_lng = (ne_lng + sw_lng) / 2

        grids = get_grids_in_viewport(ne_lat, ne_lng, sw_lat, sw_lng)
        center_grid = to_grid_id(center_lat, center_lng)
        assert center_grid in grids


# ─── FireStage enum tests ─────────────────────────────────────────


class TestFireStageEnum:
    """Test FireStage enum integrity."""

    def test_all_stages_have_configs(self):
        """Every FireStage enum value should have a config entry."""
        for stage in FireStage:
            assert stage in STAGE_CONFIGS

    def test_stages_are_sequential(self):
        """Stages should be 0-5 sequentially."""
        assert list(FireStage) == [0, 1, 2, 3, 4, 5]

    def test_stage_comparison(self):
        """Stages should be comparable as integers."""
        assert FireStage.BULSSSI < FireStage.MODAKBUL
        assert FireStage.MODAKBUL < FireStage.HWAJAE
        assert FireStage.HWAJAE < FireStage.DAEHWAJAE
        assert FireStage.DAEHWAJAE < FireStage.JEONSO

    def test_thresholds_match_ac(self):
        """Verify thresholds match the AC specification exactly."""
        assert STAGE_CONFIGS[FireStage.BULSSSI].threshold == 1
        assert STAGE_CONFIGS[FireStage.MODAKBUL].threshold == 5
        assert STAGE_CONFIGS[FireStage.HWAJAE].threshold == 15
        assert STAGE_CONFIGS[FireStage.DAEHWAJAE].threshold == 30
        assert STAGE_CONFIGS[FireStage.JEONSO].threshold == 50
