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

// ShutdownChecker reports whether the server is in graceful shutdown.
// When true, /health returns 503 so the ALB stops sending new traffic
// to this task (used during Fargate Spot interruptions).
type ShutdownChecker interface {
	IsShuttingDown() bool
}

// HealthHandler serves the GET /health endpoint for ALB health checks.
type HealthHandler struct {
	connections ConnectionCounter
	engine      EngineStatusChecker
	redis       RedisPinger
	shutdown    ShutdownChecker
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

// SetShutdown attaches a shutdown checker so /health can flip to 503
// when the server starts graceful shutdown.
func (h *HealthHandler) SetShutdown(s ShutdownChecker) {
	h.shutdown = s
}

// Handle responds with server health status.
//
//	GET /health
//	200 Response: {"status": "ok", "connections": <int>, "engine_running": <bool>, "redis": "ok"|"<error>"}
//	503 Response: {"status": "shutting_down", ...} when graceful shutdown is in progress
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

	status := "ok"
	code := http.StatusOK
	if h.shutdown != nil && h.shutdown.IsShuttingDown() {
		status = "shutting_down"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status":         status,
		"connections":    conns,
		"engine_running": running,
		"redis":          redisStatus,
	})
}

// Register adds the health endpoint to the given Gin engine.
func (h *HealthHandler) Register(r gin.IRouter) {
	r.GET("/health", h.Handle)
}
