package socketio

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// socket.io-client v4.8.3 ACK callback compatibility tests
//
// Verifies all ACK (acknowledgement) patterns used by socket.io-client v4.8.3:
//
// Section 1: Client→Server ACK — client emits with callback, server responds
// Section 2: Server→Client ACK — server emits with callback, client responds
// Section 3: Timeout handling — ACK not received within deadline
// Section 4: Edge cases — concurrent ACKs, large IDs, error responses
//
// These tests ensure the Go server produces identical ACK behavior to
// python-socketio, maintaining wire-format compatibility with the React client.
// =============================================================================

// ─── Section 1: Client→Server ACK ──────────────────────────────────────────

func TestAckCompat_ClientToServer_BasicAck(t *testing.T) {
	// socket.io-client: socket.emit('chat:join', {}, (response) => { ... })
	// Wire sent by client: 21["chat:join",{}]   (type=2, ack_id=1)
	// Wire sent by server: 31[{"status":"ok"}]   (type=3, ack_id=1)
	s := NewServer()
	sendFn, _ := mockSendFunc()
	s.SendTo = sendFn

	s.On("chat:join", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{map[string]interface{}{
			"status": "ok",
		}}, nil
	})

	connectSocket(t, s, "client-1")

	// Client sends EVENT with ack ID
	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        1,
		Data:      json.RawMessage(`["chat:join",{}]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	// Verify handler returned ack data
	if len(ackData) != 1 {
		t.Fatalf("expected 1 ack element, got %d", len(ackData))
	}

	// Build and encode the ACK response packet
	ackPkt, err := BuildAckPacket(pkt, ackData)
	if err != nil {
		t.Fatalf("BuildAckPacket error: %v", err)
	}
	if ackPkt.Type != PacketAck {
		t.Errorf("expected ACK type (3), got %d", ackPkt.Type)
	}
	if ackPkt.ID != 1 {
		t.Errorf("expected ack id 1, got %d", ackPkt.ID)
	}
	if ackPkt.Namespace != "/" {
		t.Errorf("expected namespace '/', got %q", ackPkt.Namespace)
	}

	// Verify wire format matches what socket.io-client v4 expects
	encoded, _ := Encode(ackPkt)
	if !strings.HasPrefix(encoded, "31[") {
		t.Errorf("expected wire '31[...', got %q", encoded)
	}

	// Decode and verify round-trip
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if decoded.Type != PacketAck {
		t.Errorf("decoded type: expected 3, got %d", decoded.Type)
	}
	if decoded.ID != 1 {
		t.Errorf("decoded ack id: expected 1, got %d", decoded.ID)
	}
	ackDataParsed, _ := decoded.AckData()
	if len(ackDataParsed) != 1 {
		t.Fatalf("expected 1 ack data element, got %d", len(ackDataParsed))
	}
}

func TestAckCompat_ClientToServer_MultipleReturnValues(t *testing.T) {
	// Handler returns multiple values in ACK array
	// socket.io-client: socket.emit('multi', data, (val1, val2, val3) => { ... })
	// Wire ACK: 31["ok",42,true]
	s := NewServer()

	s.On("multi", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{"ok", 42, true}, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        1,
		Data:      json.RawMessage(`["multi","data"]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(ackData) != 3 {
		t.Fatalf("expected 3 ack elements, got %d", len(ackData))
	}
	if ackData[0] != "ok" {
		t.Errorf("expected 'ok', got %v", ackData[0])
	}
	if ackData[1] != 42 {
		t.Errorf("expected 42, got %v", ackData[1])
	}
	if ackData[2] != true {
		t.Errorf("expected true, got %v", ackData[2])
	}

	// Verify wire encoding
	ackPkt, _ := BuildAckPacket(pkt, ackData)
	encoded, _ := Encode(ackPkt)
	if !strings.HasPrefix(encoded, "31") {
		t.Errorf("expected '31...', got %q", encoded)
	}

	// Verify the data array in wire format
	decoded, _ := Decode(encoded)
	ackArr, _ := decoded.AckData()
	if len(ackArr) != 3 {
		t.Fatalf("expected 3 elements in decoded ack, got %d", len(ackArr))
	}
}

func TestAckCompat_ClientToServer_EmptyAck(t *testing.T) {
	// Handler returns empty ack (callback with no arguments)
	// socket.io-client: socket.emit('ping', (ack) => { ... })
	// When handler returns []interface{}{}, NewAckPacket produces no data payload
	// Wire ACK: 31 (no data — same as python-socketio empty ack)
	s := NewServer()

	s.On("ping", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{}, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        1,
		Data:      json.RawMessage(`["ping"]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	// Empty ack array
	if ackData == nil {
		t.Fatal("expected non-nil ack data for empty ack")
	}

	ackPkt, _ := BuildAckPacket(pkt, ackData)
	encoded, _ := Encode(ackPkt)

	// Empty args → no data in wire format (matching python-socketio behavior)
	if encoded != "31" {
		t.Errorf("expected '31', got %q", encoded)
	}
}

func TestAckCompat_ClientToServer_SingleArgAck(t *testing.T) {
	// Handler returns single arg ack — most common pattern
	// socket.io-client: socket.emit('ping', (response) => { ... })
	// Wire ACK: 31["pong"]
	s := NewServer()

	s.On("ping", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{"pong"}, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        1,
		Data:      json.RawMessage(`["ping"]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	ackPkt, _ := BuildAckPacket(pkt, ackData)
	encoded, _ := Encode(ackPkt)

	if encoded != `31["pong"]` {
		t.Errorf("expected '31[\"pong\"]', got %q", encoded)
	}
}

func TestAckCompat_ClientToServer_NilAckNoPacket(t *testing.T) {
	// Handler returns nil — no ACK packet should be generated
	// (event without callback in socket.io-client)
	s := NewServer()

	s.On("fire", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1, // no ack requested
		Data:      json.RawMessage(`["fire",{"lat":37.5}]`),
	}
	ackData, _ := s.DispatchPacket("client-1", pkt)

	ackPkt, _ := BuildAckPacket(pkt, ackData)
	if ackPkt != nil {
		t.Error("expected nil ack packet for event without ack id")
	}
}

func TestAckCompat_ClientToServer_ErrorInHandler(t *testing.T) {
	// When handler returns error, ackData should be nil
	// socket.io-client with emitWithAck would see this as no callback invoked
	s := NewServer()

	s.On("fail", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, fmt.Errorf("something went wrong")
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        1,
		Data:      json.RawMessage(`["fail"]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err == nil {
		t.Fatal("expected error from handler")
	}

	// No ack data when handler errors
	if ackData != nil {
		t.Errorf("expected nil ack data on error, got %v", ackData)
	}
}

func TestAckCompat_ClientToServer_NamespaceAck(t *testing.T) {
	// ACK on custom namespace
	// Wire: 2/chat,1["msg","hello"]
	// ACK:  3/chat,1[{"status":"delivered"}]
	s := NewServer()

	chatNS := s.Of("/chat")
	chatNS.On("msg", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{map[string]interface{}{
			"status": "delivered",
		}}, nil
	})

	connectSocket(t, s, "client-1")
	connectSocketNS(t, s, "client-1", "/chat")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/chat",
		ID:        1,
		Data:      json.RawMessage(`["msg","hello"]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	ackPkt, _ := BuildAckPacket(pkt, ackData)
	encoded, _ := Encode(ackPkt)

	// Must include namespace in ACK
	if !strings.HasPrefix(encoded, "3/chat,1") {
		t.Errorf("expected '3/chat,1...', got %q", encoded)
	}

	// Verify round-trip decode
	decoded, _ := Decode(encoded)
	if decoded.Namespace != "/chat" {
		t.Errorf("expected namespace '/chat', got %q", decoded.Namespace)
	}
	if decoded.ID != 1 {
		t.Errorf("expected ack id 1, got %d", decoded.ID)
	}
}

func TestAckCompat_ClientToServer_SequentialAckIDs(t *testing.T) {
	// socket.io-client v4.8.3 uses monotonically increasing ack IDs
	// Verify server handles sequential IDs correctly
	s := NewServer()

	callCount := 0
	s.On("test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		callCount++
		return []interface{}{callCount}, nil
	})

	connectSocket(t, s, "client-1")

	for i := 0; i < 10; i++ {
		pkt := &Packet{
			Type:      PacketEvent,
			Namespace: "/",
			ID:        i,
			Data:      json.RawMessage(`["test"]`),
		}
		ackData, err := s.DispatchPacket("client-1", pkt)
		if err != nil {
			t.Fatalf("dispatch error at id %d: %v", i, err)
		}

		ackPkt, _ := BuildAckPacket(pkt, ackData)
		if ackPkt.ID != i {
			t.Errorf("expected ack id %d, got %d", i, ackPkt.ID)
		}

		encoded, _ := Encode(ackPkt)
		expectedPrefix := fmt.Sprintf("3%d[", i)
		if !strings.HasPrefix(encoded, expectedPrefix) {
			t.Errorf("expected prefix %q, got %q", expectedPrefix, encoded)
		}
	}
}

func TestAckCompat_ClientToServer_AckIDZero(t *testing.T) {
	// socket.io-client starts ack IDs from 0
	// Wire: 20["test"]   (ack_id=0)
	// ACK:  30["ok"]
	s := NewServer()

	s.On("test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{"ok"}, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        0, // ack id = 0 (valid!)
		Data:      json.RawMessage(`["test"]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	ackPkt, _ := BuildAckPacket(pkt, ackData)
	if ackPkt == nil {
		t.Fatal("expected non-nil ack packet for ack_id=0")
	}
	if ackPkt.ID != 0 {
		t.Errorf("expected ack id 0, got %d", ackPkt.ID)
	}

	encoded, _ := Encode(ackPkt)
	// Must be "30[\"ok\"]" — ack type 3, id 0
	if encoded != `30["ok"]` {
		t.Errorf("expected '30[\"ok\"]', got %q", encoded)
	}
}

func TestAckCompat_ClientToServer_ComplexNestedAck(t *testing.T) {
	// Verify complex nested objects in ACK response
	// Matches fire:ignite response pattern from Python server
	s := NewServer()

	s.On("fire:ignite", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{map[string]interface{}{
			"status":   "ok",
			"grid_id":  "grid-123",
			"event_id": "evt-456",
			"stage_info": map[string]interface{}{
				"name":     "growing",
				"duration": 300,
			},
			"spread_path": []interface{}{
				map[string]interface{}{"from": "grid-123", "to": "grid-124"},
				map[string]interface{}{"from": "grid-123", "to": "grid-122"},
			},
		}}, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        1,
		Data:      json.RawMessage(`["fire:ignite",{"lat":37.5,"lng":127.0}]`),
	}
	ackData, _ := s.DispatchPacket("client-1", pkt)

	// Build ACK, encode, decode, verify round-trip fidelity
	ackPkt, _ := BuildAckPacket(pkt, ackData)
	encoded, _ := Encode(ackPkt)
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("round-trip decode error: %v", err)
	}

	// Parse the ACK data from the decoded packet
	ackArr, _ := decoded.AckData()
	if len(ackArr) != 1 {
		t.Fatalf("expected 1 ack element, got %d", len(ackArr))
	}

	var result map[string]interface{}
	json.Unmarshal(ackArr[0], &result)

	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", result["status"])
	}
	if result["grid_id"] != "grid-123" {
		t.Errorf("expected grid_id 'grid-123', got %v", result["grid_id"])
	}

	// Verify nested object survived round-trip
	stageInfo, ok := result["stage_info"].(map[string]interface{})
	if !ok {
		t.Fatal("expected stage_info to be object")
	}
	if stageInfo["name"] != "growing" {
		t.Errorf("expected stage name 'growing', got %v", stageInfo["name"])
	}

	// Verify nested array survived round-trip
	spreadPath, ok := result["spread_path"].([]interface{})
	if !ok {
		t.Fatal("expected spread_path to be array")
	}
	if len(spreadPath) != 2 {
		t.Errorf("expected 2 spread paths, got %d", len(spreadPath))
	}
}

// ─── Section 2: Server→Client ACK ──────────────────────────────────────────

func TestAckCompat_ServerToClient_BasicAck(t *testing.T) {
	// Server emits event with ack callback, client responds with ACK packet
	// Server sends: 21["request_status"]   (EVENT with ack_id=1)
	// Client sends: 31[{"online":true}]    (ACK with ack_id=1)
	registry := NewAckRegistry()

	var receivedArgs []json.RawMessage
	var receivedErr error
	done := make(chan struct{})

	ackID := registry.NextID()
	registry.Register("client-1", ackID, 0, func(args []json.RawMessage, err error) {
		receivedArgs = args
		receivedErr = err
		close(done)
	})

	// Verify the event packet that would be sent to client
	pkt, _ := NewEventPacket("/", "request_status", ackID)
	encoded, _ := Encode(pkt)
	expectedPrefix := fmt.Sprintf("2%d", ackID)
	if !strings.HasPrefix(encoded, expectedPrefix) {
		t.Errorf("expected prefix %q, got %q", expectedPrefix, encoded)
	}

	// Simulate client ACK response
	ackData := json.RawMessage(`[{"online":true}]`)
	resolved := registry.Resolve("client-1", ackID, ackData)
	if !resolved {
		t.Fatal("expected Resolve to return true")
	}

	<-done

	if receivedErr != nil {
		t.Fatalf("expected nil error, got %v", receivedErr)
	}
	if len(receivedArgs) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(receivedArgs))
	}

	var result map[string]interface{}
	json.Unmarshal(receivedArgs[0], &result)
	if result["online"] != true {
		t.Errorf("expected online=true, got %v", result["online"])
	}

	// Verify pending count is 0 after resolution
	if registry.PendingCount("client-1") != 0 {
		t.Errorf("expected 0 pending, got %d", registry.PendingCount("client-1"))
	}
}

func TestAckCompat_ServerToClient_MultipleArgs(t *testing.T) {
	// Server sends event with ack, client responds with multiple args
	// Client ACK: 31["ok",42,true]
	registry := NewAckRegistry()

	var receivedArgs []json.RawMessage
	done := make(chan struct{})

	ackID := registry.NextID()
	registry.Register("client-1", ackID, 0, func(args []json.RawMessage, err error) {
		receivedArgs = args
		close(done)
	})

	registry.Resolve("client-1", ackID, json.RawMessage(`["ok",42,true]`))
	<-done

	if len(receivedArgs) != 3 {
		t.Fatalf("expected 3 args, got %d", len(receivedArgs))
	}

	var str string
	json.Unmarshal(receivedArgs[0], &str)
	if str != "ok" {
		t.Errorf("expected 'ok', got %q", str)
	}

	var num float64
	json.Unmarshal(receivedArgs[1], &num)
	if num != 42 {
		t.Errorf("expected 42, got %v", num)
	}
}

func TestAckCompat_ServerToClient_EmptyAckResponse(t *testing.T) {
	// Client responds with empty ACK (callback with no data)
	// Client ACK: 31[]
	registry := NewAckRegistry()

	var receivedArgs []json.RawMessage
	done := make(chan struct{})

	ackID := registry.NextID()
	registry.Register("client-1", ackID, 0, func(args []json.RawMessage, err error) {
		receivedArgs = args
		close(done)
	})

	registry.Resolve("client-1", ackID, json.RawMessage(`[]`))
	<-done

	if len(receivedArgs) != 0 {
		t.Errorf("expected 0 args for empty ack, got %d", len(receivedArgs))
	}
}

func TestAckCompat_ServerToClient_NamespaceAck(t *testing.T) {
	// ACK on custom namespace
	// Server sends: 2/chat,5["request"]
	// Client sends: 3/chat,5[{"status":"ok"}]
	registry := NewAckRegistry()

	var receivedArgs []json.RawMessage
	done := make(chan struct{})

	ackID := 5
	registry.Register("client-1", ackID, 0, func(args []json.RawMessage, err error) {
		receivedArgs = args
		close(done)
	})

	// Verify event packet encoding with namespace
	pkt, _ := NewEventPacket("/chat", "request", ackID)
	encoded, _ := Encode(pkt)
	if !strings.HasPrefix(encoded, "2/chat,5") {
		t.Errorf("expected '2/chat,5...', got %q", encoded)
	}

	// Simulate client ACK from /chat namespace
	// The ACK packet wire: 3/chat,5[{"status":"ok"}]
	ackWire := `3/chat,5[{"status":"ok"}]`
	ackPkt, err := Decode(ackWire)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if ackPkt.Type != PacketAck {
		t.Errorf("expected ACK type, got %d", ackPkt.Type)
	}
	if ackPkt.Namespace != "/chat" {
		t.Errorf("expected namespace '/chat', got %q", ackPkt.Namespace)
	}
	if ackPkt.ID != 5 {
		t.Errorf("expected ack id 5, got %d", ackPkt.ID)
	}

	registry.Resolve("client-1", ackPkt.ID, ackPkt.Data)
	<-done

	if len(receivedArgs) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(receivedArgs))
	}
}

func TestAckCompat_ServerToClient_ResolveUnknownAck(t *testing.T) {
	// Resolving an unknown ack ID should return false
	registry := NewAckRegistry()

	resolved := registry.Resolve("client-1", 999, json.RawMessage(`["ok"]`))
	if resolved {
		t.Error("expected false for unknown ack")
	}
}

func TestAckCompat_ServerToClient_ResolveAfterCancel(t *testing.T) {
	// Cancelled ack should not be resolvable
	registry := NewAckRegistry()

	called := false
	ackID := registry.NextID()
	cancel := registry.Register("client-1", ackID, 0, func(args []json.RawMessage, err error) {
		called = true
	})

	cancel()

	resolved := registry.Resolve("client-1", ackID, json.RawMessage(`["ok"]`))
	if resolved {
		t.Error("expected false after cancel")
	}
	if called {
		t.Error("callback should not be called after cancel")
	}
}

func TestAckCompat_ServerToClient_DoubleResolve(t *testing.T) {
	// Double resolution should only invoke callback once
	registry := NewAckRegistry()

	callCount := 0
	done := make(chan struct{})

	ackID := registry.NextID()
	registry.Register("client-1", ackID, 0, func(args []json.RawMessage, err error) {
		callCount++
		close(done)
	})

	// First resolve
	ok1 := registry.Resolve("client-1", ackID, json.RawMessage(`["ok"]`))
	<-done

	// Second resolve (should fail)
	ok2 := registry.Resolve("client-1", ackID, json.RawMessage(`["ok"]`))

	if !ok1 {
		t.Error("first resolve should return true")
	}
	if ok2 {
		t.Error("second resolve should return false")
	}
	if callCount != 1 {
		t.Errorf("expected callback called once, got %d", callCount)
	}
}

// ─── Section 3: Timeout handling ────────────────────────────────────────────

func TestAckCompat_Timeout_Basic(t *testing.T) {
	// Server emits with ack, client doesn't respond → timeout
	// socket.io-client v4.8.3 emitWithAck has timeout option
	registry := NewAckRegistry()

	var receivedErr error
	done := make(chan struct{})

	ackID := registry.NextID()
	registry.Register("client-1", ackID, 50*time.Millisecond, func(args []json.RawMessage, err error) {
		receivedErr = err
		close(done)
	})

	// Wait for timeout
	select {
	case <-done:
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout callback was not invoked")
	}

	if receivedErr != ErrAckTimeout {
		t.Errorf("expected ErrAckTimeout, got %v", receivedErr)
	}

	// Pending count should be 0 after timeout
	if registry.PendingCount("client-1") != 0 {
		t.Errorf("expected 0 pending after timeout, got %d", registry.PendingCount("client-1"))
	}
}

func TestAckCompat_Timeout_ResolveBeforeTimeout(t *testing.T) {
	// ACK arrives before timeout — callback gets data, not error
	registry := NewAckRegistry()

	var receivedArgs []json.RawMessage
	var receivedErr error
	done := make(chan struct{})

	ackID := registry.NextID()
	registry.Register("client-1", ackID, 500*time.Millisecond, func(args []json.RawMessage, err error) {
		receivedArgs = args
		receivedErr = err
		close(done)
	})

	// Resolve immediately (well before 500ms timeout)
	registry.Resolve("client-1", ackID, json.RawMessage(`[{"status":"ok"}]`))

	select {
	case <-done:
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("callback was not invoked")
	}

	if receivedErr != nil {
		t.Errorf("expected nil error, got %v", receivedErr)
	}
	if len(receivedArgs) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(receivedArgs))
	}

	// Wait a bit to ensure the timeout timer was actually stopped
	time.Sleep(100 * time.Millisecond)
}

func TestAckCompat_Timeout_ResolveAfterTimeoutIgnored(t *testing.T) {
	// ACK arrives after timeout — should be ignored (resolve returns false)
	registry := NewAckRegistry()

	callCount := 0
	done := make(chan struct{})

	ackID := registry.NextID()
	registry.Register("client-1", ackID, 30*time.Millisecond, func(args []json.RawMessage, err error) {
		callCount++
		if callCount == 1 {
			close(done)
		}
	})

	// Wait for timeout
	<-done

	// Try to resolve after timeout
	time.Sleep(20 * time.Millisecond) // extra safety margin
	resolved := registry.Resolve("client-1", ackID, json.RawMessage(`["late"]`))
	if resolved {
		t.Error("resolve after timeout should return false")
	}

	// Callback should only have been called once (for timeout)
	time.Sleep(20 * time.Millisecond)
	if callCount != 1 {
		t.Errorf("expected callback called once (timeout), got %d", callCount)
	}
}

func TestAckCompat_Timeout_CancelStopsTimeout(t *testing.T) {
	// Cancel should prevent timeout callback from firing
	registry := NewAckRegistry()

	called := false
	ackID := registry.NextID()
	cancel := registry.Register("client-1", ackID, 50*time.Millisecond, func(args []json.RawMessage, err error) {
		called = true
	})

	// Cancel immediately
	cancel()

	// Wait longer than timeout
	time.Sleep(100 * time.Millisecond)

	if called {
		t.Error("callback should not fire after cancel")
	}
}

func TestAckCompat_Timeout_MultipleAcksIndependent(t *testing.T) {
	// Multiple pending ACKs with different timeouts
	registry := NewAckRegistry()

	results := make(map[int]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Register 3 acks: first times out, second resolves, third times out
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		id := i
		timeout := time.Duration(50+id*10) * time.Millisecond
		registry.Register("client-1", id, timeout, func(args []json.RawMessage, err error) {
			mu.Lock()
			results[id] = err
			mu.Unlock()
			wg.Done()
		})
	}

	// Resolve only ack #2
	registry.Resolve("client-1", 2, json.RawMessage(`["ok"]`))

	// Wait for all callbacks (resolved + 2 timeouts)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("not all callbacks completed")
	}

	mu.Lock()
	defer mu.Unlock()

	// Ack #1 should have timed out
	if results[1] != ErrAckTimeout {
		t.Errorf("ack #1: expected timeout, got %v", results[1])
	}
	// Ack #2 should have resolved successfully
	if results[2] != nil {
		t.Errorf("ack #2: expected nil error, got %v", results[2])
	}
	// Ack #3 should have timed out
	if results[3] != ErrAckTimeout {
		t.Errorf("ack #3: expected timeout, got %v", results[3])
	}
}

// ─── Section 4: Edge cases ──────────────────────────────────────────────────

func TestAckCompat_ConcurrentAckRegistration(t *testing.T) {
	// Verify thread safety with concurrent ack registrations and resolutions
	registry := NewAckRegistry()

	const numAcks = 100
	var wg sync.WaitGroup
	resolved := make([]bool, numAcks)
	var mu sync.Mutex

	// Register all acks
	for i := 0; i < numAcks; i++ {
		wg.Add(1)
		id := i
		registry.Register("client-1", id, 5*time.Second, func(args []json.RawMessage, err error) {
			mu.Lock()
			resolved[id] = (err == nil)
			mu.Unlock()
			wg.Done()
		})
	}

	// Resolve all concurrently
	for i := 0; i < numAcks; i++ {
		go func(id int) {
			registry.Resolve("client-1", id, json.RawMessage(`["ok"]`))
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("not all concurrent acks resolved")
	}

	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < numAcks; i++ {
		if !resolved[i] {
			t.Errorf("ack %d was not resolved successfully", i)
		}
	}
}

func TestAckCompat_MultipleSocketsIsolated(t *testing.T) {
	// ACKs for different sockets should be isolated
	registry := NewAckRegistry()

	var client1Got, client2Got bool
	var wg sync.WaitGroup
	wg.Add(2)

	registry.Register("client-1", 1, 0, func(args []json.RawMessage, err error) {
		client1Got = true
		wg.Done()
	})
	registry.Register("client-2", 1, 0, func(args []json.RawMessage, err error) {
		client2Got = true
		wg.Done()
	})

	// Both use ack id 1 but for different sockets
	if registry.PendingCount("client-1") != 1 {
		t.Errorf("client-1 should have 1 pending, got %d", registry.PendingCount("client-1"))
	}
	if registry.PendingCount("client-2") != 1 {
		t.Errorf("client-2 should have 1 pending, got %d", registry.PendingCount("client-2"))
	}

	// Resolve only client-1's ack
	registry.Resolve("client-1", 1, json.RawMessage(`["data1"]`))
	registry.Resolve("client-2", 1, json.RawMessage(`["data2"]`))

	wg.Wait()

	if !client1Got {
		t.Error("client-1 callback should have been called")
	}
	if !client2Got {
		t.Error("client-2 callback should have been called")
	}
}

func TestAckCompat_RemoveAllOnDisconnect(t *testing.T) {
	// When a client disconnects, all pending ACKs should be cleaned up
	registry := NewAckRegistry()

	callCount := 0
	for i := 0; i < 5; i++ {
		registry.Register("client-1", i, 5*time.Second, func(args []json.RawMessage, err error) {
			callCount++
		})
	}

	if registry.PendingCount("client-1") != 5 {
		t.Errorf("expected 5 pending, got %d", registry.PendingCount("client-1"))
	}

	// Simulate disconnect
	registry.RemoveAll("client-1")

	if registry.PendingCount("client-1") != 0 {
		t.Errorf("expected 0 pending after RemoveAll, got %d", registry.PendingCount("client-1"))
	}

	// Callbacks should not have been invoked
	time.Sleep(50 * time.Millisecond)
	if callCount != 0 {
		t.Errorf("expected 0 callback invocations after RemoveAll, got %d", callCount)
	}
}

func TestAckCompat_NextIDMonotonic(t *testing.T) {
	// Verify NextID returns monotonically increasing IDs
	registry := NewAckRegistry()

	ids := make([]int, 100)
	for i := range ids {
		ids[i] = registry.NextID()
	}

	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("NextID not monotonic: ids[%d]=%d <= ids[%d]=%d",
				i, ids[i], i-1, ids[i-1])
		}
	}
}

func TestAckCompat_AckWireFormatDecodeResolve(t *testing.T) {
	// End-to-end: decode a raw ACK wire packet and resolve it in the registry
	// This simulates the actual flow when Engine.IO relays a message
	registry := NewAckRegistry()

	var receivedArgs []json.RawMessage
	done := make(chan struct{})

	registry.Register("client-1", 7, 0, func(args []json.RawMessage, err error) {
		receivedArgs = args
		close(done)
	})

	// Raw ACK wire as received from Engine.IO
	rawAck := `37[{"success":true,"count":42}]`
	pkt, err := Decode(rawAck)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if pkt.Type != PacketAck {
		t.Errorf("expected ACK type, got %d", pkt.Type)
	}
	if pkt.ID != 7 {
		t.Errorf("expected ack id 7, got %d", pkt.ID)
	}

	// Dispatch to registry
	resolved := registry.Resolve("client-1", pkt.ID, pkt.Data)
	if !resolved {
		t.Fatal("expected resolve to succeed")
	}

	<-done

	if len(receivedArgs) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(receivedArgs))
	}

	var result map[string]interface{}
	json.Unmarshal(receivedArgs[0], &result)
	if result["success"] != true {
		t.Errorf("expected success=true, got %v", result["success"])
	}
	if result["count"].(float64) != 42 {
		t.Errorf("expected count=42, got %v", result["count"])
	}
}

func TestAckCompat_AckWireFormat_NamespaceDecodeResolve(t *testing.T) {
	// Decode a namespace-scoped ACK wire packet and resolve
	registry := NewAckRegistry()

	var receivedArgs []json.RawMessage
	done := make(chan struct{})

	registry.Register("client-1", 3, 0, func(args []json.RawMessage, err error) {
		receivedArgs = args
		close(done)
	})

	// Namespace ACK wire
	rawAck := `3/chat,3[{"delivered":true}]`
	pkt, err := Decode(rawAck)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if pkt.Namespace != "/chat" {
		t.Errorf("expected namespace '/chat', got %q", pkt.Namespace)
	}

	registry.Resolve("client-1", pkt.ID, pkt.Data)
	<-done

	if len(receivedArgs) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(receivedArgs))
	}
}

func TestAckCompat_BinaryAck_ClientToServer(t *testing.T) {
	// Client sends binary event with ACK, server responds with binary ACK
	// Wire: 51-7["upload",{"_placeholder":true,"num":0}]
	// ACK:  61-7[{"_placeholder":true,"num":0}]
	s := NewServer()

	s.On("upload", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{map[string]interface{}{
			"status":   "ok",
			"checksum": "abc123",
		}}, nil
	})

	connectSocket(t, s, "client-1")

	// Decode binary event with ack
	pkt, _ := Decode(`51-7["upload",{"_placeholder":true,"num":0}]`)
	pkt.AddAttachment([]byte("file content"))

	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	// Build regular ACK (non-binary response)
	ackPkt, _ := BuildAckPacket(pkt, ackData)
	if ackPkt.Type != PacketAck {
		t.Errorf("expected ACK type, got %d", ackPkt.Type)
	}
	if ackPkt.ID != 7 {
		t.Errorf("expected ack id 7, got %d", ackPkt.ID)
	}

	encoded, _ := Encode(ackPkt)
	if !strings.HasPrefix(encoded, "37") {
		t.Errorf("expected '37...', got %q", encoded)
	}
}

func TestAckCompat_BinaryAck_ServerToClient(t *testing.T) {
	// Server sends binary ACK response to client
	// ACK with binary: 61-3[{"_placeholder":true,"num":0}]
	binaryData := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	ackPkt, err := NewBinaryAckPacket("/", 3, binaryData)
	if err != nil {
		t.Fatalf("NewBinaryAckPacket error: %v", err)
	}

	if ackPkt.Type != PacketBinaryAck {
		t.Errorf("expected BINARY_ACK (6), got %d", ackPkt.Type)
	}
	if ackPkt.ID != 3 {
		t.Errorf("expected ack id 3, got %d", ackPkt.ID)
	}

	encoded, _ := EncodeBinary(ackPkt)
	if !strings.HasPrefix(encoded.TextFrame, "61-3") {
		t.Errorf("expected '61-3...', got %q", encoded.TextFrame)
	}
	if len(encoded.BinaryFrames) != 1 {
		t.Fatalf("expected 1 binary frame, got %d", len(encoded.BinaryFrames))
	}

	// Verify binary content is preserved
	if len(encoded.BinaryFrames[0]) != 4 {
		t.Errorf("expected 4 bytes, got %d", len(encoded.BinaryFrames[0]))
	}
}

func TestAckCompat_ServerToClient_FullRoundTrip(t *testing.T) {
	// Full round-trip test simulating server→client→server ACK flow
	// 1. Server creates EVENT with ack ID
	// 2. Encodes it for wire transmission
	// 3. Client decodes the EVENT, sends ACK response
	// 4. Server decodes ACK, resolves callback
	registry := NewAckRegistry()

	// Step 1: Server creates event with ack
	ackID := registry.NextID()
	eventPkt, _ := NewEventPacket("/", "get_status", ackID, map[string]interface{}{
		"target": "all",
	})

	// Step 2: Encode for wire
	eventWire, _ := Encode(eventPkt)

	// Verify it has the ack ID
	decoded, _ := Decode(eventWire)
	if decoded.ID != ackID {
		t.Errorf("decoded ack id mismatch: expected %d, got %d", ackID, decoded.ID)
	}

	// Register callback
	var callbackArgs []json.RawMessage
	done := make(chan struct{})
	registry.Register("client-1", ackID, 5*time.Second, func(args []json.RawMessage, err error) {
		callbackArgs = args
		close(done)
	})

	// Step 3: Client builds and sends ACK response
	clientAckPkt, _ := NewAckPacket("/", ackID, map[string]interface{}{
		"status": "healthy",
		"uptime": 3600,
	})
	ackWire, _ := Encode(clientAckPkt)

	// Step 4: Server receives and decodes ACK
	receivedAck, _ := Decode(ackWire)
	if receivedAck.Type != PacketAck {
		t.Errorf("expected ACK type, got %d", receivedAck.Type)
	}

	// Resolve in registry
	registry.Resolve("client-1", receivedAck.ID, receivedAck.Data)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("callback not invoked")
	}

	if len(callbackArgs) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(callbackArgs))
	}

	var result map[string]interface{}
	json.Unmarshal(callbackArgs[0], &result)
	if result["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", result["status"])
	}
	if result["uptime"].(float64) != 3600 {
		t.Errorf("expected uptime 3600, got %v", result["uptime"])
	}
}
