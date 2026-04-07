package model

import "fmt"

// HeadlinePart represents a single segment of a news headline.
// "highlight" can be "accent", "warning", or nil (plain text).
type HeadlinePart struct {
	Text      string  `json:"text"`
	Highlight *string `json:"highlight"`
}

// NewsItem represents a single breaking-news entry for the landing page.
type NewsItem struct {
	ID            string         `json:"id"`
	Icon          string         `json:"icon"`
	IconBg        string         `json:"icon_bg"`
	HeadlineParts []HeadlinePart `json:"headline_parts"`
	Time          string         `json:"time"`
	Detail        string         `json:"detail"`
}

// highlight constants
var (
	highlightAccent  = "accent"
	highlightWarning = "warning"
)

// FormatTimeAgo converts elapsed seconds to a human-friendly Korean string.
func FormatTimeAgo(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	minutes := int(seconds / 60)
	if minutes < 1 {
		return "방금 전"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d분 전", minutes)
	}
	hours := minutes / 60
	return fmt.Sprintf("%d시간 전", hours)
}

// BuildBigFire creates a news item for a big fire (stage >= 4).
func BuildBigFire(location, stageLabel string, count int, timeAgo, gridID string) NewsItem {
	stage := GetStage(count)
	return NewsItem{
		ID:     fmt.Sprintf("news-bf-%s", gridID),
		Icon:   "\U0001F525",
		IconBg: "rgba(255,68,68,0.15)",
		HeadlineParts: []HeadlinePart{
			{Text: location, Highlight: &highlightAccent},
			{Text: " 일대 ", Highlight: nil},
			{Text: stageLabel, Highlight: &highlightWarning},
			{Text: " 발생", Highlight: nil},
		},
		Time:   timeAgo,
		Detail: fmt.Sprintf("활성 방화범 %d명 · %d단계 지속 중", count, int(stage)),
	}
}

// BuildFireActive creates a news item for an actively spreading fire (stage 2-3).
func BuildFireActive(location, stageLabel string, count int, timeAgo, gridID string) NewsItem {
	return NewsItem{
		ID:     fmt.Sprintf("news-fa-%s", gridID),
		Icon:   "\U0001F525",
		IconBg: "rgba(255,140,0,0.2)",
		HeadlineParts: []HeadlinePart{
			{Text: location, Highlight: &highlightAccent},
			{Text: " 반경 ", Highlight: nil},
			{Text: stageLabel, Highlight: &highlightWarning},
			{Text: " 확산 중", Highlight: nil},
		},
		Time:   timeAgo,
		Detail: fmt.Sprintf("활성 방화범 %d명", count),
	}
}

// BuildSmallFire creates a news item for a small fire (stage 1).
func BuildSmallFire(location string, count int, timeAgo, gridID string) NewsItem {
	return NewsItem{
		ID:     fmt.Sprintf("news-sf-%s", gridID),
		Icon:   "\U0001F525",
		IconBg: "rgba(255,140,0,0.12)",
		HeadlineParts: []HeadlinePart{
			{Text: location, Highlight: &highlightAccent},
			{Text: " 부근 ", Highlight: nil},
			{Text: "불씨", Highlight: &highlightWarning},
			{Text: " 감지", Highlight: nil},
		},
		Time:   timeAgo,
		Detail: fmt.Sprintf("활성 방화범 %d명", count),
	}
}
