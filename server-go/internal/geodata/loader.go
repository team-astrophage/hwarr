// Package geodata provides a GeoJSON-based administrative region resolver.
//
// It loads a FeatureCollection of 행정동 (administrative ward) polygons,
// builds an in-memory spatial index, and resolves (lat, lng) coordinates
// to region labels such as "서울특별시 강남구 역삼1동".
//
// This is the Go equivalent of server/services/admin_region.py and uses
// the paulmach/orb library for geometry operations.
package geodata

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
)

// nameFields is the priority-ordered list of GeoJSON property keys
// that may contain the administrative region name.
// Matches Python server: ("adm_nm", "ADM_NM", "EMD_KOR_NM", "adm_nm_kor", "name")
var nameFields = []string{"adm_nm", "ADM_NM", "EMD_KOR_NM", "adm_nm_kor", "name"}

// Region holds a single administrative region polygon and its label.
type Region struct {
	Label    string
	Geometry orb.Geometry
	Bound    orb.Bound
}

// AdminRegionResolver resolves (lat, lng) to administrative region labels
// using point-in-polygon tests against loaded GeoJSON data.
type AdminRegionResolver struct {
	regions []Region
}

// RegionCount returns the number of loaded regions.
func (r *AdminRegionResolver) RegionCount() int {
	return len(r.regions)
}

// Resolve returns the region label for the given coordinates,
// or empty string if the point is outside all loaded polygons.
// Note: lat/lng order matches the Python API (lat first, lng second).
// GeoJSON internally uses (lng, lat) but orb handles this via its Point type.
func (r *AdminRegionResolver) Resolve(lat, lng float64) string {
	point := orb.Point{lng, lat} // orb.Point is [lng, lat]

	for i := range r.regions {
		region := &r.regions[i]
		// Fast bounding box check first
		if !region.Bound.Contains(point) {
			continue
		}
		// Precise point-in-polygon test
		if containsPoint(region.Geometry, point) {
			return region.Label
		}
	}
	return ""
}

// containsPoint checks if a point is inside a geometry.
// Supports Polygon, MultiPolygon, and GeometryCollection types.
func containsPoint(geom orb.Geometry, point orb.Point) bool {
	switch g := geom.(type) {
	case orb.Polygon:
		return planar.PolygonContains(g, point)
	case orb.MultiPolygon:
		return planar.MultiPolygonContains(g, point)
	case orb.Collection:
		for _, child := range g {
			if containsPoint(child, point) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// extractName extracts the region name from feature properties,
// checking fields in priority order.
func extractName(props geojson.Properties) string {
	for _, field := range nameFields {
		if val, ok := props[field]; ok {
			if s, ok := val.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// shortenName normalizes a full administrative name by collapsing whitespace.
// e.g. "서울특별시  강남구  역삼1동" → "서울특별시 강남구 역삼1동"
func shortenName(admNm string) string {
	return strings.Join(strings.Fields(admNm), " ")
}

// LoadOrNil loads an AdminRegionResolver from a GeoJSON file.
// Returns nil if the file does not exist or contains no valid features.
// This mirrors Python's AdminRegionResolver.load_or_none().
func LoadOrNil(path string) *AdminRegionResolver {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("admin region resolver disabled: GeoJSON not found",
				"path", path)
		} else {
			slog.Error("failed to read admin GeoJSON",
				"path", path, "error", err)
		}
		return nil
	}

	fc, err := geojson.UnmarshalFeatureCollection(data)
	if err != nil {
		slog.Error("failed to parse admin GeoJSON",
			"path", path, "error", err)
		return nil
	}

	regions := make([]Region, 0, len(fc.Features))
	for _, feature := range fc.Features {
		name := extractName(feature.Properties)
		if name == "" {
			continue
		}
		geom := feature.Geometry
		if geom == nil {
			continue
		}
		// Skip empty geometries
		if isEmptyGeometry(geom) {
			continue
		}
		regions = append(regions, Region{
			Label:    shortenName(name),
			Geometry: geom,
			Bound:    geom.Bound(),
		})
	}

	if len(regions) == 0 {
		slog.Warn("admin region resolver loaded 0 features",
			"path", path)
		return nil
	}

	slog.Info(fmt.Sprintf("loaded %d admin regions from GeoJSON", len(regions)),
		"path", path)
	return &AdminRegionResolver{regions: regions}
}

// LoadFromBytes loads an AdminRegionResolver from raw GeoJSON bytes.
// Useful for testing or embedding data.
func LoadFromBytes(data []byte) (*AdminRegionResolver, error) {
	fc, err := geojson.UnmarshalFeatureCollection(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GeoJSON: %w", err)
	}

	regions := make([]Region, 0, len(fc.Features))
	for _, feature := range fc.Features {
		name := extractName(feature.Properties)
		if name == "" {
			continue
		}
		geom := feature.Geometry
		if geom == nil {
			continue
		}
		if isEmptyGeometry(geom) {
			continue
		}
		regions = append(regions, Region{
			Label:    shortenName(name),
			Geometry: geom,
			Bound:    geom.Bound(),
		})
	}

	if len(regions) == 0 {
		return nil, fmt.Errorf("no valid features found in GeoJSON")
	}

	return &AdminRegionResolver{regions: regions}, nil
}

// isEmptyGeometry checks if a geometry has no coordinates.
func isEmptyGeometry(geom orb.Geometry) bool {
	switch g := geom.(type) {
	case orb.Polygon:
		return len(g) == 0
	case orb.MultiPolygon:
		return len(g) == 0
	case orb.Point:
		return false
	default:
		// Marshal and check — fallback for unusual types
		data, err := json.Marshal(geom)
		if err != nil {
			return true
		}
		return len(data) <= 2 // "[]" or "{}"
	}
}
