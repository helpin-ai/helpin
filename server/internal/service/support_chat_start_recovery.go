package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Retry delivery handles transient launch errors. This backstop hands over turns
// abandoned before a run exists (including a worker crash or lost redelivery).
func (s *SupportChatService) sweepUnstartedSupportTurns(ctx context.Context, limit int) error {
	if s.supportAIService == nil || s.conversationRepo == nil || s.messageRepo == nil {
		return nil
	}
	rows, err := s.processingRepo.ListUnstartedOlderThan(ctx, time.Now().Add(-2*time.Minute), limit)
	if err != nil {
		return err
	}
	for _, row := range rows {
		conv, err := s.conversationRepo.GetByID(ctx, row.WorkspaceID, row.ConversationID, "", model.RoleOwner)
		if err != nil {
			return err
		}
		if !supportAIConversationSupported(conv) || conv.AIActiveRunID != nil || model.SupportAIConversationBlocked(conv) || supportConversationHumanOwned(conv) || derefString(conv.LastPublicMessageID) != row.SourceMessageID {
			continue
		}
		settings, err := s.supportAIService.loadSettings(ctx, row.WorkspaceID)
		if err != nil {
			return err
		}
		*settings = effectiveSupportAISettings(*settings, conv)
		source, err := s.messageRepo.GetByID(ctx, row.SourceMessageID)
		if err != nil {
			return err
		}
		if source == nil || !supportAIMessageEligible(source) || !shouldAutomaticallyProcessSupportAI(*settings) || !model.SupportAIReplyAllowed(*settings, conv, source) {
			continue
		}
		brief := SupportHandoffBrief{ExpectedNoRun: true, ExpectedMessageID: row.SourceMessageID, ExpectedControlVersion: &conv.AIControlVersion, UnresolvedQuestions: []string{"AI did not start a response within the recovery window. A teammate needs to answer the customer's message."}}
		if err := s.supportAIService.EscalateToHumanForMessageWithIssue(ctx, row.WorkspaceID, conv.ID, row.SourceMessageID, "ai_pipeline_error", "", "", brief); err != nil {
			slog.ErrorContext(ctx, "support chat: unstarted turn handover failed", "workspace_id", row.WorkspaceID, "conversation_id", conv.ID, "error", err)
			continue
		}
	}
	return nil
}
