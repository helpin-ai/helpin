package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	dockChildResultOpenTag      = "<child_run_result>"
	dockChildResultCloseTag     = "</child_run_result>"
	dockChildResultResumePrefix = "dock-child:"
	dockChildResultSummaryChars = 800
)

// dockChildRunResult is the payload delivered into a dock chat when a plan
// launched from that chat settles. The FE renders it as a result chip; the
// ask_agent prompt tells the model to summarize it for the user.
type dockChildRunResult struct {
	PlanID string               `json:"plan_id"`
	Status string               `json:"status"`
	Prompt string               `json:"prompt,omitempty"`
	Error  string               `json:"error,omitempty"`
	Runs   []dockChildRunReport `json:"runs"`
}

type dockChildRunReport struct {
	RunID     string `json:"run_id"`
	AgentName string `json:"agent_name,omitempty"`
	Status    string `json:"status"`
	Summary   string `json:"summary,omitempty"`
}

// NotifyPlanSettledForRun is the immediate delivery hook: called by the
// agent-runtime terminal finalizer for command-bar child runs, it resolves the
// run's plan and, when settled, delivers the result into the parent dock chat.
func (s *DockChatService) NotifyPlanSettledForRun(ctx context.Context, run *model.AgentRun) error {
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
	return s.notifyPlanSettled(ctx, plan)
}

// SweepUnnotifiedDockChatResults is the delivery backstop: it retries settled
// dock plans whose parent chat was busy (mid-turn) at finalize time.
func (s *DockChatService) SweepUnnotifiedDockChatResults(ctx context.Context, limit int) error {
	if s == nil || s.planRepo == nil {
		return nil
	}
	plans, err := s.planRepo.ListSettledUnnotifiedDockPlans(ctx, limit)
	if err != nil {
		return err
	}
	for i := range plans {
		if err := s.notifyPlanSettled(ctx, &plans[i]); err != nil {
			slog.WarnContext(ctx, "dock chat result delivery failed",
				"workspace_id", plans[i].WorkspaceID, "plan_id", plans[i].ID, "error", err)
		}
	}
	return nil
}

// StartDockChatResultSweep runs SweepUnnotifiedDockChatResults on a ticker
// until the context is cancelled. Wired next to the command-bar plan sweep.
func (s *DockChatService) StartDockChatResultSweep(ctx context.Context, interval time.Duration, limit int) error {
	if s == nil || s.planRepo == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "dock chat result sweep panic", "panic", recovered)
		}
	}()
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if limit <= 0 {
		limit = 50
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		if err := s.SweepUnnotifiedDockChatResults(ctx, limit); err != nil {
			slog.ErrorContext(ctx, "dock chat result sweep failed", "error", err)
		}
	}
}

// notifyPlanSettled delivers one settled plan's result into its dock chat by
// resuming the chat's active backing run with a <child_run_result> block. The
// delivery is idempotent (runtime resume_id dedupe + parent_notified_at).
// Delivery only happens while the chat run is paused awaiting the user; a
// busy chat is retried by the sweep, and a chat whose run ended receives the
// result through the successor run's carry-forward context.
func (s *DockChatService) notifyPlanSettled(ctx context.Context, plan *model.CommandBarPlanRecord) error {
	if plan == nil || plan.ParentChatRunID == nil || plan.ParentNotifiedAt != nil {
		return nil
	}
	if !isDockPlanTerminalStatus(plan.Status) {
		return nil
	}
	if plan.DockChatID == nil {
		return s.planRepo.MarkParentNotified(ctx, plan.WorkspaceID, plan.ID)
	}
	chat, err := s.chatRepo.GetByID(ctx, plan.WorkspaceID, *plan.DockChatID)
	if err != nil {
		return err
	}
	if chat == nil || chat.ActiveRunID == nil {
		// Orphaned plan or a chat with no backing run yet: nothing to resume.
		// A chat without an active run receives the result via carry-forward.
		if chat == nil {
			return s.planRepo.MarkParentNotified(ctx, plan.WorkspaceID, plan.ID)
		}
		return nil
	}
	chatRun, err := s.runRepo.GetByID(ctx, plan.WorkspaceID, *chat.ActiveRunID)
	if err != nil || chatRun == nil {
		return err
	}
	if chatRun.Status != model.AgentRunStatusPaused || chatRun.PauseReason != model.AgentRunPauseReasonUserMessage {
		// Mid-turn, paused on an interaction, or terminal: the sweep retries,
		// and terminal chats get the result via successor carry-forward.
		return nil
	}
	if chatRun.ExternalRuntimeID == nil || strings.TrimSpace(*chatRun.ExternalRuntimeID) == "" {
		return nil
	}
	runtimeClient := s.agentService.agentRuntimeClient
	if runtimeClient == nil {
		return nil
	}

	block, err := s.buildChildRunResultBlock(ctx, plan)
	if err != nil {
		return err
	}
	resumeID := dockChildResultResumePrefix + plan.ID
	if _, err := runtimeClient.ResumeRun(ctx, strings.TrimSpace(*chatRun.ExternalRuntimeID), AgentRuntimeResumeRunRequest{
		Intent:   "reply",
		Content:  block,
		ResumeID: resumeID,
	}); err != nil {
		// "run is not paused" and similar races resolve on the next sweep.
		return err
	}
	// Mirror the injected message locally so the dock transcript shows the
	// result chip (runtime-side user messages are not projected back).
	s.recordChildResultMessage(ctx, chatRun, block, resumeID)
	chatRun.Status = model.AgentRunStatusRunning
	chatRun.PauseReason = model.AgentRunPauseReasonNone
	if err := s.runRepo.Update(ctx, chatRun); err != nil {
		slog.WarnContext(ctx, "dock chat result: mark chat run running failed",
			"workspace_id", plan.WorkspaceID, "run_id", chatRun.ID, "error", err)
	}
	return s.planRepo.MarkParentNotified(ctx, plan.WorkspaceID, plan.ID)
}

// unnotifiedPlanResultBlocks renders undelivered settled plan results for a
// chat (used in successor-run carry-forward) and returns the plan IDs so the
// caller can mark them notified once the successor run is started.
func (s *DockChatService) unnotifiedPlanResultBlocks(ctx context.Context, chat *model.DockChat) ([]string, []string) {
	if s == nil || s.planRepo == nil || chat == nil {
		return nil, nil
	}
	plans, err := s.planRepo.ListSettledUnnotifiedDockPlans(ctx, 200)
	if err != nil {
		return nil, nil
	}
	var blocks []string
	var planIDs []string
	for i := range plans {
		plan := &plans[i]
		if plan.DockChatID == nil || *plan.DockChatID != chat.ID {
			continue
		}
		block, err := s.buildChildRunResultBlock(ctx, plan)
		if err != nil {
			continue
		}
		blocks = append(blocks, block)
		planIDs = append(planIDs, plan.ID)
	}
	return blocks, planIDs
}

func (s *DockChatService) buildChildRunResultBlock(ctx context.Context, plan *model.CommandBarPlanRecord) (string, error) {
	result := dockChildRunResult{
		PlanID: plan.ID,
		Status: strings.TrimSpace(plan.Status),
		Prompt: strings.TrimSpace(plan.Prompt),
		Error:  strings.TrimSpace(derefString(plan.ErrorMessage)),
		Runs:   []dockChildRunReport{},
	}

	var steps []model.CommandBarPlanStep
	_ = json.Unmarshal(plan.Steps, &steps)
	runIDsByStep := map[string]string{}
	_ = json.Unmarshal(plan.RunIDsByStep, &runIDsByStep)

	for stepKey, runID := range runIDsByStep {
		report := dockChildRunReport{RunID: runID}
		if index, err := strconv.Atoi(stepKey); err == nil && index >= 0 && index < len(steps) {
			report.AgentName = strings.TrimSpace(steps[index].AgentName)
		}
		run, err := s.runRepo.GetByID(ctx, plan.WorkspaceID, runID)
		if err == nil && run != nil {
			report.Status = strings.TrimSpace(run.Status)
			report.Summary = s.childRunSummary(ctx, run)
		}
		result.Runs = append(result.Runs, report)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return dockChildResultOpenTag + string(encoded) + dockChildResultCloseTag, nil
}

// childRunSummary returns the child run's final assistant message, truncated.
func (s *DockChatService) childRunSummary(ctx context.Context, run *model.AgentRun) string {
	messages, err := s.runMessageRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(messages[i].Content)
		if content == "" {
			continue
		}
		if len(content) > dockChildResultSummaryChars {
			content = content[:dockChildResultSummaryChars] + "…"
		}
		return content
	}
	return ""
}

func (s *DockChatService) recordChildResultMessage(ctx context.Context, chatRun *model.AgentRun, content, resumeID string) {
	sequence, err := s.runMessageRepo.NextSequence(ctx, chatRun.WorkspaceID, chatRun.ID)
	if err != nil {
		slog.WarnContext(ctx, "dock chat result: next sequence failed",
			"workspace_id", chatRun.WorkspaceID, "run_id", chatRun.ID, "error", err)
		return
	}
	message := &model.AgentRunMessage{
		WorkspaceID:      chatRun.WorkspaceID,
		RunID:            chatRun.ID,
		RuntimeMessageID: resumeID,
		Role:             "user",
		Content:          content,
		MessageType:      "message",
		SequenceNo:       sequence,
	}
	if err := s.runMessageRepo.Create(ctx, message); err != nil {
		slog.WarnContext(ctx, "dock chat result: record message failed",
			"workspace_id", chatRun.WorkspaceID, "run_id", chatRun.ID, "error", err)
	}
}

func isDockPlanTerminalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case model.CommandBarPlanStatusCompleted, model.CommandBarPlanStatusFailed, model.CommandBarPlanStatusCancelled:
		return true
	default:
		return false
	}
}
