package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// checkWidgetEmailRequirement enforces the saved policy independently of widget UI.
func (s *SupportInboxService) checkWidgetEmailRequirement(ctx context.Context, workspaceID, conversationID, email string, handoff bool) error {
	if strings.TrimSpace(email) != "" || s.installationRepo == nil {
		return nil
	}
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	if inst == nil {
		return nil
	}
	settings := parseSettings(inst.Settings)
	if !settings.ForceVisitorIdentity || settings.EmailCaptureTiming == "never" {
		return nil
	}
	conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return err
	}
	if conv == nil || strings.TrimSpace(derefString(conv.CustomerEmail)) != "" {
		return nil
	}
	if handoff {
		if conv.Channel != "widget" && conv.Source != "widget" {
			return nil
		}
		return fmt.Errorf("email is required")
	}
	if settings.EmailCaptureTiming != "after_ai_reply" {
		return nil
	}
	messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if message.SenderType != "ai" || message.MessageType == "system" {
			continue
		}
		var metadata AIMessageMetadata
		if message.Metadata != "" {
			if err := json.Unmarshal([]byte(message.Metadata), &metadata); err != nil {
				return fmt.Errorf("read AI reply metadata: %w", err)
			}
		}
		if metadata.AIProgressState != supportAIProgressChecking {
			return fmt.Errorf("email is required")
		}
	}
	return nil
}

func publishEmailCollectionStatus(ctx context.Context, messages *repository.SupportMessageRepository, publisher *websocket.Publisher, workspaceID, conversationID, eventType string) {
	if messages == nil {
		return
	}
	content := "Visitor provided their email."
	if eventType == model.SystemEventVisitorEmailRequested {
		content = "Waiting for visitor’s email."
	}
	message, err := messages.CreateEmailCollectionStatus(ctx, workspaceID, conversationID, eventType, content)
	if err != nil {
		slog.ErrorContext(ctx, "record email collection status", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		return
	}
	if message != nil {
		publisher.Publish(websocket.SupportMessageEvent(workspaceID, message, ""))
	}
}

func (s *SupportAIService) recordRequiredEmailWait(ctx context.Context, workspaceID, conversationID, progressState string) {
	if progressState == supportAIProgressChecking || s.installationRepo == nil {
		return
	}
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "load email collection settings", "workspace_id", workspaceID, "error", err)
		return
	}
	if inst == nil {
		return
	}
	settings := parseSettings(inst.Settings)
	if settings.EmailCaptureTiming != "after_ai_reply" || !settings.ForceVisitorIdentity {
		return
	}
	conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		slog.ErrorContext(ctx, "load email collection conversation", "conversation_id", conversationID, "error", err)
		return
	}
	if conv == nil || (conv.Channel != "widget" && conv.Source != "widget") || strings.TrimSpace(derefString(conv.CustomerEmail)) != "" {
		return
	}
	publishEmailCollectionStatus(ctx, s.messageRepo, s.wsPublisher, workspaceID, conversationID, model.SystemEventVisitorEmailRequested)
}
