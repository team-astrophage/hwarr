package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/homepy/hwarr/server/internal/auth"
	"github.com/homepy/hwarr/server/internal/config"
	"github.com/homepy/hwarr/server/internal/engine"
	"github.com/homepy/hwarr/server/internal/geodata"
	"github.com/homepy/hwarr/server/internal/handler"
	hredis "github.com/homepy/hwarr/server/internal/redis"
	"github.com/homepy/hwarr/server/internal/sio"
	engineio "github.com/homepy/hwarr/server/pkg/engineio"
	socketio "github.com/homeworldio/socketio-go"
)

// Run initializes all components and starts the HTTP server.
// It blocks until a termination signal is received.
func Run(cfg *config.Config) error {
	logger := log.New(os.Stdout, "[hwarr] ", log.LstdFlags|log.Lmsgprefix)

	// Redis
	redisClient := hredis.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.RedisTLS)
	ctx := context.Background()
	if err := redisClient.Ping(ctx); err != nil {
		logger.Printf("WARNING: Failed to connect to Redis at %s: %v — running without fire engine", cfg.RedisAddr, err)
	} else {
		logger.Printf("Connected to Redis: %s", cfg.RedisAddr)
	}

	// Token service
	tokenService := auth.NewTokenService(cfg.TokenSecret, cfg.TokenTTLMin)

	// Socket.IO + Engine.IO
	sioServer, eioServer := setupSocketServers(cfg, logger)

	// Rate limiter — created early so its cleanup callback can be passed to the
	// disconnect handler registered inside NewHandler, avoiding the previous bug
	// where a second OnDisconnect call overwrote the first.
	rl := sio.NewRateLimiter(func(sid string) {
		sioServer.DisconnectAll(sid, "rate limit exceeded")
	})

	// Connection manager + SIO event handlers
	sioHandler := sio.NewHandler(sioServer, logger, tokenService, rl.Remove)
	manager := sioHandler.Manager()

	// Admin region resolver (optional — for daily ranking)
	resolver := geodata.LoadOrNil(cfg.AdminGeoJSONPath)
	if resolver != nil {
		logger.Printf("Admin region resolver loaded: %d regions", resolver.RegionCount())
	}

	batcher := sio.NewFireBatcher(sioServer, 200*time.Millisecond, logger)
	batcher.Start()

	registerSocketEvents(sioServer, manager, redisClient, resolver, logger, batcher, rl)

	// Background engines
	progressionEngine, cleanupEngine := startBackgroundEngines(redisClient, sioServer, logger)

	// Stale connection reaper
	reaper := sio.NewReaper(manager, func(ns, sid string) error {
		sioServer.DisconnectAll(sid, "stale connection")
		return nil
	})
	reaper.Start()

	logger.Println("Background engines started")

	// HTTP server
	r := setupRouter(cfg, manager, progressionEngine, redisClient, resolver, sioServer, eioServer, tokenService)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
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

		batcher.Stop()
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
		return fmt.Errorf("server error: %w", err)
	}
	logger.Println("Server stopped")
	return nil
}

// setupSocketServers creates and bridges the Engine.IO and Socket.IO servers.
func setupSocketServers(cfg *config.Config, logger *log.Logger) (*socketio.Server, *engineio.Server) {
	sioServer := socketio.NewServer()
	sioServer.Logger = logger

	eioConfig := engineio.ServerConfig{
		PingInterval:   10 * time.Second,
		PingTimeout:    5 * time.Second,
		MaxPayload:     1_000_000,
		Upgrades:       []string{"websocket"},
		AllowedOrigins: cfg.AllowedOrigins,
	}
	eioServer := engineio.NewServer(eioConfig)

	// Bridge Engine.IO <-> Socket.IO
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
			if result != nil && result.ResponsePacket != nil {
				encoded, encErr := socketio.Encode(result.ResponsePacket)
				if encErr == nil {
					session.Send([]byte(encoded))
				}
			}
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

	return sioServer, eioServer
}

// registerSocketEvents wires all Socket.IO event handlers.
func registerSocketEvents(sioServer *socketio.Server, manager *sio.ConnectionManager, redisClient *hredis.Client, resolver *geodata.AdminRegionResolver, logger *log.Logger, batcher *sio.FireBatcher, rl *sio.RateLimiter) {
	// Rate limiting middleware — must be registered before event handlers.
	// The RateLimiter itself is created in Run() and its cleanup is handled
	// by the disconnect handler registered in NewHandler to avoid overwriting.
	sioServer.Use(sio.NewRateLimitMiddleware(rl, logger))

	// Fire events
	sio.RegisterFireIgniteHandler(sioServer, manager, redisClient, resolver, logger, batcher)
	sio.RegisterFireStateHandler(sioServer, redisClient, logger)
	sio.RegisterSubscribeViewportHandler(sioServer, manager, redisClient, logger)

	// Compat events (mock server)
	sio.RegisterFireCompatHandler(sioServer, manager, redisClient, resolver, logger, batcher)
	sio.RegisterGetFiresCompatHandler(sioServer, redisClient, logger)

	// Chat events
	chatWriter := redisClient.AsChatWriter()
	sio.RegisterChatJoinHandler(sioServer, manager, chatWriter, logger)
	sio.RegisterChatSendHandler(sioServer, manager, chatWriter, logger)
	sio.RegisterChatLeaveHandler(sioServer, manager, logger)
}

// startBackgroundEngines creates and starts the fire progression and cleanup engines.
func startBackgroundEngines(redisClient *hredis.Client, sioServer *socketio.Server, logger *log.Logger) (*engine.FireProgressionEngine, *engine.CleanupEngine) {
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

	progressionEngine.Start()
	cleanupEngine.Start()

	return progressionEngine, cleanupEngine
}

// setupRouter creates the Gin router with all HTTP routes.
func setupRouter(
	cfg *config.Config,
	manager *sio.ConnectionManager,
	progressionEngine *engine.FireProgressionEngine,
	redisClient *hredis.Client,
	resolver *geodata.AdminRegionResolver,
	sioServer *socketio.Server,
	eioServer *engineio.Server,
	tokenService *auth.TokenService,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware(cfg.AllowedOrigins))

	// Health
	handler.NewHealthHandler(manager, progressionEngine).Register(r)

	// REST API
	handler.NewGridHandler(redisClient).Register(r)
	handler.NewNewsHandler(redisClient.AsNewsReader()).Register(r)
	handler.NewQRHandler().Register(r)
	handler.NewMapConfigHandler().Register(r)
	handler.NewDemoLocationHandler().Register(r)
	handler.NewDemoFireHandler(redisClient, &broadcasterAdapter{sio: sioServer}, resolver).Register(r)
	handler.NewStatsHandler(redisClient, manager).Register(r)
	handler.NewRankingHandler(redisClient.AsRankingReader()).Register(r)
	handler.NewFeedbackHandler(redisClient.AsFeedbackRateLimiter(), handler.NewHTTPDiscordSender()).Register(r)

	// Token endpoint for Socket.IO authentication
	r.GET("/api/token", handler.TokenHandler(tokenService))

	// Engine.IO / Socket.IO transport
	r.Any("/socket.io/*any", gin.WrapH(eioServer))

	return r
}

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
