package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/homepy/hwarr/server/internal/grid"
)

// ---------------------------------------------------------------------------
// Response models
// ---------------------------------------------------------------------------

// FireLocation is a predefined clickable fire location on the map.
type FireLocation struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	GridID      string  `json:"grid_id"`
	Description string  `json:"description"`
}

// MapBounds is a bounding box for the map view.
type MapBounds struct {
	NELat float64 `json:"ne_lat"`
	NELng float64 `json:"ne_lng"`
	SWLat float64 `json:"sw_lat"`
	SWLng float64 `json:"sw_lng"`
}

// GridConfig holds grid system configuration metadata.
type GridConfig struct {
	LatUnit        float64 `json:"lat_unit"`
	LngUnit        float64 `json:"lng_unit"`
	CellSizeMeters int     `json:"cell_size_meters"`
}

// MapConfig is the complete map configuration returned by the API.
type MapConfig struct {
	CenterLat   float64        `json:"center_lat"`
	CenterLng   float64        `json:"center_lng"`
	DefaultZoom int            `json:"default_zoom"`
	MinZoom     int            `json:"min_zoom"`
	MaxZoom     int            `json:"max_zoom"`
	Bounds      MapBounds      `json:"bounds"`
	Grid        GridConfig     `json:"grid"`
	Locations   []FireLocation `json:"locations"`
}

// ---------------------------------------------------------------------------
// Predefined fire locations — Korean landmarks for the hackathon demo
// ---------------------------------------------------------------------------

type rawLocation struct {
	ID          string
	Name        string
	Lat         float64
	Lng         float64
	Description string
}

var predefinedLocations = []rawLocation{
	// Seoul landmarks
	{ID: "gwanghwamun", Name: "광화문광장", Lat: 37.5760, Lng: 126.9769, Description: "서울 광화문광장"},
	{ID: "gangnam", Name: "강남역", Lat: 37.4979, Lng: 127.0276, Description: "서울 강남역 사거리"},
	{ID: "hongdae", Name: "홍대입구", Lat: 37.5563, Lng: 126.9236, Description: "서울 홍대입구역"},
	{ID: "yeouido", Name: "여의도공원", Lat: 37.5284, Lng: 126.9344, Description: "서울 여의도공원"},
	{ID: "jamsil", Name: "잠실종합운동장", Lat: 37.5153, Lng: 127.0728, Description: "서울 잠실종합운동장"},
	{ID: "namsan", Name: "남산타워", Lat: 37.5512, Lng: 126.9882, Description: "서울 남산서울타워"},
	{ID: "itaewon", Name: "이태원", Lat: 37.5345, Lng: 126.9946, Description: "서울 이태원거리"},
	// Major cities
	{ID: "busan_haeundae", Name: "해운대해수욕장", Lat: 35.1587, Lng: 129.1604, Description: "부산 해운대해수욕장"},
	{ID: "daegu_dongseong", Name: "동성로", Lat: 35.8691, Lng: 128.5958, Description: "대구 동성로"},
	{ID: "incheon_songdo", Name: "송도센트럴파크", Lat: 37.3925, Lng: 126.6632, Description: "인천 송도센트럴파크"},
	{ID: "gwangju_chungjang", Name: "충장로", Lat: 35.1488, Lng: 126.9156, Description: "광주 충장로"},
	{ID: "daejeon_dunsan", Name: "둔산동", Lat: 36.3511, Lng: 127.3782, Description: "대전 둔산동"},
	{ID: "jeju_hallasan", Name: "한라산", Lat: 33.3617, Lng: 126.5292, Description: "제주 한라산"},
	{ID: "suwon_hwaseong", Name: "수원화성", Lat: 37.2870, Lng: 127.0095, Description: "수원 화성행궁"},
	{ID: "gyeongju_bulguksa", Name: "불국사", Lat: 35.7900, Lng: 129.3322, Description: "경주 불국사"},
	// Demo / hackathon venue fallback (Gangnam COEX area)
	{ID: "coex", Name: "코엑스", Lat: 37.5126, Lng: 127.0590, Description: "서울 코엑스 (해커톤 데모 장소)"},
}

func buildFireLocations() []FireLocation {
	locations := make([]FireLocation, len(predefinedLocations))
	for i, loc := range predefinedLocations {
		locations[i] = FireLocation{
			ID:          loc.ID,
			Name:        loc.Name,
			Lat:         loc.Lat,
			Lng:         loc.Lng,
			GridID:      grid.ToGridID(loc.Lat, loc.Lng),
			Description: loc.Description,
		}
	}
	return locations
}

// mapConfigInstance is the static map configuration, built once at init.
var mapConfigInstance = MapConfig{
	CenterLat:   36.5,
	CenterLng:   127.8,
	DefaultZoom: 7,
	MinZoom:     6,
	MaxZoom:     18,
	Bounds: MapBounds{
		NELat: 38.6,
		NELng: 131.9,
		SWLat: 33.0,
		SWLng: 124.5,
	},
	Grid: GridConfig{
		LatUnit:        grid.LatUnit,
		LngUnit:        grid.LngUnit,
		CellSizeMeters: 100,
	},
	Locations: buildFireLocations(),
}

// MapConfigHandler serves the GET /api/map/config endpoint.
type MapConfigHandler struct{}

// NewMapConfigHandler creates a new MapConfigHandler.
func NewMapConfigHandler() *MapConfigHandler {
	return &MapConfigHandler{}
}

// Handle returns the map configuration including predefined clickable fire locations.
//
//	GET /api/map/config
//	Response: MapConfig JSON
func (h *MapConfigHandler) Handle(c *gin.Context) {
	c.JSON(http.StatusOK, mapConfigInstance)
}

// Register adds the map config endpoint to the given Gin router group.
func (h *MapConfigHandler) Register(r gin.IRouter) {
	r.GET("/api/map/config", h.Handle)
}
