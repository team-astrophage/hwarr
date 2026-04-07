// Package engine implements background scheduling tasks for fire lifecycle management.
//
// FireProgressionEngine periodically scans all active fire grids in Redis,
// detects fire stage transitions, and broadcasts updates to connected clients
// via Socket.IO. It mirrors the Python server/jobs/fire_progression.py behavior.
package engine

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/homepy/hwarr/server/internal/model"
)

// RedisProgressionReader abstracts the Redis operations needed by the progression scan.
type RedisProgressionReader interface {
	// SMembers returns all members of the set at key.
	SMembers(ctx context.Context, key string) ([]string, error)
	// ZCount counts sorted set members with score between min and max (inclusive, string-encoded).
	ZCount(ctx context.Context, key, min, max string) (int64, error)
	// SRem removes members from the set at key.
	SRem(ctx context.Context, key string, members ...interface{}) (int64, error)
}

// Broadcaster abstracts Socket.IO broadcasting operations.
type Broadcaster interface {
	// BroadcastToRoom sends an event to all sockets in a specific room.
	BroadcastToRoom(namespace, room, event string, args ...interface{}) (int, error)
	// BroadcastToNamespace sends an event to all sockets in a namespace.
	BroadcastToNamespace(namespace, event string, args ...interface{}) (int, error)
}

// FireProgressionEngine is a background scheduler that periodically:
//  1. Scans all active fire grid keys in Redis (2-second default interval)
//  2. Recalculates fire stage for each grid based on active (non-expired) fire count
//  3. Detects stage transitions and broadcasts fire:update + fire:stage_transition
//  4. Broadcasts firefighter:spawn event when stage >= 4 (visual effect only)
//
// This is a 1:1 port of Python server/jobs/fire_progression.py's progression loop.
type FireProgressionEngine struct {
	redis       RedisProgressionReader
	broadcaster Broadcaster
	logger      *log.Logger

	scanInterval time.Duration

	// mu protects gridStages
	mu         sync.RWMutex
	gridStages map[string]model.FireStage

	// running state
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewFireProgressionEngine creates a new engine.
//
// Parameters:
//   - redis: Redis client for reading fire data
//   - broadcaster: Socket.IO server for broadcasting events
//   - scanInterval: interval between progression scans (default 2s)
func NewFireProgressionEngine(
	redis RedisProgressionReader,
	broadcaster Broadcaster,
	scanInterval time.Duration,
	logger *log.Logger,
) *FireProgressionEngine {
	if logger == nil {
		logger = log.Default()
	}
	if scanInterval <= 0 {
		scanInterval = 2 * time.Second
	}

	return &FireProgressionEngine{
		redis:        redis,
		broadcaster:  broadcaster,
		logger:       logger,
		scanInterval: scanInterval,
		gridStages:   make(map[string]model.FireStage),
	}
}

// IsRunning returns whether the engine is currently running.
func (e *FireProgressionEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// GridStages returns a snapshot of the current tracked stage per grid.
func (e *FireProgressionEngine) GridStages() map[string]model.FireStage {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make(map[string]model.FireStage, len(e.gridStages))
	for k, v := range e.gridStages {
		result[k] = v
	}
	return result
}

// SetGridStage sets the stage for a grid (used by external fire registration
// to keep the engine's stage tracking in sync without waiting for the next scan).
func (e *FireProgressionEngine) SetGridStage(gridID string, stage model.FireStage) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if stage == model.StageNone {
		delete(e.gridStages, gridID)
	} else {
		e.gridStages[gridID] = stage
	}
}

// GetGridStage returns the current tracked stage for a grid.
func (e *FireProgressionEngine) GetGridStage(gridID string) model.FireStage {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.gridStages[gridID]
}

// Start begins the progression scan loop. It is non-blocking and launches
// a goroutine that runs until Stop() is called.
func (e *FireProgressionEngine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		e.logger.Println("FireProgressionEngine already running")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.running = true
	e.mu.Unlock()

	e.wg.Add(1)
	go e.progressionLoop(ctx)

	e.logger.Printf("FireProgressionEngine started (scan=%.1fs)", e.scanInterval.Seconds())
}

// Stop gracefully shuts down the progression loop.
func (e *FireProgressionEngine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	e.cancel()
	e.mu.Unlock()

	e.wg.Wait()
	e.logger.Println("FireProgressionEngine stopped")
}

// progressionLoop is the main goroutine: scan grids, detect stage changes, broadcast.
func (e *FireProgressionEngine) progressionLoop(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.scanAndUpdateStages(ctx); err != nil {
				if ctx.Err() != nil {
					return // context cancelled, expected
				}
				e.logger.Printf("Error in progression scan: %v", err)
			}
		}
	}
}

// scanAndUpdateStages scans all active grids and broadcasts any stage transitions.
// This is the core logic matching Python's _scan_and_update_stages().
func (e *FireProgressionEngine) scanAndUpdateStages(ctx context.Context) error {
	now := float64(time.Now().UnixMilli()) / 1000.0
	nowStr := fmt.Sprintf("%f", now)

	gridIDs, err := e.getActiveGridIDs(ctx)
	if err != nil {
		return fmt.Errorf("get active grids: %w", err)
	}

	for _, gridID := range gridIDs {
		activeCount, err := e.getActiveCount(ctx, gridID, nowStr)
		if err != nil {
			e.logger.Printf("Error counting fires for grid %s: %v", gridID, err)
			continue
		}

		newStage := model.GetStage(activeCount)

		e.mu.RLock()
		prevStage := e.gridStages[gridID]
		e.mu.RUnlock()

		if newStage != prevStage {
			e.mu.Lock()
			e.gridStages[gridID] = newStage
			e.mu.Unlock()

			e.broadcastStageChange(gridID, activeCount, newStage, prevStage)

			// Trigger firefighter NPC spawn on escalation to stage 4+
			if newStage >= model.StageDaehwajae {
				e.broadcastFirefighterSpawn(gridID, activeCount)
			}
		}

		// Clean up tracking for empty grids
		if newStage == model.StageNone {
			e.mu.Lock()
			delete(e.gridStages, gridID)
			e.mu.Unlock()
			_ = e.removeActiveGrid(ctx, gridID)
		}
	}

	return nil
}

// getActiveGridIDs returns all grid IDs in the active_grids Redis set.
func (e *FireProgressionEngine) getActiveGridIDs(ctx context.Context) ([]string, error) {
	return e.redis.SMembers(ctx, ActiveGridsKey)
}

// getActiveCount counts active (non-expired) fires in a grid cell.
// Active fires = sorted set members with score > current time.
func (e *FireProgressionEngine) getActiveCount(ctx context.Context, gridID, nowStr string) (int, error) {
	key := FireKeyPrefix + gridID
	count, err := e.redis.ZCount(ctx, key, nowStr, "+inf")
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// removeActiveGrid removes a grid from the active tracking set.
func (e *FireProgressionEngine) removeActiveGrid(ctx context.Context, gridID string) error {
	_, err := e.redis.SRem(ctx, ActiveGridsKey, gridID)
	return err
}

// broadcastStageChange broadcasts fire:update and fire:stage_transition events.
// Matches Python _broadcast_stage_change exactly.
func (e *FireProgressionEngine) broadcastStageChange(
	gridID string,
	activeCount int,
	newStage model.FireStage,
	prevStage model.FireStage,
) {
	ts := float64(time.Now().UnixMilli()) / 1000.0

	state := model.BuildGridState(gridID, activeCount, nil, nil)
	payload := map[string]interface{}{
		"grid_id":      state.GridID,
		"active_count": state.ActiveCount,
		"stage":        state.Stage,
		"stage_info": map[string]interface{}{
			"stage":                state.StageInfo.Stage,
			"label_ko":             state.StageInfo.LabelKo,
			"label_en":             state.StageInfo.LabelEn,
			"triggers_firefighter": state.StageInfo.TriggersFirefighter,
		},
		"timestamp": ts,
		// camelCase aliases for mock server compat
		"gridId":      gridID,
		"activeCount": activeCount,
	}

	// 1) Room-scoped update — reaches viewport subscribers
	if _, err := e.broadcaster.BroadcastToRoom("/", gridID, "fire:update", payload); err != nil {
		e.logger.Printf("Failed to broadcast fire:update to room %s: %v", gridID, err)
	}

	// 2) Global update — reaches map overview clients
	if _, err := e.broadcaster.BroadcastToNamespace("/", "fire:update", payload); err != nil {
		e.logger.Printf("Failed to broadcast fire:update globally: %v", err)
	}

	// 3) Dedicated stage transition event with prev/new for animations
	transitionPayload := map[string]interface{}{
		"grid_id":      gridID,
		"active_count": activeCount,
		"prev_stage":   int(prevStage),
		"new_stage":    int(newStage),
		"stage_info":   payload["stage_info"],
		"timestamp":    ts,
	}
	if _, err := e.broadcaster.BroadcastToRoom("/", gridID, "fire:stage_transition", transitionPayload); err != nil {
		e.logger.Printf("Failed to broadcast fire:stage_transition to room %s: %v", gridID, err)
	}
	if _, err := e.broadcaster.BroadcastToNamespace("/", "fire:stage_transition", transitionPayload); err != nil {
		e.logger.Printf("Failed to broadcast fire:stage_transition globally: %v", err)
	}

	prevName := stageName(prevStage)
	newName := stageName(newStage)
	e.logger.Printf("Stage change: grid=%s stage=%s->%s count=%d", gridID, prevName, newName, activeCount)
}

// broadcastFirefighterSpawn broadcasts firefighter:spawn event (visual effect only).
// Matches Python check_and_spawn_firefighter + _broadcast_firefighter_spawn.
func (e *FireProgressionEngine) broadcastFirefighterSpawn(gridID string, activeCount int) {
	stage := model.GetStage(activeCount)
	cfg := model.StageConfigs[stage]
	if !cfg.TriggersFirefighter {
		return
	}

	npcID := fmt.Sprintf("ff-%s", randomHex(8))
	removePerSweep := cfg.FirefighterRemoveCount
	if removePerSweep < 1 {
		removePerSweep = 1
	}

	payload := map[string]interface{}{
		"npc_id":           npcID,
		"grid_id":          gridID,
		"status":           "dispatched",
		"dispatched_at":    float64(time.Now().UnixMilli()) / 1000.0,
		"fires_removed":    0,
		"remove_per_sweep": removePerSweep,
		"target_stage":     int(stage),
	}

	if _, err := e.broadcaster.BroadcastToRoom("/", gridID, "firefighter:spawn", payload); err != nil {
		e.logger.Printf("Failed to broadcast firefighter:spawn to room %s: %v", gridID, err)
	}
	if _, err := e.broadcaster.BroadcastToNamespace("/", "firefighter:spawn", payload); err != nil {
		e.logger.Printf("Failed to broadcast firefighter:spawn globally: %v", err)
	}

	e.logger.Printf("Firefighter spawn broadcast: npc_id=%s grid=%s stage=%d", npcID, gridID, int(stage))
}

// BroadcastStageChange is the public version of broadcastStageChange, used by
// external callers (e.g., fire registration) to broadcast immediate stage changes
// without waiting for the next scan.
func (e *FireProgressionEngine) BroadcastStageChange(
	gridID string,
	activeCount int,
	newStage model.FireStage,
	prevStage model.FireStage,
) {
	e.broadcastStageChange(gridID, activeCount, newStage, prevStage)

	if newStage >= model.StageDaehwajae {
		e.broadcastFirefighterSpawn(gridID, activeCount)
	}
}

// stageName returns a human-readable label for a fire stage.
func stageName(stage model.FireStage) string {
	if cfg, ok := model.StageConfigs[stage]; ok {
		return cfg.LabelEn
	}
	return "unknown"
}

// randomHex generates a random hex string of the given length.
// Uses a simple time-based approach (good enough for NPC IDs).
func randomHex(n int) string {
	const hex = "0123456789abcdef"
	// Use UnixNano for entropy
	seed := time.Now().UnixNano()
	b := make([]byte, n)
	for i := range b {
		seed = seed*6364136223846793005 + 1442695040888963407 // LCG
		idx := int(math.Abs(float64(seed))) % len(hex)
		b[i] = hex[idx]
	}
	return string(b)
}
