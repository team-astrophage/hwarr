package engineio

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ServerConfig holds Engine.IO server configuration.
type ServerConfig struct {
	PingInterval time.Duration // how often to send ping (default: 25s)
	PingTimeout  time.Duration // how long to wait for pong (default: 20s)
	MaxPayload   int64         // max payload size in bytes (default: 1MB)
	// Upgrades lists available transport upgrades.
	// For polling sessions, this typically includes "websocket".
	Upgrades []string
}

// DefaultConfig returns a default Engine.IO server configuration.
func DefaultConfig() ServerConfig {
	return ServerConfig{
		PingInterval: 25 * time.Second,
		PingTimeout:  20 * time.Second,
		MaxPayload:   1_000_000,
		Upgrades:     []string{"websocket"},
	}
}

// openPacketData is the JSON structure sent in the OPEN packet.
type openPacketData struct {
	SID          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int      `json:"pingInterval"` // milliseconds
	PingTimeout  int      `json:"pingTimeout"`  // milliseconds
	MaxPayload   int64    `json:"maxPayload"`
}

// Server implements the Engine.IO v4 server.
type Server struct {
	config   ServerConfig
	sessions sync.Map // map[string]*Session

	mu        sync.RWMutex
	onConnect func(s *Session) // callback when a new session is created
}

// NewServer creates a new Engine.IO server with the given configuration.
func NewServer(config ServerConfig) *Server {
	srv := &Server{
		config: config,
	}
	// Start ping/timeout loop
	go srv.pingLoop()
	return srv
}

// OnConnect sets the callback invoked when a new Engine.IO session is established.
func (srv *Server) OnConnect(fn func(s *Session)) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.onConnect = fn
}

// GetSession retrieves a session by its ID.
func (srv *Server) GetSession(sid string) *Session {
	if v, ok := srv.sessions.Load(sid); ok {
		return v.(*Session)
	}
	return nil
}

// RemoveSession removes a session from the server.
func (srv *Server) RemoveSession(sid string) {
	srv.sessions.Delete(sid)
}

// ServeHTTP handles Engine.IO HTTP requests (both GET and POST for polling).
func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	query := r.URL.Query()
	transport := query.Get("transport")

	switch transport {
	case "polling":
		// handled below
	case "websocket":
		srv.handleWebSocket(w, r)
		return
	default:
		srv.sendError(w, http.StatusBadRequest, "Transport unknown")
		return
	}

	sid := query.Get("sid")

	if sid == "" {
		// New connection — handshake
		srv.handleHandshake(w, r)
		return
	}

	// Existing session
	session := srv.GetSession(sid)
	if session == nil {
		srv.sendError(w, http.StatusBadRequest, "Session ID unknown")
		return
	}

	if session.IsClosed() {
		srv.sendError(w, http.StatusBadRequest, "Session closed")
		return
	}

	switch r.Method {
	case http.MethodGet:
		srv.handlePollingGet(w, r, session)
	case http.MethodPost:
		srv.handlePollingPost(w, r, session)
	default:
		srv.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleHandshake processes a new connection request.
func (srv *Server) handleHandshake(w http.ResponseWriter, r *http.Request) {
	session := NewSession(srv.config.PingInterval, srv.config.PingTimeout)

	// Store the session
	srv.sessions.Store(session.ID, session)

	// Set up session close handler to clean up
	session.SetOnClose(func(sid string, reason string) {
		srv.RemoveSession(sid)
	})

	// Build the open packet
	upgrades := srv.config.Upgrades
	if upgrades == nil {
		upgrades = []string{}
	}

	openData := openPacketData{
		SID:          session.ID,
		Upgrades:     upgrades,
		PingInterval: int(srv.config.PingInterval / time.Millisecond),
		PingTimeout:  int(srv.config.PingTimeout / time.Millisecond),
		MaxPayload:   srv.config.MaxPayload,
	}

	jsonData, err := json.Marshal(openData)
	if err != nil {
		srv.sendError(w, http.StatusInternalServerError, "Failed to marshal open packet")
		return
	}

	openPacket := &Packet{
		Type: PacketOpen,
		Data: jsonData,
	}

	// Invoke the onConnect callback
	srv.mu.RLock()
	onConnect := srv.onConnect
	srv.mu.RUnlock()

	if onConnect != nil {
		onConnect(session)
	}

	// Send the open packet as the response
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, openPacket.Encode())
}

// handlePollingGet handles long-polling GET requests.
// The client calls GET to receive queued server-to-client packets.
func (srv *Server) handlePollingGet(w http.ResponseWriter, r *http.Request, session *Session) {
	// Use a reasonable long-poll timeout (slightly less than ping interval)
	timeout := srv.config.PingInterval

	packets := session.Drain(timeout)

	if len(packets) == 0 {
		// Send a NOOP packet to keep the connection alive
		packets = []*Packet{{Type: PacketNoop}}
	}

	payload := EncodePayload(packets)

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, payload)
}

// handlePollingPost handles polling POST requests.
// The client sends packets to the server via POST.
func (srv *Server) handlePollingPost(w http.ResponseWriter, r *http.Request, session *Session) {
	// Read the request body
	body, err := io.ReadAll(io.LimitReader(r.Body, srv.config.MaxPayload))
	if err != nil {
		srv.sendError(w, http.StatusBadRequest, "Failed to read body")
		return
	}
	defer r.Body.Close()

	// Decode the payload
	packets, err := DecodePayload(string(body))
	if err != nil {
		srv.sendError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	// Process each packet
	for _, p := range packets {
		session.HandlePacket(p)
	}

	// Respond with "ok"
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

// sendError writes a JSON error response.
func (srv *Server) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]interface{}{
		"code":    3, // Engine.IO error code for server error
		"message": message,
	}
	json.NewEncoder(w).Encode(resp)
}

// pingLoop periodically sends ping packets and closes timed-out sessions.
func (srv *Server) pingLoop() {
	ticker := time.NewTicker(srv.config.PingInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		srv.sessions.Range(func(key, value interface{}) bool {
			session := value.(*Session)
			if session.IsClosed() {
				srv.sessions.Delete(key)
				return true
			}

			// Check if session has timed out
			deadline := session.LastActive().Add(srv.config.PingInterval + srv.config.PingTimeout)
			if now.After(deadline) {
				session.Close("ping timeout")
				srv.sessions.Delete(key)
				return true
			}

			// Send a ping packet
			session.SendPacket(&Packet{Type: PacketPing})
			return true
		})
	}
}
