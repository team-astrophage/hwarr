package sio

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// mockSocketBroadcaster records broadcast calls for verification.
type mockSocketBroadcaster struct {
	mu    sync.Mutex
	calls []mockBroadcastCall
}

type mockBroadcastCall struct {
	Kind      string // "ns" or "room"
	Namespace string
	Room      string
	Event     string
	Args      []interface{}
}

func (m *mockSocketBroadcaster) BroadcastToNamespace(namespace, event string, args ...interface{}) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockBroadcastCall{
		Kind:      "ns",
		Namespace: namespace,
		Event:     event,
		Args:      args,
	})
	return 1, nil
}

func (m *mockSocketBroadcaster) BroadcastToRoom(namespace, room, event string, args ...interface{}) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockBroadcastCall{
		Kind:      "room",
		Namespace: namespace,
		Room:      room,
		Event:     event,
		Args:      args,
	})
	return 1, nil
}

func (m *mockSocketBroadcaster) getCalls() []mockBroadcastCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]mockBroadcastCall, len(m.calls))
	copy(result, m.calls)
	return result
}

func testRedisAddr() string {
	if addr := os.Getenv("REDIS_URL"); addr != "" {
		return addr
	}
	return "localhost:6379"
}

func skipIfNoRedis(t *testing.T) *goredis.Client {
	t.Helper()
	rdb := goredis.NewClient(&goredis.Options{
		Addr:        testRedisAddr(),
		DialTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	return rdb
}

func TestRedisBroadcastAdapter_LocalDelivery(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	sub := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	defer sub.Close()

	local := &mockSocketBroadcaster{}
	adapter := NewRedisBroadcastAdapter(local, rdb, sub, "test:broadcast:local", "instance-A", nil)
	adapter.Start(context.Background())
	defer adapter.Stop()

	// BroadcastToNamespace should deliver locally
	payload := map[string]interface{}{"count": float64(42)}
	_, err := adapter.BroadcastToNamespace("/", "users:count", payload)
	if err != nil {
		t.Fatalf("BroadcastToNamespace failed: %v", err)
	}

	calls := local.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 local call, got %d", len(calls))
	}
	if calls[0].Kind != "ns" || calls[0].Event != "users:count" {
		t.Errorf("unexpected call: %+v", calls[0])
	}
}

func TestRedisBroadcastAdapter_CrossInstanceDelivery(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	channel := "test:broadcast:cross"

	// Instance A
	pubA := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	subA := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	defer pubA.Close()
	defer subA.Close()
	localA := &mockSocketBroadcaster{}
	adapterA := NewRedisBroadcastAdapter(localA, pubA, subA, channel, "instance-A", nil)

	// Instance B
	pubB := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	subB := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	defer pubB.Close()
	defer subB.Close()
	localB := &mockSocketBroadcaster{}
	adapterB := NewRedisBroadcastAdapter(localB, pubB, subB, channel, "instance-B", nil)

	ctx := context.Background()
	adapterA.Start(ctx)
	adapterB.Start(ctx)
	defer adapterA.Stop()
	defer adapterB.Stop()

	// Wait for subscriptions to be ready
	time.Sleep(100 * time.Millisecond)

	// Instance A broadcasts to a room
	payload := map[string]interface{}{"gridId": "grid-1", "activeCount": float64(5)}
	_, err := adapterA.BroadcastToRoom("/", "grid-1", "fire:update", payload)
	if err != nil {
		t.Fatalf("BroadcastToRoom failed: %v", err)
	}

	// Wait for Redis delivery
	time.Sleep(200 * time.Millisecond)

	// Instance A's local should have exactly 1 call (the direct local broadcast)
	callsA := localA.getCalls()
	if len(callsA) != 1 {
		t.Fatalf("instance A: expected 1 local call (no self-loop), got %d", len(callsA))
	}

	// Instance B's local should have exactly 1 call (from Redis subscription)
	callsB := localB.getCalls()
	if len(callsB) != 1 {
		t.Fatalf("instance B: expected 1 call from Redis, got %d", len(callsB))
	}
	if callsB[0].Kind != "room" || callsB[0].Room != "grid-1" || callsB[0].Event != "fire:update" {
		t.Errorf("instance B: unexpected call: %+v", callsB[0])
	}
}

func TestRedisBroadcastAdapter_NoSelfLoop(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	sub := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	defer sub.Close()

	local := &mockSocketBroadcaster{}
	adapter := NewRedisBroadcastAdapter(local, rdb, sub, "test:broadcast:selfloop", "instance-X", nil)
	adapter.Start(context.Background())
	defer adapter.Stop()

	time.Sleep(100 * time.Millisecond)

	_, _ = adapter.BroadcastToNamespace("/", "test:event", map[string]interface{}{"data": "hello"})

	// Wait for potential self-loop
	time.Sleep(200 * time.Millisecond)

	calls := local.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 local call (no self-loop), got %d", len(calls))
	}
}

func TestBroadcastEnvelope_Serialization(t *testing.T) {
	env := broadcastEnvelope{
		Origin:    "inst-1",
		Kind:      "room",
		Namespace: "/",
		Room:      "grid-1",
		Event:     "fire:update",
		Payload:   json.RawMessage(`[{"gridId":"grid-1"}]`),
	}

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded broadcastEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Origin != env.Origin {
		t.Errorf("Origin: got %q, want %q", decoded.Origin, env.Origin)
	}
	if decoded.Kind != env.Kind {
		t.Errorf("Kind: got %q, want %q", decoded.Kind, env.Kind)
	}
	if decoded.Room != env.Room {
		t.Errorf("Room: got %q, want %q", decoded.Room, env.Room)
	}
	if decoded.Event != env.Event {
		t.Errorf("Event: got %q, want %q", decoded.Event, env.Event)
	}
}

func TestRedisBroadcastAdapter_NamespaceBroadcastCrossInstance(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	channel := "test:broadcast:ns-cross"

	pubA := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	subA := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	defer pubA.Close()
	defer subA.Close()
	localA := &mockSocketBroadcaster{}
	adapterA := NewRedisBroadcastAdapter(localA, pubA, subA, channel, "ns-A", nil)

	pubB := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	subB := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	defer pubB.Close()
	defer subB.Close()
	localB := &mockSocketBroadcaster{}
	adapterB := NewRedisBroadcastAdapter(localB, pubB, subB, channel, "ns-B", nil)

	ctx := context.Background()
	adapterA.Start(ctx)
	adapterB.Start(ctx)
	defer adapterA.Stop()
	defer adapterB.Stop()

	time.Sleep(100 * time.Millisecond)

	// Namespace broadcast from A
	_, _ = adapterA.BroadcastToNamespace("/", "users:count", map[string]interface{}{"count": float64(10)})

	time.Sleep(200 * time.Millisecond)

	callsB := localB.getCalls()
	if len(callsB) != 1 {
		t.Fatalf("instance B: expected 1 namespace broadcast, got %d", len(callsB))
	}
	if callsB[0].Kind != "ns" || callsB[0].Event != "users:count" {
		t.Errorf("instance B: unexpected call: %+v", callsB[0])
	}
}
