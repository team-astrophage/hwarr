// Package sio provides application-level Socket.IO connection management.
//
// It mirrors server/sio/connection_manager.py — tracking active connections,
// heartbeat state, room subscriptions, and reconnection support via user_id.
package sio

import (
	"log"
	"net"
	"sync"
	"time"
)

// Heartbeat / reaper configuration — matches Python server values.
const (
	HeartbeatIntervalSec = 10 // server expects a heartbeat every 10s
	HeartbeatTimeoutSec  = 30 // client considered stale after 30s
	ReaperIntervalSec    = 15 // reaper check interval
)

// ConnectionInfo holds metadata for a single connected client.
// Mirrors Python's ConnectionInfo dataclass.
type ConnectionInfo struct {
	SID            string
	ConnectedAt    float64 // Unix timestamp (seconds)
	LastHeartbeat  float64
	Rooms          map[string]struct{} // set of room names (grid IDs)
	UserID         string              // optional stable ID for reconnection
	ReconnectCount int
	RemoteAddr     string // client IP address for per-IP connection limiting
}

// newConnectionInfo creates a ConnectionInfo with sensible defaults.
func newConnectionInfo(sid string, userID string, now float64, reconnectCount int) *ConnectionInfo {
	return &ConnectionInfo{
		SID:            sid,
		ConnectedAt:    now,
		LastHeartbeat:  now,
		Rooms:          make(map[string]struct{}),
		UserID:         userID,
		ReconnectCount: reconnectCount,
	}
}

// RoomNames returns a copy of the room set as a string slice.
func (ci *ConnectionInfo) RoomNames() []string {
	rooms := make([]string, 0, len(ci.Rooms))
	for r := range ci.Rooms {
		rooms = append(rooms, r)
	}
	return rooms
}

// ConnectionManager tracks active Socket.IO connections.
//
// Thread-safe: all exported methods are protected by a mutex.
//
// Features (matching Python implementation):
//   - Connection add/remove lifecycle
//   - Heartbeat tracking with stale connection detection
//   - Room-based viewport subscriptions
//   - Reconnection state restoration via user_id
type ConnectionManager struct {
	mu            sync.RWMutex
	connections   map[string]*ConnectionInfo // sid → ConnectionInfo
	userSessions  map[string]*ConnectionInfo // user_id → last known ConnectionInfo
	ipConnections map[string]int             // IP → active connection count
	logger        *log.Logger
}

// NewConnectionManager creates a new ConnectionManager.
func NewConnectionManager(logger *log.Logger) *ConnectionManager {
	if logger == nil {
		logger = log.Default()
	}
	return &ConnectionManager{
		connections:   make(map[string]*ConnectionInfo),
		userSessions:  make(map[string]*ConnectionInfo),
		ipConnections: make(map[string]int),
		logger:        logger,
	}
}

// ActiveCount returns the number of currently active connections.
func (m *ConnectionManager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// ActiveSIDs returns a copy of all active session IDs.
func (m *ConnectionManager) ActiveSIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sids := make([]string, 0, len(m.connections))
	for sid := range m.connections {
		sids = append(sids, sid)
	}
	return sids
}

// Get returns the ConnectionInfo for a given sid, or nil if not found.
func (m *ConnectionManager) Get(sid string) *ConnectionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[sid]
}

// Add registers a new connection. If the user_id was seen before,
// increments the reconnect count (matching Python behavior).
//
// Returns the newly created ConnectionInfo.
func (m *ConnectionManager) Add(sid string, userID string, remoteAddr string) *ConnectionInfo {
	now := float64(time.Now().UnixMilli()) / 1000.0 // seconds with ms precision
	reconnectCount := 0

	ip := extractIP(remoteAddr)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for previous session (reconnection detection)
	if userID != "" {
		if prev, ok := m.userSessions[userID]; ok {
			reconnectCount = prev.ReconnectCount + 1
			m.logger.Printf("Reconnection detected: user=%s prev_sid=%s new_sid=%s count=%d",
				userID, prev.SID, sid, reconnectCount)
		}
	}

	info := newConnectionInfo(sid, userID, now, reconnectCount)
	info.RemoteAddr = ip
	m.connections[sid] = info

	if ip != "" {
		m.ipConnections[ip]++
	}

	if userID != "" {
		m.userSessions[userID] = info
	}

	m.logger.Printf("Connection added: %s (total: %d)", sid, len(m.connections))
	return info
}

// Remove unregisters a connection and returns its ConnectionInfo.
// Returns nil if the sid was not found.
// Note: room cleanup in the Socket.IO namespace is the caller's responsibility.
func (m *ConnectionManager) Remove(sid string) *ConnectionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.connections[sid]
	if !ok {
		m.logger.Printf("Attempted to remove unknown sid: %s", sid)
		return nil
	}

	// Decrement IP connection count
	if info.RemoteAddr != "" {
		m.ipConnections[info.RemoteAddr]--
		if m.ipConnections[info.RemoteAddr] <= 0 {
			delete(m.ipConnections, info.RemoteAddr)
		}
	}

	delete(m.connections, sid)
	m.logger.Printf("Connection removed: %s (total: %d)", sid, len(m.connections))
	return info
}

// GetPreviousSession returns the last known session for a user_id.
// Used during reconnection to restore room subscriptions.
func (m *ConnectionManager) GetPreviousSession(userID string) *ConnectionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.userSessions[userID]
}

// RecordHeartbeat updates the last heartbeat timestamp for a sid.
// Returns true if the connection exists.
func (m *ConnectionManager) RecordHeartbeat(sid string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	info, ok := m.connections[sid]
	if !ok {
		return false
	}
	info.LastHeartbeat = float64(time.Now().UnixMilli()) / 1000.0
	return true
}

// IsStale checks if a connection has missed heartbeats.
func (m *ConnectionManager) IsStale(sid string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.connections[sid]
	if !ok {
		return true
	}
	now := float64(time.Now().UnixMilli()) / 1000.0
	return (now - info.LastHeartbeat) > HeartbeatTimeoutSec
}

// GetStaleSIDs returns all sids that have exceeded the heartbeat timeout.
func (m *ConnectionManager) GetStaleSIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := float64(time.Now().UnixMilli()) / 1000.0
	var stale []string
	for sid, info := range m.connections {
		if (now - info.LastHeartbeat) > HeartbeatTimeoutSec {
			stale = append(stale, sid)
		}
	}
	return stale
}

// SetRooms replaces the tracked rooms for a sid.
// This only updates the ConnectionManager's internal tracking;
// actual Socket.IO room join/leave is the caller's responsibility.
func (m *ConnectionManager) SetRooms(sid string, rooms []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	info, ok := m.connections[sid]
	if !ok {
		return
	}
	info.Rooms = make(map[string]struct{}, len(rooms))
	for _, r := range rooms {
		info.Rooms[r] = struct{}{}
	}
}

// GetRooms returns the current rooms for a sid.
func (m *ConnectionManager) GetRooms(sid string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.connections[sid]
	if !ok {
		return nil
	}
	return info.RoomNames()
}

// CanConnect checks whether the given remote address is allowed to open a new connection
// based on the per-IP connection limit.
func (m *ConnectionManager) CanConnect(remoteAddr string, maxPerIP int) bool {
	ip := extractIP(remoteAddr)
	if ip == "" {
		return true
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ipConnections[ip] < maxPerIP
}

// extractIP parses the host part from a host:port address string.
func extractIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// No port, use as-is
		return remoteAddr
	}
	return host
}
