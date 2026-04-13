package sio

import (
	"context"
	"encoding/json"
	"log"

	goredis "github.com/redis/go-redis/v9"
)

// SocketBroadcaster abstracts Socket.IO broadcast operations.
// The method signatures match *socketio.Server so it can be used as a drop-in replacement.
type SocketBroadcaster interface {
	BroadcastToNamespace(namespace, event string, args ...interface{}) (int, error)
	BroadcastToRoom(namespace, room, event string, args ...interface{}) (int, error)
}

// broadcastEnvelope is the message format published to Redis Pub/Sub.
type broadcastEnvelope struct {
	Origin    string          `json:"origin"`
	Kind      string          `json:"kind"`
	Namespace string          `json:"namespace"`
	Room      string          `json:"room,omitempty"`
	Event     string          `json:"event"`
	Payload   json.RawMessage `json:"payload"`
}

// RedisBroadcastAdapter wraps a local SocketBroadcaster and replicates
// broadcasts to other server instances via Redis Pub/Sub.
type RedisBroadcastAdapter struct {
	local      SocketBroadcaster
	pub        *goredis.Client
	sub        *goredis.Client
	channel    string
	instanceID string
	logger     *log.Logger

	pubsub *goredis.PubSub
	doneCh chan struct{}
}

// NewRedisBroadcastAdapter creates a new adapter.
// pub and sub should be separate go-redis client instances to avoid blocking.
func NewRedisBroadcastAdapter(
	local SocketBroadcaster,
	pub, sub *goredis.Client,
	channel, instanceID string,
	logger *log.Logger,
) *RedisBroadcastAdapter {
	if logger == nil {
		logger = log.Default()
	}
	return &RedisBroadcastAdapter{
		local:      local,
		pub:        pub,
		sub:        sub,
		channel:    channel,
		instanceID: instanceID,
		logger:     logger,
		doneCh:     make(chan struct{}),
	}
}

// BroadcastToNamespace broadcasts locally then publishes to Redis for cross-instance delivery.
func (r *RedisBroadcastAdapter) BroadcastToNamespace(namespace, event string, args ...interface{}) (int, error) {
	n, err := r.local.BroadcastToNamespace(namespace, event, args...)
	if err != nil {
		return n, err
	}
	r.publish(broadcastEnvelope{
		Origin:    r.instanceID,
		Kind:      "ns",
		Namespace: namespace,
		Event:     event,
	}, args)
	return n, nil
}

// BroadcastToRoom broadcasts locally then publishes to Redis for cross-instance delivery.
func (r *RedisBroadcastAdapter) BroadcastToRoom(namespace, room, event string, args ...interface{}) (int, error) {
	n, err := r.local.BroadcastToRoom(namespace, room, event, args...)
	if err != nil {
		return n, err
	}
	r.publish(broadcastEnvelope{
		Origin:    r.instanceID,
		Kind:      "room",
		Namespace: namespace,
		Room:      room,
		Event:     event,
	}, args)
	return n, nil
}

// publish serializes the envelope and publishes to Redis.
func (r *RedisBroadcastAdapter) publish(env broadcastEnvelope, args []interface{}) {
	payload, err := json.Marshal(args)
	if err != nil {
		r.logger.Printf("redis-pubsub: failed to marshal args: %v", err)
		return
	}
	env.Payload = payload

	data, err := json.Marshal(env)
	if err != nil {
		r.logger.Printf("redis-pubsub: failed to marshal envelope: %v", err)
		return
	}

	if err := r.pub.Publish(context.Background(), r.channel, data).Err(); err != nil {
		r.logger.Printf("redis-pubsub: publish failed: %v", err)
	}
}

// Start begins the Redis subscribe goroutine. Call Stop() to shut it down.
func (r *RedisBroadcastAdapter) Start(ctx context.Context) {
	r.pubsub = r.sub.Subscribe(ctx, r.channel)

	go func() {
		defer close(r.doneCh)

		ch := r.pubsub.Channel()
		for msg := range ch {
			r.handleMessage(msg.Payload)
		}
	}()

	r.logger.Printf("redis-pubsub: subscribed to channel %q (instance=%s)", r.channel, r.instanceID)
}

// Stop unsubscribes and waits for the goroutine to exit.
func (r *RedisBroadcastAdapter) Stop() {
	if r.pubsub != nil {
		_ = r.pubsub.Close()
	}
	<-r.doneCh
}

// handleMessage decodes an envelope and re-broadcasts locally if from another instance.
func (r *RedisBroadcastAdapter) handleMessage(raw string) {
	var env broadcastEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		r.logger.Printf("redis-pubsub: failed to unmarshal envelope: %v", err)
		return
	}

	if env.Origin == r.instanceID {
		return
	}

	var args []interface{}
	if err := json.Unmarshal(env.Payload, &args); err != nil {
		r.logger.Printf("redis-pubsub: failed to unmarshal payload: %v", err)
		return
	}

	switch env.Kind {
	case "ns":
		if _, err := r.local.BroadcastToNamespace(env.Namespace, env.Event, args...); err != nil {
			r.logger.Printf("redis-pubsub: local namespace broadcast failed: %v", err)
		}
	case "room":
		if _, err := r.local.BroadcastToRoom(env.Namespace, env.Room, env.Event, args...); err != nil {
			r.logger.Printf("redis-pubsub: local room broadcast failed: %v", err)
		}
	default:
		r.logger.Printf("redis-pubsub: unknown envelope kind: %q", env.Kind)
	}
}
