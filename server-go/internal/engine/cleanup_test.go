package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

// --- Mock Redis for Cleanup ---

// cleanupEntry represents a sorted set member with score.
type cleanupEntry struct {
	member string
	score  float64
}

// mockCleanupRedis is an in-memory mock of CleanupRedis for testing.
type mockCleanupRedis struct {
	mu         sync.Mutex
	sets       map[string]map[string]struct{} // regular sets
	sortedSets map[string][]cleanupEntry      // sorted sets
	calls      []string                       // operation log
}

func newMockCleanupRedis() *mockCleanupRedis {
	return &mockCleanupRedis{
		sets:       make(map[string]map[string]struct{}),
		sortedSets: make(map[string][]cleanupEntry),
	}
}

func (m *mockCleanupRedis) SMembers(_ context.Context, key string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, fmt.Sprintf("SMembers(%s)", key))

	s, ok := m.sets[key]
	if !ok {
		return nil, nil
	}
	var members []string
	for member := range s {
		members = append(members, member)
	}
	sort.Strings(members)
	return members, nil
}

func (m *mockCleanupRedis) ZRemRangeByScore(_ context.Context, key, min, max string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, fmt.Sprintf("ZRemRangeByScore(%s,%s,%s)", key, min, max))

	entries := m.sortedSets[key]
	if len(entries) == 0 {
		return 0, nil
	}

	maxScore, _ := strconv.ParseFloat(max, 64)
	var kept []cleanupEntry
	var removed int64
	for _, e := range entries {
		if (min == "-inf" || e.score >= 0) && e.score <= maxScore {
			removed++
		} else {
			kept = append(kept, e)
		}
	}
	m.sortedSets[key] = kept
	return removed, nil
}

func (m *mockCleanupRedis) ZCard(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, fmt.Sprintf("ZCard(%s)", key))
	return int64(len(m.sortedSets[key])), nil
}

func (m *mockCleanupRedis) Del(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		m.calls = append(m.calls, fmt.Sprintf("Del(%s)", key))
		delete(m.sortedSets, key)
	}
	return nil
}

func (m *mockCleanupRedis) SRem(_ context.Context, key, member string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, fmt.Sprintf("SRem(%s,%s)", key, member))
	if s, ok := m.sets[key]; ok {
		delete(s, member)
	}
	return nil
}

// --- helpers to populate mock ---

func (m *mockCleanupRedis) addToSet(key, member string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sets[key] == nil {
		m.sets[key] = make(map[string]struct{})
	}
	m.sets[key][member] = struct{}{}
}

func (m *mockCleanupRedis) addToSortedSet(key, member string, score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sortedSets[key] = append(m.sortedSets[key], cleanupEntry{member: member, score: score})
}

func (m *mockCleanupRedis) sortedSetLen(key string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sortedSets[key])
}

func (m *mockCleanupRedis) setMembers(key string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for member := range m.sets[key] {
		out = append(out, member)
	}
	sort.Strings(out)
	return out
}

// --- Mock GridStageTracker ---

type mockStageTracker struct {
	mu      sync.Mutex
	removed []string
}

func (t *mockStageTracker) RemoveStage(gridID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.removed = append(t.removed, gridID)
}

func (t *mockStageTracker) removedGrids() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, len(t.removed))
	copy(out, t.removed)
	return out
}

// --- Tests ---

func TestRunCleanup_RemovesExpiredEvents(t *testing.T) {
	redis := newMockCleanupRedis()

	// Grid with 3 fires: 2 expired (score < now), 1 active (score > now)
	now := time.Unix(1000, 0)
	redis.addToSet(ActiveGridsKey, "g1")
	redis.addToSortedSet("fire:g1", "fire-aaa", 500)  // expired
	redis.addToSortedSet("fire:g1", "fire-bbb", 900)  // expired
	redis.addToSortedSet("fire:g1", "fire-ccc", 2000) // active

	engine := NewCleanupEngine(redis, 60*time.Second,
		WithCleanupNowFunc(func() time.Time { return now }),
		WithCleanupLogger(log.New(os.Stderr, "[test] ", 0)),
	)

	err := engine.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("RunCleanup error: %v", err)
	}

	// Should have removed 2 expired entries, kept 1 active
	if n := redis.sortedSetLen("fire:g1"); n != 1 {
		t.Errorf("expected 1 remaining entry, got %d", n)
	}

	// Grid should still be in active_grids (not empty)
	members := redis.setMembers(ActiveGridsKey)
	if len(members) != 1 || members[0] != "g1" {
		t.Errorf("expected g1 still in active_grids, got %v", members)
	}
}

func TestRunCleanup_RemovesEmptyGrid(t *testing.T) {
	redis := newMockCleanupRedis()
	tracker := &mockStageTracker{}

	now := time.Unix(1000, 0)
	redis.addToSet(ActiveGridsKey, "g1")
	redis.addToSortedSet("fire:g1", "fire-aaa", 500) // expired
	redis.addToSortedSet("fire:g1", "fire-bbb", 800) // expired

	engine := NewCleanupEngine(redis, 60*time.Second,
		WithCleanupNowFunc(func() time.Time { return now }),
		WithGridStageTracker(tracker),
		WithCleanupLogger(log.New(os.Stderr, "[test] ", 0)),
	)

	err := engine.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("RunCleanup error: %v", err)
	}

	// Sorted set should be deleted
	if n := redis.sortedSetLen("fire:g1"); n != 0 {
		t.Errorf("expected empty sorted set, got %d entries", n)
	}

	// Grid should be removed from active_grids
	members := redis.setMembers(ActiveGridsKey)
	if len(members) != 0 {
		t.Errorf("expected empty active_grids, got %v", members)
	}

	// Stage tracker should have been notified
	removed := tracker.removedGrids()
	if len(removed) != 1 || removed[0] != "g1" {
		t.Errorf("expected tracker removal of g1, got %v", removed)
	}
}

func TestRunCleanup_NoActiveGrids(t *testing.T) {
	redis := newMockCleanupRedis()
	now := time.Unix(1000, 0)

	engine := NewCleanupEngine(redis, 60*time.Second,
		WithCleanupNowFunc(func() time.Time { return now }),
	)

	err := engine.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("RunCleanup error: %v", err)
	}
	// Should succeed with no-op
}

func TestRunCleanup_MultipleGrids(t *testing.T) {
	redis := newMockCleanupRedis()
	tracker := &mockStageTracker{}
	now := time.Unix(1000, 0)

	// g1: all expired -> should be cleaned up completely
	redis.addToSet(ActiveGridsKey, "g1")
	redis.addToSortedSet("fire:g1", "fire-1", 500)

	// g2: mix of expired and active -> partial cleanup
	redis.addToSet(ActiveGridsKey, "g2")
	redis.addToSortedSet("fire:g2", "fire-2a", 500)  // expired
	redis.addToSortedSet("fire:g2", "fire-2b", 2000) // active

	// g3: all active -> no cleanup
	redis.addToSet(ActiveGridsKey, "g3")
	redis.addToSortedSet("fire:g3", "fire-3", 2000)

	engine := NewCleanupEngine(redis, 60*time.Second,
		WithCleanupNowFunc(func() time.Time { return now }),
		WithGridStageTracker(tracker),
		WithCleanupLogger(log.New(os.Stderr, "[test] ", 0)),
	)

	err := engine.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("RunCleanup error: %v", err)
	}

	// g1 should be fully cleaned
	if n := redis.sortedSetLen("fire:g1"); n != 0 {
		t.Errorf("g1: expected 0 entries, got %d", n)
	}
	// g2 should have 1 remaining
	if n := redis.sortedSetLen("fire:g2"); n != 1 {
		t.Errorf("g2: expected 1 entry, got %d", n)
	}
	// g3 should be unchanged
	if n := redis.sortedSetLen("fire:g3"); n != 1 {
		t.Errorf("g3: expected 1 entry, got %d", n)
	}

	// Only g1 should be removed from active_grids
	members := redis.setMembers(ActiveGridsKey)
	expected := []string{"g2", "g3"}
	if len(members) != len(expected) {
		t.Errorf("expected active_grids=%v, got %v", expected, members)
	}

	// Only g1 should trigger stage tracker removal
	removed := tracker.removedGrids()
	if len(removed) != 1 || removed[0] != "g1" {
		t.Errorf("expected tracker removal of [g1], got %v", removed)
	}
}

func TestCleanupEngine_StartStop(t *testing.T) {
	redis := newMockCleanupRedis()
	engine := NewCleanupEngine(redis, 10*time.Millisecond,
		WithCleanupLogger(log.New(os.Stderr, "[test] ", 0)),
	)

	if engine.IsRunning() {
		t.Fatal("expected not running before Start")
	}

	engine.Start()
	if !engine.IsRunning() {
		t.Fatal("expected running after Start")
	}

	// Double start should be a no-op
	engine.Start()
	if !engine.IsRunning() {
		t.Fatal("expected still running after double Start")
	}

	engine.Stop()
	if engine.IsRunning() {
		t.Fatal("expected not running after Stop")
	}

	// Double stop should be safe
	engine.Stop()
}

func TestCleanupEngine_PeriodicExecution(t *testing.T) {
	redis := newMockCleanupRedis()
	now := time.Unix(1000, 0)

	// Add one expired fire
	redis.addToSet(ActiveGridsKey, "g1")
	redis.addToSortedSet("fire:g1", "fire-1", 500) // expired

	engine := NewCleanupEngine(redis, 50*time.Millisecond,
		WithCleanupNowFunc(func() time.Time { return now }),
		WithCleanupLogger(log.New(os.Stderr, "[test] ", 0)),
	)

	engine.Start()
	defer engine.Stop()

	// Wait for at least one tick
	time.Sleep(120 * time.Millisecond)

	// The expired fire should have been cleaned up
	if n := redis.sortedSetLen("fire:g1"); n != 0 {
		t.Errorf("expected cleanup after tick, still have %d entries", n)
	}
}

func TestCleanupEngine_NilTracker(t *testing.T) {
	redis := newMockCleanupRedis()
	now := time.Unix(1000, 0)

	redis.addToSet(ActiveGridsKey, "g1")
	redis.addToSortedSet("fire:g1", "fire-1", 500) // expired

	// No tracker passed — should not panic
	engine := NewCleanupEngine(redis, 60*time.Second,
		WithCleanupNowFunc(func() time.Time { return now }),
	)

	err := engine.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("RunCleanup with nil tracker error: %v", err)
	}
}

func TestNewCleanupEngine_DefaultInterval(t *testing.T) {
	redis := newMockCleanupRedis()
	engine := NewCleanupEngine(redis, 0)
	if engine.interval != 60*time.Second {
		t.Errorf("expected default interval 60s, got %s", engine.interval)
	}
}

func TestFireProgressionEngine_RemoveStage(t *testing.T) {
	// Test that FireProgressionEngine satisfies GridStageTracker interface
	r := newMockRedis()
	b := newMockBroadcaster()
	pe := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	// Set a stage, then remove via RemoveStage
	pe.SetGridStage("g1", 3) // StageHwajae
	if pe.GetGridStage("g1") != 3 {
		t.Fatal("expected stage 3 after SetGridStage")
	}

	pe.RemoveStage("g1")
	if pe.GetGridStage("g1") != 0 {
		t.Errorf("expected StageNone after RemoveStage, got %d", pe.GetGridStage("g1"))
	}

	// Verify it satisfies the interface
	var _ GridStageTracker = pe
}

func TestCleanupEngine_WithProgressionTracker(t *testing.T) {
	// Integration: CleanupEngine uses FireProgressionEngine as stage tracker
	cleanupRedis := newMockCleanupRedis()
	progressionRedis := newMockRedis()
	broadcaster := newMockBroadcaster()

	pe := NewFireProgressionEngine(progressionRedis, broadcaster, 2*time.Second, nil)
	pe.SetGridStage("g1", 3) // pre-set stage

	now := time.Unix(1000, 0)
	cleanupRedis.addToSet(ActiveGridsKey, "g1")
	cleanupRedis.addToSortedSet("fire:g1", "fire-1", 500) // expired

	ce := NewCleanupEngine(cleanupRedis, 60*time.Second,
		WithCleanupNowFunc(func() time.Time { return now }),
		WithGridStageTracker(pe),
	)

	err := ce.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("RunCleanup error: %v", err)
	}

	// After cleanup, the progression engine's stage for g1 should be cleared
	if pe.GetGridStage("g1") != 0 {
		t.Errorf("expected progression engine stage cleared for g1, got %d", pe.GetGridStage("g1"))
	}
}
