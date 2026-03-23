package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	RealtimeInstanceIDEnv = "REALTIME_INSTANCE_ID"

	wsEventsStreamName = "HELPIN_WS_EVENTS"
	wsEventsSubjectAll = "helpin.ws.events.*"
)

// ResolveRealtimeInstanceID returns a stable-enough process identifier for
// JetStream durable consumers. In Kubernetes this defaults to the pod hostname.
func ResolveRealtimeInstanceID() string {
	if value := strings.TrimSpace(os.Getenv(RealtimeInstanceIDEnv)); value != "" {
		return value
	}
	if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
		return strings.TrimSpace(hostname)
	}
	return "helpin"
}

// ConnectJetStream opens a shared NATS connection and JetStream context.
func ConnectJetStream(url, clientName string) (*nats.Conn, nats.JetStreamContext, error) {
	nc, err := nats.Connect(url,
		nats.Name(clientName),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(conn *nats.Conn, disconnectErr error) {
			slog.Warn("nats disconnected", "name", clientName, "error", disconnectErr)
		}),
		nats.ReconnectHandler(func(conn *nats.Conn) {
			slog.Info("nats reconnected", "name", clientName, "url", conn.ConnectedUrl())
		}),
		nats.ClosedHandler(func(conn *nats.Conn) {
			slog.Warn("nats connection closed", "name", clientName, "error", conn.LastError())
		}),
	)
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	return nc, js, nil
}

// EnsureJetStreamInfrastructure creates or updates the streams used for worker -> API relay.
func EnsureJetStreamInfrastructure(js nats.JetStreamContext) error {
	if js == nil {
		return fmt.Errorf("jetstream context is nil")
	}
	configs := []*nats.StreamConfig{
		{
			Name:       wsEventsStreamName,
			Subjects:   []string{wsEventsSubjectAll},
			Storage:    nats.FileStorage,
			Retention:  nats.LimitsPolicy,
			Discard:    nats.DiscardOld,
			Duplicates: 2 * time.Minute,
			MaxAge:     time.Hour,
			MaxBytes:   128 * 1024 * 1024,
		},
	}
	for _, cfg := range configs {
		if _, err := js.StreamInfo(cfg.Name); err != nil {
			if !errors.Is(err, nats.ErrStreamNotFound) {
				return err
			}
			if _, err := js.AddStream(cfg); err != nil {
				return err
			}
			continue
		}
		if _, err := js.UpdateStream(cfg); err != nil {
			return err
		}
	}
	return nil
}

// JetStreamBridge consumes JetStream messages and forwards them to the in-process WS hub.
type JetStreamBridge struct {
	js         nats.JetStreamContext
	hub        *Hub
	instanceID string
	eventsSub  *nats.Subscription
}

func NewJetStreamBridge(js nats.JetStreamContext, hub *Hub, instanceID string) *JetStreamBridge {
	return &JetStreamBridge{
		js:         js,
		hub:        hub,
		instanceID: strings.TrimSpace(instanceID),
	}
}

func (b *JetStreamBridge) Start(ctx context.Context) error {
	if err := EnsureJetStreamInfrastructure(b.js); err != nil {
		return err
	}

	instanceID := b.instanceID
	if instanceID == "" {
		instanceID = ResolveRealtimeInstanceID()
	}

	eventsSub, err := b.js.Subscribe(
		wsEventsSubjectAll,
		b.handleWorkspaceEvent,
		nats.BindStream(wsEventsStreamName),
		nats.Durable("api-ws-events-"+instanceID),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(2*time.Minute),
	)
	if err != nil {
		return err
	}

	b.eventsSub = eventsSub
	slog.Info("jetstream bridge started", "instance_id", instanceID)

	<-ctx.Done()

	if b.eventsSub != nil {
		_ = b.eventsSub.Drain()
	}
	return nil
}

func (b *JetStreamBridge) handleWorkspaceEvent(msg *nats.Msg) {
	var event Event
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.Warn("jetstream bridge: invalid workspace event payload", "subject", msg.Subject, "error", err)
		_ = msg.Ack()
		return
	}
	b.hub.Broadcast(event)
	_ = msg.Ack()
}

func workspaceEventSubject(workspaceID string) string {
	return "helpin.ws.events." + workspaceID
}
