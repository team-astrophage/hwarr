package sio

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTokenExpiryMiddleware_AllowsValidToken(t *testing.T) {
	mgr := NewConnectionManager(nil)
	info := mgr.Add("sid1", "user1")
	info.TokenExpiry = time.Now().Add(30 * time.Minute)

	called := false
	mw := NewTokenExpiryMiddleware(mgr, nil, nil)
	result, err := mw("sid1", "fire:ignite", nil, func() ([]interface{}, error) {
		called = true
		return []interface{}{"ok"}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected next() to be called")
	}
	if result[0] != "ok" {
		t.Fatalf("expected result 'ok', got %v", result[0])
	}
}

func TestTokenExpiryMiddleware_RejectsExpiredToken(t *testing.T) {
	mgr := NewConnectionManager(nil)
	info := mgr.Add("sid1", "user1")
	info.TokenExpiry = time.Now().Add(-1 * time.Second)

	var disconnectedSID string
	disconnectFn := func(sid string) { disconnectedSID = sid }

	called := false
	mw := NewTokenExpiryMiddleware(mgr, disconnectFn, nil)
	_, err := mw("sid1", "fire:ignite", nil, func() ([]interface{}, error) {
		called = true
		return []interface{}{"ok"}, nil
	})

	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if called {
		t.Fatal("next() should not be called for expired token")
	}
	if disconnectedSID != "sid1" {
		t.Fatalf("expected disconnect for sid1, got %q", disconnectedSID)
	}
}

func TestTokenExpiryMiddleware_ZeroExpiryPassesThrough(t *testing.T) {
	mgr := NewConnectionManager(nil)
	mgr.Add("sid1", "user1") // TokenExpiry is zero

	called := false
	mw := NewTokenExpiryMiddleware(mgr, nil, nil)
	_, err := mw("sid1", "fire:ignite", nil, func() ([]interface{}, error) {
		called = true
		return []interface{}{"ok"}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected next() to be called for zero expiry")
	}
}

func TestTokenExpiryMiddleware_UnknownSIDPassesThrough(t *testing.T) {
	mgr := NewConnectionManager(nil)

	called := false
	mw := NewTokenExpiryMiddleware(mgr, nil, nil)
	_, err := mw("unknown-sid", "fire:ignite", nil, func() ([]interface{}, error) {
		called = true
		return []interface{}{"ok"}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected next() to be called for unknown SID")
	}
}

// Verify unused import suppression for json.RawMessage in middleware signature.
var _ json.RawMessage
