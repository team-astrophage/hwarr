// Location mapping — grid_id to Korean place name (3-tier).
//
//	Tier 1: Landmarks (narrow range, specific places)
//	Tier 2: Districts (Seoul 25 gu + major city districts)
//	Tier 3: Provinces (si/do level)
//	Fallback: coordinate-based "N37.5° E127.0° 부근"
package grid

import "fmt"

// latLngRange represents a bounding box for location matching.
type latLngRange struct {
	Name   string
	LatMin float64
	LatMax float64
	LngMin float64
	LngMax float64
}

// Tier 1: Landmarks — narrow range, specific place names.
var landmarks = []latLngRange{
	{Name: "강남 테헤란로", LatMin: 37.49, LatMax: 37.52, LngMin: 127.02, LngMax: 127.07},
	{Name: "서초 교대역", LatMin: 37.47, LatMax: 37.50, LngMin: 126.98, LngMax: 127.02},
	{Name: "송파 잠실", LatMin: 37.505, LatMax: 37.52, LngMin: 127.07, LngMax: 127.10},
	{Name: "종로 광화문", LatMin: 37.57, LatMax: 37.58, LngMin: 126.97, LngMax: 126.98},
	{Name: "홍대입구", LatMin: 37.556, LatMax: 37.562, LngMin: 126.92, LngMax: 126.93},
	{Name: "이태원", LatMin: 37.533, LatMax: 37.537, LngMin: 126.99, LngMax: 127.00},
	{Name: "여의도", LatMin: 37.52, LatMax: 37.53, LngMin: 126.92, LngMax: 126.94},
	{Name: "디지털단지", LatMin: 37.48, LatMax: 37.49, LngMin: 126.89, LngMax: 126.91},
	{Name: "성수동", LatMin: 37.54, LatMax: 37.55, LngMin: 127.05, LngMax: 127.06},
	{Name: "판교 테크노밸리", LatMin: 37.39, LatMax: 37.41, LngMin: 127.09, LngMax: 127.11},
}

// Tier 2: Districts — Seoul 25 gu + metropolitan area districts.
var districts = []latLngRange{
	// Seoul 25 gu
	{Name: "서울 종로구", LatMin: 37.57, LatMax: 37.61, LngMin: 126.96, LngMax: 127.02},
	{Name: "서울 중구", LatMin: 37.55, LatMax: 37.57, LngMin: 126.96, LngMax: 127.01},
	{Name: "서울 용산구", LatMin: 37.52, LatMax: 37.55, LngMin: 126.95, LngMax: 127.01},
	{Name: "서울 성동구", LatMin: 37.54, LatMax: 37.57, LngMin: 127.01, LngMax: 127.07},
	{Name: "서울 광진구", LatMin: 37.53, LatMax: 37.56, LngMin: 127.07, LngMax: 127.11},
	{Name: "서울 동대문구", LatMin: 37.57, LatMax: 37.60, LngMin: 127.02, LngMax: 127.07},
	{Name: "서울 중랑구", LatMin: 37.59, LatMax: 37.62, LngMin: 127.07, LngMax: 127.11},
	{Name: "서울 성북구", LatMin: 37.58, LatMax: 37.62, LngMin: 126.99, LngMax: 127.04},
	{Name: "서울 강북구", LatMin: 37.61, LatMax: 37.65, LngMin: 126.99, LngMax: 127.04},
	{Name: "서울 도봉구", LatMin: 37.65, LatMax: 37.70, LngMin: 127.02, LngMax: 127.06},
	{Name: "서울 노원구", LatMin: 37.62, LatMax: 37.67, LngMin: 127.04, LngMax: 127.10},
	{Name: "서울 은평구", LatMin: 37.59, LatMax: 37.64, LngMin: 126.90, LngMax: 126.96},
	{Name: "서울 서대문구", LatMin: 37.56, LatMax: 37.60, LngMin: 126.92, LngMax: 126.97},
	{Name: "서울 마포구", LatMin: 37.54, LatMax: 37.57, LngMin: 126.88, LngMax: 126.96},
	{Name: "서울 양천구", LatMin: 37.51, LatMax: 37.54, LngMin: 126.85, LngMax: 126.89},
	{Name: "서울 강서구", LatMin: 37.54, LatMax: 37.58, LngMin: 126.81, LngMax: 126.86},
	{Name: "서울 구로구", LatMin: 37.48, LatMax: 37.51, LngMin: 126.85, LngMax: 126.90},
	{Name: "서울 금천구", LatMin: 37.44, LatMax: 37.48, LngMin: 126.89, LngMax: 126.92},
	{Name: "서울 영등포구", LatMin: 37.51, LatMax: 37.54, LngMin: 126.88, LngMax: 126.93},
	{Name: "서울 동작구", LatMin: 37.49, LatMax: 37.52, LngMin: 126.93, LngMax: 126.98},
	{Name: "서울 관악구", LatMin: 37.46, LatMax: 37.49, LngMin: 126.92, LngMax: 126.97},
	{Name: "서울 서초구", LatMin: 37.46, LatMax: 37.50, LngMin: 126.97, LngMax: 127.06},
	{Name: "서울 강남구", LatMin: 37.49, LatMax: 37.53, LngMin: 127.01, LngMax: 127.10},
	{Name: "서울 송파구", LatMin: 37.49, LatMax: 37.53, LngMin: 127.07, LngMax: 127.14},
	{Name: "서울 강동구", LatMin: 37.52, LatMax: 37.56, LngMin: 127.11, LngMax: 127.17},
	// Metropolitan area
	{Name: "성남 분당구", LatMin: 37.35, LatMax: 37.41, LngMin: 127.05, LngMax: 127.15},
	{Name: "수원 영통구", LatMin: 37.25, LatMax: 37.30, LngMin: 127.03, LngMax: 127.09},
	{Name: "수원 권선구", LatMin: 37.24, LatMax: 37.29, LngMin: 126.96, LngMax: 127.02},
	{Name: "용인 수지구", LatMin: 37.30, LatMax: 37.35, LngMin: 127.06, LngMax: 127.12},
	{Name: "고양 일산", LatMin: 37.64, LatMax: 37.70, LngMin: 126.72, LngMax: 126.82},
	{Name: "안양시", LatMin: 37.38, LatMax: 37.42, LngMin: 126.91, LngMax: 126.97},
	{Name: "부천시", LatMin: 37.48, LatMax: 37.52, LngMin: 126.76, LngMax: 126.83},
	{Name: "안산시", LatMin: 37.30, LatMax: 37.35, LngMin: 126.80, LngMax: 126.88},
	{Name: "화성 동탄", LatMin: 37.18, LatMax: 37.23, LngMin: 127.05, LngMax: 127.10},
	{Name: "인천 남동구", LatMin: 37.39, LatMax: 37.43, LngMin: 126.68, LngMax: 126.75},
	{Name: "인천 연수구", LatMin: 37.37, LatMax: 37.41, LngMin: 126.63, LngMax: 126.70},
	{Name: "인천 부평구", LatMin: 37.49, LatMax: 37.52, LngMin: 126.70, LngMax: 126.75},
	// Major cities
	{Name: "부산 해운대구", LatMin: 35.15, LatMax: 35.20, LngMin: 129.13, LngMax: 129.21},
	{Name: "부산 부산진구", LatMin: 35.14, LatMax: 35.18, LngMin: 129.03, LngMax: 129.08},
	{Name: "부산 남구", LatMin: 35.11, LatMax: 35.15, LngMin: 129.07, LngMax: 129.12},
	{Name: "대구 중구", LatMin: 35.86, LatMax: 35.88, LngMin: 128.58, LngMax: 128.61},
	{Name: "대구 수성구", LatMin: 35.83, LatMax: 35.87, LngMin: 128.61, LngMax: 128.67},
	{Name: "대전 유성구", LatMin: 36.33, LatMax: 36.40, LngMin: 127.30, LngMax: 127.40},
	{Name: "대전 서구", LatMin: 36.33, LatMax: 36.38, LngMin: 127.37, LngMax: 127.42},
	{Name: "광주 서구", LatMin: 35.13, LatMax: 35.17, LngMin: 126.86, LngMax: 126.91},
	{Name: "울산 남구", LatMin: 35.52, LatMax: 35.56, LngMin: 129.32, LngMax: 129.37},
}

// Tier 3: Provinces — si/do level.
var provinces = []latLngRange{
	{Name: "서울", LatMin: 37.42, LatMax: 37.70, LngMin: 126.77, LngMax: 127.18},
	{Name: "인천", LatMin: 37.35, LatMax: 37.60, LngMin: 126.35, LngMax: 126.80},
	{Name: "경기 북부", LatMin: 37.70, LatMax: 38.30, LngMin: 126.40, LngMax: 127.80},
	{Name: "경기 남부", LatMin: 36.90, LatMax: 37.42, LngMin: 126.60, LngMax: 127.60},
	{Name: "부산", LatMin: 35.05, LatMax: 35.25, LngMin: 128.85, LngMax: 129.25},
	{Name: "대구", LatMin: 35.75, LatMax: 36.05, LngMin: 128.45, LngMax: 128.80},
	{Name: "대전", LatMin: 36.25, LatMax: 36.50, LngMin: 127.30, LngMax: 127.55},
	{Name: "광주", LatMin: 35.05, LatMax: 35.25, LngMin: 126.75, LngMax: 127.00},
	{Name: "울산", LatMin: 35.45, LatMax: 35.65, LngMin: 129.20, LngMax: 129.50},
	{Name: "세종", LatMin: 36.45, LatMax: 36.65, LngMin: 126.85, LngMax: 127.10},
	{Name: "강원", LatMin: 37.00, LatMax: 38.60, LngMin: 127.50, LngMax: 129.40},
	{Name: "충북", LatMin: 36.30, LatMax: 37.20, LngMin: 127.30, LngMax: 128.30},
	{Name: "충남", LatMin: 36.00, LatMax: 37.00, LngMin: 126.00, LngMax: 127.30},
	{Name: "전북", LatMin: 35.30, LatMax: 36.20, LngMin: 126.30, LngMax: 127.50},
	{Name: "전남", LatMin: 34.00, LatMax: 35.50, LngMin: 126.00, LngMax: 127.50},
	{Name: "경북", LatMin: 35.50, LatMax: 37.10, LngMin: 128.30, LngMax: 130.00},
	{Name: "경남", LatMin: 34.50, LatMax: 35.80, LngMin: 127.50, LngMax: 129.40},
	{Name: "제주", LatMin: 33.10, LatMax: 33.60, LngMin: 126.10, LngMax: 127.00},
}

// locationTiers is the ordered list of tiers to check.
var locationTiers = [][]latLngRange{landmarks, districts, provinces}

// GetLocationName resolves a grid ID to a human-readable Korean location name.
//
// Checks 3 tiers (landmarks -> districts -> provinces), then falls back
// to a coordinate-based description.
func GetLocationName(gridID string) string {
	lat, lng, err := GridIDToCenter(gridID)
	if err != nil {
		return gridID
	}

	for _, tier := range locationTiers {
		for _, area := range tier {
			if area.LatMin <= lat && lat <= area.LatMax &&
				area.LngMin <= lng && lng <= area.LngMax {
				return area.Name
			}
		}
	}

	return fmt.Sprintf("N%.1f° E%.1f° 부근", lat, lng)
}
