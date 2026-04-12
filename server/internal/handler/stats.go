package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/homepy/hwarr/server/internal/engine"
)

// Redis key constants for stats.
const (
	StatsTotalFiresKey   = "stats:total_fires"
	StatsDailyFirePrefix = "stats:daily_fires:"
	ActiveGridsKey       = "active_grids"
)

// RedisStatsReader abstracts the Redis operations needed by StatsHandler.
type RedisStatsReader interface {
	// SMembers returns all members of a set.
	SMembers(ctx context.Context, key string) ([]string, error)
	// ZCount returns the number of elements in the sorted set with score between min and max.
	ZCount(ctx context.Context, key, min, max string) (int64, error)
	// Get returns the string value of a key.
	Get(ctx context.Context, key string) (string, error)
}

// StatsHandler serves the GET /api/stats endpoint.
type StatsHandler struct {
	redis       RedisStatsReader
	connections ConnectionCounter
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(redis RedisStatsReader, connections ConnectionCounter) *StatsHandler {
	return &StatsHandler{
		redis:       redis,
		connections: connections,
	}
}

// statsResponse matches the Python stats endpoint response (camelCase).
type statsResponse struct {
	ActiveGrids     int `json:"activeGrids"`
	TotalFires      int `json:"totalFires"`
	CumulativeFires int `json:"cumulativeFires"`
	DailyFires      int `json:"dailyFires"`
	OnlineUsers     int `json:"onlineUsers"`
}

// Handle returns global fire statistics.
//
//	GET /api/stats
//	Response: {"activeGrids": N, "totalFires": N, "cumulativeFires": N, "dailyFires": N, "onlineUsers": N}
func (h *StatsHandler) Handle(c *gin.Context) {
	ctx := c.Request.Context()
	now := fmt.Sprintf("%d", time.Now().Unix())

	// Get all active grid IDs from the set
	gridIDs, err := h.redis.SMembers(ctx, ActiveGridsKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to query active grids"})
		return
	}

	activeGrids := 0
	totalFires := 0
	for _, gridID := range gridIDs {
		key := fmt.Sprintf("%s%s", engine.FireKeyPrefix, gridID)
		count, err := h.redis.ZCount(ctx, key, now, "+inf")
		if err != nil {
			continue
		}
		if count > 0 {
			activeGrids++
			totalFires += int(count)
		}
	}

	// Cumulative total fires (persisted across restarts with Redis AOF)
	cumulativeFires := 0
	if raw, err := h.redis.Get(ctx, StatsTotalFiresKey); err == nil && raw != "" {
		fmt.Sscanf(raw, "%d", &cumulativeFires)
	}

	// Today's fire count (KST = UTC+9)
	kst := time.FixedZone("KST", 9*60*60)
	todayStr := time.Now().In(kst).Format("2006-01-02")
	todayKey := fmt.Sprintf("%s%s", StatsDailyFirePrefix, todayStr)
	dailyFires := 0
	if raw, err := h.redis.Get(ctx, todayKey); err == nil && raw != "" {
		fmt.Sscanf(raw, "%d", &dailyFires)
	}

	// Online users from connection counter
	onlineUsers := 0
	if h.connections != nil {
		onlineUsers = h.connections.ActiveCount()
	}

	c.JSON(http.StatusOK, statsResponse{
		ActiveGrids:     activeGrids,
		TotalFires:      totalFires,
		CumulativeFires: cumulativeFires,
		DailyFires:      dailyFires,
		OnlineUsers:     onlineUsers,
	})
}

// Register adds the stats endpoint to the given Gin router.
func (h *StatsHandler) Register(r gin.IRouter) {
	r.GET("/api/stats", h.Handle)
}
