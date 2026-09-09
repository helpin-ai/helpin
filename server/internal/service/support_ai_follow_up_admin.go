package service

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// CancelConversationFollowUp checks mailbox access before cancelling pending AI work.
func (s *SupportInboxService) CancelConversationFollowUp(ctx context.Context, workspaceID, id string) (*model.SupportConversation, error) {
	conv, err := s.loadConversationAccessible(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if s.followUpRepo == nil {
		return nil, fmt.Errorf("follow-up service unavailable")
	}
	if err := s.followUpRepo.CancelConversation(ctx, workspaceID, id); err != nil {
		return nil, err
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_conversation", WorkspaceID: workspaceID, EntityID: id})
	}
	return s.GetConversation(ctx, workspaceID, id)
}

// PreviewConversationFollowUps counts candidate conversations without any side effects.
func (s *SupportInboxService) PreviewConversationFollowUps(ctx context.Context, workspaceID string) (*model.SupportFollowUpPreview, error) {
	if s.followUpRepo == nil {
		return nil, fmt.Errorf("follow-up service unavailable")
	}
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	settings := model.DefaultSupportInboxSettings()
	if inst != nil {
		settings = parseSettings(inst.Settings)
	}
	return s.followUpRepo.Preview(ctx, workspaceID, settings, time.Now().UTC())
}
