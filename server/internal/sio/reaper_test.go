package sio

import (
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"
)

func TestReaper_ReapStaleConnections(t *testing.T) {
	logger := log.New(os.Stderr, "[reaper-test] ", log.LstdFlags)
	mgr := NewConnectionManager(logger)

	// Track disconnected SIDs
	var mu sync.Mutex
	disconnected := make([]string, 0)
	disconnectFn := func(ns, sid string) error {
		mu.Lock()
		disconnected = append(disconnected, sid)
		mu.Unlock()
		// Simulate what the real disconnect handler would do
		mgr.Remove(sid)
		return nil
	}

	reaper := NewReaper(mgr, disconnectFn, WithReaperLogger(logger))

	// Add a connection and make it stale by backdating its heartbeat
	mgr.Add("sid-fresh", "user1", "")
	mgr.Add("sid-stale", "user2", "")

	// Backdate the stale connection's heartbeat to 40 seconds ago
	mgr.mu.Lock()
	if info, ok := mgr.connections["sid-stale"]; ok {
		info.LastHeartbeat = float64(time.Now().Add(-40*time.Second).UnixMilli()) / 1000.0
	}
	mgr.mu.Unlock()

	// Run a single reap pass
	reaper.RunReap()

	// Verify: stale connection reaped, fresh one remains
	mu.Lock()
	defer mu.Unlock()

	if len(disconnected) != 1 || disconnected[0] != "sid-stale" {
		t.Errorf("expected [sid-stale] disconnected, got %v", disconnected)
	}

	if mgr.ActiveCount() != 1 {
		t.Errorf("expected 1 active connection, got %d", mgr.ActiveCount())
	}

	if mgr.Get("sid-fresh") == nil {
		t.Error("fresh connection should still exist")
	}
}

func TestReaper_FallbackOnDisconnectError(t *testing.T) {
	logger := log.New(os.Stderr, "[reaper-test] ", log.LstdFlags)
	mgr := NewConnectionManager(logger)

	// Disconnect always fails — reaper should force-remove
	disconnectFn := func(ns, sid string) error {
		return fmt.Errorf("simulated disconnect error")
	}

	reaper := NewReaper(mgr, disconnectFn, WithReaperLogger(logger))

	mgr.Add("sid-fail", "user1", "")

	// Backdate heartbeat
	mgr.mu.Lock()
	if info, ok := mgr.connections["sid-fail"]; ok {
		info.LastHeartbeat = float64(time.Now().Add(-40*time.Second).UnixMilli()) / 1000.0
	}
	mgr.mu.Unlock()

	reaper.RunReap()

	// Should be force-removed despite disconnect error
	if mgr.ActiveCount() != 0 {
		t.Errorf("expected 0 active connections after fallback removal, got %d", mgr.ActiveCount())
	}
}

func TestReaper_NoDisconnectFunc(t *testing.T) {
	logger := log.New(os.Stderr, "[reaper-test] ", log.LstdFlags)
	mgr := NewConnectionManager(logger)

	// nil disconnect function — should just remove from tracking
	reaper := NewReaper(mgr, nil, WithReaperLogger(logger))

	mgr.Add("sid-nil", "user1", "")

	mgr.mu.Lock()
	if info, ok := mgr.connections["sid-nil"]; ok {
		info.LastHeartbeat = float64(time.Now().Add(-40*time.Second).UnixMilli()) / 1000.0
	}
	mgr.mu.Unlock()

	reaper.RunReap()

	if mgr.ActiveCount() != 0 {
		t.Errorf("expected 0 active connections, got %d", mgr.ActiveCount())
	}
}

func TestReaper_NoStaleConnections(t *testing.T) {
	logger := log.New(os.Stderr, "[reaper-test] ", log.LstdFlags)
	mgr := NewConnectionManager(logger)

	called := false
	disconnectFn := func(ns, sid string) error {
		called = true
		return nil
	}

	reaper := NewReaper(mgr, disconnectFn, WithReaperLogger(logger))

	// All connections are fresh
	mgr.Add("sid1", "", "")
	mgr.Add("sid2", "", "")

	reaper.RunReap()

	if called {
		t.Error("disconnect should not have been called for fresh connections")
	}
	if mgr.ActiveCount() != 2 {
		t.Errorf("expected 2 active connections, got %d", mgr.ActiveCount())
	}
}

func TestReaper_StartStopLifecycle(t *testing.T) {
	logger := log.New(os.Stderr, "[reaper-test] ", log.LstdFlags)
	mgr := NewConnectionManager(logger)

	reaper := NewReaper(mgr, nil,
		WithReaperLogger(logger),
		WithReaperInterval(50*time.Millisecond),
	)

	if reaper.IsRunning() {
		t.Error("reaper should not be running before Start")
	}

	reaper.Start()

	if !reaper.IsRunning() {
		t.Error("reaper should be running after Start")
	}

	// Start again — should be a no-op
	reaper.Start()

	if !reaper.IsRunning() {
		t.Error("reaper should still be running after duplicate Start")
	}

	reaper.Stop()

	if reaper.IsRunning() {
		t.Error("reaper should not be running after Stop")
	}

	// Stop again — should be a no-op
	reaper.Stop()
}

func TestReaper_LoopReapsStaleConnections(t *testing.T) {
	logger := log.New(os.Stderr, "[reaper-test] ", log.LstdFlags)
	mgr := NewConnectionManager(logger)

	var mu sync.Mutex
	reaped := make([]string, 0)
	disconnectFn := func(ns, sid string) error {
		mu.Lock()
		reaped = append(reaped, sid)
		mu.Unlock()
		mgr.Remove(sid)
		return nil
	}

	reaper := NewReaper(mgr, disconnectFn,
		WithReaperLogger(logger),
		WithReaperInterval(50*time.Millisecond),
	)

	// Add a stale connection
	mgr.Add("sid-loop-stale", "user1", "")
	mgr.mu.Lock()
	if info, ok := mgr.connections["sid-loop-stale"]; ok {
		info.LastHeartbeat = float64(time.Now().Add(-40*time.Second).UnixMilli()) / 1000.0
	}
	mgr.mu.Unlock()

	reaper.Start()
	defer reaper.Stop()

	// Wait for the reaper loop to fire at least once
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(reaped) == 0 {
		t.Error("expected stale connection to be reaped by loop")
	}
	found := false
	for _, sid := range reaped {
		if sid == "sid-loop-stale" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected sid-loop-stale in reaped list, got %v", reaped)
	}
}

func TestReaper_DefaultInterval(t *testing.T) {
	mgr := NewConnectionManager(nil)
	reaper := NewReaper(mgr, nil)

	expected := ReaperIntervalSec * time.Second
	if reaper.interval != expected {
		t.Errorf("expected default interval %s, got %s", expected, reaper.interval)
	}
}

func TestReaper_Constants(t *testing.T) {
	// Verify constants match Python server values
	if HeartbeatIntervalSec != 10 {
		t.Errorf("HeartbeatIntervalSec should be 10, got %d", HeartbeatIntervalSec)
	}
	if HeartbeatTimeoutSec != 30 {
		t.Errorf("HeartbeatTimeoutSec should be 30, got %d", HeartbeatTimeoutSec)
	}
	if ReaperIntervalSec != 15 {
		t.Errorf("ReaperIntervalSec should be 15, got %d", ReaperIntervalSec)
	}
}
