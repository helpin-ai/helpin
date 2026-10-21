package service

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// UpdateNote edits only a human author's own internal note. It deliberately
// bypasses message creation, email delivery, automation, and mention fan-out.
func (s *SupportMessageActionsService) UpdateNote(ctx context.Context, workspaceID, conversationID, actorID, messageID string, req model.UpdateSupportNoteRequest) (*model.SupportMessage, error) {
	if s == nil || s.messageRepo == nil {
		return nil, ErrSupportMessageActionsMiswired
	}
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg == nil || msg.WorkspaceID != workspaceID || msg.ConversationID != conversationID {
		return nil, ErrSupportMessageActionNotFound
	}
	if strings.TrimSpace(actorID) == "" || msg.SenderUserID == nil || *msg.SenderUserID != actorID || msg.SenderType != "user" || !msg.IsInternal || msg.MessageType != "reply" {
		return nil, ErrSupportMessageActionForbidden
	}
	if req.ExpectedUpdatedAt.IsZero() || len(req.Content) > 1_000_000 || (req.AttachmentIDs != nil && len(*req.AttachmentIDs) > 100) {
		return nil, repository.ErrSupportNoteInvalid
	}
	updated, err := s.messageRepo.UpdateNote(ctx, workspaceID, conversationID, actorID, messageID, req, s.now())
	if err != nil {
		return nil, err
	}
	if s.wsPublisher != nil && !msg.UpdatedAt.Equal(updated.UpdatedAt) {
		s.wsPublisher.Publish(websocket.SupportMessageUpdatedEvent(workspaceID, updated, actorID))
	}
	return updated, nil
}
