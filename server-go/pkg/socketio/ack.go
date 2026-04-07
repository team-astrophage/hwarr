package socketio

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// AckCallback is invoked when an ACK response is received from the client.
// args contains the ACK response data; err is non-nil on timeout.
type AckCallback func(args []json.RawMessage, err error)

// ErrAckTimeout is returned to AckCallback when the ACK response does not
// arrive within the specified timeout.
var ErrAckTimeout = fmt.Errorf("socketio: ack timeout")

// pendingAck represents a single outstanding ACK request.
type pendingAck struct {
	callback AckCallback
	timer    *time.Timer
}

// AckRegistry tracks pending server→client ACK callbacks.
// When the server emits an event with an ACK request, it registers
// a callback here. When the ACK response packet arrives from the client,
// the registry dispatches it to the matching callback.
//
// Thread-safe: all methods can be called concurrently.
type AckRegistry struct {
	mu      sync.Mutex
	pending map[string]map[int]*pendingAck // sid -> ackID -> pendingAck
	nextID  atomic.Int64
}

// NewAckRegistry creates a new AckRegistry.
func NewAckRegistry() *AckRegistry {
	return &AckRegistry{
		pending: make(map[string]map[int]*pendingAck),
	}
}

// NextID returns a monotonically increasing ACK ID.
// This is used to generate unique ack IDs for server→client ACK requests.
func (r *AckRegistry) NextID() int {
	return int(r.nextID.Add(1))
}

// Register stores a callback for a specific socket and ACK ID.
// If timeout > 0, the callback will be invoked with ErrAckTimeout
// after the specified duration if no ACK response is received.
// Returns a cancel function that can be called to remove the pending ack.
func (r *AckRegistry) Register(sid string, ackID int, timeout time.Duration, cb AckCallback) func() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pending[sid] == nil {
		r.pending[sid] = make(map[int]*pendingAck)
	}

	pa := &pendingAck{callback: cb}

	if timeout > 0 {
		pa.timer = time.AfterFunc(timeout, func() {
			r.mu.Lock()
			// Check if still pending (may have been resolved already)
			sidMap, ok := r.pending[sid]
			if ok {
				if existing, found := sidMap[ackID]; found && existing == pa {
					delete(sidMap, ackID)
					if len(sidMap) == 0 {
						delete(r.pending, sid)
					}
				}
			}
			r.mu.Unlock()
			cb(nil, ErrAckTimeout)
		})
	}

	r.pending[sid][ackID] = pa

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		sidMap, ok := r.pending[sid]
		if !ok {
			return
		}
		if existing, found := sidMap[ackID]; found && existing == pa {
			if existing.timer != nil {
				existing.timer.Stop()
			}
			delete(sidMap, ackID)
			if len(sidMap) == 0 {
				delete(r.pending, sid)
			}
		}
	}
}

// Resolve dispatches an ACK response packet to the registered callback.
// Returns true if a matching pending ack was found and dispatched.
func (r *AckRegistry) Resolve(sid string, ackID int, data json.RawMessage) bool {
	r.mu.Lock()
	sidMap, ok := r.pending[sid]
	if !ok {
		r.mu.Unlock()
		return false
	}
	pa, found := sidMap[ackID]
	if !found {
		r.mu.Unlock()
		return false
	}
	// Remove from pending before invoking callback
	delete(sidMap, ackID)
	if len(sidMap) == 0 {
		delete(r.pending, sid)
	}
	r.mu.Unlock()

	// Stop the timeout timer if set
	if pa.timer != nil {
		pa.timer.Stop()
	}

	// Parse the ACK data array
	var args []json.RawMessage
	if data != nil {
		if err := json.Unmarshal(data, &args); err != nil {
			// If not an array, wrap as single element
			args = []json.RawMessage{data}
		}
	}

	pa.callback(args, nil)
	return true
}

// RemoveAll removes all pending ACKs for a socket (e.g., on disconnect).
// Pending callbacks are NOT invoked — they are silently dropped.
func (r *AckRegistry) RemoveAll(sid string) {
	r.mu.Lock()
	sidMap, ok := r.pending[sid]
	if ok {
		for _, pa := range sidMap {
			if pa.timer != nil {
				pa.timer.Stop()
			}
		}
		delete(r.pending, sid)
	}
	r.mu.Unlock()
}

// PendingCount returns the number of pending ACKs for a socket.
func (r *AckRegistry) PendingCount(sid string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending[sid])
}
