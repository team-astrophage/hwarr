package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/homepy/hwarr/server-go/internal/grid"
)

func TestMapConfigHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewMapConfigHandler()
	h.Register(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/map/config", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var cfg MapConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify center coordinates
	if cfg.CenterLat != 36.5 {
		t.Errorf("center_lat = %f, want 36.5", cfg.CenterLat)
	}
	if cfg.CenterLng != 127.8 {
		t.Errorf("center_lng = %f, want 127.8", cfg.CenterLng)
	}

	// Verify zoom levels
	if cfg.DefaultZoom != 7 {
		t.Errorf("default_zoom = %d, want 7", cfg.DefaultZoom)
	}
	if cfg.MinZoom != 6 {
		t.Errorf("min_zoom = %d, want 6", cfg.MinZoom)
	}
	if cfg.MaxZoom != 18 {
		t.Errorf("max_zoom = %d, want 18", cfg.MaxZoom)
	}

	// Verify bounds
	if cfg.Bounds.NELat != 38.6 || cfg.Bounds.NELng != 131.9 {
		t.Errorf("NE bounds = (%f, %f), want (38.6, 131.9)", cfg.Bounds.NELat, cfg.Bounds.NELng)
	}
	if cfg.Bounds.SWLat != 33.0 || cfg.Bounds.SWLng != 124.5 {
		t.Errorf("SW bounds = (%f, %f), want (33.0, 124.5)", cfg.Bounds.SWLat, cfg.Bounds.SWLng)
	}

	// Verify grid config
	if cfg.Grid.LatUnit != grid.LatUnit {
		t.Errorf("lat_unit = %f, want %f", cfg.Grid.LatUnit, grid.LatUnit)
	}
	if cfg.Grid.LngUnit != grid.LngUnit {
		t.Errorf("lng_unit = %f, want %f", cfg.Grid.LngUnit, grid.LngUnit)
	}
	if cfg.Grid.CellSizeMeters != 100 {
		t.Errorf("cell_size_meters = %d, want 100", cfg.Grid.CellSizeMeters)
	}

	// Verify locations
	if len(cfg.Locations) != 16 {
		t.Fatalf("expected 16 locations, got %d", len(cfg.Locations))
	}

	// Verify all IDs are unique
	idSet := make(map[string]bool)
	for _, loc := range cfg.Locations {
		if idSet[loc.ID] {
			t.Errorf("duplicate location ID: %s", loc.ID)
		}
		idSet[loc.ID] = true
	}

	// Verify grid_id is correctly computed for each location
	for _, loc := range cfg.Locations {
		expectedGridID := grid.ToGridID(loc.Lat, loc.Lng)
		if loc.GridID != expectedGridID {
			t.Errorf("location %s: grid_id = %s, want %s", loc.ID, loc.GridID, expectedGridID)
		}
	}

	// Verify all coordinates are within South Korea bounds
	for _, loc := range cfg.Locations {
		if loc.Lat < cfg.Bounds.SWLat || loc.Lat > cfg.Bounds.NELat {
			t.Errorf("location %s lat %f out of bounds", loc.ID, loc.Lat)
		}
		if loc.Lng < cfg.Bounds.SWLng || loc.Lng > cfg.Bounds.NELng {
			t.Errorf("location %s lng %f out of bounds", loc.ID, loc.Lng)
		}
	}

	// Verify specific known locations exist
	expectedIDs := []string{"gwanghwamun", "gangnam", "busan_haeundae", "coex"}
	for _, expectedID := range expectedIDs {
		if !idSet[expectedID] {
			t.Errorf("expected location %s not found", expectedID)
		}
	}
}

func TestMapConfigLocationsHaveNonEmptyFields(t *testing.T) {
	for _, loc := range mapConfigInstance.Locations {
		if loc.ID == "" {
			t.Error("found location with empty ID")
		}
		if loc.Name == "" {
			t.Errorf("location %s has empty name", loc.ID)
		}
		if loc.GridID == "" {
			t.Errorf("location %s has empty grid_id", loc.ID)
		}
		if loc.Lat == 0 || loc.Lng == 0 {
			t.Errorf("location %s has zero coordinates", loc.ID)
		}
	}
}
