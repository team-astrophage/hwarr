package grid

import (
	"strings"
	"testing"
)

func TestGetLocationName_Landmark(t *testing.T) {
	// Gangnam Teheranro: lat (37.49, 37.52), lng (127.02, 127.07)
	// Pick a point in the middle: lat=37.505, lng=127.045
	gridID := ToGridID(37.505, 127.045)
	name := GetLocationName(gridID)
	if name != "강남 테헤란로" {
		t.Errorf("expected '강남 테헤란로', got %q for gridID %s", name, gridID)
	}
}

func TestGetLocationName_District(t *testing.T) {
	// Seoul Eunpyeong-gu: lat (37.59, 37.64), lng (126.90, 126.96)
	// Pick a point: lat=37.615, lng=126.93
	gridID := ToGridID(37.615, 0.93)
	// This point is not in Eunpyeong's lng range, let's use proper coords
	gridID = ToGridID(37.615, 126.93)
	name := GetLocationName(gridID)
	if name != "서울 은평구" {
		t.Errorf("expected '서울 은평구', got %q for gridID %s", name, gridID)
	}
}

func TestGetLocationName_Province(t *testing.T) {
	// Jeju: lat (33.10, 33.60), lng (126.10, 127.00)
	gridID := ToGridID(33.35, 126.55)
	name := GetLocationName(gridID)
	if name != "제주" {
		t.Errorf("expected '제주', got %q for gridID %s", name, gridID)
	}
}

func TestGetLocationName_Fallback(t *testing.T) {
	// A point far outside Korea
	gridID := ToGridID(10.0, 100.0)
	name := GetLocationName(gridID)
	if !strings.Contains(name, "부근") {
		t.Errorf("expected fallback with '부근', got %q", name)
	}
	if !strings.HasPrefix(name, "N") {
		t.Errorf("expected fallback starting with 'N', got %q", name)
	}
}

func TestGetLocationName_InvalidGridID(t *testing.T) {
	name := GetLocationName("invalid")
	if name != "invalid" {
		t.Errorf("expected 'invalid' for invalid grid_id, got %q", name)
	}
}

func TestGetLocationName_TierPriority(t *testing.T) {
	// A point in Gwanghwamun landmark should match landmark, not district/province.
	// Gwanghwamun: lat (37.57, 37.58), lng (126.97, 126.98)
	gridID := ToGridID(37.575, 126.975)
	name := GetLocationName(gridID)
	if name != "종로 광화문" {
		t.Errorf("expected '종로 광화문' (landmark), got %q", name)
	}
}

func TestGetLocationName_BusanHaeundae(t *testing.T) {
	// Busan Haeundae: lat (35.15, 35.20), lng (129.13, 129.21)
	gridID := ToGridID(35.175, 129.17)
	name := GetLocationName(gridID)
	if name != "부산 해운대구" {
		t.Errorf("expected '부산 해운대구', got %q for gridID %s", name, gridID)
	}
}

func TestGetLocationName_Pangyo(t *testing.T) {
	// Pangyo Technovalley: lat (37.39, 37.41), lng (127.09, 127.11)
	gridID := ToGridID(37.40, 127.10)
	name := GetLocationName(gridID)
	if name != "판교 테크노밸리" {
		t.Errorf("expected '판교 테크노밸리', got %q for gridID %s", name, gridID)
	}
}
