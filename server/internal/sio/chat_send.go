package sio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	socketio "github.com/homeworldio/socketio-go"
)

// RedisChatWriter defines the Redis operations needed for persisting chat messages.
type RedisChatWriter interface {
	// LPush prepends a value to a list.
	LPush(ctx context.Context, key string, value string) error
	// LTrim trims a list to the specified range.
	LTrim(ctx context.Context, key string, start, stop int64) error
	// Expire sets a TTL on a key.
	Expire(ctx context.Context, key string, seconds int) error
}

// chatSendData is the client payload for chat:send.
type chatSendData struct {
	Text     *string `json:"text"`
	UserID   *string `json:"user_id"`
	Nickname *string `json:"nickname,omitempty"`
	Avatar   *string `json:"avatar,omitempty"`
	AvatarBg *string `json:"avatar_bg,omitempty"`
	NameColor *string `json:"name_color,omitempty"`
}

// RegisterChatSendHandler registers the "chat:send" Socket.IO event handler.
//
// On chat:send:
//  1. Validate the incoming message payload (text length, user_id presence)
//  2. Construct a message object with defaults for optional fields
//  3. Persist the message to Redis (LPUSH, LTRIM cap, EXPIRE TTL)
//  4. Broadcast "chat:message" to all clients in the global chat room
//  5. Return {status, id} ACK to the sender
//
// Mirrors Python server/sio/chat_events.py handle_chat_send.
func RegisterChatSendHandler(
	sioServer *socketio.Server,
	manager *ConnectionManager,
	redis RedisChatWriter,
	logger *log.Logger,
) {
	if logger == nil {
		logger = log.Default()
	}

	sioServer.On("chat:send", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		// Parse payload
		if len(args) == 0 || len(args[0]) == 0 {
			return []interface{}{map[string]interface{}{
				"error": "invalid_payload",
			}}, nil
		}

		var data chatSendData
		if err := json.Unmarshal(args[0], &data); err != nil {
			logger.Printf("chat:send from %s invalid JSON: %v", sid, err)
			return []interface{}{map[string]interface{}{
				"error": "invalid_payload",
			}}, nil
		}

		// Validate text
		text := ""
		if data.Text != nil {
			text = strings.TrimSpace(*data.Text)
		}
		if len(text) < ChatTextMin {
			return []interface{}{map[string]interface{}{
				"error": "text_too_short",
			}}, nil
		}
		if len(text) > ChatTextMax {
			return []interface{}{map[string]interface{}{
				"error": "text_too_long",
			}}, nil
		}

		// Validate user_id
		if data.UserID == nil || *data.UserID == "" {
			return []interface{}{map[string]interface{}{
				"error": "user_id_required",
			}}, nil
		}
		userID := *data.UserID

		// Generate message ID: msg-{12 hex chars}
		msgID := generateMsgID()

		// Build message with defaults matching Python implementation
		nickname := "익명"
		if data.Nickname != nil && *data.Nickname != "" {
			nickname = *data.Nickname
		}
		avatar := "👤"
		if data.Avatar != nil && *data.Avatar != "" {
			avatar = *data.Avatar
		}
		avatarBg := "#333"
		if data.AvatarBg != nil && *data.AvatarBg != "" {
			avatarBg = *data.AvatarBg
		}
		nameColor := "#fff"
		if data.NameColor != nil && *data.NameColor != "" {
			nameColor = *data.NameColor
		}

		msg := map[string]interface{}{
			"id":         msgID,
			"user_id":    userID,
			"nickname":   nickname,
			"avatar":     avatar,
			"avatar_bg":  avatarBg,
			"name_color": nameColor,
			"text":       text,
			"timestamp":  float64(time.Now().UnixMilli()) / 1000.0,
		}

		// Persist to Redis: newest-first list, cap at ChatMaxMessages, reset TTL.
		if redis != nil {
			persistChatMessage(redis, msg, logger)
		}

		// Broadcast chat:message to all clients in the global chat room
		if _, err := sioServer.BroadcastToRoom("/", ChatRoom, "chat:message", msg); err != nil {
			logger.Printf("Failed to broadcast chat:message: %v", err)
		}

		logger.Printf("chat:send sid=%s user=%s len=%d", sid, userID, len(text))

		// Return ACK
		return []interface{}{map[string]interface{}{
			"status": "ok",
			"id":     msgID,
		}}, nil
	})
}

// persistChatMessage stores a chat message in Redis.
// Uses LPUSH + LTRIM + EXPIRE pipeline pattern matching the Python implementation.
func persistChatMessage(redis RedisChatWriter, msg map[string]interface{}, logger *log.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	serialized, err := json.Marshal(msg)
	if err != nil {
		logger.Printf("Failed to serialize chat message: %v", err)
		return
	}

	// LPUSH (newest first)
	if err := redis.LPush(ctx, ChatRedisKey, string(serialized)); err != nil {
		logger.Printf("Failed to LPUSH chat message to Redis: %v", err)
		return
	}

	// LTRIM to cap at ChatMaxMessages
	if err := redis.LTrim(ctx, ChatRedisKey, 0, int64(ChatMaxMessages-1)); err != nil {
		logger.Printf("Failed to LTRIM chat messages: %v", err)
	}

	// EXPIRE to reset TTL
	if err := redis.Expire(ctx, ChatRedisKey, ChatTTLSec); err != nil {
		logger.Printf("Failed to set EXPIRE on chat messages: %v", err)
	}
}

// generateMsgID creates a message ID in the format "msg-{12 hex chars}".
// Matches Python: f"msg-{uuid.uuid4().hex[:12]}"
func generateMsgID() string {
	b := make([]byte, 6) // 6 bytes = 12 hex chars
	if _, err := rand.Read(b); err != nil {
		// Fallback: use timestamp-based ID
		return fmt.Sprintf("msg-%012x", time.Now().UnixNano()&0xFFFFFFFFFFFF)
	}
	return "msg-" + hex.EncodeToString(b)
}
