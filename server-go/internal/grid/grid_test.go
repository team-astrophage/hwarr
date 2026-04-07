package grid

import (
	"math"
	"testing"
)

func TestToGridID(t *testing.T) {
	tests := []struct {
		lat, lng float64
		want     string
	}{
		{37.5765, 126.9776, "41751:115434"},
		{0, 0, "0:0"},
		{-37.5765, -126.9776, "-41752:-115435"},
	}
	for _, tt := range tests {
		got := ToGridID(tt.lat, tt.lng)
		if got != tt.want {
			t.Errorf("ToGridID(%f, %f) = %s, want %s", tt.lat, tt.lng, got, tt.want)
		}
	}
}

func TestGridIDToCenter(t *testing.T) {
	// Round-trip: ToGridID then GridIDToCenter should be close to original
	origLat, origLng := 37.5765, 126.9776
	gridID := ToGridID(origLat, origLng)
	lat, lng, err := GridIDToCenter(gridID)
	if err != nil {
		t.Fatal(err)
	}
	// Center should be within one grid cell of original
	if math.Abs(lat-origLat) > LatUnit {
		t.Errorf("lat diff too large: got %f, orig %f", lat, origLat)
	}
	if math.Abs(lng-origLng) > LngUnit {
		t.Errorf("lng diff too large: got %f, orig %f", lng, origLng)
	}
}

func TestGridIDToCenter_Invalid(t *testing.T) {
	_, _, err := GridIDToCenter("invalid")
	if err == nil {
		t.Error("expected error for invalid grid ID")
	}
}

func TestGetGridsInViewport(t *testing.T) {
	// Small viewport: 2x2 grid cells
	swLat := 0.0
	swLng := 0.0
	neLat := 1.5 * LatUnit
	neLng := 1.5 * LngUnit

	grids := GetGridsInViewport(neLat, neLng, swLat, swLng)

	// Should cover grid cells 0:0, 0:1, 1:0, 1:1
	if len(grids) != 4 {
		t.Fatalf("expected 4 grids, got %d: %v", len(grids), grids)
	}

	expected := map[string]bool{"0:0": true, "0:1": true, "1:0": true, "1:1": true}
	for _, g := range grids {
		if !expected[g] {
			t.Errorf("unexpected grid %s", g)
		}
	}
}

func TestGetGridsInViewport_SingleCell(t *testing.T) {
	// Viewport within a single cell
	grids := GetGridsInViewport(0.0001, 0.0001, 0.0, 0.0)
	if len(grids) != 1 {
		t.Fatalf("expected 1 grid, got %d", len(grids))
	}
	if grids[0] != "0:0" {
		t.Errorf("expected 0:0, got %s", grids[0])
	}
}

func TestGetNeighbors8(t *testing.T) {
	neighbors, err := GetNeighbors8("5:5")
	if err != nil {
		t.Fatal(err)
	}
	if len(neighbors) != 8 {
		t.Fatalf("expected 8 neighbors, got %d", len(neighbors))
	}

	expected := map[string]bool{
		"4:4": true, "4:5": true, "4:6": true,
		"5:4": true, "5:6": true,
		"6:4": true, "6:5": true, "6:6": true,
	}
	for _, n := range neighbors {
		if !expected[n] {
			t.Errorf("unexpected neighbor %s", n)
		}
	}
}
