package engineio

import (
	"fmt"
	"strings"
)

// Engine.IO v4 packet types
const (
	PacketOpen    = '0'
	PacketClose   = '1'
	PacketPing    = '2'
	PacketPong    = '3'
	PacketMessage = '4'
	PacketUpgrade = '5'
	PacketNoop    = '6'
)

// PacketTypeString returns the human-readable name of an Engine.IO packet type.
func PacketTypeString(t byte) string {
	switch t {
	case PacketOpen:
		return "OPEN"
	case PacketClose:
		return "CLOSE"
	case PacketPing:
		return "PING"
	case PacketPong:
		return "PONG"
	case PacketMessage:
		return "MESSAGE"
	case PacketUpgrade:
		return "UPGRADE"
	case PacketNoop:
		return "NOOP"
	default:
		return fmt.Sprintf("UNKNOWN(%c)", t)
	}
}

// Packet represents an Engine.IO v4 packet.
type Packet struct {
	Type byte   // one of PacketOpen..PacketNoop
	Data []byte // payload (nil for control packets)
}

// Encode serializes a packet to its wire format for polling transport.
// Format: <type_char>[data]
func (p *Packet) Encode() string {
	if len(p.Data) == 0 {
		return string(p.Type)
	}
	return string(p.Type) + string(p.Data)
}

// DecodePacket parses a single Engine.IO packet from a string.
func DecodePacket(data string) (*Packet, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("engineio: empty packet")
	}

	t := data[0]
	if t < '0' || t > '6' {
		return nil, fmt.Errorf("engineio: invalid packet type: %c", t)
	}

	p := &Packet{Type: t}
	if len(data) > 1 {
		p.Data = []byte(data[1:])
	}
	return p, nil
}

// EncodePayload encodes multiple packets into a single polling payload.
// Engine.IO v4 uses the record separator (\x1e) to delimit packets in a payload.
func EncodePayload(packets []*Packet) string {
	if len(packets) == 0 {
		return ""
	}
	if len(packets) == 1 {
		return packets[0].Encode()
	}

	parts := make([]string, len(packets))
	for i, p := range packets {
		parts[i] = p.Encode()
	}
	return strings.Join(parts, "\x1e")
}

// DecodePayload parses a polling payload into individual packets.
func DecodePayload(data string) ([]*Packet, error) {
	if len(data) == 0 {
		return nil, nil
	}

	parts := strings.Split(data, "\x1e")
	packets := make([]*Packet, 0, len(parts))
	for _, part := range parts {
		p, err := DecodePacket(part)
		if err != nil {
			return nil, err
		}
		packets = append(packets, p)
	}
	return packets, nil
}
