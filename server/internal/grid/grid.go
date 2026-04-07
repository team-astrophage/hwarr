// Package grid provides GPS-to-grid conversion and viewport utilities.
//
// Grid system: 100m x 100m cells based on GPS coordinates.
// LAT_UNIT = 0.0009 (~100m latitude at ~37N)
// LNG_UNIT = 0.0011 (~100m longitude at ~37N)
package grid

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	// LatUnit is the latitude span of one grid cell (~100m at ~37N).
	LatUnit = 0.0009
	// LngUnit is the longitude span of one grid cell (~100m at ~37N).
	LngUnit = 0.0011
)

// ToGridID converts GPS coordinates to a grid cell ID.
func ToGridID(lat, lng float64) string {
	gridLat := int(math.Floor(lat / LatUnit))
	gridLng := int(math.Floor(lng / LngUnit))
	return fmt.Sprintf("%d:%d", gridLat, gridLng)
}

// GridIDToCenter converts a grid ID back to the center coordinates of the cell.
func GridIDToCenter(gridID string) (lat, lng float64, err error) {
	parts := strings.SplitN(gridID, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid grid_id format: %s", gridID)
	}
	gridLat, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid grid_lat: %s", parts[0])
	}
	gridLng, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid grid_lng: %s", parts[1])
	}
	lat = (float64(gridLat) + 0.5) * LatUnit
	lng = (float64(gridLng) + 0.5) * LngUnit
	return lat, lng, nil
}

// GetGridsInViewport returns all grid IDs within a map viewport bounding box.
func GetGridsInViewport(neLat, neLng, swLat, swLng float64) []string {
	latMin := int(math.Floor(swLat / LatUnit))
	latMax := int(math.Floor(neLat / LatUnit))
	lngMin := int(math.Floor(swLng / LngUnit))
	lngMax := int(math.Floor(neLng / LngUnit))

	grids := make([]string, 0, (latMax-latMin+1)*(lngMax-lngMin+1))
	for lat := latMin; lat <= latMax; lat++ {
		for lng := lngMin; lng <= lngMax; lng++ {
			grids = append(grids, fmt.Sprintf("%d:%d", lat, lng))
		}
	}
	return grids
}

// GetNeighbors8 returns the 8 neighboring grid IDs (orthogonal + diagonal).
func GetNeighbors8(gridID string) ([]string, error) {
	parts := strings.SplitN(gridID, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid grid_id format: %s", gridID)
	}
	gridLat, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid grid_lat: %s", parts[0])
	}
	gridLng, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid grid_lng: %s", parts[1])
	}

	neighbors := make([]string, 0, 8)
	for dlat := -1; dlat <= 1; dlat++ {
		for dlng := -1; dlng <= 1; dlng++ {
			if dlat == 0 && dlng == 0 {
				continue
			}
			neighbors = append(neighbors, fmt.Sprintf("%d:%d", gridLat+dlat, gridLng+dlng))
		}
	}
	return neighbors, nil
}
