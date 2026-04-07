package socketio

import "encoding/json"

// EventHandler is invoked when a client emits an event.
// sid is the socket ID, args are the decoded JSON arguments (excluding event name).
// The returned values (if any) are sent back as an ACK response.
type EventHandler func(sid string, args ...json.RawMessage) ([]interface{}, error)

// ConnectHandler is invoked when a client connects to a namespace.
// auth contains the authentication payload sent by the client (may be nil).
// Return a non-nil error to reject the connection.
type ConnectHandler func(sid string, auth json.RawMessage) error

// DisconnectHandler is invoked when a client disconnects from a namespace.
type DisconnectHandler func(sid string, reason string)
