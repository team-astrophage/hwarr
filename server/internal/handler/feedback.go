package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Redis key constants for feedback rate limiting.
const (
	FeedbackRLKeyPrefix = "feedback:rl:"
	feedbackRLWindowSec = 600 // 10 minutes
	uaMaxLen            = 80
)

// RedisFeedbackRateLimiter abstracts the Redis operations needed for rate limiting.
type RedisFeedbackRateLimiter interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, seconds int) error
}

// feedbackRequest matches the Python FeedbackRequest model.
type feedbackRequest struct {
	Category string  `json:"category" binding:"required,oneof=bug idea etc"`
	Message  string  `json:"message" binding:"required,min=1,max=500"`
	Email    *string `json:"email,omitempty" binding:"omitempty,max=200"`
	Page     *string `json:"page,omitempty" binding:"omitempty,max=200"`
}

// feedbackResponse matches the Python FeedbackResponse model.
type feedbackResponse struct {
	OK bool `json:"ok"`
}

var categoryLabels = map[string]string{
	"bug":  "🐛 버그 제보",
	"idea": "💡 기능 제안",
	"etc":  "💬 기타 의견",
}

var categoryColors = map[string]int{
	"bug":  0xF3727F, // negative red
	"idea": 0x1ED760, // accent green
	"etc":  0xFFA42B, // warning orange
}

// DiscordWebhookSender sends payloads to Discord webhooks.
// Extracted as interface for testability.
type DiscordWebhookSender interface {
	Send(webhookURL string, payload map[string]any) error
}

// HTTPDiscordSender is the default implementation using net/http.
type HTTPDiscordSender struct {
	Client *http.Client
}

// NewHTTPDiscordSender creates a sender with a 5-second timeout.
func NewHTTPDiscordSender() *HTTPDiscordSender {
	return &HTTPDiscordSender{
		Client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Send posts the payload to the Discord webhook URL.
func (s *HTTPDiscordSender) Send(webhookURL string, payload map[string]any) error {
	if webhookURL == "" {
		return fmt.Errorf("discord webhook url is empty")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	resp, err := s.Client.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}
	slog.Info("discord webhook delivered", "status", resp.StatusCode)
	return nil
}

// FeedbackHandler serves the POST /api/feedback endpoint.
type FeedbackHandler struct {
	redis      RedisFeedbackRateLimiter
	discord    DiscordWebhookSender
	webhookURL string
	rateLimit  int
}

// NewFeedbackHandler creates a new FeedbackHandler.
// redis may be nil (rate limiting will be skipped).
// discord may be nil (defaults to HTTPDiscordSender).
func NewFeedbackHandler(redis RedisFeedbackRateLimiter, discord DiscordWebhookSender) *FeedbackHandler {
	webhookURL := os.Getenv("FEEDBACK_DISCORD_WEBHOOK_URL")
	rateLimit := 3
	if v := os.Getenv("FEEDBACK_RATE_LIMIT_PER_10MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rateLimit = n
		}
	}
	if discord == nil {
		discord = NewHTTPDiscordSender()
	}
	return &FeedbackHandler{
		redis:      redis,
		discord:    discord,
		webhookURL: webhookURL,
		rateLimit:  rateLimit,
	}
}

// Handle processes a feedback submission.
//
//	POST /api/feedback
//	Request: {"category": "bug"|"idea"|"etc", "message": "...", "email": "...", "page": "..."}
//	Response 201: {"ok": true}
//	Response 429: {"detail": "rate_limited"}
//	Response 503: {"detail": "feedback_disabled"}
//	Response 502: {"detail": "webhook_failed"}
func (h *FeedbackHandler) Handle(c *gin.Context) {
	if h.webhookURL == "" {
		slog.Error("FEEDBACK_DISCORD_WEBHOOK_URL not configured — rejecting feedback")
		c.JSON(http.StatusServiceUnavailable, gin.H{"detail": "feedback_disabled"})
		return
	}

	var body feedbackRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": err.Error()})
		return
	}

	// Resolve real client IP (CloudFront/ALB-aware)
	ip := resolveClientIP(c)

	// User-Agent (CloudFront-aware)
	ua := c.GetHeader("cloudfront-viewer-user-agent")
	if ua == "" {
		ua = c.GetHeader("User-Agent")
	}
	if ua == "" {
		ua = "-"
	}

	// Rate limit (soft fail if Redis is unavailable)
	if h.redis != nil {
		ctx := c.Request.Context()
		key := FeedbackRLKeyPrefix + ip
		count, err := h.redis.Incr(ctx, key)
		if err != nil {
			slog.Error("feedback rate-limit check failed — allowing through", "error", err)
		} else {
			if count == 1 {
				if expErr := h.redis.Expire(ctx, key, feedbackRLWindowSec); expErr != nil {
					slog.Error("feedback rate-limit expire failed", "error", expErr)
				}
			}
			if count > int64(h.rateLimit) {
				c.JSON(http.StatusTooManyRequests, gin.H{"detail": "rate_limited"})
				return
			}
		}
	}

	// Compose Discord embed
	label := categoryLabels[body.Category]
	if label == "" {
		label = body.Category
	}
	color := categoryColors[body.Category]
	if color == 0 {
		color = 0x888888
	}

	fields := []map[string]any{}
	if body.Page != nil && *body.Page != "" {
		fields = append(fields, map[string]any{
			"name": "페이지", "value": fmt.Sprintf("`%s`", *body.Page), "inline": true,
		})
	}
	if body.Email != nil && *body.Email != "" {
		fields = append(fields, map[string]any{
			"name": "답변 이메일", "value": *body.Email, "inline": true,
		})
	}

	embed := map[string]any{
		"author":      map[string]any{"name": label},
		"description": quoteLines(body.Message),
		"color":       color,
		"fields":      fields,
		"footer":      map[string]any{"text": fmt.Sprintf("%s · %s", ip, shortUA(ua))},
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	payload := map[string]any{
		"embeds":           []any{embed},
		"allowed_mentions": map[string]any{"parse": []any{}},
	}

	// Send to Discord
	if err := h.discord.Send(h.webhookURL, payload); err != nil {
		slog.Error("failed to deliver feedback to discord", "category", body.Category, "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"detail": "webhook_failed"})
		return
	}

	slog.Info("feedback received",
		"category", body.Category,
		"ip", ip,
		"len", len(body.Message),
		"has_email", body.Email != nil && *body.Email != "",
	)
	c.JSON(http.StatusCreated, feedbackResponse{OK: true})
}

// Register adds the feedback endpoint to the given Gin router.
func (h *FeedbackHandler) Register(r gin.IRouter) {
	r.POST("/api/feedback", h.Handle)
}

// resolveClientIP extracts the real client IP from X-Forwarded-For or falls back
// to the direct connection IP. CloudFront prepends the viewer IP.
func resolveClientIP(c *gin.Context) string {
	xff := c.GetHeader("X-Forwarded-For")
	if xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	return c.ClientIP()
}

// shortUA truncates a User-Agent string for compact embed footer display.
func shortUA(ua string) string {
	if ua == "" || ua == "-" {
		return "unknown client"
	}
	if len(ua) <= uaMaxLen {
		return ua
	}
	return ua[:uaMaxLen-1] + "…"
}

// quoteLines prefixes each line with Discord blockquote markers.
func quoteLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + line
		}
	}
	return strings.Join(lines, "\n")
}
