package model

import (
	"encoding/json"
	"testing"
)

func TestFormatTimeAgo(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{-10, "방금 전"},
		{0, "방금 전"},
		{30, "방금 전"},
		{59, "방금 전"},
		{60, "1분 전"},
		{120, "2분 전"},
		{3540, "59분 전"},
		{3600, "1시간 전"},
		{7200, "2시간 전"},
		{86400, "24시간 전"},
	}
	for _, tt := range tests {
		got := FormatTimeAgo(tt.seconds)
		if got != tt.want {
			t.Errorf("FormatTimeAgo(%v) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestBuildBigFire(t *testing.T) {
	item := BuildBigFire("강남 테헤란로", "대화재", 150, "5분 전", "41688:115472")

	if item.ID != "news-bf-41688:115472" {
		t.Errorf("unexpected ID: %s", item.ID)
	}
	if item.Icon != "🔥" {
		t.Errorf("unexpected icon: %s", item.Icon)
	}
	if item.IconBg != "rgba(255,68,68,0.15)" {
		t.Errorf("unexpected icon_bg: %s", item.IconBg)
	}
	if len(item.HeadlineParts) != 4 {
		t.Fatalf("expected 4 headline parts, got %d", len(item.HeadlineParts))
	}
	if item.HeadlineParts[0].Text != "강남 테헤란로" || *item.HeadlineParts[0].Highlight != "accent" {
		t.Errorf("unexpected first headline part: %+v", item.HeadlineParts[0])
	}
	if item.HeadlineParts[2].Text != "대화재" || *item.HeadlineParts[2].Highlight != "warning" {
		t.Errorf("unexpected third headline part: %+v", item.HeadlineParts[2])
	}
	if item.Time != "5분 전" {
		t.Errorf("unexpected time: %s", item.Time)
	}
	// count=150 -> stage 4 (대화재)
	if item.Detail != "활성 방화범 150명 · 4단계 지속 중" {
		t.Errorf("unexpected detail: %s", item.Detail)
	}
}

func TestBuildFireActive(t *testing.T) {
	item := BuildFireActive("서울 마포구", "모닥불", 25, "10분 전", "41700:115400")

	if item.ID != "news-fa-41700:115400" {
		t.Errorf("unexpected ID: %s", item.ID)
	}
	if item.IconBg != "rgba(255,140,0,0.2)" {
		t.Errorf("unexpected icon_bg: %s", item.IconBg)
	}
	if item.HeadlineParts[1].Text != " 반경 " {
		t.Errorf("unexpected second headline part: %+v", item.HeadlineParts[1])
	}
	if item.HeadlineParts[3].Text != " 확산 중" {
		t.Errorf("unexpected fourth headline part: %+v", item.HeadlineParts[3])
	}
	if item.Detail != "활성 방화범 25명" {
		t.Errorf("unexpected detail: %s", item.Detail)
	}
}

func TestBuildSmallFire(t *testing.T) {
	item := BuildSmallFire("홍대입구", 3, "방금 전", "41728:115381")

	if item.ID != "news-sf-41728:115381" {
		t.Errorf("unexpected ID: %s", item.ID)
	}
	if item.IconBg != "rgba(255,140,0,0.12)" {
		t.Errorf("unexpected icon_bg: %s", item.IconBg)
	}
	if item.HeadlineParts[2].Text != "불씨" {
		t.Errorf("unexpected third headline part text: %s", item.HeadlineParts[2].Text)
	}
	if item.HeadlineParts[3].Text != " 감지" {
		t.Errorf("unexpected fourth headline part text: %s", item.HeadlineParts[3].Text)
	}
	if item.Detail != "활성 방화범 3명" {
		t.Errorf("unexpected detail: %s", item.Detail)
	}
}

func TestNewsItemJSON(t *testing.T) {
	item := BuildSmallFire("여의도", 5, "2분 전", "41694:115381")
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	// Verify JSON field names match Python snake_case convention
	if _, ok := parsed["icon_bg"]; !ok {
		t.Error("missing icon_bg in JSON")
	}
	if _, ok := parsed["headline_parts"]; !ok {
		t.Error("missing headline_parts in JSON")
	}

	// Verify highlight=nil serializes as null
	parts := parsed["headline_parts"].([]interface{})
	part1 := parts[1].(map[string]interface{})
	if part1["highlight"] != nil {
		t.Errorf("expected null highlight for plain text part, got %v", part1["highlight"])
	}
}

func TestHeadlinePartHighlightOmitNil(t *testing.T) {
	// Plain text part should have highlight: null in JSON (not omitted)
	part := HeadlinePart{Text: "hello", Highlight: nil}
	data, _ := json.Marshal(part)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	// The "highlight" key must be present (with null value) to match Python behavior
	if _, exists := m["highlight"]; !exists {
		t.Error("highlight key should be present in JSON even when nil")
	}
}
