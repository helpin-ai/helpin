package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// Per-finalizer idempotency markers stored in agent_runs.output_summary. They
// survive process crashes between finalizers so a redelivered terminal event
// (which re-enters dispatch only while the local run row is still
// non-terminal) never repeats a side effect that already happened.
const (
	agentRuntimeFinalizerAgentIdleSummaryKey          = "agent_runtime_finalizer_agent_idle"
	agentRuntimeFinalizerAutomationRulesSummaryKey    = "agent_runtime_finalizer_automation_rules"
	agentRuntimeFinalizerPlanningSummaryKey           = "agent_runtime_finalizer_planning"
	agentRuntimeFinalizerRepositoryDeliverySummaryKey = "agent_runtime_finalizer_repository_delivery"
	agentRuntimeFinalizerCommandBarPlanSummaryKey     = "agent_runtime_finalizer_command_bar_plan"
	agentRuntimeSupportSentMessageIDSummaryKey        = "sent_message_id"
)

type agentRunFinalizerAgentRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (*model.Agent, error)
	Update(ctx context.Context, agent *model.Agent) error
}

type agentRunFinalizerTaskRepository interface {
	GetRawByID(ctx context.Context, id string) (*model.PMTask, error)
}

type agentRunFinalizerEpicRepository interface {
	GetByID(ctx context.Context, id string) (*model.EpicWithStats, error)
	Update(ctx context.Context, epic *model.PMEpic) error
}

type agentRunFinalizerConversationRepository interface {
	GetByID(ctx context.Context, workspaceID, id, workspaceMemberID, role string) (*model.SupportConversation, error)
	ListByAnonymousID(ctx context.Context, workspaceID, anonymousID string) ([]model.SupportConversation, error)
}

type agentRunFinalizerSupportMessageRepository interface {
	GetByID(ctx context.Context, id string) (*model.SupportMessage, error)
	Create(ctx context.Context, message *model.SupportMessage) error
}

type agentRunFinalizerRuleEvaluator interface {
	EvaluateEvent(ctx context.Context, event model.AutomationEvent, execCtx *model.RuleExecutionContext)
}

type agentRunFinalizerRepositoryDeliveryService interface {
	FinalizeDelegatedRunDelivery(ctx context.Context, run *model.AgentRun, delivery AgentRunRepositoryDelivery) (*AgentRunRepositoryDeliveryResult, error)
}

type agentRunFinalizerCommandBarPlanAdvancer interface {
	AdvanceCommandBarPlanForDelegatedRun(ctx context.Context, run *model.AgentRun) (*model.AgentRun, error)
}

// AgentRunFinalizerService fires the product side effects that Temporal
// activities apply when a run reaches a terminal state, for delegated
// agent-runtime runs projected back via NATS. Every finalizer is individually
// idempotent (marker in output_summary or natural create-if-not-exists) and a
// failing finalizer never blocks the others or the status projection.
type AgentRunFinalizerService struct {
	runRepo            agentRuntimeProjectionRunRepository
	agentRepo          agentRunFinalizerAgentRepository
	taskRepo           agentRunFinalizerTaskRepository
	epicRepo           agentRunFinalizerEpicRepository
	conversationRepo   agentRunFinalizerConversationRepository
	supportMessageRepo agentRunFinalizerSupportMessageRepository
	ruleEngine         agentRunFinalizerRuleEvaluator
	repositoryDelivery agentRunFinalizerRepositoryDeliveryService
	commandBarAdvancer agentRunFinalizerCommandBarPlanAdvancer
	dockChatNotifier   agentRunFinalizerDockChatNotifier
	wsPublisher        websocket.EventPublisher
}

// NewAgentRunFinalizerService wires the finalizer set from the repositories
// and services available in the projection worker (cmd/temporal-worker).
func NewAgentRunFinalizerService(
	runRepo *repository.AgentRunRepository,
	agentRepo *repository.AgentRepository,
	taskRepo *repository.PMTaskRepository,
	epicRepo *repository.PMEpicRepository,
	conversationRepo *repository.SupportConversationRepository,
	supportMessageRepo *repository.SupportMessageRepository,
	ruleEngine *AutomationRuleEngine,
	wsPublisher websocket.EventPublisher,
) *AgentRunFinalizerService {
	svc := &AgentRunFinalizerService{wsPublisher: wsPublisher}
	if runRepo != nil {
		svc.runRepo = runRepo
	}
	if agentRepo != nil {
		svc.agentRepo = agentRepo
	}
	if taskRepo != nil {
		svc.taskRepo = taskRepo
	}
	if epicRepo != nil {
		svc.epicRepo = epicRepo
	}
	if conversationRepo != nil {
		svc.conversationRepo = conversationRepo
	}
	if supportMessageRepo != nil {
		svc.supportMessageRepo = supportMessageRepo
	}
	if ruleEngine != nil {
		svc.ruleEngine = ruleEngine
	}
	return svc
}

// SetRepositoryDeliveryService wires the git service used by the repository
// delivery finalizer to open PRs/MRs for runtime-pushed work branches.
func (s *AgentRunFinalizerService) SetRepositoryDeliveryService(gitService *GitService) *AgentRunFinalizerService {
	if s == nil {
		return s
	}
	if gitService != nil {
		s.repositoryDelivery = gitService
	}
	return s
}

// SetCommandBarPlanAdvancer wires the agent service used by the command-bar
// plan finalizer to advance parent plans when a delegated child run reaches a
// terminal state (the role AdvanceCommandBarPlanActivity plays for
// Temporal-executed runs).
func (s *AgentRunFinalizerService) SetCommandBarPlanAdvancer(advancer agentRunFinalizerCommandBarPlanAdvancer) *AgentRunFinalizerService {
	if s == nil {
		return s
	}
	if advancer != nil {
		s.commandBarAdvancer = advancer
	}
	return s
}

// agentRunFinalizerDockChatNotifier delivers a settled dock-launched plan's
// result back into its parent dock chat run. *DockChatService satisfies it.
type agentRunFinalizerDockChatNotifier interface {
	NotifyPlanSettledForRun(ctx context.Context, run *model.AgentRun) error
}

// SetDockChatResultNotifier wires the dock chat service used to deliver
// settled child-plan results into the launching chat immediately after plan
// advancement (the periodic sweep remains the backstop).
func (s *AgentRunFinalizerService) SetDockChatResultNotifier(notifier agentRunFinalizerDockChatNotifier) *AgentRunFinalizerService {
	if s == nil {
		return s
	}
	if notifier != nil {
		s.dockChatNotifier = notifier
	}
	return s
}

// FinalizeTerminalRun applies product side effects for a delegated run that
// just transitioned into a terminal status (the caller threads the
// transitioned signal; this must never be invoked for redelivered terminal
// events on an already-terminal run). runtimeSummaryAvailable reports whether
// the runtime run OutputSummary was merged into run.OutputSummary; finalizers
// that read the summary contract are skipped without markers when it is false
// so a later duplicate terminal event can retry them.
func (s *AgentRunFinalizerService) FinalizeTerminalRun(ctx context.Context, run *model.AgentRun, runtimeSummaryAvailable bool) {
	if s == nil || run == nil {
		return
	}
	completed := strings.TrimSpace(run.Status) == model.AgentRunStatusCompleted
	finalizers := []struct {
		name          string
		completedOnly bool
		needsSummary  bool
		run           func(context.Context, *model.AgentRun) error
	}{
		{name: "agent_status", run: s.finalizeAgentStatus},
		{name: "automation_rules", completedOnly: true, run: s.finalizeRunCompletedRules},
		{name: "support_draft", completedOnly: true, needsSummary: true, run: s.finalizeSupportDraft},
		{name: "planning_output", completedOnly: true, run: func(ctx context.Context, run *model.AgentRun) error {
			return s.finalizePlanningOutput(ctx, run, runtimeSummaryAvailable)
		}},
		{name: "repository_delivery", completedOnly: true, needsSummary: true, run: s.finalizeRepositoryDelivery},
		// command_bar_plan runs last so repository delivery (PR bookkeeping a
		// downstream merge step may depend on) lands before the plan advances.
		// It fires on every terminal status so both completion and failure
		// advance the plan.
		{name: "command_bar_plan", run: s.finalizeCommandBarPlan},
	}
	for _, finalizer := range finalizers {
		if finalizer.completedOnly && !completed {
			continue
		}
		if finalizer.needsSummary && !runtimeSummaryAvailable {
			slog.ErrorContext(ctx, "agent runtime finalizer skipped without runtime output summary",
				"finalizer", finalizer.name,
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
				"status", run.Status,
			)
			continue
		}
		if err := finalizer.run(ctx, run); err != nil {
			slog.ErrorContext(ctx, "agent runtime run finalizer failed",
				"finalizer", finalizer.name,
				"error", err,
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
				"agent_id", run.AgentID,
				"status", run.Status,
			)
		}
	}
}

// finalizeAgentStatus mirrors temporalapp markAgentIdle: flip the agent back
// to idle and add the run tokens to the agent monthly totals. Marker-guarded
// because the token increment is not naturally idempotent and a late replay
// must never flip an agent that has since started another run.
func (s *AgentRunFinalizerService) finalizeAgentStatus(ctx context.Context, run *model.AgentRun) error {
	if s.agentRepo == nil {
		return nil
	}
	if runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerAgentIdleSummaryKey) {
		return nil
	}
	agent, err := s.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil {
		return fmt.Errorf("load agent %q: %w", run.AgentID, err)
	}
	if agent != nil {
		agent.Status = "idle"
		agent.ActiveTaskID = nil
		agent.TokensUsedThisMonth += run.TokensUsed
		if err := s.agentRepo.Update(ctx, agent); err != nil {
			return fmt.Errorf("mark agent %q idle: %w", run.AgentID, err)
		}
	}
	return s.markRunOutputSummaryFlag(ctx, run, agentRuntimeFinalizerAgentIdleSummaryKey)
}

// finalizeRunCompletedRules mirrors temporalapp evaluateRunCompletedRules.
// The marker is written before the evaluation (at-most-once): automation
// actions are user-visible and a replay must never double-fire them.
func (s *AgentRunFinalizerService) finalizeRunCompletedRules(ctx context.Context, run *model.AgentRun) error {
	if s.ruleEngine == nil || s.taskRepo == nil {
		return nil
	}
	if run.TaskID == nil || (run.TargetType != "task" && run.TargetType != "story") {
		return nil
	}
	if runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerAutomationRulesSummaryKey) {
		return nil
	}
	task, err := s.taskRepo.GetRawByID(ctx, *run.TaskID)
	if err != nil {
		return fmt.Errorf("load task %q for agent_run.completed automation: %w", derefString(run.TaskID), err)
	}
	if task == nil {
		return s.markRunOutputSummaryFlag(ctx, run, agentRuntimeFinalizerAutomationRulesSummaryKey)
	}
	if err := s.markRunOutputSummaryFlag(ctx, run, agentRuntimeFinalizerAutomationRulesSummaryKey); err != nil {
		return err
	}
	s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
		WorkspaceID: run.WorkspaceID,
		TriggerType: model.TriggerAgentRunCompleted,
		TaskID:      task.ID,
		StoryID:     task.ID,
		StateID:     task.WorkflowStateID,
		AgentID:     run.AgentID,
		RunID:       run.ID,
		TargetType:  "task",
		TargetID:    task.ID,
	}, nil)
	return nil
}

// finalizeSupportDraft mirrors temporalapp finalizeSupportConversationRun:
// turn output_summary.draft_reply into a SupportMessage, publish the realtime
// event and refresh visitor conversation lists. Naturally idempotent: the
// reply message ID equals the run ID (create-if-not-exists) and the recorded
// sent_message_id short-circuits replays.
func (s *AgentRunFinalizerService) finalizeSupportDraft(ctx context.Context, run *model.AgentRun) error {
	if s.supportMessageRepo == nil || s.conversationRepo == nil || s.runRepo == nil {
		return nil
	}
	if run.TargetType != "support_conversation" {
		return nil
	}
	if run.ApprovalState == "pending" {
		return nil
	}
	if len(run.OutputSummary) == 0 {
		return nil
	}
	var summary struct {
		DraftReply    *supportDraftReply `json:"draft_reply"`
		SentMessageID *string            `json:"sent_message_id"`
	}
	if err := json.Unmarshal(run.OutputSummary, &summary); err != nil {
		return fmt.Errorf("parse support run summary: %w", err)
	}
	if summary.SentMessageID != nil && strings.TrimSpace(*summary.SentMessageID) != "" {
		return nil
	}
	if summary.DraftReply == nil || strings.TrimSpace(summary.DraftReply.Content) == "" {
		return nil
	}

	conversationID := strings.TrimSpace(derefString(run.ConversationID))
	if conversationID == "" {
		conversationID = strings.TrimSpace(run.TargetID)
	}
	if conversationID == "" {
		return nil
	}
	conversation, err := s.conversationRepo.GetByID(ctx, run.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return fmt.Errorf("load support conversation %q: %w", conversationID, err)
	}
	if conversation == nil {
		return fmt.Errorf("support conversation %q not found", conversationID)
	}

	messageID := run.ID
	existing, err := s.supportMessageRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("lookup support reply message: %w", err)
	}
	createdMessage := false
	if existing == nil {
		createdMessage = true
		existing = &model.SupportMessage{
			ID:                messageID,
			WorkspaceID:       run.WorkspaceID,
			ConversationID:    conversation.ID,
			SenderType:        "agent",
			SenderAgentID:     &run.AgentID,
			SenderDisplayName: summary.DraftReply.SenderDisplayName,
			Content:           strings.TrimSpace(summary.DraftReply.Content),
			IsInternal:        summary.DraftReply.IsInternal,
			MessageType:       "reply",
		}
		if err := s.supportMessageRepo.Create(ctx, existing); err != nil {
			return fmt.Errorf("create support reply message: %w", err)
		}
	}

	if err := s.setRunOutputSummaryValue(ctx, run, agentRuntimeSupportSentMessageIDSummaryKey, existing.ID); err != nil {
		return fmt.Errorf("persist support run summary: %w", err)
	}

	if s.wsPublisher != nil {
		event := websocket.SupportMessageEvent(run.WorkspaceID, existing, "")
		if !createdMessage {
			event.Data = nil
		}
		s.wsPublisher.Publish(event)
	}
	s.pushVisitorConversationRefresh(ctx, run.WorkspaceID, conversation)
	return nil
}

// finalizePlanningOutput is the scoped port of temporalapp finalizePlanningRun
// / finalizeFlowOutputRun that is safe from projection context: flow-output
// summary validation and the epic planner pointer. Deep temporalapp machinery
// (approved-preview application, planning repository prep, completion
// interaction policy, failing the run on invalid output) is intentionally
// deferred — see docs/plans/2026-07-02-delegated-run-finalizers.md.
func (s *AgentRunFinalizerService) finalizePlanningOutput(ctx context.Context, run *model.AgentRun, runtimeSummaryAvailable bool) error {
	if s.runRepo == nil {
		return nil
	}
	if runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerPlanningSummaryKey) {
		return nil
	}
	var input struct {
		FlowOutputKind string `json:"flow_output_kind"`
	}
	if len(run.Input) > 0 {
		_ = json.Unmarshal(run.Input, &input)
	}
	if kind := strings.TrimSpace(input.FlowOutputKind); kind != "" {
		if !runtimeSummaryAvailable {
			return fmt.Errorf("flow output %q cannot be validated without the runtime output summary", kind)
		}
		if err := validateDelegatedFlowOutput(kind, run.OutputSummary); err != nil {
			// Documented gap: the projection cannot fail a runtime-owned
			// run, so invalid flow output is surfaced via ERROR log only.
			slog.ErrorContext(ctx, "delegated flow output validation failed",
				"error", err,
				"flow_output_kind", kind,
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
			)
		}
		return s.markRunOutputSummaryFlag(ctx, run, agentRuntimeFinalizerPlanningSummaryKey)
	}
	if run.TargetType != "epic" || s.epicRepo == nil {
		return nil
	}
	epicWithStats, err := s.epicRepo.GetByID(ctx, run.TargetID)
	if err != nil {
		return fmt.Errorf("load epic %q: %w", run.TargetID, err)
	}
	if epicWithStats != nil {
		epic := epicWithStats.Epic
		epic.LastPlanningRunID = &run.ID
		if err := s.epicRepo.Update(ctx, &epic); err != nil {
			return fmt.Errorf("update epic %q planning pointer: %w", run.TargetID, err)
		}
	}
	return s.markRunOutputSummaryFlag(ctx, run, agentRuntimeFinalizerPlanningSummaryKey)
}

// finalizeRepositoryDelivery mirrors temporalapp recordPushAndEnsureDeliveryPR
// for delegated runs on repository-backed targets: when the runtime reports a
// pushed work branch in the merged OutputSummary ({"repository": {"pushed":
// true, ...}}), ensure the PR/MR and record it on the delivery target and git
// link. Idempotency is natural — the provider lookup reuses an existing open
// PR for the branch pair — with a marker on top so crash replays skip the
// bookkeeping writes and activity log once delivery fully succeeded.
func (s *AgentRunFinalizerService) finalizeRepositoryDelivery(ctx context.Context, run *model.AgentRun) error {
	if s.repositoryDelivery == nil {
		return nil
	}
	switch run.TargetType {
	case "repository", "task", "story", "epic":
	default:
		return nil
	}
	repository, ok := delegatedRunRepositorySummary(run.OutputSummary)
	if !ok || !repository.Pushed {
		// Nothing pushed (or no repository contract at all): quietly skip
		// without a marker so nothing here ever blocks non-repo runs.
		return nil
	}
	if runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerRepositoryDeliverySummaryKey) {
		return nil
	}
	result, err := s.repositoryDelivery.FinalizeDelegatedRunDelivery(ctx, run, AgentRunRepositoryDelivery{
		Branch:    strings.TrimSpace(repository.Branch),
		CommitSHA: strings.TrimSpace(repository.Commit),
	})
	if err != nil {
		return fmt.Errorf("deliver pushed branch %q: %w", strings.TrimSpace(repository.Branch), err)
	}
	if err := s.markRunOutputSummaryFlag(ctx, run, agentRuntimeFinalizerRepositoryDeliverySummaryKey); err != nil {
		return err
	}
	if result != nil {
		slog.InfoContext(ctx, "delegated run repository delivery finalized",
			"workspace_id", run.WorkspaceID,
			"run_id", run.ID,
			"target_type", run.TargetType,
			"target_id", run.TargetID,
			"provider", result.Provider,
			"pr_number", result.Number,
			"pr_url", result.URL,
		)
	}
	return nil
}

// finalizeCommandBarPlan mirrors temporalapp advanceCommandBarPlan for
// delegated runs: when a command-bar plan child reaches a terminal state,
// advance the owning plan (signal the DAG/pipeline workflow, start the next
// linear step, or settle fan-out/plan status). The in-memory run is passed
// through because its terminal status is not yet persisted at dispatch time.
// The marker is written after a successful advancement (at-least-once): the
// advancement itself is naturally idempotent (existing-child lookups, plan
// status checks, and re-signaling a workflow are all safe to repeat).
func (s *AgentRunFinalizerService) finalizeCommandBarPlan(ctx context.Context, run *model.AgentRun) error {
	if s.commandBarAdvancer == nil {
		return nil
	}
	if _, ok := commandBarRunPayload(run); !ok {
		// Not a command-bar child: skip quietly without a marker.
		return nil
	}
	if runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerCommandBarPlanSummaryKey) {
		return nil
	}
	if _, err := s.commandBarAdvancer.AdvanceCommandBarPlanForDelegatedRun(ctx, run); err != nil {
		return fmt.Errorf("advance command bar plan after run %q: %w", run.ID, err)
	}
	if s.dockChatNotifier != nil {
		// Delivery is idempotent (parent_notified_at + runtime resume_id);
		// failures are retried by the dock chat result sweep.
		if err := s.dockChatNotifier.NotifyPlanSettledForRun(ctx, run); err != nil {
			slog.WarnContext(ctx, "dock chat result delivery after plan advance failed",
				"workspace_id", run.WorkspaceID, "run_id", run.ID, "error", err)
		}
	}
	return s.markRunOutputSummaryFlag(ctx, run, agentRuntimeFinalizerCommandBarPlanSummaryKey)
}

// delegatedRunRepositorySummary parses the runtime repository contract from
// the merged run output summary (see agent-runtime
// commitAndPushRepositoryChanges for the producer shape).
func delegatedRunRepositorySummary(summary json.RawMessage) (agentRuntimeRepositorySummary, bool) {
	if len(summary) == 0 {
		return agentRuntimeRepositorySummary{}, false
	}
	var body struct {
		Repository *agentRuntimeRepositorySummary `json:"repository"`
	}
	if err := json.Unmarshal(summary, &body); err != nil || body.Repository == nil {
		return agentRuntimeRepositorySummary{}, false
	}
	return *body.Repository, true
}

// agentRuntimeRepositorySummary is the "repository" object the agent runtime
// writes into a delegated run's OutputSummary under the push_branch finalize
// policy.
type agentRuntimeRepositorySummary struct {
	Changed bool   `json:"changed"`
	Pushed  bool   `json:"pushed"`
	Branch  string `json:"branch"`
	Commit  string `json:"commit"`
}

func validateDelegatedFlowOutput(kind string, summary json.RawMessage) error {
	switch kind {
	case "pm.task_completion_followups":
		var assessment model.TaskCompletionAssessment
		if err := json.Unmarshal(summary, &assessment); err != nil {
			return fmt.Errorf("decode task completion assessment: %w", err)
		}
		if strings.TrimSpace(assessment.Summary) == "" {
			return fmt.Errorf("task completion assessment is missing a summary")
		}
		return nil
	case "crm.deal_review_actions":
		var plan model.CRMDealReviewActionPlan
		if err := json.Unmarshal(summary, &plan); err != nil {
			return fmt.Errorf("decode CRM deal review plan: %w", err)
		}
		if strings.TrimSpace(plan.Summary) == "" {
			return fmt.Errorf("CRM deal review plan is missing a summary")
		}
		return nil
	default:
		return nil
	}
}

func (s *AgentRunFinalizerService) pushVisitorConversationRefresh(ctx context.Context, workspaceID string, conversation *model.SupportConversation) {
	if conversation == nil || conversation.AnonymousID == nil || strings.TrimSpace(*conversation.AnonymousID) == "" || s.wsPublisher == nil {
		return
	}
	conversations, err := s.conversationRepo.ListByAnonymousID(ctx, workspaceID, *conversation.AnonymousID)
	if err != nil {
		slog.WarnContext(ctx, "visitor conversation refresh list failed",
			"error", err,
			"workspace_id", workspaceID,
			"conversation_id", conversation.ID,
		)
		return
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	listJSON, err := json.Marshal(map[string]any{"conversations": conversations})
	if err != nil {
		return
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_visitor_conversations",
		EntityID:    *conversation.AnonymousID,
		WorkspaceID: workspaceID,
		Data:        listJSON,
	})
}

// markRunOutputSummaryFlag merges a boolean marker into the run output
// summary (in memory and via the targeted UpdateOutputSummary repo method) so
// the marker survives a crash before the projection's final row update
// without ever touching projection-owned status fields.
func (s *AgentRunFinalizerService) markRunOutputSummaryFlag(ctx context.Context, run *model.AgentRun, key string) error {
	return s.setRunOutputSummaryValue(ctx, run, key, true)
}

func (s *AgentRunFinalizerService) setRunOutputSummaryValue(ctx context.Context, run *model.AgentRun, key string, value any) error {
	if s.runRepo == nil || run == nil {
		return nil
	}
	body := map[string]any{}
	if len(run.OutputSummary) > 0 {
		_ = json.Unmarshal(run.OutputSummary, &body)
	}
	body[key] = value
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal run output summary: %w", err)
	}
	run.OutputSummary = payload
	if err := s.runRepo.UpdateOutputSummary(ctx, run.ID, payload); err != nil {
		return fmt.Errorf("persist run output summary marker %q: %w", key, err)
	}
	return nil
}

func runOutputSummaryFlag(summary json.RawMessage, key string) bool {
	if len(summary) == 0 {
		return false
	}
	var body map[string]any
	if err := json.Unmarshal(summary, &body); err != nil {
		return false
	}
	value, _ := body[key].(bool)
	return value
}
