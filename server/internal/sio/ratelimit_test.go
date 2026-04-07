package sio

import (
	"testing"
	"time"
)

func TestAllow_WithinLimit(t *testing.T) {
	rl := NewRateLimiter(nil)

	// fire:ignite has burst=3, so the first 3 calls should succeed.
	for i := 0; i < 3; i++ {
		if !rl.Allow("s1", "fire:ignite") {
			t.Fatalf("expected Allow to return true on call %d", i+1)
		}
	}
}

func TestAllow_ExceedsLimit(t *testing.T) {
	rl := NewRateLimiter(nil)

	// Exhaust the burst for fire:ignite (burst=3).
	for i := 0; i < 3; i++ {
		rl.Allow("s1", "fire:ignite")
	}

	// The next call should be rejected (no time has passed for token refill).
	if rl.Allow("s1", "fire:ignite") {
		t.Fatal("expected Allow to return false after burst exhausted")
	}
}

func TestAllow_UnregisteredEvent_AlwaysAllowed(t *testing.T) {
	rl := NewRateLimiter(nil)

	for i := 0; i < 100; i++ {
		if !rl.Allow("s1", "some:unknown:event") {
			t.Fatalf("unregistered events should always be allowed, failed on call %d", i+1)
		}
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
	for i := 0; i < 3; i++ {
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

func TestRemove_AfterDisconnect_NoState(t *testing.T) {
	rl := NewRateLimiter(func(sid string) {})

	// Exhaust and trigger disconnect.
	for i := 0; i < 3; i++ {
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
