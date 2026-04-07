package engineio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsUpgrader is the default WebSocket upgrader for Engine.IO connections.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Engine.IO handles CORS at application level
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// wsConn wraps a gorilla/websocket.Conn with a write mutex for thread safety.
type wsConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// WriteTextMessage sends a text message over the WebSocket connection.
func (wc *wsConn) WriteTextMessage(data []byte) error {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	return wc.conn.WriteMessage(websocket.TextMessage, data)
}

// WritePacket encodes and sends an Engine.IO packet over the WebSocket.
// For WebSocket transport, each frame is exactly one EIO packet: <type_char>[data]
func (wc *wsConn) WritePacket(p *Packet) error {
	return wc.WriteTextMessage([]byte(p.Encode()))
}

// Close closes the underlying WebSocket connection.
func (wc *wsConn) Close() error {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	return wc.conn.Close()
}

// handleWebSocket handles a new WebSocket connection or an upgrade from polling.
func (srv *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	sid := query.Get("sid")

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade failed — response already written by upgrader
		return
	}

	wsc := &wsConn{conn: conn}

	if sid == "" {
		// Fresh WebSocket connection — perform handshake over WS
		srv.handleWSHandshake(wsc)
	} else {
		// Transport upgrade from polling → websocket
		session := srv.GetSession(sid)
		if session == nil {
			wsc.WriteTextMessage([]byte(fmt.Sprintf(`{"code":3,"message":"Session ID unknown"}`)))
			wsc.Close()
			return
		}
		if session.IsClosed() {
			wsc.WriteTextMessage([]byte(fmt.Sprintf(`{"code":3,"message":"Session closed"}`)))
			wsc.Close()
			return
		}
		srv.handleWSUpgrade(wsc, session)
	}
}

// handleWSHandshake creates a new session directly over WebSocket.
func (srv *Server) handleWSHandshake(wsc *wsConn) {
	session := NewSession(srv.config.PingInterval, srv.config.PingTimeout)
	session.Transport = "websocket"

	srv.sessions.Store(session.ID, session)

	session.SetOnClose(func(sid string, reason string) {
		srv.RemoveSession(sid)
	})

	// Build and send OPEN packet
	upgrades := []string{} // no further upgrades available on WS
	openData := openPacketData{
		SID:          session.ID,
		Upgrades:     upgrades,
		PingInterval: int(srv.config.PingInterval / time.Millisecond),
		PingTimeout:  int(srv.config.PingTimeout / time.Millisecond),
		MaxPayload:   srv.config.MaxPayload,
	}

	jsonData, err := json.Marshal(openData)
	if err != nil {
		wsc.Close()
		return
	}

	if err := wsc.WritePacket(&Packet{Type: PacketOpen, Data: jsonData}); err != nil {
		wsc.Close()
		return
	}

	// Invoke the onConnect callback
	srv.mu.RLock()
	onConnect := srv.onConnect
	srv.mu.RUnlock()

	if onConnect != nil {
		onConnect(session)
	}

	// Attach WebSocket to session and start read/write loops
	session.BindWebSocket(wsc)
	go srv.wsWriteLoop(session, wsc)
	srv.wsReadLoop(session, wsc) // blocks until connection closes
}

// handleWSUpgrade handles the polling→websocket upgrade probe sequence.
// Expected flow:
//  1. Client sends PING "probe" → server responds PONG "probe"
//  2. Client sends UPGRADE → server switches transport
//
// During the probe phase, the session is marked as "upgrading" so that
// pending polling GET requests are released with a NOOP packet. If the
// probe fails or times out, the upgrade is cancelled and polling continues.
func (srv *Server) handleWSUpgrade(wsc *wsConn, session *Session) {
	// Step 1: Wait for probe ping
	wsc.conn.SetReadDeadline(time.Now().Add(srv.config.PingTimeout))
	_, msg, err := wsc.conn.ReadMessage()
	if err != nil {
		wsc.Close()
		return
	}

	p, err := DecodePacket(string(msg))
	if err != nil || p.Type != PacketPing || string(p.Data) != "probe" {
		wsc.Close()
		return
	}

	// Mark session as upgrading — this causes pending polling Drain() to
	// return NOOP, releasing the long-poll so the client can switch.
	session.StartUpgrade()

	// Respond with PONG "probe"
	if err := wsc.WritePacket(&Packet{Type: PacketPong, Data: []byte("probe")}); err != nil {
		session.CancelUpgrade()
		wsc.Close()
		return
	}

	// Step 2: Wait for UPGRADE packet
	wsc.conn.SetReadDeadline(time.Now().Add(srv.config.PingTimeout))
	_, msg, err = wsc.conn.ReadMessage()
	if err != nil {
		session.CancelUpgrade()
		wsc.Close()
		return
	}

	p, err = DecodePacket(string(msg))
	if err != nil || p.Type != PacketUpgrade {
		session.CancelUpgrade()
		wsc.Close()
		return
	}

	// Clear read deadline
	wsc.conn.SetReadDeadline(time.Time{})

	// Switch transport — also signals any remaining poll waiters
	session.UpgradeToWebSocket(wsc)

	// Start read/write loops
	go srv.wsWriteLoop(session, wsc)
	srv.wsReadLoop(session, wsc)
}

// wsReadLoop reads Engine.IO packets from the WebSocket and dispatches them.
func (srv *Server) wsReadLoop(session *Session, wsc *wsConn) {
	defer func() {
		session.Close("transport close")
		wsc.Close()
	}()

	for {
		_, msg, err := wsc.conn.ReadMessage()
		if err != nil {
			return
		}

		p, err := DecodePacket(string(msg))
		if err != nil {
			continue
		}

		session.HandlePacket(p)

		if p.Type == PacketClose {
			return
		}
	}
}

// wsWriteLoop sends buffered packets to the WebSocket client.
// It also handles periodic ping sending for WebSocket sessions.
func (srv *Server) wsWriteLoop(session *Session, wsc *wsConn) {
	pingTicker := time.NewTicker(srv.config.PingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-session.closedChan:
			// Send close packet before exiting
			wsc.WritePacket(&Packet{Type: PacketClose})
			return

		case <-pingTicker.C:
			if err := wsc.WritePacket(&Packet{Type: PacketPing}); err != nil {
				session.Close("ping write error")
				return
			}

		case <-session.wsSendChan:
			// Drain all buffered packets
			session.mu.Lock()
			packets := session.sendBuffer
			session.sendBuffer = make([]*Packet, 0)
			session.mu.Unlock()

			for _, p := range packets {
				if err := wsc.WritePacket(p); err != nil {
					session.Close("write error")
					return
				}
			}
		}
	}
}
