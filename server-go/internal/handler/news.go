package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/homepy/hwarr/server-go/internal/grid"
	"github.com/homepy/hwarr/server-go/internal/model"
)

// MaxNewsItems is the maximum number of news entries returned.
const MaxNewsItems = 5

// newsTTLSec is the default TTL for fire entries (used to back-calculate ignite time).
var newsTTLSec = func() float64 {
	if v := os.Getenv("NEWS_TTL_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return float64(n)
		}
	}
	return 86400 // 1 day default
}()

// RedisNewsReader abstracts the Redis operations needed by NewsHandler.
type RedisNewsReader interface {
	// SMembers returns all members of the set at key.
	SMembers(ctx context.Context, key string) ([]string, error)
	// ZCount returns the number of elements in the sorted set at key
	// with a score between min and max.
	ZCount(ctx context.Context, key, min, max string) (int64, error)
	// ZRevRangeWithScores returns the specified range of elements in the
	// sorted set at key, ordered from high to low score.
	ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]model.ZMember, error)
}

// NewsHandler serves the GET /api/news endpoint.
type NewsHandler struct {
	redis RedisNewsReader
}

// NewNewsHandler creates a new NewsHandler.
func NewNewsHandler(redis RedisNewsReader) *NewsHandler {
	return &NewsHandler{redis: redis}
}

// gridEntry holds intermediate data for sorting and building news items.
type gridEntry struct {
	gridID   string
	count    int
	stage    model.FireStage
	igniteTS float64
}

// Handle returns the breaking-news feed for the landing page.
//
//	GET /api/news
//	Response: [{"id": "...", "icon": "🔥", ...}, ...]
func (h *NewsHandler) Handle(c *gin.Context) {
	ctx := c.Request.Context()
	now := float64(time.Now().Unix())

	// 1. Get all active grid IDs
	gridIDs, err := h.redis.SMembers(ctx, "active_grids")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to query active grids"})
		return
	}

	// 2. For each grid, count active fires and determine stage
	var grids []gridEntry
	for _, gridID := range gridIDs {
		count, err := h.getActiveCount(ctx, gridID, now)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to query fire state"})
			return
		}
		if count <= 0 {
			continue
		}

		stage := model.GetStage(int(count))
		igniteTS, err := h.getLatestIgniteTS(ctx, gridID)
		if err != nil {
			// Non-fatal: use current time as fallback
			igniteTS = now
		}

		grids = append(grids, gridEntry{
			gridID:   gridID,
			count:    int(count),
			stage:    stage,
			igniteTS: igniteTS,
		})
	}

	// 3. Sort by (stage desc, count desc)
	sort.Slice(grids, func(i, j int) bool {
		if grids[i].stage != grids[j].stage {
			return grids[i].stage > grids[j].stage
		}
		return grids[i].count > grids[j].count
	})

	// 4. Take top N
	if len(grids) > MaxNewsItems {
		grids = grids[:MaxNewsItems]
	}

	// 5. Build news items
	items := make([]model.NewsItem, 0, len(grids))
	for _, g := range grids {
		timeAgo := model.FormatTimeAgo(now - g.igniteTS)
		location := grid.GetLocationName(g.gridID)
		cfg := model.StageConfigs[g.stage]
		stageLabel := cfg.LabelKo

		var item model.NewsItem
		if g.stage >= 4 {
			item = model.BuildBigFire(location, stageLabel, g.count, timeAgo, g.gridID)
		} else if g.stage >= 2 {
			item = model.BuildFireActive(location, stageLabel, g.count, timeAgo, g.gridID)
		} else {
			item = model.BuildSmallFire(location, g.count, timeAgo, g.gridID)
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, items)
}

// getActiveCount counts active (non-expired) fires in a grid cell.
func (h *NewsHandler) getActiveCount(ctx context.Context, gridID string, now float64) (int64, error) {
	key := fmt.Sprintf("%s%s", FireKeyPrefix, gridID)
	return h.redis.ZCount(ctx, key, fmt.Sprintf("%f", now), "+inf")
}

// getLatestIgniteTS estimates the most recent ignite timestamp for a grid.
// It fetches the highest-scored member and subtracts NEWS_TTL_SEC.
func (h *NewsHandler) getLatestIgniteTS(ctx context.Context, gridID string) (float64, error) {
	key := fmt.Sprintf("%s%s", FireKeyPrefix, gridID)
	members, err := h.redis.ZRevRangeWithScores(ctx, key, 0, 0)
	if err != nil {
		return 0, err
	}
	if len(members) == 0 {
		return 0, fmt.Errorf("no members in sorted set for grid %s", gridID)
	}
	return members[0].Score - newsTTLSec, nil
}

// Register adds the news endpoint to the given Gin router group.
func (h *NewsHandler) Register(r gin.IRouter) {
	r.GET("/api/news", h.Handle)
}
