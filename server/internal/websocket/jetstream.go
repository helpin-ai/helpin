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

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	RealtimeInstanceIDEnv = "REALTIME_INSTANCE_ID"

	planningStreamName   = "HELPIN_PLANNING"
	planningSubjectAll   = "helpin.planning.stream.*"
	wsEventsStreamName   = "HELPIN_WS_EVENTS"
	wsEventsSubjectAll   = "helpin.ws.events.*"
	SupportAIStreamName  = "SUPPORT_AI"
	supportAIRequestAll  = "support.ai.request.*"
	supportAITypingAll   = "support.ai.typing.*"
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
			Name:       planningStreamName,
			Subjects:   []string{planningSubjectAll},
			Storage:    nats.FileStorage,
			Retention:  nats.LimitsPolicy,
			Discard:    nats.DiscardOld,
			Duplicates: 2 * time.Minute,
			MaxAge:     15 * time.Minute,
			MaxBytes:   256 * 1024 * 1024,
		},
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
		{
			Name:       SupportAIStreamName,
			Subjects:   []string{supportAIRequestAll, supportAITypingAll},
			Storage:    nats.FileStorage,
			Retention:  nats.LimitsPolicy,
			Discard:    nats.DiscardOld,
			Duplicates: 2 * time.Minute,
			MaxAge:     24 * time.Hour,
			MaxBytes:   256 * 1024 * 1024,
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

// JetStreamSessionStreamer relays planning stream events through JetStream.
type JetStreamSessionStreamer struct {
	js nats.JetStreamContext
}

func NewJetStreamSessionStreamer(js nats.JetStreamContext) *JetStreamSessionStreamer {
	return &JetStreamSessionStreamer{js: js}
}

func (s *JetStreamSessionStreamer) SendToSession(sessionID string, event interface{}) {
	if s == nil || s.js == nil || strings.TrimSpace(sessionID) == "" {
		return
	}

	payload, msgID, err := marshalPlanningStreamEvent(sessionID, event)
	if err != nil {
		slog.Error("jetstream session streamer: marshal failed", "session_id", sessionID, "error", err)
		return
	}

	if _, err := s.js.Publish(planningSubject(sessionID), payload, nats.MsgId(msgID)); err != nil {
		slog.Error("jetstream session streamer: publish failed", "session_id", sessionID, "error", err)
	}
}

func marshalPlanningStreamEvent(sessionID string, event interface{}) ([]byte, string, error) {
	switch v := event.(type) {
	case model.PlanningStreamEvent:
		return marshalTypedPlanningEvent(sessionID, v)
	case *model.PlanningStreamEvent:
		if v == nil {
			return nil, "", fmt.Errorf("planning stream event is nil")
		}
		return marshalTypedPlanningEvent(sessionID, *v)
	case json.RawMessage:
		return []byte(v), uuid.NewString(), nil
	case []byte:
		return v, uuid.NewString(), nil
	default:
		payload, err := json.Marshal(event)
		return payload, uuid.NewString(), err
	}
}

func marshalTypedPlanningEvent(sessionID string, event model.PlanningStreamEvent) ([]byte, string, error) {
	if event.SessionID == "" {
		event.SessionID = sessionID
	}
	if event.EventID == "" {
		event.EventID = uuid.NewString()
	}
	if event.SentAt.IsZero() {
		event.SentAt = time.Now().UTC()
	}
	payload, err := json.Marshal(event)
	return payload, event.EventID, err
}

// JetStreamBridge consumes JetStream messages and forwards them to the in-process WS hub.
type JetStreamBridge struct {
	js         nats.JetStreamContext
	hub        *Hub
	instanceID string
	eventsSub  *nats.Subscription
	streamSub  *nats.Subscription
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

	streamSub, err := b.js.Subscribe(
		planningSubjectAll,
		b.handlePlanningEvent,
		nats.BindStream(planningStreamName),
		nats.Durable("api-planning-stream-"+instanceID),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(2*time.Minute),
	)
	if err != nil {
		_ = eventsSub.Drain()
		return err
	}

	b.eventsSub = eventsSub
	b.streamSub = streamSub
	slog.Info("jetstream bridge started", "instance_id", instanceID)

	<-ctx.Done()

	if b.streamSub != nil {
		_ = b.streamSub.Drain()
	}
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

func (b *JetStreamBridge) handlePlanningEvent(msg *nats.Msg) {
	sessionID := subjectTail(msg.Subject)
	if sessionID == "" {
		slog.Warn("jetstream bridge: invalid planning subject", "subject", msg.Subject)
		_ = msg.Ack()
		return
	}
	b.hub.SendToSession(sessionID, json.RawMessage(msg.Data))
	_ = msg.Ack()
}

func planningSubject(sessionID string) string {
	return "helpin.planning.stream." + sessionID
}

func workspaceEventSubject(workspaceID string) string {
	return "helpin.ws.events." + workspaceID
}

func subjectTail(subject string) string {
	parts := strings.Split(subject, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
