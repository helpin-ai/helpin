package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// SupportChatMessageHandler processes one visitor message; the production
// handler is SupportChatService.HandleVisitorMessage.
type SupportChatMessageHandler func(ctx context.Context, workspaceID, conversationID string, msg *model.SupportMessage) error

// StartNATSConsumer starts the durable pull consumer for AI request events
// and routes each visitor message through handler. Runs in the API process
// (alongside the agent-runtime projection the chat runs depend on); NATS
// keeps its role as serializer / retry / poison-message layer.
func (s *SupportAIService) StartNATSConsumer(ctx context.Context, handler SupportChatMessageHandler) error {
	if s.js == nil || handler == nil {
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
			s.processNATSMessage(ctx, msg, handler)
		}
	}
}

// processNATSMessage handles a single NATS message.
func (s *SupportAIService) processNATSMessage(ctx context.Context, msg *nats.Msg, handler SupportChatMessageHandler) {
	var event AIRequestEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.Error("support AI consumer: invalid payload", "error", err)
		_ = msg.Ack()
		return
	}

	meta, _ := msg.Metadata()
	supportMsg, loadErr := s.loadIncomingSupportMessage(ctx, event)
	if loadErr != nil {
		if meta != nil && meta.NumDelivered >= 3 {
			if err := s.EscalateToHumanForMessage(ctx, event.WorkspaceID, event.ConversationID, event.MessageID, "ai_pipeline_error"); err != nil {
				slog.ErrorContext(ctx, "support AI consumer: load failure escalation failed", "error", err)
			}
			if s.processingRepo != nil {
				_ = s.processingRepo.MarkFailedBySourceMessageID(ctx, event.MessageID)
			}
			_ = msg.Ack()
			return
		}
		slog.ErrorContext(ctx, "support AI consumer: load incoming support message failed",
			"workspace_id", event.WorkspaceID,
			"conversation_id", event.ConversationID,
			"message_id", event.MessageID,
			"error", loadErr,
		)
		_ = msg.NakWithDelay(5 * time.Second)
		return
	}

	if !supportAIMessageIsChat(supportMsg) {
		_ = msg.Ack()
		return
	}
	// Poison-message handling: retries exhausted → a human takes over.
	if meta != nil && meta.NumDelivered >= 3 {
		slog.Error("support AI consumer: max deliveries reached, escalating to human",
			"workspace_id", event.WorkspaceID,
			"conversation_id", event.ConversationID,
			"message_id", event.MessageID,
		)
		if err := s.EscalateToHumanForMessage(ctx, event.WorkspaceID, event.ConversationID, event.MessageID, "ai_pipeline_error"); err != nil {
			slog.Error("support AI consumer: poison escalation failed", "message_id", event.MessageID, "error", err)
		}
		if s.processingRepo != nil {
			if err := s.processingRepo.MarkFailedBySourceMessageID(ctx, event.MessageID); err != nil {
				slog.Error("support AI consumer: failed to mark message failed", "message_id", event.MessageID, "error", err)
			}
		}
		_ = msg.Ack()
		return
	}

	if err := handler(ctx, event.WorkspaceID, event.ConversationID, supportMsg); err != nil {
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

func (s *SupportAIService) loadIncomingSupportMessage(ctx context.Context, event AIRequestEvent) (*model.SupportMessage, error) {
	supportMsg := &model.SupportMessage{
		ID:             event.MessageID,
		WorkspaceID:    event.WorkspaceID,
		ConversationID: event.ConversationID,
		Content:        event.Content,
		SenderType:     "customer",
	}
	if s == nil || s.messageRepo == nil {
		return supportMsg, nil
	}

	saved, err := s.messageRepo.GetByID(ctx, event.MessageID)
	if err != nil {
		return nil, err
	}
	if saved == nil {
		return supportMsg, nil
	}
	if strings.TrimSpace(saved.Content) == "" && strings.TrimSpace(event.Content) != "" {
		saved.Content = event.Content
	}
	if s.attachmentRepo != nil {
		msgs := []model.SupportMessage{*saved}
		if err := s.hydrateSupportMessageAttachments(ctx, msgs); err != nil {
			return nil, err
		}
		*saved = msgs[0]
	}
	return saved, nil
}

func (s *SupportAIService) hydrateSupportMessageAttachments(ctx context.Context, messages []model.SupportMessage) error {
	if s == nil || s.attachmentRepo == nil || len(messages) == 0 {
		return nil
	}

	messageIDs := make([]string, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.ID) != "" {
			messageIDs = append(messageIDs, message.ID)
		}
	}
	if len(messageIDs) == 0 {
		return nil
	}

	attachments, err := s.attachmentRepo.ListByMessageIDs(ctx, messageIDs)
	if err != nil {
		return err
	}
	if len(attachments) == 0 {
		return nil
	}

	byMessageID := make(map[string][]model.SupportAttachmentPayload, len(attachments))
	for _, attachment := range attachments {
		if attachment.MessageID == nil {
			continue
		}
		byMessageID[*attachment.MessageID] = append(byMessageID[*attachment.MessageID], model.SupportAttachmentPayload{
			ID:       attachment.ID,
			FileKey:  attachment.StorageKey,
			FileName: attachment.FileName,
			FileType: attachment.ContentType,
			FileSize: attachment.FileSize,
			URL:      attachment.PublicURL,
		})
	}

	for i := range messages {
		messages[i].Attachments = byMessageID[messages[i].ID]
	}
	return nil
}
