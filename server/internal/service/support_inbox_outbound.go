package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// CreateConversationWithMessage creates a normal support conversation and
// immediately sends the first public teammate message.
func (s *SupportInboxService) CreateConversationWithMessage(ctx context.Context, req model.CreateConversationWithMessageRequest, actorID string) (*model.CreateConversationWithMessageResponse, error) {
	if strings.TrimSpace(req.WorkspaceID) == "" || strings.TrimSpace(req.Subject) == "" {
		return nil, fmt.Errorf("workspace_id and subject are required")
	}
	if strings.TrimSpace(req.Content) == "" && len(req.AttachmentIDs) == 0 {
		return nil, fmt.Errorf("content is required")
	}
	channels := normalizeSupportDeliveryChannels(req.Channels)
	if len(req.Channels) > 0 && len(channels) == 0 {
		return nil, fmt.Errorf("at least one supported channel is required")
	}
	if supportChannelsIncludeEmail(channels) && (req.CustomerEmail == nil || strings.TrimSpace(*req.CustomerEmail) == "") {
		return nil, fmt.Errorf("customer_email is required for email delivery")
	}

	conversation, err := s.CreateConversation(ctx, model.CreateConversationRequest{
		WorkspaceID:   req.WorkspaceID,
		MailboxID:     req.MailboxID,
		Subject:       req.Subject,
		Priority:      "medium",
		CustomerName:  req.CustomerName,
		CustomerEmail: req.CustomerEmail,
		Source:        "internal",
	}, actorID)
	if err != nil {
		return nil, err
	}
	completed := false
	defer func() {
		if !completed {
			s.cleanupFailedOutboundConversation(ctx, req.WorkspaceID, conversation.ID, actorID)
		}
	}()
	if req.CRMContactID != nil && strings.TrimSpace(*req.CRMContactID) != "" {
		conversation.CRMContactID = req.CRMContactID
		if err := s.conversationRepo.Update(ctx, conversation); err != nil {
			return nil, err
		}
	}
	if supportChannelsIncludeEmail(channels) && len(req.CCEmails) > 0 {
		ccEmails := normalizeSupportEmailListExcluding(req.CCEmails, derefString(conversation.CustomerEmail))
		conversation.EmailCC = model.DocsStringArray(ccEmails)
		if err := s.conversationRepo.UpdateFields(ctx, req.WorkspaceID, conversation.ID, map[string]any{
			"email_cc": conversation.EmailCC,
		}); err != nil {
			return nil, err
		}
	}
	if s.tagRepo != nil {
		for _, tagID := range req.TagIDs {
			tagID = strings.TrimSpace(tagID)
			if tagID == "" {
				continue
			}
			if err := s.tagRepo.AddConversationTag(ctx, req.WorkspaceID, conversation.ID, tagID); err != nil {
				return nil, err
			}
		}
		hydrated := []model.SupportConversation{*conversation}
		s.hydrateConversationTags(ctx, req.WorkspaceID, hydrated)
		conversation.Tags = hydrated[0].Tags
		conversation.SystemTags = hydrated[0].SystemTags
	}

	message, err := s.CreateConversationMessage(ctx, req.WorkspaceID, conversation.ID, model.CreateMessageRequest{
		Content:       req.Content,
		IsInternal:    false,
		MessageType:   "reply",
		AttachmentIDs: req.AttachmentIDs,
		Channels:      channels,
		CCEmails:      req.CCEmails,
		BCCEmails:     req.BCCEmails,
	}, "user", &actorID, nil, nil)
	if err != nil {
		return nil, err
	}
	completed = true
	return &model.CreateConversationWithMessageResponse{
		Conversation: conversation,
		Message:      message,
	}, nil
}

// Compensate only for a new outbound conversation that never acquired a real
// message. Do not hold a database transaction across translation or delivery I/O.
func (s *SupportInboxService) cleanupFailedOutboundConversation(ctx context.Context, workspaceID, conversationID, actorID string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	deleted, err := s.conversationRepo.DeleteIfEmpty(cleanupCtx, workspaceID, conversationID)
	if err != nil {
		slog.ErrorContext(cleanupCtx, "clean up failed outbound conversation", "error", err, "workspace_id", workspaceID, "conversation_id", conversationID)
		return
	}
	if deleted {
		s.wsPublisher.Publish(websocket.Event{
			Action: "deleted", Entity: "support_conversation", EntityID: conversationID,
			WorkspaceID: workspaceID, ActorID: actorID,
		})
	}
}
