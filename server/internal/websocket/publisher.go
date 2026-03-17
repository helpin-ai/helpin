package websocket

import (
	"context"
	"log/slog"
)

// Publisher is a thin wrapper for services to broadcast events.
// It is nil-safe: calling Publish on a nil publisher is a no-op.
// When a RedisRelay is configured, events are published to both the local
// Hub (same pod) and Redis Pub/Sub (other pods).
type Publisher struct {
	hub   *Hub
	relay *RedisRelay // nil when running in local-only mode
}

// NewPublisher creates a new publisher. relay may be nil for local-only mode.
func NewPublisher(hub *Hub, relay *RedisRelay) *Publisher {
	return &Publisher{hub: hub, relay: relay}
}

// Publish broadcasts an event asynchronously. Safe to call on a nil receiver.
// Events without a WorkspaceID are logged as errors since they will not reach
// any clients (Hub.Broadcast routes by WorkspaceID).
func (p *Publisher) Publish(event Event) {
	if p == nil {
		return
	}

	if event.WorkspaceID == "" {
		slog.Error("event published without WorkspaceID — will not reach clients",
			"entity", event.Entity, "action", event.Action, "entity_id", event.EntityID)
	}

	go p.hub.Broadcast(event)

	if p.relay != nil {
		go p.relay.Publish(context.Background(), event)
	}
}
