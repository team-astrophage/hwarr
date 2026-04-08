package sio

import (
	"log"

	socketio "github.com/homeworldio/socketio-go"
)

// ChatRoom is the well-known room name for the global chat.
// Matches Python server's CHAT_ROOM = "chat:global".
const ChatRoom = "chat:global"

// RegisterDisconnectHandler registers the Socket.IO disconnect event handler.
//
// On disconnect it:
//  1. Retrieves connection info (checking if user was in chat room)
//  2. Removes the connection from ConnectionManager
//  3. Broadcasts "users:count" with updated active count to all clients
//  4. If the user was in the chat room, broadcasts updated "chat:presence"
//     count to remaining chat room participants
//
// Mirrors Python server/main.py disconnect handler.
func RegisterDisconnectHandler(
	sioServer *socketio.Server,
	manager *ConnectionManager,
	logger *log.Logger,
) {
	if logger == nil {
		logger = log.Default()
	}

	sioServer.OnDisconnect(func(sid string, reason string) {
		logger.Printf("Client disconnecting: %s (reason: %s)", sid, reason)

		// 1. Check if user was in chat room before removing
		info := manager.Get(sid)
		wasInChat := false
		if info != nil {
			_, wasInChat = info.Rooms[ChatRoom]
		}

		// 2. Remove from connection manager
		//    Note: namespace.HandleDisconnect already called Rooms.LeaveAll(sid)
		//    before invoking this handler, so Socket.IO room cleanup is done.
		manager.Remove(sid)

		// 3. Broadcast updated user count to all clients
		//    Matches: await sio.emit("users:count", {"count": manager.active_count})
		activeCount := manager.ActiveCount()
		countPayload := map[string]interface{}{
			"count": activeCount,
		}
		if _, err := sioServer.BroadcastToNamespace("/", "users:count", countPayload); err != nil {
			logger.Printf("Failed to broadcast users:count: %v", err)
		}

		// 4. If the user was in the global chat room, rebroadcast chat:presence
		//    to remaining participants.
		//    Note: Since HandleDisconnect already called LeaveAll, the room count
		//    is already updated (the disconnecting socket is removed).
		if wasInChat {
			chatMembers := sioServer.RoomMembers("/", ChatRoom)
			presencePayload := map[string]interface{}{
				"count": len(chatMembers),
			}
			if _, err := sioServer.BroadcastToRoom("/", ChatRoom, "chat:presence", presencePayload); err != nil {
				logger.Printf("Failed to rebroadcast chat presence on disconnect: %v", err)
			}
		}

		logger.Printf("Client disconnected: %s (total: %d)", sid, activeCount)
	})
}
