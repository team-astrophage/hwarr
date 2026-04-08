package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/homepy/hwarr/server/internal/model"
)

// --- Mock Redis for progression tests ---

type mockRedis struct {
	mu      sync.Mutex
	sets    map[string]map[string]struct{}
	zsets   map[string]map[string]float64
	removed map[string][]interface{}
}

func newMockRedis() *mockRedis {
	return &mockRedis{
		sets:    make(map[string]map[string]struct{}),
		zsets:   make(map[string]map[string]float64),
		removed: make(map[string][]interface{}),
	}
}

func (r *mockRedis) SMembers(_ context.Context, key string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.sets[key]
	result := make([]string, 0, len(s))
	for m := range s {
		result = append(result, m)
	}
	return result, nil
}

func (r *mockRedis) ZCount(_ context.Context, key, min, max string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var minF float64
	if min == "-inf" {
		minF = -1e18
	} else {
		_, _ = fmt.Sscanf(min, "%f", &minF)
	}

	zs := r.zsets[key]
	var count int64
	for _, score := range zs {
		if score >= minF {
			count++
		}
	}
	return count, nil
}

func (r *mockRedis) SRem(_ context.Context, key string, members ...interface{}) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removed[key] = append(r.removed[key], members...)

	s := r.sets[key]
	var removed int64
	for _, m := range members {
		ms := fmt.Sprintf("%v", m)
		if _, ok := s[ms]; ok {
			delete(s, ms)
			removed++
		}
	}
	return removed, nil
}

// Helper: add members to a set
func (r *mockRedis) addToSet(key string, members ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sets[key] == nil {
		r.sets[key] = make(map[string]struct{})
	}
	for _, m := range members {
		r.sets[key][m] = struct{}{}
	}
}

// Helper: add member to sorted set with score
func (r *mockRedis) addToZSet(key, member string, score float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.zsets[key] == nil {
		r.zsets[key] = make(map[string]float64)
	}
	r.zsets[key][member] = score
}

// --- Mock Broadcaster ---

type broadcastCall struct {
	Method    string // "room" or "namespace"
	Namespace string
	Room      string
	Event     string
	Args      []interface{}
}

type mockBroadcaster struct {
	mu    sync.Mutex
	calls []broadcastCall
}

func newMockBroadcaster() *mockBroadcaster {
	return &mockBroadcaster{}
}

func (b *mockBroadcaster) BroadcastToRoom(namespace, room, event string, args ...interface{}) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, broadcastCall{
		Method:    "room",
		Namespace: namespace,
		Room:      room,
		Event:     event,
		Args:      args,
	})
	return 1, nil
}

func (b *mockBroadcaster) BroadcastToNamespace(namespace, event string, args ...interface{}) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, broadcastCall{
		Method:    "namespace",
		Namespace: namespace,
		Event:     event,
		Args:      args,
	})
	return 1, nil
}

func (b *mockBroadcaster) getCalls() []broadcastCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]broadcastCall, len(b.calls))
	copy(result, b.calls)
	return result
}

func (b *mockBroadcaster) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = nil
}

// --- Tests ---

func TestNewFireProgressionEngine_Defaults(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()

	e := NewFireProgressionEngine(r, b, 0, nil)
	if e.scanInterval != 2*time.Second {
		t.Errorf("expected default scan interval 2s, got %v", e.scanInterval)
	}
	if e.IsRunning() {
		t.Error("engine should not be running before Start()")
	}
}

func TestFireProgressionEngine_StartStop(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()

	e := NewFireProgressionEngine(r, b, 100*time.Millisecond, nil)

	e.Start()
	if !e.IsRunning() {
		t.Fatal("engine should be running after Start()")
	}

	// Starting again should be a no-op
	e.Start()

	e.Stop()
	if e.IsRunning() {
		t.Error("engine should not be running after Stop()")
	}
}

func TestScanAndUpdateStages_NoActiveGrids(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	err := e.scanAndUpdateStages(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := b.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected no broadcasts, got %d", len(calls))
	}
}

func TestScanAndUpdateStages_StageTransition_NoneToEmber(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	// Setup: grid "10:20" has 5 active fires
	gridID := "10:20"
	r.addToSet(ActiveGridsKey, gridID)
	futureScore := float64(time.Now().Add(30*time.Minute).UnixMilli()) / 1000.0
	for i := 0; i < 5; i++ {
		r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
	}

	err := e.scanAndUpdateStages(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have stage BULSSSI (1) tracked
	stage := e.GetGridStage(gridID)
	if stage != model.StageBulsssi {
		t.Errorf("expected stage Bulsssi(1), got %d", stage)
	}

	// Should have broadcasts: fire:update (room + namespace) + fire:stage_transition (room + namespace)
	calls := b.getCalls()
	if len(calls) != 4 {
		t.Fatalf("expected 4 broadcast calls, got %d", len(calls))
	}

	// Verify event names
	expectedEvents := []struct {
		method string
		event  string
	}{
		{"room", "fire:update"},
		{"namespace", "fire:update"},
		{"room", "fire:stage_transition"},
		{"namespace", "fire:stage_transition"},
	}
	for i, exp := range expectedEvents {
		if calls[i].Method != exp.method || calls[i].Event != exp.event {
			t.Errorf("call[%d]: expected %s/%s, got %s/%s",
				i, exp.method, exp.event, calls[i].Method, calls[i].Event)
		}
	}

	// Verify fire:update payload has required fields
	payload := calls[0].Args[0].(map[string]interface{})
	if payload["grid_id"] != gridID {
		t.Errorf("expected grid_id=%s, got %v", gridID, payload["grid_id"])
	}
	if payload["gridId"] != gridID {
		t.Errorf("expected gridId=%s (camelCase), got %v", gridID, payload["gridId"])
	}
	if payload["active_count"] != 5 {
		t.Errorf("expected active_count=5, got %v", payload["active_count"])
	}
	if payload["activeCount"] != 5 {
		t.Errorf("expected activeCount=5, got %v", payload["activeCount"])
	}
	if payload["stage"] != 1 {
		t.Errorf("expected stage=1, got %v", payload["stage"])
	}
	if _, ok := payload["timestamp"]; !ok {
		t.Error("expected timestamp in payload")
	}

	// Verify stage_info in payload
	stageInfo, ok := payload["stage_info"].(map[string]interface{})
	if !ok {
		t.Fatal("expected stage_info map in payload")
	}
	if stageInfo["label_ko"] != "불씨" {
		t.Errorf("expected label_ko=불씨, got %v", stageInfo["label_ko"])
	}

	// Verify stage_transition payload
	transPayload := calls[2].Args[0].(map[string]interface{})
	if transPayload["prev_stage"] != int(model.StageNone) {
		t.Errorf("expected prev_stage=0, got %v", transPayload["prev_stage"])
	}
	if transPayload["new_stage"] != int(model.StageBulsssi) {
		t.Errorf("expected new_stage=1, got %v", transPayload["new_stage"])
	}
}

func TestScanAndUpdateStages_NoChangeNoBroadcast(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	gridID := "10:20"
	r.addToSet(ActiveGridsKey, gridID)
	futureScore := float64(time.Now().Add(30*time.Minute).UnixMilli()) / 1000.0
	for i := 0; i < 5; i++ {
		r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
	}

	// First scan: stage None -> Bulsssi (broadcasts)
	_ = e.scanAndUpdateStages(context.Background())
	b.reset()

	// Second scan: still Bulsssi (no change, no broadcast)
	_ = e.scanAndUpdateStages(context.Background())
	calls := b.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected no broadcasts on same stage, got %d", len(calls))
	}
}

func TestScanAndUpdateStages_StageEscalation(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	gridID := "10:20"
	r.addToSet(ActiveGridsKey, gridID)
	futureScore := float64(time.Now().Add(30*time.Minute).UnixMilli()) / 1000.0

	// Start with 5 fires (stage 1)
	for i := 0; i < 5; i++ {
		r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
	}
	_ = e.scanAndUpdateStages(context.Background())
	b.reset()

	// Add more fires to reach stage 2 (10+)
	for i := 5; i < 15; i++ {
		r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
	}
	_ = e.scanAndUpdateStages(context.Background())

	stage := e.GetGridStage(gridID)
	if stage != model.StageModakbul {
		t.Errorf("expected StageModakbul(2), got %d", stage)
	}

	calls := b.getCalls()
	if len(calls) != 4 {
		t.Fatalf("expected 4 broadcasts for stage transition, got %d", len(calls))
	}

	// Verify transition payload
	transPayload := calls[2].Args[0].(map[string]interface{})
	if transPayload["prev_stage"] != int(model.StageBulsssi) {
		t.Errorf("expected prev_stage=1, got %v", transPayload["prev_stage"])
	}
	if transPayload["new_stage"] != int(model.StageModakbul) {
		t.Errorf("expected new_stage=2, got %v", transPayload["new_stage"])
	}
}

func TestScanAndUpdateStages_Stage4TriggersFirefighter(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	gridID := "10:20"
	r.addToSet(ActiveGridsKey, gridID)
	futureScore := float64(time.Now().Add(30*time.Minute).UnixMilli()) / 1000.0

	// Add 120 fires to reach stage 4 (DAEHWAJAE)
	for i := 0; i < 120; i++ {
		r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
	}

	_ = e.scanAndUpdateStages(context.Background())

	stage := e.GetGridStage(gridID)
	if stage != model.StageDaehwajae {
		t.Errorf("expected StageDaehwajae(4), got %d", stage)
	}

	calls := b.getCalls()
	// 4 (stage change) + 2 (firefighter spawn: room + namespace)
	if len(calls) != 6 {
		t.Fatalf("expected 6 broadcasts (4 stage + 2 firefighter), got %d", len(calls))
	}

	// Verify firefighter:spawn events
	ffRoom := calls[4]
	ffNs := calls[5]
	if ffRoom.Event != "firefighter:spawn" || ffRoom.Method != "room" {
		t.Errorf("expected firefighter:spawn room broadcast, got %s/%s", ffRoom.Method, ffRoom.Event)
	}
	if ffNs.Event != "firefighter:spawn" || ffNs.Method != "namespace" {
		t.Errorf("expected firefighter:spawn namespace broadcast, got %s/%s", ffNs.Method, ffNs.Event)
	}

	// Verify firefighter payload
	ffPayload := ffRoom.Args[0].(map[string]interface{})
	if ffPayload["grid_id"] != gridID {
		t.Errorf("expected grid_id=%s, got %v", gridID, ffPayload["grid_id"])
	}
	if ffPayload["status"] != "dispatched" {
		t.Errorf("expected status=dispatched, got %v", ffPayload["status"])
	}
	if ffPayload["target_stage"] != 4 {
		t.Errorf("expected target_stage=4, got %v", ffPayload["target_stage"])
	}
	if ffPayload["fires_removed"] != 0 {
		t.Errorf("expected fires_removed=0, got %v", ffPayload["fires_removed"])
	}
	if ffPayload["remove_per_sweep"] != 2 {
		t.Errorf("expected remove_per_sweep=2, got %v", ffPayload["remove_per_sweep"])
	}
	npcID, ok := ffPayload["npc_id"].(string)
	if !ok || len(npcID) < 3 || npcID[:3] != "ff-" {
		t.Errorf("expected npc_id starting with ff-, got %v", ffPayload["npc_id"])
	}
}

func TestScanAndUpdateStages_GridBecomesNone_Cleanup(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	gridID := "10:20"
	r.addToSet(ActiveGridsKey, gridID)

	// First: add active fires
	futureScore := float64(time.Now().Add(30*time.Minute).UnixMilli()) / 1000.0
	for i := 0; i < 5; i++ {
		r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
	}
	_ = e.scanAndUpdateStages(context.Background())

	// Now make all fires expired (score in the past)
	r.mu.Lock()
	r.zsets[FireKeyPrefix+gridID] = map[string]float64{}
	r.mu.Unlock()

	b.reset()
	_ = e.scanAndUpdateStages(context.Background())

	// Stage should be cleaned up
	stage := e.GetGridStage(gridID)
	if stage != model.StageNone {
		t.Errorf("expected StageNone after all fires expired, got %d", stage)
	}

	// Should have broadcast transition to NONE
	calls := b.getCalls()
	if len(calls) < 4 {
		t.Fatalf("expected at least 4 broadcasts for stage transition to NONE, got %d", len(calls))
	}

	// Grid should have been removed from active_grids
	r.mu.Lock()
	removed := r.removed[ActiveGridsKey]
	r.mu.Unlock()
	if len(removed) == 0 {
		t.Error("expected SRem call to remove grid from active_grids")
	}
}

func TestScanAndUpdateStages_MultipleGrids(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	grid1 := "10:20"
	grid2 := "30:40"
	r.addToSet(ActiveGridsKey, grid1, grid2)

	futureScore := float64(time.Now().Add(30*time.Minute).UnixMilli()) / 1000.0

	// grid1: 5 fires (stage 1), grid2: 50 fires (stage 3)
	for i := 0; i < 5; i++ {
		r.addToZSet(FireKeyPrefix+grid1, fmt.Sprintf("fire-%d", i), futureScore)
	}
	for i := 0; i < 50; i++ {
		r.addToZSet(FireKeyPrefix+grid2, fmt.Sprintf("fire-%d", i), futureScore)
	}

	_ = e.scanAndUpdateStages(context.Background())

	if e.GetGridStage(grid1) != model.StageBulsssi {
		t.Errorf("grid1: expected StageBulsssi, got %d", e.GetGridStage(grid1))
	}
	if e.GetGridStage(grid2) != model.StageHwajae {
		t.Errorf("grid2: expected StageHwajae, got %d", e.GetGridStage(grid2))
	}

	// Both grids transitioned, so at least 8 broadcast calls (4 per grid)
	calls := b.getCalls()
	if len(calls) < 8 {
		t.Errorf("expected at least 8 broadcasts for 2 grids, got %d", len(calls))
	}
}

func TestSetGridStage(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	e.SetGridStage("10:20", model.StageHwajae)
	if e.GetGridStage("10:20") != model.StageHwajae {
		t.Error("SetGridStage should update stage")
	}

	e.SetGridStage("10:20", model.StageNone)
	if e.GetGridStage("10:20") != model.StageNone {
		t.Error("SetGridStage with StageNone should remove from map")
	}

	// GridStages snapshot
	e.SetGridStage("a:b", model.StageBulsssi)
	e.SetGridStage("c:d", model.StageModakbul)
	stages := e.GridStages()
	if len(stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(stages))
	}
}

func TestBroadcastStageChange_PayloadFormat(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	e.broadcastStageChange("10:20", 45, model.StageHwajae, model.StageModakbul)

	calls := b.getCalls()
	if len(calls) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(calls))
	}

	// All calls should target "/" namespace
	for i, c := range calls {
		if c.Namespace != "/" {
			t.Errorf("call[%d] namespace: expected /, got %s", i, c.Namespace)
		}
	}

	// Room calls should target the grid room
	if calls[0].Room != "10:20" {
		t.Errorf("fire:update room should be grid ID, got %s", calls[0].Room)
	}
	if calls[2].Room != "10:20" {
		t.Errorf("fire:stage_transition room should be grid ID, got %s", calls[2].Room)
	}

	// Verify payload completeness
	payload := calls[0].Args[0].(map[string]interface{})
	requiredKeys := []string{"grid_id", "gridId", "active_count", "activeCount", "stage", "stage_info", "timestamp"}
	for _, key := range requiredKeys {
		if _, ok := payload[key]; !ok {
			t.Errorf("missing required key %q in fire:update payload", key)
		}
	}

	// Transition payload
	tp := calls[2].Args[0].(map[string]interface{})
	transKeys := []string{"grid_id", "active_count", "prev_stage", "new_stage", "stage_info", "timestamp"}
	for _, key := range transKeys {
		if _, ok := tp[key]; !ok {
			t.Errorf("missing required key %q in fire:stage_transition payload", key)
		}
	}
}

func TestFireProgressionEngine_IntegrationLoop(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()

	// Use a very short scan interval for testing
	e := NewFireProgressionEngine(r, b, 50*time.Millisecond, nil)

	gridID := "10:20"
	r.addToSet(ActiveGridsKey, gridID)
	futureScore := float64(time.Now().Add(30*time.Minute).UnixMilli()) / 1000.0
	for i := 0; i < 5; i++ {
		r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
	}

	e.Start()
	// Wait for at least 2 scan intervals
	time.Sleep(150 * time.Millisecond)
	e.Stop()

	calls := b.getCalls()
	if len(calls) < 4 {
		t.Errorf("expected at least 4 broadcasts from live engine loop, got %d", len(calls))
	}

	// After stop, no more broadcasts should occur
	prevLen := len(calls)
	time.Sleep(100 * time.Millisecond)
	if len(b.getCalls()) != prevLen {
		t.Error("broadcasts should stop after engine is stopped")
	}
}

func TestStage5_FirefighterSpawn(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	gridID := "10:20"
	r.addToSet(ActiveGridsKey, gridID)
	futureScore := float64(time.Now().Add(30*time.Minute).UnixMilli()) / 1000.0

	// 280 fires -> stage 5 (JEONSO)
	for i := 0; i < 280; i++ {
		r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
	}

	_ = e.scanAndUpdateStages(context.Background())

	stage := e.GetGridStage(gridID)
	if stage != model.StageJeonso {
		t.Errorf("expected StageJeonso(5), got %d", stage)
	}

	// Find firefighter:spawn broadcast
	calls := b.getCalls()
	var ffCalls []broadcastCall
	for _, c := range calls {
		if c.Event == "firefighter:spawn" {
			ffCalls = append(ffCalls, c)
		}
	}
	if len(ffCalls) != 2 {
		t.Fatalf("expected 2 firefighter:spawn broadcasts, got %d", len(ffCalls))
	}

	ffPayload := ffCalls[0].Args[0].(map[string]interface{})
	if ffPayload["remove_per_sweep"] != 3 {
		t.Errorf("stage 5 should have remove_per_sweep=3, got %v", ffPayload["remove_per_sweep"])
	}
	if ffPayload["target_stage"] != 5 {
		t.Errorf("expected target_stage=5, got %v", ffPayload["target_stage"])
	}
}

func TestPublicBroadcastStageChange(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	// Test public method for external callers
	e.BroadcastStageChange("10:20", 150, model.StageDaehwajae, model.StageHwajae)

	calls := b.getCalls()
	// 4 (stage change) + 2 (firefighter for stage 4+)
	if len(calls) != 6 {
		t.Errorf("expected 6 broadcasts from BroadcastStageChange at stage 4, got %d", len(calls))
	}
}

func TestRandomHex(t *testing.T) {
	h1 := randomHex(8)
	if len(h1) != 8 {
		t.Errorf("expected length 8, got %d", len(h1))
	}
	for _, c := range h1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("invalid hex char: %c", c)
		}
	}
}

func TestScanAndUpdateStages_AllStageThresholds(t *testing.T) {
	tests := []struct {
		count    int
		expected model.FireStage
	}{
		{0, model.StageNone},
		{1, model.StageBulsssi},
		{9, model.StageBulsssi},
		{10, model.StageModakbul},
		{39, model.StageModakbul},
		{40, model.StageHwajae},
		{119, model.StageHwajae},
		{120, model.StageDaehwajae},
		{279, model.StageDaehwajae},
		{280, model.StageJeonso},
		{500, model.StageJeonso},
	}

	for _, tt := range tests {
		r := newMockRedis()
		b := newMockBroadcaster()
		e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

		gridID := "test:grid"
		r.addToSet(ActiveGridsKey, gridID)
		futureScore := float64(time.Now().Add(30*time.Minute).UnixMilli()) / 1000.0
		for i := 0; i < tt.count; i++ {
			r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
		}

		_ = e.scanAndUpdateStages(context.Background())
		got := e.GetGridStage(gridID)
		if got != tt.expected {
			t.Errorf("count=%d: expected stage %d, got %d", tt.count, tt.expected, got)
		}
	}
}
