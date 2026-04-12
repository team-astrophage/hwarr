package sio

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/homepy/hwarr/server/internal/auth"
	socketio "github.com/homeworldio/socketio-go"
)

// Handler wires application-level Socket.IO event handlers to a socketio.Server.
// It owns the ConnectionManager and translates low-level connect/disconnect
// callbacks into the same session lifecycle as the Python server.
type Handler struct {
	sio                  *socketio.Server
	manager              *ConnectionManager
	logger               *log.Logger
	tokenService         *auth.TokenService
	onDisconnectCleanup  func(sid string)
}

// NewHandler creates a Handler and registers event handlers on the Socket.IO server.
// onDisconnectCleanup is called on disconnect for additional cleanup (e.g., rate limiter removal).
func NewHandler(sioServer *socketio.Server, logger *log.Logger, tokenService *auth.TokenService, onDisconnectCleanup func(sid string)) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	h := &Handler{
		sio:                  sioServer,
		manager:              NewConnectionManager(logger),
		logger:               logger,
		tokenService:         tokenService,
		onDisconnectCleanup:  onDisconnectCleanup,
	}
	h.registerHandlers()
	return h
}

// Manager returns the underlying ConnectionManager for external access
// (e.g., broadcasting user counts, checking active connections).
func (h *Handler) Manager() *ConnectionManager {
	return h.manager
}

// registerHandlers registers connect/event handlers on the default namespace.
// Disconnect is registered separately via RegisterDisconnectHandler for chat:presence support.
func (h *Handler) registerHandlers() {
	h.sio.OnConnect(h.handleConnect)
	RegisterDisconnectHandler(h.sio, h.manager, h.logger, h.onDisconnectCleanup)
	h.sio.On("heartbeat", h.handleHeartbeat)
}

// connectAuth represents the auth payload sent by the client on connect.
type connectAuth struct {
	UserID string `json:"user_id,omitempty"`
	Token  string `json:"token,omitempty"`
}

// handleConnect processes a new Socket.IO connection.
//
// Flow (matching Python server/main.py connect handler):
//  1. Extract user_id from auth payload (for reconnection support)
//  2. Check for previous session to detect reconnection
//  3. Register connection in ConnectionManager (sid→session)
//  4. Restore room subscriptions on reconnect
//  5. Emit "connected" ack to the client
//  6. Broadcast "users:count" to all clients
func (h *Handler) handleConnect(sid string, authRaw json.RawMessage) error {
	h.logger.Printf("Client connecting: %s", sid)

	// 1. Extract auth payload
	var authData connectAuth
	if len(authRaw) > 0 {
		_ = json.Unmarshal(authRaw, &authData)
	}

	// 2. Validate HMAC token
	if authData.Token == "" {
		return fmt.Errorf("authentication required")
	}
	userID, tokenExpiry, err := h.tokenService.ValidateWithExpiry(authData.Token)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	// 3. Check for previous session (reconnection detection)
	var previousRooms []string
	if userID != "" {
		if prev := h.manager.GetPreviousSession(userID); prev != nil {
			previousRooms = prev.RoomNames()
		}
	}

	// 4. Register connection in manager (sid→session map)
	info := h.manager.Add(sid, userID)
	info.TokenExpiry = tokenExpiry

	// 4. Restore room subscriptions on reconnect
	if len(previousRooms) > 0 {
		h.manager.SetRooms(sid, previousRooms)
		for _, room := range previousRooms {
			_ = h.sio.EnterRoom("/", sid, room)
		}
		h.logger.Printf("Restored %d rooms for reconnected user=%s sid=%s",
			len(previousRooms), userID, sid)
	}

	// 5. Emit "connected" ack to the connecting client
	connectedPayload := map[string]interface{}{
		"sid":                sid,
		"connected_at":       info.ConnectedAt,
		"active_connections": h.manager.ActiveCount(),
		"heartbeat_interval": HeartbeatIntervalSec,
		"reconnect_count":    info.ReconnectCount,
		"restored_rooms":     len(previousRooms),
	}
	if err := h.sio.SendToSocket("/", sid, "connected", connectedPayload); err != nil {
		h.logger.Printf("Failed to send connected ack to %s: %v", sid, err)
	}

	// 6. Broadcast users:count to all clients
	countPayload := map[string]interface{}{
		"count": h.manager.ActiveCount(),
	}
	if _, err := h.sio.BroadcastToNamespace("/", "users:count", countPayload); err != nil {
		h.logger.Printf("Failed to broadcast users:count: %v", err)
	}

	h.logger.Printf("Client connected: %s (total: %d)", sid, h.manager.ActiveCount())
	return nil
}

// heartbeatData represents the optional payload sent by the client.
type heartbeatData struct {
	TS *float64 `json:"ts,omitempty"`
}

// handleHeartbeat processes a client heartbeat ping.
//
// Client sends:  {"ts": <client_timestamp>}  (optional)
// Server returns: {"status":"ok","server_ts":<float64>,"client_ts":<float64|null>}
//
// If the sid is unknown (not in ConnectionManager), returns {"error":"unknown_sid"}.
// Matches Python server/main.py handle_heartbeat exactly.
func (h *Handler) handleHeartbeat(sid string, args ...json.RawMessage) ([]interface{}, error) {
	recorded := h.manager.RecordHeartbeat(sid)
	if !recorded {
		h.logger.Printf("Heartbeat from unknown sid: %s", sid)
		return []interface{}{map[string]interface{}{
			"error": "unknown_sid",
		}}, nil
	}

	// Parse optional client data
	var data heartbeatData
	if len(args) > 0 && len(args[0]) > 0 {
		_ = json.Unmarshal(args[0], &data) // ignore error — data is optional
	}

	serverTS := float64(time.Now().UnixMilli()) / 1000.0

	result := map[string]interface{}{
		"status":    "ok",
		"server_ts": serverTS,
		"client_ts": nil,
	}
	if data.TS != nil {
		result["client_ts"] = *data.TS
	}

	return []interface{}{result}, nil
}
