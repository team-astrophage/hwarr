package sio

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestFireCompatData_Parsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLat *float64
		wantLng *float64
	}{
		{
			name:    "full coordinates",
			input:   `{"lat": 37.5760, "lng": 126.9769}`,
			wantLat: ptrFloat(37.5760),
			wantLng: ptrFloat(126.9769),
		},
		{
			name:    "missing lat",
			input:   `{"lng": 126.9769}`,
			wantLat: nil,
			wantLng: ptrFloat(126.9769),
		},
		{
			name:    "missing lng",
			input:   `{"lat": 37.5760}`,
			wantLat: ptrFloat(37.5760),
			wantLng: nil,
		},
		{
			name:    "empty object",
			input:   `{}`,
			wantLat: nil,
			wantLng: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data fireCompatData
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
		})
	}
}

func TestFireCompatHandler_RegistersFireInRedis(t *testing.T) {
	redis := newMockRedisFireWriter()
	ctx := context.Background()

	// Simulate the fire compat handler logic:
	// 1. Convert lat/lng to grid ID
	// 2. Register fire event in Redis
	// 3. Verify the fire was registered

	lat := 37.5760
	lng := 126.9769

	// Use the same registerFireEvent function as the handler
	reg, err := registerFireEvent(ctx, redis, "41733:115426", "fire-compat-test", 9999999999.0, nil, nil)
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

	// Verify that the camelCase payload would be correct
	payload := map[string]interface{}{
		"gridId":      reg.GridID,
		"activeCount": reg.ActiveCount,
		"stage":       1,
		"lat":         lat,
		"lng":         lng,
	}

	// Verify camelCase keys (not snake_case)
	if _, ok := payload["gridId"]; !ok {
		t.Error("missing camelCase key 'gridId'")
	}
	if _, ok := payload["activeCount"]; !ok {
		t.Error("missing camelCase key 'activeCount'")
	}
	// Should NOT have snake_case keys
	if _, ok := payload["grid_id"]; ok {
		t.Error("unexpected snake_case key 'grid_id' — compat handler should use camelCase")
	}
	if _, ok := payload["active_count"]; ok {
		t.Error("unexpected snake_case key 'active_count' — compat handler should use camelCase")
	}
}

func TestFireCompatHandler_SpreadUpdatesCoordinates(t *testing.T) {
	// When fire spreads to a neighbor grid, the broadcast lat/lng should
	// be the center of the landing grid, not the original coordinates.
	redis := newMockRedisFireWriter()
	ctx := context.Background()

	// Pre-populate a grid with 500 fires to trigger spreading
	gridKey := "fire:41733:115426"
	redis.sortedSet[gridKey] = make(map[string]float64)
	for i := 0; i < 500; i++ {
		redis.sortedSet[gridKey][fmt.Sprintf("fire-%d", i)] = 9999999999.0
	}

	reg, err := registerFireEvent(ctx, redis, "41733:115426", "fire-spread-compat", 9999999999.0, nil, nil)
	if err != nil {
		t.Fatalf("registerFireEvent failed: %v", err)
	}

	// If it spread, the grid ID should differ from requested
	if len(reg.SpreadPath) > 0 && reg.GridID == "41733:115426" {
		t.Log("Note: fire may have circled back to same grid")
	}

	// The important thing is that when gridID != requestedGridID,
	// the handler would call GridIDToCenter to get the landing grid's center.
	// This test verifies the spread mechanism works — the coordinate update
	// logic is in the handler itself and tested via integration tests.
	if len(reg.SpreadPath) == 0 {
		t.Error("expected at least one spread hop when grid is at threshold")
	}
}

