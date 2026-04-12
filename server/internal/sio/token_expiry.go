package sio

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	socketio "github.com/homeworldio/socketio-go"
)

// NewTokenExpiryMiddleware returns a Socket.IO middleware that rejects events
// from connections whose token has expired, and disconnects them.
func NewTokenExpiryMiddleware(manager *ConnectionManager, disconnectFn func(string), logger *log.Logger) socketio.MiddlewareFunc {
	if logger == nil {
		logger = log.Default()
	}
	return func(sid string, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		info := manager.Get(sid)
		if info == nil {
			return next()
		}
		if !info.TokenExpiry.IsZero() && time.Now().After(info.TokenExpiry) {
			logger.Printf("Token expired: sid=%s event=%s", sid, event)
			if disconnectFn != nil {
				disconnectFn(sid)
			}
			return nil, fmt.Errorf("token expired")
		}
		return next()
	}
}
