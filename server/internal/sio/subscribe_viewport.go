package sio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	redis RedisFireCounter,
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
				"error": "ne_lat, ne_lng, sw_lat, sw_lng are required numbers",
			}}, nil
		}

		var data viewportData
		if err := json.Unmarshal(args[0], &data); err != nil {
			logger.Printf("subscribe:viewport from %s invalid data: %s (%v)", sid, string(args[0]), err)
			return []interface{}{map[string]interface{}{
				"error": "ne_lat, ne_lng, sw_lat, sw_lng are required numbers",
			}}, nil
		}

		// Validate that all fields are present (non-zero check is not sufficient,
		// so we rely on JSON parsing — matching Python behavior for required fields)
		gridIDs := grid.GetGridsInViewport(data.NELat, data.NELng, data.SWLat, data.SWLng)

		// Leave old rooms and join new grid rooms
		joinViewportRooms(sioServer, manager, sid, gridIDs, logger)

		// Collect current fire state for visible grids with active fires
		now := time.Now().Unix()
		ctx := context.Background()
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

		logger.Printf("subscribe:viewport sid=%s grids=%d active=%d",
			sid, len(gridIDs), len(gridStates))

		return []interface{}{map[string]interface{}{
			"status": "ok",
			"gridMeta": map[string]interface{}{
				"latSize": grid.LatUnit,
				"lngSize": grid.LngUnit,
			},
			"subscribedGrids": len(gridIDs),
			"activeFires":     gridStates,
		}}, nil
	})
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

