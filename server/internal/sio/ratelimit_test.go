package sio

import (
	"testing"
	"time"
)

func TestAllow_WithinLimit(t *testing.T) {
	rl := NewRateLimiter(nil)

	// fire:ignite has burst=17, so the first 17 calls should succeed.
	for i := 0; i < 17; i++ {
		if !rl.Allow("s1", "fire:ignite") {
			t.Fatalf("expected Allow to return true on call %d", i+1)
		}
	}
}

func TestAllow_ExceedsLimit(t *testing.T) {
	rl := NewRateLimiter(nil)

	// Exhaust the burst for fire:ignite (burst=17).
	for i := 0; i < 17; i++ {
		rl.Allow("s1", "fire:ignite")
	}

	// The next call should be rejected (no time has passed for token refill).
	if rl.Allow("s1", "fire:ignite") {
		t.Fatal("expected Allow to return false after burst exhausted")
	}
}

func TestAllow_UnregisteredEvent_DefaultLimit(t *testing.T) {
	rl := NewRateLimiter(nil)

	// Default burst is 5, so the first 5 calls should succeed.
	for i := 0; i < 5; i++ {
		if !rl.Allow("s1", "some:unknown:event") {
			t.Fatalf("expected Allow for unregistered event on call %d", i+1)
		}
	}

	// 6th call should be rejected (burst exhausted, no time for refill).
	if rl.Allow("s1", "some:unknown:event") {
		t.Fatal("expected rejection for unregistered event after burst exhausted")
	}
}

func TestAllow_DisconnectAfterMaxViolations(t *testing.T) {
	disconnected := false
	var disconnectedSID string

	rl := NewRateLimiter(func(sid string) {
		disconnected = true
		disconnectedSID = sid
	})

	// Exhaust burst for fire:ignite.
	for i := 0; i < 17; i++ {
		rl.Allow("s1", "fire:ignite")
	}

	// Generate maxViolations+1 rejections to trigger disconnect.
	for i := 0; i < maxViolations+1; i++ {
		rl.Allow("s1", "fire:ignite")
	}

	if !disconnected {
		t.Fatal("expected disconnect callback to be invoked")
	}
	if disconnectedSID != "s1" {
		t.Fatalf("expected disconnected sid to be s1, got %s", disconnectedSID)
	}
}

func TestRemove_CleansUpState(t *testing.T) {
	rl := NewRateLimiter(nil)

	// Create some state.
	rl.Allow("s1", "fire:ignite")
	rl.Allow("s1", "chat:send")

	rl.Remove("s1")

	rl.mu.RLock()
	_, hasBucket := rl.buckets["s1"]
	_, hasViolation := rl.violations["s1"]
	rl.mu.RUnlock()

	if hasBucket {
		t.Fatal("expected buckets to be cleaned up after Remove")
	}
	if hasViolation {
		t.Fatal("expected violations to be cleaned up after Remove")
	}
}

func TestAllow_TokenRefillAfterWait(t *testing.T) {
	rl := NewRateLimiter(nil)

	// Exhaust burst for subscribe:viewport (burst=5, rate=5/s → 200ms per token).
	for i := 0; i < 5; i++ {
		rl.Allow("s1", "subscribe:viewport")
	}

	// Should be rejected immediately.
	if rl.Allow("s1", "subscribe:viewport") {
		t.Fatal("expected rejection after burst exhausted")
	}

	// Wait for one token to refill (200ms + buffer).
	time.Sleep(250 * time.Millisecond)

	if !rl.Allow("s1", "subscribe:viewport") {
		t.Fatal("expected Allow to succeed after token refill")
	}
}

func TestAllow_ViolationDecay(t *testing.T) {
	disconnected := false
	rl := NewRateLimiter(func(sid string) {
		disconnected = true
	})

	// Exhaust burst for chat:send (burst=2).
	for i := 0; i < 2; i++ {
		rl.Allow("s1", "chat:send")
	}

	// Generate 9 violations (just under maxViolations=10).
	for i := 0; i < 9; i++ {
		rl.Allow("s1", "chat:send")
	}

	rl.mu.RLock()
	v := rl.violations["s1"]
	rl.mu.RUnlock()
	if v != 9 {
		t.Fatalf("expected 9 violations, got %d", v)
	}

	// Simulate 30+ seconds elapsed by backdating lastViolation.
	rl.mu.Lock()
	rl.lastViolation["s1"] = time.Now().Add(-31 * time.Second)
	rl.mu.Unlock()

	// Next violation should trigger decay: reset to 0, then increment to 1.
	rl.Allow("s1", "chat:send")

	rl.mu.RLock()
	v = rl.violations["s1"]
	rl.mu.RUnlock()
	if v != 1 {
		t.Fatalf("expected violations to decay to 1 after 30s, got %d", v)
	}

	if disconnected {
		t.Fatal("should not have disconnected after decay")
	}
}

func TestAllow_Heartbeat(t *testing.T) {
	rl := NewRateLimiter(nil)

	// heartbeat has burst=2, so first 2 should succeed.
	if !rl.Allow("s1", "heartbeat") {
		t.Fatal("expected first heartbeat to be allowed")
	}
	if !rl.Allow("s1", "heartbeat") {
		t.Fatal("expected second heartbeat to be allowed")
	}

	// 3rd should be rejected (burst exhausted).
	if rl.Allow("s1", "heartbeat") {
		t.Fatal("expected third heartbeat to be rejected")
	}
}

func TestRemove_AfterDisconnect_NoState(t *testing.T) {
	rl := NewRateLimiter(func(sid string) {})

	// Exhaust and trigger disconnect.
	for i := 0; i < 17; i++ {
		rl.Allow("s1", "fire:ignite")
	}
	for i := 0; i < maxViolations+1; i++ {
		rl.Allow("s1", "fire:ignite")
	}

	// Remove should be safe to call even after auto-cleanup.
	rl.Remove("s1")

	rl.mu.RLock()
	_, hasBucket := rl.buckets["s1"]
	rl.mu.RUnlock()

	if hasBucket {
		t.Fatal("expected no state after Remove post-disconnect")
	}
}
