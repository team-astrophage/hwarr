package sio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/homepy/hwarr/server/internal/geodata"
	"github.com/homepy/hwarr/server/internal/grid"
	"github.com/homepy/hwarr/server/internal/model"
	socketio "github.com/homeworldio/socketio-go"
)

// fireCompatData is the client payload for the "fire" compat event.
// Frontend sends: { "lat": float, "lng": float }
type fireCompatData struct {
	Lat *float64 `json:"lat,omitempty"`
	Lng *float64 `json:"lng,omitempty"`
}

// RegisterFireCompatHandler registers the "fire" Socket.IO event (mock server compat).
//
// This handler mirrors the Python handle_fire_compat in server/sio/events.py.
// It accepts the same payload as fire:ignite but broadcasts "fire:update" in
// camelCase format, matching the original mock server's event format.
//
// Client sends:  { "lat": float, "lng": float }
// Server:
//  1. Converts GPS to grid ID
//  2. Registers fire event in Redis (with neighbor spreading if threshold exceeded)
//  3. Broadcasts "fire:update" with camelCase keys to all clients
//
// Unlike fire:ignite, this handler does NOT support demo mode fallback and does
// NOT return an ack to the sender — it only broadcasts fire:update globally.
func RegisterFireCompatHandler(
	sioServer *socketio.Server,
	manager *ConnectionManager,
	redis RedisFireWriter,
	resolver *geodata.AdminRegionResolver,
	logger *log.Logger,
	batcher *FireBatcher,
) {
	if logger == nil {
		logger = log.Default()
	}

	sioServer.On("fire", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		// Parse client payload
		if len(args) == 0 || len(args[0]) == 0 {
			return nil, nil
		}

		var data fireCompatData
		if err := json.Unmarshal(args[0], &data); err != nil {
			return nil, nil
		}

		// Validate lat/lng presence
		if data.Lat == nil || data.Lng == nil {
			return nil, nil
		}

		lat := *data.Lat
		lng := *data.Lng

		// GPS -> grid ID
		requestedGridID := grid.ToGridID(lat, lng)

		// Generate unique event ID
		eventID := fmt.Sprintf("fire-%s", randomHexSIO(12))
		expireAt := float64(time.Now().Unix()) + FireTTLSec

		// Register fire in Redis (may spread to neighbor if threshold exceeded)
		ctx := context.Background()
		reg, err := registerFireEvent(ctx, redis, requestedGridID, eventID, expireAt, resolver, logger)
		if err != nil {
			logger.Printf("fire (compat) from %s Redis error: %v", sid, err)
			return nil, nil
		}

		gridID := reg.GridID
		activeCount := reg.ActiveCount

		// When fire spread to a neighbor, use the landing grid's center
		// coordinates so the broadcast is consistent with the grid_id.
		if gridID != requestedGridID {
			centerLat, centerLng, centerErr := grid.GridIDToCenter(gridID)
			if centerErr == nil {
				lat = centerLat
				lng = centerLng
			}
		}

		// Build grid state with stage info
		latP := &lat
		lngP := &lng
		state := model.BuildGridState(gridID, activeCount, latP, lngP)

		// Enqueue batched room-scoped update (replaces global broadcast)
		batcher.Add(FireUpdate{
			GridID:      gridID,
			ActiveCount: activeCount,
			Stage:       state.Stage,
			Lat:         &lat,
			Lng:         &lng,
		})

		logger.Printf("fire (compat) sid=%s requested=%s landed=%s count=%d stage=%d spread=%d",
			sid, requestedGridID, gridID, activeCount, state.Stage, len(reg.SpreadPath))

		return nil, nil
	})
}
