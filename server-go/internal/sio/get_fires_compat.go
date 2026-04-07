package sio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/homepy/hwarr/server-go/internal/model"
	socketio "github.com/homeworldio/socketio-go"
)

// RedisGetFiresReader abstracts the Redis operations needed by the get_fires handler.
type RedisGetFiresReader interface {
	// SMembers returns all members of the set at key.
	SMembers(ctx context.Context, key string) ([]string, error)
	// ZCount returns the number of elements in the sorted set at key
	// with a score between min and max.
	ZCount(ctx context.Context, key, min, max string) (int64, error)
}

// RegisterGetFiresCompatHandler registers the "get_fires" Socket.IO event (mock server compat).
//
// This handler mirrors the Python handle_get_fires_compat in server/sio/events.py.
// It returns all active fires via "fires:sync" event to the requesting client,
// using camelCase keys matching the original mock server format.
//
// Client sends:  (no data required)
// Server emits:  "fires:sync" → [ { "gridId": str, "activeCount": int, "stage": int }, ... ]
//
// The response is sent only to the requesting client (not broadcast).
func RegisterGetFiresCompatHandler(
	sioServer *socketio.Server,
	redis RedisGetFiresReader,
	logger *log.Logger,
) {
	if logger == nil {
		logger = log.Default()
	}

	sioServer.On("get_fires", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		ctx := context.Background()
		now := float64(time.Now().Unix())

		// Get all active grid IDs from Redis
		gridIDs, err := redis.SMembers(ctx, "active_grids")
		if err != nil {
			logger.Printf("get_fires SMembers error: %v", err)
			// Send empty list on error
			if sendErr := sioServer.SendToSocket("/", sid, "fires:sync", []interface{}{}); sendErr != nil {
				logger.Printf("get_fires fires:sync send error: %v", sendErr)
			}
			return nil, nil
		}

		// Collect active fires with camelCase keys (mock server compat)
		firesList := make([]map[string]interface{}, 0)
		for _, gridID := range gridIDs {
			key := fmt.Sprintf("fire:%s", gridID)
			count, err := redis.ZCount(ctx, key, fmt.Sprintf("%f", now), "+inf")
			if err != nil {
				logger.Printf("get_fires ZCount error for grid %s: %v", gridID, err)
				continue
			}
			if count > 0 {
				stage := model.GetStage(int(count))
				firesList = append(firesList, map[string]interface{}{
					"gridId":      gridID,
					"activeCount": int(count),
					"stage":       int(stage),
				})
			}
		}

		// Send fires:sync to the requesting client only (not broadcast)
		if err := sioServer.SendToSocket("/", sid, "fires:sync", firesList); err != nil {
			logger.Printf("get_fires fires:sync send to %s failed: %v", sid, err)
		}

		logger.Printf("get_fires sid=%s active_grids=%d", sid, len(firesList))

		return nil, nil
	})
}
