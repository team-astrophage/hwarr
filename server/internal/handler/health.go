package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ConnectionCounter provides the number of active connections.
type ConnectionCounter interface {
	ActiveCount() int
}

// EngineStatusChecker reports whether the fire progression engine is running.
type EngineStatusChecker interface {
	IsRunning() bool
}

// RedisPinger checks the Redis connection health.
type RedisPinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves the GET /health endpoint for ALB health checks.
type HealthHandler struct {
	connections ConnectionCounter
	engine      EngineStatusChecker
	redis       RedisPinger
}

// NewHealthHandler creates a new HealthHandler.
// All parameters may be nil; nil values default to safe zero values.
func NewHealthHandler(connections ConnectionCounter, engine EngineStatusChecker) *HealthHandler {
	return &HealthHandler{
		connections: connections,
		engine:      engine,
	}
}

// SetRedis attaches a Redis pinger for health checks.
func (h *HealthHandler) SetRedis(r RedisPinger) {
	h.redis = r
}

// Handle responds with server health status.
//
//	GET /health
//	Response: {"status": "ok", "connections": <int>, "engine_running": <bool>, "redis": "ok"|"<error>"}
func (h *HealthHandler) Handle(c *gin.Context) {
	conns := 0
	if h.connections != nil {
		conns = h.connections.ActiveCount()
	}

	running := false
	if h.engine != nil {
		running = h.engine.IsRunning()
	}

	redisStatus := "not configured"
	if h.redis != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := h.redis.Ping(ctx); err != nil {
			redisStatus = err.Error()
		} else {
			redisStatus = "ok"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"connections":    conns,
		"engine_running": running,
		"redis":          redisStatus,
	})
}

// Register adds the health endpoint to the given Gin engine.
func (h *HealthHandler) Register(r gin.IRouter) {
	r.GET("/health", h.Handle)
}
