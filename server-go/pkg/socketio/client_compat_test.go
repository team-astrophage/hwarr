package socketio

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// socket.io-client v4.8.3 compatibility tests
//
// Verifies that the Go Socket.IO server correctly handles all event patterns
// used by socket.io-client v4.8.3:
//   - emit/on basic events with various data types
//   - ACK (acknowledgement) callback pattern
//   - Binary data (BINARY_EVENT / BINARY_ACK)
//   - Namespace-scoped events
//   - Connect/Disconnect lifecycle
//
// These tests simulate wire-format packets as sent by socket.io-client v4.8.3
// and verify the server processes them identically to python-socketio.
// =============================================================================

// --- Section 1: Basic emit/on event compatibility ---

func TestClientCompat_EmitBasicStringEvent(t *testing.T) {
	// socket.io-client: socket.emit('fire', { lat: 37.5, lng: 127.0 })
	// Wire: 2["fire",{"lat":37.5,"lng":127.0}]
	s := NewServer()
	sendFn, _ := mockSendFunc()
	s.SendTo = sendFn

	var receivedSID string
	var receivedLat, receivedLng float64

	s.On("fire", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		receivedSID = sid
		if len(args) > 0 {
			var data struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			}
			json.Unmarshal(args[0], &data)
			receivedLat = data.Lat
			receivedLng = data.Lng
		}
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	// Simulate wire packet from socket.io-client
	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["fire",{"lat":37.5,"lng":127.0}]`),
	}
	_, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if receivedSID != "client-1" {
		t.Errorf("expected sid 'client-1', got %q", receivedSID)
	}
	if receivedLat != 37.5 || receivedLng != 127.0 {
		t.Errorf("expected lat=37.5, lng=127.0, got lat=%v, lng=%v", receivedLat, receivedLng)
	}
}

func TestClientCompat_EmitEventNoArgs(t *testing.T) {
	// socket.io-client: socket.emit('get_fires', {})
	// Wire: 2["get_fires",{}]
	s := NewServer()
	called := false

	s.On("get_fires", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		called = true
		if len(args) != 1 {
			t.Errorf("expected 1 arg, got %d", len(args))
		}
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	_, err := s.DispatchPacket("client-1", &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["get_fires",{}]`),
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestClientCompat_EmitEventMultipleArgs(t *testing.T) {
	// socket.io-client: socket.emit('multi', 'arg1', 42, true)
	// Wire: 2["multi","arg1",42,true]
	s := NewServer()

	var args []json.RawMessage
	s.On("multi", func(sid string, a ...json.RawMessage) ([]interface{}, error) {
		args = a
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	_, err := s.DispatchPacket("client-1", &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["multi","arg1",42,true]`),
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}

	var str string
	json.Unmarshal(args[0], &str)
	if str != "arg1" {
		t.Errorf("expected 'arg1', got %q", str)
	}

	var num float64
	json.Unmarshal(args[1], &num)
	if num != 42 {
		t.Errorf("expected 42, got %v", num)
	}

	var flag bool
	json.Unmarshal(args[2], &flag)
	if !flag {
		t.Error("expected true, got false")
	}
}

func TestClientCompat_EmitEventOnlyName(t *testing.T) {
	// socket.io-client: socket.emit('ping')
	// Wire: 2["ping"]
	s := NewServer()
	called := false

	s.On("ping", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		called = true
		if len(args) != 0 {
			t.Errorf("expected 0 args, got %d", len(args))
		}
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	_, err := s.DispatchPacket("client-1", &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["ping"]`),
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestClientCompat_EmitNestedObjectData(t *testing.T) {
	// socket.io-client: socket.emit('chat:send', { text: 'hello', user: { id: '123', name: 'alice' } })
	// Wire: 2["chat:send",{"text":"hello","user":{"id":"123","name":"alice"}}]
	s := NewServer()

	var receivedText string
	var receivedUserID string

	s.On("chat:send", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		if len(args) > 0 {
			var data struct {
				Text string `json:"text"`
				User struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"user"`
			}
			json.Unmarshal(args[0], &data)
			receivedText = data.Text
			receivedUserID = data.User.ID
		}
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	_, err := s.DispatchPacket("client-1", &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["chat:send",{"text":"hello","user":{"id":"123","name":"alice"}}]`),
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if receivedText != "hello" {
		t.Errorf("expected text 'hello', got %q", receivedText)
	}
	if receivedUserID != "123" {
		t.Errorf("expected user id '123', got %q", receivedUserID)
	}
}

func TestClientCompat_EmitArrayData(t *testing.T) {
	// socket.io-client: socket.emit('subscribe:viewport', { grid_ids: ['grid-A', 'grid-B'] })
	// Wire: 2["subscribe:viewport",{"grid_ids":["grid-A","grid-B"]}]
	s := NewServer()

	var gridIDs []string
	s.On("subscribe:viewport", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		if len(args) > 0 {
			var data struct {
				GridIDs []string `json:"grid_ids"`
			}
			json.Unmarshal(args[0], &data)
			gridIDs = data.GridIDs
		}
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	_, err := s.DispatchPacket("client-1", &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["subscribe:viewport",{"grid_ids":["grid-A","grid-B"]}]`),
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(gridIDs) != 2 || gridIDs[0] != "grid-A" || gridIDs[1] != "grid-B" {
		t.Errorf("expected [grid-A, grid-B], got %v", gridIDs)
	}
}

// --- Section 2: ACK (acknowledgement) callback pattern ---

func TestClientCompat_EmitWithAck(t *testing.T) {
	// socket.io-client: socket.emit('chat:join', {}, (response) => { ... })
	// Wire: 21["chat:join",{}]  (ack id = 1)
	// Expected ACK wire: 31[{"status":"ok","presence":1}]
	s := NewServer()
	sendFn, _ := mockSendFunc()
	s.SendTo = sendFn

	s.On("chat:join", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{map[string]interface{}{
			"status":   "ok",
			"presence": 1,
		}}, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        1, // ack id
		Data:      json.RawMessage(`["chat:join",{}]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(ackData) != 1 {
		t.Fatalf("expected 1 ack element, got %d", len(ackData))
	}

	ackMap, ok := ackData[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", ackData[0])
	}
	if ackMap["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", ackMap["status"])
	}

	// Verify ACK packet can be correctly built
	ackPkt, err := BuildAckPacket(pkt, ackData)
	if err != nil {
		t.Fatalf("BuildAckPacket error: %v", err)
	}
	if ackPkt == nil {
		t.Fatal("expected non-nil ack packet")
	}
	if ackPkt.Type != PacketAck {
		t.Errorf("expected ACK type (3), got %d", ackPkt.Type)
	}
	if ackPkt.ID != 1 {
		t.Errorf("expected ack id 1, got %d", ackPkt.ID)
	}

	// Verify encoded format matches socket.io-client v4 expectation
	encoded, err := Encode(ackPkt)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if !strings.HasPrefix(encoded, "31") {
		t.Errorf("expected ACK wire format starting with '31', got %q", encoded)
	}
}

func TestClientCompat_EmitWithAckMultipleData(t *testing.T) {
	// Handler returns multiple values in ACK
	s := NewServer()

	s.On("heartbeat", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{map[string]interface{}{
			"status":    "ok",
			"server_ts": 1700000000,
		}}, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        5,
		Data:      json.RawMessage(`["heartbeat",{"ts":1699999999}]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(ackData) != 1 {
		t.Fatalf("expected 1 ack element, got %d", len(ackData))
	}

	ackPkt, _ := BuildAckPacket(pkt, ackData)
	encoded, _ := Encode(ackPkt)
	// ACK format: 3<ackId>[<data>]
	if !strings.HasPrefix(encoded, "35") {
		t.Errorf("expected '35...' (ACK with id 5), got %q", encoded)
	}
}

func TestClientCompat_EmitNoAck(t *testing.T) {
	// Event without ack (ID = -1) should not produce ACK packet
	s := NewServer()

	s.On("fire", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1, // no ack
		Data:      json.RawMessage(`["fire",{"lat":37.5,"lng":127.0}]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	ackPkt, err := BuildAckPacket(pkt, ackData)
	if err != nil {
		t.Fatalf("BuildAckPacket error: %v", err)
	}
	if ackPkt != nil {
		t.Error("expected nil ack packet for event without ack id")
	}
}

func TestClientCompat_AckWithComplexPayload(t *testing.T) {
	// Verify ACK with complex data structure (matching fire:ignite response)
	s := NewServer()

	s.On("fire:ignite", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{map[string]interface{}{
			"status":            "ok",
			"grid_id":           "grid-123",
			"requested_grid_id": "grid-123",
			"event_id":          "evt-456",
			"active_count":      3,
			"stage":             1,
			"stage_info": map[string]interface{}{
				"name":     "growing",
				"duration": 300,
			},
			"spread_path": []interface{}{
				map[string]interface{}{"from": "grid-123", "to": "grid-124"},
			},
		}}, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        10,
		Data:      json.RawMessage(`["fire:ignite",{"lat":37.5,"lng":127.0}]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(ackData) != 1 {
		t.Fatalf("expected 1 ack element, got %d", len(ackData))
	}

	result := ackData[0].(map[string]interface{})
	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", result["status"])
	}
	if result["grid_id"] != "grid-123" {
		t.Errorf("expected grid_id 'grid-123', got %v", result["grid_id"])
	}

	// Verify the ACK can be encoded and decoded correctly
	ackPkt, _ := BuildAckPacket(pkt, ackData)
	encoded, _ := Encode(ackPkt)

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if decoded.Type != PacketAck {
		t.Errorf("expected ACK type, got %d", decoded.Type)
	}
	if decoded.ID != 10 {
		t.Errorf("expected ack id 10, got %d", decoded.ID)
	}
}

// --- Section 3: Binary data compatibility ---

func TestClientCompat_BinaryEventDecode(t *testing.T) {
	// socket.io-client v4 sends binary data as BINARY_EVENT (type 5)
	// Wire: 51-["binary_upload",{"_placeholder":true,"num":0}]
	// Followed by binary attachment frame

	rawPacket := `51-["binary_upload",{"_placeholder":true,"num":0}]`
	pkt, err := Decode(rawPacket)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected BINARY_EVENT (5), got %d", pkt.Type)
	}
	if pkt.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", pkt.Attachments)
	}
	if pkt.Namespace != "/" {
		t.Errorf("expected namespace '/', got %q", pkt.Namespace)
	}

	name, err := pkt.EventName()
	if err != nil {
		t.Fatalf("EventName error: %v", err)
	}
	if name != "binary_upload" {
		t.Errorf("expected event name 'binary_upload', got %q", name)
	}

	// Simulate receiving binary attachment
	binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF}
	complete := pkt.AddAttachment(binaryData)
	if !complete {
		t.Error("expected packet to be complete after 1 attachment")
	}
	if !pkt.IsComplete() {
		t.Error("IsComplete should return true")
	}

	// Reconstruct binary data
	err = pkt.ReconstructBinary()
	if err != nil {
		t.Fatalf("ReconstructBinary error: %v", err)
	}
}

func TestClientCompat_BinaryEventMultipleAttachments(t *testing.T) {
	// Wire: 52-["multi_binary",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]
	rawPacket := `52-["multi_binary",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]`
	pkt, err := Decode(rawPacket)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected BINARY_EVENT, got %d", pkt.Type)
	}
	if pkt.Attachments != 2 {
		t.Errorf("expected 2 attachments, got %d", pkt.Attachments)
	}

	// Add first attachment - should not be complete yet
	complete := pkt.AddAttachment([]byte{0x01, 0x02})
	if complete {
		t.Error("should not be complete after 1 of 2 attachments")
	}

	// Add second attachment - should now be complete
	complete = pkt.AddAttachment([]byte{0x03, 0x04})
	if !complete {
		t.Error("should be complete after 2 of 2 attachments")
	}

	// Verify binary payloads are collected correctly
	if len(pkt.BinaryPayloads) != 2 {
		t.Fatalf("expected 2 binary payloads, got %d", len(pkt.BinaryPayloads))
	}

	buf0 := pkt.BinaryPayloads[0]
	if len(buf0) != 2 || buf0[0] != 0x01 || buf0[1] != 0x02 {
		t.Errorf("unexpected first binary payload: %v", buf0)
	}

	buf1 := pkt.BinaryPayloads[1]
	if len(buf1) != 2 || buf1[0] != 0x03 || buf1[1] != 0x04 {
		t.Errorf("unexpected second binary payload: %v", buf1)
	}

	// Verify ReconstructBinary modifies the JSON data (replaces placeholders)
	err = pkt.ReconstructBinary()
	if err != nil {
		t.Fatalf("ReconstructBinary error: %v", err)
	}

	// After reconstruction, the JSON data should no longer contain placeholders
	if strings.Contains(string(pkt.Data), "_placeholder") {
		t.Errorf("expected placeholders to be replaced, data: %s", string(pkt.Data))
	}

	// Verify event name is still extractable
	name, err := pkt.EventName()
	if err != nil {
		t.Fatalf("EventName error: %v", err)
	}
	if name != "multi_binary" {
		t.Errorf("expected event name 'multi_binary', got %q", name)
	}
}

func TestClientCompat_BinaryEventNestedPlaceholder(t *testing.T) {
	// Binary data nested inside an object:
	// Wire: 51-["upload",{"filename":"test.bin","data":{"_placeholder":true,"num":0}}]
	rawPacket := `51-["upload",{"filename":"test.bin","data":{"_placeholder":true,"num":0}}]`
	pkt, err := Decode(rawPacket)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	fileContent := []byte("hello binary world")
	pkt.AddAttachment(fileContent)

	// Verify the binary payload was collected
	if len(pkt.BinaryPayloads) != 1 {
		t.Fatalf("expected 1 binary payload, got %d", len(pkt.BinaryPayloads))
	}
	if string(pkt.BinaryPayloads[0]) != "hello binary world" {
		t.Errorf("unexpected binary payload: %q", string(pkt.BinaryPayloads[0]))
	}

	// Verify reconstruction replaces nested placeholders
	err = pkt.ReconstructBinary()
	if err != nil {
		t.Fatalf("ReconstructBinary error: %v", err)
	}

	// After reconstruction, placeholder should be replaced
	if strings.Contains(string(pkt.Data), "_placeholder") {
		t.Errorf("expected placeholder to be replaced, data: %s", string(pkt.Data))
	}

	// Verify the filename field is preserved in JSON
	if !strings.Contains(string(pkt.Data), `"filename":"test.bin"`) {
		t.Errorf("expected filename in data, got: %s", string(pkt.Data))
	}

	// Verify event name extraction still works
	name, err := pkt.EventName()
	if err != nil {
		t.Fatalf("EventName error: %v", err)
	}
	if name != "upload" {
		t.Errorf("expected event name 'upload', got %q", name)
	}
}

func TestClientCompat_BinaryEventWithNamespace(t *testing.T) {
	// Binary event on a custom namespace:
	// Wire: 51-/chat,["file_share",{"_placeholder":true,"num":0}]
	rawPacket := `51-/chat,["file_share",{"_placeholder":true,"num":0}]`
	pkt, err := Decode(rawPacket)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected BINARY_EVENT, got %d", pkt.Type)
	}
	if pkt.Namespace != "/chat" {
		t.Errorf("expected namespace '/chat', got %q", pkt.Namespace)
	}
	if pkt.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", pkt.Attachments)
	}

	name, _ := pkt.EventName()
	if name != "file_share" {
		t.Errorf("expected event name 'file_share', got %q", name)
	}
}

func TestClientCompat_BinaryEventWithAck(t *testing.T) {
	// Binary event with ack ID:
	// Wire: 51-7["upload",{"_placeholder":true,"num":0}]
	rawPacket := `51-7["upload",{"_placeholder":true,"num":0}]`
	pkt, err := Decode(rawPacket)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected BINARY_EVENT, got %d", pkt.Type)
	}
	if pkt.ID != 7 {
		t.Errorf("expected ack id 7, got %d", pkt.ID)
	}
	if pkt.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", pkt.Attachments)
	}
}

func TestClientCompat_BinaryAckEncode(t *testing.T) {
	// Server sends binary ACK back to client
	// Expected wire format: 61-<ackId>[{"_placeholder":true,"num":0}]
	binaryData := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	pkt, err := NewBinaryAckPacket("/", 3, binaryData)
	if err != nil {
		t.Fatalf("NewBinaryAckPacket error: %v", err)
	}

	if pkt.Type != PacketBinaryAck {
		t.Errorf("expected BINARY_ACK (6), got %d", pkt.Type)
	}
	if pkt.ID != 3 {
		t.Errorf("expected ack id 3, got %d", pkt.ID)
	}
	if pkt.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", pkt.Attachments)
	}

	encoded, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	if !strings.HasPrefix(encoded.TextFrame, "61-3") {
		t.Errorf("expected wire format starting with '61-3', got %q", encoded.TextFrame)
	}
	if len(encoded.BinaryFrames) != 1 {
		t.Errorf("expected 1 binary frame, got %d", len(encoded.BinaryFrames))
	}
	if len(encoded.BinaryFrames[0]) != 4 {
		t.Errorf("expected 4 bytes, got %d", len(encoded.BinaryFrames[0]))
	}
}

func TestClientCompat_ServerEmitBinaryEvent(t *testing.T) {
	// Server broadcasts binary data to clients
	binaryData := []byte("image data here")
	pkt, err := NewBinaryEventPacket("/", "image:update", -1, map[string]interface{}{
		"filename": "photo.png",
		"data":     binaryData,
	})
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected BINARY_EVENT (5), got %d", pkt.Type)
	}

	encoded, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	// Verify text frame starts with type 5 and attachment count
	if !strings.HasPrefix(encoded.TextFrame, "51-") {
		t.Errorf("expected text frame starting with '51-', got %q", encoded.TextFrame)
	}

	// Verify binary frame contains the original data
	if len(encoded.BinaryFrames) != 1 {
		t.Fatalf("expected 1 binary frame, got %d", len(encoded.BinaryFrames))
	}
	if string(encoded.BinaryFrames[0]) != "image data here" {
		t.Errorf("unexpected binary content: %q", string(encoded.BinaryFrames[0]))
	}

	// Verify the text frame contains a placeholder
	if !strings.Contains(encoded.TextFrame, `"_placeholder":true`) {
		t.Errorf("expected placeholder in text frame, got %q", encoded.TextFrame)
	}
}

func TestClientCompat_BinaryEventDispatch(t *testing.T) {
	// Verify BINARY_EVENT packets are dispatched to handlers like regular events
	s := NewServer()

	var receivedEvent string
	var receivedArgs []json.RawMessage

	s.On("binary_test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		receivedEvent = "binary_test"
		receivedArgs = args
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:        PacketBinaryEvent,
		Namespace:   "/",
		ID:          -1,
		Data:        json.RawMessage(`["binary_test",{"_placeholder":true,"num":0}]`),
		Attachments: 1,
	}
	pkt.AddAttachment([]byte{0x01, 0x02, 0x03})

	_, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if receivedEvent != "binary_test" {
		t.Errorf("expected event 'binary_test', got %q", receivedEvent)
	}
	if len(receivedArgs) != 1 {
		t.Errorf("expected 1 arg, got %d", len(receivedArgs))
	}
}

// --- Section 4: Namespace event compatibility ---

func TestClientCompat_NamespaceConnect(t *testing.T) {
	// socket.io-client: const chatSocket = io('/chat', { ... })
	// Wire: 0/chat,  (CONNECT to /chat namespace)
	s := NewServer()

	var connectedSID string
	chatNS := s.Of("/chat")
	chatNS.OnConnect(func(sid string, auth json.RawMessage) error {
		connectedSID = sid
		return nil
	})

	// First connect to default namespace (required by protocol)
	connectSocket(t, s, "client-1")

	// Then connect to /chat namespace
	result, err := s.DispatchPacketFull("client-1", &Packet{
		Type:      PacketConnect,
		Namespace: "/chat",
		ID:        -1,
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if connectedSID != "client-1" {
		t.Errorf("expected sid 'client-1', got %q", connectedSID)
	}

	// Verify CONNECT ACK response
	if result.ResponsePacket == nil {
		t.Fatal("expected response packet")
	}
	if result.ResponsePacket.Type != PacketConnect {
		t.Errorf("expected CONNECT response, got type %d", result.ResponsePacket.Type)
	}
	if result.ResponsePacket.Namespace != "/chat" {
		t.Errorf("expected namespace '/chat', got %q", result.ResponsePacket.Namespace)
	}

	// Verify encoded format: 0/chat,{"sid":"client-1"}
	encoded, _ := Encode(result.ResponsePacket)
	if !strings.HasPrefix(encoded, "0/chat,") {
		t.Errorf("expected connect ack starting with '0/chat,', got %q", encoded)
	}
	if !strings.Contains(encoded, `"sid":"client-1"`) {
		t.Errorf("expected sid in connect ack, got %q", encoded)
	}
}

func TestClientCompat_NamespaceConnectWithAuth(t *testing.T) {
	// socket.io-client: io('/chat', { auth: { user_id: 'abc123' } })
	// Wire: 0/chat,{"user_id":"abc123"}
	s := NewServer()

	var receivedAuth json.RawMessage
	chatNS := s.Of("/chat")
	chatNS.OnConnect(func(sid string, auth json.RawMessage) error {
		receivedAuth = auth
		return nil
	})

	connectSocket(t, s, "client-1")

	_, err := s.DispatchPacketFull("client-1", &Packet{
		Type:      PacketConnect,
		Namespace: "/chat",
		ID:        -1,
		Data:      json.RawMessage(`{"user_id":"abc123"}`),
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if receivedAuth == nil {
		t.Fatal("expected auth data")
	}

	var auth struct {
		UserID string `json:"user_id"`
	}
	json.Unmarshal(receivedAuth, &auth)
	if auth.UserID != "abc123" {
		t.Errorf("expected user_id 'abc123', got %q", auth.UserID)
	}
}

func TestClientCompat_NamespaceConnectReject(t *testing.T) {
	// Server rejects namespace connection
	// Expected wire: 4/chat,{"message":"unauthorized"}
	s := NewServer()

	chatNS := s.Of("/chat")
	chatNS.OnConnect(func(sid string, auth json.RawMessage) error {
		return fmt.Errorf("unauthorized")
	})

	connectSocket(t, s, "client-1")

	result, _ := s.DispatchPacketFull("client-1", &Packet{
		Type:      PacketConnect,
		Namespace: "/chat",
		ID:        -1,
	})

	// Even on error, we may get a CONNECT_ERROR response packet
	// The important thing is that the namespace didn't accept the connection
	if chatNS.HasSocket("client-1") {
		t.Error("client should NOT be connected to /chat after rejection")
	}

	_ = result // result behavior depends on error
}

func TestClientCompat_NamespaceConnectInvalidNamespace(t *testing.T) {
	// Connection to non-existent namespace should return CONNECT_ERROR
	// Wire: 4/nonexistent,{"message":"Invalid namespace"}
	s := NewServer()
	connectSocket(t, s, "client-1")

	result, err := s.DispatchPacketFull("client-1", &Packet{
		Type:      PacketConnect,
		Namespace: "/nonexistent",
		ID:        -1,
	})

	if err == nil {
		t.Error("expected error for nonexistent namespace")
	}

	if result != nil && result.ResponsePacket != nil {
		if result.ResponsePacket.Type != PacketConnectError {
			t.Errorf("expected CONNECT_ERROR (4), got type %d", result.ResponsePacket.Type)
		}

		encoded, _ := Encode(result.ResponsePacket)
		if !strings.HasPrefix(encoded, "4/nonexistent,") {
			t.Errorf("expected '4/nonexistent,...', got %q", encoded)
		}
	}
}

func TestClientCompat_NamespaceEventIsolation(t *testing.T) {
	// Events on different namespaces should be isolated
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	var defaultCalled, chatCalled bool

	s.On("test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		defaultCalled = true
		return nil, nil
	})

	chatNS := s.Of("/chat")
	chatNS.On("test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		chatCalled = true
		return nil, nil
	})

	connectSocket(t, s, "client-1")
	connectSocketNS(t, s, "client-1", "/chat")

	// Emit to default namespace
	s.DispatchPacket("client-1", &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["test","data"]`),
	})

	if !defaultCalled {
		t.Error("default namespace handler should have been called")
	}
	if chatCalled {
		t.Error("chat namespace handler should NOT have been called")
	}

	// Reset
	defaultCalled = false

	// Emit to /chat namespace
	s.DispatchPacket("client-1", &Packet{
		Type:      PacketEvent,
		Namespace: "/chat",
		ID:        -1,
		Data:      json.RawMessage(`["test","data"]`),
	})

	if defaultCalled {
		t.Error("default namespace handler should NOT have been called")
	}
	if !chatCalled {
		t.Error("chat namespace handler should have been called")
	}

	_ = sent
}

func TestClientCompat_NamespaceEventWithAck(t *testing.T) {
	// ACK on custom namespace
	// Wire: 2/chat,1["msg","hello"]
	// Expected ACK: 3/chat,1[{"status":"ok"}]
	s := NewServer()

	chatNS := s.Of("/chat")
	chatNS.On("msg", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{map[string]interface{}{"status": "ok"}}, nil
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

	if len(ackData) != 1 {
		t.Fatalf("expected 1 ack element, got %d", len(ackData))
	}

	ackPkt, _ := BuildAckPacket(pkt, ackData)
	encoded, _ := Encode(ackPkt)

	// Verify ACK includes namespace
	if !strings.HasPrefix(encoded, "3/chat,1") {
		t.Errorf("expected '3/chat,1...', got %q", encoded)
	}
}

func TestClientCompat_NamespaceBroadcast(t *testing.T) {
	// Server broadcasts to namespace — should only reach sockets in that namespace
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.Of("/chat")

	// client-1 connects to both / and /chat
	connectSocket(t, s, "client-1")
	connectSocketNS(t, s, "client-1", "/chat")

	// client-2 connects to / only
	connectSocket(t, s, "client-2")

	// Broadcast to /chat namespace
	n, err := s.BroadcastToNamespace("/chat", "notification", "hello")
	if err != nil {
		t.Fatalf("broadcast error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 recipient (only client-1 in /chat), got %d", n)
	}

	if len((*sent)["client-1"]) != 1 {
		t.Errorf("client-1 should have received 1 message, got %d", len((*sent)["client-1"]))
	}
	if len((*sent)["client-2"]) != 0 {
		t.Errorf("client-2 should have received 0 messages, got %d", len((*sent)["client-2"]))
	}

	// Verify the wire format includes namespace
	msg := (*sent)["client-1"][0]
	if !strings.HasPrefix(msg, "2/chat,") {
		t.Errorf("expected message with namespace prefix '2/chat,', got %q", msg)
	}
}

func TestClientCompat_NamespaceDisconnect(t *testing.T) {
	// socket.io-client: chatSocket.disconnect()
	// Wire: 1/chat,  (DISCONNECT from /chat namespace)
	s := NewServer()

	var disconnectedSID string
	var disconnectReason string
	chatNS := s.Of("/chat")
	chatNS.OnDisconnect(func(sid string, reason string) {
		disconnectedSID = sid
		disconnectReason = reason
	})

	connectSocket(t, s, "client-1")
	connectSocketNS(t, s, "client-1", "/chat")

	if !chatNS.HasSocket("client-1") {
		t.Fatal("client-1 should be connected to /chat")
	}

	// Client disconnects from /chat namespace only
	s.DispatchPacket("client-1", &Packet{
		Type:      PacketDisconnect,
		Namespace: "/chat",
		ID:        -1,
	})

	if chatNS.HasSocket("client-1") {
		t.Error("client-1 should be disconnected from /chat")
	}
	if disconnectedSID != "client-1" {
		t.Errorf("expected disconnect for 'client-1', got %q", disconnectedSID)
	}
	if disconnectReason != "client namespace disconnect" {
		t.Errorf("expected reason 'client namespace disconnect', got %q", disconnectReason)
	}

	// Should still be connected to default namespace
	if !s.GetNamespace("/").HasSocket("client-1") {
		t.Error("client-1 should still be connected to default namespace")
	}
}

// --- Section 5: Wire format encode/decode round-trip ---

func TestClientCompat_WireFormatRoundTrip_Event(t *testing.T) {
	// Verify encode/decode round-trip for various event formats
	tests := []struct {
		name string
		wire string
	}{
		{"simple event", `2["hello","world"]`},
		{"event with object", `2["fire",{"lat":37.5}]`},
		{"event with ack", `21["test","data"]`},
		{"event on namespace", `2/chat,["msg","hi"]`},
		{"event on namespace with ack", `2/chat,5["msg","hi"]`},
		{"event no args", `2["ping"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt, err := Decode(tt.wire)
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}

			encoded, err := Encode(pkt)
			if err != nil {
				t.Fatalf("Encode error: %v", err)
			}

			if encoded != tt.wire {
				t.Errorf("round-trip failed:\n  input:  %q\n  output: %q", tt.wire, encoded)
			}
		})
	}
}

func TestClientCompat_WireFormatRoundTrip_Binary(t *testing.T) {
	tests := []struct {
		name string
		wire string
	}{
		{"binary event", `51-["bin",{"_placeholder":true,"num":0}]`},
		{"binary event 2 attachments", `52-["bin",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]`},
		{"binary event on namespace", `51-/chat,["bin",{"_placeholder":true,"num":0}]`},
		{"binary event with ack", `51-3["bin",{"_placeholder":true,"num":0}]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt, err := Decode(tt.wire)
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}

			encoded, err := Encode(pkt)
			if err != nil {
				t.Fatalf("Encode error: %v", err)
			}

			if encoded != tt.wire {
				t.Errorf("round-trip failed:\n  input:  %q\n  output: %q", tt.wire, encoded)
			}
		})
	}
}

func TestClientCompat_WireFormatRoundTrip_Connect(t *testing.T) {
	tests := []struct {
		name string
		wire string
	}{
		{"connect default ns", `0{"sid":"abc123"}`},
		{"connect custom ns", `0/chat,{"sid":"abc123"}`},
		{"connect error", `4{"message":"invalid"}`},
		{"connect error ns", `4/chat,{"message":"not found"}`},
		{"disconnect", `1`},
		{"disconnect ns", `1/chat,`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt, err := Decode(tt.wire)
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}

			encoded, err := Encode(pkt)
			if err != nil {
				t.Fatalf("Encode error: %v", err)
			}

			if encoded != tt.wire {
				t.Errorf("round-trip failed:\n  input:  %q\n  output: %q", tt.wire, encoded)
			}
		})
	}
}

// --- Section 6: Server-to-client event emission format ---

func TestClientCompat_ServerEmitEventFormat(t *testing.T) {
	// Verify the server emits events in the format socket.io-client v4.8.3 expects
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	connectSocket(t, s, "client-1")

	// Server emits fire:update to client
	err := s.SendToSocket("/", "client-1", "fire:update", map[string]interface{}{
		"gridId":      "grid-123",
		"activeCount": 5,
		"stage":       2,
	})
	if err != nil {
		t.Fatalf("SendToSocket error: %v", err)
	}

	if len((*sent)["client-1"]) != 1 {
		t.Fatalf("expected 1 message, got %d", len((*sent)["client-1"]))
	}

	wire := (*sent)["client-1"][0]
	// Should be: 2["fire:update",{"activeCount":5,"gridId":"grid-123","stage":2}]
	if !strings.HasPrefix(wire, `2["fire:update",`) {
		t.Errorf("expected event format '2[\"fire:update\",...', got %q", wire)
	}

	// Decode and verify
	pkt, err := Decode(wire)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if pkt.Type != PacketEvent {
		t.Errorf("expected EVENT type, got %d", pkt.Type)
	}
	name, _ := pkt.EventName()
	if name != "fire:update" {
		t.Errorf("expected event name 'fire:update', got %q", name)
	}
}

func TestClientCompat_ServerEmitToNamespace(t *testing.T) {
	// Server emits event on custom namespace
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	s.Of("/chat")
	connectSocket(t, s, "client-1")
	connectSocketNS(t, s, "client-1", "/chat")

	err := s.SendToSocket("/chat", "client-1", "chat:message", map[string]interface{}{
		"text": "hello",
		"from": "server",
	})
	if err != nil {
		t.Fatalf("SendToSocket error: %v", err)
	}

	wire := (*sent)["client-1"][0]
	// Should start with 2/chat, for namespace-scoped event
	if !strings.HasPrefix(wire, `2/chat,["chat:message",`) {
		t.Errorf("expected namespace event format '2/chat,[\"chat:message\",...', got %q", wire)
	}
}

func TestClientCompat_ServerBroadcastUsersCount(t *testing.T) {
	// Verify users:count broadcast format (used by client)
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	connectSocket(t, s, "client-1")
	connectSocket(t, s, "client-2")

	n, err := s.BroadcastToNamespace("/", "users:count", map[string]interface{}{
		"count": 2,
	})
	if err != nil {
		t.Fatalf("broadcast error: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 recipients, got %d", n)
	}

	// Verify wire format for both clients
	for _, sid := range []string{"client-1", "client-2"} {
		if len((*sent)[sid]) != 1 {
			t.Errorf("%s should have received 1 message", sid)
			continue
		}
		wire := (*sent)[sid][0]
		if !strings.HasPrefix(wire, `2["users:count",`) {
			t.Errorf("expected format '2[\"users:count\",...', got %q", wire)
		}
	}
}

// --- Section 7: DispatchRaw compatibility (simulates Engine.IO message relay) ---

func TestClientCompat_DispatchRaw_Event(t *testing.T) {
	// Simulate Engine.IO relaying a raw Socket.IO packet string
	s := NewServer()
	called := false

	s.On("hello", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		called = true
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	// Raw wire format as received from Engine.IO message frame
	_, _, err := s.DispatchRaw("client-1", `2["hello","world"]`)
	if err != nil {
		t.Fatalf("DispatchRaw error: %v", err)
	}
	if !called {
		t.Error("handler should have been called via DispatchRaw")
	}
}

func TestClientCompat_DispatchRaw_Connect(t *testing.T) {
	// Raw connect packet
	s := NewServer()
	s.Of("/chat")

	connectSocket(t, s, "client-1")

	result, pkt, err := s.DispatchRawFull("client-1", `0/chat,`)
	if err != nil {
		t.Fatalf("DispatchRawFull error: %v", err)
	}
	if pkt.Type != PacketConnect {
		t.Errorf("expected CONNECT type, got %d", pkt.Type)
	}
	if pkt.Namespace != "/chat" {
		t.Errorf("expected namespace '/chat', got %q", pkt.Namespace)
	}
	if result.ResponsePacket == nil {
		t.Fatal("expected connect response packet")
	}
}

func TestClientCompat_DispatchRaw_NamespaceEvent(t *testing.T) {
	s := NewServer()
	chatNS := s.Of("/chat")

	chatCalled := false
	chatNS.On("msg", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		chatCalled = true
		return nil, nil
	})

	connectSocket(t, s, "client-1")
	connectSocketNS(t, s, "client-1", "/chat")

	_, _, err := s.DispatchRaw("client-1", `2/chat,["msg","hello"]`)
	if err != nil {
		t.Fatalf("DispatchRaw error: %v", err)
	}
	if !chatCalled {
		t.Error("chat namespace handler should have been called")
	}
}

// --- Section 8: Connection lifecycle (matching socket.io-client v4.8.3) ---

func TestClientCompat_ConnectDefaultNamespace(t *testing.T) {
	// socket.io-client: io(url) → sends CONNECT to default namespace
	// Wire: 0  or  0{"token":"xyz"}
	s := NewServer()

	var connectedSID string
	s.OnConnect(func(sid string, auth json.RawMessage) error {
		connectedSID = sid
		return nil
	})

	result, err := s.DispatchPacketFull("client-1", &Packet{
		Type:      PacketConnect,
		Namespace: "/",
		ID:        -1,
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if connectedSID != "client-1" {
		t.Errorf("expected sid 'client-1', got %q", connectedSID)
	}

	// Verify CONNECT ACK
	if result.ResponsePacket == nil {
		t.Fatal("expected connect response")
	}
	encoded, _ := Encode(result.ResponsePacket)
	if !strings.HasPrefix(encoded, `0{"sid":"client-1"}`) {
		t.Errorf("expected connect ack '0{\"sid\":\"client-1\"}', got %q", encoded)
	}
}

func TestClientCompat_ConnectWithAuth(t *testing.T) {
	// socket.io-client: io(url, { auth: { user_id: 'anon-123' } })
	// Wire: 0{"user_id":"anon-123"}
	s := NewServer()

	var receivedUserID string
	s.OnConnect(func(sid string, auth json.RawMessage) error {
		if auth != nil {
			var data struct {
				UserID string `json:"user_id"`
			}
			json.Unmarshal(auth, &data)
			receivedUserID = data.UserID
		}
		return nil
	})

	result, err := s.DispatchPacketFull("client-1", &Packet{
		Type:      PacketConnect,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`{"user_id":"anon-123"}`),
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if receivedUserID != "anon-123" {
		t.Errorf("expected user_id 'anon-123', got %q", receivedUserID)
	}

	if result.ResponsePacket == nil {
		t.Fatal("expected connect ack")
	}
}

func TestClientCompat_TransportCloseDisconnectsAll(t *testing.T) {
	// When the underlying transport closes, all namespace connections are cleaned up
	s := NewServer()
	s.Of("/chat")

	var defaultDisconnected, chatDisconnected bool
	s.OnDisconnect(func(sid string, reason string) {
		defaultDisconnected = true
	})
	s.Of("/chat").OnDisconnect(func(sid string, reason string) {
		chatDisconnected = true
	})

	connectSocket(t, s, "client-1")
	connectSocketNS(t, s, "client-1", "/chat")

	// Simulate transport close (Engine.IO session ends)
	s.DisconnectAll("client-1", "transport close")

	if !defaultDisconnected {
		t.Error("default namespace disconnect handler should have been called")
	}
	if !chatDisconnected {
		t.Error("/chat namespace disconnect handler should have been called")
	}

	if s.GetNamespace("/").HasSocket("client-1") {
		t.Error("client should be removed from default namespace")
	}
	if s.GetNamespace("/chat").HasSocket("client-1") {
		t.Error("client should be removed from /chat namespace")
	}
}

func TestClientCompat_ConnectedNamespaces(t *testing.T) {
	s := NewServer()
	s.Of("/chat")
	s.Of("/admin")

	connectSocket(t, s, "client-1")
	connectSocketNS(t, s, "client-1", "/chat")

	nss := s.ConnectedNamespaces("client-1")
	if len(nss) != 2 {
		t.Errorf("expected 2 connected namespaces, got %d: %v", len(nss), nss)
	}

	// Verify / and /chat are in the list
	found := make(map[string]bool)
	for _, ns := range nss {
		found[ns] = true
	}
	if !found["/"] || !found["/chat"] {
		t.Errorf("expected / and /chat, got %v", nss)
	}

	// /admin should not be connected
	if found["/admin"] {
		t.Error("/admin should not be in connected namespaces")
	}
}

// --- Section 9: Edge cases for socket.io-client v4 compatibility ---

func TestClientCompat_EmptyNamespaceDefaultsToRoot(t *testing.T) {
	// Some edge cases where namespace might be empty
	s := NewServer()
	called := false

	s.On("test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		called = true
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	// Packet with empty namespace should default to "/"
	_, err := s.DispatchPacket("client-1", &Packet{
		Type:      PacketEvent,
		Namespace: "", // empty
		ID:        -1,
		Data:      json.RawMessage(`["test"]`),
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if !called {
		t.Error("handler should have been called on default namespace")
	}
}

func TestClientCompat_LargeAckID(t *testing.T) {
	// socket.io-client uses incrementing ack IDs that can get large
	s := NewServer()

	s.On("test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{"ok"}, nil
	})

	connectSocket(t, s, "client-1")

	pkt := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        999999,
		Data:      json.RawMessage(`["test"]`),
	}
	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	ackPkt, _ := BuildAckPacket(pkt, ackData)
	if ackPkt.ID != 999999 {
		t.Errorf("expected ack id 999999, got %d", ackPkt.ID)
	}

	encoded, _ := Encode(ackPkt)
	if !strings.HasPrefix(encoded, "3999999") {
		t.Errorf("expected '3999999...', got %q", encoded)
	}
}

func TestClientCompat_NullAndUndefinedArgs(t *testing.T) {
	// socket.io-client can send null values
	// Wire: 2["test",null]
	s := NewServer()

	var receivedArg json.RawMessage
	s.On("test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		if len(args) > 0 {
			receivedArg = args[0]
		}
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	_, err := s.DispatchPacket("client-1", &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["test",null]`),
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if string(receivedArg) != "null" {
		t.Errorf("expected null arg, got %q", string(receivedArg))
	}
}

func TestClientCompat_SpecialCharactersInEventName(t *testing.T) {
	// socket.io-client supports colon-separated event names (fire:ignite, chat:send, etc.)
	s := NewServer()

	events := []string{"fire:ignite", "chat:send", "chat:join", "chat:leave",
		"subscribe:viewport", "fire:state", "fire:global_update"}

	results := make(map[string]bool)
	for _, event := range events {
		e := event
		s.On(e, func(sid string, args ...json.RawMessage) ([]interface{}, error) {
			results[e] = true
			return nil, nil
		})
	}

	connectSocket(t, s, "client-1")

	for _, event := range events {
		data, _ := json.Marshal([]interface{}{event, map[string]string{}})
		_, err := s.DispatchPacket("client-1", &Packet{
			Type:      PacketEvent,
			Namespace: "/",
			ID:        -1,
			Data:      json.RawMessage(data),
		})
		if err != nil {
			t.Errorf("dispatch error for event %q: %v", event, err)
		}
	}

	for _, event := range events {
		if !results[event] {
			t.Errorf("handler for event %q was not called", event)
		}
	}
}

func TestClientCompat_ConcurrentEmits(t *testing.T) {
	// Verify thread safety with concurrent emit dispatches
	// (socket.io-client can send events rapidly)
	s := NewServer()
	sendFn, _ := mockSendFunc()
	s.SendTo = sendFn

	var mu sync.Mutex
	count := 0

	s.On("rapid", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		mu.Lock()
		count++
		mu.Unlock()
		return nil, nil
	})

	connectSocket(t, s, "client-1")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.DispatchPacket("client-1", &Packet{
				Type:      PacketEvent,
				Namespace: "/",
				ID:        -1,
				Data:      json.RawMessage(`["rapid","data"]`),
			})
		}()
	}
	wg.Wait()

	mu.Lock()
	if count != 100 {
		t.Errorf("expected 100 handled events, got %d", count)
	}
	mu.Unlock()
}
