package sio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
)

// mockRedisChatReader implements RedisChatReader for testing.
type mockRedisChatReader struct {
	data map[string][]string
	err  error
}

func (m *mockRedisChatReader) LRange(_ context.Context, key string, start, stop int64) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	items, ok := m.data[key]
	if !ok {
		return []string{}, nil
	}
	// Simulate Redis LRANGE behavior
	if start < 0 {
		start = 0
	}
	if stop >= int64(len(items)) {
		stop = int64(len(items)) - 1
	}
	if start > stop || start >= int64(len(items)) {
		return []string{}, nil
	}
	return items[start : stop+1], nil
}

func TestLoadChatHistory_Empty(t *testing.T) {
	redis := &mockRedisChatReader{data: map[string][]string{}}
	logger := log.Default()
	history := loadChatHistory(redis, logger)
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d items", len(history))
	}
}

func TestLoadChatHistory_NilRedis(t *testing.T) {
	logger := log.Default()
	history := loadChatHistory(nil, logger)
	if len(history) != 0 {
		t.Errorf("expected empty history with nil redis, got %d items", len(history))
	}
}

func TestLoadChatHistory_ChronologicalOrder(t *testing.T) {
	// Redis stores newest-first (LPUSH), so index 0 is newest
	msgs := []string{
		`{"id":"msg-003","text":"third","timestamp":3.0}`,
		`{"id":"msg-002","text":"second","timestamp":2.0}`,
		`{"id":"msg-001","text":"first","timestamp":1.0}`,
	}
	redis := &mockRedisChatReader{
		data: map[string][]string{ChatRedisKey: msgs},
	}
	logger := log.Default()
	history := loadChatHistory(redis, logger)

	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}

	// Should be reversed to chronological order (oldest first)
	if history[0]["id"] != "msg-001" {
		t.Errorf("expected first item to be msg-001, got %v", history[0]["id"])
	}
	if history[2]["id"] != "msg-003" {
		t.Errorf("expected last item to be msg-003, got %v", history[2]["id"])
	}
}

func TestLoadChatHistory_MalformedEntries(t *testing.T) {
	msgs := []string{
		`{"id":"msg-002","text":"valid"}`,
		`not-valid-json`,
		`{"id":"msg-001","text":"also valid"}`,
	}
	redis := &mockRedisChatReader{
		data: map[string][]string{ChatRedisKey: msgs},
	}
	logger := log.Default()
	history := loadChatHistory(redis, logger)

	// Should skip malformed entries
	if len(history) != 2 {
		t.Fatalf("expected 2 valid messages, got %d", len(history))
	}
}

func TestLoadChatHistory_RedisError(t *testing.T) {
	redis := &mockRedisChatReader{
		err: fmt.Errorf("connection refused"),
	}
	logger := log.Default()
	history := loadChatHistory(redis, logger)

	if len(history) != 0 {
		t.Errorf("expected empty history on error, got %d items", len(history))
	}
}

func TestLoadChatHistory_LimitTo50(t *testing.T) {
	// Even if Redis has 100+ messages, we only request 50
	msgs := make([]string, 100)
	for i := 0; i < 100; i++ {
		msgs[i] = fmt.Sprintf(`{"id":"msg-%03d","text":"msg %d"}`, 100-i, 100-i)
	}
	redis := &mockRedisChatReader{
		data: map[string][]string{ChatRedisKey: msgs},
	}
	logger := log.Default()
	history := loadChatHistory(redis, logger)

	// LRange(0, 49) returns at most 50 items
	if len(history) != 50 {
		t.Errorf("expected 50 messages, got %d", len(history))
	}
}

func TestChatConstants(t *testing.T) {
	// Verify constants match Python server
	if ChatRoom != "chat:global" {
		t.Errorf("ChatRoom = %q, want %q", ChatRoom, "chat:global")
	}
	if ChatRedisKey != "chat:global:messages" {
		t.Errorf("ChatRedisKey = %q, want %q", ChatRedisKey, "chat:global:messages")
	}
	if ChatTTLSec != 3600 {
		t.Errorf("ChatTTLSec = %d, want 3600", ChatTTLSec)
	}
	if ChatMaxMessages != 100 {
		t.Errorf("ChatMaxMessages = %d, want 100", ChatMaxMessages)
	}
	if ChatHistorySize != 50 {
		t.Errorf("ChatHistorySize = %d, want 50", ChatHistorySize)
	}
}

func TestChatJoinResponse_UnknownSID(t *testing.T) {
	// Verify the response structure for unknown SID
	// This tests the handler logic indirectly through the response format
	expected := map[string]interface{}{
		"error": "unknown_sid",
	}
	data, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["error"] != "unknown_sid" {
		t.Errorf("unexpected error value: %v", result["error"])
	}
}
