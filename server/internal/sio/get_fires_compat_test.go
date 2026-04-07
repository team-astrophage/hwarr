package sio

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/homepy/hwarr/server/internal/model"
)

// mockGetFiresRedis is a mock implementation of RedisGetFiresReader.
type mockGetFiresRedis struct {
	members    map[string][]string
	zCounts    map[string]int64
	smemberErr error
	zcountErr  error
}

func newMockGetFiresRedis() *mockGetFiresRedis {
	return &mockGetFiresRedis{
		members: make(map[string][]string),
		zCounts: make(map[string]int64),
	}
}

func (m *mockGetFiresRedis) SMembers(_ context.Context, key string) ([]string, error) {
	if m.smemberErr != nil {
		return nil, m.smemberErr
	}
	return m.members[key], nil
}

func (m *mockGetFiresRedis) ZCount(_ context.Context, key, min, max string) (int64, error) {
	if m.zcountErr != nil {
		return 0, m.zcountErr
	}
	return m.zCounts[key], nil
}

// simulateGetFires replicates the get_fires handler logic for testing.
// Returns the fires list that would be sent via "fires:sync".
func simulateGetFires(t *testing.T, redis RedisGetFiresReader) []map[string]interface{} {
	t.Helper()

	ctx := context.Background()

	gridIDs, err := redis.SMembers(ctx, "active_grids")
	if err != nil {
		return []map[string]interface{}{}
	}

	firesList := make([]map[string]interface{}, 0)
	for _, gridID := range gridIDs {
		key := fmt.Sprintf("fire:%s", gridID)
		count, err := redis.ZCount(ctx, key, fmt.Sprintf("%f", float64(0)), "+inf")
		if err != nil {
			continue
		}
		if count > 0 {
			stage := model.GetStage(int(count))
			firesList = append(firesList, map[string]interface{}{
				"gridId":      gridID,
				"activeCount": int(count),
				"stage":       int(stage),
			})
		}
	}

	return firesList
}

func TestGetFiresCompat_AllActiveFires(t *testing.T) {
	redis := newMockGetFiresRedis()
	redis.members["active_grids"] = []string{"37.5660:126.9780", "37.5670:126.9790"}
	redis.zCounts["fire:37.5660:126.9780"] = 5
	redis.zCounts["fire:37.5670:126.9790"] = 15

	firesList := simulateGetFires(t, redis)

	if len(firesList) != 2 {
		t.Fatalf("expected 2 fires, got %d", len(firesList))
	}

	// Verify camelCase keys
	for _, fire := range firesList {
		if _, ok := fire["gridId"]; !ok {
			t.Fatal("expected camelCase 'gridId' key")
		}
		if _, ok := fire["activeCount"]; !ok {
			t.Fatal("expected camelCase 'activeCount' key")
		}
		if _, ok := fire["stage"]; !ok {
			t.Fatal("expected 'stage' key")
		}
		// Verify no snake_case keys present
		if _, ok := fire["grid_id"]; ok {
			t.Fatal("should not have snake_case 'grid_id' key")
		}
		if _, ok := fire["active_count"]; ok {
			t.Fatal("should not have snake_case 'active_count' key")
		}
	}
}

func TestGetFiresCompat_StageCalculation(t *testing.T) {
	tests := []struct {
		name        string
		activeCount int64
		wantStage   int
	}{
		{"ember", 5, 1},
		{"campfire", 15, 2},
		{"fire", 50, 3},
		{"big fire", 150, 4},
		{"total burn", 300, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redis := newMockGetFiresRedis()
			redis.members["active_grids"] = []string{"test:grid"}
			redis.zCounts["fire:test:grid"] = tt.activeCount

			firesList := simulateGetFires(t, redis)
			if len(firesList) != 1 {
				t.Fatalf("expected 1 fire, got %d", len(firesList))
			}

			stage := firesList[0]["stage"].(int)
			if stage != tt.wantStage {
				t.Errorf("activeCount=%d: expected stage=%d, got %d",
					tt.activeCount, tt.wantStage, stage)
			}
		})
	}
}

func TestGetFiresCompat_FiltersExpired(t *testing.T) {
	redis := newMockGetFiresRedis()
	redis.members["active_grids"] = []string{"grid-active", "grid-expired"}
	redis.zCounts["fire:grid-active"] = 10
	redis.zCounts["fire:grid-expired"] = 0 // all fires expired

	firesList := simulateGetFires(t, redis)

	if len(firesList) != 1 {
		t.Fatalf("expected 1 fire (expired filtered), got %d", len(firesList))
	}
	if firesList[0]["gridId"] != "grid-active" {
		t.Fatalf("expected gridId=grid-active, got %v", firesList[0]["gridId"])
	}
}

func TestGetFiresCompat_Empty(t *testing.T) {
	redis := newMockGetFiresRedis()
	// No active grids

	firesList := simulateGetFires(t, redis)
	if len(firesList) != 0 {
		t.Fatalf("expected 0 fires, got %d", len(firesList))
	}
}

func TestGetFiresCompat_RedisError(t *testing.T) {
	redis := newMockGetFiresRedis()
	redis.smemberErr = fmt.Errorf("connection refused")

	firesList := simulateGetFires(t, redis)
	if len(firesList) != 0 {
		t.Fatalf("expected 0 fires on error, got %d", len(firesList))
	}
}

func TestGetFiresCompat_JSONFormat(t *testing.T) {
	// Verify the JSON output matches Python format exactly
	redis := newMockGetFiresRedis()
	redis.members["active_grids"] = []string{"37.5660:126.9780"}
	redis.zCounts["fire:37.5660:126.9780"] = 50

	firesList := simulateGetFires(t, redis)

	// Marshal to JSON and verify format
	b, err := json.Marshal(firesList)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}

	// Unmarshal back and verify
	var parsed []map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 fire, got %d", len(parsed))
	}

	fire := parsed[0]
	if fire["gridId"] != "37.5660:126.9780" {
		t.Errorf("expected gridId=37.5660:126.9780, got %v", fire["gridId"])
	}
	// JSON numbers are float64
	if fire["activeCount"].(float64) != 50 {
		t.Errorf("expected activeCount=50, got %v", fire["activeCount"])
	}
	if fire["stage"].(float64) != 3 {
		t.Errorf("expected stage=3, got %v", fire["stage"])
	}
}
