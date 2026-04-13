// stats_broadcaster.go periodically broadcasts global fire statistics to all
// connected Socket.IO clients. Unlike fire:update which is scoped to viewport
// rooms, stats:fires delivers nationwide totals so the UI can show a count
// that does not depend on the user's current map view.
package engine

import (
	"context"
	"log"
	"sync"
	"time"

	socketio "github.com/homeworldio/socketio-go"
)

// StatsRedis abstracts the Redis operations needed by the stats broadcaster.
type StatsRedis interface {
	SCard(ctx context.Context, key string) (int64, error)
}

// StatsBroadcaster abstracts the Socket.IO broadcast target so tests can
// inject a fake without pulling in the full server dependency.
type StatsBroadcaster interface {
	BroadcastToNamespace(namespace, event string, args ...interface{}) (int, error)
}

// StatsEngine runs a periodic loop that reads the global active fire count
// from Redis and broadcasts it to every connected client.
type StatsEngine struct {
	redis       StatsRedis
	broadcaster StatsBroadcaster
	interval    time.Duration
	logger      *log.Logger

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewStatsEngine creates a StatsEngine with the given interval.
// Default interval is 5 seconds if interval <= 0.
func NewStatsEngine(redis StatsRedis, broadcaster StatsBroadcaster, interval time.Duration, logger *log.Logger) *StatsEngine {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if logger == nil {
		logger = log.Default()
	}
	return &StatsEngine{
		redis:       redis,
		broadcaster: broadcaster,
		interval:    interval,
		logger:      logger,
	}
}

// Start begins the broadcast loop in a background goroutine.
func (e *StatsEngine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		e.logger.Println("StatsEngine already running")
		return
	}

	e.running = true
	e.stopCh = make(chan struct{})
	e.doneCh = make(chan struct{})

	go e.loop()
	e.logger.Printf("StatsEngine started (interval=%s)", e.interval)
}

// Stop gracefully stops the loop and waits for it to finish.
func (e *StatsEngine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	close(e.stopCh)
	e.mu.Unlock()

	<-e.doneCh
	e.logger.Println("StatsEngine stopped")
}

// IsRunning reports whether the broadcast loop is active.
func (e *StatsEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

func (e *StatsEngine) loop() {
	defer close(e.doneCh)

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	// Emit once immediately so freshly connected clients do not wait a full
	// interval for their first count.
	e.broadcastOnce(context.Background())

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.broadcastOnce(context.Background())
		}
	}
}

// broadcastOnce reads the active grid count and broadcasts it. Exported-like
// semantics via a separate method keep the loop simple and testable.
func (e *StatsEngine) broadcastOnce(ctx context.Context) {
	n, err := e.redis.SCard(ctx, ActiveGridsKey)
	if err != nil {
		e.logger.Printf("StatsEngine SCard error: %v", err)
		return
	}
	payload := map[string]interface{}{"total": n}
	if _, err := e.broadcaster.BroadcastToNamespace("/", "stats:fires", payload); err != nil {
		e.logger.Printf("StatsEngine broadcast error: %v", err)
	}
}

// Compile-time check that socketio.Server satisfies the StatsBroadcaster interface.
var _ StatsBroadcaster = (*socketio.Server)(nil)
