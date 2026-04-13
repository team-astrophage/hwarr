package model

// ZMember represents a sorted set member with its score.
// Shared across handler and redis packages to avoid adapter boilerplate.
type ZMember struct {
	Member string
	Score  float64
}

// FireStage represents fire intensity stages.
type FireStage int

const (
	StageNone      FireStage = 0 // no active fires
	StageBulsssi   FireStage = 1 // ember/spark (1-5 clicks)
	StageModakbul  FireStage = 2 // campfire (6-23 clicks)
	StageHwajae    FireStage = 3 // fire (24-71 clicks)
	StageDaehwajae FireStage = 4 // big fire (72-169 clicks)
	StageJeonso    FireStage = 5 // total burn (170+ clicks)
)

// StageConfig holds configuration for a single fire stage.
type StageConfig struct {
	Stage                  FireStage `json:"stage"`
	LabelKo                string    `json:"label_ko"`
	LabelEn                string    `json:"label_en"`
	Threshold              int       `json:"threshold"`
	DurationSec            int       `json:"duration_sec"`
	TriggersFirefighter    bool      `json:"triggers_firefighter"`
	FirefighterRemoveCount int       `json:"firefighter_remove_count"`
}

// StageConfigs maps each fire stage to its configuration.
var StageConfigs = map[FireStage]StageConfig{
	StageNone: {
		Stage: StageNone, LabelKo: "없음", LabelEn: "none",
		Threshold: 0, DurationSec: 0,
		TriggersFirefighter: false, FirefighterRemoveCount: 0,
	},
	StageBulsssi: {
		Stage: StageBulsssi, LabelKo: "불씨", LabelEn: "ember",
		Threshold: 1, DurationSec: 86400,
		TriggersFirefighter: false, FirefighterRemoveCount: 0,
	},
	StageModakbul: {
		Stage: StageModakbul, LabelKo: "모닥불", LabelEn: "campfire",
		Threshold: 6, DurationSec: 86400,
		TriggersFirefighter: false, FirefighterRemoveCount: 0,
	},
	StageHwajae: {
		Stage: StageHwajae, LabelKo: "화재", LabelEn: "fire",
		Threshold: 24, DurationSec: 86400,
		TriggersFirefighter: false, FirefighterRemoveCount: 0,
	},
	StageDaehwajae: {
		Stage: StageDaehwajae, LabelKo: "대화재", LabelEn: "big fire",
		Threshold: 72, DurationSec: 86400,
		TriggersFirefighter: true, FirefighterRemoveCount: 2,
	},
	StageJeonso: {
		Stage: StageJeonso, LabelKo: "전소", LabelEn: "total burn",
		Threshold: 170, DurationSec: 86400,
		TriggersFirefighter: true, FirefighterRemoveCount: 3,
	},
}

// stageThresholds is pre-sorted descending for efficient lookup.
var stageThresholds = []struct {
	Threshold int
	Stage     FireStage
}{
	{170, StageJeonso},
	{72, StageDaehwajae},
	{24, StageHwajae},
	{6, StageModakbul},
	{1, StageBulsssi},
	{0, StageNone},
}

// GetStage determines the fire stage from active fire count.
func GetStage(activeCount int) FireStage {
	for _, t := range stageThresholds {
		if activeCount >= t.Threshold {
			return t.Stage
		}
	}
	return StageNone
}

// FireStageInfo is the stage info returned in API responses.
type FireStageInfo struct {
	Stage               int    `json:"stage"`
	LabelKo             string `json:"labelKo"`
	LabelEn             string `json:"labelEn"`
	TriggersFirefighter bool   `json:"triggersFirefighter"`
}

// GridState represents the state of a single grid cell.
type GridState struct {
	GridID      string        `json:"gridId"`
	Lat         float64       `json:"lat"`
	Lng         float64       `json:"lng"`
	ActiveCount int           `json:"activeCount"`
	Stage       int           `json:"stage"`
	StageInfo   FireStageInfo `json:"stageInfo"`
}

// BuildGridState creates a GridState from grid ID, active count, and center coordinates.
func BuildGridState(gridID string, activeCount int, lat, lng float64) GridState {
	stage := GetStage(activeCount)
	cfg := StageConfigs[stage]
	return GridState{
		GridID:      gridID,
		Lat:         lat,
		Lng:         lng,
		ActiveCount: activeCount,
		Stage:       int(stage),
		StageInfo: FireStageInfo{
			Stage:               int(stage),
			LabelKo:             cfg.LabelKo,
			LabelEn:             cfg.LabelEn,
			TriggersFirefighter: cfg.TriggersFirefighter,
		},
	}
}
