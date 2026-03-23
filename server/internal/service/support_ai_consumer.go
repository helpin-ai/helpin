package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// StartNATSConsumer starts the durable pull consumer for AI request events.
// Runs on the worker node, not the API process.
func (s *SupportAIService) StartNATSConsumer(ctx context.Context) error {
	if s.js == nil {
		return nil
	}

	// Ensure SUPPORT_AI stream exists.
	if err := websocket.EnsureJetStreamInfrastructure(s.js); err != nil {
		return err
	}

	sub, err := s.js.PullSubscribe(
		"support.ai.request.*",
		"ai-responder",
		nats.Bind(websocket.SupportAIStreamName, "ai-responder"),
	)
	if err != nil {
		// Consumer may not exist yet — create it.
		consumerCfg := &nats.ConsumerConfig{
			Durable:       "ai-responder",
			FilterSubject: "support.ai.request.*",
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       45 * time.Second,
			MaxDeliver:    3,
			BackOff:       []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second},
			DeliverPolicy: nats.DeliverAllPolicy,
			MaxAckPending: 10,
		}
		if _, addErr := s.js.AddConsumer(websocket.SupportAIStreamName, consumerCfg); addErr != nil {
			return addErr
		}
		sub, err = s.js.PullSubscribe(
			"support.ai.request.*",
			"ai-responder",
			nats.Bind(websocket.SupportAIStreamName, "ai-responder"),
		)
		if err != nil {
			return err
		}
	}

	slog.Info("support AI consumer started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("support AI consumer shutting down")
			return sub.Drain()
		default:
		}

		msgs, err := sub.Fetch(1, nats.MaxWait(5*time.Second))
		if err != nil {
			if err == nats.ErrTimeout {
				continue
			}
			slog.Error("support AI consumer fetch error", "error", err)
			continue
		}

		for _, msg := range msgs {
			s.processNATSMessage(ctx, msg)
		}
	}
}

// processNATSMessage handles a single NATS message.
func (s *SupportAIService) processNATSMessage(ctx context.Context, msg *nats.Msg) {
	var event AIRequestEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.Error("support AI consumer: invalid payload", "error", err)
		_ = msg.Ack()
		return
	}

	// Check delivery count for poison message handling.
	meta, _ := msg.Metadata()
	if meta != nil && meta.NumDelivered >= 3 {
		slog.Error("support AI consumer: max deliveries reached, marking failed",
			"workspace_id", event.WorkspaceID,
			"conversation_id", event.ConversationID,
			"message_id", event.MessageID,
		)
		if s.processingRepo != nil {
			if err := s.processingRepo.MarkFailedBySourceMessageID(ctx, event.MessageID); err != nil {
				slog.Error("support AI consumer: failed to mark message failed", "message_id", event.MessageID, "error", err)
			}
		}
		_ = msg.Ack()
		return
	}

	// Build a SupportMessage for the handler.
	supportMsg := &model.SupportMessage{
		ID:             event.MessageID,
		WorkspaceID:    event.WorkspaceID,
		ConversationID: event.ConversationID,
		Content:        event.Content,
		SenderType:     "customer",
	}

	if err := s.HandleIncomingMessage(ctx, event.WorkspaceID, event.ConversationID, supportMsg); err != nil {
		slog.ErrorContext(ctx, "support AI consumer: processing failed",
			"workspace_id", event.WorkspaceID,
			"conversation_id", event.ConversationID,
			"message_id", event.MessageID,
			"error", err,
		)
		_ = msg.NakWithDelay(5 * time.Second)
		return
	}

	_ = msg.Ack()
}
