package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/homepy/hwarr/server/internal/engine"
	"github.com/homepy/hwarr/server/internal/grid"
	"github.com/homepy/hwarr/server/internal/model"
)

// RedisGridReader abstracts the Redis operations needed by GridHandler.
type RedisGridReader interface {
	// ZCount returns the number of elements in the sorted set at key
	// with a score between min and max.
	ZCount(ctx context.Context, key, min, max string) (int64, error)
}

// GridHandler serves the GET /api/grid/:grid_id endpoint.
type GridHandler struct {
	redis RedisGridReader
}

// NewGridHandler creates a new GridHandler.
func NewGridHandler(redis RedisGridReader) *GridHandler {
	return &GridHandler{redis: redis}
}

// Handle returns the current fire state for a specific grid cell.
//
//	GET /api/grid/:grid_id
//	Response: model.GridState (camelCase with gridId, lat, lng, activeCount, stage, stageInfo)
func (h *GridHandler) Handle(c *gin.Context) {
	gridID := c.Param("grid_id")
	if gridID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "grid_id is required"})
		return
	}

	centerLat, centerLng, err := grid.GridIDToCenter(gridID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid grid_id format"})
		return
	}

	now := time.Now().Unix()
	activeCount, err := h.getActiveCount(c.Request.Context(), gridID, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to query fire state"})
		return
	}

	state := model.BuildGridState(gridID, int(activeCount), centerLat, centerLng)
	c.JSON(http.StatusOK, state)
}

// getActiveCount counts active (non-expired) fires in a grid cell.
// Uses ZCOUNT fire:{gridId} with score range [now, +inf].
func (h *GridHandler) getActiveCount(ctx context.Context, gridID string, now int64) (int64, error) {
	key := fmt.Sprintf("%s%s", engine.FireKeyPrefix, gridID)
	return h.redis.ZCount(ctx, key, fmt.Sprintf("%d", now), "+inf")
}

// viewportResponse matches the viewport API response shape.
type viewportResponse struct {
	Grids            []model.GridState `json:"grids"`
	TotalActiveGrids int               `json:"totalActiveGrids"`
}

// HandleViewport returns fire states for all grid cells within a map viewport.
//
//	GET /api/grid/viewport?neLat=...&neLng=...&swLat=...&swLng=...
//	Response: {"grids": [...], "totalActiveGrids": N}
func (h *GridHandler) HandleViewport(c *gin.Context) {
	neLat, err := strconv.ParseFloat(c.Query("neLat"), 64)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "neLat is required and must be a number"})
		return
	}
	neLng, err := strconv.ParseFloat(c.Query("neLng"), 64)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "neLng is required and must be a number"})
		return
	}
	swLat, err := strconv.ParseFloat(c.Query("swLat"), 64)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "swLat is required and must be a number"})
		return
	}
	swLng, err := strconv.ParseFloat(c.Query("swLng"), 64)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "swLng is required and must be a number"})
		return
	}

	gridIDs := grid.GetGridsInViewport(neLat, neLng, swLat, swLng)
	now := time.Now().Unix()
	ctx := c.Request.Context()

	var gridStates []model.GridState
	for _, gridID := range gridIDs {
		activeCount, err := h.getActiveCount(ctx, gridID, now)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to query fire state"})
			return
		}
		if activeCount > 0 {
			centerLat, centerLng, cerr := grid.GridIDToCenter(gridID)
			if cerr != nil {
				continue
			}
			state := model.BuildGridState(gridID, int(activeCount), centerLat, centerLng)
			gridStates = append(gridStates, state)
		}
	}

	if gridStates == nil {
		gridStates = []model.GridState{}
	}

	c.JSON(http.StatusOK, viewportResponse{
		Grids:            gridStates,
		TotalActiveGrids: len(gridStates),
	})
}

// Register adds the grid endpoints to the given Gin router group.
func (h *GridHandler) Register(r gin.IRouter) {
	r.GET("/api/grid/viewport", h.HandleViewport)
	r.GET("/api/grid/:grid_id", h.Handle)
}
