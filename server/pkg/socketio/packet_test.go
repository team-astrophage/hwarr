package socketio

import (
	"encoding/json"
	"testing"
)

// --- Decode tests ---

func TestDecodeEventSimple(t *testing.T) {
	p, err := Decode(`2["hello","world"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketEvent {
		t.Errorf("expected type EVENT(2), got %d", p.Type)
	}
	if p.Namespace != "/" {
		t.Errorf("expected namespace /, got %s", p.Namespace)
	}
	if p.ID != -1 {
		t.Errorf("expected no ack id (-1), got %d", p.ID)
	}
	name, err := p.EventName()
	if err != nil {
		t.Fatalf("EventName error: %v", err)
	}
	if name != "hello" {
		t.Errorf("expected event name 'hello', got '%s'", name)
	}
	args, err := p.EventArgs()
	if err != nil {
		t.Fatalf("EventArgs error: %v", err)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
	var arg string
	json.Unmarshal(args[0], &arg)
	if arg != "world" {
		t.Errorf("expected arg 'world', got '%s'", arg)
	}
}

func TestDecodeEventWithNamespace(t *testing.T) {
	p, err := Decode(`2/chat,["hello"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketEvent {
		t.Errorf("expected type EVENT(2), got %d", p.Type)
	}
	if p.Namespace != "/chat" {
		t.Errorf("expected namespace /chat, got %s", p.Namespace)
	}
}

func TestDecodeEventWithAckID(t *testing.T) {
	p, err := Decode(`213["hello"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketEvent {
		t.Errorf("expected type EVENT(2), got %d", p.Type)
	}
	if p.ID != 13 {
		t.Errorf("expected ack id 13, got %d", p.ID)
	}
	if p.Namespace != "/" {
		t.Errorf("expected namespace /, got %s", p.Namespace)
	}
}

func TestDecodeEventWithNamespaceAndAckID(t *testing.T) {
	p, err := Decode(`2/chat,13["hello"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketEvent {
		t.Errorf("expected type EVENT(2), got %d", p.Type)
	}
	if p.Namespace != "/chat" {
		t.Errorf("expected namespace /chat, got %s", p.Namespace)
	}
	if p.ID != 13 {
		t.Errorf("expected ack id 13, got %d", p.ID)
	}
}

func TestDecodeAckSimple(t *testing.T) {
	p, err := Decode(`31["result"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketAck {
		t.Errorf("expected type ACK(3), got %d", p.Type)
	}
	if p.ID != 1 {
		t.Errorf("expected ack id 1, got %d", p.ID)
	}
	data, err := p.AckData()
	if err != nil {
		t.Fatalf("AckData error: %v", err)
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 data element, got %d", len(data))
	}
	var val string
	json.Unmarshal(data[0], &val)
	if val != "result" {
		t.Errorf("expected 'result', got '%s'", val)
	}
}

func TestDecodeAckWithNamespace(t *testing.T) {
	p, err := Decode(`3/chat,7["ok",42]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketAck {
		t.Errorf("expected type ACK(3), got %d", p.Type)
	}
	if p.Namespace != "/chat" {
		t.Errorf("expected namespace /chat, got %s", p.Namespace)
	}
	if p.ID != 7 {
		t.Errorf("expected ack id 7, got %d", p.ID)
	}
	data, err := p.AckData()
	if err != nil {
		t.Fatalf("AckData error: %v", err)
	}
	if len(data) != 2 {
		t.Fatalf("expected 2 data elements, got %d", len(data))
	}
}

func TestDecodeConnect(t *testing.T) {
	p, err := Decode(`0`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketConnect {
		t.Errorf("expected type CONNECT(0), got %d", p.Type)
	}
	if p.Namespace != "/" {
		t.Errorf("expected namespace /, got %s", p.Namespace)
	}
}

func TestDecodeConnectWithNamespace(t *testing.T) {
	p, err := Decode(`0/admin`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketConnect {
		t.Errorf("expected type CONNECT(0), got %d", p.Type)
	}
	if p.Namespace != "/admin" {
		t.Errorf("expected namespace /admin, got %s", p.Namespace)
	}
}

func TestDecodeConnectWithAuth(t *testing.T) {
	p, err := Decode(`0{"token":"abc"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketConnect {
		t.Errorf("expected type CONNECT(0), got %d", p.Type)
	}
	if p.Data == nil {
		t.Fatal("expected auth data")
	}
}

func TestDecodeConnectNamespaceWithAuth(t *testing.T) {
	p, err := Decode(`0/admin,{"token":"abc"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketConnect {
		t.Errorf("expected type CONNECT(0), got %d", p.Type)
	}
	if p.Namespace != "/admin" {
		t.Errorf("expected namespace /admin, got %s", p.Namespace)
	}
}

func TestDecodeDisconnect(t *testing.T) {
	p, err := Decode(`1`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketDisconnect {
		t.Errorf("expected type DISCONNECT(1), got %d", p.Type)
	}
}

func TestDecodeBinaryEvent(t *testing.T) {
	p, err := Decode(`51-["binary",{"_placeholder":true,"num":0}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketBinaryEvent {
		t.Errorf("expected type BINARY_EVENT(5), got %d", p.Type)
	}
	if p.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", p.Attachments)
	}
}

func TestDecodeBinaryEventWithNamespace(t *testing.T) {
	p, err := Decode(`51-/chat,["binary",{"_placeholder":true,"num":0}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketBinaryEvent {
		t.Errorf("expected type BINARY_EVENT(5), got %d", p.Type)
	}
	if p.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", p.Attachments)
	}
	if p.Namespace != "/chat" {
		t.Errorf("expected namespace /chat, got %s", p.Namespace)
	}
}

func TestDecodeBinaryEventWithNamespaceAndAckID(t *testing.T) {
	p, err := Decode(`52-/chat,7["binary",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketBinaryEvent {
		t.Errorf("expected type BINARY_EVENT(5), got %d", p.Type)
	}
	if p.Attachments != 2 {
		t.Errorf("expected 2 attachments, got %d", p.Attachments)
	}
	if p.Namespace != "/chat" {
		t.Errorf("expected namespace /chat, got %s", p.Namespace)
	}
	if p.ID != 7 {
		t.Errorf("expected ack id 7, got %d", p.ID)
	}
}

func TestDecodeBinaryEventMultipleAttachments(t *testing.T) {
	p, err := Decode(`53-["upload",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1},{"_placeholder":true,"num":2}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Attachments != 3 {
		t.Errorf("expected 3 attachments, got %d", p.Attachments)
	}
	name, err := p.EventName()
	if err != nil {
		t.Fatalf("EventName error: %v", err)
	}
	if name != "upload" {
		t.Errorf("expected event name 'upload', got '%s'", name)
	}
}

func TestDecodeBinaryAck(t *testing.T) {
	p, err := Decode(`61-5[{"_placeholder":true,"num":0}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketBinaryAck {
		t.Errorf("expected type BINARY_ACK(6), got %d", p.Type)
	}
	if p.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", p.Attachments)
	}
	if p.ID != 5 {
		t.Errorf("expected ack id 5, got %d", p.ID)
	}
}

func TestDecodeBinaryAckWithNamespace(t *testing.T) {
	p, err := Decode(`62-/admin,3[{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketBinaryAck {
		t.Errorf("expected type BINARY_ACK(6), got %d", p.Type)
	}
	if p.Attachments != 2 {
		t.Errorf("expected 2 attachments, got %d", p.Attachments)
	}
	if p.Namespace != "/admin" {
		t.Errorf("expected namespace /admin, got %s", p.Namespace)
	}
	if p.ID != 3 {
		t.Errorf("expected ack id 3, got %d", p.ID)
	}
}

func TestDecodeBinaryAckNoData(t *testing.T) {
	// BINARY_ACK with attachment count but no JSON data yet
	p, err := Decode(`61-5`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketBinaryAck {
		t.Errorf("expected type BINARY_ACK(6), got %d", p.Type)
	}
	if p.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", p.Attachments)
	}
	if p.ID != 5 {
		t.Errorf("expected ack id 5, got %d", p.ID)
	}
	if p.Data != nil {
		t.Errorf("expected nil data")
	}
}

func TestDecodeConnectError(t *testing.T) {
	p, err := Decode(`4{"message":"Not authorized"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketConnectError {
		t.Errorf("expected type CONNECT_ERROR(4), got %d", p.Type)
	}
}

func TestDecodeEventWithObjectArg(t *testing.T) {
	p, err := Decode(`2["fire:ignite",{"lat":37.5,"lng":127.0}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name, err := p.EventName()
	if err != nil {
		t.Fatalf("EventName error: %v", err)
	}
	if name != "fire:ignite" {
		t.Errorf("expected event 'fire:ignite', got '%s'", name)
	}
	args, err := p.EventArgs()
	if err != nil {
		t.Fatalf("EventArgs error: %v", err)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
	var obj map[string]float64
	json.Unmarshal(args[0], &obj)
	if obj["lat"] != 37.5 || obj["lng"] != 127.0 {
		t.Errorf("unexpected coordinates: %v", obj)
	}
}

func TestDecodeEventMultipleArgs(t *testing.T) {
	p, err := Decode(`2["chat:send","hello",42,true]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args, err := p.EventArgs()
	if err != nil {
		t.Fatalf("EventArgs error: %v", err)
	}
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
}

func TestDecodeEventNoArgs(t *testing.T) {
	p, err := Decode(`2["ping"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name, _ := p.EventName()
	if name != "ping" {
		t.Errorf("expected 'ping', got '%s'", name)
	}
	args, _ := p.EventArgs()
	if args != nil {
		t.Errorf("expected nil args, got %v", args)
	}
}

// --- Error cases ---

func TestDecodeEmpty(t *testing.T) {
	_, err := Decode("")
	if err != ErrEmptyPacket {
		t.Errorf("expected ErrEmptyPacket, got %v", err)
	}
}

func TestDecodeInvalidType(t *testing.T) {
	_, err := Decode("9")
	if err != ErrInvalidType {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestDecodeInvalidJSON(t *testing.T) {
	_, err := Decode(`2[invalid`)
	if err != ErrInvalidJSON {
		t.Errorf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestDecodeBinaryMissingDash(t *testing.T) {
	_, err := Decode(`5["hello"]`)
	if err == nil {
		t.Error("expected error for binary event without dash")
	}
}

// --- Encode tests ---

func TestEncodeEventSimple(t *testing.T) {
	p := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        -1,
		Data:      json.RawMessage(`["hello","world"]`),
	}
	s, err := Encode(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != `2["hello","world"]` {
		t.Errorf("expected '2[\"hello\",\"world\"]', got '%s'", s)
	}
}

func TestEncodeEventWithNamespace(t *testing.T) {
	p := &Packet{
		Type:      PacketEvent,
		Namespace: "/chat",
		ID:        -1,
		Data:      json.RawMessage(`["hello"]`),
	}
	s, err := Encode(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != `2/chat,["hello"]` {
		t.Errorf("expected '2/chat,[\"hello\"]', got '%s'", s)
	}
}

func TestEncodeEventWithAckID(t *testing.T) {
	p := &Packet{
		Type:      PacketEvent,
		Namespace: "/",
		ID:        13,
		Data:      json.RawMessage(`["hello"]`),
	}
	s, err := Encode(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != `213["hello"]` {
		t.Errorf("expected '213[\"hello\"]', got '%s'", s)
	}
}

func TestEncodeEventWithNamespaceAndAckID(t *testing.T) {
	p := &Packet{
		Type:      PacketEvent,
		Namespace: "/chat",
		ID:        13,
		Data:      json.RawMessage(`["hello"]`),
	}
	s, err := Encode(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != `2/chat,13["hello"]` {
		t.Errorf("expected '2/chat,13[\"hello\"]', got '%s'", s)
	}
}

func TestEncodeAck(t *testing.T) {
	p := &Packet{
		Type:      PacketAck,
		Namespace: "/",
		ID:        1,
		Data:      json.RawMessage(`["result"]`),
	}
	s, err := Encode(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != `31["result"]` {
		t.Errorf("expected '31[\"result\"]', got '%s'", s)
	}
}

func TestEncodeAckWithNamespace(t *testing.T) {
	p := &Packet{
		Type:      PacketAck,
		Namespace: "/chat",
		ID:        7,
		Data:      json.RawMessage(`["ok",42]`),
	}
	s, err := Encode(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != `3/chat,7["ok",42]` {
		t.Errorf("expected '3/chat,7[\"ok\",42]', got '%s'", s)
	}
}

func TestEncodeConnect(t *testing.T) {
	p := &Packet{
		Type:      PacketConnect,
		Namespace: "/",
		ID:        -1,
	}
	s, _ := Encode(p)
	if s != "0" {
		t.Errorf("expected '0', got '%s'", s)
	}
}

func TestEncodeDisconnect(t *testing.T) {
	p := &Packet{
		Type:      PacketDisconnect,
		Namespace: "/admin",
		ID:        -1,
	}
	s, _ := Encode(p)
	if s != "1/admin," {
		t.Errorf("expected '1/admin,', got '%s'", s)
	}
}

func TestEncodeBinaryEvent(t *testing.T) {
	p := &Packet{
		Type:        PacketBinaryEvent,
		Namespace:   "/",
		ID:          -1,
		Attachments: 1,
		Data:        json.RawMessage(`["binary",{"_placeholder":true,"num":0}]`),
	}
	s, _ := Encode(p)
	if s != `51-["binary",{"_placeholder":true,"num":0}]` {
		t.Errorf("unexpected: '%s'", s)
	}
}

// --- Roundtrip tests ---

func TestRoundtrip(t *testing.T) {
	cases := []string{
		`2["hello","world"]`,
		`2/chat,["hello"]`,
		`213["hello"]`,
		`2/chat,13["hello"]`,
		`31["result"]`,
		`3/chat,7["ok",42]`,
		`0`,
		`1`,
		`1/admin,`,
		`4{"message":"error"}`,
		`51-["binary",{"_placeholder":true,"num":0}]`,
	}

	for _, tc := range cases {
		p, err := Decode(tc)
		if err != nil {
			t.Errorf("Decode(%q) failed: %v", tc, err)
			continue
		}
		encoded, err := Encode(p)
		if err != nil {
			t.Errorf("Encode after Decode(%q) failed: %v", tc, err)
			continue
		}
		if encoded != tc {
			t.Errorf("roundtrip mismatch: input=%q output=%q", tc, encoded)
		}
	}
}

// --- Constructor tests ---

func TestNewEventPacket(t *testing.T) {
	p, err := NewEventPacket("/", "fire:ignite", -1, map[string]float64{"lat": 37.5, "lng": 127.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketEvent {
		t.Errorf("expected EVENT type, got %d", p.Type)
	}
	name, _ := p.EventName()
	if name != "fire:ignite" {
		t.Errorf("expected 'fire:ignite', got '%s'", name)
	}

	s, _ := Encode(p)
	// Verify it can be decoded back
	p2, err := Decode(s)
	if err != nil {
		t.Fatalf("roundtrip decode failed: %v", err)
	}
	name2, _ := p2.EventName()
	if name2 != "fire:ignite" {
		t.Errorf("roundtrip event name mismatch: '%s'", name2)
	}
}

func TestNewEventPacketWithAck(t *testing.T) {
	p, err := NewEventPacket("/chat", "message", 42, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != 42 {
		t.Errorf("expected ack id 42, got %d", p.ID)
	}
	if p.Namespace != "/chat" {
		t.Errorf("expected /chat, got %s", p.Namespace)
	}
}

func TestNewAckPacket(t *testing.T) {
	p, err := NewAckPacket("/", 5, "ok", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketAck {
		t.Errorf("expected ACK type, got %d", p.Type)
	}
	if p.ID != 5 {
		t.Errorf("expected ack id 5, got %d", p.ID)
	}
	data, _ := p.AckData()
	if len(data) != 2 {
		t.Fatalf("expected 2 data elements, got %d", len(data))
	}
}

func TestNewAckPacketEmpty(t *testing.T) {
	p, err := NewAckPacket("/", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Data != nil {
		t.Errorf("expected nil data for empty ack")
	}
}

// --- Edge cases ---

func TestDecodeEventAckIDZero(t *testing.T) {
	p, err := Decode(`20["hello"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != 0 {
		t.Errorf("expected ack id 0, got %d", p.ID)
	}
}

func TestDecodeAckNoData(t *testing.T) {
	p, err := Decode(`35`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketAck {
		t.Errorf("expected ACK, got %d", p.Type)
	}
	if p.ID != 5 {
		t.Errorf("expected ack id 5, got %d", p.ID)
	}
	if p.Data != nil {
		t.Errorf("expected nil data")
	}
}

func TestPacketTypeString(t *testing.T) {
	if PacketTypeString(PacketEvent) != "EVENT" {
		t.Error("expected EVENT")
	}
	if PacketTypeString(PacketAck) != "ACK" {
		t.Error("expected ACK")
	}
	if PacketTypeString(99) != "UNKNOWN(99)" {
		t.Error("expected UNKNOWN(99)")
	}
}

func TestEventNameOnNonEvent(t *testing.T) {
	p := &Packet{Type: PacketConnect}
	_, err := p.EventName()
	if err == nil {
		t.Error("expected error calling EventName on CONNECT packet")
	}
}

func TestAckDataOnNonAck(t *testing.T) {
	p := &Packet{Type: PacketEvent}
	_, err := p.AckData()
	if err == nil {
		t.Error("expected error calling AckData on EVENT packet")
	}
}

// --- Binary helper method tests ---

func TestAddAttachmentSingle(t *testing.T) {
	p, err := Decode(`51-["binary",{"_placeholder":true,"num":0}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.IsComplete() {
		t.Error("expected incomplete before adding attachment")
	}
	done := p.AddAttachment([]byte{0x01, 0x02, 0x03})
	if !done {
		t.Error("expected done=true after adding 1/1 attachment")
	}
	if !p.IsComplete() {
		t.Error("expected complete after adding all attachments")
	}
	if len(p.BinaryPayloads) != 1 {
		t.Errorf("expected 1 payload, got %d", len(p.BinaryPayloads))
	}
}

func TestAddAttachmentMultiple(t *testing.T) {
	p, err := Decode(`53-["upload",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1},{"_placeholder":true,"num":2}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	done := p.AddAttachment([]byte{0x01})
	if done {
		t.Error("expected done=false after 1/3")
	}
	done = p.AddAttachment([]byte{0x02})
	if done {
		t.Error("expected done=false after 2/3")
	}
	done = p.AddAttachment([]byte{0x03})
	if !done {
		t.Error("expected done=true after 3/3")
	}
	if len(p.BinaryPayloads) != 3 {
		t.Errorf("expected 3 payloads, got %d", len(p.BinaryPayloads))
	}
}

func TestAddAttachmentOnNonBinaryPacket(t *testing.T) {
	p := &Packet{Type: PacketEvent, Namespace: "/", ID: -1}
	done := p.AddAttachment([]byte{0x01})
	if done {
		t.Error("AddAttachment on non-binary packet should return false")
	}
}

func TestIsCompleteNonBinaryPacket(t *testing.T) {
	p := &Packet{Type: PacketEvent, Namespace: "/", ID: -1}
	if !p.IsComplete() {
		t.Error("non-binary packet should always be complete")
	}
}

func TestReconstructBinaryIncomplete(t *testing.T) {
	p, _ := Decode(`52-["data",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]`)
	p.AddAttachment([]byte{0x01})
	err := p.ReconstructBinary()
	if err == nil {
		t.Error("expected error for incomplete binary reconstruction")
	}
}

func TestReconstructBinaryComplete(t *testing.T) {
	p, _ := Decode(`51-["data",{"_placeholder":true,"num":0}]`)
	p.AddAttachment([]byte{0x01, 0x02})
	err := p.ReconstructBinary()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReconstructBinaryNonBinaryPacket(t *testing.T) {
	p := &Packet{Type: PacketEvent, Namespace: "/", ID: -1}
	err := p.ReconstructBinary()
	if err != nil {
		t.Errorf("unexpected error for non-binary packet: %v", err)
	}
}

func TestReconstructBinaryReplacesPlaceholder(t *testing.T) {
	p, _ := Decode(`51-["binary",{"_placeholder":true,"num":0}]`)
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	p.AddAttachment(payload)

	data, err := p.ReconstructedData()
	if err != nil {
		t.Fatalf("ReconstructedData error: %v", err)
	}

	arr, ok := data.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", data)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}
	if arr[0] != "binary" {
		t.Errorf("expected first element 'binary', got %v", arr[0])
	}

	// The placeholder should be replaced with the base64 string of the binary data
	// (since json.Marshal encodes []byte as base64)
	buf, ok := arr[1].([]byte)
	if !ok {
		t.Fatalf("expected []byte at index 1, got %T", arr[1])
	}
	if len(buf) != 4 || buf[0] != 0xDE || buf[1] != 0xAD || buf[2] != 0xBE || buf[3] != 0xEF {
		t.Errorf("unexpected binary payload: %v", buf)
	}
}

func TestReconstructBinaryMultiplePlaceholders(t *testing.T) {
	p, _ := Decode(`52-["upload",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]`)
	p.AddAttachment([]byte{0x01, 0x02})
	p.AddAttachment([]byte{0x03, 0x04})

	data, err := p.ReconstructedData()
	if err != nil {
		t.Fatalf("ReconstructedData error: %v", err)
	}

	arr, ok := data.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", data)
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}

	buf0, ok := arr[1].([]byte)
	if !ok {
		t.Fatalf("expected []byte at index 1, got %T", arr[1])
	}
	if buf0[0] != 0x01 || buf0[1] != 0x02 {
		t.Errorf("unexpected payload 0: %v", buf0)
	}

	buf1, ok := arr[2].([]byte)
	if !ok {
		t.Fatalf("expected []byte at index 2, got %T", arr[2])
	}
	if buf1[0] != 0x03 || buf1[1] != 0x04 {
		t.Errorf("unexpected payload 1: %v", buf1)
	}
}

func TestReconstructBinaryNestedPlaceholder(t *testing.T) {
	p, _ := Decode(`51-["file",{"name":"test.bin","data":{"_placeholder":true,"num":0}}]`)
	payload := []byte{0xFF, 0x00}
	p.AddAttachment(payload)

	data, err := p.ReconstructedData()
	if err != nil {
		t.Fatalf("ReconstructedData error: %v", err)
	}

	arr, ok := data.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", data)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}

	obj, ok := arr[1].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map at index 1, got %T", arr[1])
	}
	if obj["name"] != "test.bin" {
		t.Errorf("expected name 'test.bin', got %v", obj["name"])
	}

	buf, ok := obj["data"].([]byte)
	if !ok {
		t.Fatalf("expected []byte for nested 'data', got %T", obj["data"])
	}
	if buf[0] != 0xFF || buf[1] != 0x00 {
		t.Errorf("unexpected nested binary: %v", buf)
	}
}

func TestReconstructBinaryNilData(t *testing.T) {
	p := &Packet{
		Type:           PacketBinaryEvent,
		Namespace:      "/",
		ID:             -1,
		Attachments:    1,
		BinaryPayloads: [][]byte{{0x01}},
	}
	err := p.ReconstructBinary()
	if err != nil {
		t.Errorf("expected no error for nil data, got: %v", err)
	}
}

func TestReconstructBinaryInPlaceModifiesData(t *testing.T) {
	p, _ := Decode(`51-["ev",{"_placeholder":true,"num":0}]`)
	p.AddAttachment([]byte{0xAB, 0xCD})
	err := p.ReconstructBinary()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After ReconstructBinary, p.Data should contain base64-encoded binary
	// since json.Marshal encodes []byte as base64 string
	var arr []json.RawMessage
	if err := json.Unmarshal(p.Data, &arr); err != nil {
		t.Fatalf("failed to unmarshal reconstructed data: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}

	// The binary data should be base64-encoded in JSON
	var b64 string
	if err := json.Unmarshal(arr[1], &b64); err != nil {
		t.Fatalf("expected base64 string at index 1: %v", err)
	}
	// "q80=" is base64 of [0xAB, 0xCD]
	if b64 != "q80=" {
		t.Errorf("expected base64 'q80=', got '%s'", b64)
	}
}

func TestReconstructedDataNonBinaryPacket(t *testing.T) {
	p, _ := Decode(`2["hello","world"]`)
	data, err := p.ReconstructedData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr, ok := data.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", data)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}
	if arr[0] != "hello" || arr[1] != "world" {
		t.Errorf("unexpected data: %v", arr)
	}
}

func TestReconstructedDataNilData(t *testing.T) {
	p := &Packet{Type: PacketEvent, Namespace: "/", ID: -1}
	data, err := p.ReconstructedData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil, got %v", data)
	}
}

func TestReconstructBinaryOutOfRangePlaceholder(t *testing.T) {
	// Placeholder num:5 but only 1 attachment — should leave placeholder as-is
	p, _ := Decode(`51-["ev",{"_placeholder":true,"num":5}]`)
	p.AddAttachment([]byte{0x01})

	data, err := p.ReconstructedData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	arr, ok := data.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", data)
	}
	// Out-of-range placeholder left as map
	m, ok := arr[1].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map for out-of-range placeholder, got %T", arr[1])
	}
	if m["_placeholder"] != true {
		t.Error("expected _placeholder to remain true")
	}
}

func TestIsPlaceholderEdgeCases(t *testing.T) {
	// Not a placeholder - missing _placeholder key
	m1 := map[string]interface{}{"num": float64(0)}
	if _, ok := isPlaceholder(m1); ok {
		t.Error("expected false for missing _placeholder")
	}

	// Not a placeholder - _placeholder is false
	m2 := map[string]interface{}{"_placeholder": false, "num": float64(0)}
	if _, ok := isPlaceholder(m2); ok {
		t.Error("expected false for _placeholder=false")
	}

	// Not a placeholder - _placeholder is not bool
	m3 := map[string]interface{}{"_placeholder": "true", "num": float64(0)}
	if _, ok := isPlaceholder(m3); ok {
		t.Error("expected false for _placeholder=string")
	}

	// Not a placeholder - missing num
	m4 := map[string]interface{}{"_placeholder": true}
	if _, ok := isPlaceholder(m4); ok {
		t.Error("expected false for missing num")
	}

	// Valid placeholder
	m5 := map[string]interface{}{"_placeholder": true, "num": float64(2)}
	idx, ok := isPlaceholder(m5)
	if !ok {
		t.Error("expected true for valid placeholder")
	}
	if idx != 2 {
		t.Errorf("expected idx 2, got %d", idx)
	}
}

func TestReconstructBinaryAck(t *testing.T) {
	p, _ := Decode(`61-5[{"_placeholder":true,"num":0}]`)
	p.AddAttachment([]byte{0xAA, 0xBB})

	data, err := p.ReconstructedData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	arr, ok := data.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", data)
	}
	buf, ok := arr[0].([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", arr[0])
	}
	if buf[0] != 0xAA || buf[1] != 0xBB {
		t.Errorf("unexpected payload: %v", buf)
	}
}

func TestBinaryAckAddAttachment(t *testing.T) {
	p, err := Decode(`61-5[{"_placeholder":true,"num":0}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.IsComplete() {
		t.Error("expected incomplete")
	}
	done := p.AddAttachment([]byte{0xFF})
	if !done {
		t.Error("expected done after 1/1")
	}
	if !p.IsComplete() {
		t.Error("expected complete")
	}
}

// --- Encode binary tests ---

func TestEncodeBinaryAck(t *testing.T) {
	p := &Packet{
		Type:        PacketBinaryAck,
		Namespace:   "/",
		ID:          5,
		Attachments: 1,
		Data:        json.RawMessage(`[{"_placeholder":true,"num":0}]`),
	}
	s, err := Encode(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != `61-5[{"_placeholder":true,"num":0}]` {
		t.Errorf("unexpected: '%s'", s)
	}
}

func TestEncodeBinaryEventWithNamespace(t *testing.T) {
	p := &Packet{
		Type:        PacketBinaryEvent,
		Namespace:   "/chat",
		ID:          -1,
		Attachments: 2,
		Data:        json.RawMessage(`["upload",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]`),
	}
	s, err := Encode(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != `52-/chat,["upload",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]` {
		t.Errorf("unexpected: '%s'", s)
	}
}

func TestEncodeBinaryAckWithNamespace(t *testing.T) {
	p := &Packet{
		Type:        PacketBinaryAck,
		Namespace:   "/admin",
		ID:          3,
		Attachments: 1,
		Data:        json.RawMessage(`[{"_placeholder":true,"num":0}]`),
	}
	s, err := Encode(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != `61-/admin,3[{"_placeholder":true,"num":0}]` {
		t.Errorf("unexpected: '%s'", s)
	}
}

// --- Binary roundtrip tests ---

func TestBinaryRoundtrip(t *testing.T) {
	cases := []string{
		`51-["binary",{"_placeholder":true,"num":0}]`,
		`52-["multi",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]`,
		`51-/chat,["binary",{"_placeholder":true,"num":0}]`,
		`52-/chat,7["binary",{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]`,
		`61-5[{"_placeholder":true,"num":0}]`,
		`62-/admin,3[{"_placeholder":true,"num":0},{"_placeholder":true,"num":1}]`,
	}

	for _, tc := range cases {
		p, err := Decode(tc)
		if err != nil {
			t.Errorf("Decode(%q) failed: %v", tc, err)
			continue
		}
		encoded, err := Encode(p)
		if err != nil {
			t.Errorf("Encode after Decode(%q) failed: %v", tc, err)
			continue
		}
		if encoded != tc {
			t.Errorf("binary roundtrip mismatch: input=%q output=%q", tc, encoded)
		}
	}
}

// Test compatibility with socket.io-client v4 format
func TestSocketIOClientV4Compatibility(t *testing.T) {
	// These are actual packet formats sent by socket.io-client v4.8.x
	cases := []struct {
		input     string
		pktType   int
		namespace string
		ackID     int
	}{
		// Default namespace connect
		{`0{"sid":"abc123"}`, PacketConnect, "/", -1},
		// Event with no namespace
		{`2["heartbeat",{}]`, PacketEvent, "/", -1},
		// Event with ack
		{`20["fire:state",{"gridId":"abc"}]`, PacketEvent, "/", 0},
		// ACK response
		{`30[{"status":"ok"}]`, PacketAck, "/", 0},
		// Disconnect
		{`1`, PacketDisconnect, "/", -1},
	}

	for _, tc := range cases {
		p, err := Decode(tc.input)
		if err != nil {
			t.Errorf("Decode(%q) failed: %v", tc.input, err)
			continue
		}
		if p.Type != tc.pktType {
			t.Errorf("Decode(%q): type=%d, want=%d", tc.input, p.Type, tc.pktType)
		}
		if p.Namespace != tc.namespace {
			t.Errorf("Decode(%q): ns=%s, want=%s", tc.input, p.Namespace, tc.namespace)
		}
		if p.ID != tc.ackID {
			t.Errorf("Decode(%q): id=%d, want=%d", tc.input, p.ID, tc.ackID)
		}
	}
}

// --- DeconstructBinary tests ---

func TestDeconstructBinary_NoBinaryData(t *testing.T) {
	pkt, err := NewEventPacket("/", "hello", -1, "world")
	if err != nil {
		t.Fatalf("NewEventPacket error: %v", err)
	}
	if err := pkt.DeconstructBinary(); err != nil {
		t.Fatalf("DeconstructBinary error: %v", err)
	}
	// Should remain EVENT (no binary data)
	if pkt.Type != PacketEvent {
		t.Errorf("expected type EVENT(2), got %d", pkt.Type)
	}
	if pkt.Attachments != 0 {
		t.Errorf("expected 0 attachments, got %d", pkt.Attachments)
	}
}

func TestDeconstructBinary_NilData(t *testing.T) {
	pkt := &Packet{Type: PacketEvent, Namespace: "/", ID: -1}
	if err := pkt.DeconstructBinary(); err != nil {
		t.Fatalf("DeconstructBinary error: %v", err)
	}
	if pkt.Type != PacketEvent {
		t.Errorf("expected type EVENT(2), got %d", pkt.Type)
	}
}

func TestDeconstructBinary_SingleBinaryArg(t *testing.T) {
	binData := []byte{0xCA, 0xFE, 0xBA, 0xBE}

	// Build the data manually with []byte in the tree
	arr := []interface{}{"upload", binData}
	data, _ := json.Marshal(arr)
	// json.Marshal encodes []byte as base64 string, but DeconstructBinary
	// works on pre-marshaled interface{} trees. We need to use the
	// NewBinaryEventPacket convenience function instead.

	_ = data // not used directly

	pkt, err := NewBinaryEventPacket("/", "upload", -1, binData)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected type BINARY_EVENT(5), got %d", pkt.Type)
	}
	if pkt.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", pkt.Attachments)
	}
	if len(pkt.BinaryPayloads) != 1 {
		t.Fatalf("expected 1 binary payload, got %d", len(pkt.BinaryPayloads))
	}

	// Verify the binary payload matches
	for i, b := range binData {
		if pkt.BinaryPayloads[0][i] != b {
			t.Errorf("binary payload byte %d: got 0x%02X, want 0x%02X", i, pkt.BinaryPayloads[0][i], b)
		}
	}

	// Verify placeholder in data
	var arr2 []json.RawMessage
	if err := json.Unmarshal(pkt.Data, &arr2); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if len(arr2) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr2))
	}
	// Second element should be a placeholder
	var ph placeholder
	if err := json.Unmarshal(arr2[1], &ph); err != nil {
		t.Fatalf("failed to unmarshal placeholder: %v", err)
	}
	if !ph.Placeholder || ph.Num != 0 {
		t.Errorf("expected placeholder {true, 0}, got {%v, %d}", ph.Placeholder, ph.Num)
	}
}

func TestDeconstructBinary_MultipleBinaryArgs(t *testing.T) {
	buf1 := []byte{0x01, 0x02}
	buf2 := []byte{0x03, 0x04, 0x05}

	pkt, err := NewBinaryEventPacket("/", "multi", -1, buf1, "text", buf2)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected type BINARY_EVENT(5), got %d", pkt.Type)
	}
	if pkt.Attachments != 2 {
		t.Errorf("expected 2 attachments, got %d", pkt.Attachments)
	}
	if len(pkt.BinaryPayloads) != 2 {
		t.Fatalf("expected 2 binary payloads, got %d", len(pkt.BinaryPayloads))
	}

	// Verify buffers are in order
	if len(pkt.BinaryPayloads[0]) != 2 || pkt.BinaryPayloads[0][0] != 0x01 {
		t.Error("first binary payload mismatch")
	}
	if len(pkt.BinaryPayloads[1]) != 3 || pkt.BinaryPayloads[1][0] != 0x03 {
		t.Error("second binary payload mismatch")
	}
}

func TestDeconstructBinary_NestedBinary(t *testing.T) {
	binData := []byte{0xDE, 0xAD}
	nested := map[string]interface{}{
		"name": "file.bin",
		"data": binData,
	}

	pkt, err := NewBinaryEventPacket("/", "upload", -1, nested)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected type BINARY_EVENT(5), got %d", pkt.Type)
	}
	if pkt.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", pkt.Attachments)
	}

	// Verify the data contains the placeholder nested inside the object
	var arr []json.RawMessage
	if err := json.Unmarshal(pkt.Data, &arr); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	// arr[0] = "upload", arr[1] = {"name":"file.bin","data":{"_placeholder":true,"num":0}}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(arr[1], &obj); err != nil {
		t.Fatalf("failed to unmarshal nested object: %v", err)
	}

	var ph placeholder
	if err := json.Unmarshal(obj["data"], &ph); err != nil {
		t.Fatalf("failed to unmarshal placeholder: %v", err)
	}
	if !ph.Placeholder || ph.Num != 0 {
		t.Errorf("expected placeholder {true, 0}, got {%v, %d}", ph.Placeholder, ph.Num)
	}
}

func TestDeconstructBinary_WithNamespace(t *testing.T) {
	pkt, err := NewBinaryEventPacket("/chat", "file", 42, []byte{0xFF})
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	if pkt.Namespace != "/chat" {
		t.Errorf("expected namespace /chat, got %s", pkt.Namespace)
	}
	if pkt.ID != 42 {
		t.Errorf("expected ack ID 42, got %d", pkt.ID)
	}
	if pkt.Type != PacketBinaryEvent {
		t.Errorf("expected BINARY_EVENT, got %d", pkt.Type)
	}
}

// --- EncodeBinary tests ---

func TestEncodeBinary_SingleAttachment(t *testing.T) {
	binData := []byte{0xCA, 0xFE}
	pkt, err := NewBinaryEventPacket("/", "data", -1, binData)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	// Text frame should start with "51-" (BINARY_EVENT, 1 attachment)
	if len(result.TextFrame) < 3 {
		t.Fatalf("text frame too short: %q", result.TextFrame)
	}
	if result.TextFrame[:3] != "51-" {
		t.Errorf("expected text frame to start with '51-', got %q", result.TextFrame[:3])
	}

	// Should have 1 binary frame
	if len(result.BinaryFrames) != 1 {
		t.Fatalf("expected 1 binary frame, got %d", len(result.BinaryFrames))
	}
	if len(result.BinaryFrames[0]) != 2 || result.BinaryFrames[0][0] != 0xCA {
		t.Error("binary frame data mismatch")
	}
}

func TestEncodeBinary_MultipleAttachments(t *testing.T) {
	buf1 := []byte{0x01}
	buf2 := []byte{0x02}
	buf3 := []byte{0x03}
	pkt, err := NewBinaryEventPacket("/", "multi", -1, buf1, buf2, buf3)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	// Text frame should start with "53-" (BINARY_EVENT, 3 attachments)
	if result.TextFrame[:3] != "53-" {
		t.Errorf("expected text frame to start with '53-', got %q", result.TextFrame[:3])
	}

	if len(result.BinaryFrames) != 3 {
		t.Fatalf("expected 3 binary frames, got %d", len(result.BinaryFrames))
	}
}

func TestEncodeBinary_WithNamespace(t *testing.T) {
	pkt, err := NewBinaryEventPacket("/chat", "file", -1, []byte{0xFF})
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	// Should contain namespace: "51-/chat,[...]"
	if len(result.TextFrame) < 9 {
		t.Fatalf("text frame too short: %q", result.TextFrame)
	}
	expected := "51-/chat,"
	if result.TextFrame[:len(expected)] != expected {
		t.Errorf("expected text frame to start with %q, got %q", expected, result.TextFrame[:len(expected)])
	}
}

func TestEncodeBinary_WithAckID(t *testing.T) {
	pkt, err := NewBinaryEventPacket("/", "file", 7, []byte{0xAB})
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	// "51-7[...]" — BINARY_EVENT, 1 attachment, ack ID 7
	if len(result.TextFrame) < 4 {
		t.Fatalf("text frame too short: %q", result.TextFrame)
	}
	if result.TextFrame[:4] != "51-7" {
		t.Errorf("expected text frame to start with '51-7', got %q", result.TextFrame[:4])
	}
}

func TestEncodeBinary_NonBinaryPacket(t *testing.T) {
	pkt, err := NewEventPacket("/", "hello", -1, "world")
	if err != nil {
		t.Fatalf("NewEventPacket error: %v", err)
	}

	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	// Should be a regular EVENT packet with no binary frames
	if result.TextFrame[0] != '2' {
		t.Errorf("expected type 2 (EVENT), got %c", result.TextFrame[0])
	}
	if len(result.BinaryFrames) != 0 {
		t.Errorf("expected 0 binary frames, got %d", len(result.BinaryFrames))
	}
}

// --- NewBinaryAckPacket tests ---

func TestNewBinaryAckPacket_WithBinary(t *testing.T) {
	binData := []byte{0xBE, 0xEF}
	pkt, err := NewBinaryAckPacket("/", 5, binData)
	if err != nil {
		t.Fatalf("NewBinaryAckPacket error: %v", err)
	}

	if pkt.Type != PacketBinaryAck {
		t.Errorf("expected type BINARY_ACK(6), got %d", pkt.Type)
	}
	if pkt.ID != 5 {
		t.Errorf("expected ack ID 5, got %d", pkt.ID)
	}
	if pkt.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", pkt.Attachments)
	}

	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}
	// "61-5[...]" — BINARY_ACK, 1 attachment, ack ID 5
	if result.TextFrame[:4] != "61-5" {
		t.Errorf("expected text frame to start with '61-5', got %q", result.TextFrame[:4])
	}
}

func TestNewBinaryAckPacket_NoBinary(t *testing.T) {
	pkt, err := NewBinaryAckPacket("/", 3, "result")
	if err != nil {
		t.Fatalf("NewBinaryAckPacket error: %v", err)
	}

	// Should fall back to regular ACK
	if pkt.Type != PacketAck {
		t.Errorf("expected type ACK(3), got %d", pkt.Type)
	}
}

func TestNewBinaryEventPacket_NoBinary(t *testing.T) {
	pkt, err := NewBinaryEventPacket("/", "hello", -1, "world", 42)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	// Should fall back to regular EVENT
	if pkt.Type != PacketEvent {
		t.Errorf("expected type EVENT(2), got %d", pkt.Type)
	}
}

// --- Round-trip: Deconstruct → Encode → Decode → Reconstruct ---

func TestBinaryRoundTrip(t *testing.T) {
	originalBin := []byte{0xDE, 0xAD, 0xBE, 0xEF}

	// 1. Create binary event packet (server-side emit)
	pkt, err := NewBinaryEventPacket("/", "binary_data", -1, originalBin)
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	// 2. Encode for wire
	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	// 3. Decode text frame (simulating client receive)
	decoded, err := Decode(result.TextFrame)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if decoded.Type != PacketBinaryEvent {
		t.Errorf("expected BINARY_EVENT, got %d", decoded.Type)
	}
	if decoded.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", decoded.Attachments)
	}

	// 4. Add binary attachments
	for _, frame := range result.BinaryFrames {
		decoded.AddAttachment(frame)
	}

	if !decoded.IsComplete() {
		t.Fatal("expected packet to be complete after adding attachments")
	}

	// 5. Reconstruct binary data
	if err := decoded.ReconstructBinary(); err != nil {
		t.Fatalf("ReconstructBinary error: %v", err)
	}

	// 6. Verify event name
	name, err := decoded.EventName()
	if err != nil {
		t.Fatalf("EventName error: %v", err)
	}
	if name != "binary_data" {
		t.Errorf("expected event name 'binary_data', got '%s'", name)
	}
}

func TestBinaryRoundTrip_WithNamespaceAndAck(t *testing.T) {
	buf := []byte{0x01, 0x02, 0x03}

	pkt, err := NewBinaryEventPacket("/chat", "file:upload", 99, buf, "metadata")
	if err != nil {
		t.Fatalf("NewBinaryEventPacket error: %v", err)
	}

	result, err := EncodeBinary(pkt)
	if err != nil {
		t.Fatalf("EncodeBinary error: %v", err)
	}

	decoded, err := Decode(result.TextFrame)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if decoded.Namespace != "/chat" {
		t.Errorf("expected namespace /chat, got %s", decoded.Namespace)
	}
	if decoded.ID != 99 {
		t.Errorf("expected ack ID 99, got %d", decoded.ID)
	}
	if decoded.Attachments != 1 {
		t.Errorf("expected 1 attachment, got %d", decoded.Attachments)
	}
}

// --- containsBinary / hasBinaryValue tests ---

func TestContainsBinary(t *testing.T) {
	cases := []struct {
		name   string
		args   []interface{}
		expect bool
	}{
		{"no binary", []interface{}{"hello", 42, true}, false},
		{"direct binary", []interface{}{[]byte{0x01}}, true},
		{"nested in map", []interface{}{map[string]interface{}{"data": []byte{0x01}}}, true},
		{"nested in slice", []interface{}{[]interface{}{[]byte{0x01}}}, true},
		{"empty", []interface{}{}, false},
		{"nil values", []interface{}{nil, nil}, false},
		{"deeply nested", []interface{}{
			map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": []interface{}{
						map[string]interface{}{
							"data": []byte{0xFF},
						},
					},
				},
			},
		}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := containsBinary(tc.args)
			if got != tc.expect {
				t.Errorf("containsBinary(%v) = %v, want %v", tc.args, got, tc.expect)
			}
		})
	}
}

// --- Security limit tests ---

func TestDecodeRejectsExcessiveAttachments(t *testing.T) {
	_, err := Decode(`5999999999-["test",{"_placeholder":true,"num":0}]`)
	if err != ErrTooManyAttachments {
		t.Errorf("expected ErrTooManyAttachments, got %v", err)
	}
}

func TestDecodeAcceptsBoundaryAttachments(t *testing.T) {
	// MaxAttachments (100) should be accepted
	p, err := Decode(`5100-["test",{"_placeholder":true,"num":0}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Attachments != 100 {
		t.Errorf("expected 100 attachments, got %d", p.Attachments)
	}
}

func TestDecodeRejectsAttachmentsOverBoundary(t *testing.T) {
	_, err := Decode(`5101-["test",{"_placeholder":true,"num":0}]`)
	if err != ErrTooManyAttachments {
		t.Errorf("expected ErrTooManyAttachments, got %v", err)
	}
}

func TestDecodeRejectsDeepJSON(t *testing.T) {
	// Build 100-level nested array: [[[[...]]]]
	var deep string
	for i := 0; i < 100; i++ {
		deep += "["
	}
	for i := 0; i < 100; i++ {
		deep += "]"
	}
	_, err := Decode("2" + deep)
	if err != ErrJSONTooDeep {
		t.Errorf("expected ErrJSONTooDeep, got %v", err)
	}
}

func TestDecodeAcceptsReasonableDepth(t *testing.T) {
	// Build 30-level nested array (under the 32 limit)
	var nested string
	for i := 0; i < 30; i++ {
		nested += "["
	}
	nested += `"event"`
	for i := 0; i < 30; i++ {
		nested += "]"
	}
	p, err := Decode("2" + nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != PacketEvent {
		t.Errorf("expected EVENT type, got %d", p.Type)
	}
}

func TestCheckJSONDepthBoundary(t *testing.T) {
	// Exactly MaxJSONDepth (32) should pass
	var atLimit string
	for i := 0; i < MaxJSONDepth; i++ {
		atLimit += "["
	}
	for i := 0; i < MaxJSONDepth; i++ {
		atLimit += "]"
	}
	if err := checkJSONDepth([]byte(atLimit)); err != nil {
		t.Errorf("expected no error at depth %d, got %v", MaxJSONDepth, err)
	}

	// MaxJSONDepth + 1 (33) should fail
	var overLimit string
	for i := 0; i < MaxJSONDepth+1; i++ {
		overLimit += "["
	}
	for i := 0; i < MaxJSONDepth+1; i++ {
		overLimit += "]"
	}
	if err := checkJSONDepth([]byte(overLimit)); err != ErrJSONTooDeep {
		t.Errorf("expected ErrJSONTooDeep at depth %d, got %v", MaxJSONDepth+1, err)
	}
}
