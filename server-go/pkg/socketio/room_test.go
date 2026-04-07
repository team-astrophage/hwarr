package socketio

import (
	"fmt"
	"sort"
	"sync"
	"testing"
)

func TestRoomManager_Join(t *testing.T) {
	rm := NewRoomManager()
	rm.Join("sid-1", "room-a")

	if !rm.InRoom("sid-1", "room-a") {
		t.Error("expected sid-1 to be in room-a")
	}
	if rm.RoomCount("room-a") != 1 {
		t.Errorf("expected room-a count 1, got %d", rm.RoomCount("room-a"))
	}
}

func TestRoomManager_JoinMultipleRooms(t *testing.T) {
	rm := NewRoomManager()
	rm.Join("sid-1", "room-a")
	rm.Join("sid-1", "room-b")

	rooms := rm.Rooms("sid-1")
	sort.Strings(rooms)
	if len(rooms) != 2 || rooms[0] != "room-a" || rooms[1] != "room-b" {
		t.Errorf("expected [room-a room-b], got %v", rooms)
	}
}

func TestRoomManager_JoinMultipleSids(t *testing.T) {
	rm := NewRoomManager()
	rm.Join("sid-1", "room-a")
	rm.Join("sid-2", "room-a")

	members := rm.Members("room-a")
	sort.Strings(members)
	if len(members) != 2 || members[0] != "sid-1" || members[1] != "sid-2" {
		t.Errorf("expected [sid-1 sid-2], got %v", members)
	}
}

func TestRoomManager_JoinIdempotent(t *testing.T) {
	rm := NewRoomManager()
	rm.Join("sid-1", "room-a")
	rm.Join("sid-1", "room-a")

	if rm.RoomCount("room-a") != 1 {
		t.Errorf("expected room-a count 1 after double join, got %d", rm.RoomCount("room-a"))
	}
}

func TestRoomManager_Leave(t *testing.T) {
	rm := NewRoomManager()
	rm.Join("sid-1", "room-a")
	rm.Leave("sid-1", "room-a")

	if rm.InRoom("sid-1", "room-a") {
		t.Error("expected sid-1 to not be in room-a after leave")
	}
	if rm.RoomCount("room-a") != 0 {
		t.Errorf("expected room-a count 0, got %d", rm.RoomCount("room-a"))
	}
	// Room should be cleaned up
	if rm.Members("room-a") != nil {
		t.Error("expected nil members for empty room")
	}
}

func TestRoomManager_LeaveNonexistent(t *testing.T) {
	rm := NewRoomManager()
	// Should not panic
	rm.Leave("sid-1", "room-a")
}

func TestRoomManager_LeavePreservesOtherRooms(t *testing.T) {
	rm := NewRoomManager()
	rm.Join("sid-1", "room-a")
	rm.Join("sid-1", "room-b")
	rm.Leave("sid-1", "room-a")

	if rm.InRoom("sid-1", "room-a") {
		t.Error("expected sid-1 to not be in room-a")
	}
	if !rm.InRoom("sid-1", "room-b") {
		t.Error("expected sid-1 to still be in room-b")
	}
}

func TestRoomManager_LeaveAll(t *testing.T) {
	rm := NewRoomManager()
	rm.Join("sid-1", "room-a")
	rm.Join("sid-1", "room-b")
	rm.Join("sid-1", "room-c")
	rm.Join("sid-2", "room-a") // another user in room-a

	rm.LeaveAll("sid-1")

	if rm.InRoom("sid-1", "room-a") || rm.InRoom("sid-1", "room-b") || rm.InRoom("sid-1", "room-c") {
		t.Error("expected sid-1 to be removed from all rooms")
	}
	if rm.Rooms("sid-1") != nil {
		t.Error("expected nil rooms for sid-1 after LeaveAll")
	}

	// sid-2 should still be in room-a
	if !rm.InRoom("sid-2", "room-a") {
		t.Error("expected sid-2 to still be in room-a")
	}
	if rm.RoomCount("room-a") != 1 {
		t.Errorf("expected room-a count 1, got %d", rm.RoomCount("room-a"))
	}
}

func TestRoomManager_LeaveAllNonexistent(t *testing.T) {
	rm := NewRoomManager()
	// Should not panic
	rm.LeaveAll("sid-1")
}

func TestRoomManager_LeaveAllCleansUpEmptyRooms(t *testing.T) {
	rm := NewRoomManager()
	rm.Join("sid-1", "room-a")
	rm.LeaveAll("sid-1")

	if rm.Members("room-a") != nil {
		t.Error("expected room-a to be cleaned up after LeaveAll")
	}
}

func TestRoomManager_Members_EmptyRoom(t *testing.T) {
	rm := NewRoomManager()
	if rm.Members("nonexistent") != nil {
		t.Error("expected nil for nonexistent room")
	}
}

func TestRoomManager_Rooms_UnknownSid(t *testing.T) {
	rm := NewRoomManager()
	if rm.Rooms("unknown") != nil {
		t.Error("expected nil for unknown sid")
	}
}

func TestRoomManager_InRoom_False(t *testing.T) {
	rm := NewRoomManager()
	if rm.InRoom("sid-1", "room-a") {
		t.Error("expected false for non-member")
	}
}

func TestRoomManager_ConcurrentAccess(t *testing.T) {
	rm := NewRoomManager()
	var wg sync.WaitGroup
	const goroutines = 100

	// Concurrent joins
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sid := fmt.Sprintf("sid-%d", n)
			rm.Join(sid, "shared-room")
			rm.Join(sid, fmt.Sprintf("private-%d", n))
		}(i)
	}
	wg.Wait()

	if rm.RoomCount("shared-room") != goroutines {
		t.Errorf("expected %d members in shared-room, got %d", goroutines, rm.RoomCount("shared-room"))
	}

	// Concurrent leaves
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sid := fmt.Sprintf("sid-%d", n)
			rm.Leave(sid, "shared-room")
		}(i)
	}
	wg.Wait()

	if rm.RoomCount("shared-room") != 0 {
		t.Errorf("expected 0 members in shared-room after leaves, got %d", rm.RoomCount("shared-room"))
	}
}

func TestRoomManager_ConcurrentLeaveAll(t *testing.T) {
	rm := NewRoomManager()
	var wg sync.WaitGroup
	const goroutines = 100

	// Setup: each sid in multiple rooms
	for i := 0; i < goroutines; i++ {
		sid := fmt.Sprintf("sid-%d", i)
		rm.Join(sid, "room-a")
		rm.Join(sid, "room-b")
		rm.Join(sid, "room-c")
	}

	// Concurrent LeaveAll
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			rm.LeaveAll(fmt.Sprintf("sid-%d", n))
		}(i)
	}
	wg.Wait()

	if rm.RoomCount("room-a") != 0 {
		t.Errorf("expected 0 in room-a, got %d", rm.RoomCount("room-a"))
	}
	if rm.RoomCount("room-b") != 0 {
		t.Errorf("expected 0 in room-b, got %d", rm.RoomCount("room-b"))
	}
}

func TestRoomManager_ConcurrentMixed(t *testing.T) {
	rm := NewRoomManager()
	var wg sync.WaitGroup

	// Mix of joins, leaves, reads concurrently
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sid := fmt.Sprintf("sid-%d", n%10)
			room := fmt.Sprintf("room-%d", n%5)
			switch n % 4 {
			case 0:
				rm.Join(sid, room)
			case 1:
				rm.Leave(sid, room)
			case 2:
				_ = rm.Members(room)
			case 3:
				_ = rm.Rooms(sid)
			}
		}(i)
	}
	wg.Wait()
	// No panics or data races = pass
}
