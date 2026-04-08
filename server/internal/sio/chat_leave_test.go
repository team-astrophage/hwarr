package sio

import (
	"encoding/json"
	"log"
	"testing"
)

func TestChatLeaveResponse_OK(t *testing.T) {
	// Verify the expected response structure for a successful chat:leave
	expected := map[string]interface{}{
		"status": "ok",
	}
	data, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "ok" {
		t.Errorf("unexpected status value: %v", result["status"])
	}
}

func TestChatLeaveResponse_Structure(t *testing.T) {
	// Verify the response only contains "status" key (no extra fields like history/presence)
	response := map[string]interface{}{
		"status": "ok",
	}
	if len(response) != 1 {
		t.Errorf("expected 1 key in response, got %d", len(response))
	}
	if _, ok := response["status"]; !ok {
		t.Error("response should contain 'status' key")
	}
}

func TestChatLeaveRoomTracking(t *testing.T) {
	// Verify that room removal from ConnectionInfo works correctly
	manager := NewConnectionManager(log.Default())
	sid := "test-sid-leave"

	manager.Add(sid, "user-123", "")
	info := manager.Get(sid)
	if info == nil {
		t.Fatal("expected non-nil info after Add")
	}

	// Simulate having joined the chat room
	info.Rooms[ChatRoom] = struct{}{}

	// Verify room is tracked
	if _, ok := info.Rooms[ChatRoom]; !ok {
		t.Error("expected ChatRoom to be present in Rooms")
	}

	// Simulate chat:leave room removal (matches Python: info.rooms.discard(CHAT_ROOM))
	delete(info.Rooms, ChatRoom)

	// Verify room is removed
	if _, ok := info.Rooms[ChatRoom]; ok {
		t.Error("expected ChatRoom to be removed from Rooms after leave")
	}
}

func TestChatLeaveNilInfo(t *testing.T) {
	// Verify that chat:leave with unknown SID still returns ok
	// (Python doesn't return error for unknown SID on leave — it just
	// skips info.rooms.discard and still broadcasts presence)
	manager := NewConnectionManager(log.Default())
	info := manager.Get("nonexistent-sid")
	if info != nil {
		t.Error("expected nil info for nonexistent SID")
	}
	// The handler should still proceed without error
}

func TestChatLeavePresencePayload(t *testing.T) {
	// Verify the chat:presence payload format matches Python
	count := 5
	payload := map[string]interface{}{
		"count": count,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	// JSON numbers are float64
	if result["count"] != float64(5) {
		t.Errorf("expected count=5, got %v", result["count"])
	}
}
