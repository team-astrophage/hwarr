package socketio

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"testing"
)

// =============================================================================
// Integration tests for Socket.IO join/leave event handler wiring.
//
// These tests verify the full flow:
//   1. Client connects
//   2. Client emits join event → server adds to room
//   3. Broadcast reaches only room members
//   4. Client emits leave event → server removes from room
//   5. Disconnect triggers automatic LeaveAll cleanup
//
// Matches python-socketio's enter_room/leave_room/disconnect behavior.
// =============================================================================

// --- Helper: connect a socket via DispatchPacketFull ---

func connectSocket(t *testing.T, s *Server, sid string) {
	t.Helper()
	pkt := &Packet{Type: PacketConnect, Namespace: "/", ID: -1}
	_, err := s.DispatchPacketFull(sid, pkt)
	if err != nil {
		t.Fatalf("failed to connect %s: %v", sid, err)
	}
}

func connectSocketNS(t *testing.T, s *Server, sid, namespace string) {
	t.Helper()
	pkt := &Packet{Type: PacketConnect, Namespace: namespace, ID: -1}
	_, err := s.DispatchPacketFull(sid, pkt)
	if err != nil {
		t.Fatalf("failed to connect %s to %s: %v", sid, namespace, err)
	}
}

// --- Integration: join event handler wires to room ---

func TestIntegration_JoinEventAddsToRoom(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	// Register a "chat:join" event that adds the client to a room
	s.On("chat:join", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		var data struct {
			Room string `json:"room"`
		}
		if len(args) > 0 {
			json.Unmarshal(args[0], &data)
		}
		if data.Room == "" {
			data.Room = "chat:global"
		}
		s.EnterRoom("/", sid, data.Room)

		count := s.GetNamespace("/").Rooms.RoomCount(data.Room)
		return []interface{}{map[string]interface{}{
			"status": "ok",
			"count":  count,
		}}, nil
	})

	// Connect two sockets
	connectSocket(t, s, "sid-1")
	connectSocket(t, s, "sid-2")

	// sid-1 joins chat room
	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        1,
		Data:      json.RawMessage(`["chat:join",{"room":"chat:global"}]`),
	}
	ackData, err := s.DispatchPacket("sid-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	// Verify ack response
	if len(ackData) != 1 {
		t.Fatalf("expected 1 ack element, got %d", len(ackData))
	}
	ackMap, ok := ackData[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map ack, got %T", ackData[0])
	}
	if ackMap["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", ackMap["status"])
	}

	// Verify room membership
	if !s.GetNamespace("/").Rooms.InRoom("sid-1", "chat:global") {
		t.Error("sid-1 should be in chat:global room")
	}
	if s.GetNamespace("/").Rooms.InRoom("sid-2", "chat:global") {
		t.Error("sid-2 should NOT be in chat:global room")
	}

	// Broadcast to room should only reach sid-1
	n, err := s.BroadcastToRoom("/", "chat:global", "chat:message", map[string]interface{}{
		"text": "hello",
	})
	if err != nil {
		t.Fatalf("broadcast error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 recipient, got %d", n)
	}
	if len((*sent)["sid-1"]) != 1 {
		t.Error("sid-1 should have received the broadcast")
	}
	if len((*sent)["sid-2"]) != 0 {
		t.Error("sid-2 should NOT have received the broadcast")
	}
}

// --- Integration: leave event handler removes from room ---

func TestIntegration_LeaveEventRemovesFromRoom(t *testing.T) {
	s := NewServer()
	sendFn, _ := mockSendFunc()
	s.SendTo = sendFn

	// Register join and leave handlers
	s.On("chat:join", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		s.EnterRoom("/", sid, "chat:global")
		return []interface{}{map[string]interface{}{"status": "ok"}}, nil
	})

	s.On("chat:leave", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		s.LeaveRoom("/", sid, "chat:global")
		return []interface{}{map[string]interface{}{"status": "ok"}}, nil
	})

	connectSocket(t, s, "sid-1")

	// Join
	joinPkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        1,
		Data:      json.RawMessage(`["chat:join"]`),
	}
	s.DispatchPacket("sid-1", joinPkt)

	if !s.GetNamespace("/").Rooms.InRoom("sid-1", "chat:global") {
		t.Fatal("sid-1 should be in chat:global after join")
	}

	// Leave
	leavePkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        2,
		Data:      json.RawMessage(`["chat:leave"]`),
	}
	s.DispatchPacket("sid-1", leavePkt)

	if s.GetNamespace("/").Rooms.InRoom("sid-1", "chat:global") {
		t.Error("sid-1 should NOT be in chat:global after leave")
	}
}

// --- Integration: disconnect triggers automatic LeaveAll ---

func TestIntegration_DisconnectAutoLeaveAll(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.On("chat:join", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		s.EnterRoom("/", sid, "chat:global")
		return nil, nil
	})

	// Connect and join rooms
	connectSocket(t, s, "sid-1")
	connectSocket(t, s, "sid-2")

	// sid-1 joins multiple rooms
	s.EnterRoom("/", "sid-1", "room-a")
	s.EnterRoom("/", "sid-1", "room-b")
	s.EnterRoom("/", "sid-1", "room-c")

	// sid-2 joins room-a
	s.EnterRoom("/", "sid-2", "room-a")

	// Verify initial state
	rooms := s.SocketRooms("/", "sid-1")
	if len(rooms) != 3 {
		t.Fatalf("expected sid-1 in 3 rooms, got %d", len(rooms))
	}

	// Disconnect sid-1 (simulates transport close)
	s.DisconnectAll("sid-1", "transport close")

	// Verify all rooms cleaned up for sid-1
	rooms = s.SocketRooms("/", "sid-1")
	if rooms != nil {
		t.Errorf("sid-1 should have no rooms after disconnect, got %v", rooms)
	}

	// sid-2 should still be in room-a
	if !s.GetNamespace("/").Rooms.InRoom("sid-2", "room-a") {
		t.Error("sid-2 should still be in room-a")
	}

	// Broadcast to room-a should only reach sid-2
	n, _ := s.BroadcastToRoom("/", "room-a", "test", "data")
	if n != 1 {
		t.Errorf("expected 1 recipient in room-a, got %d", n)
	}
	if len((*sent)["sid-2"]) != 1 {
		t.Error("sid-2 should have received broadcast in room-a")
	}
	if len((*sent)["sid-1"]) != 0 {
		t.Error("sid-1 should NOT receive broadcast after disconnect")
	}
}

// --- Integration: disconnect handler sees rooms already cleaned ---

func TestIntegration_DisconnectHandlerAfterLeaveAll(t *testing.T) {
	s := NewServer()

	var roomsAtDisconnect []string
	s.OnDisconnect(func(sid string, reason string) {
		roomsAtDisconnect = s.SocketRooms("/", sid)
	})

	connectSocket(t, s, "sid-1")
	s.EnterRoom("/", "sid-1", "room-a")
	s.EnterRoom("/", "sid-1", "room-b")

	s.DisconnectAll("sid-1", "transport close")

	// Rooms should already be cleaned before disconnect handler runs
	if roomsAtDisconnect != nil {
		t.Errorf("rooms should be nil in disconnect handler, got %v", roomsAtDisconnect)
	}
}

// --- Integration: viewport subscription replaces rooms ---

func TestIntegration_ReplaceRooms_ViewportSubscription(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	// Register subscribe:viewport handler that replaces rooms
	s.On("subscribe:viewport", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		var data struct {
			GridIDs []string `json:"grid_ids"`
		}
		if len(args) > 0 {
			json.Unmarshal(args[0], &data)
		}
		if err := s.ReplaceRooms("/", sid, data.GridIDs); err != nil {
			return nil, err
		}
		return []interface{}{map[string]interface{}{
			"status":           "ok",
			"subscribed_grids": len(data.GridIDs),
		}}, nil
	})

	connectSocket(t, s, "sid-1")

	// First viewport subscription: grids A, B, C
	pkt1 := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        1,
		Data:      json.RawMessage(`["subscribe:viewport",{"grid_ids":["grid-A","grid-B","grid-C"]}]`),
	}
	s.DispatchPacket("sid-1", pkt1)

	rooms := s.SocketRooms("/", "sid-1")
	sort.Strings(rooms)
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms, got %d: %v", len(rooms), rooms)
	}

	// Second viewport: grids D, E (replaces A, B, C)
	pkt2 := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        2,
		Data:      json.RawMessage(`["subscribe:viewport",{"grid_ids":["grid-D","grid-E"]}]`),
	}
	s.DispatchPacket("sid-1", pkt2)

	rooms = s.SocketRooms("/", "sid-1")
	sort.Strings(rooms)
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms after replace, got %d: %v", len(rooms), rooms)
	}
	if rooms[0] != "grid-D" || rooms[1] != "grid-E" {
		t.Errorf("expected [grid-D, grid-E], got %v", rooms)
	}

	// Broadcast to old room should NOT reach sid-1
	n, _ := s.BroadcastToRoom("/", "grid-A", "fire:update", "data")
	if n != 0 {
		t.Error("broadcast to old room should reach 0 clients")
	}

	// Broadcast to new room should reach sid-1
	n, _ = s.BroadcastToRoom("/", "grid-D", "fire:update", "data")
	if n != 1 {
		t.Errorf("expected 1 recipient in grid-D, got %d", n)
	}
	if len((*sent)["sid-1"]) != 1 {
		t.Error("sid-1 should have received broadcast in grid-D")
	}
}

// --- Integration: multi-client room broadcast ---

func TestIntegration_MultiClientRoomBroadcast(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	// Connect 5 clients
	for i := 1; i <= 5; i++ {
		sid := "sid-" + string(rune('0'+i))
		connectSocket(t, s, sid)
	}

	// sid-1, sid-2, sid-3 join room-a
	s.EnterRoom("/", "sid-1", "room-a")
	s.EnterRoom("/", "sid-2", "room-a")
	s.EnterRoom("/", "sid-3", "room-a")

	// sid-3, sid-4 join room-b
	s.EnterRoom("/", "sid-3", "room-b")
	s.EnterRoom("/", "sid-4", "room-b")

	// Broadcast to room-a
	n, _ := s.BroadcastToRoom("/", "room-a", "update", "data-a")
	if n != 3 {
		t.Errorf("expected 3 recipients in room-a, got %d", n)
	}

	// Broadcast to room-b
	n, _ = s.BroadcastToRoom("/", "room-b", "update", "data-b")
	if n != 2 {
		t.Errorf("expected 2 recipients in room-b, got %d", n)
	}

	// sid-3 should have received both broadcasts
	if len((*sent)["sid-3"]) != 2 {
		t.Errorf("sid-3 should have 2 messages, got %d", len((*sent)["sid-3"]))
	}

	// sid-5 should have received nothing
	if len((*sent)["sid-5"]) != 0 {
		t.Errorf("sid-5 should have 0 messages, got %d", len((*sent)["sid-5"]))
	}
}

// --- Integration: BroadcastToRoomExcept skips sender ---

func TestIntegration_BroadcastExcludeSender(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	connectSocket(t, s, "sid-1")
	connectSocket(t, s, "sid-2")
	connectSocket(t, s, "sid-3")

	s.EnterRoom("/", "sid-1", "game")
	s.EnterRoom("/", "sid-2", "game")
	s.EnterRoom("/", "sid-3", "game")

	// sid-1 broadcasts but should not receive own message
	n, _ := s.BroadcastToRoomExcept("/", "game", "sid-1", "action", "move")
	if n != 2 {
		t.Errorf("expected 2 recipients (excluding sender), got %d", n)
	}
	if len((*sent)["sid-1"]) != 0 {
		t.Error("sender sid-1 should NOT receive broadcast")
	}
	if len((*sent)["sid-2"]) != 1 {
		t.Error("sid-2 should receive broadcast")
	}
	if len((*sent)["sid-3"]) != 1 {
		t.Error("sid-3 should receive broadcast")
	}
}

// --- Integration: namespace-scoped rooms are isolated ---

func TestIntegration_NamespaceRoomIsolation(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	// Create /chat namespace
	s.Of("/chat")

	// Connect sid-1 to both namespaces
	connectSocket(t, s, "sid-1")
	connectSocketNS(t, s, "sid-1", "/chat")

	// Connect sid-2 to / only
	connectSocket(t, s, "sid-2")

	// Join "lobby" in default namespace
	s.EnterRoom("/", "sid-1", "lobby")
	s.EnterRoom("/", "sid-2", "lobby")

	// Join "lobby" in /chat namespace (same room name, different namespace)
	s.EnterRoom("/chat", "sid-1", "lobby")

	// Broadcast to /chat lobby should only reach sid-1
	n, _ := s.BroadcastToRoom("/chat", "lobby", "chat:msg", "hi")
	if n != 1 {
		t.Errorf("expected 1 recipient in /chat lobby, got %d", n)
	}

	// Broadcast to / lobby should reach both
	n, _ = s.BroadcastToRoom("/", "lobby", "update", "data")
	if n != 2 {
		t.Errorf("expected 2 recipients in / lobby, got %d", n)
	}

	// Verify messages
	// sid-1: 1 from /chat + 1 from / = 2
	if len((*sent)["sid-1"]) != 2 {
		t.Errorf("sid-1 expected 2 messages, got %d", len((*sent)["sid-1"]))
	}
	// sid-2: 1 from / only
	if len((*sent)["sid-2"]) != 1 {
		t.Errorf("sid-2 expected 1 message, got %d", len((*sent)["sid-2"]))
	}
}

// --- Integration: DisconnectAll cleans rooms across namespaces ---

func TestIntegration_DisconnectAllCleansAllNamespaces(t *testing.T) {
	s := NewServer()
	s.Of("/chat")

	connectSocket(t, s, "sid-1")
	connectSocketNS(t, s, "sid-1", "/chat")

	s.EnterRoom("/", "sid-1", "room-a")
	s.EnterRoom("/chat", "sid-1", "chat-room")

	// Verify rooms exist
	if !s.GetNamespace("/").Rooms.InRoom("sid-1", "room-a") {
		t.Fatal("sid-1 should be in room-a in /")
	}
	if !s.GetNamespace("/chat").Rooms.InRoom("sid-1", "chat-room") {
		t.Fatal("sid-1 should be in chat-room in /chat")
	}

	// Disconnect all
	s.DisconnectAll("sid-1", "transport close")

	// Verify all rooms cleaned in both namespaces
	if s.GetNamespace("/").Rooms.InRoom("sid-1", "room-a") {
		t.Error("sid-1 should not be in room-a after disconnect")
	}
	if s.GetNamespace("/chat").Rooms.InRoom("sid-1", "chat-room") {
		t.Error("sid-1 should not be in chat-room after disconnect")
	}
}

// --- Integration: EnterRoom/LeaveRoom server convenience methods ---

func TestIntegration_ServerEnterLeaveRoom(t *testing.T) {
	s := NewServer()

	connectSocket(t, s, "sid-1")

	// EnterRoom
	if err := s.EnterRoom("/", "sid-1", "test-room"); err != nil {
		t.Fatalf("EnterRoom error: %v", err)
	}
	if !s.GetNamespace("/").Rooms.InRoom("sid-1", "test-room") {
		t.Error("sid-1 should be in test-room")
	}

	// LeaveRoom
	if err := s.LeaveRoom("/", "sid-1", "test-room"); err != nil {
		t.Fatalf("LeaveRoom error: %v", err)
	}
	if s.GetNamespace("/").Rooms.InRoom("sid-1", "test-room") {
		t.Error("sid-1 should not be in test-room after leave")
	}

	// EnterRoom on nonexistent namespace
	if err := s.EnterRoom("/nonexistent", "sid-1", "room"); err == nil {
		t.Error("expected error for nonexistent namespace")
	}

	// LeaveRoom on nonexistent namespace is a no-op
	if err := s.LeaveRoom("/nonexistent", "sid-1", "room"); err != nil {
		t.Errorf("LeaveRoom on nonexistent namespace should return nil, got %v", err)
	}
}

// --- Integration: LeaveAllRooms ---

func TestIntegration_ServerLeaveAllRooms(t *testing.T) {
	s := NewServer()
	connectSocket(t, s, "sid-1")

	s.EnterRoom("/", "sid-1", "r1")
	s.EnterRoom("/", "sid-1", "r2")
	s.EnterRoom("/", "sid-1", "r3")

	rooms := s.SocketRooms("/", "sid-1")
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms, got %d", len(rooms))
	}

	s.LeaveAllRooms("/", "sid-1")

	rooms = s.SocketRooms("/", "sid-1")
	if rooms != nil {
		t.Errorf("expected nil rooms after LeaveAllRooms, got %v", rooms)
	}
}

// --- Integration: RoomMembers ---

func TestIntegration_RoomMembers(t *testing.T) {
	s := NewServer()

	connectSocket(t, s, "sid-1")
	connectSocket(t, s, "sid-2")
	connectSocket(t, s, "sid-3")

	s.EnterRoom("/", "sid-1", "room-x")
	s.EnterRoom("/", "sid-2", "room-x")

	members := s.RoomMembers("/", "room-x")
	sort.Strings(members)
	if len(members) != 2 || members[0] != "sid-1" || members[1] != "sid-2" {
		t.Errorf("expected [sid-1, sid-2], got %v", members)
	}

	// Nonexistent namespace
	members = s.RoomMembers("/nonexistent", "room-x")
	if members != nil {
		t.Error("expected nil for nonexistent namespace")
	}

	// Nonexistent room
	members = s.RoomMembers("/", "nonexistent-room")
	if members != nil {
		t.Error("expected nil for nonexistent room")
	}
}

// --- Integration: concurrent join/leave/disconnect ---

func TestIntegration_ConcurrentJoinLeaveDisconnect(t *testing.T) {
	s := NewServer()
	sendFn, _ := mockSendFunc()
	s.SendTo = sendFn

	const numClients = 50
	const numRooms = 10

	// Connect all clients
	for i := 0; i < numClients; i++ {
		sid := fmt.Sprintf("sid-%d", i)
		connectSocket(t, s, sid)
	}

	var wg sync.WaitGroup

	// Concurrent joins
	for i := 0; i < numClients; i++ {
		for j := 0; j < numRooms; j++ {
			wg.Add(1)
			go func(clientIdx, roomIdx int) {
				defer wg.Done()
				sid := fmt.Sprintf("sid-%d", clientIdx)
				room := fmt.Sprintf("room-%d", roomIdx)
				s.EnterRoom("/", sid, room)
			}(i, j)
		}
	}
	wg.Wait()

	// Verify all clients in all rooms
	for i := 0; i < numClients; i++ {
		rooms := s.SocketRooms("/", fmt.Sprintf("sid-%d", i))
		if len(rooms) != numRooms {
			t.Errorf("sid-%d expected %d rooms, got %d", i, numRooms, len(rooms))
		}
	}

	// Concurrent leaves (half the rooms)
	for i := 0; i < numClients; i++ {
		for j := 0; j < numRooms/2; j++ {
			wg.Add(1)
			go func(clientIdx, roomIdx int) {
				defer wg.Done()
				sid := fmt.Sprintf("sid-%d", clientIdx)
				room := fmt.Sprintf("room-%d", roomIdx)
				s.LeaveRoom("/", sid, room)
			}(i, j)
		}
	}
	wg.Wait()

	// Verify remaining rooms
	for i := 0; i < numClients; i++ {
		rooms := s.SocketRooms("/", fmt.Sprintf("sid-%d", i))
		if len(rooms) != numRooms/2 {
			t.Errorf("sid-%d expected %d rooms after leave, got %d", i, numRooms/2, len(rooms))
		}
	}

	// Concurrent disconnects (half the clients)
	for i := 0; i < numClients/2; i++ {
		wg.Add(1)
		go func(clientIdx int) {
			defer wg.Done()
			sid := fmt.Sprintf("sid-%d", clientIdx)
			s.DisconnectAll(sid, "transport close")
		}(i)
	}
	wg.Wait()

	// Verify disconnected clients have no rooms
	for i := 0; i < numClients/2; i++ {
		rooms := s.SocketRooms("/", fmt.Sprintf("sid-%d", i))
		if rooms != nil {
			t.Errorf("sid-%d should have no rooms after disconnect, got %v", i, rooms)
		}
	}

	// Verify remaining clients still have rooms
	for i := numClients / 2; i < numClients; i++ {
		rooms := s.SocketRooms("/", fmt.Sprintf("sid-%d", i))
		if len(rooms) != numRooms/2 {
			t.Errorf("sid-%d expected %d rooms, got %d", i, numRooms/2, len(rooms))
		}
	}
}

// --- Integration: full chat join/leave/disconnect lifecycle ---

func TestIntegration_ChatLifecycle(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	const chatRoom = "chat:global"

	// Register handlers matching Python chat_events.py pattern
	s.On("chat:join", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		s.EnterRoom("/", sid, chatRoom)
		count := s.GetNamespace("/").Rooms.RoomCount(chatRoom)

		// Broadcast presence to room
		s.BroadcastToRoom("/", chatRoom, "chat:presence", map[string]interface{}{
			"count": count,
		})

		return []interface{}{map[string]interface{}{
			"status":   "ok",
			"presence": count,
		}}, nil
	})

	s.On("chat:send", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		var data struct {
			Text string `json:"text"`
		}
		if len(args) > 0 {
			json.Unmarshal(args[0], &data)
		}

		msg := map[string]interface{}{
			"id":   "msg-001",
			"text": data.Text,
			"from": sid,
		}
		s.BroadcastToRoom("/", chatRoom, "chat:message", msg)
		return []interface{}{map[string]interface{}{"status": "ok"}}, nil
	})

	s.On("chat:leave", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		s.LeaveRoom("/", sid, chatRoom)
		count := s.GetNamespace("/").Rooms.RoomCount(chatRoom)
		s.BroadcastToRoom("/", chatRoom, "chat:presence", map[string]interface{}{
			"count": count,
		})
		return []interface{}{map[string]interface{}{"status": "ok"}}, nil
	})

	// Connect 3 clients
	connectSocket(t, s, "alice")
	connectSocket(t, s, "bob")
	connectSocket(t, s, "charlie")

	// Alice and Bob join chat
	s.DispatchPacket("alice", &Packet{
		Type: PacketEvent, Namespace: "/", ID: 1,
		Data: json.RawMessage(`["chat:join"]`),
	})
	s.DispatchPacket("bob", &Packet{
		Type: PacketEvent, Namespace: "/", ID: 2,
		Data: json.RawMessage(`["chat:join"]`),
	})

	// Clear sent messages to start fresh
	*sent = make(map[string][]string)

	// Alice sends a message
	s.DispatchPacket("alice", &Packet{
		Type: PacketEvent, Namespace: "/", ID: 3,
		Data: json.RawMessage(`["chat:send",{"text":"hello everyone!"}]`),
	})

	// Both alice and bob should receive the message (they're in the room)
	if len((*sent)["alice"]) != 1 {
		t.Errorf("alice should have received 1 message, got %d", len((*sent)["alice"]))
	}
	if len((*sent)["bob"]) != 1 {
		t.Errorf("bob should have received 1 message, got %d", len((*sent)["bob"]))
	}
	// Charlie is NOT in the chat room
	if len((*sent)["charlie"]) != 0 {
		t.Errorf("charlie should have received 0 messages, got %d", len((*sent)["charlie"]))
	}

	// Bob leaves chat
	*sent = make(map[string][]string)
	s.DispatchPacket("bob", &Packet{
		Type: PacketEvent, Namespace: "/", ID: 4,
		Data: json.RawMessage(`["chat:leave"]`),
	})

	// Presence update should only reach alice (bob already left)
	if len((*sent)["alice"]) != 1 {
		t.Errorf("alice should receive presence update, got %d messages", len((*sent)["alice"]))
	}
	if len((*sent)["bob"]) != 0 {
		t.Errorf("bob should NOT receive presence update after leaving, got %d", len((*sent)["bob"]))
	}

	// Alice disconnects (transport close)
	*sent = make(map[string][]string)
	s.DisconnectAll("alice", "transport close")

	// Verify alice is cleaned up from the room
	if s.GetNamespace("/").Rooms.InRoom("alice", chatRoom) {
		t.Error("alice should be removed from chat room after disconnect")
	}
	if s.GetNamespace("/").Rooms.RoomCount(chatRoom) != 0 {
		t.Errorf("chat room should be empty, got %d members",
			s.GetNamespace("/").Rooms.RoomCount(chatRoom))
	}
}

// --- Integration: SendToSocket for direct messages ---

func TestIntegration_SendToSocket(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	connectSocket(t, s, "sid-1")
	connectSocket(t, s, "sid-2")

	// Send to specific socket
	err := s.SendToSocket("/", "sid-1", "private:msg", map[string]interface{}{
		"text": "hello sid-1",
	})
	if err != nil {
		t.Fatalf("SendToSocket error: %v", err)
	}

	if len((*sent)["sid-1"]) != 1 {
		t.Error("sid-1 should have received the direct message")
	}
	if len((*sent)["sid-2"]) != 0 {
		t.Error("sid-2 should NOT have received the direct message")
	}
}

// --- Integration: BroadcastToNamespace reaches all connected sockets ---

func TestIntegration_BroadcastToNamespace(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	connectSocket(t, s, "sid-1")
	connectSocket(t, s, "sid-2")
	connectSocket(t, s, "sid-3")

	n, err := s.BroadcastToNamespace("/", "global:update", map[string]interface{}{
		"type": "status",
	})
	if err != nil {
		t.Fatalf("broadcast error: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3 recipients, got %d", n)
	}

	for _, sid := range []string{"sid-1", "sid-2", "sid-3"} {
		if len((*sent)[sid]) != 1 {
			t.Errorf("%s should have received 1 message, got %d", sid, len((*sent)[sid]))
		}
	}
}

// --- Integration: idempotent join ---

func TestIntegration_IdempotentJoin(t *testing.T) {
	s := NewServer()
	connectSocket(t, s, "sid-1")

	// Join same room multiple times
	s.EnterRoom("/", "sid-1", "room-a")
	s.EnterRoom("/", "sid-1", "room-a")
	s.EnterRoom("/", "sid-1", "room-a")

	// Should still only be in room once
	count := s.GetNamespace("/").Rooms.RoomCount("room-a")
	if count != 1 {
		t.Errorf("expected room count 1 (idempotent), got %d", count)
	}

	rooms := s.SocketRooms("/", "sid-1")
	if len(rooms) != 1 {
		t.Errorf("expected 1 room, got %d", len(rooms))
	}
}

// --- Integration: LeaveRoom idempotent ---

func TestIntegration_IdempotentLeave(t *testing.T) {
	s := NewServer()
	connectSocket(t, s, "sid-1")

	s.EnterRoom("/", "sid-1", "room-a")
	s.LeaveRoom("/", "sid-1", "room-a")
	// Leave again — should be no-op, no panic
	s.LeaveRoom("/", "sid-1", "room-a")

	rooms := s.SocketRooms("/", "sid-1")
	if rooms != nil {
		t.Errorf("expected nil rooms, got %v", rooms)
	}
}
