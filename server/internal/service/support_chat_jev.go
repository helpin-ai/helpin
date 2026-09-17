package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// assessJevHandoff runs before reply generation. Uncertain or unavailable
// assessments leave the existing agent path in charge.
func (s *SupportChatService) assessJevHandoff(ctx context.Context, conv *model.SupportConversation, msg *model.SupportMessage, history []model.SupportMessage) (bool, error) {
	if msg.WorkspaceID != conv.WorkspaceID || msg.ConversationID != conv.ID || msg.MessageType != "reply" || supportConversationHumanOwned(conv) || model.SupportAIConversationBlocked(conv) || derefString(conv.LastPublicMessageID) != msg.ID {
		return false, nil
	}
	reason, err := s.jev.classifyHandoff(ctx, conv, msg, history)
	if err != nil {
		slog.WarnContext(ctx, "Jev handoff assessment unavailable; using support agent", "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID, "error", err)
		return false, nil
	}
	if reason == "" || reason == "continue" {
		return false, nil
	}
	// A customer reply, manual takeover or an edit during the provider call
	// invalidates this assessment. The control transaction repeats the ownership
	// and source fences to cover a subsequent takeover or new public reply.
	current, err := s.conversationRepo.GetByID(ctx, conv.WorkspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		return false, err
	}
	if current == nil || current.AIControlVersion != conv.AIControlVersion || derefString(current.LastPublicMessageID) != msg.ID || model.SupportAIConversationBlocked(current) || supportConversationHumanOwned(current) {
		return true, nil
	}
	latest, err := s.messageRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID, false)
	if err != nil {
		return false, err
	}
	before, valid := supportJevLifecycleState(conv.WorkspaceID, conv.ID, msg.ID, history)
	after, stillValid := supportJevLifecycleState(conv.WorkspaceID, conv.ID, msg.ID, latest)
	if !valid || !stillValid || before != after {
		return false, nil
	}
	settings, err := s.supportAIService.loadSettings(ctx, conv.WorkspaceID)
	if err != nil {
		return false, err
	}
	if !shouldAutomaticallyProcessSupportAI(*settings) || !model.SupportAIReplyAllowed(*settings, current, msg) {
		return true, nil
	}
	brief := SupportHandoffBrief{ExpectedMessageID: msg.ID, ExpectedControlVersion: &conv.AIControlVersion}
	if err := s.supportAIService.EscalateToHumanForMessageWithIssue(ctx, conv.WorkspaceID, conv.ID, msg.ID, reason, "", "", brief); err != nil {
		return false, fmt.Errorf("Jev handoff: %w", err)
	}
	return true, nil
}
