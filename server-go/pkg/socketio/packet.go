package socketio

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Packet types per Socket.IO v4 protocol
const (
	PacketConnect      = 0
	PacketDisconnect   = 1
	PacketEvent        = 2
	PacketAck          = 3
	PacketConnectError = 4
	PacketBinaryEvent  = 5
	PacketBinaryAck    = 6
)

// PacketTypeString returns the human-readable name of a packet type.
func PacketTypeString(t int) string {
	switch t {
	case PacketConnect:
		return "CONNECT"
	case PacketDisconnect:
		return "DISCONNECT"
	case PacketEvent:
		return "EVENT"
	case PacketAck:
		return "ACK"
	case PacketConnectError:
		return "CONNECT_ERROR"
	case PacketBinaryEvent:
		return "BINARY_EVENT"
	case PacketBinaryAck:
		return "BINARY_ACK"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", t)
	}
}

// Packet represents a decoded Socket.IO v4 packet.
type Packet struct {
	Type      int             // packet type (0-6)
	Namespace string          // namespace path ("/" for default)
	ID        int             // acknowledgement id, -1 if absent
	Data      json.RawMessage // JSON payload (array for EVENT/ACK, object for CONNECT)
	// Binary fields (for BINARY_EVENT / BINARY_ACK)
	Attachments    int      // number of expected binary attachments
	BinaryPayloads [][]byte // collected binary buffers
}

// Common errors
var (
	ErrEmptyPacket     = errors.New("socketio: empty packet")
	ErrInvalidType     = errors.New("socketio: invalid packet type")
	ErrMalformedPacket = errors.New("socketio: malformed packet")
	ErrInvalidJSON     = errors.New("socketio: invalid JSON data")
	ErrEventNoArray    = errors.New("socketio: EVENT/ACK data must be a JSON array")
	ErrEventNoName     = errors.New("socketio: EVENT packet must have event name as first element")
)

// Decode parses a raw Socket.IO v4 packet string into a Packet struct.
//
// Socket.IO v4 packet format:
//
//	<type>[<attachments>-][<namespace>,][<ack id>][<data>]
//
// Examples:
//
//	"2[\"hello\",\"world\"]"                    → EVENT on / with data ["hello","world"]
//	"2/chat,[\"hello\"]"                        → EVENT on /chat
//	"21[\"hello\"]"                             → EVENT on / with ack id 1
//	"2/chat,1[\"hello\"]"                       → EVENT on /chat with ack id 1
//	"3/chat,1[\"result\"]"                      → ACK on /chat with ack id 1
//	"51-[\"binary\",{\"_placeholder\":true}]"   → BINARY_EVENT with 1 attachment
func Decode(data string) (*Packet, error) {
	if len(data) == 0 {
		return nil, ErrEmptyPacket
	}

	p := &Packet{
		Namespace: "/",
		ID:        -1,
	}

	// 1. Parse packet type (first character)
	typeChar := int(data[0] - '0')
	if typeChar < 0 || typeChar > 6 {
		return nil, ErrInvalidType
	}
	p.Type = typeChar

	pos := 1

	// 2. Parse binary attachment count (for type 5, 6)
	if p.Type == PacketBinaryEvent || p.Type == PacketBinaryAck {
		dashIdx := strings.IndexByte(data[pos:], '-')
		if dashIdx < 0 {
			return nil, ErrMalformedPacket
		}
		count, err := strconv.Atoi(data[pos : pos+dashIdx])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid attachment count", ErrMalformedPacket)
		}
		p.Attachments = count
		p.BinaryPayloads = make([][]byte, 0, count)
		pos += dashIdx + 1
	}

	// 3. Parse namespace (starts with /, ends with ,)
	if pos < len(data) && data[pos] == '/' {
		commaIdx := strings.IndexByte(data[pos:], ',')
		if commaIdx < 0 {
			// Namespace with no comma means the rest is the namespace and there's no more data
			p.Namespace = data[pos:]
			return p, nil
		}
		p.Namespace = data[pos : pos+commaIdx]
		pos += commaIdx + 1
	}

	// 4. Parse acknowledgement ID (sequence of digits before [ or end)
	idStart := pos
	for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
		pos++
	}
	if pos > idStart {
		id, err := strconv.Atoi(data[idStart:pos])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid ack id", ErrMalformedPacket)
		}
		p.ID = id
	}

	// 5. Parse data (remaining string, should be valid JSON)
	if pos < len(data) {
		raw := json.RawMessage(data[pos:])
		if !json.Valid(raw) {
			return nil, ErrInvalidJSON
		}
		p.Data = raw
	}

	return p, nil
}

// Encode serializes a Packet into its Socket.IO v4 wire format string.
func Encode(p *Packet) (string, error) {
	var b strings.Builder

	// 1. Type
	b.WriteByte(byte('0' + p.Type))

	// 2. Binary attachment count
	if p.Type == PacketBinaryEvent || p.Type == PacketBinaryAck {
		b.WriteString(strconv.Itoa(p.Attachments))
		b.WriteByte('-')
	}

	// 3. Namespace (omit if default "/")
	if p.Namespace != "/" && p.Namespace != "" {
		b.WriteString(p.Namespace)
		b.WriteByte(',')
	}

	// 4. Ack ID
	if p.ID >= 0 {
		b.WriteString(strconv.Itoa(p.ID))
	}

	// 5. Data
	if p.Data != nil {
		b.Write(p.Data)
	}

	return b.String(), nil
}

// EventName extracts the event name from an EVENT or BINARY_EVENT packet.
// The data must be a JSON array with the first element being a string.
func (p *Packet) EventName() (string, error) {
	if p.Type != PacketEvent && p.Type != PacketBinaryEvent {
		return "", fmt.Errorf("socketio: EventName called on %s packet", PacketTypeString(p.Type))
	}
	if p.Data == nil {
		return "", ErrEventNoName
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(p.Data, &arr); err != nil {
		return "", ErrEventNoArray
	}
	if len(arr) == 0 {
		return "", ErrEventNoName
	}

	var name string
	if err := json.Unmarshal(arr[0], &name); err != nil {
		return "", ErrEventNoName
	}
	return name, nil
}

// EventArgs extracts the event arguments from an EVENT or BINARY_EVENT packet.
// Returns all elements after the event name.
func (p *Packet) EventArgs() ([]json.RawMessage, error) {
	if p.Type != PacketEvent && p.Type != PacketBinaryEvent {
		return nil, fmt.Errorf("socketio: EventArgs called on %s packet", PacketTypeString(p.Type))
	}
	if p.Data == nil {
		return nil, nil
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(p.Data, &arr); err != nil {
		return nil, ErrEventNoArray
	}
	if len(arr) <= 1 {
		return nil, nil
	}
	return arr[1:], nil
}

// AckData extracts the ACK response data from an ACK packet.
// Returns the array elements as raw JSON messages.
func (p *Packet) AckData() ([]json.RawMessage, error) {
	if p.Type != PacketAck && p.Type != PacketBinaryAck {
		return nil, fmt.Errorf("socketio: AckData called on %s packet", PacketTypeString(p.Type))
	}
	if p.Data == nil {
		return nil, nil
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(p.Data, &arr); err != nil {
		return nil, ErrEventNoArray
	}
	return arr, nil
}

// AddAttachment appends a binary payload to the packet's collected buffers.
// Returns true when all expected attachments have been received.
func (p *Packet) AddAttachment(data []byte) bool {
	if p.Type != PacketBinaryEvent && p.Type != PacketBinaryAck {
		return false
	}
	p.BinaryPayloads = append(p.BinaryPayloads, data)
	return len(p.BinaryPayloads) >= p.Attachments
}

// IsComplete returns true if the binary packet has received all expected
// attachments, or if the packet is not a binary type.
func (p *Packet) IsComplete() bool {
	if p.Type != PacketBinaryEvent && p.Type != PacketBinaryAck {
		return true
	}
	return len(p.BinaryPayloads) >= p.Attachments
}

// ReconstructBinary replaces placeholder objects in the JSON data with
// the corresponding binary payloads. Placeholders have the format:
//
//	{"_placeholder":true,"num":N}
//
// This is called after all attachments are collected to produce the
// final packet data with binary buffers inline.
func (p *Packet) ReconstructBinary() error {
	if p.Type != PacketBinaryEvent && p.Type != PacketBinaryAck {
		return nil
	}
	if !p.IsComplete() {
		return fmt.Errorf("socketio: cannot reconstruct binary, %d/%d attachments received",
			len(p.BinaryPayloads), p.Attachments)
	}
	if p.Data == nil {
		return nil
	}

	var parsed interface{}
	if err := json.Unmarshal(p.Data, &parsed); err != nil {
		return fmt.Errorf("socketio: failed to parse data for binary reconstruction: %w", err)
	}

	reconstructed := reconstructNode(parsed, p.BinaryPayloads)

	data, err := json.Marshal(reconstructed)
	if err != nil {
		return fmt.Errorf("socketio: failed to marshal reconstructed data: %w", err)
	}
	p.Data = json.RawMessage(data)
	return nil
}

// ReconstructedData returns the packet data as a parsed interface{} tree
// with binary placeholders replaced by []byte values from BinaryPayloads.
// For non-binary packets, it simply unmarshals the JSON data.
func (p *Packet) ReconstructedData() (interface{}, error) {
	if p.Data == nil {
		return nil, nil
	}

	var parsed interface{}
	if err := json.Unmarshal(p.Data, &parsed); err != nil {
		return nil, fmt.Errorf("socketio: failed to parse data: %w", err)
	}

	if (p.Type == PacketBinaryEvent || p.Type == PacketBinaryAck) && p.IsComplete() {
		return reconstructNode(parsed, p.BinaryPayloads), nil
	}
	return parsed, nil
}

// placeholder represents a Socket.IO binary placeholder object.
type placeholder struct {
	Placeholder bool `json:"_placeholder"`
	Num         int  `json:"num"`
}

// isPlaceholder checks if a map represents a binary placeholder.
// Returns the attachment index and true if it is a placeholder.
func isPlaceholder(m map[string]interface{}) (int, bool) {
	ph, ok := m["_placeholder"]
	if !ok {
		return 0, false
	}
	phBool, ok := ph.(bool)
	if !ok || !phBool {
		return 0, false
	}
	num, ok := m["num"]
	if !ok {
		return 0, false
	}
	// JSON numbers are float64 when unmarshalled into interface{}
	numFloat, ok := num.(float64)
	if !ok {
		return 0, false
	}
	return int(numFloat), true
}

// reconstructNode recursively walks a parsed JSON tree and replaces
// placeholder objects with the corresponding []byte attachment.
func reconstructNode(node interface{}, attachments [][]byte) interface{} {
	switch v := node.(type) {
	case map[string]interface{}:
		if idx, ok := isPlaceholder(v); ok {
			if idx >= 0 && idx < len(attachments) {
				return attachments[idx]
			}
			// Out-of-range placeholder — leave as-is
			return v
		}
		// Recurse into map values
		for key, val := range v {
			v[key] = reconstructNode(val, attachments)
		}
		return v
	case []interface{}:
		for i, item := range v {
			v[i] = reconstructNode(item, attachments)
		}
		return v
	default:
		return node
	}
}

// DeconstructBinary scans the packet data for []byte values and replaces
// them with placeholder objects {"_placeholder":true,"num":N}, collecting
// the binary buffers into BinaryPayloads. This converts an EVENT packet
// into a BINARY_EVENT (or ACK into BINARY_ACK) if binary data is present.
//
// This is used for server-to-client encoding when binary data is involved.
// After calling DeconstructBinary, use EncodeBinary to serialize the packet
// into its text frame + binary attachment frames.
func (p *Packet) DeconstructBinary() error {
	if p.Data == nil {
		return nil
	}

	// Parse the JSON data into an interface{} tree
	var parsed interface{}
	if err := json.Unmarshal(p.Data, &parsed); err != nil {
		return fmt.Errorf("socketio: failed to parse data for binary deconstruction: %w", err)
	}

	// Walk the tree, replacing []byte with placeholders and collecting buffers
	buffers := make([][]byte, 0)
	replaced := deconstructNode(parsed, &buffers)

	if len(buffers) == 0 {
		// No binary data found — nothing to do
		return nil
	}

	// Re-marshal the modified tree
	data, err := json.Marshal(replaced)
	if err != nil {
		return fmt.Errorf("socketio: failed to marshal deconstructed data: %w", err)
	}

	p.Data = json.RawMessage(data)
	p.BinaryPayloads = buffers
	p.Attachments = len(buffers)

	// Upgrade packet type to binary variant
	switch p.Type {
	case PacketEvent:
		p.Type = PacketBinaryEvent
	case PacketAck:
		p.Type = PacketBinaryAck
	}

	return nil
}

// deconstructNode recursively walks a parsed JSON/interface{} tree and replaces
// []byte values with placeholder objects {"_placeholder":true,"num":N}.
// The binary buffers are collected into the provided slice.
func deconstructNode(node interface{}, buffers *[][]byte) interface{} {
	switch v := node.(type) {
	case []byte:
		idx := len(*buffers)
		*buffers = append(*buffers, v)
		return map[string]interface{}{
			"_placeholder": true,
			"num":          idx,
		}
	case map[string]interface{}:
		for key, val := range v {
			v[key] = deconstructNode(val, buffers)
		}
		return v
	case []interface{}:
		for i, item := range v {
			v[i] = deconstructNode(item, buffers)
		}
		return v
	default:
		return node
	}
}

// EncodedBinaryPacket holds the result of encoding a binary Socket.IO packet.
// The TextFrame is the BINARY_EVENT/BINARY_ACK packet string (with placeholders),
// and BinaryFrames are the raw binary attachment buffers to send as subsequent
// binary WebSocket frames.
type EncodedBinaryPacket struct {
	TextFrame    string   // e.g., `51-["event",{"_placeholder":true,"num":0}]`
	BinaryFrames [][]byte // binary attachment payloads, in order
}

// EncodeBinary serializes a packet that may contain binary data into its wire
// format. If the packet has no binary attachments (regular EVENT/ACK), this
// returns a single text frame with no binary frames.
//
// For BINARY_EVENT / BINARY_ACK packets, the result contains:
//   - TextFrame: the encoded packet string with placeholder objects
//   - BinaryFrames: the binary attachment buffers in order
//
// Note: Call DeconstructBinary first if the packet was built with []byte args
// using NewEventPacket or NewAckPacket. If the packet already has
// Type=PacketBinaryEvent with placeholders in Data, this just encodes it.
func EncodeBinary(p *Packet) (*EncodedBinaryPacket, error) {
	textFrame, err := Encode(p)
	if err != nil {
		return nil, err
	}

	result := &EncodedBinaryPacket{
		TextFrame: textFrame,
	}

	if (p.Type == PacketBinaryEvent || p.Type == PacketBinaryAck) && len(p.BinaryPayloads) > 0 {
		result.BinaryFrames = p.BinaryPayloads
	}

	return result, nil
}

// NewBinaryEventPacket creates an EVENT packet with the given namespace, event
// name, and arguments, then deconstructs any []byte arguments into binary
// placeholders. If any binary data is found, the packet type is automatically
// upgraded to BINARY_EVENT.
//
// This is a convenience function that combines NewEventPacket + DeconstructBinary.
func NewBinaryEventPacket(namespace, event string, ackID int, args ...interface{}) (*Packet, error) {
	// Check if any arg contains binary data
	hasBinary := containsBinary(args)
	if !hasBinary {
		// No binary data — use regular event packet
		return NewEventPacket(namespace, event, ackID, args...)
	}

	// Build the array: [eventName, arg1, arg2, ...]
	arr := make([]interface{}, 0, 1+len(args))
	arr = append(arr, event)
	arr = append(arr, args...)

	// Walk the tree to replace []byte with placeholders
	buffers := make([][]byte, 0)
	var deconstructed []interface{}
	for _, item := range arr {
		deconstructed = append(deconstructed, deconstructNode(item, &buffers))
	}

	data, err := json.Marshal(deconstructed)
	if err != nil {
		return nil, fmt.Errorf("socketio: failed to marshal binary event data: %w", err)
	}

	if namespace == "" {
		namespace = "/"
	}

	return &Packet{
		Type:           PacketBinaryEvent,
		Namespace:      namespace,
		ID:             ackID,
		Data:           json.RawMessage(data),
		Attachments:    len(buffers),
		BinaryPayloads: buffers,
	}, nil
}

// NewBinaryAckPacket creates an ACK packet with the given namespace, ack ID,
// and data, then deconstructs any []byte values into binary placeholders.
// If any binary data is found, the packet type is automatically upgraded
// to BINARY_ACK.
func NewBinaryAckPacket(namespace string, ackID int, args ...interface{}) (*Packet, error) {
	hasBinary := containsBinary(args)
	if !hasBinary {
		return NewAckPacket(namespace, ackID, args...)
	}

	// Walk args to replace []byte with placeholders
	buffers := make([][]byte, 0)
	var deconstructed []interface{}
	for _, item := range args {
		deconstructed = append(deconstructed, deconstructNode(item, &buffers))
	}

	data, err := json.Marshal(deconstructed)
	if err != nil {
		return nil, fmt.Errorf("socketio: failed to marshal binary ack data: %w", err)
	}

	if namespace == "" {
		namespace = "/"
	}

	return &Packet{
		Type:           PacketBinaryAck,
		Namespace:      namespace,
		ID:             ackID,
		Data:           json.RawMessage(data),
		Attachments:    len(buffers),
		BinaryPayloads: buffers,
	}, nil
}

// containsBinary recursively checks if any value in the tree is a []byte.
func containsBinary(args []interface{}) bool {
	for _, arg := range args {
		if hasBinaryValue(arg) {
			return true
		}
	}
	return false
}

// hasBinaryValue recursively checks if a single value contains []byte data.
func hasBinaryValue(v interface{}) bool {
	switch val := v.(type) {
	case []byte:
		return true
	case map[string]interface{}:
		for _, item := range val {
			if hasBinaryValue(item) {
				return true
			}
		}
	case []interface{}:
		for _, item := range val {
			if hasBinaryValue(item) {
				return true
			}
		}
	}
	return false
}

// NewEventPacket creates an EVENT packet with the given namespace, event name,
// and arguments. If ackID >= 0, it is set as the packet's acknowledgement ID.
func NewEventPacket(namespace, event string, ackID int, args ...interface{}) (*Packet, error) {
	// Build the array: [eventName, arg1, arg2, ...]
	arr := make([]interface{}, 0, 1+len(args))
	arr = append(arr, event)
	arr = append(arr, args...)

	data, err := json.Marshal(arr)
	if err != nil {
		return nil, fmt.Errorf("socketio: failed to marshal event data: %w", err)
	}

	if namespace == "" {
		namespace = "/"
	}

	return &Packet{
		Type:      PacketEvent,
		Namespace: namespace,
		ID:        ackID,
		Data:      json.RawMessage(data),
	}, nil
}

// NewConnectPacket creates a CONNECT response packet (sent server→client on
// successful connection). The data payload contains {"sid": sid}.
func NewConnectPacket(namespace, sid string) (*Packet, error) {
	if namespace == "" {
		namespace = "/"
	}
	data, err := json.Marshal(map[string]string{"sid": sid})
	if err != nil {
		return nil, fmt.Errorf("socketio: failed to marshal connect data: %w", err)
	}
	return &Packet{
		Type:      PacketConnect,
		Namespace: namespace,
		ID:        -1,
		Data:      json.RawMessage(data),
	}, nil
}

// NewConnectErrorPacket creates a CONNECT_ERROR packet sent when the server
// rejects a namespace connection. The data payload is {"message": message}.
func NewConnectErrorPacket(namespace, message string) (*Packet, error) {
	if namespace == "" {
		namespace = "/"
	}
	data, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		return nil, fmt.Errorf("socketio: failed to marshal connect error data: %w", err)
	}
	return &Packet{
		Type:      PacketConnectError,
		Namespace: namespace,
		ID:        -1,
		Data:      json.RawMessage(data),
	}, nil
}

// NewAckPacket creates an ACK packet with the given namespace, ack ID, and data.
func NewAckPacket(namespace string, ackID int, args ...interface{}) (*Packet, error) {
	var data json.RawMessage
	if len(args) > 0 {
		d, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("socketio: failed to marshal ack data: %w", err)
		}
		data = json.RawMessage(d)
	}

	if namespace == "" {
		namespace = "/"
	}

	return &Packet{
		Type:      PacketAck,
		Namespace: namespace,
		ID:        ackID,
		Data:      data,
	}, nil
}
