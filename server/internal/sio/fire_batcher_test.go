package sio

import (
	"sync"
	"testing"
	"time"

	socketio "github.com/homeworldio/socketio-go"
)

// broadcastRecord captures a single BroadcastToRoom invocation.
type broadcastRecord struct {
	Room  string
	Event string
	Data  string // encoded packet
}

// newTestSIOServer returns a socketio.Server wired to record broadcasts.
func newTestSIOServer() (*socketio.Server, *[]broadcastRecord, *sync.Mutex) {
	s := socketio.NewServer()
	var records []broadcastRecord
	var mu sync.Mutex

	s.SendTo = func(sid string, data string) error {
		mu.Lock()
		records = append(records, broadcastRecord{Room: sid, Event: "", Data: data})
		mu.Unlock()
		return nil
	}

	return s, &records, &mu
}

func TestFireBatcher_Add_And_Flush(t *testing.T) {
	sioServer := socketio.NewServer()

	var sent []broadcastRecord
	var mu sync.Mutex

	// Wire up SendTo so BroadcastToRoom actually works.
	// BroadcastToRoom sends to room members; we need a member in the room.
	ns := sioServer.Of("/")
	ns.Rooms.Join("client-1", "grid-A")

	sioServer.SendTo = func(sid string, data string) error {
		mu.Lock()
		sent = append(sent, broadcastRecord{Room: sid, Data: data})
		mu.Unlock()
		return nil
	}

	batcher := NewFireBatcher(sioServer, 50*time.Millisecond, nil)
	batcher.Add(FireUpdate{
		GridID:      "grid-A",
		ActiveCount: 5,
		Stage:       2,
	})

	// Manually flush (no need to start the ticker)
	batcher.flush()

	mu.Lock()
	defer mu.Unlock()

	if len(sent) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(sent))
	}
	if sent[0].Room != "client-1" {
		t.Errorf("expected broadcast to client-1, got %s", sent[0].Room)
	}

	// Pending should be empty after flush
	batcher.mu.Lock()
	pendingLen := len(batcher.pending)
	batcher.mu.Unlock()
	if pendingLen != 0 {
		t.Errorf("expected 0 pending after flush, got %d", pendingLen)
	}
}

func TestFireBatcher_Coalesce_SameGrid(t *testing.T) {
	sioServer := socketio.NewServer()

	var sent []broadcastRecord
	var mu sync.Mutex

	ns := sioServer.Of("/")
	ns.Rooms.Join("client-1", "grid-A")

	sioServer.SendTo = func(sid string, data string) error {
		mu.Lock()
		sent = append(sent, broadcastRecord{Room: sid, Data: data})
		mu.Unlock()
		return nil
	}

	batcher := NewFireBatcher(sioServer, 50*time.Millisecond, nil)

	// Add multiple updates for the same grid
	batcher.Add(FireUpdate{GridID: "grid-A", ActiveCount: 1, Stage: 1})
	batcher.Add(FireUpdate{GridID: "grid-A", ActiveCount: 5, Stage: 2})
	batcher.Add(FireUpdate{GridID: "grid-A", ActiveCount: 10, Stage: 3})

	batcher.flush()

	mu.Lock()
	defer mu.Unlock()

	// Only 1 broadcast should be sent (last update wins)
	if len(sent) != 1 {
		t.Fatalf("expected 1 broadcast (coalesced), got %d", len(sent))
	}
}

func TestFireBatcher_MultipleGrids(t *testing.T) {
	sioServer := socketio.NewServer()

	var sent []broadcastRecord
	var mu sync.Mutex

	ns := sioServer.Of("/")
	ns.Rooms.Join("client-1", "grid-A")
	ns.Rooms.Join("client-2", "grid-B")

	sioServer.SendTo = func(sid string, data string) error {
		mu.Lock()
		sent = append(sent, broadcastRecord{Room: sid, Data: data})
		mu.Unlock()
		return nil
	}

	batcher := NewFireBatcher(sioServer, 50*time.Millisecond, nil)
	batcher.Add(FireUpdate{GridID: "grid-A", ActiveCount: 1, Stage: 1})
	batcher.Add(FireUpdate{GridID: "grid-B", ActiveCount: 2, Stage: 1})

	batcher.flush()

	mu.Lock()
	defer mu.Unlock()

	// Each grid has one member, so 2 broadcasts total
	if len(sent) != 2 {
		t.Fatalf("expected 2 broadcasts, got %d", len(sent))
	}
}

func TestFireBatcher_FlushEmpty(t *testing.T) {
	sioServer := socketio.NewServer()
	sioServer.SendTo = func(sid string, data string) error {
		t.Fatal("should not broadcast when pending is empty")
		return nil
	}

	batcher := NewFireBatcher(sioServer, 50*time.Millisecond, nil)
	batcher.flush() // should be a no-op
}

func TestFireBatcher_StopPreventsFlush(t *testing.T) {
	sioServer := socketio.NewServer()

	called := false
	sioServer.SendTo = func(sid string, data string) error {
		called = true
		return nil
	}

	batcher := NewFireBatcher(sioServer, 10*time.Millisecond, nil)
	batcher.Start()
	batcher.Stop()

	// Add after stop — the ticker loop has exited so flush won't run.
	ns := sioServer.Of("/")
	ns.Rooms.Join("client-1", "grid-A")
	batcher.Add(FireUpdate{GridID: "grid-A", ActiveCount: 1, Stage: 1})

	time.Sleep(50 * time.Millisecond)

	if called {
		t.Error("expected no broadcast after Stop")
	}
}

func TestFireBatcher_TickerFlush(t *testing.T) {
	sioServer := socketio.NewServer()

	var mu sync.Mutex
	var count int

	ns := sioServer.Of("/")
	ns.Rooms.Join("client-1", "grid-A")

	sioServer.SendTo = func(sid string, data string) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}

	batcher := NewFireBatcher(sioServer, 20*time.Millisecond, nil)
	batcher.Start()
	defer batcher.Stop()

	batcher.Add(FireUpdate{GridID: "grid-A", ActiveCount: 1, Stage: 1})

	// Wait for at least one tick
	time.Sleep(60 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if count == 0 {
		t.Error("expected at least one broadcast from ticker flush")
	}
}
