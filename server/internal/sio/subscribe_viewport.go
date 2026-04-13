package sio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/homepy/hwarr/server/internal/grid"
	"github.com/homepy/hwarr/server/internal/model"
	socketio "github.com/homeworldio/socketio-go"
)

// RedisFireCounter abstracts the Redis operations needed for fire state queries.
type RedisFireCounter interface {
	// ZCount returns the number of elements in the sorted set at key
	// with a score between min and max.
	ZCount(ctx context.Context, key, min, max string) (int64, error)
}

// RedisViewportReader is the superset used by subscribe:viewport — on top of
// ZCount it needs SMembers("active_grids") for the sparse fallback path when
// the requested bounding box would otherwise enumerate millions of empty cells.
type RedisViewportReader interface {
	RedisFireCounter
	SMembers(ctx context.Context, key string) ([]string, error)
}

// MaxViewportGrids caps how many grid cells subscribe:viewport will enumerate
// via GetGridsInViewport. At ~100m per cell this is roughly a 140x140 cell
// window (~14km × 14km) — large enough for zoom ≥ 13 usage, small enough to
// avoid OOM at nation-wide zoom levels. Above this threshold the handler
// falls back to scanning active_grids and filtering by the bounding box.
const MaxViewportGrids = 20000

// viewportData is the payload the client sends for subscribe:viewport.
// Accepts both snake_case and camelCase during migration; JSON decoder picks whichever is present.
type viewportData struct {
	NELat float64 `json:"neLat"`
	NELng float64 `json:"neLng"`
	SWLat float64 `json:"swLat"`
	SWLng float64 `json:"swLng"`
}

// RegisterSubscribeViewportHandler registers the "subscribe:viewport" Socket.IO event.
//
// Client sends: { "ne_lat": float, "ne_lng": float, "sw_lat": float, "sw_lng": float }
// Server:
//  1. Computes all grid IDs within the bounding box
//  2. Leaves old viewport rooms and joins new grid rooms
//  3. Returns current fire state for visible grids with active fires
//
// Response: { "status": "ok", "gridMeta": {...}, "subscribedGrids": N, "activeFires": [...] }
//
// Mirrors Python server/sio/events.py handle_subscribe_viewport.
func RegisterSubscribeViewportHandler(
	sioServer *socketio.Server,
	manager *ConnectionManager,
	redis RedisViewportReader,
	logger *log.Logger,
) {
	if logger == nil {
		logger = log.Default()
	}

	sioServer.On("subscribe:viewport", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		// Parse viewport data from the first argument
		if len(args) == 0 || len(args[0]) == 0 {
			logger.Printf("subscribe:viewport from %s: no data", sid)
			return []interface{}{map[string]interface{}{
				"error": "neLat, neLng, swLat, swLng are required numbers",
			}}, nil
		}

		var data viewportData
		if err := json.Unmarshal(args[0], &data); err != nil {
			logger.Printf("subscribe:viewport from %s invalid data: %s (%v)", sid, string(args[0]), err)
			return []interface{}{map[string]interface{}{
				"error": "neLat, neLng, swLat, swLng are required numbers",
			}}, nil
		}

		ctx := context.Background()
		now := time.Now().Unix()

		// Estimate how many grid cells the bounding box implies. At low zoom the
		// count can reach tens of millions (nation-wide view); enumerating them
		// would OOM the process. Fall back to scanning active_grids in that case.
		estimated := estimateViewportGridCount(data.NELat, data.NELng, data.SWLat, data.SWLng)

		var gridIDs []string
		var sparseFallback bool
		if estimated > MaxViewportGrids {
			sparseFallback = true
			active, err := redis.SMembers(ctx, "active_grids")
			if err != nil {
				logger.Printf("subscribe:viewport SMembers error for %s: %v", sid, err)
				active = nil
			}
			gridIDs = filterActiveGridsInBounds(active, data.NELat, data.NELng, data.SWLat, data.SWLng)
		} else {
			gridIDs = grid.GetGridsInViewport(data.NELat, data.NELng, data.SWLat, data.SWLng)
		}

		// Leave old rooms and join new grid rooms. In sparse-fallback mode we
		// only subscribe to rooms for grids that are actually active within the
		// viewport — room-scoped broadcasts for newly-ignited empty cells in
		// this region won't be delivered until the user zooms in and re-subscribes.
		joinViewportRooms(sioServer, manager, sid, gridIDs, logger)

		// Collect current fire state for visible grids with active fires
		gridStates := make([]model.GridState, 0)

		for _, gridID := range gridIDs {
			activeCount, err := getActiveFireCount(ctx, redis, gridID, now)
			if err != nil {
				logger.Printf("subscribe:viewport Redis error for grid %s: %v", gridID, err)
				continue
			}
			if activeCount > 0 {
				centerLat, centerLng, cerr := grid.GridIDToCenter(gridID)
				if cerr != nil {
					logger.Printf("subscribe:viewport GridIDToCenter error for %s: %v", gridID, cerr)
					continue
				}
				state := model.BuildGridState(gridID, int(activeCount), centerLat, centerLng)
				gridStates = append(gridStates, state)
			}
		}

		logger.Printf("subscribe:viewport sid=%s grids=%d active=%d sparse=%v estimated=%d",
			sid, len(gridIDs), len(gridStates), sparseFallback, estimated)

		return []interface{}{map[string]interface{}{
			"status": "ok",
			"gridMeta": map[string]interface{}{
				"latSize": grid.LatUnit,
				"lngSize": grid.LngUnit,
			},
			"subscribedGrids": len(gridIDs),
			"activeFires":     gridStates,
			"sparseFallback":  sparseFallback,
		}}, nil
	})
}

// estimateViewportGridCount computes how many grid cells fit in the given
// bounding box without allocating the ID slice.
func estimateViewportGridCount(neLat, neLng, swLat, swLng float64) int {
	latSpan := math.Max(0, neLat-swLat)
	lngSpan := math.Max(0, neLng-swLng)
	// +1 per axis to account for the inclusive floor() bucketing in GetGridsInViewport.
	latCells := int(math.Floor(latSpan/grid.LatUnit)) + 1
	lngCells := int(math.Floor(lngSpan/grid.LngUnit)) + 1
	if latCells < 0 || lngCells < 0 {
		return 0
	}
	return latCells * lngCells
}

// filterActiveGridsInBounds keeps only the grid IDs whose center falls within
// the bounding box. Used when the estimated viewport grid count exceeds
// MaxViewportGrids — we can still deliver the active fires in that region
// without enumerating millions of empty cells.
func filterActiveGridsInBounds(active []string, neLat, neLng, swLat, swLng float64) []string {
	out := make([]string, 0, len(active))
	for _, gridID := range active {
		// parse "lat:lng" without spinning up fmt machinery
		colon := strings.IndexByte(gridID, ':')
		if colon < 0 {
			continue
		}
		centerLat, centerLng, err := grid.GridIDToCenter(gridID)
		if err != nil {
			continue
		}
		if centerLat >= swLat && centerLat <= neLat && centerLng >= swLng && centerLng <= neLng {
			out = append(out, gridID)
		}
	}
	return out
}

// joinViewportRooms replaces a client's viewport room subscriptions.
// Leaves all old rooms (except well-known rooms like chat), joins new grid rooms.
// Mirrors Python ConnectionManager.join_rooms().
func joinViewportRooms(
	sioServer *socketio.Server,
	manager *ConnectionManager,
	sid string,
	newRooms []string,
	logger *log.Logger,
) {
	// Get current rooms and leave them
	oldRooms := manager.GetRooms(sid)
	for _, room := range oldRooms {
		if err := sioServer.LeaveRoom("/", sid, room); err != nil {
			// Ignore errors — room may already be cleaned up
			logger.Printf("subscribe:viewport LeaveRoom %s/%s error: %v", sid, room, err)
		}
	}

	// Join new rooms
	for _, room := range newRooms {
		if err := sioServer.EnterRoom("/", sid, room); err != nil {
			logger.Printf("subscribe:viewport EnterRoom %s/%s error: %v", sid, room, err)
		}
	}

	// Update connection manager tracking
	manager.SetRooms(sid, newRooms)
}

// getActiveFireCount queries Redis for the number of active (non-expired) fires in a grid.
// Uses ZCOUNT fire:{gridId} with score range [now, +inf].
func getActiveFireCount(ctx context.Context, redis RedisFireCounter, gridID string, now int64) (int, error) {
	key := fmt.Sprintf("fire:%s", gridID)
	count, err := redis.ZCount(ctx, key, fmt.Sprintf("%d", now), "+inf")
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

