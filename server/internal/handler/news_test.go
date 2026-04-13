package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/homepy/hwarr/server/internal/config"
	"github.com/homepy/hwarr/server/internal/model"
)

// mockRedisNewsReader implements RedisNewsReader for testing.
type mockRedisNewsReader struct {
	members   map[string][]string        // key -> set members
	counts    map[string]int64           // key -> zcount result
	zrevrange map[string][]model.ZMember // key -> sorted set members
	err       error
}

func (m *mockRedisNewsReader) SMembers(_ context.Context, key string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.members[key], nil
}

func (m *mockRedisNewsReader) ZCount(_ context.Context, key, _, _ string) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.counts[key], nil
}

func (m *mockRedisNewsReader) ZRevRangeWithScores(_ context.Context, key string, _, _ int64) ([]model.ZMember, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.zrevrange[key], nil
}

func setupNewsRouter(redis RedisNewsReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewNewsHandler(redis)
	h.Register(r)
	return r
}

func TestGetNews_Empty(t *testing.T) {
	mock := &mockRedisNewsReader{
		members: map[string][]string{"active_grids": {}},
	}
	router := setupNewsRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/news", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var items []model.NewsItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestGetNews_SingleSmallFire(t *testing.T) {
	gridID := "41728:115381"
	fireKey := fmt.Sprintf("fire:%s", gridID)
	now := float64(1700000000)

	mock := &mockRedisNewsReader{
		members:   map[string][]string{"active_grids": {gridID}},
		counts:    map[string]int64{fireKey: 3},
		zrevrange: map[string][]model.ZMember{fireKey: {{Member: "user1", Score: now + config.FireTTLSec}}},
	}
	router := setupNewsRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/news", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var items []model.NewsItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.ID != fmt.Sprintf("news-sf-%s", gridID) {
		t.Errorf("unexpected ID: %s", item.ID)
	}
	if item.Icon != "🔥" {
		t.Errorf("unexpected icon: %s", item.Icon)
	}
	if item.IconBg != "rgba(255,140,0,0.12)" {
		t.Errorf("unexpected icon_bg: %s", item.IconBg)
	}
	// Stage 1 -> small fire -> "불씨" in headline
	if len(item.HeadlineParts) != 4 {
		t.Fatalf("expected 4 headline parts, got %d", len(item.HeadlineParts))
	}
	if item.HeadlineParts[2].Text != "불씨" {
		t.Errorf("expected '불씨', got %q", item.HeadlineParts[2].Text)
	}
}

func TestGetNews_MultipleFires_SortedByStageAndCount(t *testing.T) {
	grid1 := "41688:115472" // big fire
	grid2 := "41700:115400" // campfire
	grid3 := "41728:115381" // ember
	now := float64(1700000000)

	mock := &mockRedisNewsReader{
		members: map[string][]string{"active_grids": {grid3, grid1, grid2}},
		counts: map[string]int64{
			fmt.Sprintf("fire:%s", grid1): 150, // stage 4 (대화재)
			fmt.Sprintf("fire:%s", grid2): 15,  // stage 2 (모닥불)
			fmt.Sprintf("fire:%s", grid3): 5,   // stage 1 (불씨)
		},
		zrevrange: map[string][]model.ZMember{
			fmt.Sprintf("fire:%s", grid1): {{Score: now + config.FireTTLSec}},
			fmt.Sprintf("fire:%s", grid2): {{Score: now + config.FireTTLSec}},
			fmt.Sprintf("fire:%s", grid3): {{Score: now + config.FireTTLSec}},
		},
	}
	router := setupNewsRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/news", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var items []model.NewsItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Verify order: big fire (stage 4) > campfire (stage 2) > ember (stage 1)
	if items[0].ID != fmt.Sprintf("news-bf-%s", grid1) {
		t.Errorf("first item should be big fire, got %s", items[0].ID)
	}
	if items[1].ID != fmt.Sprintf("news-fa-%s", grid2) {
		t.Errorf("second item should be fire active, got %s", items[1].ID)
	}
	if items[2].ID != fmt.Sprintf("news-sf-%s", grid3) {
		t.Errorf("third item should be small fire, got %s", items[2].ID)
	}
}

func TestGetNews_MaxItems(t *testing.T) {
	// Create 7 grids — should return only 5
	gridIDs := make([]string, 7)
	counts := make(map[string]int64)
	zrev := make(map[string][]model.ZMember)
	now := float64(1700000000)

	for i := 0; i < 7; i++ {
		gid := fmt.Sprintf("4%d:115%d", 1700+i, 400+i)
		gridIDs[i] = gid
		key := fmt.Sprintf("fire:%s", gid)
		counts[key] = int64((i + 1) * 5) // 5, 10, 15, 20, 25, 30, 35
		zrev[key] = []model.ZMember{{Score: now + config.FireTTLSec}}
	}

	mock := &mockRedisNewsReader{
		members:   map[string][]string{"active_grids": gridIDs},
		counts:    counts,
		zrevrange: zrev,
	}
	router := setupNewsRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/news", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var items []model.NewsItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != MaxNewsItems {
		t.Errorf("expected %d items, got %d", MaxNewsItems, len(items))
	}
}

func TestGetNews_SkipsZeroCountGrids(t *testing.T) {
	grid1 := "41688:115472"
	grid2 := "41700:115400"
	now := float64(1700000000)

	mock := &mockRedisNewsReader{
		members: map[string][]string{"active_grids": {grid1, grid2}},
		counts: map[string]int64{
			fmt.Sprintf("fire:%s", grid1): 0,  // no active fires
			fmt.Sprintf("fire:%s", grid2): 10, // campfire
		},
		zrevrange: map[string][]model.ZMember{
			fmt.Sprintf("fire:%s", grid2): {{Score: now + config.FireTTLSec}},
		},
	}
	router := setupNewsRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/news", nil)
	router.ServeHTTP(w, req)

	var items []model.NewsItem
	json.Unmarshal(w.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Errorf("expected 1 item (skipping zero-count grid), got %d", len(items))
	}
}

func TestGetNews_RedisError(t *testing.T) {
	mock := &mockRedisNewsReader{err: errors.New("connection refused")}
	router := setupNewsRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/news", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetNews_BigFireTemplate(t *testing.T) {
	gridID := "41688:115472"
	fireKey := fmt.Sprintf("fire:%s", gridID)
	now := float64(1700000000)

	mock := &mockRedisNewsReader{
		members:   map[string][]string{"active_grids": {gridID}},
		counts:    map[string]int64{fireKey: 150}, // stage 4
		zrevrange: map[string][]model.ZMember{fireKey: {{Score: now + config.FireTTLSec}}},
	}
	router := setupNewsRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/news", nil)
	router.ServeHTTP(w, req)

	var items []model.NewsItem
	json.Unmarshal(w.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.IconBg != "rgba(255,68,68,0.15)" {
		t.Errorf("big fire should have red icon_bg, got %s", item.IconBg)
	}
	if item.HeadlineParts[1].Text != " 일대 " {
		t.Errorf("big fire should have ' 일대 ', got %q", item.HeadlineParts[1].Text)
	}
}

func TestGetNews_FireActiveTemplate(t *testing.T) {
	gridID := "41700:115400"
	fireKey := fmt.Sprintf("fire:%s", gridID)
	now := float64(1700000000)

	mock := &mockRedisNewsReader{
		members:   map[string][]string{"active_grids": {gridID}},
		counts:    map[string]int64{fireKey: 50}, // stage 3
		zrevrange: map[string][]model.ZMember{fireKey: {{Score: now + config.FireTTLSec}}},
	}
	router := setupNewsRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/news", nil)
	router.ServeHTTP(w, req)

	var items []model.NewsItem
	json.Unmarshal(w.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.IconBg != "rgba(255,140,0,0.2)" {
		t.Errorf("fire active should have orange icon_bg, got %s", item.IconBg)
	}
	if item.HeadlineParts[3].Text != " 확산 중" {
		t.Errorf("fire active should have ' 확산 중', got %q", item.HeadlineParts[3].Text)
	}
}

func TestGetNews_ResponseFormat(t *testing.T) {
	gridID := "41728:115381"
	fireKey := fmt.Sprintf("fire:%s", gridID)
	now := float64(1700000000)

	mock := &mockRedisNewsReader{
		members:   map[string][]string{"active_grids": {gridID}},
		counts:    map[string]int64{fireKey: 3},
		zrevrange: map[string][]model.ZMember{fireKey: {{Score: now + config.FireTTLSec}}},
	}
	router := setupNewsRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/news", nil)
	router.ServeHTTP(w, req)

	// Verify JSON field names match Python snake_case convention
	var raw []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 item, got %d", len(raw))
	}

	item := raw[0]
	requiredFields := []string{"id", "icon", "icon_bg", "headline_parts", "time", "detail"}
	for _, field := range requiredFields {
		if _, ok := item[field]; !ok {
			t.Errorf("missing required field %q in response", field)
		}
	}

	// Verify headline_parts structure
	parts := item["headline_parts"].([]interface{})
	for i, p := range parts {
		part := p.(map[string]interface{})
		if _, ok := part["text"]; !ok {
			t.Errorf("headline_parts[%d] missing 'text'", i)
		}
		if _, ok := part["highlight"]; !ok {
			t.Errorf("headline_parts[%d] missing 'highlight'", i)
		}
	}
}
