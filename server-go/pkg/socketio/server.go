package socketio

import (
	"fmt"
	"log"
	"sync"
)

// SendFunc is a callback that sends an encoded Socket.IO packet (wire format
// string) to the client identified by sid. This is set by the integration layer
// that bridges Socket.IO to Engine.IO.
// Returns an error if the send fails (e.g., session not found or closed).
type SendFunc func(sid string, data string) error

// Server is the top-level Socket.IO server that manages namespaces and
// dispatches incoming packets to the appropriate namespace handler.
//
// Room memberships are scoped per-namespace (accessed via ns.Rooms),
// matching python-socketio behavior. The deprecated Server.Rooms field
// is kept as an alias to the default namespace's RoomManager for backward
// compatibility.
//
// Thread-safe: all methods can be called concurrently.
type Server struct {
	mu         sync.RWMutex
	namespaces map[string]*Namespace

	// Rooms is an alias to the default "/" namespace's RoomManager.
	// Prefer using s.Of(namespace).Rooms for namespace-scoped operations.
	// Kept for backward compatibility.
	Rooms *RoomManager

	// SendTo is the callback used to deliver packets to individual sockets.
	// Must be set by the integration layer before calling broadcast methods.
	SendTo SendFunc

	// Logger for debug/info messages. Defaults to log.Default().
	Logger *log.Logger
}

// NewServer creates a new Socket.IO server with the default "/" namespace.
func NewServer() *Server {
	defaultNS := NewNamespace("/")
	s := &Server{
		namespaces: make(map[string]*Namespace),
		Rooms:      defaultNS.Rooms, // alias to default namespace's RoomManager
		Logger:     log.Default(),
	}
	// Always register the default namespace
	s.namespaces["/"] = defaultNS
	return s
}

// Of returns the namespace for the given path, creating it if it doesn't exist.
// The path must start with "/". An empty string defaults to "/".
//
// This mirrors python-socketio's server.namespace() / @sio.on(event, namespace=...).
func (s *Server) Of(path string) *Namespace {
	if path == "" {
		path = "/"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ns, ok := s.namespaces[path]
	if !ok {
		ns = NewNamespace(path)
		s.namespaces[path] = ns
	}
	return ns
}

// GetNamespace returns the namespace for the given path, or nil if not found.
func (s *Server) GetNamespace(path string) *Namespace {
	if path == "" {
		path = "/"
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.namespaces[path]
}

// Namespaces returns a list of all registered namespace paths.
func (s *Server) Namespaces() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	paths := make([]string, 0, len(s.namespaces))
	for p := range s.namespaces {
		paths = append(paths, p)
	}
	return paths
}

// Use is a convenience method that appends a middleware to the default "/" namespace.
// For namespace-specific middleware, use s.Of(namespace).Use(mw).
func (s *Server) Use(mw MiddlewareFunc) {
	s.Of("/").Use(mw)
}

// On is a convenience method that registers an event handler on the default "/" namespace.
func (s *Server) On(event string, handler EventHandler) {
	s.Of("/").On(event, handler)
}

// OnConnect is a convenience method that registers a connect handler on the default "/" namespace.
func (s *Server) OnConnect(handler ConnectHandler) {
	s.Of("/").OnConnect(handler)
}

// OnDisconnect is a convenience method that registers a disconnect handler on the default "/" namespace.
func (s *Server) OnDisconnect(handler DisconnectHandler) {
	s.Of("/").OnDisconnect(handler)
}

// DispatchResult holds the outcome of dispatching a Socket.IO packet.
// For CONNECT packets, ResponsePacket contains the CONNECT ACK or CONNECT_ERROR.
type DispatchResult struct {
	AckData        []interface{} // response data for EVENT ACK
	ResponsePacket *Packet       // CONNECT / CONNECT_ERROR response packet
}

// DispatchPacket routes a decoded Socket.IO packet to the appropriate namespace
// handler based on the packet's namespace field.
//
// For CONNECT packets: calls HandleConnect. On success, returns a CONNECT response
// packet with the socket's SID. On failure, returns a CONNECT_ERROR packet.
// For EVENT packets: extracts event name and args, then calls HandleEvent.
// For DISCONNECT packets: calls HandleDisconnect.
// For ACK packets: should be handled by the caller (ack registry).
//
// Returns:
//   - ackData: response data for ACK (nil if no ack needed)
//   - err: dispatch error (namespace not found, no handler, etc.)
func (s *Server) DispatchPacket(sid string, pkt *Packet) (ackData []interface{}, err error) {
	result, err := s.DispatchPacketFull(sid, pkt)
	if err != nil {
		return nil, err
	}
	return result.AckData, nil
}

// DispatchPacketFull is the full-featured dispatch method that returns a
// DispatchResult including any response packets (e.g., CONNECT ACK).
//
// For CONNECT packets to the default "/" namespace (or any registered namespace):
//   - On success: result.ResponsePacket = CONNECT with {"sid": sid}
//   - On failure: result.ResponsePacket = CONNECT_ERROR with {"message": ...}
//
// Namespace fallback: if the packet's namespace is empty, it defaults to "/".
func (s *Server) DispatchPacketFull(sid string, pkt *Packet) (*DispatchResult, error) {
	// Fallback to default namespace
	if pkt.Namespace == "" {
		pkt.Namespace = "/"
	}

	result := &DispatchResult{}

	switch pkt.Type {
	case PacketConnect:
		ns := s.GetNamespace(pkt.Namespace)
		if ns == nil {
			// Namespace not found — send CONNECT_ERROR
			errPkt, pktErr := NewConnectErrorPacket(pkt.Namespace, "Invalid namespace")
			if pktErr != nil {
				return nil, fmt.Errorf("socketio: failed to build connect error packet: %w", pktErr)
			}
			result.ResponsePacket = errPkt
			return result, fmt.Errorf("socketio: namespace %q not found", pkt.Namespace)
		}

		if err := ns.HandleConnect(sid, pkt.Data); err != nil {
			// Connection rejected by handler — send CONNECT_ERROR
			errPkt, pktErr := NewConnectErrorPacket(pkt.Namespace, err.Error())
			if pktErr != nil {
				return nil, fmt.Errorf("socketio: failed to build connect error packet: %w", pktErr)
			}
			result.ResponsePacket = errPkt
			return result, err
		}

		// Connection accepted — send CONNECT ACK with sid
		ackPkt, pktErr := NewConnectPacket(pkt.Namespace, sid)
		if pktErr != nil {
			return nil, fmt.Errorf("socketio: failed to build connect ack packet: %w", pktErr)
		}
		result.ResponsePacket = ackPkt
		return result, nil

	case PacketDisconnect:
		ns := s.GetNamespace(pkt.Namespace)
		if ns == nil {
			return result, nil
		}
		ns.HandleDisconnect(sid, "client namespace disconnect")
		return result, nil

	case PacketEvent, PacketBinaryEvent:
		ns := s.GetNamespace(pkt.Namespace)
		if ns == nil {
			return nil, fmt.Errorf("socketio: namespace %q not found", pkt.Namespace)
		}
		eventName, nameErr := pkt.EventName()
		if nameErr != nil {
			return nil, fmt.Errorf("socketio: failed to extract event name: %w", nameErr)
		}
		args, argsErr := pkt.EventArgs()
		if argsErr != nil {
			return nil, fmt.Errorf("socketio: failed to extract event args: %w", argsErr)
		}
		ackData, err := ns.HandleEvent(sid, eventName, args)
		result.AckData = ackData
		return result, err

	case PacketAck, PacketBinaryAck:
		// ACK packets are handled by the ack callback registry, not namespace dispatch.
		return result, nil

	default:
		return nil, fmt.Errorf("socketio: unhandled packet type %d", pkt.Type)
	}
}

// DisconnectAll disconnects the given socket from all namespaces it is
// connected to. This should be called when the underlying Engine.IO transport
// closes to ensure proper cleanup and disconnect handler invocation.
func (s *Server) DisconnectAll(sid string, reason string) {
	s.mu.RLock()
	namespaces := make([]*Namespace, 0, len(s.namespaces))
	for _, ns := range s.namespaces {
		namespaces = append(namespaces, ns)
	}
	s.mu.RUnlock()

	for _, ns := range namespaces {
		if ns.HasSocket(sid) {
			ns.HandleDisconnect(sid, reason)
		}
	}
}

// ConnectedNamespaces returns the list of namespace paths that the given
// socket is currently connected to.
func (s *Server) ConnectedNamespaces(sid string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var paths []string
	for path, ns := range s.namespaces {
		if ns.HasSocket(sid) {
			paths = append(paths, path)
		}
	}
	return paths
}

// DispatchRaw is a convenience method that decodes a raw Socket.IO packet string
// and dispatches it. Returns the ack response data and the decoded packet.
func (s *Server) DispatchRaw(sid string, data string) (ackData []interface{}, pkt *Packet, err error) {
	pkt, err = Decode(data)
	if err != nil {
		return nil, nil, fmt.Errorf("socketio: failed to decode packet: %w", err)
	}
	ackData, err = s.DispatchPacket(sid, pkt)
	return ackData, pkt, err
}

// DispatchRawFull decodes a raw Socket.IO packet and dispatches it, returning
// the full DispatchResult including any response packets.
func (s *Server) DispatchRawFull(sid string, data string) (*DispatchResult, *Packet, error) {
	pkt, err := Decode(data)
	if err != nil {
		return nil, nil, fmt.Errorf("socketio: failed to decode packet: %w", err)
	}
	result, err := s.DispatchPacketFull(sid, pkt)
	return result, pkt, err
}

// BuildAckPacket is a helper that creates an ACK response packet if the original
// packet requires one (has a non-negative ID) and ackData is not nil.
func BuildAckPacket(pkt *Packet, ackData []interface{}) (*Packet, error) {
	if pkt.ID < 0 || ackData == nil {
		return nil, nil
	}
	return NewAckPacket(pkt.Namespace, pkt.ID, ackData...)
}

// BroadcastToNamespace sends an event to ALL sockets connected to the given
// namespace. This matches python-socketio's `sio.emit(event, data)` without
// a room parameter.
//
// Returns the number of sockets the event was successfully sent to.
func (s *Server) BroadcastToNamespace(namespace, event string, args ...interface{}) (int, error) {
	if s.SendTo == nil {
		return 0, fmt.Errorf("socketio: SendTo not configured")
	}

	ns := s.GetNamespace(namespace)
	if ns == nil {
		return 0, fmt.Errorf("socketio: namespace %q not found", namespace)
	}

	pkt, err := NewEventPacket(namespace, event, -1, args...)
	if err != nil {
		return 0, fmt.Errorf("socketio: failed to build event packet: %w", err)
	}

	encoded, err := Encode(pkt)
	if err != nil {
		return 0, fmt.Errorf("socketio: failed to encode event packet: %w", err)
	}

	sids := ns.Sockets()
	var (
		sent    int
		lastErr error
	)
	for _, sid := range sids {
		if err := s.SendTo(sid, encoded); err != nil {
			lastErr = err
			s.Logger.Printf("socketio: failed to send to %s: %v", sid, err)
			continue
		}
		sent++
	}
	return sent, lastErr
}

// BroadcastToRoom sends an event to all sockets in the given room.
// The namespace parameter determines the Socket.IO namespace for the packet
// and the room lookup (rooms are namespace-scoped).
//
// Returns the number of sockets the event was successfully sent to, and any
// errors from individual sends (only the last error is returned for simplicity).
func (s *Server) BroadcastToRoom(namespace, room, event string, args ...interface{}) (int, error) {
	return s.broadcastToRoom(namespace, room, "", event, args...)
}

// BroadcastToRoomExcept sends an event to all sockets in the given room except
// the specified sender. This is the typical "broadcast" pattern where the sender
// should not receive their own event.
//
// Returns the number of sockets the event was successfully sent to.
func (s *Server) BroadcastToRoomExcept(namespace, room, excludeSID, event string, args ...interface{}) (int, error) {
	return s.broadcastToRoom(namespace, room, excludeSID, event, args...)
}

// SendToSocket sends an event to a specific socket in a namespace.
// This matches python-socketio's `sio.emit(event, data, to=sid)`.
func (s *Server) SendToSocket(namespace, sid, event string, args ...interface{}) error {
	if s.SendTo == nil {
		return fmt.Errorf("socketio: SendTo not configured")
	}

	pkt, err := NewEventPacket(namespace, event, -1, args...)
	if err != nil {
		return fmt.Errorf("socketio: failed to build event packet: %w", err)
	}

	encoded, err := Encode(pkt)
	if err != nil {
		return fmt.Errorf("socketio: failed to encode event packet: %w", err)
	}

	return s.SendTo(sid, encoded)
}

// EnterRoom adds a socket to a room in the given namespace.
// This matches python-socketio's `sio.enter_room(sid, room)`.
// If the namespace doesn't exist, returns an error.
func (s *Server) EnterRoom(namespace, sid, room string) error {
	ns := s.GetNamespace(namespace)
	if ns == nil {
		return fmt.Errorf("socketio: namespace %q not found", namespace)
	}
	ns.Rooms.Join(sid, room)
	return nil
}

// LeaveRoom removes a socket from a room in the given namespace.
// This matches python-socketio's `sio.leave_room(sid, room)`.
// If the namespace doesn't exist, this is a no-op (returns nil).
func (s *Server) LeaveRoom(namespace, sid, room string) error {
	ns := s.GetNamespace(namespace)
	if ns == nil {
		return nil
	}
	ns.Rooms.Leave(sid, room)
	return nil
}

// LeaveAllRooms removes a socket from all rooms in the given namespace.
// If the namespace doesn't exist, this is a no-op.
func (s *Server) LeaveAllRooms(namespace, sid string) {
	ns := s.GetNamespace(namespace)
	if ns == nil {
		return
	}
	ns.Rooms.LeaveAll(sid)
}

// ReplaceRooms replaces all room memberships for a socket in the given
// namespace with a new set of rooms. This matches python-socketio's
// ConnectionManager.join_rooms() behavior: leave old rooms, join new ones.
//
// This is the typical pattern for viewport subscriptions where a client
// watches only one set of rooms at a time.
func (s *Server) ReplaceRooms(namespace, sid string, newRooms []string) error {
	ns := s.GetNamespace(namespace)
	if ns == nil {
		return fmt.Errorf("socketio: namespace %q not found", namespace)
	}

	// Leave all current rooms
	ns.Rooms.LeaveAll(sid)

	// Join new rooms
	for _, room := range newRooms {
		ns.Rooms.Join(sid, room)
	}
	return nil
}

// SocketRooms returns all rooms a socket belongs to in the given namespace.
func (s *Server) SocketRooms(namespace, sid string) []string {
	ns := s.GetNamespace(namespace)
	if ns == nil {
		return nil
	}
	return ns.Rooms.Rooms(sid)
}

// RoomMembers returns all socket IDs in a room within the given namespace.
func (s *Server) RoomMembers(namespace, room string) []string {
	ns := s.GetNamespace(namespace)
	if ns == nil {
		return nil
	}
	return ns.Rooms.Members(room)
}

// broadcastToRoom is the internal implementation for room broadcasts.
// Uses the namespace-scoped RoomManager for room membership lookup.
// If excludeSID is non-empty, that socket is skipped.
func (s *Server) broadcastToRoom(namespace, room, excludeSID, event string, args ...interface{}) (int, error) {
	if s.SendTo == nil {
		return 0, fmt.Errorf("socketio: SendTo not configured")
	}

	// Look up the namespace to get its scoped RoomManager
	ns := s.GetNamespace(namespace)
	if ns == nil {
		return 0, fmt.Errorf("socketio: namespace %q not found", namespace)
	}

	// Build the event packet once
	pkt, err := NewEventPacket(namespace, event, -1, args...)
	if err != nil {
		return 0, fmt.Errorf("socketio: failed to build event packet: %w", err)
	}

	encoded, err := Encode(pkt)
	if err != nil {
		return 0, fmt.Errorf("socketio: failed to encode event packet: %w", err)
	}

	// Get all members in the namespace-scoped room
	members := ns.Rooms.Members(room)
	if len(members) == 0 {
		return 0, nil
	}

	var (
		sent    int
		lastErr error
	)

	for _, sid := range members {
		if sid == excludeSID {
			continue
		}
		if err := s.SendTo(sid, encoded); err != nil {
			lastErr = err
			s.Logger.Printf("socketio: failed to send to %s: %v", sid, err)
			continue
		}
		sent++
	}

	return sent, lastErr
}
