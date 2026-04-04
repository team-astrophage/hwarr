"""Tests for fire stage enum, transition rules, and model builders."""

import pytest

from models.fire import (
    FireStage,
    STAGE_CONFIGS,
    StageConfig,
    build_grid_state,
    get_firefighter_remove_count,
    get_stage,
    get_stage_config,
    should_dispatch_firefighter,
)


class TestFireStage:
    """FireStage enum basics."""

    def test_stage_values(self):
        assert FireStage.NONE == 0
        assert FireStage.BULSSSI == 1
        assert FireStage.MODAKBUL == 2
        assert FireStage.HWAJAE == 3
        assert FireStage.DAEHWAJAE == 4
        assert FireStage.JEONSO == 5

    def test_stage_ordering(self):
        stages = list(FireStage)
        assert stages == sorted(stages, key=lambda s: s.value)

    def test_all_stages_have_config(self):
        for stage in FireStage:
            assert stage in STAGE_CONFIGS


class TestGetStage:
    """Stage determination from active count."""

    @pytest.mark.parametrize(
        "count,expected",
        [
            (0, FireStage.NONE),
            (1, FireStage.BULSSSI),
            (4, FireStage.BULSSSI),
            (5, FireStage.MODAKBUL),
            (14, FireStage.MODAKBUL),
            (15, FireStage.HWAJAE),
            (29, FireStage.HWAJAE),
            (30, FireStage.DAEHWAJAE),
            (49, FireStage.DAEHWAJAE),
            (50, FireStage.JEONSO),
            (100, FireStage.JEONSO),
            (999, FireStage.JEONSO),
        ],
    )
    def test_threshold_boundaries(self, count: int, expected: FireStage):
        assert get_stage(count) == expected

    def test_negative_count_returns_none(self):
        # Edge case: negative should still return NONE
        assert get_stage(-1) == FireStage.NONE


class TestStageConfig:
    """Stage configuration validation."""

    def test_thresholds_are_monotonically_increasing(self):
        thresholds = [
            STAGE_CONFIGS[stage].threshold
            for stage in sorted(FireStage, key=lambda s: s.value)
        ]
        for i in range(1, len(thresholds)):
            assert thresholds[i] >= thresholds[i - 1]

    def test_duration_is_positive_for_active_stages(self):
        for stage in FireStage:
            if stage == FireStage.NONE:
                continue
            assert STAGE_CONFIGS[stage].duration_sec > 0

    def test_get_stage_config(self):
        cfg = get_stage_config(FireStage.DAEHWAJAE)
        assert isinstance(cfg, StageConfig)
        assert cfg.label_ko == "대화재"
        assert cfg.triggers_firefighter is True


class TestFirefighterLogic:
    """Firefighter dispatch rules."""

    def test_no_firefighter_below_stage_4(self):
        for count in [0, 1, 5, 15, 29]:
            assert should_dispatch_firefighter(count) is False

    def test_firefighter_at_stage_4(self):
        assert should_dispatch_firefighter(30) is True
        assert should_dispatch_firefighter(40) is True

    def test_firefighter_at_stage_5(self):
        assert should_dispatch_firefighter(50) is True
        assert should_dispatch_firefighter(100) is True

    def test_remove_count_stage_4(self):
        assert get_firefighter_remove_count(35) == 2

    def test_remove_count_stage_5(self):
        assert get_firefighter_remove_count(60) == 3

    def test_remove_count_below_threshold(self):
        assert get_firefighter_remove_count(5) == 0


class TestBuildGridState:
    """GridState builder."""

    def test_basic_build(self):
        state = build_grid_state("123:456", active_count=35)
        assert state.grid_id == "123:456"
        assert state.active_count == 35
        assert state.stage == 4
        assert state.stage_info.label_ko == "대화재"
        assert state.stage_info.triggers_firefighter is True

    def test_build_with_coords(self):
        state = build_grid_state("10:20", active_count=1, lat=37.5, lng=127.0)
        assert state.lat == 37.5
        assert state.lng == 127.0
        assert state.stage == 1

    def test_build_empty_grid(self):
        state = build_grid_state("0:0", active_count=0)
        assert state.stage == 0
        assert state.stage_info.label_ko == "없음"

    def test_serialization(self):
        state = build_grid_state("1:2", active_count=15)
        data = state.model_dump()
        assert "grid_id" in data
        assert "stage_info" in data
        assert data["stage_info"]["label_en"] == "fire"
