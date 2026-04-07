package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockFeedbackRedis implements RedisFeedbackRateLimiter for testing.
type mockFeedbackRedis struct {
	counts  map[string]int64
	expired map[string]int
	failAt  int64 // if > 0, fail when count reaches this value
}

func newMockFeedbackRedis() *mockFeedbackRedis {
	return &mockFeedbackRedis{
		counts:  make(map[string]int64),
		expired: make(map[string]int),
	}
}

func (m *mockFeedbackRedis) Incr(_ context.Context, key string) (int64, error) {
	m.counts[key]++
	if m.failAt > 0 && m.counts[key] == m.failAt {
		return 0, fmt.Errorf("redis error")
	}
	return m.counts[key], nil
}

func (m *mockFeedbackRedis) Expire(_ context.Context, key string, seconds int) error {
	m.expired[key] = seconds
	return nil
}

// mockDiscordSender implements DiscordWebhookSender for testing.
type mockDiscordSender struct {
	lastPayload map[string]any
	callCount   int
	shouldFail  bool
}

func (m *mockDiscordSender) Send(_ string, payload map[string]any) error {
	m.callCount++
	m.lastPayload = payload
	if m.shouldFail {
		return fmt.Errorf("discord error")
	}
	return nil
}

func setupFeedbackHandler(redis RedisFeedbackRateLimiter, discord DiscordWebhookSender, webhookURL string, rateLimit int) (*FeedbackHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	h := &FeedbackHandler{
		redis:      redis,
		discord:    discord,
		webhookURL: webhookURL,
		rateLimit:  rateLimit,
	}
	r := gin.New()
	h.Register(r)
	return h, r
}

func TestFeedback_Success(t *testing.T) {
	discord := &mockDiscordSender{}
	redis := newMockFeedbackRedis()
	_, router := setupFeedbackHandler(redis, discord, "https://discord.com/api/webhooks/test", 3)

	body := `{"category":"bug","message":"Something broke"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp feedbackResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}

	if discord.callCount != 1 {
		t.Fatalf("expected 1 discord call, got %d", discord.callCount)
	}

	// Verify rate limit key was set
	if redis.counts["feedback:rl:1.2.3.4"] != 1 {
		t.Fatalf("expected rate limit count 1, got %d", redis.counts["feedback:rl:1.2.3.4"])
	}
	if redis.expired["feedback:rl:1.2.3.4"] != 600 {
		t.Fatalf("expected expire 600, got %d", redis.expired["feedback:rl:1.2.3.4"])
	}
}

func TestFeedback_AllCategories(t *testing.T) {
	for _, cat := range []string{"bug", "idea", "etc"} {
		t.Run(cat, func(t *testing.T) {
			discord := &mockDiscordSender{}
			_, router := setupFeedbackHandler(nil, discord, "https://discord.com/api/webhooks/test", 3)

			body := fmt.Sprintf(`{"category":"%s","message":"test message"}`, cat)
			req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
			}

			embeds := discord.lastPayload["embeds"].([]any)
			embed := embeds[0].(map[string]any)
			author := embed["author"].(map[string]any)
			label := author["name"].(string)
			expectedLabel := categoryLabels[cat]
			if label != expectedLabel {
				t.Fatalf("expected label %q, got %q", expectedLabel, label)
			}
		})
	}
}

func TestFeedback_WithOptionalFields(t *testing.T) {
	discord := &mockDiscordSender{}
	_, router := setupFeedbackHandler(nil, discord, "https://discord.com/api/webhooks/test", 3)

	body := `{"category":"idea","message":"Add dark mode","email":"user@example.com","page":"/settings"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	embeds := discord.lastPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	fields := embed["fields"].([]map[string]any)
	if len(fields) != 2 {
		t.Fatalf("expected 2 embed fields, got %d", len(fields))
	}
	if fields[0]["name"] != "페이지" {
		t.Fatalf("expected first field to be 페이지, got %s", fields[0]["name"])
	}
	if fields[1]["name"] != "답변 이메일" {
		t.Fatalf("expected second field to be 답변 이메일, got %s", fields[1]["name"])
	}
}

func TestFeedback_WebhookNotConfigured(t *testing.T) {
	discord := &mockDiscordSender{}
	_, router := setupFeedbackHandler(nil, discord, "", 3)

	body := `{"category":"bug","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["detail"] != "feedback_disabled" {
		t.Fatalf("expected detail 'feedback_disabled', got %q", resp["detail"])
	}
}

func TestFeedback_RateLimited(t *testing.T) {
	discord := &mockDiscordSender{}
	redis := newMockFeedbackRedis()
	_, router := setupFeedbackHandler(redis, discord, "https://discord.com/api/webhooks/test", 3)

	// Send 4 requests — the 4th should be rate limited
	for i := 0; i < 4; i++ {
		body := `{"category":"bug","message":"spam"}`
		req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "5.6.7.8")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i < 3 {
			if w.Code != http.StatusCreated {
				t.Fatalf("request %d: expected 201, got %d: %s", i+1, w.Code, w.Body.String())
			}
		} else {
			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("request %d: expected 429, got %d", i+1, w.Code)
			}
			var resp map[string]string
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["detail"] != "rate_limited" {
				t.Fatalf("expected detail 'rate_limited', got %q", resp["detail"])
			}
		}
	}

	// Discord should have been called only 3 times
	if discord.callCount != 3 {
		t.Fatalf("expected 3 discord calls, got %d", discord.callCount)
	}
}

func TestFeedback_RedisFail_AllowsThrough(t *testing.T) {
	discord := &mockDiscordSender{}
	redis := newMockFeedbackRedis()
	redis.failAt = 1 // fail on first incr
	_, router := setupFeedbackHandler(redis, discord, "https://discord.com/api/webhooks/test", 3)

	body := `{"category":"bug","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should allow through even though Redis failed
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 (soft fail), got %d: %s", w.Code, w.Body.String())
	}
}

func TestFeedback_DiscordFail(t *testing.T) {
	discord := &mockDiscordSender{shouldFail: true}
	_, router := setupFeedbackHandler(nil, discord, "https://discord.com/api/webhooks/test", 3)

	body := `{"category":"bug","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["detail"] != "webhook_failed" {
		t.Fatalf("expected detail 'webhook_failed', got %q", resp["detail"])
	}
}

func TestFeedback_InvalidCategory(t *testing.T) {
	discord := &mockDiscordSender{}
	_, router := setupFeedbackHandler(nil, discord, "https://discord.com/api/webhooks/test", 3)

	body := `{"category":"invalid","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestFeedback_EmptyMessage(t *testing.T) {
	discord := &mockDiscordSender{}
	_, router := setupFeedbackHandler(nil, discord, "https://discord.com/api/webhooks/test", 3)

	body := `{"category":"bug","message":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestFeedback_IPFromXFF(t *testing.T) {
	discord := &mockDiscordSender{}
	redis := newMockFeedbackRedis()
	_, router := setupFeedbackHandler(redis, discord, "https://discord.com/api/webhooks/test", 3)

	body := `{"category":"bug","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.1, 10.0.0.2")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	// Should use the first IP from X-Forwarded-For
	if _, ok := redis.counts["feedback:rl:203.0.113.50"]; !ok {
		t.Fatal("expected rate limit key for 203.0.113.50")
	}
}

func TestFeedback_NoRedis(t *testing.T) {
	discord := &mockDiscordSender{}
	_, router := setupFeedbackHandler(nil, discord, "https://discord.com/api/webhooks/test", 3)

	body := `{"category":"bug","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should work fine without Redis
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShortUA(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "unknown client"},
		{"-", "unknown client"},
		{"Firefox", "Firefox"},
		{strings.Repeat("A", 80), strings.Repeat("A", 80)},
		{strings.Repeat("A", 81), strings.Repeat("A", 79) + "…"},
	}
	for _, tt := range tests {
		got := shortUA(tt.input)
		if got != tt.expected {
			t.Errorf("shortUA(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestQuoteLines(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "> hello"},
		{"line1\nline2", "> line1\n> line2"},
		{"line1\n\nline3", "> line1\n>\n> line3"},
	}
	for _, tt := range tests {
		got := quoteLines(tt.input)
		if got != tt.expected {
			t.Errorf("quoteLines(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFeedback_EmbedStructure(t *testing.T) {
	discord := &mockDiscordSender{}
	_, router := setupFeedbackHandler(nil, discord, "https://discord.com/api/webhooks/test", 3)

	body := `{"category":"bug","message":"Something broke\nOn this page"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("User-Agent", "TestBrowser/1.0")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	// Verify embed structure matches Python
	embeds := discord.lastPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)

	// author
	author := embed["author"].(map[string]any)
	if author["name"] != "🐛 버그 제보" {
		t.Fatalf("unexpected author: %v", author["name"])
	}

	// description (blockquoted)
	desc := embed["description"].(string)
	if desc != "> Something broke\n> On this page" {
		t.Fatalf("unexpected description: %q", desc)
	}

	// color
	if embed["color"] != 0xF3727F {
		t.Fatalf("unexpected color: %v", embed["color"])
	}

	// footer
	footer := embed["footer"].(map[string]any)
	footerText := footer["text"].(string)
	if !strings.Contains(footerText, "1.2.3.4") {
		t.Fatalf("footer should contain IP: %q", footerText)
	}
	if !strings.Contains(footerText, "TestBrowser/1.0") {
		t.Fatalf("footer should contain UA: %q", footerText)
	}

	// allowed_mentions
	am := discord.lastPayload["allowed_mentions"].(map[string]any)
	parse := am["parse"].([]any)
	if len(parse) != 0 {
		t.Fatalf("allowed_mentions.parse should be empty, got %v", parse)
	}

	// timestamp
	if _, ok := embed["timestamp"]; !ok {
		t.Fatal("embed should have timestamp")
	}
}
