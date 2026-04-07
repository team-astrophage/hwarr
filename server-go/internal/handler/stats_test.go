package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockStatsRedis implements RedisStatsReader for testing.
type mockStatsRedis struct {
	members map[string][]string
	zcount  map[string]int64
	values  map[string]string
}

func newMockStatsRedis() *mockStatsRedis {
	return &mockStatsRedis{
		members: make(map[string][]string),
		zcount:  make(map[string]int64),
		values:  make(map[string]string),
	}
}

func (m *mockStatsRedis) SMembers(_ context.Context, key string) ([]string, error) {
	if v, ok := m.members[key]; ok {
		return v, nil
	}
	return nil, nil
}

func (m *mockStatsRedis) ZCount(_ context.Context, key, _, _ string) (int64, error) {
	if v, ok := m.zcount[key]; ok {
		return v, nil
	}
	return 0, nil
}

func (m *mockStatsRedis) Get(_ context.Context, key string) (string, error) {
	if v, ok := m.values[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("key not found")
}

// mockCounter implements ConnectionCounter for testing.
type mockCounter struct {
	count int
}

func (m *mockCounter) ActiveCount() int {
	return m.count
}

func TestStatsHandler_Handle_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redis := newMockStatsRedis()
	h := NewStatsHandler(redis, &mockCounter{count: 0})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/stats", nil)

	h.Handle(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp statsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.ActiveGrids != 0 {
		t.Errorf("expected activeGrids=0, got %d", resp.ActiveGrids)
	}
	if resp.TotalFires != 0 {
		t.Errorf("expected totalFires=0, got %d", resp.TotalFires)
	}
	if resp.CumulativeFires != 0 {
		t.Errorf("expected cumulativeFires=0, got %d", resp.CumulativeFires)
	}
	if resp.DailyFires != 0 {
		t.Errorf("expected dailyFires=0, got %d", resp.DailyFires)
	}
	if resp.OnlineUsers != 0 {
		t.Errorf("expected onlineUsers=0, got %d", resp.OnlineUsers)
	}
}

func TestStatsHandler_Handle_WithData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redis := newMockStatsRedis()

	// Setup mock data
	redis.members["active_grids"] = []string{"grid_1", "grid_2", "grid_3"}
	redis.zcount["fire:grid_1"] = 5
	redis.zcount["fire:grid_2"] = 10
	// grid_3 has 0 active fires (expired)
	redis.values["stats:total_fires"] = "42"
	redis.values["stats:daily_fires:2026-04-07"] = "15"

	h := NewStatsHandler(redis, &mockCounter{count: 7})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/stats", nil)

	h.Handle(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp statsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.ActiveGrids != 2 {
		t.Errorf("expected activeGrids=2, got %d", resp.ActiveGrids)
	}
	if resp.TotalFires != 15 {
		t.Errorf("expected totalFires=15, got %d", resp.TotalFires)
	}
	if resp.CumulativeFires != 42 {
		t.Errorf("expected cumulativeFires=42, got %d", resp.CumulativeFires)
	}
	if resp.OnlineUsers != 7 {
		t.Errorf("expected onlineUsers=7, got %d", resp.OnlineUsers)
	}
}

func TestStatsHandler_Handle_NilConnections(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redis := newMockStatsRedis()
	h := NewStatsHandler(redis, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/stats", nil)

	h.Handle(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp statsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.OnlineUsers != 0 {
		t.Errorf("expected onlineUsers=0 with nil connections, got %d", resp.OnlineUsers)
	}
}

func TestStatsHandler_Register(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redis := newMockStatsRedis()
	h := NewStatsHandler(redis, nil)

	r := gin.New()
	h.Register(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from registered route, got %d", w.Code)
	}
}

func TestStatsHandler_ResponseFormat_CamelCase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redis := newMockStatsRedis()
	h := NewStatsHandler(redis, &mockCounter{count: 3})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/stats", nil)

	h.Handle(c)

	// Verify camelCase keys in raw JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal raw: %v", err)
	}

	expectedKeys := []string{"activeGrids", "totalFires", "cumulativeFires", "dailyFires", "onlineUsers"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected key %q in response, got keys: %v", key, raw)
		}
	}
}
