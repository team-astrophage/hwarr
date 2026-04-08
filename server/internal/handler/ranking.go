package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/homepy/hwarr/server/internal/model"
)

// Redis key prefix for daily ranking sorted sets.
const StatsDailyRankingPrefix = "stats:daily_ranking:"

// RedisRankingReader abstracts the Redis operations needed by RankingHandler.
type RedisRankingReader interface {
	// ZRevRangeWithScores returns the specified range of elements in the
	// sorted set at key, ordered from high to low score.
	ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]model.ZMember, error)
}

// RankingHandler serves the GET /api/ranking/today endpoint.
type RankingHandler struct {
	redis RedisRankingReader
}

// NewRankingHandler creates a new RankingHandler.
func NewRankingHandler(redis RedisRankingReader) *RankingHandler {
	return &RankingHandler{redis: redis}
}

// rankingItem represents a single ranking entry.
type rankingItem struct {
	Rank   int    `json:"rank"`
	Region string `json:"region"`
	Count  int    `json:"count"`
}

// rankingResponse is the response shape for GET /api/ranking/today.
type rankingResponse struct {
	Date  string        `json:"date"`
	Items []rankingItem `json:"items"`
}

// Handle returns today's top arson regions (KST).
//
//	GET /api/ranking/today?limit=10
//	Response: {"date": "YYYY-MM-DD", "items": [{"rank": 1, "region": "...", "count": N}, ...]}
func (h *RankingHandler) Handle(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse limit query parameter (default=10, min=1, max=50).
	limit := 10
	if raw := c.Query("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}

	// KST = UTC+9
	kst := time.FixedZone("KST", 9*60*60)
	todayStr := time.Now().In(kst).Format("2006-01-02")
	key := StatsDailyRankingPrefix + todayStr

	// ZREVRANGE with scores → top N regions by fire count.
	members, err := h.redis.ZRevRangeWithScores(ctx, key, 0, int64(limit-1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to query ranking"})
		return
	}

	items := make([]rankingItem, 0, len(members))
	for i, m := range members {
		items = append(items, rankingItem{
			Rank:   i + 1,
			Region: m.Member,
			Count:  int(m.Score),
		})
	}

	c.JSON(http.StatusOK, rankingResponse{
		Date:  todayStr,
		Items: items,
	})
}

// Register adds the ranking endpoint to the given Gin router.
func (h *RankingHandler) Register(r gin.IRouter) {
	r.GET("/api/ranking/today", h.Handle)
}
