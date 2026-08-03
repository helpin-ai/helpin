package service

// Child-plan result delivery for support chat runs: when a plan launched from
// a support conversation's chat run settles, its result is resumed into the
// chat run as a <child_run_result> block (same contract as the dock). A busy
// run is retried by the sweep; an ended run gets a successor so the waiting
// visitor still receives the answer.

import (
	"context"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportChildResultResumePrefix = "support-child:"

// NotifyPlanSettledForRun is the immediate delivery hook: called by the
// agent-runtime terminal finalizer for command-bar child runs, it resolves the
// run's plan and, when settled, delivers the result into the parent support
// chat run.
func (s *SupportChatService) NotifyPlanSettledForRun(ctx context.Context, run *model.AgentRun) error {
	if s == nil || s.planRepo == nil || run == nil {
		return nil
	}
	payload, ok := commandBarRunPayload(run)
	if !ok || strings.TrimSpace(payload.PlanID) == "" {
		return nil
	}
	plan, err := s.planRepo.GetByID(ctx, run.WorkspaceID, payload.PlanID)
	if err != nil || plan == nil {
		return err
	}
	return s.notifySupportPlanSettled(ctx, plan)
}

// SweepUnnotifiedSupportChatResults retries settled support-launched plans
// whose parent chat run was busy at finalize time.
func (s *SupportChatService) SweepUnnotifiedSupportChatResults(ctx context.Context, limit int) error {
	if s == nil || s.planRepo == nil {
		return nil
	}
	plans, err := s.planRepo.ListSettledUnnotifiedDockPlans(ctx, limit)
	if err != nil {
		return err
	}
	for i := range plans {
		if plans[i].SupportConversationID == nil {
			continue
		}
		if err := s.notifySupportPlanSettled(ctx, &plans[i]); err != nil {
			slog.WarnContext(ctx, "support chat result delivery failed",
				"workspace_id", plans[i].WorkspaceID, "plan_id", plans[i].ID, "error", err)
		}
	}
	return nil
}

// notifySupportPlanSettled delivers one settled plan's result into its support
// conversation's chat run. Idempotent via parent_notified_at + runtime
// resume_id dedupe.
func (s *SupportChatService) notifySupportPlanSettled(ctx context.Context, plan *model.CommandBarPlanRecord) error {
	if plan == nil || plan.ParentChatRunID == nil || plan.ParentNotifiedAt != nil {
		return nil
	}
	if !isDockPlanTerminalStatus(plan.Status) || plan.SupportConversationID == nil {
		return nil
	}
	conversationID := strings.TrimSpace(*plan.SupportConversationID)
	conv, err := s.conversationRepo.GetByID(ctx, plan.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return err
	}
	if conv == nil || supportPlanDeliveryBlocked(conv) {
		// Orphaned plan, or the conversation left AI handling (takeover /
		// escalation / visitor asked for a human): a human owns it now, so the
		// child result is permanently undeliverable to the agent.
		return s.planRepo.MarkParentNotified(ctx, plan.WorkspaceID, plan.ID)
	}

	var chatRun *model.AgentRun
	if conv.AIActiveRunID != nil && strings.TrimSpace(*conv.AIActiveRunID) != "" {
		chatRun, err = s.runRepo.GetByID(ctx, plan.WorkspaceID, *conv.AIActiveRunID)
		if err != nil {
			return err
		}
	}
	if chatRun == nil || !model.IsAgentRunActiveStatus(chatRun.Status) {
		return s.deliverPlanResultViaSuccessor(ctx, conv, chatRun, plan)
	}
	if chatRun.Status != model.AgentRunStatusPaused || chatRun.PauseReason != model.AgentRunPauseReasonUserMessage {
		// Mid-turn or paused on an interaction: the sweep retries.
		return nil
	}
	if chatRun.ExternalRuntimeID == nil || strings.TrimSpace(*chatRun.ExternalRuntimeID) == "" {
		return nil
	}
	runtimeClient := s.agentService.agentRuntimeClient
	if runtimeClient == nil {
		return nil
	}

	block, err := buildChildRunResultBlockWith(ctx, plan, s.runRepo, s.agentService.runMessageRepo, s.agentService)
	if err != nil {
		return err
	}
	resumeID := supportChildResultResumePrefix + plan.ID
	if _, err := runtimeClient.ResumeRun(ctx, strings.TrimSpace(*chatRun.ExternalRuntimeID), AgentRuntimeResumeRunRequest{
		Intent:   "reply",
		Content:  block,
		ResumeID: resumeID,
	}); err != nil {
		if isChatRunExpiredError(err) {
			return s.deliverPlanResultViaSuccessor(ctx, conv, chatRun, plan)
		}
		// "run is not paused" and similar races resolve on the next sweep.
		return err
	}
	recordChildResultRunMessage(ctx, s.agentService.runMessageRepo, chatRun, block, resumeID)
	chatRun.Status = model.AgentRunStatusRunning
	chatRun.PauseReason = model.AgentRunPauseReasonNone
	if err := s.runRepo.Update(ctx, chatRun); err != nil {
		slog.WarnContext(ctx, "support chat result: mark chat run running failed",
			"workspace_id", plan.WorkspaceID, "run_id", chatRun.ID, "error", err)
	}
	s.supportAIService.publishTypingIndicator(ctx, plan.WorkspaceID, conversationID, true)
	return s.planRepo.MarkParentNotified(ctx, plan.WorkspaceID, plan.ID)
}

// deliverPlanResultViaSuccessor starts a successor chat run carrying the child
// result — the visitor asked a question whose answer depended on the child, so
// delivery must not wait for their next message.
func (s *SupportChatService) deliverPlanResultViaSuccessor(ctx context.Context, conv *model.SupportConversation, previousRun *model.AgentRun, plan *model.CommandBarPlanRecord) error {
	settings, err := s.supportAIService.loadSettings(ctx, conv.WorkspaceID)
	if err != nil {
		return err
	}
	agentID := strings.TrimSpace(derefString(settings.AIAgentID))
	if agentID == "" || !shouldAutomaticallyProcessSupportAI(*settings) {
		return s.planRepo.MarkParentNotified(ctx, conv.WorkspaceID, plan.ID)
	}
	agent, err := s.agentService.GetAgent(ctx, conv.WorkspaceID, agentID)
	if err != nil || agent == nil {
		return err
	}
	block, err := buildChildRunResultBlockWith(ctx, plan, s.runRepo, s.agentService.runMessageRepo, s.agentService)
	if err != nil {
		return err
	}
	if err := s.startSupportChatRun(ctx, conv, agent, block, previousRun); err != nil {
		return err
	}
	s.supportAIService.publishTypingIndicator(ctx, conv.WorkspaceID, conv.ID, true)
	return s.planRepo.MarkParentNotified(ctx, conv.WorkspaceID, plan.ID)
}

// supportPlanDeliveryBlocked reports whether the conversation has left AI
// handling, making child-result delivery to the agent moot.
func supportPlanDeliveryBlocked(conv *model.SupportConversation) bool {
	if conv.CustomerRequestedHumanAt != nil || (conv.HumanTakeover != nil && *conv.HumanTakeover) {
		return true
	}
	return derefString(conv.AIState) == "escalated"
}
