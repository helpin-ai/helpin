package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Publisher is a thin wrapper for services to broadcast events.
// It is nil-safe: calling Publish on a nil publisher is a no-op.
// When a RedisRelay is configured, events are published to both the local
// Hub (same pod) and Redis Pub/Sub (other pods).
type Publisher struct {
	hub   *Hub
	relay *RedisRelay // nil when running in local-only mode
	js    nats.JetStreamContext
}

// NewPublisher creates a new publisher. relay may be nil for local-only mode.
func NewPublisher(hub *Hub, relay *RedisRelay) *Publisher {
	return &Publisher{hub: hub, relay: relay}
}

// NewJetStreamPublisher creates a new JetStream-backed publisher for cross-process events.
func NewJetStreamPublisher(js nats.JetStreamContext) *Publisher {
	return &Publisher{js: js}
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

	if p.hub != nil {
		go p.hub.Broadcast(event)
	}

	if p.relay != nil {
		go p.relay.Publish(context.Background(), event)
	}

	if p.js != nil {
		go p.publishJetStream(event)
	}
}

func (p *Publisher) publishJetStream(event Event) {
	if strings.TrimSpace(event.WorkspaceID) == "" {
		slog.Warn("jetstream publisher: skipping event with empty workspace_id", "entity", event.Entity, "entity_id", event.EntityID)
		return
	}
	if event.EventID == "" {
		event.EventID = uuid.NewString()
	}
	if event.SentAt.IsZero() {
		event.SentAt = time.Now().UTC()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("jetstream publisher: marshal failed", "entity", event.Entity, "error", err)
		return
	}
	if _, err := p.js.Publish(workspaceEventSubject(event.WorkspaceID), payload, nats.MsgId(event.EventID)); err != nil {
		slog.Error("jetstream publisher: publish failed", "entity", event.Entity, "workspace_id", event.WorkspaceID, "error", err)
	}
}
