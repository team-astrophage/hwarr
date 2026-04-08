package sio

import (
	"encoding/json"
	"log"
	"testing"

	"github.com/homepy/hwarr/server/internal/auth"
	socketio "github.com/homeworldio/socketio-go"
)

// newTestHandler creates a Handler with a mock Socket.IO server for testing.
func newTestHandler() *Handler {
	sioServer := socketio.NewServer()
	// Set a no-op SendTo so broadcasts don't fail
	sioServer.SendTo = func(sid string, data string) error {
		return nil
	}
	logger := log.Default()
	ts := auth.NewTokenService("test-secret", 30)
	return NewHandler(sioServer, logger, ts, nil)
}

func TestHandleHeartbeat_ReturnsAck(t *testing.T) {
	h := newTestHandler()

	// Register a connection first
	h.manager.Add("test-hb-sid", "")

	// Call heartbeat with ts payload
	tsVal := 1000.0
	data, _ := json.Marshal(heartbeatData{TS: &tsVal})
	args := []json.RawMessage{data}

	result, err := h.handleHeartbeat("test-hb-sid", args...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}

	m, ok := result[0].(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if m["status"] != "ok" {
		t.Errorf("status = %v, want %q", m["status"], "ok")
	}
	if m["server_ts"] == nil {
		t.Error("server_ts should not be nil")
	}
	serverTS, ok := m["server_ts"].(float64)
	if !ok || serverTS <= 0 {
		t.Error("server_ts should be a positive float64")
	}
	if m["client_ts"] != tsVal {
		t.Errorf("client_ts = %v, want %v", m["client_ts"], tsVal)
	}
}

func TestHandleHeartbeat_UnknownSID(t *testing.T) {
	h := newTestHandler()

	result, err := h.handleHeartbeat("unknown-sid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}

	m, ok := result[0].(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if m["error"] != "unknown_sid" {
		t.Errorf("error = %v, want %q", m["error"], "unknown_sid")
	}
}

func TestHandleHeartbeat_NoData(t *testing.T) {
	h := newTestHandler()

	h.manager.Add("test-hb-sid-2", "")

	// Call heartbeat with no args (client sent no payload)
	result, err := h.handleHeartbeat("test-hb-sid-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}

	m, ok := result[0].(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if m["status"] != "ok" {
		t.Errorf("status = %v, want %q", m["status"], "ok")
	}
	if m["client_ts"] != nil {
		t.Errorf("client_ts = %v, want nil", m["client_ts"])
	}
}

func TestHandleHeartbeat_UpdatesTimestamp(t *testing.T) {
	h := newTestHandler()

	info := h.manager.Add("test-hb-sid-3", "")
	originalTS := info.LastHeartbeat

	// Manually set last heartbeat to an old value
	info.LastHeartbeat = originalTS - 100

	// Call heartbeat
	_, err := h.handleHeartbeat("test-hb-sid-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify heartbeat was recorded (timestamp updated)
	updatedInfo := h.manager.Get("test-hb-sid-3")
	if updatedInfo.LastHeartbeat <= (originalTS - 100) {
		t.Error("LastHeartbeat should have been updated")
	}
}

func TestHandleHeartbeat_RegisteredViaHandler(t *testing.T) {
	// Verify that the heartbeat handler is registered on the Socket.IO server
	sioServer := socketio.NewServer()
	sioServer.SendTo = func(sid string, data string) error {
		return nil
	}
	_ = NewHandler(sioServer, log.Default(), auth.NewTokenService("test-secret", 30), nil)

	ns := sioServer.GetNamespace("/")
	if ns == nil {
		t.Fatal("default namespace not found")
	}
	if !ns.HasEvent("heartbeat") {
		t.Error("heartbeat event handler not registered on default namespace")
	}
}

func TestHandleHeartbeat_EmptyObject(t *testing.T) {
	h := newTestHandler()

	h.manager.Add("test-hb-sid-4", "")

	// Client sends empty object {} (no ts field)
	data := json.RawMessage(`{}`)
	result, err := h.handleHeartbeat("test-hb-sid-4", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result[0].(map[string]interface{})
	if m["status"] != "ok" {
		t.Errorf("status = %v, want %q", m["status"], "ok")
	}
	if m["client_ts"] != nil {
		t.Errorf("client_ts = %v, want nil (no ts in payload)", m["client_ts"])
	}
}
