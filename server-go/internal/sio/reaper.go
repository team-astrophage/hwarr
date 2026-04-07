// reaper.go implements the stale connection reaper — a background goroutine
// that periodically checks for and disconnects clients that have not sent
// a heartbeat within the configured timeout.
//
// This mirrors Python server/sio/connection_manager.py start_reaper / _reaper_loop:
//   - Runs every ReaperIntervalSec (15s)
//   - Disconnects connections that exceed HeartbeatTimeoutSec (30s) without heartbeat
//   - Falls back to manual removal if Socket.IO disconnect fails
package sio

import (
	"log"
	"sync"
	"time"
)

// DisconnectFunc is the callback used by the reaper to forcefully disconnect
// a stale Socket.IO session. Typically wraps socketio.Server.Disconnect.
type DisconnectFunc func(namespace, sid string) error

// Reaper periodically scans for stale connections and disconnects them.
// It follows the same goroutine lifecycle pattern as engine.CleanupEngine.
type Reaper struct {
	manager    *ConnectionManager
	disconnect DisconnectFunc
	interval   time.Duration
	logger     *log.Logger

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}

	// nowFunc allows injecting a custom clock for testing.
	nowFunc func() time.Time
}

// ReaperOption configures optional Reaper settings.
type ReaperOption func(*Reaper)

// WithReaperLogger sets a custom logger.
func WithReaperLogger(l *log.Logger) ReaperOption {
	return func(r *Reaper) { r.logger = l }
}

// WithReaperNowFunc injects a custom clock (primarily for testing).
func WithReaperNowFunc(fn func() time.Time) ReaperOption {
	return func(r *Reaper) { r.nowFunc = fn }
}

// WithReaperInterval overrides the default reaper interval (primarily for testing).
func WithReaperInterval(d time.Duration) ReaperOption {
	return func(r *Reaper) { r.interval = d }
}

// NewReaper creates a Reaper that scans the given ConnectionManager
// and uses the disconnect callback to remove stale sessions.
func NewReaper(manager *ConnectionManager, disconnect DisconnectFunc, opts ...ReaperOption) *Reaper {
	r := &Reaper{
		manager:    manager,
		disconnect: disconnect,
		interval:   ReaperIntervalSec * time.Second,
		logger:     log.Default(),
		nowFunc:    time.Now,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Start begins the reaper loop in a background goroutine.
// Safe to call multiple times; subsequent calls are no-ops.
func (r *Reaper) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		r.logger.Println("Stale connection reaper already running")
		return
	}

	r.running = true
	r.stopCh = make(chan struct{})
	r.doneCh = make(chan struct{})

	go r.loop()
	r.logger.Printf("Stale connection reaper started (interval=%s)", r.interval)
}

// Stop gracefully stops the reaper and waits for it to finish.
func (r *Reaper) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	r.running = false
	close(r.stopCh)
	r.mu.Unlock()

	<-r.doneCh
	r.logger.Println("Stale connection reaper stopped")
}

// IsRunning reports whether the reaper loop is active.
func (r *Reaper) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// loop is the main goroutine that ticks at the configured interval.
func (r *Reaper) loop() {
	defer close(r.doneCh)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.reap()
		}
	}
}

// reap executes a single reap pass — identifies stale connections and
// disconnects them. Exported-logic helper kept private; use RunReap
// for external/test invocation.
func (r *Reaper) reap() {
	stale := r.manager.GetStaleSIDs()
	if len(stale) == 0 {
		return
	}

	for _, sid := range stale {
		r.logger.Printf("Reaping stale connection: %s", sid)
		if r.disconnect != nil {
			if err := r.disconnect("/", sid); err != nil {
				// Force-remove from tracking if disconnect fails
				// (matches Python fallback behavior)
				r.logger.Printf("Disconnect failed for %s, force-removing: %v", sid, err)
				r.manager.Remove(sid)
			}
		} else {
			// No disconnect callback — just remove from tracking
			r.manager.Remove(sid)
		}
	}

	r.logger.Printf("Reaped %d stale connection(s)", len(stale))
}

// RunReap executes a single reap pass. Exported so it can be called
// directly in tests or on-demand (matches CleanupEngine.RunCleanup pattern).
func (r *Reaper) RunReap() {
	r.reap()
}
