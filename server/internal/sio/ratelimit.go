package sio

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	socketio "github.com/homeworldio/socketio-go"
	"golang.org/x/time/rate"
)

// maxViolations is the number of rate-limit violations before a client is
// forcefully disconnected.
const maxViolations = 10

// RateLimiter enforces per-event rate limits on a per-socket basis.
//
// Each (sid, event) pair gets its own token-bucket limiter. Events that are
// not registered in the limits table pass through without restriction.
// After maxViolations consecutive rejections for a given sid the disconnect
// callback is invoked to drop the abusive client.
type RateLimiter struct {
	mu           sync.RWMutex
	buckets      map[string]map[string]*rate.Limiter // sid → event → limiter
	violations   map[string]int                      // sid → violation count
	limits       map[string]rate.Limit               // event → rate
	bursts       map[string]int                      // event → burst
	disconnectFn func(sid string)
}

// NewRateLimiter creates a RateLimiter with sensible defaults for known
// events. disconnectFn is called when a client exceeds maxViolations.
func NewRateLimiter(disconnectFn func(string)) *RateLimiter {
	return &RateLimiter{
		buckets:    make(map[string]map[string]*rate.Limiter),
		violations: make(map[string]int),
		limits: map[string]rate.Limit{
			"fire:ignite":        rate.Limit(17), // 17/s
			"chat:send":          rate.Every(1 * time.Second),        // 1/s
			"subscribe:viewport": rate.Every(200 * time.Millisecond), // 5/s
		},
		bursts: map[string]int{
			"fire:ignite":        17,
			"chat:send":          2,
			"subscribe:viewport": 5,
		},
		disconnectFn: disconnectFn,
	}
}

// Allow checks whether the event from sid is permitted under the configured
// rate limit. Unregistered events always return true.
//
// When a request is denied the violation counter for sid is incremented. If
// the counter exceeds maxViolations the disconnect callback fires and the
// sid's state is cleaned up.
func (rl *RateLimiter) Allow(sid, event string) bool {
	limit, ok := rl.limits[event]
	if !ok {
		// No limit configured for this event — allow unconditionally.
		return true
	}

	rl.mu.Lock()

	// Lazily create per-sid bucket map.
	eventMap, exists := rl.buckets[sid]
	if !exists {
		eventMap = make(map[string]*rate.Limiter)
		rl.buckets[sid] = eventMap
	}

	// Lazily create per-event limiter.
	limiter, exists := eventMap[event]
	if !exists {
		burst := rl.bursts[event]
		limiter = rate.NewLimiter(limit, burst)
		eventMap[event] = limiter
	}

	if limiter.Allow() {
		rl.mu.Unlock()
		return true
	}

	// Violation path.
	rl.violations[sid]++
	shouldDisconnect := rl.violations[sid] > maxViolations
	if shouldDisconnect {
		delete(rl.buckets, sid)
		delete(rl.violations, sid)
	}
	rl.mu.Unlock()

	// Disconnect callback runs outside the lock to avoid deadlock:
	// disconnectFn → DisconnectAll → onCleanup → rl.Remove() would
	// re-acquire rl.mu.
	if shouldDisconnect && rl.disconnectFn != nil {
		rl.disconnectFn(sid)
	}
	return false
}

// Remove cleans up all state for the given sid. Must be called on disconnect
// to prevent memory leaks.
func (rl *RateLimiter) Remove(sid string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.buckets, sid)
	delete(rl.violations, sid)
}

// NewRateLimitMiddleware returns a Socket.IO middleware that enforces the
// rate limits defined in rl. Rejected events return an error that is
// delivered to the client as an ACK error message.
func NewRateLimitMiddleware(rl *RateLimiter, logger *log.Logger) socketio.MiddlewareFunc {
	if logger == nil {
		logger = log.Default()
	}
	return func(sid string, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		if !rl.Allow(sid, event) {
			logger.Printf("Rate limit exceeded: sid=%s event=%s", sid, event)
			return nil, fmt.Errorf("rate limit exceeded for event %s", event)
		}
		return next()
	}
}
