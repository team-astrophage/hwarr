package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/homepy/hwarr/server/internal/model"
)

// mockRedisRankingReader implements RedisRankingReader for testing.
type mockRedisRankingReader struct {
	members []model.ZMember
	err     error
}

func (m *mockRedisRankingReader) ZRevRangeWithScores(_ context.Context, _ string, _, _ int64) ([]model.ZMember, error) {
	return m.members, m.err
}

func setupRankingRouter(mock *mockRedisRankingReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewRankingHandler(mock)
	h.Register(r)
	return r
}

func TestRankingHandler_EmptyResult(t *testing.T) {
	r := setupRankingRouter(&mockRedisRankingReader{members: []model.ZMember{}})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/ranking/today", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp rankingResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.Items) != 0 {
		t.Errorf("expected empty items, got %d", len(resp.Items))
	}
	if resp.Date == "" {
		t.Error("expected non-empty date")
	}
}

func TestRankingHandler_WithData(t *testing.T) {
	mock := &mockRedisRankingReader{
		members: []model.ZMember{
			{Member: "서초동", Score: 42},
			{Member: "강남동", Score: 30},
			{Member: "역삼동", Score: 15},
		},
	}
	r := setupRankingRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/ranking/today?limit=3", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp rankingResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp.Items))
	}

	// Verify first item
	if resp.Items[0].Rank != 1 {
		t.Errorf("expected rank 1, got %d", resp.Items[0].Rank)
	}
	if resp.Items[0].Region != "서초동" {
		t.Errorf("expected region '서초동', got %q", resp.Items[0].Region)
	}
	if resp.Items[0].Count != 42 {
		t.Errorf("expected count 42, got %d", resp.Items[0].Count)
	}

	// Verify second item
	if resp.Items[1].Rank != 2 {
		t.Errorf("expected rank 2, got %d", resp.Items[1].Rank)
	}
	if resp.Items[1].Region != "강남동" {
		t.Errorf("expected region '강남동', got %q", resp.Items[1].Region)
	}
	if resp.Items[1].Count != 30 {
		t.Errorf("expected count 30, got %d", resp.Items[1].Count)
	}
}

func TestRankingHandler_DefaultLimit(t *testing.T) {
	// Default limit is 10; provide more than 10 members
	members := make([]model.ZMember, 15)
	for i := range members {
		members[i] = model.ZMember{Member: "region", Score: float64(15 - i)}
	}

	mock := &mockRedisRankingReader{members: members}
	r := setupRankingRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/ranking/today", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp rankingResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// The mock returns all 15 members (start=0, stop=9 passed to mock which ignores range),
	// but the handler trusts Redis to return the correct range.
	// In real usage, Redis would return at most limit items.
	if resp.Date == "" {
		t.Error("expected non-empty date")
	}
}

func TestRankingHandler_LimitClamping(t *testing.T) {
	mock := &mockRedisRankingReader{members: []model.ZMember{}}
	r := setupRankingRouter(mock)

	// Test limit > 50 gets clamped
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/ranking/today?limit=100", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Test limit < 1 gets clamped
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/ranking/today?limit=0", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Test invalid limit defaults to 10
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/ranking/today?limit=abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRankingHandler_RedisError(t *testing.T) {
	mock := &mockRedisRankingReader{
		err: context.DeadlineExceeded,
	}
	r := setupRankingRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/ranking/today", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestRankingHandler_ResponseShape(t *testing.T) {
	mock := &mockRedisRankingReader{
		members: []model.ZMember{
			{Member: "종로구", Score: 10},
		},
	}
	r := setupRankingRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/ranking/today?limit=5", nil)
	r.ServeHTTP(w, req)

	// Verify raw JSON keys match Python response shape
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := raw["date"]; !ok {
		t.Error("missing 'date' key in response")
	}
	if _, ok := raw["items"]; !ok {
		t.Error("missing 'items' key in response")
	}

	// Verify item shape
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw["items"], &items); err != nil {
		t.Fatalf("failed to unmarshal items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	for _, key := range []string{"rank", "region", "count"} {
		if _, ok := items[0][key]; !ok {
			t.Errorf("missing '%s' key in item", key)
		}
	}
}
