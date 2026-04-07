package sio

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"testing"

	socketio "github.com/homeworldio/socketio-go"
)

// newTestHandlerWithSent creates a Handler with a mock SendTo that records sent packets.
func newTestHandlerWithSent(t *testing.T) (*Handler, *[]sentEvent) {
	t.Helper()
	sioServer := socketio.NewServer()
	var mu sync.Mutex
	var sent []sentEvent

	sioServer.SendTo = func(sid string, data string) error {
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, sentEvent{SID: sid, Data: data})
		return nil
	}

	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	h := NewHandler(sioServer, logger)
	return h, &sent
}

func TestHandleConnect_CreatesSession(t *testing.T) {
	h := newTestHandler()

	// Simulate connect with no auth
	err := h.handleConnect("sid1", nil)
	if err != nil {
		t.Fatalf("handleConnect returned error: %v", err)
	}

	// Verify session was created in the connection map
	info := h.manager.Get("sid1")
	if info == nil {
		t.Fatal("session not found in connection map after connect")
	}
	if info.SID != "sid1" {
		t.Errorf("SID = %q, want %q", info.SID, "sid1")
	}
	if info.ReconnectCount != 0 {
		t.Errorf("ReconnectCount = %d, want 0", info.ReconnectCount)
	}
	if h.manager.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", h.manager.ActiveCount())
	}
}

func TestHandleConnect_WithUserID(t *testing.T) {
	h := newTestHandler()

	auth, _ := json.Marshal(connectAuth{UserID: "user-123"})
	err := h.handleConnect("sid1", auth)
	if err != nil {
		t.Fatalf("handleConnect returned error: %v", err)
	}

	info := h.manager.Get("sid1")
	if info == nil {
		t.Fatal("session not found")
	}
	if info.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", info.UserID, "user-123")
	}
}

func TestHandleConnect_Reconnection(t *testing.T) {
	h, _ := newTestHandlerWithSent(t)

	// First connection with user_id
	auth, _ := json.Marshal(connectAuth{UserID: "user-123"})
	_ = h.handleConnect("sid1", auth)

	// Add rooms to first session
	info1 := h.manager.Get("sid1")
	info1.Rooms["grid:1"] = struct{}{}
	info1.Rooms["grid:2"] = struct{}{}

	// Disconnect first session via the registered disconnect handler
	h.sio.DisconnectAll("sid1", "transport close")

	// Reconnect with same user_id — should detect reconnection
	_ = h.handleConnect("sid2", auth)

	info2 := h.manager.Get("sid2")
	if info2 == nil {
		t.Fatal("reconnected session not found")
	}
	if info2.ReconnectCount != 1 {
		t.Errorf("ReconnectCount = %d, want 1", info2.ReconnectCount)
	}
	// Rooms should be restored
	if len(info2.Rooms) != 2 {
		t.Errorf("restored rooms = %d, want 2", len(info2.Rooms))
	}
}

func TestHandleConnect_EmitsConnectedEvent(t *testing.T) {
	h, sent := newTestHandlerWithSent(t)

	// Need to add sid1 to the namespace so broadcast can reach it
	ns := h.sio.Of("/")
	_ = ns.HandleConnect("sid1", nil)

	err := h.handleConnect("sid1", nil)
	if err != nil {
		t.Fatalf("handleConnect returned error: %v", err)
	}

	// Should have sent at least 2 packets:
	// 1. "connected" to sid1
	// 2. "users:count" broadcast to sid1
	if len(*sent) < 2 {
		t.Fatalf("expected at least 2 sent packets, got %d", len(*sent))
	}

	// First packet should be the "connected" ack to sid1
	first := (*sent)[0]
	if first.SID != "sid1" {
		t.Errorf("first packet SID = %q, want %q", first.SID, "sid1")
	}

	// Decode and verify "connected" event
	eventName, payload := decodeEventPayload(t, first.Data)
	if eventName != "connected" {
		t.Errorf("event name = %q, want %q", eventName, "connected")
	}

	// Verify payload fields match Python server output
	if payload["sid"] != "sid1" {
		t.Errorf("payload sid = %v, want %q", payload["sid"], "sid1")
	}
	if payload["heartbeat_interval"] == nil {
		t.Error("payload missing heartbeat_interval")
	}
	if payload["reconnect_count"].(float64) != 0 {
		t.Errorf("reconnect_count = %v, want 0", payload["reconnect_count"])
	}
	if payload["restored_rooms"].(float64) != 0 {
		t.Errorf("restored_rooms = %v, want 0", payload["restored_rooms"])
	}
}

func TestHandleConnect_BroadcastsUsersCount(t *testing.T) {
	h, sent := newTestHandlerWithSent(t)

	// Connect sid1 so it's in namespace for broadcast
	ns := h.sio.Of("/")
	_ = ns.HandleConnect("sid1", nil)
	_ = h.handleConnect("sid1", nil)

	// Find users:count event
	found := false
	for _, ev := range *sent {
		if ev.SID == "sid1" {
			name, payload := decodeEventPayload(t, ev.Data)
			if name == "users:count" {
				if payload["count"].(float64) != 1 {
					t.Errorf("users:count = %v, want 1", payload["count"])
				}
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("users:count broadcast not received by sid1")
	}
}

func TestHandleConnect_RegisteredOnServer(t *testing.T) {
	sioServer := socketio.NewServer()
	sioServer.SendTo = func(sid string, data string) error {
		return nil
	}
	_ = NewHandler(sioServer, log.Default())

	// Dispatching a CONNECT packet should trigger our handler and succeed
	pkt := &socketio.Packet{
		Type:      socketio.PacketConnect,
		Namespace: "/",
	}
	result, err := sioServer.DispatchPacketFull("test-sid", pkt)
	if err != nil {
		t.Fatalf("DispatchPacketFull returned error: %v", err)
	}
	if result.ResponsePacket == nil {
		t.Fatal("expected a response packet (CONNECT ACK)")
	}
}

func TestHandleConnect_DisconnectRemovesSession(t *testing.T) {
	h, _ := newTestHandlerWithSent(t)

	// Dispatch a full CONNECT packet so namespace also tracks the socket
	pkt := &socketio.Packet{Type: socketio.PacketConnect, Namespace: "/"}
	_, _ = h.sio.DispatchPacketFull("sid1", pkt)

	if h.manager.ActiveCount() != 1 {
		t.Fatalf("ActiveCount = %d, want 1 after connect", h.manager.ActiveCount())
	}

	// DisconnectAll triggers the registered disconnect handler
	h.sio.DisconnectAll("sid1", "transport close")

	if h.manager.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d, want 0 after disconnect", h.manager.ActiveCount())
	}
	if info := h.manager.Get("sid1"); info != nil {
		t.Error("session should be removed after disconnect")
	}
}

func TestHandleConnect_DisconnectPreservesUserSession(t *testing.T) {
	h := newTestHandler()

	// Use DispatchPacketFull with auth so namespace tracks the socket
	auth, _ := json.Marshal(connectAuth{UserID: "user-123"})
	pkt := &socketio.Packet{Type: socketio.PacketConnect, Namespace: "/", Data: auth}
	_, _ = h.sio.DispatchPacketFull("sid1", pkt)
	h.sio.DisconnectAll("sid1", "transport close")

	// user_sessions should still have the entry for reconnection
	prev := h.manager.GetPreviousSession("user-123")
	if prev == nil {
		t.Error("user session should be preserved after disconnect for reconnection")
	}
}

func TestMultipleConnections(t *testing.T) {
	h := newTestHandler()

	// Use DispatchPacketFull so namespace also tracks the sockets
	for _, sid := range []string{"sid1", "sid2", "sid3"} {
		pkt := &socketio.Packet{Type: socketio.PacketConnect, Namespace: "/"}
		_, _ = h.sio.DispatchPacketFull(sid, pkt)
	}

	if h.manager.ActiveCount() != 3 {
		t.Errorf("ActiveCount = %d, want 3", h.manager.ActiveCount())
	}

	h.sio.DisconnectAll("sid2", "transport close")
	if h.manager.ActiveCount() != 2 {
		t.Errorf("ActiveCount = %d, want 2 after one disconnect", h.manager.ActiveCount())
	}
}
