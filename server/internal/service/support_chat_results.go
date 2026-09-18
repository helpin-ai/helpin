package service

// Pause-hook + sweep for support chat runs: when a turn ends (the run pauses
// awaiting the next visitor message), verify the turn actually produced an
// outcome (send_reply/escalate settled the processing row — nudge once, then
// escalate), stop the thinking indicator, and drain visitor messages that
// arrived mid-turn into one coalesced resume. The 30s sweep is the backstop
// for missed pause events.

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportChatNudgeContent = "System correction: you ended your turn without responding to the visitor. You MUST respond now with exactly one call to send_support_reply, or hand off with escalate_to_human. Do not end your turn silently."

// OnSupportChatRunPaused runs after the projection persists a chat run's
// pause. Cheap no-op for non-support-chat runs.
func (s *SupportChatService) OnSupportChatRunPaused(ctx context.Context, run *model.AgentRun) {
	if s == nil || run == nil || strings.TrimSpace(run.TargetType) != "support_conversation" {
		return
	}
	if runInputTriggerType(run) != supportChatTriggerType {
		return
	}
	conversationID := strings.TrimSpace(derefString(run.ConversationID))
	if conversationID == "" {
		conversationID = strings.TrimSpace(run.TargetID)
	}
	if run.PauseReason != model.AgentRunPauseReasonUserMessage {
		return
	}

	if s.closeSupportChatRunIfTerminal(ctx, run, conversationID, time.Now().UTC()) {
		s.supportAIService.publishTypingIndicator(ctx, run.WorkspaceID, conversationID, false)
		return
	}
	if s.waitingForSupportResult(ctx, run, conversationID) {
		s.supportAIService.publishProgress(run.WorkspaceID, conversationID, supportAIProgressChecking)
		s.drainDeferredMessages(ctx, run, conversationID)
		return
	}
	s.supportAIService.publishTypingIndicator(ctx, run.WorkspaceID, conversationID, false)

	if s.enforceTurnSettlement(ctx, run, conversationID) {
		// A nudge resume was sent — the run is active again; deferred
		// messages drain on the next pause.
		return
	}
	s.drainDeferredMessages(ctx, run, conversationID)
}

// A paused parent with pending child work has not gone silent. Keep its source
// turn open; result delivery resumes it without requiring another visitor message.
func (s *SupportChatService) waitingForSupportResult(ctx context.Context, run *model.AgentRun, conversationID string) bool {
	if s.planRepo == nil || s.processingRepo == nil {
		return false
	}
	row, err := s.processingRepo.LatestProcessingForConversation(ctx, run.WorkspaceID, conversationID)
	if err != nil {
		slog.WarnContext(ctx, "support chat: load pending turn", "error", err)
		return true
	}
	if row == nil {
		return false
	}
	pending, err := s.planRepo.HasPendingSupportResult(ctx, run.WorkspaceID, conversationID, run.ID)
	if err != nil {
		slog.WarnContext(ctx, "support chat: load pending child work", "error", err)
		return true
	}
	return pending
}

// closeSupportChatRunIfTerminal ends chat-mode runtime runs that no longer
// have a valid next AI turn. Human handoff closes immediately; ordinary idle
// support chats retain their 24-hour continuation window.
func (s *SupportChatService) closeSupportChatRunIfTerminal(ctx context.Context, run *model.AgentRun, conversationID string, now time.Time) bool {
	if s == nil || run == nil || s.conversationRepo == nil {
		return false
	}
	conversation, err := s.conversationRepo.GetByID(ctx, run.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		slog.WarnContext(ctx, "support chat: load conversation for run closure failed", "error", err, "conversation_id", conversationID, "run_id", run.ID)
		return false
	}
	settings, err := s.supportAIService.loadSettings(ctx, run.WorkspaceID)
	if err != nil {
		return true
	}
	closeRun, reason := supportChatRunClosureDecision(run, conversation, now, *settings)
	if conversation != nil && conversation.AIControlVersion > 0 && derefString(conversation.AIActiveRunID) != run.ID {
		closeRun, reason = true, "ownership_changed"
	}
	if !closeRun && conversation != nil {
		allowed, err := s.channelAllowsPendingTurn(ctx, *settings, conversation)
		if err != nil {
			return true
		}
		if !allowed {
			closeRun = true
			reason = "unsupported_channel"
		}
	}
	if !closeRun {
		return false
	}
	if s.runCloser == nil {
		slog.WarnContext(ctx, "support chat: run closer unavailable", "conversation_id", conversationID, "run_id", run.ID, "reason", reason)
		return true
	}
	if _, err := s.runCloser.CancelRun(ctx, run.WorkspaceID, run.ID, ""); err != nil {
		slog.WarnContext(ctx, "support chat: close terminal chat run failed", "error", err, "conversation_id", conversationID, "run_id", run.ID, "reason", reason)
	} else {
		slog.InfoContext(ctx, "support chat run closed", "conversation_id", conversationID, "run_id", run.ID, "reason", reason)
	}
	// Once a human owns the conversation, never nudge or resume the AI even if
	// runtime cancellation needs a later sweep retry.
	return true
}

func supportChatRunClosureDecision(run *model.AgentRun, conversation *model.SupportConversation, now time.Time, channelSettings ...model.SupportInboxSettings) (bool, string) {
	settings := model.DefaultSupportInboxSettings()
	if len(channelSettings) > 0 {
		settings = channelSettings[0]
	}
	if run == nil || strings.TrimSpace(run.TargetType) != "support_conversation" || runInputTriggerType(run) != supportChatTriggerType {
		return false, ""
	}
	if conversation != nil && (!supportAIConversationSupported(conversation) || ((conversation.Channel == "email" || conversation.Source == "email") && !model.SupportAIReplyAllowed(settings, conversation, nil))) {
		return true, "unsupported_channel"
	}
	if conversation != nil && model.SupportAIConversationBlocked(conversation) {
		return true, "human_handoff"
	}
	if run.Status == model.AgentRunStatusPaused && run.PauseReason == model.AgentRunPauseReasonUserMessage &&
		!run.UpdatedAt.IsZero() && now.Sub(run.UpdatedAt) >= time.Duration(defaultSupportChatIdleTimeoutSeconds)*time.Second {
		return true, "idle_timeout"
	}
	return false, ""
}

// enforceTurnSettlement checks that the just-ended turn produced its outcome.
// Returns true when it resumed the run (nudge), so callers stop processing.
func (s *SupportChatService) enforceTurnSettlement(ctx context.Context, run *model.AgentRun, conversationID string) bool {
	row, err := s.processingRepo.LatestProcessingForConversation(ctx, run.WorkspaceID, conversationID)
	if err != nil || row == nil {
		return false
	}
	// Still 'processing' after the run paused = the agent went silent.
	if row.Attempts <= 1 {
		if err := s.processingRepo.IncrementAttempts(ctx, row.ID); err != nil {
			slog.WarnContext(ctx, "support chat: mark nudge failed", "error", err, "conversation_id", conversationID)
			return false
		}
		if _, err := s.agentService.SendRunMessage(ctx, run.WorkspaceID, run.ID, "", model.SendAgentRunMessageRequest{Content: supportChatNudgeContent}); err != nil {
			slog.WarnContext(ctx, "support chat: nudge resume failed", "error", err, "conversation_id", conversationID)
			return false
		}
		s.supportAIService.publishTypingIndicator(ctx, run.WorkspaceID, conversationID, true)
		return true
	}
	// Already nudged once — give up and hand off.
	if err := s.supportAIService.EscalateToHumanForMessage(ctx, run.WorkspaceID, conversationID, row.SourceMessageID, "agent_no_reply"); err != nil {
		slog.ErrorContext(ctx, "support chat: agent_no_reply escalation failed", "error", err, "conversation_id", conversationID)
		return false
	}
	_ = s.processingRepo.MarkCompleted(ctx, row.ID, nil, 0)
	return false
}

// drainDeferredMessages coalesces messages parked mid-turn into one resume.
func (s *SupportChatService) drainDeferredMessages(ctx context.Context, run *model.AgentRun, conversationID string) {
	if s.closeSupportChatRunIfTerminal(ctx, run, conversationID, time.Now().UTC()) {
		return
	}
	rows, err := s.processingRepo.ListDeferredForConversation(ctx, run.WorkspaceID, conversationID)
	if err != nil || len(rows) == 0 {
		return
	}
	rows = s.filterChatDeferredMessages(ctx, rows)
	if len(rows) == 0 {
		return
	}
	contents := make([]string, 0, len(rows))
	for _, row := range rows {
		message, msgErr := s.messageRepo.GetByID(ctx, row.SourceMessageID)
		if msgErr != nil || message == nil {
			continue
		}
		if content := strings.TrimSpace(message.Content); content != "" {
			contents = append(contents, content)
		}
	}
	if len(contents) == 0 {
		for _, row := range rows {
			_ = s.processingRepo.MarkCompleted(ctx, row.ID, nil, 0)
		}
		return
	}
	// The newest row becomes the turn being settled; older ones are folded in.
	for _, row := range rows[:len(rows)-1] {
		_ = s.processingRepo.MarkCompleted(ctx, row.ID, nil, 0)
	}
	if err := s.processingRepo.MarkProcessing(ctx, rows[len(rows)-1].ID); err != nil {
		slog.WarnContext(ctx, "support chat: reclaim deferred row failed", "error", err, "conversation_id", conversationID)
	}

	composed := contents[0]
	if len(contents) > 1 {
		composed = "The visitor sent several messages:\n- " + strings.Join(contents, "\n- ")
	}
	if _, err := s.agentService.SendRunMessage(ctx, run.WorkspaceID, run.ID, "", model.SendAgentRunMessageRequest{Content: composed}); err != nil {
		slog.WarnContext(ctx, "support chat: coalesced resume failed", "error", err, "conversation_id", conversationID)
		// Re-park so the sweep retries.
		_ = s.processingRepo.MarkDeferred(ctx, rows[len(rows)-1].ID)
		return
	}
	s.supportAIService.publishTypingIndicator(ctx, run.WorkspaceID, conversationID, true)
}

// SweepSupportChat is the backstop for missed pause events: it retries parked
// messages whose conversation run is idle-paused.
func (s *SupportChatService) SweepSupportChat(ctx context.Context, staleAfter time.Duration, limit int) error {
	if s == nil || s.processingRepo == nil {
		return nil
	}
	rows, err := s.processingRepo.ListDeferredOlderThan(ctx, time.Now().Add(-staleAfter), limit)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, row := range rows {
		if seen[row.ConversationID] {
			continue
		}
		seen[row.ConversationID] = true
		conv, convErr := s.conversationRepo.GetByID(ctx, row.WorkspaceID, row.ConversationID, "", model.RoleOwner)
		if convErr != nil || conv == nil || conv.AIActiveRunID == nil {
			continue
		}
		run, runErr := s.runRepo.GetByID(ctx, row.WorkspaceID, *conv.AIActiveRunID)
		if runErr != nil || run == nil {
			continue
		}
		if run.Status == model.AgentRunStatusPaused && run.PauseReason == model.AgentRunPauseReasonUserMessage {
			s.drainDeferredMessages(ctx, run, row.ConversationID)
		} else if !model.IsAgentRunActiveStatus(run.Status) {
			// Run ended while messages were parked: start a successor with
			// the parked content folded in via the normal path.
			s.reviveDeferredConversation(ctx, conv, run)
		}
	}
	return s.sweepClosableSupportChatRuns(ctx, staleAfter, limit)
}

func (s *SupportChatService) sweepClosableSupportChatRuns(ctx context.Context, staleAfter time.Duration, limit int) error {
	if s == nil || s.runRepo == nil {
		return nil
	}
	if staleAfter <= 0 {
		staleAfter = 30 * time.Second
	}
	now := time.Now().UTC()
	runs, err := s.runRepo.ListActiveByExternalRuntime(ctx, agentRuntimeName, now.Add(-staleAfter), limit)
	if err != nil {
		return err
	}
	for idx := range runs {
		run := &runs[idx]
		if strings.TrimSpace(run.TargetType) != "support_conversation" || runInputTriggerType(run) != supportChatTriggerType {
			continue
		}
		conversationID := strings.TrimSpace(derefString(run.ConversationID))
		if conversationID == "" {
			conversationID = strings.TrimSpace(run.TargetID)
		}
		if conversationID == "" {
			continue
		}
		s.closeSupportChatRunIfTerminal(ctx, run, conversationID, now)
	}
	return nil
}

// reviveDeferredConversation starts a successor run for parked messages whose
// run ended before they could be delivered.
func (s *SupportChatService) reviveDeferredConversation(ctx context.Context, conv *model.SupportConversation, previousRun *model.AgentRun) {
	if !supportAIConversationSupported(conv) {
		return
	}
	rows, err := s.processingRepo.ListDeferredForConversation(ctx, conv.WorkspaceID, conv.ID)
	if err != nil || len(rows) == 0 {
		return
	}
	rows = s.filterChatDeferredMessages(ctx, rows)
	if len(rows) == 0 {
		return
	}
	contents := make([]string, 0, len(rows))
	for _, row := range rows {
		message, msgErr := s.messageRepo.GetByID(ctx, row.SourceMessageID)
		if msgErr != nil || message == nil {
			continue
		}
		if content := strings.TrimSpace(message.Content); content != "" {
			contents = append(contents, content)
		}
	}
	for _, row := range rows[:max(len(rows)-1, 0)] {
		_ = s.processingRepo.MarkCompleted(ctx, row.ID, nil, 0)
	}
	if len(contents) == 0 {
		if len(rows) > 0 {
			_ = s.processingRepo.MarkCompleted(ctx, rows[len(rows)-1].ID, nil, 0)
		}
		return
	}
	_ = s.processingRepo.MarkProcessing(ctx, rows[len(rows)-1].ID)

	settings, err := s.supportAIService.loadSettings(ctx, conv.WorkspaceID)
	if err != nil {
		return
	}
	agentID := strings.TrimSpace(derefString(settings.AIAgentID))
	if agentID == "" {
		return
	}
	agent, err := s.agentService.GetAgent(ctx, conv.WorkspaceID, agentID)
	if err != nil || agent == nil {
		return
	}
	composed := contents[0]
	if len(contents) > 1 {
		composed = "The visitor sent several messages:\n- " + strings.Join(contents, "\n- ")
	}
	if err := s.startSupportChatRun(ctx, conv, agent, composed, previousRun, nil); err != nil {
		slog.WarnContext(ctx, "support chat: revive successor failed", "error", err, "conversation_id", conv.ID)
		_ = s.processingRepo.MarkDeferred(ctx, rows[len(rows)-1].ID)
	}
}

// StartSupportChatSweep runs SweepSupportChat on a ticker until the context
// is cancelled. Wired next to the dock result sweep in cmd/api.
func (s *SupportChatService) StartSupportChatSweep(ctx context.Context, interval, staleAfter time.Duration, limit int) error {
	if s == nil || s.processingRepo == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "support chat sweep panic", "panic", recovered)
		}
	}()
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if staleAfter <= 0 {
		staleAfter = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		if s.followUpService != nil {
			if err := s.followUpService.Tick(ctx); err != nil {
				slog.ErrorContext(ctx, "support follow-up sweep failed", "error", err)
			}
		}
		if err := s.SweepSupportChat(ctx, staleAfter, limit); err != nil {
			slog.ErrorContext(ctx, "support chat sweep failed", "error", err)
		}
		if err := s.SweepUnnotifiedSupportChatResults(ctx, limit); err != nil {
			slog.ErrorContext(ctx, "support chat result sweep failed", "error", err)
		}
	}
}

func (s *SupportChatService) filterChatDeferredMessages(ctx context.Context, rows []model.AIMessageProcessing) []model.AIMessageProcessing {
	var eligible []model.AIMessageProcessing
	for _, row := range rows {
		message, err := s.messageRepo.GetByID(ctx, row.SourceMessageID)
		if err != nil {
			continue
		}
		settings, err := s.supportAIService.loadSettings(ctx, row.WorkspaceID)
		if err != nil {
			continue
		}
		conv, err := s.conversationRepo.GetByID(ctx, row.WorkspaceID, row.ConversationID, "", model.RoleOwner)
		if err != nil {
			continue
		}
		if message == nil || model.SupportAIConversationBlocked(conv) || !model.SupportAIReplyAllowed(*settings, conv, message) {
			_ = s.processingRepo.MarkCompleted(ctx, row.ID, nil, 0)
			continue
		}
		eligible = append(eligible, row)
	}
	return eligible
}
