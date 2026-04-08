package sio

import (
	"log"
	"testing"
)

func newTestManager() *ConnectionManager {
	return NewConnectionManager(log.Default())
}

func TestAdd_BasicConnection(t *testing.T) {
	m := newTestManager()

	info := m.Add("sid1", "", "")
	if info == nil {
		t.Fatal("Add returned nil")
	}
	if info.SID != "sid1" {
		t.Errorf("SID = %q, want %q", info.SID, "sid1")
	}
	if info.ConnectedAt <= 0 {
		t.Error("ConnectedAt should be positive")
	}
	if info.LastHeartbeat != info.ConnectedAt {
		t.Error("LastHeartbeat should equal ConnectedAt initially")
	}
	if info.ReconnectCount != 0 {
		t.Errorf("ReconnectCount = %d, want 0", info.ReconnectCount)
	}
	if m.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", m.ActiveCount())
	}
}

func TestAdd_WithUserID(t *testing.T) {
	m := newTestManager()

	info := m.Add("sid1", "user-abc", "")
	if info.UserID != "user-abc" {
		t.Errorf("UserID = %q, want %q", info.UserID, "user-abc")
	}
}

func TestAdd_ReconnectionDetection(t *testing.T) {
	m := newTestManager()

	// First connection
	info1 := m.Add("sid1", "user-abc", "")
	if info1.ReconnectCount != 0 {
		t.Fatalf("first connection ReconnectCount = %d, want 0", info1.ReconnectCount)
	}

	// Simulate disconnect (remove sid1) then reconnect with same user_id
	m.Remove("sid1")
	info2 := m.Add("sid2", "user-abc", "")
	if info2.ReconnectCount != 1 {
		t.Errorf("reconnection ReconnectCount = %d, want 1", info2.ReconnectCount)
	}

	// Third connection
	m.Remove("sid2")
	info3 := m.Add("sid3", "user-abc", "")
	if info3.ReconnectCount != 2 {
		t.Errorf("third connection ReconnectCount = %d, want 2", info3.ReconnectCount)
	}
}

func TestGet_Found(t *testing.T) {
	m := newTestManager()
	m.Add("sid1", "", "")

	info := m.Get("sid1")
	if info == nil {
		t.Fatal("Get returned nil for existing sid")
	}
	if info.SID != "sid1" {
		t.Errorf("SID = %q, want %q", info.SID, "sid1")
	}
}

func TestGet_NotFound(t *testing.T) {
	m := newTestManager()

	info := m.Get("nonexistent")
	if info != nil {
		t.Error("Get should return nil for unknown sid")
	}
}

func TestRemove_ExistingConnection(t *testing.T) {
	m := newTestManager()
	m.Add("sid1", "", "")

	info := m.Remove("sid1")
	if info == nil {
		t.Fatal("Remove returned nil for existing sid")
	}
	if m.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d, want 0 after remove", m.ActiveCount())
	}
}

func TestRemove_UnknownSID(t *testing.T) {
	m := newTestManager()

	info := m.Remove("nonexistent")
	if info != nil {
		t.Error("Remove should return nil for unknown sid")
	}
}

func TestActiveSIDs(t *testing.T) {
	m := newTestManager()
	m.Add("sid1", "", "")
	m.Add("sid2", "", "")

	sids := m.ActiveSIDs()
	if len(sids) != 2 {
		t.Errorf("len(ActiveSIDs) = %d, want 2", len(sids))
	}

	sidSet := make(map[string]bool)
	for _, s := range sids {
		sidSet[s] = true
	}
	if !sidSet["sid1"] || !sidSet["sid2"] {
		t.Errorf("ActiveSIDs = %v, want sid1 and sid2", sids)
	}
}

func TestGetPreviousSession(t *testing.T) {
	m := newTestManager()

	// No previous session
	if prev := m.GetPreviousSession("user-abc"); prev != nil {
		t.Error("expected nil for unknown user_id")
	}

	// Add and remove — user_sessions should persist
	info := m.Add("sid1", "user-abc", "")
	info.Rooms["grid:1"] = struct{}{}
	info.Rooms["grid:2"] = struct{}{}
	m.Remove("sid1")

	prev := m.GetPreviousSession("user-abc")
	if prev == nil {
		t.Fatal("expected previous session after remove")
	}
	if len(prev.Rooms) != 2 {
		t.Errorf("previous session rooms = %d, want 2", len(prev.Rooms))
	}
}

func TestRecordHeartbeat(t *testing.T) {
	m := newTestManager()
	m.Add("sid1", "", "")

	ok := m.RecordHeartbeat("sid1")
	if !ok {
		t.Error("RecordHeartbeat returned false for existing sid")
	}

	ok = m.RecordHeartbeat("nonexistent")
	if ok {
		t.Error("RecordHeartbeat returned true for unknown sid")
	}
}

func TestSetAndGetRooms(t *testing.T) {
	m := newTestManager()
	m.Add("sid1", "", "")

	m.SetRooms("sid1", []string{"room-a", "room-b"})
	rooms := m.GetRooms("sid1")
	if len(rooms) != 2 {
		t.Errorf("len(GetRooms) = %d, want 2", len(rooms))
	}

	// Replace rooms
	m.SetRooms("sid1", []string{"room-c"})
	rooms = m.GetRooms("sid1")
	if len(rooms) != 1 {
		t.Errorf("len(GetRooms) = %d, want 1 after replace", len(rooms))
	}
}

func TestGetRooms_UnknownSID(t *testing.T) {
	m := newTestManager()

	rooms := m.GetRooms("nonexistent")
	if rooms != nil {
		t.Errorf("GetRooms for unknown sid should return nil, got %v", rooms)
	}
}
