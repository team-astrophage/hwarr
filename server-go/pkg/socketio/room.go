package socketio

import "sync"

// RoomManager manages Socket.IO room memberships.
// Each room is a set of session IDs, and each session tracks which rooms
// it belongs to for efficient LeaveAll on disconnect.
//
// Thread-safe: all methods can be called concurrently.
type RoomManager struct {
	mu sync.RWMutex
	// rooms maps room name → set of sids
	rooms map[string]map[string]struct{}
	// sids maps sid → set of room names (reverse index)
	sids map[string]map[string]struct{}
}

// NewRoomManager creates a new empty RoomManager.
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]map[string]struct{}),
		sids:  make(map[string]map[string]struct{}),
	}
}

// Join adds a session to a room. If the session is already in the room,
// this is a no-op.
func (rm *RoomManager) Join(sid, room string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Add sid to room set
	if rm.rooms[room] == nil {
		rm.rooms[room] = make(map[string]struct{})
	}
	rm.rooms[room][sid] = struct{}{}

	// Add room to sid's reverse index
	if rm.sids[sid] == nil {
		rm.sids[sid] = make(map[string]struct{})
	}
	rm.sids[sid][room] = struct{}{}
}

// Leave removes a session from a room. If the session is not in the room,
// this is a no-op. Empty rooms are cleaned up automatically.
func (rm *RoomManager) Leave(sid, room string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.leaveNoLock(sid, room)
}

// LeaveAll removes a session from all rooms it belongs to.
// This should be called on disconnect.
func (rm *RoomManager) LeaveAll(sid string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rooms, ok := rm.sids[sid]
	if !ok {
		return
	}

	for room := range rooms {
		// Remove sid from room, but don't modify sids[sid] during iteration
		if members, exists := rm.rooms[room]; exists {
			delete(members, sid)
			if len(members) == 0 {
				delete(rm.rooms, room)
			}
		}
	}

	delete(rm.sids, sid)
}

// Members returns all session IDs in a given room.
// Returns nil if the room doesn't exist.
func (rm *RoomManager) Members(room string) []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	members, ok := rm.rooms[room]
	if !ok {
		return nil
	}

	result := make([]string, 0, len(members))
	for sid := range members {
		result = append(result, sid)
	}
	return result
}

// Rooms returns all rooms that a session belongs to.
// Returns nil if the session is not in any room.
func (rm *RoomManager) Rooms(sid string) []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rooms, ok := rm.sids[sid]
	if !ok {
		return nil
	}

	result := make([]string, 0, len(rooms))
	for room := range rooms {
		result = append(result, room)
	}
	return result
}

// InRoom reports whether a session is in a given room.
func (rm *RoomManager) InRoom(sid, room string) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	members, ok := rm.rooms[room]
	if !ok {
		return false
	}
	_, exists := members[sid]
	return exists
}

// RoomCount returns the number of members in a room.
func (rm *RoomManager) RoomCount(room string) int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return len(rm.rooms[room])
}

// leaveNoLock removes a session from a room without acquiring the lock.
// Caller must hold rm.mu.
func (rm *RoomManager) leaveNoLock(sid, room string) {
	// Remove sid from room
	if members, ok := rm.rooms[room]; ok {
		delete(members, sid)
		if len(members) == 0 {
			delete(rm.rooms, room)
		}
	}

	// Remove room from sid's reverse index
	if rooms, ok := rm.sids[sid]; ok {
		delete(rooms, room)
		if len(rooms) == 0 {
			delete(rm.sids, sid)
		}
	}
}
