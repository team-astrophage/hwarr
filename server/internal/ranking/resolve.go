package ranking

import (
	"github.com/homepy/hwarr/server/internal/geodata"
	"github.com/homepy/hwarr/server/internal/grid"
)

// ResolveMember returns the ranking member name for a given grid ID.
//
// Resolution chain:
//  1. GeoJSON AdminRegionResolver (precise admin-dong name, e.g. "서울특별시 강남구 역삼1동")
//  2. Hardcoded bounding-box lookup via grid.GetLocationName (approximate)
//  3. Raw gridID as last resort
//
// Matches Python FireProgressionEngine.register_fire() behavior where
// resolver.resolve(lat, lng) is used for ZINCRBY member.
func ResolveMember(gridID string, resolver *geodata.AdminRegionResolver) string {
	if resolver != nil {
		lat, lng, err := grid.GridIDToCenter(gridID)
		if err == nil {
			if region := resolver.Resolve(lat, lng); region != "" {
				return region
			}
		}
	}
	if name := grid.GetLocationName(gridID); name != "" {
		return name
	}
	return gridID
}
