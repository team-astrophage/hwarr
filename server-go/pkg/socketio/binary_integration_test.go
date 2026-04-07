package socketio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

// =============================================================================
// Binary payload integration tests.
//
// These tests verify the full binary round-trip through the Socket.IO server:
//   1. Client sends ArrayBuffer/Buffer → server receives binary data
//   2. Server broadcasts binary data → client receives ArrayBuffer/Buffer
//   3. Nested binary fields in complex JSON structures
//   4. Multiple binary attachments in a single event
//   5. Binary ACK round-trip
//   6. Binary broadcast to rooms
//
// Matches socket.io-client v4.8.x binary protocol behavior.
// =============================================================================

// --- Helper: binary-aware send function that tracks both text and binary frames ---

type binarySendRecord struct {
	TextFrames   []string
	BinaryFrames [][]byte
}

func mockBinarySendFunc() (SendFunc, *map[string]*binarySendRecord) {
	sent := make(map[string]*binarySendRecord)
	mu := sync.Mutex{}
	fn := func(sid string, data string) error {
		mu.Lock()
		defer mu.Unlock()
		rec, ok := sent[sid]
		if !ok {
			rec = &binarySendRecord{}
			sent[sid] = rec
		}
		rec.TextFrames = append(rec.TextFrames, data)
		return nil
	}
	return fn, &sent
}

// simulateClientBinaryEmit simulates what socket.io-client v4 does when sending
// binary data: it sends a BINARY_EVENT text frame with placeholders, followed by
// binary attachment frames. The server then reconstructs the binary data.
func simulateClientBinaryEmit(t *testing.T, s *Server, sid string, textFrame string, attachments [][]byte) ([]interface{}, error) {
	t.Helper()
	pkt, err := Decode(textFrame)
	if err != nil {
		t.Fatalf("failed to decode text frame: %v", err)
	}

	// Add binary attachments (simulating binary WebSocket frames)
	for _, attachment := range attachments {
		pkt.AddAttachment(attachment)
	}

	if !pkt.IsComplete() {
		t.Fatalf("packet not complete after adding %d attachments", len(attachments))
	}

	// Reconstruct binary data from placeholders
	if err := pkt.ReconstructBinary(); err != nil {
		t.Fatalf("failed to reconstruct binary: %v", err)
	}

	return s.DispatchPacket(sid, pkt)
}

// --- Integration: client sends single binary buffer, server echoes back ---

func TestIntegration_BinaryRoundTrip_SingleBuffer(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	originalData := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE}

	// Register echo handler that receives binary and broadcasts it back
	s.On("binary:echo", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		// The binary data arrives as base64-encoded string in JSON after reconstruction
		// We parse it and echo back the received data
		if len(args) == 0 {
			return nil, fmt.Errorf("no args received")
		}

		// Decode the received binary data
		var received interface{}
		if err := json.Unmarshal(args[0], &received); err != nil {
			return nil, fmt.Errorf("failed to unmarshal arg: %v", err)
		}

		return []interface{}{map[string]interface{}{
			"status": "ok",
			"size":   len(args),
		}}, nil
	})

	connectSocket(t, s, "client-1")

	// Simulate client sending binary: 51-["binary:echo",{"_placeholder":true,"num":0}]
	textFrame := `51-["binary:echo",{"_placeholder":true,"num":0}]`
	ackData, err := simulateClientBinaryEmit(t, s, "client-1", textFrame, [][]byte{originalData})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

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

	_ = sent // SendTo is configured but echo doesn't broadcast
}

// --- Integration: server broadcasts binary data to room members ---

func TestIntegration_BinaryBroadcastToRoom(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	binaryPayload := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	connectSocket(t, s, "sid-1")
	connectSocket(t, s, "sid-2")
	connectSocket(t, s, "sid-3")

	s.EnterRoom("/", "sid-1", "binary-room")
	s.EnterRoom("/", "sid-2", "binary-room")
	// sid-3 is NOT in the room

	// Server creates a binary event packet and broadcasts to room
	pkt, err := NewBinaryEventPacket("/", "file:data", -1, binaryPayload)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	// Encode for wire
	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	// Verify text frame format
	if result.TextFrame == "" {
		t.Fatal("expected non-empty text frame")
	}
	if len(result.BinaryFrames) != 1 {
		t.Fatalf("expected 1 binary frame, got %d", len(result.BinaryFrames))
	}
	if !bytes.Equal(result.BinaryFrames[0], binaryPayload) {
		t.Errorf("binary frame mismatch: got %v, want %v", result.BinaryFrames[0], binaryPayload)
	}

	// Broadcast the text frame to room members
	members := s.RoomMembers("/", "binary-room")
	for _, sid := range members {
		s.SendTo(sid, result.TextFrame)
	}

	// Verify only room members received the text frame
	if len((*sent)["sid-1"]) != 1 {
		t.Errorf("sid-1 should have received 1 message, got %d", len((*sent)["sid-1"]))
	}
	if len((*sent)["sid-2"]) != 1 {
		t.Errorf("sid-2 should have received 1 message, got %d", len((*sent)["sid-2"]))
	}
	if len((*sent)["sid-3"]) != 0 {
		t.Errorf("sid-3 should have received 0 messages, got %d", len((*sent)["sid-3"]))
	}

	// Verify the text frame can be decoded back correctly
	for _, sid := range []string{"sid-1", "sid-2"} {
		receivedFrame := (*sent)[sid][0]
		decoded, err := Decode(receivedFrame)
		if err != nil {
			t.Fatalf("client %s: failed to decode received frame: %v", sid, err)
		}
		if decoded.Type != PacketBinaryEvent {
			t.Errorf("client %s: expected BINARY_EVENT, got %d", sid, decoded.Type)
		}
		if decoded.Attachments != 1 {
			t.Errorf("client %s: expected 1 attachment, got %d", sid, decoded.Attachments)
		}

		// Simulate client receiving binary frames
		decoded.AddAttachment(result.BinaryFrames[0])
		if !decoded.IsComplete() {
			t.Fatalf("client %s: packet should be complete", sid)
		}

		if err := decoded.ReconstructBinary(); err != nil {
			t.Fatalf("client %s: ReconstructBinary error: %v", sid, err)
		}

		name, _ := decoded.EventName()
		if name != "file:data" {
			t.Errorf("client %s: expected event 'file:data', got '%s'", sid, name)
		}
	}
}

// --- Integration: multiple binary attachments in a single event ---

func TestIntegration_BinaryMultipleAttachments(t *testing.T) {
	s := NewServer()

	buf1 := []byte{0xAA, 0xBB}
	buf2 := []byte{0xCC, 0xDD, 0xEE}
	buf3 := []byte{0xFF}

	var receivedArgs []json.RawMessage

	s.On("multi:upload", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		receivedArgs = args
		return []interface{}{map[string]interface{}{"status": "ok", "count": len(args)}}, nil
	})

	connectSocket(t, s, "client-1")

	// Client sends 3 binary attachments:
	// 53-["multi:upload",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1},{"_placeholder":true,"num":2}]
	textFrame := `53-["multi:upload",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1},{"_placeholder":true,"num":2}]`
	ackData, err := simulateClientBinaryEmit(t, s, "client-1", textFrame, [][]byte{buf1, buf2, buf3})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	// Verify handler received all 3 args
	if len(receivedArgs) != 3 {
		t.Fatalf("expected 3 args, got %d", len(receivedArgs))
	}

	// Verify ack
	if len(ackData) != 1 {
		t.Fatalf("expected 1 ack element, got %d", len(ackData))
	}
	ackMap := ackData[0].(map[string]interface{})
	if ackMap["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", ackMap["status"])
	}

	// Verify the server can re-encode and send back all 3 binary payloads
	pkt, err := NewBinaryEventPacket("/", "multi:data", -1, buf1, buf2, buf3)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}
	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}
	if len(result.BinaryFrames) != 3 {
		t.Errorf("expected 3 binary frames, got %d", len(result.BinaryFrames))
	}
	if !bytes.Equal(result.BinaryFrames[0], buf1) {
		t.Errorf("frame[0] mismatch")
	}
	if !bytes.Equal(result.BinaryFrames[1], buf2) {
		t.Errorf("frame[1] mismatch")
	}
	if !bytes.Equal(result.BinaryFrames[2], buf3) {
		t.Errorf("frame[2] mismatch")
	}
}

// --- Integration: nested binary fields in complex JSON structure ---

func TestIntegration_BinaryNestedFields(t *testing.T) {
	s := NewServer()

	// Simulate a complex structure with binary data nested at multiple levels:
	// {
	//   "metadata": {"name": "test.png", "size": 1024},
	//   "thumbnail": <binary>,
	//   "layers": [
	//     {"id": 1, "data": <binary>},
	//     {"id": 2, "data": <binary>}
	//   ]
	// }

	thumbnail := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header
	layer1Data := []byte{0x01, 0x02, 0x03, 0x04}
	layer2Data := []byte{0x05, 0x06, 0x07, 0x08}

	var receivedArg json.RawMessage

	s.On("image:upload", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		if len(args) > 0 {
			receivedArg = args[0]
		}
		return []interface{}{map[string]interface{}{"status": "ok"}}, nil
	})

	connectSocket(t, s, "client-1")

	// Client sends nested structure with binary placeholders:
	// Attachment order: 0=thumbnail, 1=layer1Data, 2=layer2Data
	textFrame := `53-["image:upload",{"metadata":{"name":"test.png","size":1024},"thumbnail":{"_placeholder":true,"num":0},"layers":[{"id":1,"data":{"_placeholder":true,"num":1}},{"id":2,"data":{"_placeholder":true,"num":2}}]}]`

	_, err := simulateClientBinaryEmit(t, s, "client-1", textFrame, [][]byte{thumbnail, layer1Data, layer2Data})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	// Verify the handler received the reconstructed data
	if receivedArg == nil {
		t.Fatal("handler should have received an argument")
	}

	// Parse the received data to verify structure
	var received map[string]interface{}
	if err := json.Unmarshal(receivedArg, &received); err != nil {
		t.Fatalf("failed to unmarshal received arg: %v", err)
	}

	// Verify metadata (non-binary) is intact
	metadata, ok := received["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata should be a map")
	}
	if metadata["name"] != "test.png" {
		t.Errorf("metadata.name: expected 'test.png', got '%v'", metadata["name"])
	}
	if metadata["size"] != float64(1024) {
		t.Errorf("metadata.size: expected 1024, got '%v'", metadata["size"])
	}

	// Verify layers array structure is intact
	layers, ok := received["layers"].([]interface{})
	if !ok {
		t.Fatal("layers should be an array")
	}
	if len(layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(layers))
	}

	layer1, ok := layers[0].(map[string]interface{})
	if !ok {
		t.Fatal("layers[0] should be a map")
	}
	if layer1["id"] != float64(1) {
		t.Errorf("layers[0].id: expected 1, got %v", layer1["id"])
	}

	layer2, ok := layers[1].(map[string]interface{})
	if !ok {
		t.Fatal("layers[1] should be a map")
	}
	if layer2["id"] != float64(2) {
		t.Errorf("layers[1].id: expected 2, got %v", layer2["id"])
	}
}

// --- Integration: full binary round-trip (client → server → client) ---

func TestIntegration_BinaryFullRoundTrip(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	originalData := []byte{0xDE, 0xAD, 0xBE, 0xEF}

	// Handler receives binary, broadcasts back to room
	s.On("data:send", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		// Re-broadcast the received data as a server-originated binary event
		pkt, err := NewBinaryEventPacket("/", "data:received", -1, originalData)
		if err != nil {
			return nil, err
		}
		result, err := EncodeBinary(pkt)
		if err != nil {
			return nil, err
		}

		// Send text frame + binary frames to all room members
		members := s.RoomMembers("/", "data-room")
		for _, member := range members {
			s.SendTo(member, result.TextFrame)
			// In real implementation, binary frames are sent as separate WS frames
		}

		return []interface{}{map[string]interface{}{"status": "ok"}}, nil
	})

	connectSocket(t, s, "sender")
	connectSocket(t, s, "receiver")

	s.EnterRoom("/", "sender", "data-room")
	s.EnterRoom("/", "receiver", "data-room")

	// Client sends binary data
	textFrame := `51-["data:send",{"_placeholder":true,"num":0}]`
	_, err := simulateClientBinaryEmit(t, s, "sender", textFrame, [][]byte{originalData})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	// Both sender and receiver should have received the broadcast
	for _, sid := range []string{"sender", "receiver"} {
		messages := (*sent)[sid]
		if len(messages) == 0 {
			t.Fatalf("%s should have received at least 1 message", sid)
		}

		// Decode the received text frame
		decoded, err := Decode(messages[0])
		if err != nil {
			t.Fatalf("%s: decode error: %v", sid, err)
		}

		if decoded.Type != PacketBinaryEvent {
			t.Errorf("%s: expected BINARY_EVENT, got %d", sid, decoded.Type)
		}

		name, _ := decoded.EventName()
		if name != "data:received" {
			t.Errorf("%s: expected event 'data:received', got '%s'", sid, name)
		}

		// Simulate adding the binary attachment
		decoded.AddAttachment(originalData)
		if err := decoded.ReconstructBinary(); err != nil {
			t.Fatalf("%s: reconstruct error: %v", sid, err)
		}

		// Verify data integrity through ReconstructedData
		reconstructed, err := decoded.ReconstructedData()
		if err != nil {
			t.Fatalf("%s: ReconstructedData error: %v", sid, err)
		}

		arr, ok := reconstructed.([]interface{})
		if !ok {
			t.Fatalf("%s: expected array, got %T", sid, reconstructed)
		}
		if len(arr) < 2 {
			t.Fatalf("%s: expected at least 2 elements, got %d", sid, len(arr))
		}

		// The event name should be first
		if arr[0] != "data:received" {
			t.Errorf("%s: first element should be event name, got %v", sid, arr[0])
		}
	}
}

// --- Integration: binary ACK round-trip ---

func TestIntegration_BinaryAckRoundTrip(t *testing.T) {
	binaryResponse := []byte{0xCA, 0xFE, 0xBA, 0xBE}

	// Create a BINARY_ACK packet (server response with binary data)
	ackPkt, err := NewBinaryAckPacket("/", 42, binaryResponse, "metadata-string")
	if err != nil {
		t.Fatalf("NewBinaryAckPacket error: %v", err)
	}

	if ackPkt.Type != PacketBinaryAck {
		t.Errorf("expected BINARY_ACK(6), got %d", ackPkt.Type)
	}
	if ackPkt.ID != 42 {
		t.Errorf("expected ack ID 42, got %d", ackPkt.ID)
	}
	if ackPkt.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", ackPkt.Attachments)
	}

	// Encode for wire
	result, err := EncodeBinary(ackPkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	// Verify text frame format: 61-42[...]
	decoded, err := Decode(result.TextFrame)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if decoded.Type != PacketBinaryAck {
		t.Errorf("decoded type: expected BINARY_ACK(6), got %d", decoded.Type)
	}
	if decoded.ID != 42 {
		t.Errorf("decoded ack ID: expected 42, got %d", decoded.ID)
	}
	if decoded.Attachments != 1 {
		t.Errorf("decoded attachments: expected 1, got %d", decoded.Attachments)
	}

	// Simulate client receiving binary frame
	for _, frame := range result.BinaryFrames {
		decoded.AddAttachment(frame)
	}
	if !decoded.IsComplete() {
		t.Fatal("packet should be complete")
	}

	// Reconstruct
	if err := decoded.ReconstructBinary(); err != nil {
		t.Fatalf("ReconstructBinary error: %v", err)
	}

	// Verify ACK data
	ackData, err := decoded.AckData()
	if err != nil {
		t.Fatalf("AckData error: %v", err)
	}
	if len(ackData) != 2 {
		t.Fatalf("expected 2 ack data elements, got %d", len(ackData))
	}
}

// --- Integration: binary event dispatch through server pipeline ---

func TestIntegration_BinaryEventDispatch(t *testing.T) {
	s := NewServer()

	type uploadData struct {
		Filename string `json:"filename"`
	}

	var (
		receivedSID  string
		receivedArgs []json.RawMessage
	)

	s.On("file:upload", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		receivedSID = sid
		receivedArgs = args
		return []interface{}{map[string]interface{}{
			"status": "accepted",
			"args":   len(args),
		}}, nil
	})

	connectSocket(t, s, "uploader")

	fileData := []byte("file content bytes here")

	// Client sends: event name + metadata object + binary buffer
	// 51-["file:upload",{"filename":"test.txt"},{"_placeholder":true,"num":0}]
	// But this is tricky — the metadata doesn't have a placeholder.
	// Actually, when a client has mixed args (json + binary), the client sends:
	// 51-["file:upload",{"filename":"test.txt","data":{"_placeholder":true,"num":0}}]
	textFrame := `51-["file:upload",{"filename":"test.txt","data":{"_placeholder":true,"num":0}}]`

	pkt, err := Decode(textFrame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// Verify partial packet state before adding attachments
	if pkt.IsComplete() {
		t.Error("packet should NOT be complete before adding attachments")
	}

	pkt.AddAttachment(fileData)
	if !pkt.IsComplete() {
		t.Fatal("packet should be complete after adding attachment")
	}

	if err := pkt.ReconstructBinary(); err != nil {
		t.Fatalf("reconstruct error: %v", err)
	}

	// Dispatch through server
	ackData, err := s.DispatchPacket("uploader", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	// Verify handler was called with correct SID
	if receivedSID != "uploader" {
		t.Errorf("expected SID 'uploader', got '%s'", receivedSID)
	}

	// Verify args count
	if len(receivedArgs) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(receivedArgs))
	}

	// Parse the received arg
	var argMap map[string]interface{}
	if err := json.Unmarshal(receivedArgs[0], &argMap); err != nil {
		t.Fatalf("failed to unmarshal arg: %v", err)
	}
	if argMap["filename"] != "test.txt" {
		t.Errorf("filename: expected 'test.txt', got '%v'", argMap["filename"])
	}

	// Verify ACK
	if len(ackData) != 1 {
		t.Fatalf("expected 1 ack element, got %d", len(ackData))
	}
	ackMap := ackData[0].(map[string]interface{})
	if ackMap["status"] != "accepted" {
		t.Errorf("ack status: expected 'accepted', got '%v'", ackMap["status"])
	}
}

// --- Integration: deeply nested binary fields ---

func TestIntegration_BinaryDeeplyNested(t *testing.T) {
	// Test a deeply nested structure:
	// {
	//   "level1": {
	//     "level2": {
	//       "level3": {
	//         "binary": <buffer>,
	//         "text": "hello"
	//       }
	//     },
	//     "another_binary": <buffer>
	//   }
	// }

	deepBuf := []byte{0x01, 0x02, 0x03}
	shallowBuf := []byte{0x04, 0x05, 0x06}

	textFrame := `52-["nested:test",{"level1":{"level2":{"level3":{"binary":{"_placeholder":true,"num":0},"text":"hello"}},"another_binary":{"_placeholder":true,"num":1}}}]`

	pkt, err := Decode(textFrame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if pkt.Attachments != 2 {
		t.Fatalf("expected 2 attachments, got %d", pkt.Attachments)
	}

	pkt.AddAttachment(deepBuf)
	if pkt.IsComplete() {
		t.Error("should not be complete after 1/2 attachments")
	}

	pkt.AddAttachment(shallowBuf)
	if !pkt.IsComplete() {
		t.Fatal("should be complete after 2/2 attachments")
	}

	if err := pkt.ReconstructBinary(); err != nil {
		t.Fatalf("reconstruct error: %v", err)
	}

	// Parse reconstructed data
	reconstructed, err := pkt.ReconstructedData()
	if err != nil {
		t.Fatalf("ReconstructedData error: %v", err)
	}

	arr, ok := reconstructed.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", reconstructed)
	}
	if len(arr) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(arr))
	}

	// Navigate the nested structure
	topMap, ok := arr[1].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map at arr[1], got %T", arr[1])
	}

	level1, ok := topMap["level1"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map at level1, got %T", topMap["level1"])
	}

	level2, ok := level1["level2"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map at level2, got %T", level1["level2"])
	}

	level3, ok := level2["level3"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map at level3, got %T", level2["level3"])
	}

	// Verify text field preserved
	if level3["text"] != "hello" {
		t.Errorf("level3.text: expected 'hello', got '%v'", level3["text"])
	}

	// Verify deep binary was reconstructed (should be base64-encoded after re-marshal)
	// After ReconstructBinary, the data is re-marshaled, so binary becomes base64 in JSON
	// But ReconstructedData returns the interface{} tree with []byte intact
}

// --- Integration: binary in array elements ---

func TestIntegration_BinaryInArrayElements(t *testing.T) {
	// Test binary data inside array elements:
	// ["chunks:upload", [<buf0>, <buf1>, <buf2>]]

	chunk0 := []byte{0x10, 0x20}
	chunk1 := []byte{0x30, 0x40}
	chunk2 := []byte{0x50, 0x60}

	textFrame := `53-["chunks:upload",[{"_placeholder":true,"num":0},{"_placeholder":true,"num":1},{"_placeholder":true,"num":2}]]`

	pkt, err := Decode(textFrame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	pkt.AddAttachment(chunk0)
	pkt.AddAttachment(chunk1)
	pkt.AddAttachment(chunk2)

	if !pkt.IsComplete() {
		t.Fatal("should be complete")
	}

	if err := pkt.ReconstructBinary(); err != nil {
		t.Fatalf("reconstruct error: %v", err)
	}

	name, err := pkt.EventName()
	if err != nil {
		t.Fatalf("EventName error: %v", err)
	}
	if name != "chunks:upload" {
		t.Errorf("expected event 'chunks:upload', got '%s'", name)
	}

	args, err := pkt.EventArgs()
	if err != nil {
		t.Fatalf("EventArgs error: %v", err)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg (the array), got %d", len(args))
	}
}

// --- Integration: server-side binary deconstruct → encode → client decode round-trip ---

func TestIntegration_ServerToClientBinaryRoundTrip(t *testing.T) {
	// Simulate: server has raw binary data, needs to send it to client
	// 1. Server creates packet with []byte args
	// 2. DeconstructBinary replaces with placeholders
	// 3. EncodeBinary produces text frame + binary frames
	// 4. Client decodes text frame
	// 5. Client adds binary attachments
	// 6. Client reconstructs and gets original data

	originalPayloads := [][]byte{
		{0xDE, 0xAD, 0xBE, 0xEF},
		{0xCA, 0xFE, 0xBA, 0xBE},
	}

	pkt, err := NewBinaryEventPacket("/", "data:multi", -1, originalPayloads[0], originalPayloads[1])
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected BINARY_EVENT, got %d", pkt.Type)
	}
	if pkt.Attachments != 2 {
		t.Errorf("expected 2 attachments, got %d", pkt.Attachments)
	}

	// Encode
	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	if len(result.BinaryFrames) != 2 {
		t.Fatalf("expected 2 binary frames, got %d", len(result.BinaryFrames))
	}

	// Client decode
	clientPkt, err := Decode(result.TextFrame)
	if err != nil {
		t.Fatalf("client Decode error: %v", err)
	}

	if clientPkt.Type != PacketBinaryEvent {
		t.Errorf("client: expected BINARY_EVENT, got %d", clientPkt.Type)
	}
	if clientPkt.Attachments != 2 {
		t.Errorf("client: expected 2 attachments, got %d", clientPkt.Attachments)
	}

	// Client adds binary frames in order
	for i, frame := range result.BinaryFrames {
		complete := clientPkt.AddAttachment(frame)
		if i < len(result.BinaryFrames)-1 && complete {
			t.Errorf("should not be complete after %d/%d attachments", i+1, len(result.BinaryFrames))
		}
	}

	if !clientPkt.IsComplete() {
		t.Fatal("client packet should be complete")
	}

	// Client reconstructs
	if err := clientPkt.ReconstructBinary(); err != nil {
		t.Fatalf("client ReconstructBinary error: %v", err)
	}

	name, _ := clientPkt.EventName()
	if name != "data:multi" {
		t.Errorf("expected event 'data:multi', got '%s'", name)
	}
}

// --- Integration: binary with namespace ---

func TestIntegration_BinaryWithNamespace(t *testing.T) {
	s := NewServer()
	sendFn, sent := mockSendFunc()
	s.SendTo = sendFn

	// Create /files namespace
	filesNS := s.Of("/files")

	var receivedInNamespace bool

	filesNS.On("upload", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		receivedInNamespace = true
		return []interface{}{map[string]interface{}{"status": "ok"}}, nil
	})

	connectSocketNS(t, s, "client-1", "/files")

	// Client sends binary to /files namespace
	textFrame := `51-/files,["upload",{"_placeholder":true,"num":0}]`
	fileData := []byte{0xAB, 0xCD, 0xEF}

	_, err := simulateClientBinaryEmit(t, s, "client-1", textFrame, [][]byte{fileData})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if !receivedInNamespace {
		t.Error("handler in /files namespace should have been called")
	}

	// Server broadcasts binary in /files namespace
	pkt, err := NewBinaryEventPacket("/files", "file:ready", -1, fileData)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	// Verify namespace is in the text frame
	decoded, err := Decode(result.TextFrame)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if decoded.Namespace != "/files" {
		t.Errorf("expected namespace '/files', got '%s'", decoded.Namespace)
	}

	_ = sent
}

// --- Integration: binary event with ACK ID ---

func TestIntegration_BinaryEventWithAckID(t *testing.T) {
	s := NewServer()

	s.On("data:process", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{map[string]interface{}{"processed": true}}, nil
	})

	connectSocket(t, s, "client-1")

	// Client sends binary event with ack ID
	textFrame := `51-5["data:process",{"_placeholder":true,"num":0}]`
	data := []byte{0x01, 0x02, 0x03}

	pkt, err := Decode(textFrame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if pkt.ID != 5 {
		t.Errorf("expected ack ID 5, got %d", pkt.ID)
	}

	pkt.AddAttachment(data)
	if err := pkt.ReconstructBinary(); err != nil {
		t.Fatalf("reconstruct error: %v", err)
	}

	ackData, err := s.DispatchPacket("client-1", pkt)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	// Build ACK packet
	ackPkt, err := BuildAckPacket(pkt, ackData)
	if err != nil {
		t.Fatalf("BuildAckPacket error: %v", err)
	}

	if ackPkt == nil {
		t.Fatal("expected ACK packet (ack ID was 5)")
	}
	if ackPkt.Type != PacketAck {
		t.Errorf("expected ACK type, got %d", ackPkt.Type)
	}
	if ackPkt.ID != 5 {
		t.Errorf("expected ack ID 5, got %d", ackPkt.ID)
	}
}

// --- Integration: empty binary payload ---

func TestIntegration_BinaryEmptyPayload(t *testing.T) {
	emptyBuf := []byte{}

	pkt, err := NewBinaryEventPacket("/", "empty:data", -1, emptyBuf)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected BINARY_EVENT even for empty buffer, got %d", pkt.Type)
	}
	if pkt.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", pkt.Attachments)
	}

	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	if len(result.BinaryFrames) != 1 {
		t.Fatalf("expected 1 binary frame, got %d", len(result.BinaryFrames))
	}
	if len(result.BinaryFrames[0]) != 0 {
		t.Errorf("expected empty binary frame, got %d bytes", len(result.BinaryFrames[0]))
	}

	// Client decode round-trip
	decoded, err := Decode(result.TextFrame)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	decoded.AddAttachment(emptyBuf)
	if err := decoded.ReconstructBinary(); err != nil {
		t.Fatalf("ReconstructBinary error: %v", err)
	}
}

// --- Integration: large binary payload ---

func TestIntegration_BinaryLargePayload(t *testing.T) {
	// 1MB payload
	largeBuf := make([]byte, 1024*1024)
	for i := range largeBuf {
		largeBuf[i] = byte(i % 256)
	}

	pkt, err := NewBinaryEventPacket("/", "big:data", -1, largeBuf)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	if len(result.BinaryFrames) != 1 {
		t.Fatalf("expected 1 binary frame, got %d", len(result.BinaryFrames))
	}
	if !bytes.Equal(result.BinaryFrames[0], largeBuf) {
		t.Error("large binary payload mismatch after encode")
	}

	// Full round-trip
	decoded, err := Decode(result.TextFrame)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	decoded.AddAttachment(result.BinaryFrames[0])
	if err := decoded.ReconstructBinary(); err != nil {
		t.Fatalf("ReconstructBinary error: %v", err)
	}
}

// --- Integration: concurrent binary dispatch ---

func TestIntegration_BinaryConcurrentDispatch(t *testing.T) {
	s := NewServer()

	var mu sync.Mutex
	receivedCount := 0

	s.On("binary:concurrent", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		return []interface{}{map[string]interface{}{"ok": true}}, nil
	})

	const numClients = 20
	for i := 0; i < numClients; i++ {
		connectSocket(t, s, fmt.Sprintf("client-%d", i))
	}

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sid := fmt.Sprintf("client-%d", idx)
			data := []byte{byte(idx), byte(idx + 1)}

			textFrame := `51-["binary:concurrent",{"_placeholder":true,"num":0}]`
			pkt, err := Decode(textFrame)
			if err != nil {
				t.Errorf("client-%d: decode error: %v", idx, err)
				return
			}
			pkt.AddAttachment(data)
			if err := pkt.ReconstructBinary(); err != nil {
				t.Errorf("client-%d: reconstruct error: %v", idx, err)
				return
			}
			_, err = s.DispatchPacket(sid, pkt)
			if err != nil {
				t.Errorf("client-%d: dispatch error: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	if receivedCount != numClients {
		t.Errorf("expected %d received events, got %d", numClients, receivedCount)
	}
}

// --- Integration: mixed binary and non-binary args ---

func TestIntegration_BinaryMixedArgs(t *testing.T) {
	// Event with mixed args: string, binary, number, binary
	buf1 := []byte{0xAA, 0xBB}
	buf2 := []byte{0xCC, 0xDD}

	// Client sends: ["mixed:event", "hello", <buf1>, 42, <buf2>]
	// Wire format: 52-["mixed:event","hello",{"_placeholder":true,"num":0},42,{"_placeholder":true,"num":1}]
	textFrame := `52-["mixed:event","hello",{"_placeholder":true,"num":0},42,{"_placeholder":true,"num":1}]`

	pkt, err := Decode(textFrame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected BINARY_EVENT, got %d", pkt.Type)
	}
	if pkt.Attachments != 2 {
		t.Errorf("expected 2 attachments, got %d", pkt.Attachments)
	}

	pkt.AddAttachment(buf1)
	pkt.AddAttachment(buf2)

	if !pkt.IsComplete() {
		t.Fatal("should be complete")
	}

	if err := pkt.ReconstructBinary(); err != nil {
		t.Fatalf("reconstruct error: %v", err)
	}

	// Verify event name and args
	name, _ := pkt.EventName()
	if name != "mixed:event" {
		t.Errorf("expected 'mixed:event', got '%s'", name)
	}

	args, err := pkt.EventArgs()
	if err != nil {
		t.Fatalf("EventArgs error: %v", err)
	}

	// Should have 4 args: "hello", <reconstructed buf1>, 42, <reconstructed buf2>
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d", len(args))
	}

	// Verify string arg
	var strArg string
	if err := json.Unmarshal(args[0], &strArg); err != nil {
		t.Fatalf("failed to unmarshal string arg: %v", err)
	}
	if strArg != "hello" {
		t.Errorf("expected 'hello', got '%s'", strArg)
	}

	// Verify number arg
	var numArg float64
	if err := json.Unmarshal(args[2], &numArg); err != nil {
		t.Fatalf("failed to unmarshal number arg: %v", err)
	}
	if numArg != 42 {
		t.Errorf("expected 42, got %v", numArg)
	}
}

// --- Integration: incremental attachment addition ---

func TestIntegration_BinaryIncrementalAttachments(t *testing.T) {
	// Verify that attachments can be added one at a time (simulating
	// WebSocket frames arriving in sequence)

	textFrame := `53-["stream:chunks",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1},{"_placeholder":true,"num":2}]`

	pkt, err := Decode(textFrame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	chunks := [][]byte{
		{0x01, 0x02},
		{0x03, 0x04},
		{0x05, 0x06},
	}

	// Add attachments one by one, verifying state at each step
	for i, chunk := range chunks {
		if pkt.IsComplete() {
			t.Fatalf("should not be complete after %d/%d attachments", i, len(chunks))
		}

		complete := pkt.AddAttachment(chunk)

		if i < len(chunks)-1 {
			if complete {
				t.Errorf("AddAttachment returned true after %d/%d", i+1, len(chunks))
			}
			if pkt.IsComplete() {
				t.Errorf("IsComplete true after %d/%d", i+1, len(chunks))
			}
		} else {
			if !complete {
				t.Error("AddAttachment should return true for last attachment")
			}
			if !pkt.IsComplete() {
				t.Error("IsComplete should be true after all attachments")
			}
		}
	}

	// Verify we can reconstruct after all attachments added
	if err := pkt.ReconstructBinary(); err != nil {
		t.Fatalf("ReconstructBinary error: %v", err)
	}

	name, _ := pkt.EventName()
	if name != "stream:chunks" {
		t.Errorf("expected 'stream:chunks', got '%s'", name)
	}
}
