package sio

import (
	"log"
	"sync"
	"time"

	socketio "github.com/homeworldio/socketio-go"
)

// FireUpdate represents a single grid update to be batched and broadcast.
type FireUpdate struct {
	GridID      string
	ActiveCount int
	Stage       int
	EventID     string
	Timestamp   float64
	// Extra fields for camelCase payload (compat)
	Lat *float64
	Lng *float64
}

// FireBatcher accumulates fire updates and flushes them to room-scoped
// broadcasts at a fixed interval. Multiple updates for the same grid within
// one interval are coalesced — only the latest state is sent.
type FireBatcher struct {
	mu        sync.Mutex
	pending   map[string]FireUpdate // gridID -> latest update
	sioServer *socketio.Server
	logger    *log.Logger
	ticker    *time.Ticker
	interval  time.Duration
	done      chan struct{}
}

// NewFireBatcher creates a FireBatcher that flushes every interval.
func NewFireBatcher(sioServer *socketio.Server, interval time.Duration, logger *log.Logger) *FireBatcher {
	if logger == nil {
		logger = log.Default()
	}
	return &FireBatcher{
		pending:   make(map[string]FireUpdate),
		sioServer: sioServer,
		logger:    logger,
		interval:  interval,
	}
}

// Add enqueues a fire update. If an update for the same grid already exists
// in the current batch, it is replaced (last-writer-wins).
func (b *FireBatcher) Add(update FireUpdate) {
	b.mu.Lock()
	b.pending[update.GridID] = update
	b.mu.Unlock()
}

// Start begins the background flush loop.
func (b *FireBatcher) Start() {
	b.ticker = time.NewTicker(b.interval)
	b.done = make(chan struct{})
	go func() {
		for {
			select {
			case <-b.ticker.C:
				b.flush()
			case <-b.done:
				return
			}
		}
	}()
}

// Stop halts the background flush loop.
func (b *FireBatcher) Stop() {
	b.ticker.Stop()
	close(b.done)
}

// flush sends all pending updates via room-scoped BroadcastToRoom and clears
// the pending map. Each grid update is sent only to clients subscribed to
// that grid's room.
func (b *FireBatcher) flush() {
	b.mu.Lock()
	if len(b.pending) == 0 {
		b.mu.Unlock()
		return
	}
	batch := b.pending
	b.pending = make(map[string]FireUpdate)
	b.mu.Unlock()

	for _, u := range batch {
		payload := map[string]interface{}{
			"gridId":      u.GridID,
			"activeCount": u.ActiveCount,
			"stage":       u.Stage,
		}
		if u.Lat != nil {
			payload["lat"] = *u.Lat
		}
		if u.Lng != nil {
			payload["lng"] = *u.Lng
		}
		if _, err := b.sioServer.BroadcastToRoom("/", u.GridID, "fire:update", payload); err != nil {
			b.logger.Printf("fire:update batch broadcast to room %s failed: %v", u.GridID, err)
		}
	}
}
