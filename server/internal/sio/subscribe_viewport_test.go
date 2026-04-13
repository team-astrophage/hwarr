package sio

import (
	"context"
	"fmt"
	"testing"

	"github.com/homepy/hwarr/server/internal/model"
)

// mockRedisFireCounter implements RedisFireCounter for testing.
type mockRedisFireCounter struct {
	// counts maps Redis keys to their ZCOUNT results.
	counts map[string]int64
}

func (m *mockRedisFireCounter) ZCount(_ context.Context, key, _, _ string) (int64, error) {
	if count, ok := m.counts[key]; ok {
		return count, nil
	}
	return 0, nil
}

func TestGetActiveFireCount(t *testing.T) {
	redis := &mockRedisFireCounter{
		counts: map[string]int64{
			"fire:41667:115454": 5,
			"fire:41668:115455": 0,
		},
	}

	tests := []struct {
		gridID string
		want   int
	}{
		{"41667:115454", 5},
		{"41668:115455", 0},
		{"99999:99999", 0}, // non-existent grid
	}

	for _, tt := range tests {
		t.Run(tt.gridID, func(t *testing.T) {
			got, err := getActiveFireCount(context.Background(), redis, tt.gridID, 1000.0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("getActiveFireCount(%s) = %d, want %d", tt.gridID, got, tt.want)
			}
		})
	}
}

func TestBuildGridState(t *testing.T) {
	// Test with coordinates (activeCount=5 → stage 1, 불씨/ember)
	state := model.BuildGridState("41667:115454", 5, 37.5, 127.0)

	if state.GridID != "41667:115454" {
		t.Errorf("GridID = %v, want 41667:115454", state.GridID)
	}
	if state.ActiveCount != 5 {
		t.Errorf("ActiveCount = %v, want 5", state.ActiveCount)
	}
	if state.Stage != 1 {
		t.Errorf("Stage = %v, want 1", state.Stage)
	}
	if state.Lat != 37.5 {
		t.Errorf("Lat = %v, want 37.5", state.Lat)
	}
	if state.Lng != 127.0 {
		t.Errorf("Lng = %v, want 127.0", state.Lng)
	}
	if state.StageInfo.LabelKo != "불씨" {
		t.Errorf("StageInfo.LabelKo = %v, want 불씨", state.StageInfo.LabelKo)
	}
	if state.StageInfo.LabelEn != "ember" {
		t.Errorf("StageInfo.LabelEn = %v, want ember", state.StageInfo.LabelEn)
	}

	// 50 clicks → stage 3 (화재/fire)
	state2 := model.BuildGridState("41667:115454", 50, 37.5, 127.0)
	if state2.Stage != 3 {
		t.Errorf("Stage = %v, want 3", state2.Stage)
	}
}

func TestBuildGridStateStages(t *testing.T) {
	tests := []struct {
		count int
		stage int
		label string
	}{
		{0, 0, "없음"},
		{1, 1, "불씨"},
		{6, 2, "모닥불"},
		{24, 3, "화재"},
		{72, 4, "대화재"},
		{170, 5, "전소"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("count_%d", tt.count), func(t *testing.T) {
			state := model.BuildGridState("test:grid", tt.count, 0, 0)
			if state.Stage != tt.stage {
				t.Errorf("Stage = %v, want %d", state.Stage, tt.stage)
			}
			if state.StageInfo.LabelKo != tt.label {
				t.Errorf("LabelKo = %v, want %s", state.StageInfo.LabelKo, tt.label)
			}
		})
	}
}

func TestViewportGridIDsComputation(t *testing.T) {
	// A small viewport that should cover a few grid cells
	grids := computeViewportGridIDs(37.5005, 127.0015, 37.4995, 127.0005)
	if len(grids) == 0 {
		t.Error("expected at least one grid ID for valid viewport")
	}

	// Verify format: each grid should be "lat:lng"
	for _, g := range grids {
		if len(g) < 3 {
			t.Errorf("grid ID too short: %s", g)
		}
	}
}

func TestJoinViewportRoomsUpdatesManager(t *testing.T) {
	manager := NewConnectionManager(nil)
	manager.Add("test-sid", "user1")

	// Initially no rooms
	rooms := manager.GetRooms("test-sid")
	if len(rooms) != 0 {
		t.Errorf("expected 0 rooms initially, got %d", len(rooms))
	}

	// Set some rooms (simulating viewport subscription)
	newRooms := []string{"41667:115454", "41667:115455", "41668:115454"}
	manager.SetRooms("test-sid", newRooms)

	rooms = manager.GetRooms("test-sid")
	if len(rooms) != 3 {
		t.Errorf("expected 3 rooms after set, got %d", len(rooms))
	}

	// Replace rooms (simulating viewport change)
	newRooms2 := []string{"50000:60000", "50001:60000"}
	manager.SetRooms("test-sid", newRooms2)

	rooms = manager.GetRooms("test-sid")
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms after replacement, got %d", len(rooms))
	}
}

func TestGetActiveFireCountRedisKey(t *testing.T) {
	// Verify the Redis key format matches Python: "fire:{gridId}"
	redis := &mockRedisFireCounter{counts: map[string]int64{}}
	tracking := &trackingRedisFireCounter{inner: redis}

	_, _ = getActiveFireCount(context.Background(), tracking, "41667:115454", 1000.0)

	expectedKey := "fire:41667:115454"
	if tracking.lastKey != expectedKey {
		t.Errorf("Redis key = %s, want %s", tracking.lastKey, expectedKey)
	}
}

// trackingRedisFireCounter wraps a RedisFireCounter and tracks the last key queried.
type trackingRedisFireCounter struct {
	inner   RedisFireCounter
	lastKey string
}

func (tr *trackingRedisFireCounter) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	tr.lastKey = key
	return tr.inner.ZCount(ctx, key, min, max)
}

// computeViewportGridIDs is a test helper that mirrors grid.GetGridsInViewport
// without importing the grid package (to avoid any circular dependency concerns).
func computeViewportGridIDs(neLat, neLng, swLat, swLng float64) []string {
	const latUnit = 0.0009
	const lngUnit = 0.0011
	latMin := int(swLat / latUnit)
	latMax := int(neLat / latUnit)
	lngMin := int(swLng / lngUnit)
	lngMax := int(neLng / lngUnit)

	var grids []string
	for lat := latMin; lat <= latMax; lat++ {
		for lng := lngMin; lng <= lngMax; lng++ {
			grids = append(grids, fmt.Sprintf("%d:%d", lat, lng))
		}
	}
	return grids
}
