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
)

// mockRedisGridReader implements RedisGridReader for testing.
type mockRedisGridReader struct {
	counts map[string]int64
	err    error
}

func (m *mockRedisGridReader) ZCount(_ context.Context, key, _, _ string) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.counts[key], nil
}

func setupGridRouter(redis RedisGridReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewGridHandler(redis)
	h.Register(r)
	return r
}

func TestGetGridState_NoFires(t *testing.T) {
	mock := &mockRedisGridReader{counts: map[string]int64{}}
	router := setupGridRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/grid/10:20", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp["gridId"] != "10:20" {
		t.Errorf("expected grid_id=10:20, got %v", resp["gridId"])
	}
	if resp["activeCount"].(float64) != 0 {
		t.Errorf("expected active_count=0, got %v", resp["activeCount"])
	}
	if resp["stage"].(float64) != 0 {
		t.Errorf("expected stage=0, got %v", resp["stage"])
	}

	stageInfo := resp["stageInfo"].(map[string]interface{})
	if stageInfo["labelEn"] != "none" {
		t.Errorf("expected label_en=none, got %v", stageInfo["labelEn"])
	}
}

func TestGetGridState_WithFires(t *testing.T) {
	tests := []struct {
		name        string
		gridID      string
		count       int64
		wantStage   float64
		wantLabelEn string
	}{
		{"ember", "5:5", 3, 1, "ember"},
		{"campfire", "5:5", 15, 2, "campfire"},
		{"fire", "5:5", 50, 3, "fire"},
		{"big fire", "5:5", 150, 4, "big fire"},
		{"total burn", "5:5", 300, 5, "total burn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRedisGridReader{
				counts: map[string]int64{
					fmt.Sprintf("fire:%s", tt.gridID): tt.count,
				},
			}
			router := setupGridRouter(mock)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/grid/%s", tt.gridID), nil)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}

			if resp["activeCount"].(float64) != float64(tt.count) {
				t.Errorf("expected active_count=%d, got %v", tt.count, resp["activeCount"])
			}
			if resp["stage"].(float64) != tt.wantStage {
				t.Errorf("expected stage=%v, got %v", tt.wantStage, resp["stage"])
			}
			stageInfo := resp["stageInfo"].(map[string]interface{})
			if stageInfo["labelEn"] != tt.wantLabelEn {
				t.Errorf("expected label_en=%s, got %v", tt.wantLabelEn, stageInfo["labelEn"])
			}
		})
	}
}

func TestGetGridState_RedisError(t *testing.T) {
	mock := &mockRedisGridReader{err: errors.New("connection refused")}
	router := setupGridRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/grid/1:1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetViewportFires_NoFires(t *testing.T) {
	mock := &mockRedisGridReader{counts: map[string]int64{}}
	router := setupGridRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/grid/viewport?neLat=37.6&neLng=127.1&swLat=37.5&swLng=126.9", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	grids := resp["grids"].([]interface{})
	if len(grids) != 0 {
		t.Errorf("expected 0 active grids, got %d", len(grids))
	}
	if resp["totalActiveGrids"].(float64) != 0 {
		t.Errorf("expected total_active_grids=0, got %v", resp["totalActiveGrids"])
	}
}

func TestGetViewportFires_WithFires(t *testing.T) {
	mock := &mockRedisGridReader{
		counts: map[string]int64{
			"fire:41666:115363": 5,
			"fire:41667:115363": 50,
		},
	}
	router := setupGridRouter(mock)

	// Viewport that covers grid cells 41666:115363 and 41667:115363
	// 41666 * 0.0009 = 37.4994, 41667 * 0.0009 = 37.50003
	// 115363 * 0.0011 = 126.8993
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET",
		"/api/grid/viewport?swLat=37.4994&swLng=126.8993&neLat=37.5010&neLng=126.9005", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	totalActive := int(resp["totalActiveGrids"].(float64))
	grids := resp["grids"].([]interface{})

	if totalActive != len(grids) {
		t.Errorf("total_active_grids=%d does not match grids length=%d", totalActive, len(grids))
	}

	// All returned grids should have active_count > 0
	for _, g := range grids {
		gm := g.(map[string]interface{})
		if gm["activeCount"].(float64) <= 0 {
			t.Errorf("returned grid with active_count <= 0: %v", gm)
		}
		// Should have lat/lng
		if _, ok := gm["lat"]; !ok {
			t.Error("missing lat in viewport grid response")
		}
		if _, ok := gm["lng"]; !ok {
			t.Error("missing lng in viewport grid response")
		}
		// Should have stage_info
		if _, ok := gm["stageInfo"]; !ok {
			t.Error("missing stage_info in viewport grid response")
		}
	}
}

func TestGetViewportFires_MissingParams(t *testing.T) {
	mock := &mockRedisGridReader{counts: map[string]int64{}}
	router := setupGridRouter(mock)

	tests := []struct {
		name string
		url  string
	}{
		{"missing neLat", "/api/grid/viewport?neLng=127&swLat=37&swLng=126"},
		{"missing neLng", "/api/grid/viewport?neLat=38&swLat=37&swLng=126"},
		{"missing swLat", "/api/grid/viewport?neLat=38&neLng=127&swLng=126"},
		{"missing swLng", "/api/grid/viewport?neLat=38&neLng=127&swLat=37"},
		{"invalid neLat", "/api/grid/viewport?neLat=abc&neLng=127&swLat=37&swLng=126"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.url, nil)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("expected 422, got %d", w.Code)
			}
		})
	}
}

func TestGetViewportFires_RedisError(t *testing.T) {
	mock := &mockRedisGridReader{err: errors.New("connection refused")}
	router := setupGridRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET",
		"/api/grid/viewport?neLat=37.501&neLng=126.901&swLat=37.500&swLng=126.900", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetViewportFires_ResponseFormat(t *testing.T) {
	// Verify response matches Python ViewportResponse format
	mock := &mockRedisGridReader{
		counts: map[string]int64{
			"fire:41666:115363": 150, // big fire
		},
	}
	router := setupGridRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET",
		"/api/grid/viewport?swLat=37.4994&swLng=126.8993&neLat=37.4996&neLng=126.8995", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp viewportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.TotalActiveGrids != len(resp.Grids) {
		t.Errorf("totalActiveGrids mismatch: %d vs %d", resp.TotalActiveGrids, len(resp.Grids))
	}

	for _, g := range resp.Grids {
		if g.GridID == "" {
			t.Error("grid_id should not be empty")
		}
		if g.Lat == 0 && g.Lng == 0 {
			t.Error("viewport grids should have lat/lng")
		}
		if g.StageInfo.LabelKo == "" || g.StageInfo.LabelEn == "" {
			t.Error("stage_info should have labels")
		}
	}
}

func TestGetGridState_StageThresholdBoundaries(t *testing.T) {
	// Test exact threshold boundaries matching Python implementation
	tests := []struct {
		count     int64
		wantStage float64
	}{
		{0, 0},   // none
		{1, 1},   // ember threshold
		{9, 1},   // still ember
		{10, 2},  // campfire threshold
		{39, 2},  // still campfire
		{40, 3},  // fire threshold
		{119, 3}, // still fire
		{120, 4}, // big fire threshold
		{279, 4}, // still big fire
		{280, 5}, // total burn threshold
		{500, 5}, // still total burn
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("count_%d", tt.count), func(t *testing.T) {
			mock := &mockRedisGridReader{
				counts: map[string]int64{"fire:1:1": tt.count},
			}
			router := setupGridRouter(mock)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/grid/1:1", nil)
			router.ServeHTTP(w, req)

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)

			if resp["stage"].(float64) != tt.wantStage {
				t.Errorf("count=%d: expected stage=%v, got %v", tt.count, tt.wantStage, resp["stage"])
			}
		})
	}
}
