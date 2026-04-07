package handler

import (
	"net/http"

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

// HealthHandler serves the GET /health endpoint for ALB health checks.
type HealthHandler struct {
	connections ConnectionCounter
	engine      EngineStatusChecker
}

// NewHealthHandler creates a new HealthHandler.
// Both parameters may be nil; nil values default to 0 connections and engine not running.
func NewHealthHandler(connections ConnectionCounter, engine EngineStatusChecker) *HealthHandler {
	return &HealthHandler{
		connections: connections,
		engine:      engine,
	}
}

// Handle responds with server health status.
//
//	GET /health
//	Response: {"status": "ok", "connections": <int>, "engine_running": <bool>}
func (h *HealthHandler) Handle(c *gin.Context) {
	conns := 0
	if h.connections != nil {
		conns = h.connections.ActiveCount()
	}

	running := false
	if h.engine != nil {
		running = h.engine.IsRunning()
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"connections":    conns,
		"engine_running": running,
	})
}

// Register adds the health endpoint to the given Gin engine.
func (h *HealthHandler) Register(r gin.IRouter) {
	r.GET("/health", h.Handle)
}
