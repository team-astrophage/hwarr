// 전국불판 — Go Gin + Socket.IO application entry point.
//
// Creates the Gin HTTP server, Engine.IO/Socket.IO servers, and wires
// all handlers, background engines, and middleware together.
// This is the Go equivalent of server/main.py.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/homepy/hwarr/server-go/internal/engine"
	"github.com/homepy/hwarr/server-go/internal/geodata"
	"github.com/homepy/hwarr/server-go/internal/handler"
	hredis "github.com/homepy/hwarr/server-go/internal/redis"
	"github.com/homepy/hwarr/server-go/internal/sio"
	engineio "github.com/homepy/hwarr/server-go/pkg/engineio"
	socketio "github.com/homeworldio/socketio-go"
)

func main() {
	logger := log.New(os.Stdout, "[hwarr] ", log.LstdFlags|log.Lmsgprefix)

	// ---------------------------------------------------------------
	// Configuration from environment
	// ---------------------------------------------------------------
	host := envOr("HOST", "0.0.0.0")
	port := envOr("PORT", "8000")
	redisURL := envOr("REDIS_URL", "redis://localhost:6379/0")
	adminGeoJSONPath := envOr("ADMIN_GEOJSON_PATH", "data/admin_dong.geojson")

	// Parse Redis URL
	redisAddr, redisPassword, redisDB := parseRedisURL(redisURL)

	// ---------------------------------------------------------------
	// Redis
	// ---------------------------------------------------------------
	redisClient := hredis.NewClient(redisAddr, redisPassword, redisDB)
	ctx := context.Background()
	if err := redisClient.Ping(ctx); err != nil {
		logger.Printf("WARNING: Failed to connect to Redis at %s: %v — running without fire engine", redisAddr, err)
	} else {
		logger.Printf("Connected to Redis: %s", redisAddr)
	}

	// ---------------------------------------------------------------
	// Socket.IO + Engine.IO servers
	// ---------------------------------------------------------------
	sioServer := socketio.NewServer()
	sioServer.Logger = logger

	eioConfig := engineio.ServerConfig{
		PingInterval: 10 * time.Second,
		PingTimeout:  5 * time.Second,
		MaxPayload:   1_000_000,
		Upgrades:     []string{"websocket"},
	}
	eioServer := engineio.NewServer(eioConfig)

	// Bridge Engine.IO ↔ Socket.IO
	// Set the global SendTo callback (routes SIO packets through EIO sessions)
	sioServer.SendTo = func(sid string, data string) error {
		s := eioServer.GetSession(sid)
		if s == nil {
			return fmt.Errorf("session %s not found", sid)
		}
		s.Send([]byte(data))
		return nil
	}

	eioServer.OnConnect(func(session *engineio.Session) {
		sid := session.ID

		session.SetOnMessage(func(data []byte) {
			dataStr := string(data)
			result, pkt, err := sioServer.DispatchRawFull(sid, dataStr)
			if err != nil {
				logger.Printf("SIO dispatch error sid=%s: %v", sid, err)
			}
			// Send response packet (CONNECT ACK, etc.)
			if result != nil && result.ResponsePacket != nil {
				encoded, encErr := socketio.Encode(result.ResponsePacket)
				if encErr == nil {
					session.Send([]byte(encoded))
				}
			}
			// Send ACK if needed
			if pkt != nil && result != nil && result.AckData != nil {
				ackPkt, ackErr := socketio.BuildAckPacket(pkt, result.AckData)
				if ackErr == nil && ackPkt != nil {
					encoded, encErr := socketio.Encode(ackPkt)
					if encErr == nil {
						session.Send([]byte(encoded))
					}
				}
			}
		})

		session.SetOnClose(func(closedSID string, reason string) {
			sioServer.DisconnectAll(closedSID, reason)
		})
	})

	// ---------------------------------------------------------------
	// Connection manager + SIO event handlers
	// ---------------------------------------------------------------
	sioHandler := sio.NewHandler(sioServer, logger)
	manager := sioHandler.Manager()

	// Fire events
	sio.RegisterFireIgniteHandler(sioServer, manager, redisClient, logger)
	sio.RegisterFireStateHandler(sioServer, redisClient, logger)
	sio.RegisterSubscribeViewportHandler(sioServer, manager, redisClient, logger)

	// Compat events (mock server)
	sio.RegisterFireCompatHandler(sioServer, manager, redisClient, logger)
	sio.RegisterGetFiresCompatHandler(sioServer, redisClient, logger)

	// Chat events
	chatRedis := &chatRedisAdapter{c: redisClient}
	sio.RegisterChatJoinHandler(sioServer, manager, chatRedis, logger)
	sio.RegisterChatSendHandler(sioServer, manager, chatRedis, logger)
	sio.RegisterChatLeaveHandler(sioServer, manager, logger)

	// ---------------------------------------------------------------
	// Background engines
	// ---------------------------------------------------------------
	progressionEngine := engine.NewFireProgressionEngine(
		redisClient.AsProgressionReader(),
		sioServer,
		2*time.Second,
		logger,
	)

	cleanupEngine := engine.NewCleanupEngine(
		redisClient.AsCleanupRedis(),
		60*time.Second,
		engine.WithGridStageTracker(progressionEngine),
		engine.WithCleanupLogger(logger),
	)

	// Admin region resolver (optional — for daily ranking)
	resolver := geodata.LoadOrNil(adminGeoJSONPath)
	if resolver != nil {
		logger.Printf("Admin region resolver loaded: %d regions", resolver.RegionCount())
	}
	// TODO: integrate resolver with fire registration for daily ranking
	// (requires adding SetRegionResolver to FireProgressionEngine)
	_ = resolver

	progressionEngine.Start()
	cleanupEngine.Start()

	// Stale connection reaper
	reaper := sio.NewReaper(manager, func(ns, sid string) error {
		sioServer.DisconnectAll(sid, "stale connection")
		return nil
	})
	reaper.Start()

	logger.Println("Background engines started")

	// ---------------------------------------------------------------
	// Gin HTTP server + REST API
	// ---------------------------------------------------------------
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Health
	handler.NewHealthHandler(manager, progressionEngine).Register(r)

	// API routes (handlers already include /api prefix in their paths)
	handler.NewGridHandler(redisClient).Register(r)
	handler.NewNewsHandler(redisClient.AsNewsReader()).Register(r)
	handler.NewQRHandler().Register(r)
	handler.NewMapConfigHandler().Register(r)
	handler.NewDemoLocationHandler().Register(r)
	handler.NewDemoFireHandler(redisClient, &broadcasterAdapter{sio: sioServer}).Register(r)
	handler.NewStatsHandler(redisClient, manager).Register(r)
	handler.NewRankingHandler(redisClient.AsRankingReader()).Register(r)
	handler.NewFeedbackHandler(&feedbackRedisAdapter{c: redisClient}, handler.NewHTTPDiscordSender()).Register(r)

	// Engine.IO / Socket.IO transport
	r.Any("/socket.io/*any", gin.WrapH(eioServer))

	// ---------------------------------------------------------------
	// Start server
	// ---------------------------------------------------------------
	addr := fmt.Sprintf("%s:%s", host, port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Println("Shutting down...")

		reaper.Stop()
		progressionEngine.Stop()
		cleanupEngine.Stop()
		_ = redisClient.Close()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Printf("Server starting on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Server error: %v", err)
	}
	logger.Println("Server stopped")
}

// envOr returns the value of an environment variable or a default.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// --- Adapters to bridge interface differences ---

// broadcasterAdapter bridges socketio.Server to handler.Broadcaster interface.
type broadcasterAdapter struct {
	sio *socketio.Server
}

func (b *broadcasterAdapter) BroadcastToRoom(event string, data interface{}, room string) error {
	_, err := b.sio.BroadcastToRoom("/", room, event, data)
	return err
}

func (b *broadcasterAdapter) Broadcast(event string, data interface{}) error {
	_, err := b.sio.BroadcastToNamespace("/", event, data)
	return err
}

// feedbackRedisAdapter bridges redis.Client to handler.RedisFeedbackRateLimiter.
type feedbackRedisAdapter struct {
	c *hredis.Client
}

func (a *feedbackRedisAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return a.c.IncrBy(ctx, key, 1)
}

func (a *feedbackRedisAdapter) Expire(ctx context.Context, key string, seconds int) error {
	return a.c.Expire(ctx, key, time.Duration(seconds)*time.Second)
}

// chatRedisAdapter bridges redis.Client to sio.RedisChatWriter (Expire takes int seconds).
type chatRedisAdapter struct {
	c *hredis.Client
}

func (a *chatRedisAdapter) LPush(ctx context.Context, key string, value string) error {
	return a.c.LPush(ctx, key, value)
}

func (a *chatRedisAdapter) LTrim(ctx context.Context, key string, start, stop int64) error {
	return a.c.LTrim(ctx, key, start, stop)
}

func (a *chatRedisAdapter) Expire(ctx context.Context, key string, seconds int) error {
	return a.c.Expire(ctx, key, time.Duration(seconds)*time.Second)
}

func (a *chatRedisAdapter) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return a.c.LRange(ctx, key, start, stop)
}

// corsMiddleware returns a Gin middleware that allows all origins.
// Matches Python server CORS config: allow_origins=["*"].
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "*")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}

// parseRedisURL parses a redis:// URL into addr, password, and db.
// Supports formats:
//   - redis://localhost:6379/0
//   - redis://:password@localhost:6379/0
//   - localhost:6379 (plain)
func parseRedisURL(raw string) (addr, password string, db int) {
	db = 0

	if !strings.HasPrefix(raw, "redis://") {
		addr = raw
		return
	}

	raw = strings.TrimPrefix(raw, "redis://")

	// password
	if idx := strings.Index(raw, "@"); idx >= 0 {
		passpart := raw[:idx]
		raw = raw[idx+1:]
		passpart = strings.TrimPrefix(passpart, ":")
		password = passpart
	}

	// db number
	if idx := strings.LastIndex(raw, "/"); idx >= 0 {
		dbStr := raw[idx+1:]
		raw = raw[:idx]
		if n, err := strconv.Atoi(dbStr); err == nil {
			db = n
		}
	}

	addr = raw
	return
}
