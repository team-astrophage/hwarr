package engineio

import (
	"testing"
)

func TestPacketEncode(t *testing.T) {
	tests := []struct {
		name   string
		packet Packet
		want   string
	}{
		{"open", Packet{Type: PacketOpen, Data: []byte(`{"sid":"abc"}`)}, `0{"sid":"abc"}`},
		{"close", Packet{Type: PacketClose}, "1"},
		{"ping", Packet{Type: PacketPing}, "2"},
		{"ping with data", Packet{Type: PacketPing, Data: []byte("probe")}, "2probe"},
		{"pong", Packet{Type: PacketPong}, "3"},
		{"message", Packet{Type: PacketMessage, Data: []byte("hello")}, "4hello"},
		{"noop", Packet{Type: PacketNoop}, "6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.packet.Encode()
			if got != tt.want {
				t.Errorf("Encode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodePacket(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantT   byte
		wantD   string
		wantErr bool
	}{
		{"open", `0{"sid":"abc"}`, PacketOpen, `{"sid":"abc"}`, false},
		{"close", "1", PacketClose, "", false},
		{"ping", "2", PacketPing, "", false},
		{"ping with data", "2probe", PacketPing, "probe", false},
		{"pong", "3", PacketPong, "", false},
		{"message", "4hello", PacketMessage, "hello", false},
		{"noop", "6", PacketNoop, "", false},
		{"empty", "", 0, "", true},
		{"invalid type", "9test", 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := DecodePacket(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Type != tt.wantT {
				t.Errorf("Type = %c, want %c", p.Type, tt.wantT)
			}
			gotData := ""
			if p.Data != nil {
				gotData = string(p.Data)
			}
			if gotData != tt.wantD {
				t.Errorf("Data = %q, want %q", gotData, tt.wantD)
			}
		})
	}
}

func TestEncodeDecodePayload(t *testing.T) {
	packets := []*Packet{
		{Type: PacketMessage, Data: []byte("hello")},
		{Type: PacketMessage, Data: []byte("world")},
	}

	payload := EncodePayload(packets)
	// Should be separated by \x1e
	expected := "4hello\x1e4world"
	if payload != expected {
		t.Errorf("EncodePayload = %q, want %q", payload, expected)
	}

	decoded, err := DecodePayload(payload)
	if err != nil {
		t.Fatalf("DecodePayload error: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("decoded %d packets, want 2", len(decoded))
	}
	if string(decoded[0].Data) != "hello" {
		t.Errorf("decoded[0].Data = %q, want %q", decoded[0].Data, "hello")
	}
	if string(decoded[1].Data) != "world" {
		t.Errorf("decoded[1].Data = %q, want %q", decoded[1].Data, "world")
	}
}

func TestEncodePayloadSingle(t *testing.T) {
	packets := []*Packet{{Type: PacketNoop}}
	payload := EncodePayload(packets)
	if payload != "6" {
		t.Errorf("EncodePayload single = %q, want %q", payload, "6")
	}
}

func TestDecodePayloadEmpty(t *testing.T) {
	packets, err := DecodePayload("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if packets != nil {
		t.Errorf("expected nil, got %v", packets)
	}
}
