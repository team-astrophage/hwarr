package socketio

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Namespace represents a Socket.IO namespace that groups event handlers
// and manages sockets connected to a specific path (e.g., "/", "/chat").
//
// Each namespace has its own RoomManager, providing complete room isolation
// between namespaces — matching python-socketio behavior where rooms are
// always scoped to a namespace.
//
// Middleware can be registered via Use() and will execute in order before
// the final event handler on every HandleEvent call.
//
// Thread-safe: all methods can be called concurrently.
type Namespace struct {
	path string

	mu                sync.RWMutex
	eventHandlers     map[string]EventHandler
	connectHandler    ConnectHandler
	disconnectHandler DisconnectHandler
	sockets           map[string]bool // set of connected socket IDs

	// middleware is the chain of middleware functions executed before event handlers.
	middleware *MiddlewareChain

	// Rooms manages room memberships scoped to this namespace.
	// Each namespace has its own independent room space.
	Rooms *RoomManager
}

// NewNamespace creates a new namespace for the given path.
func NewNamespace(path string) *Namespace {
	if path == "" {
		path = "/"
	}
	return &Namespace{
		path:          path,
		eventHandlers: make(map[string]EventHandler),
		sockets:       make(map[string]bool),
		middleware:    NewMiddlewareChain(),
		Rooms:         NewRoomManager(),
	}
}

// Path returns the namespace path.
func (ns *Namespace) Path() string {
	return ns.path
}

// Use appends a middleware to the namespace's middleware chain. Middlewares
// execute in the order they are added (first added = outermost) and run
// before the final event handler on every HandleEvent call.
//
// Example:
//
//	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
//	    log.Printf("event %s from %s", event, sid)
//	    return next()
//	})
func (ns *Namespace) Use(mw MiddlewareFunc) {
	ns.middleware.Add(mw)
}

// On registers an event handler for the given event name.
// Overwrites any previously registered handler for the same event.
func (ns *Namespace) On(event string, handler EventHandler) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.eventHandlers[event] = handler
}

// OnConnect registers a handler invoked when a client connects to this namespace.
func (ns *Namespace) OnConnect(handler ConnectHandler) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.connectHandler = handler
}

// OnDisconnect registers a handler invoked when a client disconnects from this namespace.
func (ns *Namespace) OnDisconnect(handler DisconnectHandler) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.disconnectHandler = handler
}

// HandleConnect dispatches a CONNECT packet to the registered connect handler.
// On success (handler returns nil or no handler registered), the socket is
// added to the namespace's connected set. Returns nil if no handler is
// registered (implicit accept).
func (ns *Namespace) HandleConnect(sid string, auth json.RawMessage) error {
	ns.mu.RLock()
	h := ns.connectHandler
	ns.mu.RUnlock()
	if h != nil {
		if err := h(sid, auth); err != nil {
			return err
		}
	}
	ns.mu.Lock()
	ns.sockets[sid] = true
	ns.mu.Unlock()
	return nil
}

// HandleDisconnect removes the socket from the connected set, cleans up all
// room memberships in this namespace, and dispatches a DISCONNECT event to
// the registered handler.
func (ns *Namespace) HandleDisconnect(sid string, reason string) {
	ns.mu.Lock()
	delete(ns.sockets, sid)
	h := ns.disconnectHandler
	ns.mu.Unlock()

	// Clean up all room memberships for this socket in this namespace
	ns.Rooms.LeaveAll(sid)

	if h != nil {
		h(sid, reason)
	}
}

// HasSocket reports whether the given socket ID is connected to this namespace.
func (ns *Namespace) HasSocket(sid string) bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.sockets[sid]
}

// SocketCount returns the number of sockets connected to this namespace.
func (ns *Namespace) SocketCount() int {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return len(ns.sockets)
}

// Sockets returns a copy of all connected socket IDs.
func (ns *Namespace) Sockets() []string {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	sids := make([]string, 0, len(ns.sockets))
	for sid := range ns.sockets {
		sids = append(sids, sid)
	}
	return sids
}

// HandleEvent dispatches an EVENT packet through the middleware chain and then
// to the registered event handler. Middlewares run in registration order before
// the final handler. If no handler is found, returns an error.
func (ns *Namespace) HandleEvent(sid string, event string, args []json.RawMessage) ([]interface{}, error) {
	ns.mu.RLock()
	h, ok := ns.eventHandlers[event]
	ns.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("socketio: no handler for event %q in namespace %q", event, ns.path)
	}

	handler := func() ([]interface{}, error) {
		return h(sid, args...)
	}

	return ns.middleware.Execute(sid, event, args, handler)
}

// HasEvent reports whether a handler is registered for the given event.
func (ns *Namespace) HasEvent(event string) bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	_, ok := ns.eventHandlers[event]
	return ok
}

// Events returns a list of all registered event names.
func (ns *Namespace) Events() []string {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	events := make([]string, 0, len(ns.eventHandlers))
	for e := range ns.eventHandlers {
		events = append(events, e)
	}
	return events
}
