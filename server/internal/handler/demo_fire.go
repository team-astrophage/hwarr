package handler

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/homepy/hwarr/server/internal/geodata"
	"github.com/homepy/hwarr/server/internal/grid"
	"github.com/homepy/hwarr/server/internal/model"
)

// FireTTLSec is the default fire TTL in seconds (12 hours).
const FireTTLSec = 43200

// FireSpreadThreshold is the active fire count above which fires spread to neighbors.
const FireSpreadThreshold = 500

// FireMaxCascadeDepth limits how many spread hops a single fire can cascade.
const FireMaxCascadeDepth = 8

// FireRegistration is the result of registering a fire event.
type FireRegistration struct {
	GridID      string
	ActiveCount int
	SpreadPath  [][2]string // [(from, to), ...] — empty if fire landed on requested grid
}

// RedisFireWriter abstracts the Redis operations needed by DemoFireHandler.
type RedisFireWriter interface {
	// ZAdd adds a member with score to a sorted set.
	ZAdd(ctx context.Context, key string, score float64, member string) error
	// ZCount returns the number of elements in the sorted set with score between min and max.
	ZCount(ctx context.Context, key, min, max string) (int64, error)
	// SAdd adds a member to a set.
	SAdd(ctx context.Context, key string, member string) error
}

// Broadcaster abstracts Socket.IO broadcasting.
type Broadcaster interface {
	BroadcastToRoom(event string, data interface{}, room string) error
	Broadcast(event string, data interface{}) error
}

// DemoFireRequest is the JSON body for POST /api/demo/fire.
type DemoFireRequest struct {
	LocationID *string `json:"location_id,omitempty"`
}

// DemoFireResponse is the JSON response for POST /api/demo/fire.
type DemoFireResponse struct {
	GridID      string               `json:"grid_id"`
	EventID     string               `json:"event_id"`
	ActiveCount int                  `json:"active_count"`
	Stage       int                  `json:"stage"`
	StageInfo   model.FireStageInfo  `json:"stage_info"`
	Location    DemoLocationResponse `json:"location"`
	Demo        bool                 `json:"demo"`
}

// DemoFireHandler serves the POST /api/demo/fire endpoint.
type DemoFireHandler struct {
	redis       RedisFireWriter
	broadcaster Broadcaster
	resolver    *geodata.AdminRegionResolver
}

// NewDemoFireHandler creates a new DemoFireHandler.
func NewDemoFireHandler(redis RedisFireWriter, broadcaster Broadcaster, resolver *geodata.AdminRegionResolver) *DemoFireHandler {
	return &DemoFireHandler{redis: redis, broadcaster: broadcaster, resolver: resolver}
}

// Handle ignites a fire at a demo location without GPS.
//
//	POST /api/demo/fire
//	Request body (optional): {"location_id": "gangnam"}
//	Response (201): DemoFireResponse
func (h *DemoFireHandler) Handle(c *gin.Context) {
	var body DemoFireRequest
	// Body is optional — bind only if present, ignore errors for empty body
	_ = c.ShouldBindJSON(&body)

	locationID := ""
	if body.LocationID != nil {
		locationID = *body.LocationID
	}

	loc, err := pickDemoLocation(locationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"detail": err.Error(),
		})
		return
	}

	lat := loc.Lat
	lng := loc.Lng

	// GPS → Grid ID
	gridID := grid.ToGridID(lat, lng)

	// Generate fire event
	eventID := fmt.Sprintf("fire-demo-%s", randomHex(12))
	expireAt := float64(time.Now().Unix()) + FireTTLSec

	// Register in Redis (may spread to neighbor if threshold exceeded)
	ctx := c.Request.Context()
	reg, err := h.registerFire(ctx, gridID, eventID, expireAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"detail": "failed to register fire event",
		})
		return
	}

	if reg.GridID != gridID {
		newLat, newLng, centerErr := grid.GridIDToCenter(reg.GridID)
		if centerErr == nil {
			lat = newLat
			lng = newLng
		}
	}
	gridID = reg.GridID
	activeCount := reg.ActiveCount

	// Build state
	state := model.BuildGridState(gridID, activeCount, &lat, &lng)

	// Broadcast to clients
	now := float64(time.Now().UnixMilli()) / 1000.0
	ignitePayload := map[string]interface{}{
		"grid_id":       gridID,
		"event_id":      eventID,
		"lat":           lat,
		"lng":           lng,
		"active_count":  activeCount,
		"stage":         state.Stage,
		"stage_info":    state.StageInfo,
		"ignited_by":    "demo_mode",
		"demo":          true,
		"location_name": loc.Name,
		"timestamp":     now,
	}

	globalPayload := map[string]interface{}{
		"grid_id":      gridID,
		"active_count": activeCount,
		"stage":        state.Stage,
		"stage_info":   state.StageInfo,
		"demo":         true,
		"timestamp":    now,
	}

	if h.broadcaster != nil {
		_ = h.broadcaster.BroadcastToRoom("fire:ignite", ignitePayload, gridID)
		_ = h.broadcaster.BroadcastToRoom("fire:update", globalPayload, gridID)

		// Broadcast fire:spread animation to affected grid rooms only
		if len(reg.SpreadPath) > 0 {
			spreadPathMaps := make([]map[string]string, len(reg.SpreadPath))
			affectedGrids := make(map[string]struct{})
			for i, sp := range reg.SpreadPath {
				spreadPathMaps[i] = map[string]string{
					"from": sp[0],
					"to":   sp[1],
				}
				affectedGrids[sp[0]] = struct{}{}
				affectedGrids[sp[1]] = struct{}{}
			}
			spreadPayload := map[string]interface{}{
				"path":      spreadPathMaps,
				"event_id":  eventID,
				"timestamp": now,
			}
			for room := range affectedGrids {
				_ = h.broadcaster.BroadcastToRoom("fire:spread", spreadPayload, room)
			}
		}
	}

	demoLoc := locationToResponse(loc)

	c.JSON(http.StatusCreated, DemoFireResponse{
		GridID:      gridID,
		EventID:     eventID,
		ActiveCount: activeCount,
		Stage:       state.Stage,
		StageInfo:   state.StageInfo,
		Location:    demoLoc,
		Demo:        true,
	})
}

// registerFire registers a fire event in Redis, spreading to neighbors if needed.
func (h *DemoFireHandler) registerFire(ctx context.Context, gridID, eventID string, expireAt float64) (*FireRegistration, error) {
	now := fmt.Sprintf("%d", time.Now().Unix())
	currentGrid := gridID
	var spreadPath [][2]string

	// Walk the cascade: spread to neighbor if current grid is at/above threshold
	for i := 0; i < FireMaxCascadeDepth; i++ {
		key := fmt.Sprintf("%s%s", FireKeyPrefix, currentGrid)
		count, err := h.redis.ZCount(ctx, key, now, "+inf")
		if err != nil {
			return nil, err
		}
		if count < FireSpreadThreshold {
			break // capacity here — land at currentGrid
		}
		// Spread to a random neighbor
		neighbors, err := grid.GetNeighbors8(currentGrid)
		if err != nil {
			break
		}
		nextGrid := neighbors[rand.Intn(len(neighbors))]
		spreadPath = append(spreadPath, [2]string{currentGrid, nextGrid})
		currentGrid = nextGrid
	}

	landingKey := fmt.Sprintf("%s%s", FireKeyPrefix, currentGrid)

	// Add fire event to the landing grid's sorted set
	if err := h.redis.ZAdd(ctx, landingKey, expireAt, eventID); err != nil {
		return nil, err
	}

	// Track active grid
	if err := h.redis.SAdd(ctx, "active_grids", currentGrid); err != nil {
		return nil, err
	}

	// NOTE: demo fires intentionally skip stats (total_fires, daily_fires, daily_ranking)
	// to avoid polluting production rankings. See GitHub issue #89.

	// Get final active count
	activeCount, err := h.redis.ZCount(ctx, landingKey, now, "+inf")
	if err != nil {
		return nil, err
	}

	return &FireRegistration{
		GridID:      currentGrid,
		ActiveCount: int(activeCount),
		SpreadPath:  spreadPath,
	}, nil
}

// Register adds the demo fire endpoint to the given Gin router.
func (h *DemoFireHandler) Register(r gin.IRouter) {
	r.POST("/api/demo/fire", h.Handle)
}

// randomHex generates a random hex string of the given length.
func randomHex(n int) string {
	const hexChars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hexChars[rand.Intn(len(hexChars))]
	}
	return string(b)
}
