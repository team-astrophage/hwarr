package engineio

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// Session represents an Engine.IO client session.
type Session struct {
	ID           string
	Transport    string // "polling" or "websocket"
	RemoteAddr   string // client IP:port from the initial HTTP request
	PingInterval time.Duration
	PingTimeout  time.Duration

	mu         sync.Mutex
	sendBuffer []*Packet      // packets waiting to be sent via polling GET
	recvChan   chan struct{}  // signals when new packets are available (polling)
	pollReady  chan []*Packet // delivers packets to the waiting poll request
	closed     bool
	closedChan chan struct{}
	lastActive time.Time
	onMessage  func(data []byte)               // callback for incoming messages
	onClose    func(sid string, reason string) // callback on session close

	// WebSocket transport fields
	wsConn     *wsConn       // active WebSocket connection (nil for polling)
	wsSendChan chan struct{} // signals when packets are available for WS write loop

	// Upgrade state fields
	upgrading   bool          // true during polling→websocket probe phase
	upgradeChan chan struct{} // closed when upgrade completes, to release pending polls
}

// generateSID creates a URL-safe random session ID (20 bytes base64-encoded).
func generateSID() string {
	b := make([]byte, 15)
	_, _ = rand.Read(b)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
}

// NewSession creates a new Engine.IO session.
func NewSession(pingInterval, pingTimeout time.Duration) *Session {
	s := &Session{
		ID:           generateSID(),
		Transport:    "polling",
		PingInterval: pingInterval,
		PingTimeout:  pingTimeout,
		sendBuffer:   make([]*Packet, 0),
		recvChan:     make(chan struct{}, 1),
		pollReady:    make(chan []*Packet, 1),
		closedChan:   make(chan struct{}),
		lastActive:   time.Now(),
		upgradeChan:  make(chan struct{}),
	}
	return s
}

// SetOnMessage sets the callback invoked when the session receives a message packet.
func (s *Session) SetOnMessage(fn func(data []byte)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMessage = fn
}

// SetOnClose sets the callback invoked when the session is closed.
func (s *Session) SetOnClose(fn func(sid string, reason string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onClose = fn
}

// Send queues a message packet to be delivered to the client.
func (s *Session) Send(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.sendBuffer = append(s.sendBuffer, &Packet{
		Type: PacketMessage,
		Data: data,
	})
	s.signalSend()
}

// SendPacket queues a raw packet to be delivered to the client.
func (s *Session) SendPacket(p *Packet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.sendBuffer = append(s.sendBuffer, p)
	s.signalSend()
}

// signalSend notifies the appropriate transport that packets are available.
// Must be called with s.mu held.
func (s *Session) signalSend() {
	if s.wsConn != nil {
		// WebSocket transport
		select {
		case s.wsSendChan <- struct{}{}:
		default:
		}
	} else {
		// Polling transport
		select {
		case s.recvChan <- struct{}{}:
		default:
		}
	}
}

// BindWebSocket attaches a WebSocket connection to this session.
// Used for fresh WebSocket connections (not upgrades).
func (s *Session) BindWebSocket(wsc *wsConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wsConn = wsc
	s.wsSendChan = make(chan struct{}, 1)
	s.Transport = "websocket"
}

// StartUpgrade marks the session as upgrading. During this phase,
// pending polling GET requests are released with a NOOP packet so the
// client can complete the transport switch.
func (s *Session) StartUpgrade() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.upgrading {
		return
	}
	s.upgrading = true
	// Signal any pending polling Drain to wake up and return NOOP
	select {
	case s.recvChan <- struct{}{}:
	default:
	}
}

// IsUpgrading reports whether the session is in the upgrade probe phase.
func (s *Session) IsUpgrading() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.upgrading
}

// UpgradeToWebSocket switches the session from polling to WebSocket transport.
// Any buffered polling packets are migrated to the WebSocket send channel.
func (s *Session) UpgradeToWebSocket(wsc *wsConn) {
	s.mu.Lock()
	s.wsConn = wsc
	s.wsSendChan = make(chan struct{}, 1)
	s.Transport = "websocket"
	s.upgrading = false

	// Close upgradeChan to release any remaining polling waiters
	select {
	case <-s.upgradeChan:
		// already closed
	default:
		close(s.upgradeChan)
	}

	// Signal if there are buffered packets to flush via WS
	if len(s.sendBuffer) > 0 {
		select {
		case s.wsSendChan <- struct{}{}:
		default:
		}
	}
	s.mu.Unlock()

	// Signal polling recvChan one more time so any blocked Drain returns
	select {
	case s.recvChan <- struct{}{}:
	default:
	}
}

// CancelUpgrade aborts an in-progress upgrade (e.g. on probe timeout).
func (s *Session) CancelUpgrade() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upgrading = false
}

// Drain returns all buffered packets and clears the buffer.
// If no packets are available, it blocks until packets arrive or the timeout expires.
// During a transport upgrade, it returns a NOOP packet immediately so the
// client can release the polling connection and complete the switch.
func (s *Session) Drain(timeout time.Duration) []*Packet {
	// First check if there are already buffered packets
	s.mu.Lock()
	upgrading := s.upgrading
	if len(s.sendBuffer) > 0 {
		packets := s.sendBuffer
		s.sendBuffer = make([]*Packet, 0)
		s.lastActive = time.Now()
		s.mu.Unlock()
		// If upgrading, append a NOOP to tell the client to stop polling
		if upgrading {
			packets = append(packets, &Packet{Type: PacketNoop})
		}
		return packets
	}
	// If upgrade is in progress, return NOOP immediately to release the poll
	if upgrading {
		s.mu.Unlock()
		return []*Packet{{Type: PacketNoop}}
	}
	s.mu.Unlock()

	// Wait for packets or timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-s.recvChan:
		s.mu.Lock()
		upgrading = s.upgrading
		packets := s.sendBuffer
		s.sendBuffer = make([]*Packet, 0)
		s.lastActive = time.Now()
		s.mu.Unlock()
		if upgrading && len(packets) == 0 {
			return []*Packet{{Type: PacketNoop}}
		}
		if upgrading {
			packets = append(packets, &Packet{Type: PacketNoop})
		}
		return packets
	case <-s.closedChan:
		return nil
	case <-timer.C:
		return nil
	}
}

// HandlePacket processes an incoming Engine.IO packet.
func (s *Session) HandlePacket(p *Packet) {
	s.mu.Lock()
	s.lastActive = time.Now()
	onMessage := s.onMessage
	s.mu.Unlock()

	switch p.Type {
	case PacketPing:
		// Respond with pong carrying the same data
		s.SendPacket(&Packet{Type: PacketPong, Data: p.Data})
	case PacketMessage:
		if onMessage != nil {
			onMessage(p.Data)
		}
	case PacketClose:
		s.Close("client close")
	}
}

// Close terminates the session.
func (s *Session) Close(reason string) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	onClose := s.onClose
	close(s.closedChan)
	s.mu.Unlock()

	if onClose != nil {
		onClose(s.ID, reason)
	}
}

// IsClosed returns true if the session has been closed.
func (s *Session) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// LastActive returns the time of last activity.
func (s *Session) LastActive() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastActive
}
