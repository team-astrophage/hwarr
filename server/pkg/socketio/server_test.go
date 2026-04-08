package socketio

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
)

func TestNewServer_HasDefaultNamespace(t *testing.T) {
	s := NewServer()
	ns := s.GetNamespace("/")
	if ns == nil {
		t.Fatal("default namespace '/' should exist")
	}
	if ns.Path() != "/" {
		t.Errorf("expected path '/', got %q", ns.Path())
	}
}

func TestServer_Of_CreatesNamespace(t *testing.T) {
	s := NewServer()
	ns := s.Of("/chat")
	if ns == nil {
		t.Fatal("Of should return a namespace")
	}
	if ns.Path() != "/chat" {
		t.Errorf("expected path '/chat', got %q", ns.Path())
	}

	// Same namespace returned on second call
	ns2 := s.Of("/chat")
	if ns != ns2 {
		t.Error("Of should return the same namespace instance")
	}
}

func TestServer_Of_EmptyDefaultsToRoot(t *testing.T) {
	s := NewServer()
	ns := s.Of("")
	if ns.Path() != "/" {
		t.Errorf("expected path '/', got %q", ns.Path())
	}
}

func TestServer_GetNamespace_NotFound(t *testing.T) {
	s := NewServer()
	ns := s.GetNamespace("/nonexistent")
	if ns != nil {
		t.Error("expected nil for nonexistent namespace")
	}
}

func TestServer_Namespaces(t *testing.T) {
	s := NewServer()
	s.Of("/chat")
	s.Of("/admin")
	paths := s.Namespaces()
	sort.Strings(paths)
	expected := []string{"/", "/admin", "/chat"}
	if len(paths) != len(expected) {
		t.Fatalf("expected %d namespaces, got %d: %v", len(expected), len(paths), paths)
	}
	for i, p := range paths {
		if p != expected[i] {
			t.Errorf("expected %q at index %d, got %q", expected[i], i, p)
		}
	}
}

func TestServer_OnConvenience(t *testing.T) {
	s := NewServer()
	called := false
	s.On("test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		called = true
		return nil, nil
	})
	if !s.Of("/").HasEvent("test") {
		t.Error("event should be registered on default namespace")
	}

	// Trigger via dispatch
	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["test","hello"]`),
	}
	_, err := s.DispatchPacket("sid-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called via DispatchPacket")
	}
}

func TestServer_DispatchPacket_Connect(t *testing.T) {
	s := NewServer()
	connected := false
	s.OnConnect(func(sid string, auth json.RawMessage) error {
		connected = true
		return nil
	})

	pkt := &Packet{
		Type:      PacketConnect,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`{"user_id":"abc"}`),
	}
	_, err := s.DispatchPacket("sid-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if !connected {
		t.Fatal("connect handler was not called")
	}
}

func TestServer_DispatchPacket_ConnectReject(t *testing.T) {
	s := NewServer()
	s.OnConnect(func(sid string, auth json.RawMessage) error {
		return errors.New("unauthorized")
	})

	pkt := &Packet{
		Type:      PacketConnect,
		Namespace: "/",
		ID:        -1,
	}
	_, err := s.DispatchPacket("sid-1", pkt)
	if err == nil {
		t.Fatal("expected connection rejection error")
	}
}

func TestServer_DispatchPacket_Disconnect(t *testing.T) {
	s := NewServer()
	disconnected := false
	s.OnDisconnect(func(sid string, reason string) {
		disconnected = true
	})

	pkt := &Packet{
		Type:      PacketDisconnect,
		Namespace: "/",
		ID:        -1,
	}
	_, err := s.DispatchPacket("sid-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if !disconnected {
		t.Fatal("disconnect handler was not called")
	}
}

func TestServer_DispatchPacket_Event_WithAck(t *testing.T) {
	s := NewServer()
	s.On("greet", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{"hi back"}, nil
	})

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        42,
		Data:      json.RawMessage(`["greet","hello"]`),
	}
	ackData, err := s.DispatchPacket("sid-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if len(ackData) != 1 || ackData[0] != "hi back" {
		t.Errorf("unexpected ack data: %v", ackData)
	}
}

func TestServer_DispatchPacket_NamespaceNotFound(t *testing.T) {
	s := NewServer()
	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/nonexistent",
		ID:        -1,
		Data:      json.RawMessage(`["test"]`),
	}
	_, err := s.DispatchPacket("sid-1", pkt)
	if err == nil {
		t.Fatal("expected error for nonexistent namespace")
	}
}

func TestServer_DispatchPacket_MultipleNamespaces(t *testing.T) {
	s := NewServer()
	rootCalled := false
	chatCalled := false

	s.On("msg", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		rootCalled = true
		return nil, nil
	})
	s.Of("/chat").On("msg", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		chatCalled = true
		return nil, nil
	})

	// Dispatch to root
	pkt1 := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["msg","hi"]`),
	}
	s.DispatchPacket("sid-1", pkt1)

	// Dispatch to /chat
	pkt2 := &Packet{
		Type:      PacketEvent,
		Namespace: "/chat",
		ID:        -1,
		Data:      json.RawMessage(`["msg","hi"]`),
	}
	s.DispatchPacket("sid-1", pkt2)

	if !rootCalled {
		t.Error("root namespace handler was not called")
	}
	if !chatCalled {
		t.Error("/chat namespace handler was not called")
	}
}

func TestServer_DispatchRaw(t *testing.T) {
	s := NewServer()
	var receivedEvent string
	s.On("ping", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		receivedEvent = "ping"
		return []interface{}{"pong"}, nil
	})

	// Raw packet: type=2 (EVENT), namespace=/ (default), data=["ping","data"]
	ackData, pkt, err := s.DispatchRaw("sid-1", `2["ping","data"]`)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if receivedEvent != "ping" {
		t.Errorf("expected event 'ping', got %q", receivedEvent)
	}
	if pkt.Type != PacketEvent {
		t.Errorf("expected packet type EVENT, got %d", pkt.Type)
	}
	if len(ackData) != 1 || ackData[0] != "pong" {
		t.Errorf("unexpected ack data: %v", ackData)
	}
}

func TestServer_DispatchRaw_WithNamespace(t *testing.T) {
	s := NewServer()
	s.Of("/chat").On("say", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, nil
	})

	// Raw packet: type=2, namespace=/chat, data=["say","hello"]
	_, pkt, err := s.DispatchRaw("sid-1", `2/chat,["say","hello"]`)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if pkt.Namespace != "/chat" {
		t.Errorf("expected namespace '/chat', got %q", pkt.Namespace)
	}
}

func TestServer_DispatchRaw_InvalidPacket(t *testing.T) {
	s := NewServer()
	_, _, err := s.DispatchRaw("sid-1", "")
	if err == nil {
		t.Fatal("expected error for empty packet")
	}
}

func TestBuildAckPacket(t *testing.T) {
	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/chat",
		ID:        7,
		Data:      json.RawMessage(`["hello"]`),
	}

	ackPkt, err := BuildAckPacket(pkt, []interface{}{"ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ackPkt == nil {
		t.Fatal("expected ack packet")
	}
	if ackPkt.Type != PacketAck {
		t.Errorf("expected ACK type, got %d", ackPkt.Type)
	}
	if ackPkt.Namespace != "/chat" {
		t.Errorf("expected namespace '/chat', got %q", ackPkt.Namespace)
	}
	if ackPkt.ID != 7 {
		t.Errorf("expected ack ID 7, got %d", ackPkt.ID)
	}

	// Encode and verify
	encoded, err := Encode(ackPkt)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if encoded != `3/chat,7["ok"]` {
		t.Errorf("unexpected encoded ack: %q", encoded)
	}
}

func TestBuildAckPacket_NoAckNeeded(t *testing.T) {
	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
	}
	ackPkt, err := BuildAckPacket(pkt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ackPkt != nil {
		t.Error("expected nil ack packet when ID < 0")
	}
}

func TestServer_DispatchPacket_AckPassthrough(t *testing.T) {
	s := NewServer()
	pkt := &Packet{
		Type:      PacketAck,
		Namespace: "/",
		ID:        5,
		Data:      json.RawMessage(`["result"]`),
	}
	ackData, err := s.DispatchPacket("sid-1", pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ackData != nil {
		t.Error("ACK packets should return nil ackData")
	}
}

// --- Default namespace auto-registration and connection handling tests ---

func TestServer_DispatchPacketFull_ConnectDefaultNamespace(t *testing.T) {
	s := NewServer()

	// Client sends CONNECT to default namespace (no namespace specified → "/")
	pkt := &Packet{
		Type:      PacketConnect,
		Namespace: "/",
		ID:        -1,
	}
	result, err := s.DispatchPacketFull("sid-1", pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResponsePacket == nil {
		t.Fatal("expected CONNECT response packet")
	}
	if result.ResponsePacket.Type != PacketConnect {
		t.Errorf("expected CONNECT type, got %d", result.ResponsePacket.Type)
	}
	if result.ResponsePacket.Namespace != "/" {
		t.Errorf("expected namespace '/', got %q", result.ResponsePacket.Namespace)
	}

	// Verify the response contains {"sid": "sid-1"}
	var data map[string]string
	if err := json.Unmarshal(result.ResponsePacket.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal connect response: %v", err)
	}
	if data["sid"] != "sid-1" {
		t.Errorf("expected sid 'sid-1' in response, got %q", data["sid"])
	}

	// Verify socket is tracked
	ns := s.GetNamespace("/")
	if !ns.HasSocket("sid-1") {
		t.Error("socket should be tracked in default namespace after CONNECT")
	}
}

func TestServer_DispatchPacketFull_ConnectEmptyNamespaceFallback(t *testing.T) {
	s := NewServer()

	// Empty namespace should fallback to "/"
	pkt := &Packet{
		Type:      PacketConnect,
		Namespace: "",
		ID:        -1,
	}
	result, err := s.DispatchPacketFull("sid-1", pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResponsePacket == nil {
		t.Fatal("expected CONNECT response packet")
	}
	if result.ResponsePacket.Namespace != "/" {
		t.Errorf("expected namespace '/' after fallback, got %q", result.ResponsePacket.Namespace)
	}

	// Verify socket is connected to "/"
	ns := s.GetNamespace("/")
	if !ns.HasSocket("sid-1") {
		t.Error("socket should be in default namespace after empty-namespace connect")
	}
}

func TestServer_DispatchPacketFull_ConnectWithAuth(t *testing.T) {
	s := NewServer()
	var receivedAuth json.RawMessage
	s.OnConnect(func(sid string, auth json.RawMessage) error {
		receivedAuth = auth
		return nil
	})

	pkt := &Packet{
		Type:      PacketConnect,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`{"token":"abc123"}`),
	}
	result, err := s.DispatchPacketFull("sid-1", pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResponsePacket == nil || result.ResponsePacket.Type != PacketConnect {
		t.Fatal("expected CONNECT response")
	}
	if string(receivedAuth) != `{"token":"abc123"}` {
		t.Errorf("expected auth payload, got %q", string(receivedAuth))
	}
}

func TestServer_DispatchPacketFull_ConnectRejectReturnsError(t *testing.T) {
	s := NewServer()
	s.OnConnect(func(sid string, auth json.RawMessage) error {
		return errors.New("unauthorized")
	})

	pkt := &Packet{
		Type:      PacketConnect,
		Namespace: "/",
		ID:        -1,
	}
	result, err := s.DispatchPacketFull("sid-1", pkt)
	if err == nil {
		t.Fatal("expected error for rejected connection")
	}
	if result == nil || result.ResponsePacket == nil {
		t.Fatal("expected CONNECT_ERROR response packet even on rejection")
	}
	if result.ResponsePacket.Type != PacketConnectError {
		t.Errorf("expected CONNECT_ERROR type, got %d", result.ResponsePacket.Type)
	}

	// Verify error message is in the packet
	var data map[string]string
	json.Unmarshal(result.ResponsePacket.Data, &data)
	if data["message"] != "unauthorized" {
		t.Errorf("expected error message 'unauthorized', got %q", data["message"])
	}

	// Socket should NOT be tracked
	ns := s.GetNamespace("/")
	if ns.HasSocket("sid-1") {
		t.Error("socket should NOT be tracked after rejected CONNECT")
	}
}

func TestServer_DispatchPacketFull_ConnectUnknownNamespace(t *testing.T) {
	s := NewServer()

	pkt := &Packet{
		Type:      PacketConnect,
		Namespace: "/nonexistent",
		ID:        -1,
	}
	result, err := s.DispatchPacketFull("sid-1", pkt)
	if err == nil {
		t.Fatal("expected error for unknown namespace")
	}
	if result == nil || result.ResponsePacket == nil {
		t.Fatal("expected CONNECT_ERROR response")
	}
	if result.ResponsePacket.Type != PacketConnectError {
		t.Errorf("expected CONNECT_ERROR, got %d", result.ResponsePacket.Type)
	}
}

func TestServer_DispatchPacketFull_ConnectMultipleNamespaces(t *testing.T) {
	s := NewServer()
	s.Of("/chat")

	// Connect to default
	pkt1 := &Packet{Type: PacketConnect, Namespace: "/", ID: -1}
	_, err := s.DispatchPacketFull("sid-1", pkt1)
	if err != nil {
		t.Fatalf("connect to / failed: %v", err)
	}

	// Connect to /chat
	pkt2 := &Packet{Type: PacketConnect, Namespace: "/chat", ID: -1}
	_, err = s.DispatchPacketFull("sid-1", pkt2)
	if err != nil {
		t.Fatalf("connect to /chat failed: %v", err)
	}

	// Verify socket is in both namespaces
	paths := s.ConnectedNamespaces("sid-1")
	if len(paths) != 2 {
		t.Fatalf("expected 2 connected namespaces, got %d: %v", len(paths), paths)
	}
}

func TestServer_DisconnectAll(t *testing.T) {
	s := NewServer()
	s.Of("/chat")

	disconnectReasons := make(map[string]string) // namespace → reason

	s.Of("/").OnDisconnect(func(sid string, reason string) {
		disconnectReasons["/"] = reason
	})
	s.Of("/chat").OnDisconnect(func(sid string, reason string) {
		disconnectReasons["/chat"] = reason
	})

	// Connect to both namespaces
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/", ID: -1})
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/chat", ID: -1})

	// Verify connected
	if !s.GetNamespace("/").HasSocket("sid-1") {
		t.Fatal("should be connected to /")
	}
	if !s.GetNamespace("/chat").HasSocket("sid-1") {
		t.Fatal("should be connected to /chat")
	}

	// Disconnect all (simulates transport close)
	s.DisconnectAll("sid-1", "transport close")

	// Verify disconnected from both
	if s.GetNamespace("/").HasSocket("sid-1") {
		t.Error("should be disconnected from /")
	}
	if s.GetNamespace("/chat").HasSocket("sid-1") {
		t.Error("should be disconnected from /chat")
	}

	// Verify disconnect handlers were called
	if disconnectReasons["/"] != "transport close" {
		t.Errorf("expected disconnect reason 'transport close' for /, got %q", disconnectReasons["/"])
	}
	if disconnectReasons["/chat"] != "transport close" {
		t.Errorf("expected disconnect reason 'transport close' for /chat, got %q", disconnectReasons["/chat"])
	}
}

func TestServer_DisconnectAll_SkipsUnconnected(t *testing.T) {
	s := NewServer()
	s.Of("/chat")

	disconnectCalled := false
	s.Of("/chat").OnDisconnect(func(sid string, reason string) {
		disconnectCalled = true
	})

	// Only connect to /
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/", ID: -1})

	s.DisconnectAll("sid-1", "transport close")

	// /chat disconnect handler should NOT be called
	if disconnectCalled {
		t.Error("/chat disconnect handler should not be called for unconnected socket")
	}
}

func TestServer_ConnectedNamespaces_Empty(t *testing.T) {
	s := NewServer()
	paths := s.ConnectedNamespaces("unknown-sid")
	if len(paths) != 0 {
		t.Errorf("expected empty paths, got %v", paths)
	}
}

func TestNamespace_SocketTracking(t *testing.T) {
	ns := NewNamespace("/")

	// HandleConnect adds to socket set
	ns.HandleConnect("sid-1", nil)
	ns.HandleConnect("sid-2", nil)

	if !ns.HasSocket("sid-1") {
		t.Error("sid-1 should be tracked")
	}
	if !ns.HasSocket("sid-2") {
		t.Error("sid-2 should be tracked")
	}
	if ns.SocketCount() != 2 {
		t.Errorf("expected 2 sockets, got %d", ns.SocketCount())
	}

	// HandleDisconnect removes from socket set
	ns.HandleDisconnect("sid-1", "test")
	if ns.HasSocket("sid-1") {
		t.Error("sid-1 should no longer be tracked")
	}
	if ns.SocketCount() != 1 {
		t.Errorf("expected 1 socket, got %d", ns.SocketCount())
	}

	// Sockets returns a copy
	sids := ns.Sockets()
	if len(sids) != 1 || sids[0] != "sid-2" {
		t.Errorf("unexpected sockets: %v", sids)
	}
}

func TestNamespace_ConnectRejectDoesNotTrack(t *testing.T) {
	ns := NewNamespace("/")
	ns.OnConnect(func(sid string, auth json.RawMessage) error {
		return errors.New("rejected")
	})

	err := ns.HandleConnect("sid-1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if ns.HasSocket("sid-1") {
		t.Error("rejected socket should not be tracked")
	}
}

func TestServer_DispatchRawFull_ConnectFlow(t *testing.T) {
	s := NewServer()

	// Client sends raw "0" (CONNECT to default namespace)
	result, pkt, err := s.DispatchRawFull("sid-1", "0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkt.Type != PacketConnect {
		t.Errorf("expected CONNECT packet, got %d", pkt.Type)
	}
	if result.ResponsePacket == nil {
		t.Fatal("expected CONNECT response packet")
	}

	// Encode the response and verify it's valid
	encoded, err := Encode(result.ResponsePacket)
	if err != nil {
		t.Fatalf("failed to encode response: %v", err)
	}
	// Should be: 0{"sid":"sid-1"}
	if encoded != `0{"sid":"sid-1"}` {
		t.Errorf("unexpected encoded response: %q", encoded)
	}

	// Verify socket is tracked
	if !s.GetNamespace("/").HasSocket("sid-1") {
		t.Error("socket should be tracked after raw connect")
	}
}

func TestServer_FullConnectionLifecycle(t *testing.T) {
	s := NewServer()

	var connectSID, disconnectSID, disconnectReason string
	eventReceived := false

	s.OnConnect(func(sid string, auth json.RawMessage) error {
		connectSID = sid
		return nil
	})
	s.On("test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		eventReceived = true
		return []interface{}{"ok"}, nil
	})
	s.OnDisconnect(func(sid string, reason string) {
		disconnectSID = sid
		disconnectReason = reason
	})

	// 1. CONNECT
	result, _, err := s.DispatchRawFull("sid-1", "0")
	if err != nil {
		t.Fatalf("CONNECT failed: %v", err)
	}
	if connectSID != "sid-1" {
		t.Errorf("connect handler not called with correct SID")
	}
	if result.ResponsePacket.Type != PacketConnect {
		t.Error("expected CONNECT response")
	}

	// 2. EVENT
	ackData, _, err := s.DispatchRaw("sid-1", `20["test","data"]`)
	if err != nil {
		t.Fatalf("EVENT failed: %v", err)
	}
	if !eventReceived {
		t.Error("event handler not called")
	}
	if len(ackData) != 1 || ackData[0] != "ok" {
		t.Errorf("unexpected ack: %v", ackData)
	}

	// 3. DISCONNECT (transport close)
	s.DisconnectAll("sid-1", "transport close")
	if disconnectSID != "sid-1" {
		t.Error("disconnect handler not called")
	}
	if disconnectReason != "transport close" {
		t.Errorf("expected reason 'transport close', got %q", disconnectReason)
	}
	if s.GetNamespace("/").HasSocket("sid-1") {
		t.Error("socket should be removed after disconnect")
	}
}

// --- Broadcast tests ---

func TestServer_NewServer_HasRoomManager(t *testing.T) {
	s := NewServer()
	if s.Rooms == nil {
		t.Fatal("NewServer should initialize RoomManager")
	}
}

// mockSendFunc creates a SendFunc that records all sent messages.
// Returns the SendFunc and a pointer to the recorded messages map (sid → []data).
func mockSendFunc() (SendFunc, *map[string][]string) {
	sent := make(map[string][]string)
	fn := func(sid string, data string) error {
		sent[sid] = append(sent[sid], data)
		return nil
	}
	return fn, &sent
}

func TestServer_BroadcastToRoom_Basic(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.Rooms.Join("sid-1", "game:42")
	s.Rooms.Join("sid-2", "game:42")
	s.Rooms.Join("sid-3", "game:42")

	n, err := s.BroadcastToRoom("/", "game:42", "fire:update", map[string]interface{}{
		"gridId": "1:2",
		"stage":  1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3 sends, got %d", n)
	}

	for _, sid := range []string{"sid-1", "sid-2", "sid-3"} {
		msgs, ok := (*sent)[sid]
		if !ok {
			t.Errorf("expected %s to receive a message", sid)
			continue
		}
		if len(msgs) != 1 {
			t.Errorf("expected 1 message for %s, got %d", sid, len(msgs))
		}
	}
}

func TestServer_BroadcastToRoom_PacketFormat(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.Rooms.Join("sid-1", "room-a")

	_, err := s.BroadcastToRoom("/", "room-a", "hello", "world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := (*sent)["sid-1"]
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	pkt, err := Decode(msgs[0])
	if err != nil {
		t.Fatalf("failed to decode sent packet: %v", err)
	}
	if pkt.Type != PacketEvent {
		t.Errorf("expected EVENT type, got %d", pkt.Type)
	}
	if pkt.Namespace != "/" {
		t.Errorf("expected namespace '/', got %q", pkt.Namespace)
	}

	eventName, err := pkt.EventName()
	if err != nil {
		t.Fatalf("failed to get event name: %v", err)
	}
	if eventName != "hello" {
		t.Errorf("expected event 'hello', got %q", eventName)
	}
}

func TestServer_BroadcastToRoom_CustomNamespace(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	// Join via the /chat namespace's RoomManager
	chatNS := s.Of("/chat")
	chatNS.Rooms.Join("sid-1", "room-a")

	_, err := s.BroadcastToRoom("/chat", "room-a", "msg", "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := (*sent)["sid-1"]
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	pkt, err := Decode(msgs[0])
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if pkt.Namespace != "/chat" {
		t.Errorf("expected namespace '/chat', got %q", pkt.Namespace)
	}
}

func TestServer_BroadcastToRoom_NamespaceIsolation(t *testing.T) {
	// Rooms in different namespaces should be isolated
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	// sid-1 in room-a of default namespace
	s.Rooms.Join("sid-1", "room-a")
	// sid-2 in room-a of /chat namespace
	s.Of("/chat").Rooms.Join("sid-2", "room-a")

	// Broadcast to room-a in default namespace — only sid-1 should receive
	n, err := s.BroadcastToRoom("/", "room-a", "event", "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 send, got %d", n)
	}
	if _, ok := (*sent)["sid-1"]; !ok {
		t.Error("sid-1 should have received the message")
	}
	if _, ok := (*sent)["sid-2"]; ok {
		t.Error("sid-2 should NOT have received the message (different namespace)")
	}
}

func TestServer_BroadcastToRoom_EmptyRoom(t *testing.T) {
	s := NewServer()
	sendFn, _ := mockSendFunc()
	s.SendTo = sendFn

	n, err := s.BroadcastToRoom("/", "empty-room", "event", "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 sends for empty room, got %d", n)
	}
}

func TestServer_BroadcastToRoom_NoSendFunc(t *testing.T) {
	s := NewServer()
	// SendTo is nil

	s.Rooms.Join("sid-1", "room-a")

	_, err := s.BroadcastToRoom("/", "room-a", "event", "data")
	if err == nil {
		t.Fatal("expected error when SendTo is nil")
	}
}

func TestServer_BroadcastToRoomExcept_Basic(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.Rooms.Join("sid-1", "room-a")
	s.Rooms.Join("sid-2", "room-a")
	s.Rooms.Join("sid-3", "room-a")

	n, err := s.BroadcastToRoomExcept("/", "room-a", "sid-2", "update", "payload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 sends, got %d", n)
	}

	// sid-2 should NOT have received the message
	if msgs, ok := (*sent)["sid-2"]; ok && len(msgs) > 0 {
		t.Error("sid-2 should not have received the message")
	}

	// sid-1 and sid-3 should have received the message
	for _, sid := range []string{"sid-1", "sid-3"} {
		msgs, ok := (*sent)[sid]
		if !ok || len(msgs) != 1 {
			t.Errorf("expected %s to receive 1 message", sid)
		}
	}
}

func TestServer_BroadcastToRoomExcept_ExcludeOnlyMember(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.Rooms.Join("sid-1", "room-a")

	n, err := s.BroadcastToRoomExcept("/", "room-a", "sid-1", "event", "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 sends, got %d", n)
	}
	if len(*sent) != 0 {
		t.Error("no messages should have been sent")
	}
}

func TestServer_BroadcastToRoomExcept_ExcludeNonMember(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.Rooms.Join("sid-1", "room-a")
	s.Rooms.Join("sid-2", "room-a")

	// Exclude a sid that isn't in the room — should send to both members
	n, err := s.BroadcastToRoomExcept("/", "room-a", "sid-999", "event", "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 sends, got %d", n)
	}
	if len(*sent) != 2 {
		t.Errorf("expected 2 sids to receive messages, got %d", len(*sent))
	}
}

func TestServer_BroadcastToRoom_SendError(t *testing.T) {
	s := NewServer()
	callCount := 0
	s.SendTo = func(sid string, data string) error {
		callCount++
		if sid == "sid-2" {
			return fmt.Errorf("connection closed")
		}
		return nil
	}

	s.Rooms.Join("sid-1", "room-a")
	s.Rooms.Join("sid-2", "room-a")
	s.Rooms.Join("sid-3", "room-a")

	n, err := s.BroadcastToRoom("/", "room-a", "event", "data")
	// Should still send to others despite one failure
	if n < 2 {
		t.Errorf("expected at least 2 successful sends, got %d", n)
	}
	// err should be non-nil because one send failed
	if err == nil {
		t.Error("expected error from failed send")
	}
}

func TestServer_BroadcastToRoom_MultipleArgs(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.Rooms.Join("sid-1", "room-a")

	_, err := s.BroadcastToRoom("/", "room-a", "update", "arg1", 42, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := (*sent)["sid-1"]
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	pkt, err := Decode(msgs[0])
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(pkt.Data, &arr); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	// Should be ["update", "arg1", 42, true]
	if len(arr) != 4 {
		t.Errorf("expected 4 elements, got %d", len(arr))
	}
}

func TestServer_BroadcastToRoom_NoArgs(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.Rooms.Join("sid-1", "room-a")

	_, err := s.BroadcastToRoom("/", "room-a", "ping")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := (*sent)["sid-1"]
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	pkt, err := Decode(msgs[0])
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(pkt.Data, &arr); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	// Should be ["ping"] — event name only
	if len(arr) != 1 {
		t.Errorf("expected 1 element (event name only), got %d", len(arr))
	}
}

func TestServer_BroadcastToRoom_Concurrent(t *testing.T) {
	s := NewServer()
	var mu sync.Mutex
	sent := make(map[string]int)
	s.SendTo = func(sid string, data string) error {
		mu.Lock()
		sent[sid]++
		mu.Unlock()
		return nil
	}

	// Add many sockets to a room
	const numSockets = 100
	for i := 0; i < numSockets; i++ {
		s.Rooms.Join(fmt.Sprintf("sid-%d", i), "big-room")
	}

	// Broadcast concurrently
	var wg sync.WaitGroup
	const numBroadcasts = 50
	for i := 0; i < numBroadcasts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.BroadcastToRoom("/", "big-room", "event", "data")
		}()
	}
	wg.Wait()

	// Each socket should have received numBroadcasts messages
	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < numSockets; i++ {
		sid := fmt.Sprintf("sid-%d", i)
		if sent[sid] != numBroadcasts {
			t.Errorf("expected %s to receive %d messages, got %d", sid, numBroadcasts, sent[sid])
		}
	}
}

func TestServer_BroadcastToRoomExcept_Concurrent(t *testing.T) {
	s := NewServer()
	var mu sync.Mutex
	sent := make(map[string]int)
	s.SendTo = func(sid string, data string) error {
		mu.Lock()
		sent[sid]++
		mu.Unlock()
		return nil
	}

	const numSockets = 50
	for i := 0; i < numSockets; i++ {
		s.Rooms.Join(fmt.Sprintf("sid-%d", i), "room")
	}

	var wg sync.WaitGroup
	const numBroadcasts = 30
	for i := 0; i < numBroadcasts; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			excludeSID := fmt.Sprintf("sid-%d", n%numSockets)
			s.BroadcastToRoomExcept("/", "room", excludeSID, "event", "data")
		}(i)
	}
	wg.Wait()

	// sid-0 should be excluded from broadcasts where n%numSockets == 0
	// Total messages for any socket = numBroadcasts - (times it was excluded)
	// This is just a race-safety test, so no panics = pass
}

func TestServer_NamespaceRooms_AreIsolated(t *testing.T) {
	s := NewServer()
	s.Of("/chat")

	// Verify each namespace has its own RoomManager
	rootRooms := s.Of("/").Rooms
	chatRooms := s.Of("/chat").Rooms

	if rootRooms == chatRooms {
		t.Fatal("each namespace should have its own RoomManager instance")
	}
}

func TestServer_Rooms_AliasesDefaultNamespace(t *testing.T) {
	s := NewServer()

	// Server.Rooms should be the same as the default namespace's RoomManager
	if s.Rooms != s.Of("/").Rooms {
		t.Fatal("Server.Rooms should alias the default namespace's RoomManager")
	}
}

func TestServer_DisconnectAll_CleansUpNamespaceRooms(t *testing.T) {
	s := NewServer()
	s.Of("/chat")
	sendFn, _ := mockSendFunc()
	s.SendTo = sendFn

	// Connect and join rooms in both namespaces
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/", ID: -1})
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/chat", ID: -1})
	s.Of("/").Rooms.Join("sid-1", "lobby")
	s.Of("/chat").Rooms.Join("sid-1", "chat:global")

	// Verify rooms before disconnect
	if !s.Of("/").Rooms.InRoom("sid-1", "lobby") {
		t.Fatal("sid-1 should be in lobby of /")
	}
	if !s.Of("/chat").Rooms.InRoom("sid-1", "chat:global") {
		t.Fatal("sid-1 should be in chat:global of /chat")
	}

	// DisconnectAll should clean up rooms in all namespaces
	s.DisconnectAll("sid-1", "transport close")

	if s.Of("/").Rooms.InRoom("sid-1", "lobby") {
		t.Error("sid-1 should not be in lobby after disconnect")
	}
	if s.Of("/chat").Rooms.InRoom("sid-1", "chat:global") {
		t.Error("sid-1 should not be in chat:global after disconnect")
	}
}

func TestServer_BroadcastToNamespace(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	// Connect sockets to different namespaces
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/", ID: -1})
	s.DispatchPacketFull("sid-2", &Packet{Type: PacketConnect, Namespace: "/", ID: -1})
	s.Of("/chat")
	s.DispatchPacketFull("sid-3", &Packet{Type: PacketConnect, Namespace: "/chat", ID: -1})

	// Broadcast to default namespace — should reach sid-1 and sid-2 only
	n, err := s.BroadcastToNamespace("/", "notification", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 sends, got %d", n)
	}
	if _, ok := (*sent)["sid-1"]; !ok {
		t.Error("sid-1 should have received the broadcast")
	}
	if _, ok := (*sent)["sid-2"]; !ok {
		t.Error("sid-2 should have received the broadcast")
	}
	if _, ok := (*sent)["sid-3"]; ok {
		t.Error("sid-3 should NOT have received the broadcast (different namespace)")
	}
}

func TestServer_BroadcastToNamespace_NotFound(t *testing.T) {
	s := NewServer()
	sendFn, _ := mockSendFunc()
	s.SendTo = sendFn

	_, err := s.BroadcastToNamespace("/nonexistent", "event", "data")
	if err == nil {
		t.Fatal("expected error for nonexistent namespace")
	}
}

func TestServer_SendToSocket(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	err := s.SendToSocket("/", "sid-1", "private", "msg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := (*sent)["sid-1"]
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	pkt, err := Decode(msgs[0])
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if pkt.Type != PacketEvent {
		t.Errorf("expected EVENT type, got %d", pkt.Type)
	}
	eventName, _ := pkt.EventName()
	if eventName != "private" {
		t.Errorf("expected event 'private', got %q", eventName)
	}
}

func TestServer_BroadcastToRoom_NamespaceNotFound(t *testing.T) {
	s := NewServer()
	sendFn, _ := mockSendFunc()
	s.SendTo = sendFn

	_, err := s.BroadcastToRoom("/nonexistent", "room", "event", "data")
	if err == nil {
		t.Fatal("expected error for nonexistent namespace")
	}
}

func TestServer_BroadcastToRoom_CrossNamespaceIsolation_Full(t *testing.T) {
	// Comprehensive test: same room name, different namespaces, different data
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn
	s.Of("/chat")

	// Both sockets join "room-x" but in different namespaces
	s.Of("/").Rooms.Join("sid-1", "room-x")
	s.Of("/chat").Rooms.Join("sid-2", "room-x")

	// Broadcast to room-x in /chat — only sid-2 should receive
	n, err := s.BroadcastToRoom("/chat", "room-x", "msg", "chat-data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 send, got %d", n)
	}
	if _, ok := (*sent)["sid-1"]; ok {
		t.Error("sid-1 should NOT receive (different namespace)")
	}
	if msgs, ok := (*sent)["sid-2"]; !ok || len(msgs) != 1 {
		t.Error("sid-2 should have received 1 message")
	}
}

func TestServer_BroadcastToRoom_SamePacketToAll(t *testing.T) {
	// Verify all recipients receive the exact same encoded packet
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.Rooms.Join("sid-1", "room")
	s.Rooms.Join("sid-2", "room")
	s.Rooms.Join("sid-3", "room")

	_, err := s.BroadcastToRoom("/", "room", "update", "payload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All sockets should receive the same encoded packet
	var firstMsg string
	for sid, msgs := range *sent {
		if len(msgs) != 1 {
			t.Fatalf("expected 1 message for %s", sid)
		}
		if firstMsg == "" {
			firstMsg = msgs[0]
		} else if msgs[0] != firstMsg {
			t.Errorf("expected all sockets to receive same packet, %s got %q, first was %q", sid, msgs[0], firstMsg)
		}
	}
}

// --- Middleware integration tests on Server ---

func TestServer_Use_DefaultNamespace(t *testing.T) {
	s := NewServer()
	var order []string

	s.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw")
		return next()
	})

	s.On("ping", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		order = append(order, "handler")
		return []interface{}{"pong"}, nil
	})

	// Connect first
	connectPkt, _ := NewConnectPacket("/", "")
	s.DispatchPacketFull("sid-1", connectPkt)

	// Dispatch event
	eventPkt, _ := NewEventPacket("/", "ping", 0)
	result, err := s.DispatchPacketFull("sid-1", eventPkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.AckData) != 1 || result.AckData[0] != "pong" {
		t.Fatalf("unexpected ack: %v", result.AckData)
	}

	expected := []string{"mw", "handler"}
	if len(order) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("order[%d]=%q, want %q", i, v, order[i])
		}
	}
}

func TestServer_Use_NamespaceScoped(t *testing.T) {
	s := NewServer()
	var defaultMwCalled, chatMwCalled bool

	s.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		defaultMwCalled = true
		return next()
	})

	s.Of("/chat").Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		chatMwCalled = true
		return next()
	})

	s.Of("/chat").On("msg", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, nil
	})

	// Connect to /chat
	connectPkt, _ := NewConnectPacket("/chat", "")
	s.DispatchPacketFull("sid-1", connectPkt)

	// Dispatch event to /chat
	eventPkt, _ := NewEventPacket("/chat", "msg", 0)
	_, err := s.DispatchPacketFull("sid-1", eventPkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if defaultMwCalled {
		t.Error("default namespace middleware should NOT run for /chat events")
	}
	if !chatMwCalled {
		t.Error("/chat namespace middleware should have run")
	}
}

// --- Server-level error handling and chain interruption tests ---

func TestServer_DispatchPacket_MiddlewareError_PropagatesAsDispatchError(t *testing.T) {
	s := NewServer()
	errAuth := errors.New("unauthorized")

	// Add error-producing middleware to default namespace
	s.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return nil, errAuth
	})

	s.On("action", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		t.Fatal("handler should not be called when middleware returns error")
		return nil, nil
	})

	// Connect first
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/", ID: -1})

	// Dispatch event — should fail with middleware error
	eventPkt, _ := NewEventPacket("/", "action", -1)
	_, err := s.DispatchPacket("sid-1", eventPkt)
	if !errors.Is(err, errAuth) {
		t.Fatalf("expected errAuth from dispatch, got %v", err)
	}
}

func TestServer_DispatchPacket_MiddlewareShortCircuit_ReturnsAckData(t *testing.T) {
	s := NewServer()

	// Middleware short-circuits with custom response
	s.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		if event == "cached" {
			return []interface{}{"cached_response"}, nil
		}
		return next()
	})

	s.On("cached", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		t.Fatal("handler should not be called for cached event")
		return nil, nil
	})

	// Connect
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/", ID: -1})

	// Dispatch with ack
	eventPkt, _ := NewEventPacket("/", "cached", 1)
	ackData, err := s.DispatchPacket("sid-1", eventPkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ackData) != 1 || ackData[0] != "cached_response" {
		t.Fatalf("expected cached_response ack, got %v", ackData)
	}
}

func TestServer_DispatchRaw_MiddlewareError_InChain(t *testing.T) {
	s := NewServer()
	errRateLimit := errors.New("rate limited")

	var mwOrder []string

	// mw1: logging pass-through
	s.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		mwOrder = append(mwOrder, "log-before")
		result, err := next()
		mwOrder = append(mwOrder, "log-after")
		return result, err
	})

	// mw2: rate limiter that blocks
	s.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		mwOrder = append(mwOrder, "ratelimit-reject")
		return nil, errRateLimit
	})

	s.On("action", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		mwOrder = append(mwOrder, "handler")
		return nil, nil
	})

	// Connect
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/", ID: -1})

	// Dispatch via raw
	_, _, err := s.DispatchRaw("sid-1", `2["action"]`)
	if !errors.Is(err, errRateLimit) {
		t.Fatalf("expected errRateLimit, got %v", err)
	}

	// mw1 wraps mw2, so: log-before → ratelimit-reject → log-after
	expected := []string{"log-before", "ratelimit-reject", "log-after"}
	if len(mwOrder) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, mwOrder)
	}
	for i, v := range expected {
		if mwOrder[i] != v {
			t.Fatalf("expected order[%d]=%q, got %q", i, v, mwOrder[i])
		}
	}
}

func TestServer_DispatchPacket_HandlerError_PropagatesThroughMiddleware(t *testing.T) {
	s := NewServer()
	errDB := errors.New("database error")

	var middlewareSawError bool
	s.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		result, err := next()
		if err != nil {
			middlewareSawError = true
		}
		return result, err
	})

	s.On("save", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, errDB
	})

	// Connect
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/", ID: -1})

	eventPkt, _ := NewEventPacket("/", "save", -1)
	_, err := s.DispatchPacket("sid-1", eventPkt)
	if !errors.Is(err, errDB) {
		t.Fatalf("expected errDB, got %v", err)
	}
	if !middlewareSawError {
		t.Fatal("middleware should have seen the handler error")
	}
}

func TestServer_DispatchPacketFull_MiddlewareError_NoResponsePacket(t *testing.T) {
	s := NewServer()
	errForbidden := errors.New("forbidden")

	s.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return nil, errForbidden
	})

	s.On("action", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, nil
	})

	// Connect
	s.DispatchPacketFull("sid-1", &Packet{Type: PacketConnect, Namespace: "/", ID: -1})

	eventPkt, _ := NewEventPacket("/", "action", 5)
	result, err := s.DispatchPacketFull("sid-1", eventPkt)
	if !errors.Is(err, errForbidden) {
		t.Fatalf("expected errForbidden, got %v", err)
	}
	// For EVENT packets, error should propagate but result should still exist
	if result == nil {
		t.Fatal("result should not be nil even on error")
	}
	// AckData should be nil since middleware errored
	if result.AckData != nil {
		t.Fatalf("expected nil ack data on error, got %v", result.AckData)
	}
}
