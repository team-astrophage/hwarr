package sio

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"testing"

	socketio "github.com/homeworldio/socketio-go"
)

// sentEvent captures a broadcast sent via the mock SendFunc.
type sentEvent struct {
	SID  string
	Data string
}

// setupTestServer creates a socketio.Server with a mock SendFunc that records
// all sent packets. Returns the server, connection manager, and a function to
// retrieve sent events.
func setupTestServer(t *testing.T) (*socketio.Server, *ConnectionManager, *[]sentEvent) {
	t.Helper()

	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	server := socketio.NewServer()
	manager := NewConnectionManager(logger)

	var mu sync.Mutex
	var events []sentEvent

	server.SendTo = func(sid string, data string) error {
		mu.Lock()
		events = append(events, sentEvent{SID: sid, Data: data})
		mu.Unlock()
		return nil
	}

	RegisterDisconnectHandler(server, manager, logger)

	return server, manager, &events
}

// simulateConnect adds a socket to the connection manager and namespace.
func simulateConnect(t *testing.T, server *socketio.Server, manager *ConnectionManager, sid string) {
	t.Helper()
	manager.Add(sid, "")
	ns := server.Of("/")
	_ = ns.HandleConnect(sid, nil)
}

// findEventsForSID filters sent events by target socket ID.
func findEventsForSID(events []sentEvent, sid string) []sentEvent {
	var result []sentEvent
	for _, e := range events {
		if e.SID == sid {
			result = append(result, e)
		}
	}
	return result
}

// decodeEventPayload parses a Socket.IO wire-format event to extract event name and payload.
// Wire format: "2["event_name",{...}]" (type 2 = EVENT)
func decodeEventPayload(t *testing.T, data string) (string, map[string]interface{}) {
	t.Helper()

	// Find the JSON array after the packet type prefix
	idx := 0
	for idx < len(data) && data[idx] != '[' {
		idx++
	}
	if idx >= len(data) {
		t.Fatalf("no JSON array found in packet: %s", data)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(data[idx:]), &arr); err != nil {
		t.Fatalf("failed to unmarshal event array: %v (data: %s)", err, data)
	}

	if len(arr) < 2 {
		t.Fatalf("expected at least 2 elements in event array, got %d", len(arr))
	}

	var eventName string
	if err := json.Unmarshal(arr[0], &eventName); err != nil {
		t.Fatalf("failed to unmarshal event name: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(arr[1], &payload); err != nil {
		t.Fatalf("failed to unmarshal event payload: %v", err)
	}

	return eventName, payload
}

func TestDisconnect_RemovesConnection(t *testing.T) {
	server, manager, _ := setupTestServer(t)

	simulateConnect(t, server, manager, "sid1")

	if manager.ActiveCount() != 1 {
		t.Fatalf("expected 1 active connection, got %d", manager.ActiveCount())
	}

	// Trigger disconnect
	server.DisconnectAll("sid1", "transport close")

	if manager.ActiveCount() != 0 {
		t.Fatalf("expected 0 active connections after disconnect, got %d", manager.ActiveCount())
	}

	if info := manager.Get("sid1"); info != nil {
		t.Fatal("expected nil info after disconnect")
	}
}

func TestDisconnect_BroadcastsUsersCount(t *testing.T) {
	server, manager, events := setupTestServer(t)

	// Connect two clients
	simulateConnect(t, server, manager, "sid1")
	simulateConnect(t, server, manager, "sid2")

	// Disconnect sid1
	server.DisconnectAll("sid1", "transport close")

	// sid2 should receive users:count with count=1
	sid2Events := findEventsForSID(*events, "sid2")

	found := false
	for _, e := range sid2Events {
		eventName, payload := decodeEventPayload(t, e.Data)
		if eventName == "users:count" {
			count, ok := payload["count"]
			if !ok {
				t.Fatal("users:count payload missing 'count' field")
			}
			// JSON numbers are float64
			if count.(float64) != 1 {
				t.Fatalf("expected users:count=1, got %v", count)
			}
			found = true
			break
		}
	}

	if !found {
		t.Fatal("sid2 did not receive users:count broadcast")
	}
}

func TestDisconnect_UsersCountZeroWhenAllDisconnect(t *testing.T) {
	server, manager, events := setupTestServer(t)

	simulateConnect(t, server, manager, "sid1")
	simulateConnect(t, server, manager, "sid2")

	// Disconnect both
	server.DisconnectAll("sid1", "transport close")
	*events = nil // clear events
	server.DisconnectAll("sid2", "transport close")

	// After sid2 disconnects, count should be 0
	// No one left to receive the broadcast, but the handler still runs
	if manager.ActiveCount() != 0 {
		t.Fatalf("expected 0 active connections, got %d", manager.ActiveCount())
	}
}

func TestDisconnect_ChatPresenceBroadcast(t *testing.T) {
	server, manager, events := setupTestServer(t)

	// Connect three clients
	simulateConnect(t, server, manager, "sid1")
	simulateConnect(t, server, manager, "sid2")
	simulateConnect(t, server, manager, "sid3")

	// sid1 and sid2 join the chat room
	ns := server.Of("/")
	ns.Rooms.Join("sid1", ChatRoom)
	ns.Rooms.Join("sid2", ChatRoom)

	// Track chat room membership in connection manager too
	manager.SetRooms("sid1", []string{ChatRoom})
	manager.SetRooms("sid2", []string{ChatRoom})

	// Disconnect sid1 (who was in chat)
	server.DisconnectAll("sid1", "transport close")

	// sid2 should receive chat:presence with count=1 (only sid2 remains in chat)
	sid2Events := findEventsForSID(*events, "sid2")

	foundPresence := false
	for _, e := range sid2Events {
		eventName, payload := decodeEventPayload(t, e.Data)
		if eventName == "chat:presence" {
			count, ok := payload["count"]
			if !ok {
				t.Fatal("chat:presence payload missing 'count' field")
			}
			if count.(float64) != 1 {
				t.Fatalf("expected chat:presence count=1, got %v", count)
			}
			foundPresence = true
			break
		}
	}

	if !foundPresence {
		t.Fatal("sid2 did not receive chat:presence broadcast after sid1 disconnect")
	}

	// sid3 should NOT receive chat:presence (not in chat room)
	sid3Events := findEventsForSID(*events, "sid3")
	for _, e := range sid3Events {
		eventName, _ := decodeEventPayload(t, e.Data)
		if eventName == "chat:presence" {
			t.Fatal("sid3 should not receive chat:presence (not in chat room)")
		}
	}
}

func TestDisconnect_NoChatPresenceWhenNotInChat(t *testing.T) {
	server, manager, events := setupTestServer(t)

	simulateConnect(t, server, manager, "sid1")
	simulateConnect(t, server, manager, "sid2")

	// Neither client is in chat room

	// Disconnect sid1
	server.DisconnectAll("sid1", "transport close")

	// sid2 should receive users:count but NOT chat:presence
	sid2Events := findEventsForSID(*events, "sid2")

	for _, e := range sid2Events {
		eventName, _ := decodeEventPayload(t, e.Data)
		if eventName == "chat:presence" {
			t.Fatal("chat:presence should not be broadcast when disconnecting user was not in chat")
		}
	}
}

func TestDisconnect_RoomCleanup(t *testing.T) {
	server, manager, _ := setupTestServer(t)

	simulateConnect(t, server, manager, "sid1")

	// Join some viewport rooms
	ns := server.Of("/")
	ns.Rooms.Join("sid1", "grid:1234")
	ns.Rooms.Join("sid1", "grid:5678")
	ns.Rooms.Join("sid1", ChatRoom)
	manager.SetRooms("sid1", []string{"grid:1234", "grid:5678", ChatRoom})

	// Verify rooms exist
	if !ns.Rooms.InRoom("sid1", "grid:1234") {
		t.Fatal("expected sid1 in grid:1234")
	}

	// Disconnect — HandleDisconnect calls LeaveAll before our handler
	server.DisconnectAll("sid1", "transport close")

	// Rooms should be cleaned up
	if ns.Rooms.InRoom("sid1", "grid:1234") {
		t.Fatal("sid1 should not be in grid:1234 after disconnect")
	}
	if ns.Rooms.InRoom("sid1", "grid:5678") {
		t.Fatal("sid1 should not be in grid:5678 after disconnect")
	}
	if ns.Rooms.InRoom("sid1", ChatRoom) {
		t.Fatal("sid1 should not be in chat room after disconnect")
	}
}

func TestDisconnect_ClientNamespaceDisconnect(t *testing.T) {
	server, manager, events := setupTestServer(t)

	simulateConnect(t, server, manager, "sid1")
	simulateConnect(t, server, manager, "sid2")

	// Simulate client-side namespace disconnect via packet
	pkt := &socketio.Packet{
		Type:      socketio.PacketDisconnect,
		Namespace: "/",
	}
	_, _ = server.DispatchPacket("sid1", pkt)

	// Connection should be removed
	if manager.ActiveCount() != 1 {
		t.Fatalf("expected 1 active connection after client disconnect, got %d", manager.ActiveCount())
	}

	// sid2 should receive users:count
	sid2Events := findEventsForSID(*events, "sid2")
	found := false
	for _, e := range sid2Events {
		eventName, payload := decodeEventPayload(t, e.Data)
		if eventName == "users:count" {
			if payload["count"].(float64) != 1 {
				t.Fatalf("expected users:count=1, got %v", payload["count"])
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("sid2 did not receive users:count after client namespace disconnect")
	}
}

func TestDisconnect_UnknownSID(t *testing.T) {
	server, manager, _ := setupTestServer(t)

	// Disconnect a socket that was never connected — should not panic
	server.DisconnectAll("unknown-sid", "transport close")

	if manager.ActiveCount() != 0 {
		t.Fatalf("expected 0 connections, got %d", manager.ActiveCount())
	}
}
