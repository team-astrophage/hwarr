// cleanup.go implements the 60-second interval expired fire event cleanup loop.
//
// This mirrors Python server/jobs/fire_progression.py _cleanup_loop / _run_cleanup:
//  1. Iterates all active grid IDs from the "active_grids" Redis set
//  2. Removes expired fire events (sorted set score < now) via ZREMRANGEBYSCORE
//  3. Deletes empty fire keys and removes grids from the active set
//  4. Notifies the stage tracker to remove entries for fully-extinguished grids
package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// CleanupRedis abstracts the Redis operations needed by the cleanup loop.
// These are separate from RedisProgressionReader because cleanup requires
// write operations (ZREMRANGEBYSCORE, DEL, etc.) in addition to reads.
type CleanupRedis interface {
	// SMembers returns all members of the set at key.
	SMembers(ctx context.Context, key string) ([]string, error)
	// ZRemRangeByScore removes all members in a sorted set with score between min and max.
	// Returns the number of removed members.
	ZRemRangeByScore(ctx context.Context, key, min, max string) (int64, error)
	// ZCard returns the number of members in a sorted set.
	ZCard(ctx context.Context, key string) (int64, error)
	// Del deletes one or more keys.
	Del(ctx context.Context, keys ...string) error
	// SRem removes a member from a set.
	SRem(ctx context.Context, key, member string) error
}

// GridStageTracker allows the cleanup loop to remove stage tracking entries
// for grids that become empty. This is optional — pass nil if stage tracking
// is managed elsewhere.
//
// FireProgressionEngine satisfies this via RemoveStage (mapped to SetGridStage(gridID, StageNone)).
type GridStageTracker interface {
	RemoveStage(gridID string)
}

// CleanupEngine runs a periodic loop that removes expired fire events
// from Redis sorted sets and cleans up empty grid keys.
//
// It implements handler.EngineStatusChecker via IsRunning().
type CleanupEngine struct {
	redis    CleanupRedis
	tracker  GridStageTracker
	interval time.Duration
	logger   *log.Logger

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}

	// nowFunc allows injecting a custom clock for testing.
	nowFunc func() time.Time
}

// CleanupOption configures optional CleanupEngine settings.
type CleanupOption func(*CleanupEngine)

// WithCleanupLogger sets a custom logger.
func WithCleanupLogger(l *log.Logger) CleanupOption {
	return func(e *CleanupEngine) { e.logger = l }
}

// WithGridStageTracker sets the stage tracker for empty-grid cleanup.
func WithGridStageTracker(t GridStageTracker) CleanupOption {
	return func(e *CleanupEngine) { e.tracker = t }
}

// WithCleanupNowFunc injects a custom clock (primarily for testing).
func WithCleanupNowFunc(fn func() time.Time) CleanupOption {
	return func(e *CleanupEngine) { e.nowFunc = fn }
}

// NewCleanupEngine creates a CleanupEngine with the given interval.
// Default interval is 60 seconds if interval <= 0.
func NewCleanupEngine(redis CleanupRedis, interval time.Duration, opts ...CleanupOption) *CleanupEngine {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	e := &CleanupEngine{
		redis:    redis,
		interval: interval,
		logger:   log.Default(),
		nowFunc:  time.Now,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Start begins the cleanup loop in a background goroutine.
// It is safe to call Start multiple times; subsequent calls are no-ops.
func (e *CleanupEngine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		e.logger.Println("CleanupEngine already running")
		return
	}

	e.running = true
	e.stopCh = make(chan struct{})
	e.doneCh = make(chan struct{})

	go e.loop()
	e.logger.Printf("CleanupEngine started (interval=%s)", e.interval)
}

// Stop gracefully stops the cleanup loop and waits for it to finish.
func (e *CleanupEngine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	close(e.stopCh)
	e.mu.Unlock()

	<-e.doneCh
	e.logger.Println("CleanupEngine stopped")
}

// IsRunning reports whether the cleanup loop is active.
// Implements handler.EngineStatusChecker.
func (e *CleanupEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// loop is the main goroutine that ticks at the configured interval.
func (e *CleanupEngine) loop() {
	defer close(e.doneCh)

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			if err := e.RunCleanup(context.Background()); err != nil {
				e.logger.Printf("CleanupEngine error: %v", err)
			}
		}
	}
}

// RunCleanup executes a single cleanup pass. It removes all expired fire events
// (sorted set members with score < current time) from every active grid, and
// fully cleans up grids that become empty.
//
// Exported so it can be called directly in tests or on-demand.
func (e *CleanupEngine) RunCleanup(ctx context.Context) error {
	now := e.nowFunc()
	nowStr := formatCleanupScore(float64(now.Unix()))

	gridIDs, err := e.redis.SMembers(ctx, ActiveGridsKey)
	if err != nil {
		return err
	}

	var cleaned int64

	for _, gridID := range gridIDs {
		key := FireKeyPrefix + gridID

		// Remove members with score < now (expired)
		removed, err := e.redis.ZRemRangeByScore(ctx, key, "-inf", nowStr)
		if err != nil {
			e.logger.Printf("CleanupEngine ZRemRangeByScore %s error: %v", key, err)
			continue
		}
		if removed > 0 {
			cleaned += removed
		}

		// Check if the sorted set is now empty
		remaining, err := e.redis.ZCard(ctx, key)
		if err != nil {
			e.logger.Printf("CleanupEngine ZCard %s error: %v", key, err)
			continue
		}

		if remaining == 0 {
			// Delete the empty sorted set key
			if err := e.redis.Del(ctx, key); err != nil {
				e.logger.Printf("CleanupEngine Del %s error: %v", key, err)
			}
			// Remove from active_grids tracking set
			if err := e.redis.SRem(ctx, ActiveGridsKey, gridID); err != nil {
				e.logger.Printf("CleanupEngine SRem %s error: %v", gridID, err)
			}
			// Remove stage tracking if tracker is available
			if e.tracker != nil {
				e.tracker.RemoveStage(gridID)
			}
		}
	}

	if cleaned > 0 {
		e.logger.Printf("Cleanup: removed %d expired fire events", cleaned)
	}

	return nil
}

// formatCleanupScore converts a float64 timestamp to the string format expected by Redis.
func formatCleanupScore(ts float64) string {
	return fmt.Sprintf("%.6f", ts)
}

// RemoveStage on FireProgressionEngine adapts it to the GridStageTracker interface.
// This is defined here so the cleanup engine can clear stage tracking for empty grids.
func (e *FireProgressionEngine) RemoveStage(gridID string) {
	e.SetGridStage(gridID, 0) // StageNone = 0
}
