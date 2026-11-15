package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"gorm.io/gorm"
)

// Save the customer reply and its reopen transition under the same conversation
// lock. Concurrent replies emit one reopen, and failures leave neither behind.
func (s *SupportInboxService) createWidgetReply(ctx context.Context, msg *model.SupportMessage) (*model.SupportMessage, error) {
	var reopened *model.SupportMessage
	changed := false
	err := s.messageRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		reopened, changed, err = s.createCustomerReplyTx(ctx, tx, msg)
		return err
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.publishConversationReopened(msg)
	}
	return reopened, nil
}

// createCustomerReplyTx saves a customer reply inside the caller's
// transaction and reopens a resolved or waiting conversation. changed reports
// a status change for the caller to publish after commit.
func (s *SupportInboxService) createCustomerReplyTx(ctx context.Context, tx *gorm.DB, msg *model.SupportMessage) (reopened *model.SupportMessage, changed bool, err error) {
	messages := s.messageRepo.WithTx(tx)
	if err := messages.Create(ctx, msg); err != nil {
		return nil, false, err
	}
	// Create holds the routing and conversation locks, including merged roots.
	conv, err := repository.NewSupportMergeRepository(tx).LockConversation(ctx, msg.WorkspaceID, msg.ConversationID)
	if err != nil {
		return nil, false, err
	}
	if conv.Status != model.SupportConversationStatusResolved && conv.Status != model.SupportConversationStatusWaitingOnCustomer {
		return nil, false, nil
	}
	if err := s.conversationRepo.WithTx(tx).UpdateFields(ctx, msg.WorkspaceID, msg.ConversationID, map[string]any{
		"status":      model.SupportConversationStatusOpen,
		"flow_state":  supportEmailReopenFlowState(conv),
		"resolved_at": nil, "closed_at": nil, "ai_resolved_at": nil, "ai_resolution_type": nil,
	}); err != nil {
		return nil, false, err
	}
	if conv.Status == model.SupportConversationStatusResolved {
		reopened = customerReopenedMessage(conv)
		if err := messages.Create(ctx, reopened); err != nil {
			return nil, false, err
		}
	}
	return reopened, true, nil
}

func (s *SupportInboxService) publishConversationReopened(msg *model.SupportMessage) {
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_conversation", EntityID: msg.ConversationID, WorkspaceID: msg.WorkspaceID})
}

func customerReopenedMessage(conv *model.SupportConversation) *model.SupportMessage {
	return &model.SupportMessage{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID,
		SenderType: "user", Content: "Customer reply reopened this conversation.",
		MessageType: "system", SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventReopened), IsInternal: true,
	}
}
