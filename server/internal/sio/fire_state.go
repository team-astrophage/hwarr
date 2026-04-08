package sio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/homepy/hwarr/server/internal/model"
	socketio "github.com/homeworldio/socketio-go"
)

// RedisFireStateReader abstracts the Redis operations needed by the fire:state handler.
type RedisFireStateReader interface {
	// SMembers returns all members of the set at key.
	SMembers(ctx context.Context, key string) ([]string, error)
	// ZCount returns the number of elements in the sorted set at key
	// with a score between min and max.
	ZCount(ctx context.Context, key, min, max string) (int64, error)
}

// fireStateData is the optional payload the client sends for fire:state.
type fireStateData struct {
	GridIDs []string `json:"grid_ids"`
}

// RegisterFireStateHandler registers the "fire:state" Socket.IO event.
//
// Client sends: { "grid_ids": ["gridA", "gridB"] }  (optional — all active if omitted)
// Server returns: { "status": "ok", "grids": [...], "total_active_grids": N }
//
// Mirrors Python server/sio/events.py handle_fire_state.
func RegisterFireStateHandler(
	sioServer *socketio.Server,
	redis RedisFireStateReader,
	logger *log.Logger,
) {
	if logger == nil {
		logger = log.Default()
	}

	sioServer.On("fire:state", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		ctx := context.Background()
		now := float64(time.Now().Unix())

		// Parse optional grid_ids from the first argument
		var data fireStateData
		if len(args) > 0 && len(args[0]) > 0 {
			_ = json.Unmarshal(args[0], &data) // ignore error — data is optional
		}

		// Determine which grid IDs to query
		gridIDs := data.GridIDs
		if len(gridIDs) == 0 {
			// Return all active grids from Redis set
			members, err := redis.SMembers(ctx, "active_grids")
			if err != nil {
				logger.Printf("fire:state SMembers error: %v", err)
				return []interface{}{map[string]interface{}{
					"status":             "ok",
					"grids":              []interface{}{},
					"total_active_grids": 0,
				}}, nil
			}
			gridIDs = members
		}

		// Collect state for each grid with active fires
		gridStates := make([]map[string]interface{}, 0)
		for _, gridID := range gridIDs {
			key := fmt.Sprintf("fire:%s", gridID)
			count, err := redis.ZCount(ctx, key, fmt.Sprintf("%f", now), "+inf")
			if err != nil {
				logger.Printf("fire:state ZCount error for grid %s: %v", gridID, err)
				continue
			}
			if count > 0 {
				state := model.BuildGridState(gridID, int(count), nil, nil)
				gridStates = append(gridStates, gridStateToMap(state))
			}
		}

		logger.Printf("fire:state sid=%s requested=%d active=%d", sid, len(gridIDs), len(gridStates))

		return []interface{}{map[string]interface{}{
			"status":             "ok",
			"grids":              gridStates,
			"total_active_grids": len(gridStates),
		}}, nil
	})
}
