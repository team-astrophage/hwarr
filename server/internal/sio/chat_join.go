package sio

import (
	"context"
	"encoding/json"
	"log"
	"time"

	socketio "github.com/homeworldio/socketio-go"
)

// Chat constants matching Python server/sio/chat_events.py.
const (
	ChatRedisKey    = "chat:global:messages"
	ChatTTLSec      = 3600 // 1 hour
	ChatMaxMessages = 100
	ChatHistorySize = 50
	ChatTextMin     = 1
	ChatTextMax     = 300
)

// RedisChatReader defines the Redis operations needed for loading chat history.
type RedisChatReader interface {
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
}

// RegisterChatJoinHandler registers the "chat:join" Socket.IO event handler.
//
// On chat:join:
//  1. Verify SID exists in ConnectionManager
//  2. Join the client to the global chat room
//  3. Load recent chat history from Redis (newest 50, returned chronologically)
//  4. Broadcast updated chat:presence to all chat room members
//  5. Return {status, history, presence} ACK to the caller
//
// Mirrors Python server/sio/chat_events.py handle_chat_join.
func RegisterChatJoinHandler(
	sioServer *socketio.Server,
	manager *ConnectionManager,
	redis RedisChatReader,
	logger *log.Logger,
) {
	if logger == nil {
		logger = log.Default()
	}

	sioServer.On("chat:join", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		// 1. Verify SID
		info := manager.Get(sid)
		if info == nil {
			logger.Printf("chat:join from unknown sid: %s", sid)
			return []interface{}{map[string]interface{}{
				"error": "unknown_sid",
			}}, nil
		}

		// 2. Join the global chat room
		if err := sioServer.EnterRoom("/", sid, ChatRoom); err != nil {
			logger.Printf("chat:join failed to enter room for %s: %v", sid, err)
		}
		// Track in ConnectionManager (matches Python: info.rooms.add(CHAT_ROOM))
		info.Rooms[ChatRoom] = struct{}{}

		// 3. Load recent history from Redis
		history := loadChatHistory(redis, logger)

		// 4. Broadcast updated presence to everyone in the chat room
		chatMembers := sioServer.RoomMembers("/", ChatRoom)
		count := len(chatMembers)

		presencePayload := map[string]interface{}{
			"count": count,
		}
		if _, err := sioServer.BroadcastToRoom("/", ChatRoom, "chat:presence", presencePayload); err != nil {
			logger.Printf("Failed to broadcast chat:presence: %v", err)
		}

		logger.Printf("chat:join sid=%s presence=%d history=%d", sid, count, len(history))

		// 5. Return ACK
		return []interface{}{map[string]interface{}{
			"status":   "ok",
			"history":  history,
			"presence": count,
		}}, nil
	})
}

// loadChatHistory retrieves recent chat messages from Redis.
// Messages are stored newest-first via LPUSH, so LRANGE 0 49 returns
// newest→oldest. We reverse them for chronological order (oldest→newest).
func loadChatHistory(redis RedisChatReader, logger *log.Logger) []map[string]interface{} {
	history := make([]map[string]interface{}, 0)

	if redis == nil {
		return history
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	raw, err := redis.LRange(ctx, ChatRedisKey, 0, ChatHistorySize-1)
	if err != nil {
		logger.Printf("Failed to load chat history from Redis: %v", err)
		return history
	}

	// Parse in reverse order (newest-first → chronological)
	for i := len(raw) - 1; i >= 0; i-- {
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(raw[i]), &msg); err != nil {
			continue // skip malformed entries
		}
		history = append(history, msg)
	}

	return history
}
