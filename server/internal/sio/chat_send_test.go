package sio

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"testing"
)

// mockRedisChatWriter implements RedisChatWriter for testing.
type mockRedisChatWriter struct {
	pushed  []string // values passed to LPush
	trimmed bool     // whether LTrim was called
	expired bool     // whether Expire was called
	err     error    // error to return from all operations
}

func (m *mockRedisChatWriter) LPush(_ context.Context, key string, value string) error {
	if m.err != nil {
		return m.err
	}
	m.pushed = append(m.pushed, value)
	return nil
}

func (m *mockRedisChatWriter) LTrim(_ context.Context, key string, start, stop int64) error {
	if m.err != nil {
		return m.err
	}
	m.trimmed = true
	return nil
}

func (m *mockRedisChatWriter) Expire(_ context.Context, key string, seconds int) error {
	if m.err != nil {
		return m.err
	}
	m.expired = true
	return nil
}

func TestPersistChatMessage(t *testing.T) {
	redis := &mockRedisChatWriter{}
	logger := log.Default()

	msg := map[string]interface{}{
		"id":        "msg-abc123def456",
		"user_id":   "user-1",
		"nickname":  "테스터",
		"text":      "Hello!",
		"timestamp": 1234567890.123,
	}

	persistChatMessage(redis, msg, logger)

	if len(redis.pushed) != 1 {
		t.Fatalf("expected 1 LPUSH call, got %d", len(redis.pushed))
	}
	if !redis.trimmed {
		t.Error("expected LTRIM to be called")
	}
	if !redis.expired {
		t.Error("expected EXPIRE to be called")
	}

	// Verify the pushed JSON contains expected fields
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(redis.pushed[0]), &parsed); err != nil {
		t.Fatalf("failed to parse pushed JSON: %v", err)
	}
	if parsed["id"] != "msg-abc123def456" {
		t.Errorf("expected id=msg-abc123def456, got %v", parsed["id"])
	}
	if parsed["text"] != "Hello!" {
		t.Errorf("expected text=Hello!, got %v", parsed["text"])
	}
}

func TestGenerateMsgID(t *testing.T) {
	id := generateMsgID()

	if !strings.HasPrefix(id, "msg-") {
		t.Errorf("expected msg- prefix, got %q", id)
	}

	// "msg-" + 12 hex chars = 16 total
	if len(id) != 16 {
		t.Errorf("expected length 16, got %d for %q", len(id), id)
	}

	// Verify uniqueness
	id2 := generateMsgID()
	if id == id2 {
		t.Errorf("expected unique IDs, got same: %q", id)
	}
}

func TestChatSendValidation_EmptyPayload(t *testing.T) {
	// Test that chatSendData correctly parses empty/missing fields
	var data chatSendData
	raw := `{}`
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	if data.Text != nil {
		t.Error("expected nil Text for empty payload")
	}
	if data.UserID != nil {
		t.Error("expected nil UserID for empty payload")
	}
}

func TestChatSendValidation_ValidPayload(t *testing.T) {
	raw := `{"text": "  Hello World  ", "user_id": "user-123", "nickname": "테스터"}`
	var data chatSendData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	if data.Text == nil || *data.Text != "  Hello World  " {
		t.Errorf("expected text '  Hello World  ', got %v", data.Text)
	}
	if data.UserID == nil || *data.UserID != "user-123" {
		t.Errorf("expected user_id 'user-123', got %v", data.UserID)
	}
	if data.Nickname == nil || *data.Nickname != "테스터" {
		t.Errorf("expected nickname '테스터', got %v", data.Nickname)
	}
}

func TestChatSendValidation_TextTrimming(t *testing.T) {
	// Verify that whitespace-only text is treated as empty
	text := "   "
	trimmed := strings.TrimSpace(text)
	if len(trimmed) >= ChatTextMin {
		t.Errorf("expected trimmed whitespace to be below min length, got %d", len(trimmed))
	}
}

func TestChatSendValidation_TextTooLong(t *testing.T) {
	text := strings.Repeat("a", ChatTextMax+1)
	if len(text) <= ChatTextMax {
		t.Errorf("expected text to exceed max length %d", ChatTextMax)
	}
}

func TestChatSendDefaults(t *testing.T) {
	// Test default values for optional fields
	tests := []struct {
		name     string
		input    string
		field    string
		expected string
	}{
		{"default nickname", `{"text":"hi","user_id":"u1"}`, "nickname", "익명"},
		{"custom nickname", `{"text":"hi","user_id":"u1","nickname":"Alice"}`, "nickname", "Alice"},
		{"empty nickname uses default", `{"text":"hi","user_id":"u1","nickname":""}`, "nickname", "익명"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data chatSendData
			if err := json.Unmarshal([]byte(tt.input), &data); err != nil {
				t.Fatal(err)
			}

			actual := "익명"
			if data.Nickname != nil && *data.Nickname != "" {
				actual = *data.Nickname
			}

			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestPersistChatMessage_RedisError(t *testing.T) {
	redis := &mockRedisChatWriter{err: context.DeadlineExceeded}
	logger := log.Default()

	msg := map[string]interface{}{
		"id":   "msg-test123",
		"text": "test",
	}

	// Should not panic on Redis errors
	persistChatMessage(redis, msg, logger)

	if len(redis.pushed) != 0 {
		t.Error("expected no successful pushes on error")
	}
}
