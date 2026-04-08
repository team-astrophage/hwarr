package handler

import (
	"math/rand"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/homepy/hwarr/server/internal/grid"
)

// DemoLocationResponse is a single demo location returned as GPS fallback.
type DemoLocationResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	GridID      string  `json:"grid_id"`
	Description string  `json:"description"`
	Demo        bool    `json:"demo"`
}

// DemoLocationsListResponse is the full list of all demo locations.
type DemoLocationsListResponse struct {
	Locations []DemoLocationResponse `json:"locations"`
	Total     int                    `json:"total"`
	Demo      bool                   `json:"demo"`
}

// DemoLocationHandler serves the GET /api/demo/location endpoint.
type DemoLocationHandler struct{}

// NewDemoLocationHandler creates a new DemoLocationHandler.
func NewDemoLocationHandler() *DemoLocationHandler {
	return &DemoLocationHandler{}
}

// Handle returns a random demo location, or a specific one if location_id is provided.
//
//	GET /api/demo/location?location_id=gangnam
//	Response: DemoLocationResponse JSON
func (h *DemoLocationHandler) Handle(c *gin.Context) {
	locationID := c.Query("location_id")

	loc, err := pickDemoLocation(locationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, locationToResponse(loc))
}

// HandleList returns all available demo locations.
//
//	GET /api/demo/locations
//	Response: DemoLocationsListResponse JSON
func (h *DemoLocationHandler) HandleList(c *gin.Context) {
	locations := make([]DemoLocationResponse, len(predefinedLocations))
	for i, loc := range predefinedLocations {
		locations[i] = locationToResponse(loc)
	}
	c.JSON(http.StatusOK, DemoLocationsListResponse{
		Locations: locations,
		Total:     len(locations),
		Demo:      true,
	})
}

// Register adds the demo location endpoints to the given Gin router group.
func (h *DemoLocationHandler) Register(r gin.IRouter) {
	r.GET("/api/demo/location", h.Handle)
	r.GET("/api/demo/locations", h.HandleList)
}

// pickDemoLocation returns a location by ID or a random one if id is empty.
func pickDemoLocation(id string) (rawLocation, error) {
	if id != "" {
		for _, loc := range predefinedLocations {
			if loc.ID == id {
				return loc, nil
			}
		}
		ids := make([]string, len(predefinedLocations))
		for i, l := range predefinedLocations {
			ids[i] = l.ID
		}
		return rawLocation{}, &demoLocationNotFoundError{ID: id, Available: ids}
	}
	return predefinedLocations[rand.Intn(len(predefinedLocations))], nil
}

// locationToResponse converts a rawLocation to a DemoLocationResponse.
func locationToResponse(loc rawLocation) DemoLocationResponse {
	return DemoLocationResponse{
		ID:          loc.ID,
		Name:        loc.Name,
		Lat:         loc.Lat,
		Lng:         loc.Lng,
		GridID:      grid.ToGridID(loc.Lat, loc.Lng),
		Description: loc.Description,
		Demo:        true,
	}
}

// demoLocationNotFoundError is returned when a location_id is not found.
type demoLocationNotFoundError struct {
	ID        string
	Available []string
}

func (e *demoLocationNotFoundError) Error() string {
	return "Demo location '" + e.ID + "' not found. Available: " + formatIDs(e.Available)
}

func formatIDs(ids []string) string {
	result := "["
	for i, id := range ids {
		if i > 0 {
			result += ", "
		}
		result += "'" + id + "'"
	}
	result += "]"
	return result
}
