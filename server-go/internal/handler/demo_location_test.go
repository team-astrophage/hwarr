package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupDemoLocationRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewDemoLocationHandler()
	h.Register(r)
	return r
}

func TestDemoLocationRandom(t *testing.T) {
	r := setupDemoLocationRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/demo/location", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp DemoLocationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID == "" {
		t.Error("expected non-empty ID")
	}
	if resp.Name == "" {
		t.Error("expected non-empty Name")
	}
	if resp.Lat == 0 || resp.Lng == 0 {
		t.Error("expected non-zero coordinates")
	}
	if resp.GridID == "" {
		t.Error("expected non-empty GridID")
	}
	if !resp.Demo {
		t.Error("expected demo=true")
	}
}

func TestDemoLocationByID(t *testing.T) {
	r := setupDemoLocationRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/demo/location?location_id=gangnam", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp DemoLocationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != "gangnam" {
		t.Errorf("expected ID=gangnam, got %s", resp.ID)
	}
	if resp.Name != "강남역" {
		t.Errorf("expected Name=강남역, got %s", resp.Name)
	}
}

func TestDemoLocationNotFound(t *testing.T) {
	r := setupDemoLocationRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/demo/location?location_id=nonexistent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	detail, ok := body["detail"].(string)
	if !ok || detail == "" {
		t.Error("expected non-empty detail field in error response")
	}
}

func TestDemoLocationsList(t *testing.T) {
	r := setupDemoLocationRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/demo/locations", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp DemoLocationsListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != len(predefinedLocations) {
		t.Errorf("expected total=%d, got %d", len(predefinedLocations), resp.Total)
	}
	if len(resp.Locations) != len(predefinedLocations) {
		t.Errorf("expected %d locations, got %d", len(predefinedLocations), len(resp.Locations))
	}
	if !resp.Demo {
		t.Error("expected demo=true")
	}

	// Verify each location has required fields
	for _, loc := range resp.Locations {
		if loc.ID == "" {
			t.Error("expected non-empty ID")
		}
		if loc.Name == "" {
			t.Error("expected non-empty Name")
		}
		if loc.Lat == 0 || loc.Lng == 0 {
			t.Errorf("expected non-zero coordinates for %s", loc.ID)
		}
		if loc.GridID == "" {
			t.Errorf("expected non-empty GridID for %s", loc.ID)
		}
		if !loc.Demo {
			t.Errorf("expected demo=true for %s", loc.ID)
		}
	}

	// Verify known location is in the list
	found := false
	for _, loc := range resp.Locations {
		if loc.ID == "gangnam" {
			found = true
			if loc.Name != "강남역" {
				t.Errorf("expected Name=강남역, got %s", loc.Name)
			}
			break
		}
	}
	if !found {
		t.Error("expected gangnam to be in locations list")
	}
}

func TestDemoLocationsListMatchesPredefined(t *testing.T) {
	r := setupDemoLocationRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/demo/locations", nil)
	r.ServeHTTP(w, req)

	var resp DemoLocationsListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify total matches locations length
	if resp.Total != len(resp.Locations) {
		t.Errorf("total (%d) should match locations length (%d)", resp.Total, len(resp.Locations))
	}

	// Verify all predefined IDs are present
	idSet := make(map[string]bool)
	for _, loc := range resp.Locations {
		idSet[loc.ID] = true
	}
	for _, loc := range predefinedLocations {
		if !idSet[loc.ID] {
			t.Errorf("predefined location %s missing from response", loc.ID)
		}
	}
}

func TestDemoLocationResponseFields(t *testing.T) {
	// Verify all predefined locations produce valid responses
	for _, loc := range predefinedLocations {
		resp := locationToResponse(loc)
		if resp.ID != loc.ID {
			t.Errorf("ID mismatch for %s", loc.ID)
		}
		if resp.GridID == "" {
			t.Errorf("empty GridID for %s", loc.ID)
		}
		if !resp.Demo {
			t.Errorf("demo should be true for %s", loc.ID)
		}
	}
}
