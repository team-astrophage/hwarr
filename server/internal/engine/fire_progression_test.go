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

	// Room-scoped only: fire:update (room) + fire:stage_transition (room)
	calls := b.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 broadcast calls, got %d", len(calls))
	}

	expectedEvents := []struct {
		method string
		event  string
	}{
		{"room", "fire:update"},
		{"room", "fire:stage_transition"},
	}
	for i, exp := range expectedEvents {
		if calls[i].Method != exp.method || calls[i].Event != exp.event {
			t.Errorf("call[%d]: expected %s/%s, got %s/%s",
				i, exp.method, exp.event, calls[i].Method, calls[i].Event)
		}
	}

	// Verify fire:update payload has required fields
	payload := calls[0].Args[0].(map[string]interface{})
	if payload["gridId"] != gridID {
		t.Errorf("expected gridId=%s, got %v", gridID, payload["gridId"])
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
	if _, ok := payload["lat"]; !ok {
		t.Error("expected lat in payload")
	}

	// Verify stageInfo in payload (serialized via model.GridState → FireStageInfo struct)
	stageInfo, ok := payload["stageInfo"].(model.FireStageInfo)
	if !ok {
		t.Fatalf("expected stageInfo FireStageInfo in payload, got %T", payload["stageInfo"])
	}
	if stageInfo.LabelKo != "불씨" {
		t.Errorf("expected labelKo=불씨, got %v", stageInfo.LabelKo)
	}

	// Verify stage_transition payload (room call is at index 1 now)
	transPayload := calls[1].Args[0].(map[string]interface{})
	if transPayload["prevStage"] != int(model.StageNone) {
		t.Errorf("expected prevStage=0, got %v", transPayload["prevStage"])
	}
	if transPayload["newStage"] != int(model.StageBulsssi) {
		t.Errorf("expected newStage=1, got %v", transPayload["newStage"])
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

	// Add more fires to reach stage 2 (6+)
	for i := 5; i < 15; i++ {
		r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
	}
	_ = e.scanAndUpdateStages(context.Background())

	stage := e.GetGridStage(gridID)
	if stage != model.StageModakbul {
		t.Errorf("expected StageModakbul(2), got %d", stage)
	}

	calls := b.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 broadcasts for stage transition (room-only), got %d", len(calls))
	}

	// Verify transition payload (index 1: room fire:stage_transition)
	transPayload := calls[1].Args[0].(map[string]interface{})
	if transPayload["prevStage"] != int(model.StageBulsssi) {
		t.Errorf("expected prevStage=1, got %v", transPayload["prevStage"])
	}
	if transPayload["newStage"] != int(model.StageModakbul) {
		t.Errorf("expected newStage=2, got %v", transPayload["newStage"])
	}
}

func TestScanAndUpdateStages_Stage4TriggersFirefighter(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	gridID := "10:20"
	r.addToSet(ActiveGridsKey, gridID)
	futureScore := float64(time.Now().Add(30*time.Minute).UnixMilli()) / 1000.0

	// Add 72 fires to reach stage 4 (DAEHWAJAE)
	for i := 0; i < 72; i++ {
		r.addToZSet(FireKeyPrefix+gridID, fmt.Sprintf("fire-%d", i), futureScore)
	}

	_ = e.scanAndUpdateStages(context.Background())

	stage := e.GetGridStage(gridID)
	if stage != model.StageDaehwajae {
		t.Errorf("expected StageDaehwajae(4), got %d", stage)
	}

	calls := b.getCalls()
	// 2 (stage change: fire:update + fire:stage_transition) + 1 (firefighter:spawn room)
	if len(calls) != 3 {
		t.Fatalf("expected 3 broadcasts (2 stage + 1 firefighter), got %d", len(calls))
	}

	// Verify firefighter:spawn event
	ffRoom := calls[2]
	if ffRoom.Event != "firefighter:spawn" || ffRoom.Method != "room" {
		t.Errorf("expected firefighter:spawn room broadcast, got %s/%s", ffRoom.Method, ffRoom.Event)
	}

	// Verify firefighter payload
	ffPayload := ffRoom.Args[0].(map[string]interface{})
	if ffPayload["gridId"] != gridID {
		t.Errorf("expected gridId=%s, got %v", gridID, ffPayload["gridId"])
	}
	if ffPayload["status"] != "dispatched" {
		t.Errorf("expected status=dispatched, got %v", ffPayload["status"])
	}
	if ffPayload["targetStage"] != 4 {
		t.Errorf("expected targetStage=4, got %v", ffPayload["targetStage"])
	}
	if ffPayload["firesRemoved"] != 0 {
		t.Errorf("expected firesRemoved=0, got %v", ffPayload["firesRemoved"])
	}
	if ffPayload["removePerSweep"] != 2 {
		t.Errorf("expected removePerSweep=2, got %v", ffPayload["removePerSweep"])
	}
	npcID, ok := ffPayload["npcId"].(string)
	if !ok || len(npcID) < 3 || npcID[:3] != "ff-" {
		t.Errorf("expected npcId starting with ff-, got %v", ffPayload["npcId"])
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

	// Should have broadcast transition to NONE (2 room broadcasts)
	calls := b.getCalls()
	if len(calls) < 2 {
		t.Fatalf("expected at least 2 broadcasts for stage transition to NONE, got %d", len(calls))
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

	// Both grids transitioned, so at least 4 broadcast calls (2 per grid: fire:update + fire:stage_transition)
	calls := b.getCalls()
	if len(calls) < 4 {
		t.Errorf("expected at least 4 broadcasts for 2 grids, got %d", len(calls))
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
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (room-only), got %d", len(calls))
	}

	for i, c := range calls {
		if c.Namespace != "/" {
			t.Errorf("call[%d] namespace: expected /, got %s", i, c.Namespace)
		}
		if c.Method != "room" {
			t.Errorf("call[%d] method: expected room, got %s", i, c.Method)
		}
		if c.Room != "10:20" {
			t.Errorf("call[%d] room: expected 10:20, got %s", i, c.Room)
		}
	}

	// Verify payload completeness
	payload := calls[0].Args[0].(map[string]interface{})
	requiredKeys := []string{"gridId", "lat", "lng", "activeCount", "stage", "stageInfo", "timestamp"}
	for _, key := range requiredKeys {
		if _, ok := payload[key]; !ok {
			t.Errorf("missing required key %q in fire:update payload", key)
		}
	}

	// Transition payload
	tp := calls[1].Args[0].(map[string]interface{})
	transKeys := []string{"gridId", "activeCount", "prevStage", "newStage", "stageInfo", "timestamp"}
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
	if len(calls) < 2 {
		t.Errorf("expected at least 2 broadcasts from live engine loop, got %d", len(calls))
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

	// 170 fires -> stage 5 (JEONSO)
	for i := 0; i < 170; i++ {
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
	if len(ffCalls) != 1 {
		t.Fatalf("expected 1 firefighter:spawn broadcast (room-only), got %d", len(ffCalls))
	}

	ffPayload := ffCalls[0].Args[0].(map[string]interface{})
	if ffPayload["removePerSweep"] != 3 {
		t.Errorf("stage 5 should have removePerSweep=3, got %v", ffPayload["removePerSweep"])
	}
	if ffPayload["targetStage"] != 5 {
		t.Errorf("expected targetStage=5, got %v", ffPayload["targetStage"])
	}
}

func TestPublicBroadcastStageChange(t *testing.T) {
	r := newMockRedis()
	b := newMockBroadcaster()
	e := NewFireProgressionEngine(r, b, 2*time.Second, nil)

	// Test public method for external callers
	e.BroadcastStageChange("10:20", 150, model.StageDaehwajae, model.StageHwajae)

	calls := b.getCalls()
	// 2 (stage change: fire:update + fire:stage_transition) + 1 (firefighter for stage 4+)
	if len(calls) != 3 {
		t.Errorf("expected 3 broadcasts from BroadcastStageChange at stage 4, got %d", len(calls))
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
		{5, model.StageBulsssi},
		{6, model.StageModakbul},
		{23, model.StageModakbul},
		{24, model.StageHwajae},
		{71, model.StageHwajae},
		{72, model.StageDaehwajae},
		{169, model.StageDaehwajae},
		{170, model.StageJeonso},
		{300, model.StageJeonso},
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
