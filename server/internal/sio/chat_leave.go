package sio

import (
	"encoding/json"
	"log"

	socketio "github.com/homeworldio/socketio-go"
)

// RegisterChatLeaveHandler registers the "chat:leave" Socket.IO event handler.
//
// On chat:leave:
//  1. Verify SID exists in ConnectionManager
//  2. Leave the global chat room
//  3. Remove room from ConnectionManager tracking
//  4. Broadcast updated chat:presence to remaining chat room members
//  5. Return {status: "ok"} ACK to the caller
//
// Mirrors Python server/sio/chat_events.py handle_chat_leave.
func RegisterChatLeaveHandler(
	sioServer *socketio.Server,
	broadcaster SocketBroadcaster,
	manager *ConnectionManager,
	logger *log.Logger,
) {
	if logger == nil {
		logger = log.Default()
	}

	sioServer.On("chat:leave", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		// 1. Verify SID (Python: info = manager.get(sid))
		info := manager.Get(sid)

		// 2. Leave the global chat room
		// Python wraps this in try/except, we ignore errors similarly.
		if err := sioServer.LeaveRoom("/", sid, ChatRoom); err != nil {
			logger.Printf("chat:leave failed to leave room for %s: %v", sid, err)
		}

		// 3. Remove room from ConnectionManager tracking
		// Matches Python: info.rooms.discard(CHAT_ROOM)
		if info != nil {
			delete(info.Rooms, ChatRoom)
		}

		// 4. Broadcast updated presence to remaining chat room members
		chatMembers := sioServer.RoomMembers("/", ChatRoom)
		count := len(chatMembers)

		presencePayload := map[string]interface{}{
			"count": count,
		}
		if _, err := broadcaster.BroadcastToRoom("/", ChatRoom, "chat:presence", presencePayload); err != nil {
			logger.Printf("Failed to broadcast chat:presence on leave: %v", err)
		}

		logger.Printf("chat:leave sid=%s presence=%d", sid, count)

		// 5. Return ACK
		return []interface{}{map[string]interface{}{
			"status": "ok",
		}}, nil
	})
}
