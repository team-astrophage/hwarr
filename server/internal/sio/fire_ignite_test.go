package sio

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/homepy/hwarr/server/internal/model"
)

// mockRedisFireWriter is a test double for RedisFireWriter.
type mockRedisFireWriter struct {
	mu        sync.Mutex
	sortedSet map[string]map[string]float64 // key -> {member: score}
	sets      map[string]map[string]bool    // key -> {member: true}
	counters  map[string]int64              // key -> counter
	zscores   map[string]map[string]float64 // key -> {member: score} for ranking
}

func newMockRedisFireWriter() *mockRedisFireWriter {
	return &mockRedisFireWriter{
		sortedSet: make(map[string]map[string]float64),
		sets:      make(map[string]map[string]bool),
		counters:  make(map[string]int64),
		zscores:   make(map[string]map[string]float64),
	}
}

func (m *mockRedisFireWriter) ZCount(_ context.Context, key, min, _ string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ss, ok := m.sortedSet[key]
	if !ok {
		return 0, nil
	}
	var minF float64
	fmt.Sscanf(min, "%f", &minF)
	var count int64
	for _, score := range ss {
		if score >= minF {
			count++
		}
	}
	return count, nil
}

func (m *mockRedisFireWriter) ZAdd(_ context.Context, key string, score float64, member string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sortedSet[key] == nil {
		m.sortedSet[key] = make(map[string]float64)
	}
	m.sortedSet[key][member] = score
	return nil
}

func (m *mockRedisFireWriter) SAdd(_ context.Context, key string, member string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sets[key] == nil {
		m.sets[key] = make(map[string]bool)
	}
	m.sets[key][member] = true
	return nil
}

func (m *mockRedisFireWriter) Incr(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[key]++
	return nil
}

func (m *mockRedisFireWriter) ZIncrBy(_ context.Context, key string, _ float64, member string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.zscores[key] == nil {
		m.zscores[key] = make(map[string]float64)
	}
	m.zscores[key][member]++
	return nil
}

func (m *mockRedisFireWriter) Expire(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func TestRegisterFireEvent_BasicRegistration(t *testing.T) {
	redis := newMockRedisFireWriter()
	ctx := context.Background()

	reg, err := registerFireEvent(ctx, redis, "41733:115426", "fire-abc123", 9999999999.0, nil, nil)
	if err != nil {
		t.Fatalf("registerFireEvent failed: %v", err)
	}

	// Should land on the requested grid (no spreading since threshold not met)
	if reg.GridID != "41733:115426" {
		t.Errorf("expected grid_id=41733:115426, got %s", reg.GridID)
	}
	if reg.ActiveCount != 1 {
		t.Errorf("expected active_count=1, got %d", reg.ActiveCount)
	}
	if len(reg.SpreadPath) != 0 {
		t.Errorf("expected no spread, got %d hops", len(reg.SpreadPath))
	}

	// Verify Redis state
	if !redis.sets["active_grids"]["41733:115426"] {
		t.Error("expected grid to be in active_grids set")
	}
	if redis.counters["stats:total_fires"] != 1 {
		t.Errorf("expected stats:total_fires=1, got %d", redis.counters["stats:total_fires"])
	}
}

func TestRegisterFireEvent_SpreadOnThreshold(t *testing.T) {
	redis := newMockRedisFireWriter()
	ctx := context.Background()

	// Pre-populate a grid with 500 fires to trigger spreading
	gridKey := "fire:41733:115426"
	redis.sortedSet[gridKey] = make(map[string]float64)
	for i := 0; i < 500; i++ {
		redis.sortedSet[gridKey][fmt.Sprintf("fire-%d", i)] = 9999999999.0
	}

	reg, err := registerFireEvent(ctx, redis, "41733:115426", "fire-spread-test", 9999999999.0, nil, nil)
	if err != nil {
		t.Fatalf("registerFireEvent failed: %v", err)
	}

	// Should have spread to a neighbor
	if reg.GridID == "41733:115426" {
		// Very unlikely but possible if random neighbor also at threshold
		// Just check that spread was attempted
		t.Log("Note: fire may have landed back on same grid if all neighbors at threshold")
	}
	if len(reg.SpreadPath) == 0 {
		t.Error("expected at least one spread hop")
	}
}

func TestFireIgniteData_Parsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLat *float64
		wantLng *float64
		wantDemo bool
	}{
		{
			name:     "full coordinates",
			input:    `{"lat": 37.5760, "lng": 126.9769}`,
			wantLat:  ptrFloat(37.5760),
			wantLng:  ptrFloat(126.9769),
			wantDemo: false,
		},
		{
			name:     "demo mode without coords",
			input:    `{"demo": true}`,
			wantLat:  nil,
			wantLng:  nil,
			wantDemo: true,
		},
		{
			name:     "demo mode with coords",
			input:    `{"lat": 37.5760, "lng": 126.9769, "demo": true}`,
			wantLat:  ptrFloat(37.5760),
			wantLng:  ptrFloat(126.9769),
			wantDemo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data fireIgniteData
			if err := json.Unmarshal([]byte(tt.input), &data); err != nil {
				t.Fatalf("failed to parse: %v", err)
			}
			if tt.wantLat == nil && data.Lat != nil {
				t.Error("expected nil lat")
			}
			if tt.wantLat != nil && (data.Lat == nil || *data.Lat != *tt.wantLat) {
				t.Errorf("lat mismatch: want %v, got %v", tt.wantLat, data.Lat)
			}
			if tt.wantLng == nil && data.Lng != nil {
				t.Error("expected nil lng")
			}
			if tt.wantLng != nil && (data.Lng == nil || *data.Lng != *tt.wantLng) {
				t.Errorf("lng mismatch: want %v, got %v", tt.wantLng, data.Lng)
			}
			if data.Demo != tt.wantDemo {
				t.Errorf("demo mismatch: want %v, got %v", tt.wantDemo, data.Demo)
			}
		})
	}
}

func TestPredefinedDemoLocations(t *testing.T) {
	if len(predefinedDemoLocations) != 16 {
		t.Errorf("expected 16 predefined locations, got %d", len(predefinedDemoLocations))
	}

	// Verify first and last entries
	if predefinedDemoLocations[0].ID != "gwanghwamun" {
		t.Errorf("first location should be gwanghwamun, got %s", predefinedDemoLocations[0].ID)
	}
	if predefinedDemoLocations[15].ID != "coex" {
		t.Errorf("last location should be coex, got %s", predefinedDemoLocations[15].ID)
	}

	// Verify all locations have valid coordinates
	for _, loc := range predefinedDemoLocations {
		if loc.Lat == 0 || loc.Lng == 0 {
			t.Errorf("location %s has zero coordinates", loc.ID)
		}
		if loc.Name == "" {
			t.Errorf("location %s has empty name", loc.ID)
		}
	}
}

func TestRandomHexSIO(t *testing.T) {
	hex := randomHexSIO(12)
	if len(hex) != 12 {
		t.Errorf("expected length 12, got %d", len(hex))
	}
	for _, c := range hex {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("invalid hex char: %c", c)
		}
	}
}

func TestGridStateInfoToMap(t *testing.T) {
	info := model.FireStageInfo{
		Stage:               3,
		LabelKo:            "화재",
		LabelEn:            "fire",
		TriggersFirefighter: false,
	}
	m := gridStateInfoToMap(info)
	if m["stage"] != 3 {
		t.Errorf("stage mismatch: got %v", m["stage"])
	}
	if m["label_ko"] != "화재" {
		t.Errorf("label_ko mismatch: got %v", m["label_ko"])
	}
	if m["triggers_firefighter"] != false {
		t.Errorf("triggers_firefighter should be false")
	}
}

func ptrFloat(f float64) *float64 {
	return &f
}
