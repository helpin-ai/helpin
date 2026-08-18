package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type fakeFinalizerAgentRepo struct {
	agent    *model.Agent
	getErr   error
	getCalls int
	updates  int
}

type fakeFinalizerSupportCoverage struct {
	calls []supportCoverageGapOutcomeSummary
	gapID string
}

func (f *fakeFinalizerSupportCoverage) RecordAgentOutcome(_ context.Context, _, gapID, outcome, documentID string) error {
	f.gapID = gapID
	f.calls = append(f.calls, supportCoverageGapOutcomeSummary{Outcome: outcome, DocumentID: documentID})
	return nil
}

func (r *fakeFinalizerAgentRepo) GetByID(_ context.Context, _, _ string) (*model.Agent, error) {
	r.getCalls++
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.agent, nil
}

func (r *fakeFinalizerAgentRepo) Update(_ context.Context, agent *model.Agent) error {
	r.updates++
	r.agent = agent
	return nil
}

type fakeFinalizerTaskRepo struct {
	task     *model.PMTask
	getCalls int
}

func (r *fakeFinalizerTaskRepo) GetRawByID(_ context.Context, _ string) (*model.PMTask, error) {
	r.getCalls++
	return r.task, nil
}

type fakeFinalizerEpicRepo struct {
	epic     *model.EpicWithStats
	getCalls int
	updates  []model.PMEpic
}

func (r *fakeFinalizerEpicRepo) GetByID(_ context.Context, _ string) (*model.EpicWithStats, error) {
	r.getCalls++
	return r.epic, nil
}

func (r *fakeFinalizerEpicRepo) Update(_ context.Context, epic *model.PMEpic) error {
	r.updates = append(r.updates, *epic)
	return nil
}

type fakeFinalizerConversationRepo struct {
	conversation *model.SupportConversation
	listCalls    int
}

func (r *fakeFinalizerConversationRepo) GetByID(_ context.Context, _, _, _, _ string) (*model.SupportConversation, error) {
	return r.conversation, nil
}

func (r *fakeFinalizerConversationRepo) ListByAnonymousID(_ context.Context, _, _ string) ([]model.SupportConversation, error) {
	r.listCalls++
	if r.conversation == nil {
		return nil, nil
	}
	return []model.SupportConversation{*r.conversation}, nil
}

type fakeFinalizerSupportMessageRepo struct {
	messages map[string]*model.SupportMessage
	creates  int
}

func (r *fakeFinalizerSupportMessageRepo) GetByID(_ context.Context, id string) (*model.SupportMessage, error) {
	if r.messages == nil {
		return nil, nil
	}
	return r.messages[id], nil
}

func (r *fakeFinalizerSupportMessageRepo) Create(_ context.Context, message *model.SupportMessage) error {
	r.creates++
	if r.messages == nil {
		r.messages = map[string]*model.SupportMessage{}
	}
	r.messages[message.ID] = message
	return nil
}

type fakeFinalizerRuleEvaluator struct {
	events []model.AutomationEvent
}

func (e *fakeFinalizerRuleEvaluator) EvaluateEvent(_ context.Context, event model.AutomationEvent, _ *model.RuleExecutionContext) {
	e.events = append(e.events, event)
}

type fakeFinalizerPublisher struct {
	events []websocket.Event
}

func (p *fakeFinalizerPublisher) Publish(event websocket.Event) {
	p.events = append(p.events, event)
}

func newDelegatedTerminalTestRun(id, runtimeID string) *model.AgentRun {
	return &model.AgentRun{
		ID:                id,
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		TargetType:        "task",
		TargetID:          "task-1",
		TaskID:            stringPointer("task-1"),
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer(runtimeID),
		TokensUsed:        40,
		OutputSummary:     json.RawMessage(`{}`),
	}
}

func TestAgentRuntimeFinalizersFireOnTerminalTransitionOnly(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-final", "run_runtime_final")
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_final": run},
	}
	agentRepo := &fakeFinalizerAgentRepo{agent: &model.Agent{ID: "agent-1", Status: "working", ActiveTaskID: stringPointer("task-1"), TokensUsedThisMonth: 10}}
	taskRepo := &fakeFinalizerTaskRepo{task: &model.PMTask{ID: "task-1", WorkflowStateID: "state-1"}}
	ruleEvaluator := &fakeFinalizerRuleEvaluator{}
	finalizers := &AgentRunFinalizerService{
		runRepo:    runRepo,
		agentRepo:  agentRepo,
		taskRepo:   taskRepo,
		ruleEngine: ruleEvaluator,
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:       runRepo,
		runFinalizers: finalizers,
		now:           time.Now,
	}
	event := AgentRuntimeEventEnvelope{
		RunID: "run_runtime_final",
		Type:  "run.completed",
	}

	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusCompleted {
		t.Fatalf("expected completed status, got %q", run.Status)
	}
	if agentRepo.updates != 1 || agentRepo.agent.Status != "idle" || agentRepo.agent.ActiveTaskID != nil {
		t.Fatalf("expected agent marked idle once, updates=%d agent=%#v", agentRepo.updates, agentRepo.agent)
	}
	if agentRepo.agent.TokensUsedThisMonth != 50 {
		t.Fatalf("expected run tokens added to monthly total, got %d", agentRepo.agent.TokensUsedThisMonth)
	}
	if len(ruleEvaluator.events) != 1 {
		t.Fatalf("expected one agent_run.completed evaluation, got %#v", ruleEvaluator.events)
	}
	ruleEvent := ruleEvaluator.events[0]
	if ruleEvent.TriggerType != model.TriggerAgentRunCompleted || ruleEvent.TaskID != "task-1" || ruleEvent.RunID != run.ID {
		t.Fatalf("unexpected automation event: %#v", ruleEvent)
	}
	if !runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerAgentIdleSummaryKey) ||
		!runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerAutomationRulesSummaryKey) {
		t.Fatalf("expected finalizer markers in output summary, got %s", string(run.OutputSummary))
	}

	// Redelivered terminal event on the already-terminal run must be a no-op.
	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent redelivery returned error: %v", err)
	}
	if agentRepo.updates != 1 || len(ruleEvaluator.events) != 1 {
		t.Fatalf("expected replay no-op, agent updates=%d rule events=%d", agentRepo.updates, len(ruleEvaluator.events))
	}
	if runRepo.updates != 1 {
		t.Fatalf("expected a single run update, got %d", runRepo.updates)
	}
}

func TestAgentRuntimeFinalizersMarkerSkipsAlreadyFiredFinalizerOnCrashReplay(t *testing.T) {
	// Simulates a crash after the agent-idle finalizer persisted its marker
	// but before the terminal status persisted: the redelivered event
	// re-enters dispatch (run still non-terminal) and must only fire the
	// remaining finalizers.
	run := newDelegatedTerminalTestRun("helpin-run-crash", "run_runtime_crash")
	run.OutputSummary = json.RawMessage(`{"` + agentRuntimeFinalizerAgentIdleSummaryKey + `":true}`)
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_crash": run},
	}
	agentRepo := &fakeFinalizerAgentRepo{agent: &model.Agent{ID: "agent-1", Status: "working"}}
	taskRepo := &fakeFinalizerTaskRepo{task: &model.PMTask{ID: "task-1", WorkflowStateID: "state-1"}}
	ruleEvaluator := &fakeFinalizerRuleEvaluator{}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		runFinalizers: &AgentRunFinalizerService{
			runRepo:    runRepo,
			agentRepo:  agentRepo,
			taskRepo:   taskRepo,
			ruleEngine: ruleEvaluator,
		},
		now: time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_crash",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if agentRepo.getCalls != 0 || agentRepo.updates != 0 || agentRepo.agent.Status != "working" {
		t.Fatalf("expected marker to skip agent finalizer, getCalls=%d updates=%d", agentRepo.getCalls, agentRepo.updates)
	}
	if len(ruleEvaluator.events) != 1 {
		t.Fatalf("expected pending automation finalizer to still fire, got %#v", ruleEvaluator.events)
	}
}

func TestAgentRuntimeFinalizerFailureDoesNotBlockOthersOrStatusProjection(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-isolation", "run_runtime_isolation")
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_isolation": run},
	}
	agentRepo := &fakeFinalizerAgentRepo{getErr: errors.New("agents table unavailable")}
	taskRepo := &fakeFinalizerTaskRepo{task: &model.PMTask{ID: "task-1", WorkflowStateID: "state-1"}}
	ruleEvaluator := &fakeFinalizerRuleEvaluator{}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		runFinalizers: &AgentRunFinalizerService{
			runRepo:    runRepo,
			agentRepo:  agentRepo,
			taskRepo:   taskRepo,
			ruleEngine: ruleEvaluator,
		},
		now: time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_isolation",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(ruleEvaluator.events) != 1 {
		t.Fatalf("expected automation finalizer to run despite agent finalizer failure, got %#v", ruleEvaluator.events)
	}
	if run.Status != model.AgentRunStatusCompleted || runRepo.updates != 1 || runRepo.notifications != 1 {
		t.Fatalf("expected terminal status projection despite finalizer failure: status=%s updates=%d notify=%d", run.Status, runRepo.updates, runRepo.notifications)
	}
	if runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerAgentIdleSummaryKey) {
		t.Fatalf("failed finalizer must not record its marker, got %s", string(run.OutputSummary))
	}
}

func TestAgentRuntimeFinalizersSkipCompletedOnlyFinalizersOnFailure(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-failed", "run_runtime_failed")
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_failed": run},
	}
	agentRepo := &fakeFinalizerAgentRepo{agent: &model.Agent{ID: "agent-1", Status: "working"}}
	taskRepo := &fakeFinalizerTaskRepo{task: &model.PMTask{ID: "task-1", WorkflowStateID: "state-1"}}
	ruleEvaluator := &fakeFinalizerRuleEvaluator{}
	messageRepo := &fakeFinalizerSupportMessageRepo{}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		runFinalizers: &AgentRunFinalizerService{
			runRepo:            runRepo,
			agentRepo:          agentRepo,
			taskRepo:           taskRepo,
			ruleEngine:         ruleEvaluator,
			supportMessageRepo: messageRepo,
			conversationRepo:   &fakeFinalizerConversationRepo{},
		},
		now: time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_failed",
		Type:  "run.failed",
		Data:  map[string]any{"error": "runtime exploded"},
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusFailed {
		t.Fatalf("expected failed status, got %q", run.Status)
	}
	if agentRepo.updates != 1 || agentRepo.agent.Status != "idle" {
		t.Fatalf("expected agent idle bookkeeping on failed transition, updates=%d", agentRepo.updates)
	}
	if len(ruleEvaluator.events) != 0 {
		t.Fatalf("agent_run.completed rules must not fire for failed runs, got %#v", ruleEvaluator.events)
	}
	if messageRepo.creates != 0 {
		t.Fatalf("support draft must not finalize for failed runs, creates=%d", messageRepo.creates)
	}
}

func TestAgentRuntimeFinalizersSupportDraftFromRuntimeSummary(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-support", "run_runtime_support")
	run.TargetType = "support_conversation"
	run.TargetID = "conv-1"
	run.TaskID = nil
	run.ConversationID = stringPointer("conv-1")
	run.ApprovalState = "not_required"
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_support": run},
	}
	conversation := &model.SupportConversation{ID: "conv-1", WorkspaceID: "ws-1", AnonymousID: stringPointer("anon-1")}
	conversationRepo := &fakeFinalizerConversationRepo{conversation: conversation}
	messageRepo := &fakeFinalizerSupportMessageRepo{}
	publisher := &fakeFinalizerPublisher{}
	agentRepo := &fakeFinalizerAgentRepo{agent: &model.Agent{ID: "agent-1", Status: "working"}}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		getRuns: map[string]*AgentRuntimeRun{
			"run_runtime_support": {
				ID:            "run_runtime_support",
				HostRunID:     run.ID,
				Status:        model.AgentRunStatusCompleted,
				OutputSummary: json.RawMessage(`{"draft_reply":{"content":"Hi, here is your answer.","is_internal":false,"approval_required":false}}`),
			},
		},
	}
	finalizers := &AgentRunFinalizerService{
		runRepo:            runRepo,
		agentRepo:          agentRepo,
		conversationRepo:   conversationRepo,
		supportMessageRepo: messageRepo,
		wsPublisher:        publisher,
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		agentRuntimeClient: runtimeClient,
		runFinalizers:      finalizers,
		now:                time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_support",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if messageRepo.creates != 1 {
		t.Fatalf("expected one support reply message, got %d", messageRepo.creates)
	}
	message := messageRepo.messages[run.ID]
	if message == nil || message.ConversationID != "conv-1" || message.SenderType != "agent" || message.Content != "Hi, here is your answer." {
		t.Fatalf("unexpected support message: %#v", message)
	}
	var summary map[string]any
	if err := json.Unmarshal(run.OutputSummary, &summary); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if got := strings.TrimSpace(getStringFromMap(summary, "sent_message_id")); got != run.ID {
		t.Fatalf("expected sent_message_id %q, got %q (summary=%s)", run.ID, got, string(run.OutputSummary))
	}
	if len(publisher.events) != 5 {
		t.Fatalf("expected streamed reply + message + visitor refresh events, got %#v", publisher.events)
	}
	wantActions := []string{"response_started", "response_delta", "created", "response_completed", "updated"}
	for index, wantAction := range wantActions {
		if publisher.events[index].Action != wantAction {
			t.Fatalf("event %d action = %q, want %q", index, publisher.events[index].Action, wantAction)
		}
	}

	// Crash-replay while still non-terminal: sent_message_id short-circuits.
	finalizers.FinalizeTerminalRun(context.Background(), run, true)
	if messageRepo.creates != 1 || len(publisher.events) != 5 {
		t.Fatalf("expected support draft replay no-op, creates=%d events=%d", messageRepo.creates, len(publisher.events))
	}
}

func TestAgentRuntimeFinalizersSupportDraftSkippedWhenRuntimeSummaryUnavailable(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-support-missing", "run_runtime_support_missing")
	run.TargetType = "support_conversation"
	run.TargetID = "conv-1"
	run.TaskID = nil
	run.ConversationID = stringPointer("conv-1")
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_support_missing": run},
	}
	messageRepo := &fakeFinalizerSupportMessageRepo{}
	agentRepo := &fakeFinalizerAgentRepo{agent: &model.Agent{ID: "agent-1", Status: "working"}}
	runtimeClient := &fakeAgentRuntimeSignalClient{getErr: errors.New("runtime unreachable")}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		agentRuntimeClient: runtimeClient,
		runFinalizers: &AgentRunFinalizerService{
			runRepo:            runRepo,
			agentRepo:          agentRepo,
			conversationRepo:   &fakeFinalizerConversationRepo{conversation: &model.SupportConversation{ID: "conv-1"}},
			supportMessageRepo: messageRepo,
		},
		now: time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_support_missing",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if messageRepo.creates != 0 {
		t.Fatalf("summary-dependent finalizer must be skipped without the runtime summary, creates=%d", messageRepo.creates)
	}
	// Summary-independent finalizers still run.
	if agentRepo.updates != 1 || agentRepo.agent.Status != "idle" {
		t.Fatalf("expected agent idle bookkeeping despite summary fetch failure, updates=%d", agentRepo.updates)
	}
	if run.Status != model.AgentRunStatusCompleted || runRepo.updates != 1 {
		t.Fatalf("expected status projection despite summary fetch failure: status=%s updates=%d", run.Status, runRepo.updates)
	}
}

func TestAgentRuntimeFinalizersEpicPlanningPointerIdempotent(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-epic", "run_runtime_epic")
	run.TargetType = "epic"
	run.TargetID = "epic-1"
	run.TaskID = nil
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_epic": run},
	}
	epicRepo := &fakeFinalizerEpicRepo{epic: &model.EpicWithStats{Epic: model.PMEpic{ID: "epic-1"}}}
	finalizers := &AgentRunFinalizerService{
		runRepo:  runRepo,
		epicRepo: epicRepo,
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:       runRepo,
		runFinalizers: finalizers,
		now:           time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_epic",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(epicRepo.updates) != 1 || epicRepo.updates[0].LastPlanningRunID == nil || *epicRepo.updates[0].LastPlanningRunID != run.ID {
		t.Fatalf("expected epic planning pointer update, got %#v", epicRepo.updates)
	}
	if !runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerPlanningSummaryKey) {
		t.Fatalf("expected planning marker, got %s", string(run.OutputSummary))
	}

	// Crash-replay while still non-terminal: marker prevents a second write.
	finalizers.FinalizeTerminalRun(context.Background(), run, true)
	if epicRepo.getCalls != 1 || len(epicRepo.updates) != 1 {
		t.Fatalf("expected planning finalizer replay no-op, getCalls=%d updates=%d", epicRepo.getCalls, len(epicRepo.updates))
	}
}

func TestAgentRuntimeFinalizersFlowOutputValidationMarksWithoutFailingRun(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-flow", "run_runtime_flow")
	run.TargetType = "task"
	run.Input = json.RawMessage(`{"flow_output_kind":"pm.task_completion_followups"}`)
	// Missing "summary": validation fails, is logged, and must not fail the run.
	run.OutputSummary = json.RawMessage(`{"status":"success"}`)
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_flow": run},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		runFinalizers: &AgentRunFinalizerService{
			runRepo: runRepo,
		},
		now: time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_flow",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusCompleted {
		t.Fatalf("expected completed status despite invalid flow output, got %q", run.Status)
	}
	if !runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerPlanningSummaryKey) {
		t.Fatalf("expected planning marker after flow validation, got %s", string(run.OutputSummary))
	}
}

type fakeFinalizerCommandBarAdvancer struct {
	calls []model.AgentRun
	err   error
}

func (a *fakeFinalizerCommandBarAdvancer) AdvanceCommandBarPlanForDelegatedRun(_ context.Context, run *model.AgentRun) (*model.AgentRun, error) {
	a.calls = append(a.calls, *run)
	if a.err != nil {
		return nil, a.err
	}
	return nil, nil
}

func commandBarDelegatedRunInput(t *testing.T, planID string, stepIndex int) json.RawMessage {
	t.Helper()
	target := model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"}
	steps := []model.CommandBarPlanStep{
		{AgentID: "agent-1", AgentName: "Forge", Target: target, Instructions: "Build it."},
		{AgentID: "agent-1", AgentName: "Lens", Target: target, Instructions: "Review it."},
	}
	trigger, err := buildCommandBarTriggerContext("run forge then lens", target, steps, stepIndex, planID)
	if err != nil {
		t.Fatalf("build command bar trigger: %v", err)
	}
	input, err := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	if err != nil {
		t.Fatalf("marshal command bar input: %v", err)
	}
	return input
}

func TestAgentRuntimeFinalizersCommandBarPlanAdvancesWithInMemoryStatus(t *testing.T) {
	tests := []struct {
		name       string
		eventType  string
		wantStatus string
	}{
		{name: "completed", eventType: "run.completed", wantStatus: model.AgentRunStatusCompleted},
		{name: "failed", eventType: "run.failed", wantStatus: model.AgentRunStatusFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := newDelegatedTerminalTestRun("helpin-run-cbp-"+tt.name, "run_runtime_cbp_"+tt.name)
			run.Input = commandBarDelegatedRunInput(t, "plan-1", 0)
			runRepo := &fakeAgentRuntimeProjectionRunRepo{
				byID:       map[string]*model.AgentRun{run.ID: run},
				byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_cbp_" + tt.name: run},
			}
			advancer := &fakeFinalizerCommandBarAdvancer{}
			svc := &AgentRuntimeProjectionService{
				runRepo: runRepo,
				runFinalizers: &AgentRunFinalizerService{
					runRepo:            runRepo,
					commandBarAdvancer: advancer,
				},
				now: time.Now,
			}
			event := AgentRuntimeEventEnvelope{RunID: "run_runtime_cbp_" + tt.name, Type: tt.eventType}

			if err := svc.ApplyEvent(context.Background(), event); err != nil {
				t.Fatalf("ApplyEvent returned error: %v", err)
			}
			if len(advancer.calls) != 1 {
				t.Fatalf("expected one plan advancement call, got %d", len(advancer.calls))
			}
			if advancer.calls[0].Status != tt.wantStatus {
				t.Fatalf("expected in-memory terminal status %q passed to advancer, got %q", tt.wantStatus, advancer.calls[0].Status)
			}
			if !runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerCommandBarPlanSummaryKey) {
				t.Fatalf("expected command bar plan marker, got %s", string(run.OutputSummary))
			}

			// Redelivered terminal event on the now-terminal run is a no-op.
			if err := svc.ApplyEvent(context.Background(), event); err != nil {
				t.Fatalf("ApplyEvent redelivery returned error: %v", err)
			}
			if len(advancer.calls) != 1 {
				t.Fatalf("expected redelivery to skip advancement, got %d calls", len(advancer.calls))
			}
		})
	}
}

func TestAgentRuntimeFinalizersCommandBarPlanMarkerSkipsCrashReplay(t *testing.T) {
	// Crash after the advancement marker persisted but before the terminal
	// status: the redelivered event re-enters dispatch and must not advance
	// the plan a second time.
	run := newDelegatedTerminalTestRun("helpin-run-cbp-replay", "run_runtime_cbp_replay")
	run.Input = commandBarDelegatedRunInput(t, "plan-1", 0)
	run.OutputSummary = json.RawMessage(`{"` + agentRuntimeFinalizerCommandBarPlanSummaryKey + `":true}`)
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_cbp_replay": run},
	}
	advancer := &fakeFinalizerCommandBarAdvancer{}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		runFinalizers: &AgentRunFinalizerService{
			runRepo:            runRepo,
			commandBarAdvancer: advancer,
		},
		now: time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_cbp_replay",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(advancer.calls) != 0 {
		t.Fatalf("expected marker to skip plan advancement, got %d calls", len(advancer.calls))
	}
	if run.Status != model.AgentRunStatusCompleted {
		t.Fatalf("expected terminal status projection, got %q", run.Status)
	}
}

func TestAgentRuntimeFinalizersCommandBarPlanSkipsNonCommandBarRunsWithoutMarker(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-cbp-plain", "run_runtime_cbp_plain")
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_cbp_plain": run},
	}
	advancer := &fakeFinalizerCommandBarAdvancer{}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		runFinalizers: &AgentRunFinalizerService{
			runRepo:            runRepo,
			commandBarAdvancer: advancer,
		},
		now: time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_cbp_plain",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(advancer.calls) != 0 {
		t.Fatalf("expected non-command-bar run to skip advancement, got %d calls", len(advancer.calls))
	}
	if runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerCommandBarPlanSummaryKey) {
		t.Fatalf("non-command-bar run must not record the plan marker, got %s", string(run.OutputSummary))
	}
}

func TestAgentRuntimeFinalizersCommandBarPlanAdvancerFailureIsIsolated(t *testing.T) {
	run := newDelegatedTerminalTestRun("helpin-run-cbp-fail", "run_runtime_cbp_fail")
	run.Input = commandBarDelegatedRunInput(t, "plan-1", 0)
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_cbp_fail": run},
	}
	agentRepo := &fakeFinalizerAgentRepo{agent: &model.Agent{ID: "agent-1", Status: "working"}}
	advancer := &fakeFinalizerCommandBarAdvancer{err: errors.New("plan repo unavailable")}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		runFinalizers: &AgentRunFinalizerService{
			runRepo:            runRepo,
			agentRepo:          agentRepo,
			commandBarAdvancer: advancer,
		},
		now: time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_cbp_fail",
		Type:  "run.completed",
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(advancer.calls) != 1 {
		t.Fatalf("expected one failed advancement attempt, got %d", len(advancer.calls))
	}
	if runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerCommandBarPlanSummaryKey) {
		t.Fatalf("failed advancement must not record the marker, got %s", string(run.OutputSummary))
	}
	if agentRepo.updates != 1 {
		t.Fatalf("expected other finalizers to run despite advancement failure, agent updates=%d", agentRepo.updates)
	}
	if run.Status != model.AgentRunStatusCompleted || runRepo.updates != 1 {
		t.Fatalf("expected status projection despite advancement failure: status=%s updates=%d", run.Status, runRepo.updates)
	}
}

func TestMergeRuntimeOutputSummaryPayloadPreservesHostMarkers(t *testing.T) {
	local := json.RawMessage(`{"agent_runtime_usage_consumed":true,"agent_runtime_finalizer_agent_idle":true,"support_coverage_gap_outcome":{"outcome":"resolved","action":"document_updated","document_id":"doc-host","summary":"host validated"}}`)
	runtime := json.RawMessage(`{"draft_reply":{"content":"hello"},"agent_runtime_usage_consumed":false,"support_coverage_gap_outcome":{"outcome":"handoff","summary":"runtime supplied"},"status":"success"}`)
	merged := mergeRuntimeOutputSummaryPayload(local, runtime)
	var body map[string]any
	if err := json.Unmarshal(merged, &body); err != nil {
		t.Fatalf("unmarshal merged summary: %v", err)
	}
	if consumed, _ := body["agent_runtime_usage_consumed"].(bool); !consumed {
		t.Fatalf("host-reserved key must win, got %s", string(merged))
	}
	if idle, _ := body["agent_runtime_finalizer_agent_idle"].(bool); !idle {
		t.Fatalf("host marker lost in merge: %s", string(merged))
	}
	coverage, _ := body[supportCoverageGapOutcomeSummaryKey].(map[string]any)
	if coverage["document_id"] != "doc-host" || coverage["outcome"] != SupportCoverageAgentOutcomeResolved {
		t.Fatalf("host-validated support coverage outcome lost in merge: %s", string(merged))
	}
	if _, ok := body["draft_reply"]; !ok {
		t.Fatalf("runtime contract key missing after merge: %s", string(merged))
	}
	if status, _ := body["status"].(string); status != "success" {
		t.Fatalf("runtime key not merged: %s", string(merged))
	}
}

func TestAgentRuntimeFinalizerReappliesSupportCoverageGapOutcome(t *testing.T) {
	outcome := supportCoverageGapOutcomeSummary{
		Outcome: SupportCoverageAgentOutcomeResolved, DocumentID: "doc-1", Summary: "Updated reset guidance.",
	}
	payload, err := json.Marshal(map[string]any{supportCoverageGapOutcomeSummaryKey: outcome})
	if err != nil {
		t.Fatal(err)
	}
	run := &model.AgentRun{
		ID: "run-gap", WorkspaceID: "ws-1", TargetType: "support_coverage_gap", TargetID: "gap-1",
		Status: model.AgentRunStatusCompleted, OutputSummary: payload,
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{run.ID: run}}
	coverage := &fakeFinalizerSupportCoverage{}
	finalizers := (&AgentRunFinalizerService{runRepo: runRepo}).SetSupportCoverageService(coverage)

	if err := finalizers.finalizeSupportCoverageGap(context.Background(), run); err != nil {
		t.Fatalf("finalizeSupportCoverageGap returned error: %v", err)
	}
	if len(coverage.calls) != 1 || coverage.gapID != "gap-1" || coverage.calls[0].DocumentID != "doc-1" {
		t.Fatalf("unexpected coverage finalizer calls: %#v gap=%q", coverage.calls, coverage.gapID)
	}
	if !runOutputSummaryFlag(run.OutputSummary, agentRuntimeFinalizerSupportCoverageGapSummaryKey) {
		t.Fatalf("coverage finalizer marker missing: %s", string(run.OutputSummary))
	}
}

func getStringFromMap(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}
