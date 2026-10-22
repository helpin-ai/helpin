package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Progress is durable but not an outcome. Reuse the existing pause recovery for
// direct investigations, preserving child-result delivery when a child is pending.
func (s *SupportChatService) sweepAcknowledgedSupportTurns(ctx context.Context, staleAfter time.Duration, limit int) error {
	if s.runRepo == nil || s.supportAIService == nil {
		return nil
	}
	if staleAfter <= 0 {
		staleAfter = 30 * time.Second
	}
	rows, err := s.processingRepo.ListAcknowledgedOlderThan(ctx, time.Now().Add(-staleAfter), limit)
	if err != nil {
		return err
	}
	for _, row := range rows {
		conv, err := s.conversationRepo.GetByID(ctx, row.WorkspaceID, row.ConversationID, "", model.RoleOwner)
		if err != nil {
			return err
		}
		if conv == nil || conv.AIActiveRunID == nil || conv.AnonymizedAt != nil || conv.MergedIntoID != nil || model.SupportAIConversationBlocked(conv) {
			continue
		}
		run, err := s.runRepo.GetByID(ctx, row.WorkspaceID, *conv.AIActiveRunID)
		if err != nil {
			return err
		}
		if run == nil || run.TargetType != "support_conversation" || run.TargetID != conv.ID || runInputTriggerType(run) != supportChatTriggerType {
			continue
		}
		// A newer customer turn may already be in progress. Never recover an old
		// acknowledgment into that turn or hand over on its behalf.
		current, err := s.processingRepo.LatestProcessingForConversation(ctx, row.WorkspaceID, row.ConversationID)
		if err != nil {
			return err
		}
		if current == nil || current.ID != row.ID || s.waitingForSupportResult(ctx, run, conv.ID) {
			continue
		}
		if run.Status == model.AgentRunStatusPaused && run.PauseReason == model.AgentRunPauseReasonUserMessage {
			s.OnSupportChatRunPaused(ctx, run)
			continue
		}
		if model.IsAgentRunActiveStatus(run.Status) {
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
		if source == nil || !shouldAutomaticallyProcessSupportAI(*settings) || !model.SupportAIReplyAllowed(*settings, conv, source) {
			continue
		}
		if err := s.supportAIService.EscalateToHumanForMessageWithIssue(ctx, row.WorkspaceID, conv.ID, row.SourceMessageID, "agent_no_reply", "", "", SupportHandoffBrief{ExpectedRunID: run.ID, UnresolvedQuestions: []string{"The investigation ended after an acknowledgment without a final answer."}}); err != nil {
			slog.ErrorContext(ctx, "support progress recovery handover failed", "workspace_id", row.WorkspaceID, "conversation_id", conv.ID, "run_id", run.ID, "error", err)
			continue
		}
		if err := s.processingRepo.MarkCompleted(ctx, row.ID, nil, 0); err != nil {
			return err
		}
		s.supportAIService.publishTypingIndicator(ctx, row.WorkspaceID, conv.ID, false)
	}
	return nil
}
