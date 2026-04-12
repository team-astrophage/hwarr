package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockRedisFireWriter implements RedisFireWriter for testing.
type mockRedisFireWriter struct {
	counts map[string]int64
	err    error
}

func (m *mockRedisFireWriter) ZAdd(_ context.Context, key string, _ float64, _ string) error {
	if m.err != nil {
		return m.err
	}
	if m.counts == nil {
		m.counts = make(map[string]int64)
	}
	m.counts[key]++
	return nil
}

func (m *mockRedisFireWriter) ZCount(_ context.Context, key, _, _ string) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.counts[key], nil
}

func (m *mockRedisFireWriter) SAdd(_ context.Context, _, _ string) error {
	return m.err
}

// mockBroadcaster implements Broadcaster for testing.
type mockBroadcaster struct {
	events []string
}

func (m *mockBroadcaster) BroadcastToRoom(event string, _ interface{}, _ string) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockBroadcaster) Broadcast(event string, _ interface{}) error {
	m.events = append(m.events, event)
	return nil
}

func setupDemoFireRouter(redis RedisFireWriter, broadcaster Broadcaster) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewDemoFireHandler(redis, broadcaster, nil)
	h.Register(r)
	return r
}

func TestDemoFire_RandomLocation(t *testing.T) {
	mock := &mockRedisFireWriter{counts: map[string]int64{}}
	bc := &mockBroadcaster{}
	router := setupDemoFireRouter(mock, bc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/demo/fire", nil)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp DemoFireResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.GridID == "" {
		t.Error("expected non-empty grid_id")
	}
	if resp.EventID == "" {
		t.Error("expected non-empty event_id")
	}
	if !strings.HasPrefix(resp.EventID, "fire-demo-") {
		t.Errorf("event_id should start with 'fire-demo-', got %s", resp.EventID)
	}
	if resp.Demo != true {
		t.Error("expected demo=true")
	}
	if resp.Location.ID == "" {
		t.Error("expected non-empty location ID")
	}
	if resp.Location.Demo != true {
		t.Error("expected location.demo=true")
	}
	if resp.Stage < 0 {
		t.Error("expected non-negative stage")
	}
}

func TestDemoFire_SpecificLocation(t *testing.T) {
	mock := &mockRedisFireWriter{counts: map[string]int64{}}
	bc := &mockBroadcaster{}
	router := setupDemoFireRouter(mock, bc)

	w := httptest.NewRecorder()
	body := `{"location_id": "gangnam"}`
	req, _ := http.NewRequest("POST", "/api/demo/fire", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp DemoFireResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Location.ID != "gangnam" {
		t.Errorf("expected location_id=gangnam, got %s", resp.Location.ID)
	}
	if resp.Location.Name != "강남역" {
		t.Errorf("expected location name=강남역, got %s", resp.Location.Name)
	}
}

func TestDemoFire_InvalidLocation(t *testing.T) {
	mock := &mockRedisFireWriter{counts: map[string]int64{}}
	bc := &mockBroadcaster{}
	router := setupDemoFireRouter(mock, bc)

	w := httptest.NewRecorder()
	body := `{"location_id": "nonexistent"}`
	req, _ := http.NewRequest("POST", "/api/demo/fire", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDemoFire_RedisError(t *testing.T) {
	mock := &mockRedisFireWriter{err: errors.New("connection refused")}
	bc := &mockBroadcaster{}
	router := setupDemoFireRouter(mock, bc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/demo/fire", nil)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestDemoFire_BroadcastEvents(t *testing.T) {
	mock := &mockRedisFireWriter{counts: map[string]int64{}}
	bc := &mockBroadcaster{}
	router := setupDemoFireRouter(mock, bc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/demo/fire", nil)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	// Should broadcast fire:ignite and fire:update (room-scoped)
	if len(bc.events) != 2 {
		t.Fatalf("expected 2 broadcast events, got %d", len(bc.events))
	}
	if bc.events[0] != "fire:ignite" {
		t.Errorf("expected first broadcast=fire:ignite, got %s", bc.events[0])
	}
	if bc.events[1] != "fire:update" {
		t.Errorf("expected second broadcast=fire:update, got %s", bc.events[1])
	}
}

func TestDemoFire_NilBroadcaster(t *testing.T) {
	mock := &mockRedisFireWriter{counts: map[string]int64{}}
	router := setupDemoFireRouter(mock, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/demo/fire", nil)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 even with nil broadcaster, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDemoFire_SpreadBroadcastOnThreshold(t *testing.T) {
	// Pre-populate with 500 fires so threshold is exceeded and spread occurs
	mock := &mockRedisFireWriter{counts: map[string]int64{}}
	// Pick a known demo location grid key — use gangnam
	// gangnam lat=37.4979, lng=127.0276 → grid key will be computed at runtime
	// Instead, we set a wildcard high count on all fire: keys via ZCount override
	spreadMock := &mockRedisFireWriterWithSpread{
		inner:     mock,
		threshold: FireSpreadThreshold,
	}
	bc := &mockBroadcaster{}
	h := NewDemoFireHandler(spreadMock, bc, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r)

	w := httptest.NewRecorder()
	body := `{"location_id": "gangnam"}`
	req, _ := http.NewRequest("POST", "/api/demo/fire", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Should broadcast fire:ignite, fire:global_update, AND fire:spread
	hasSpread := false
	for _, ev := range bc.events {
		if ev == "fire:spread" {
			hasSpread = true
		}
	}
	if !hasSpread {
		t.Errorf("expected fire:spread broadcast when threshold exceeded, events=%v", bc.events)
	}
}

// mockRedisFireWriterWithSpread returns a count >= threshold for any fire: key
// on the first ZCount call, then 0 for subsequent calls (to let the fire land).
type mockRedisFireWriterWithSpread struct {
	inner     *mockRedisFireWriter
	threshold int64
	callCount int
}

func (m *mockRedisFireWriterWithSpread) ZAdd(ctx context.Context, key string, score float64, member string) error {
	return m.inner.ZAdd(ctx, key, score, member)
}

func (m *mockRedisFireWriterWithSpread) ZCount(_ context.Context, key, _, _ string) (int64, error) {
	m.callCount++
	if m.callCount == 1 {
		return m.threshold, nil // first grid at threshold → triggers spread
	}
	return 0, nil // neighbor has capacity
}

func (m *mockRedisFireWriterWithSpread) SAdd(ctx context.Context, key, member string) error {
	return m.inner.SAdd(ctx, key, member)
}

func TestDemoFire_ResponseFields(t *testing.T) {
	mock := &mockRedisFireWriter{counts: map[string]int64{}}
	bc := &mockBroadcaster{}
	router := setupDemoFireRouter(mock, bc)

	w := httptest.NewRecorder()
	body := `{"location_id": "coex"}`
	req, _ := http.NewRequest("POST", "/api/demo/fire", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	// Parse raw JSON to verify all fields exist
	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	requiredFields := []string{"grid_id", "event_id", "active_count", "stage", "stage_info", "location", "demo"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	// Verify location sub-fields
	loc, ok := raw["location"].(map[string]interface{})
	if !ok {
		t.Fatal("location should be an object")
	}
	locFields := []string{"id", "name", "lat", "lng", "grid_id", "description", "demo"}
	for _, field := range locFields {
		if _, ok := loc[field]; !ok {
			t.Errorf("missing location field: %s", field)
		}
	}
}
