package socketio

import (
	"encoding/json"
	"sync"
)

// MiddlewareFunc is a function that intercepts Socket.IO events before they
// reach the final event handler. It follows the classic middleware pattern:
//
//   - sid: the socket ID of the client
//   - event: the event name being emitted
//   - args: the decoded JSON arguments
//   - next: call this to pass control to the next middleware (or the final handler)
//
// A middleware can:
//   - Inspect/modify event args before calling next
//   - Short-circuit the chain by returning without calling next
//   - Return an error to reject the event
//   - Wrap next to perform post-processing (e.g., logging)
type MiddlewareFunc func(sid string, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error)

// MiddlewareChain manages an ordered list of middleware functions and executes
// them as a chain around a final handler. Thread-safe for concurrent Add calls.
type MiddlewareChain struct {
	mu          sync.RWMutex
	middlewares []MiddlewareFunc
}

// NewMiddlewareChain creates an empty middleware chain.
func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{}
}

// Add appends a middleware to the end of the chain. Middlewares execute in the
// order they are added — first added runs first (outermost).
func (c *MiddlewareChain) Add(mw MiddlewareFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = append(c.middlewares, mw)
}

// Len returns the number of middlewares in the chain.
func (c *MiddlewareChain) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.middlewares)
}

// Execute runs the middleware chain and then calls the final handler.
// Each middleware wraps the next one, forming a nested call stack:
//
//	mw1 → mw2 → mw3 → handler
//
// If the chain is empty, the handler is called directly.
// If handler is nil, a no-op handler returning (nil, nil) is used.
func (c *MiddlewareChain) Execute(sid string, event string, args []json.RawMessage, handler func() ([]interface{}, error)) ([]interface{}, error) {
	c.mu.RLock()
	mws := make([]MiddlewareFunc, len(c.middlewares))
	copy(mws, c.middlewares)
	c.mu.RUnlock()

	if handler == nil {
		handler = func() ([]interface{}, error) { return nil, nil }
	}

	// Build the chain from inside out: start with the handler, then wrap
	// each middleware around it in reverse order.
	next := handler
	for i := len(mws) - 1; i >= 0; i-- {
		mw := mws[i]
		captured := next // capture for closure
		next = func() ([]interface{}, error) {
			return mw(sid, event, args, captured)
		}
	}

	return next()
}
