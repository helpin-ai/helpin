package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// setupAutomationDelegationTestDB extends the command-bar plan schema with the
// automation-rule and trigger-execution tables needed to exercise the
// automation start_agent_run -> delegated agent-runtime launch path.
func setupAutomationDelegationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupCommandBarPlanTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE automation_rules (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			team_id TEXT,
			workflow_id TEXT,
			trigger_type TEXT NOT NULL,
			trigger_config TEXT NOT NULL DEFAULT '{}',
			action_type TEXT NOT NULL,
			action_config TEXT NOT NULL DEFAULT '{}',
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			position INTEGER NOT NULL DEFAULT 0,
			stop_on_match BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_trigger_executions (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			actor_id TEXT,
			binding_id TEXT NOT NULL,
			binding_kind TEXT NOT NULL,
			trigger_type TEXT,
			reference_id TEXT,
			reference_type TEXT,
			target_type TEXT,
			target_id TEXT,
			run_id TEXT,
			status TEXT NOT NULL,
			error_message TEXT,
			fired_at DATETIME,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create automation delegation test table: %v", err)
		}
	}
	return db
}

func seedDelegatingWorkspaceAgent(t *testing.T, db *gorm.DB, agentID, workspaceID string) {
	t.Helper()
	if err := db.Create(&model.Agent{
		ID:                    agentID,
		WorkspaceID:           workspaceID,
		IsSystem:              true,
		Name:                  "Marketer",
		PresetKey:             model.AgentPresetMarketer,
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		ExecutionConfig:       model.JSONBlob(`{}`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "never",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}).Error; err != nil {
		t.Fatalf("seed delegating agent: %v", err)
	}
}

// TestScheduledRuleStartAgentRunDelegatesToAgentRuntime drives the full
// automation cron loop (ScheduledRuleWorkflow activity -> ExecuteScheduledRule
// -> executeStartAgentRun -> startTargetRun -> createRun) for an agent whose
// preset/target delegates to the agent runtime, and verifies the run is
// launched on the runtime (no Temporal workflow recorded) with the trigger
// execution recorded before the delegation branch.
func TestScheduledRuleStartAgentRunDelegatesToAgentRuntime(t *testing.T) {
	db := setupAutomationDelegationTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	ruleRepo := repository.NewAutomationRuleRepository(db)
	triggerExecRepo := repository.NewAgentTriggerExecutionRepository(db)

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	agentID := "22222222-2222-2222-2222-222222222222"
	seedDelegatingWorkspaceAgent(t, db, agentID, workspaceID)

	runtimeClient := &fakeAgentRuntimeSignalClient{}
	agentService := (&AgentService{agentRepo: agentRepo, runRepo: runRepo}).
		SetTriggerExecutionRepository(triggerExecRepo).
		SetAgentRuntimeClient(runtimeClient).
		SetAgentRuntimeLaunchEnabled(true)
	if !agentService.delegatesRunToAgentRuntime(&model.Agent{IsSystem: true, PresetKey: model.AgentPresetMarketer, RuntimeKind: "native_sdk"}, "workspace") {
		t.Skip("marketer/workspace is no longer a delegated preset target; update this test to a delegated combination")
	}

	engine := NewAutomationRuleEngine(ruleRepo, nil, nil, nil, nil, nil, nil, nil)
	engine.SetAgentService(agentService)

	rule := &model.AutomationRule{
		ID:            "33333333-3333-3333-3333-333333333333",
		WorkspaceID:   workspaceID,
		Name:          "Weekly digest",
		Enabled:       true,
		TriggerType:   model.TriggerCron,
		TriggerConfig: json.RawMessage(`{"preset":"weekly"}`),
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  json.RawMessage(`{"agent_id":"` + agentID + `","target_type":"workspace","target_id":"` + workspaceID + `"}`),
	}
	if err := ruleRepo.Create(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	if err := engine.ExecuteScheduledRule(ctx, workspaceID, rule.ID); err != nil {
		t.Fatalf("ExecuteScheduledRule returned error: %v", err)
	}

	if len(runtimeClient.upsertAgents) != 1 || len(runtimeClient.startRunCalls) != 1 {
		t.Fatalf("expected one runtime upsert and start, got upserts=%d starts=%d", len(runtimeClient.upsertAgents), len(runtimeClient.startRunCalls))
	}
	runs, err := runRepo.ListByTarget(ctx, workspaceID, "workspace", workspaceID)
	if err != nil {
		t.Fatalf("list workspace runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one delegated run, got %#v", runs)
	}
	run := runs[0]
	if run.WorkflowID != nil || run.WorkflowRunID != nil {
		t.Fatalf("delegated run must not record a Temporal workflow, got %v/%v", run.WorkflowID, run.WorkflowRunID)
	}
	if run.ExternalRuntime == nil || *run.ExternalRuntime != agentRuntimeName || run.ExternalRuntimeID == nil || *run.ExternalRuntimeID != "run_runtime_1" {
		t.Fatalf("expected delegated runtime linkage, got runtime=%v id=%v", run.ExternalRuntime, run.ExternalRuntimeID)
	}

	var executions []model.AgentTriggerExecution
	if err := db.Find(&executions).Error; err != nil {
		t.Fatalf("list trigger executions: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("expected one trigger execution, got %#v", executions)
	}
	execution := executions[0]
	if execution.BindingID != "automation_rule.cron" || execution.BindingKind != "automation_rule" {
		t.Fatalf("unexpected trigger binding: %s/%s", execution.BindingID, execution.BindingKind)
	}
	if execution.RunID == nil || *execution.RunID != run.ID {
		t.Fatalf("expected trigger execution linked to run %q, got %#v", run.ID, execution.RunID)
	}
	if execution.ReferenceID == nil || *execution.ReferenceID != rule.ID {
		t.Fatalf("expected trigger execution referencing rule %q, got %#v", rule.ID, execution.ReferenceID)
	}
	if execution.Status == model.AgentTriggerExecutionStatusFailed {
		t.Fatalf("expected non-failed trigger execution, got %q (%v)", execution.Status, execution.ErrorMessage)
	}
}

func TestCreateRunRecordsTriggerFailureWhenLaunchPromptPersistenceFails(t *testing.T) {
	db := setupAutomationDelegationTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	triggerExecRepo := repository.NewAgentTriggerExecutionRepository(db)
	// setupAutomationDelegationTestDB intentionally has no agent_run_messages
	// table, so a configured repository reaches the prompt-persistence failure
	// path after the run row has been created.
	messageRepo := repository.NewAgentRunMessageRepository(db)
	service := (&AgentService{
		runRepo:        runRepo,
		runMessageRepo: messageRepo,
	}).SetTriggerExecutionRepository(triggerExecRepo)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	agentID := "22222222-2222-2222-2222-222222222222"
	ruleID := "33333333-3333-3333-3333-333333333333"
	trigger := &model.AgentRunTriggerContext{
		Source:      model.AgentRunTriggerSourceAutomationRule,
		TriggerType: model.TriggerCron,
		RuleID:      &ruleID,
	}
	payload, err := json.Marshal(model.AgentRunInputPayload{AdditionalContext: "Persist this launch context."})
	if err != nil {
		t.Fatalf("marshal run input: %v", err)
	}

	_, err = service.createRun(context.Background(), createRunParams{
		workspaceID: workspaceID,
		agent: &model.Agent{
			ID:          agentID,
			IsSystem:    true,
			PresetKey:   model.AgentPresetMarketer,
			RuntimeKind: "native_sdk",
		},
		targetType: "workspace",
		targetID:   workspaceID,
		trigger:    trigger,
		input:      payload,
	})
	if err == nil {
		t.Fatal("expected launch prompt persistence to fail")
	}

	var executions []model.AgentTriggerExecution
	if err := db.Find(&executions).Error; err != nil {
		t.Fatalf("list trigger executions: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("expected one failed trigger execution, got %#v", executions)
	}
	execution := executions[0]
	if execution.Status != model.AgentTriggerExecutionStatusFailed || execution.ErrorMessage == nil {
		t.Fatalf("expected failed trigger execution with error, got %#v", execution)
	}
	if execution.RunID == nil || strings.TrimSpace(*execution.RunID) == "" {
		t.Fatalf("expected failed trigger execution to retain its created run ID, got %#v", execution)
	}
}

// TestReconcileStuckRunLeavesDelegatedQueuedRunAlone guards the WorkflowID
// assumption in reconcileStaleQueuedRun: delegated runs never record a
// Temporal workflow, so an old queued delegated run (runtime slow to start)
// must not be failed with "no Temporal workflow execution was recorded".
func TestReconcileStuckRunLeavesDelegatedQueuedRunAlone(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	service := &AgentService{runRepo: runRepo, agentRepo: agentRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	runtimeName := agentRuntimeName
	runtimeRunID := "run_runtime_stale"
	run := &model.AgentRun{
		ID:                "22222222-2222-2222-2222-222222222222",
		WorkspaceID:       workspaceID,
		AgentID:           "33333333-3333-3333-3333-333333333333",
		TargetType:        "workspace",
		TargetID:          workspaceID,
		RuntimeKind:       "native_sdk",
		InvocationMode:    model.InvocationModeAutonomous,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonNone,
		Status:            model.AgentRunStatusQueued,
		ExternalRuntime:   &runtimeName,
		ExternalRuntimeID: &runtimeRunID,
		Input:             json.RawMessage("{}"),
		OutputSummary:     json.RawMessage("{}"),
	}
	if err := runRepo.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	staleCreatedAt := time.Now().Add(-2 * staleQueuedRunThreshold)
	if err := db.Model(&model.AgentRun{}).Where("id = ?", run.ID).Update("created_at", staleCreatedAt).Error; err != nil {
		t.Fatalf("age run: %v", err)
	}
	run.CreatedAt = staleCreatedAt

	updated := service.reconcileStuckRun(ctx, run)
	if updated == nil || updated.Status != model.AgentRunStatusQueued {
		t.Fatalf("expected delegated queued run to be left alone, got %#v", updated)
	}
	persisted, err := runRepo.GetByIDAny(ctx, run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if persisted.Status != model.AgentRunStatusQueued {
		t.Fatalf("expected persisted status queued, got %q", persisted.Status)
	}

	// Contrast: the same stale queued run without runtime linkage is still
	// reconciled as failed-to-start.
	orphan := &model.AgentRun{
		ID:             "44444444-4444-4444-4444-444444444444",
		WorkspaceID:    workspaceID,
		AgentID:        "33333333-3333-3333-3333-333333333333",
		TargetType:     "workspace",
		TargetID:       workspaceID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusQueued,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}
	if err := runRepo.Create(ctx, orphan); err != nil {
		t.Fatalf("create orphan run: %v", err)
	}
	if err := db.Model(&model.AgentRun{}).Where("id = ?", orphan.ID).Update("created_at", staleCreatedAt).Error; err != nil {
		t.Fatalf("age orphan run: %v", err)
	}
	orphan.CreatedAt = staleCreatedAt
	if updated := service.reconcileStuckRun(ctx, orphan); updated == nil || updated.Status != model.AgentRunStatusFailed {
		t.Fatalf("expected orphan queued run to be failed, got %#v", updated)
	}
}

// TestAdvanceCommandBarPlanForDelegatedRunFanOutUsesInMemoryStatus covers the
// delegated finalizer path for fan-out plans: the terminal child's database
// row is still stale (running) when the finalizer dispatches, so the
// in-memory run status must decide plan completion.
func TestAdvanceCommandBarPlanForDelegatedRunFanOutUsesInMemoryStatus(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &AgentService{runRepo: runRepo, commandBarPlanRepo: planRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	agentID := "33333333-3333-3333-3333-333333333333"
	runOneID := "44444444-4444-4444-4444-444444444444"
	runTwoID := "55555555-5555-5555-5555-555555555555"
	pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: "epic-1", DisplayTitle: "Epic 1"}
	steps := []model.CommandBarPlanStep{
		{
			AgentID:      agentID,
			AgentName:    "Lens",
			PlanKind:     model.CommandBarPlanKindFanOut,
			Target:       model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
			Instructions: "Review task 1.",
		},
		{
			AgentID:      agentID,
			AgentName:    "Lens",
			PlanKind:     model.CommandBarPlanKindFanOut,
			Target:       model.CommandBarPageContext{EntityType: "task", EntityID: "task-2", DisplayTitle: "Task 2"},
			Instructions: "Review task 2.",
		},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run lens across all child tasks", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: runOneID, 1: runTwoID})
	plan.RunIDsByStep = runIDs
	plan.RunCount = 2
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	trigger, err := buildCommandBarTriggerContext("run lens across all child tasks", pageContext, steps, 1, planID)
	if err != nil {
		t.Fatalf("build trigger: %v", err)
	}
	input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             runOneID,
		WorkspaceID:    workspaceID,
		AgentID:        agentID,
		TargetType:     "task",
		TargetID:       "task-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusCompleted,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create first run: %v", err)
	}
	// The delegated child's row is still active: the projection persists the
	// terminal status only after finalizers dispatch.
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             runTwoID,
		WorkspaceID:    workspaceID,
		AgentID:        agentID,
		TargetType:     "task",
		TargetID:       "task-2",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          input,
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create second run: %v", err)
	}

	inMemory, err := runRepo.GetByIDAny(ctx, runTwoID)
	if err != nil || inMemory == nil {
		t.Fatalf("load run: %v", err)
	}
	inMemory.Status = model.AgentRunStatusCompleted

	if _, err := service.AdvanceCommandBarPlanForDelegatedRun(ctx, inMemory); err != nil {
		t.Fatalf("advance delegated fan-out run: %v", err)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusCompleted {
		t.Fatalf("expected in-memory terminal status to complete fan-out plan, got %q", updated.Status)
	}
}

// TestAdvanceCommandBarPlanForDelegatedRunLinearFailureUsesInMemoryStatus
// covers the delegated finalizer path for linear plans: a failed child whose
// database row is still stale must fail the plan from the in-memory status.
func TestAdvanceCommandBarPlanForDelegatedRunLinearFailureUsesInMemoryStatus(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &AgentService{runRepo: runRepo, commandBarPlanRepo: planRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	agentID := "33333333-3333-3333-3333-333333333333"
	runID := "44444444-4444-4444-4444-444444444444"
	targetID := "55555555-5555-5555-5555-555555555555"
	pageContext := model.CommandBarPageContext{EntityType: "task", EntityID: targetID, DisplayTitle: "Task 1"}
	steps := []model.CommandBarPlanStep{
		{AgentID: agentID, AgentName: "Forge", Target: pageContext, Instructions: "Build it."},
		{AgentID: agentID, AgentName: "Lens", Target: pageContext, Instructions: "Review it."},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run forge and lens", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: runID})
	plan.RunIDsByStep = runIDs
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	trigger, err := buildCommandBarTriggerContext("run forge and lens", pageContext, steps, 0, planID)
	if err != nil {
		t.Fatalf("build trigger: %v", err)
	}
	input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             runID,
		WorkspaceID:    workspaceID,
		AgentID:        agentID,
		TargetType:     "task",
		TargetID:       targetID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          input,
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}

	inMemory, err := runRepo.GetByIDAny(ctx, runID)
	if err != nil || inMemory == nil {
		t.Fatalf("load run: %v", err)
	}
	errMsg := "forge failed"
	inMemory.Status = model.AgentRunStatusFailed
	inMemory.ErrorMessage = &errMsg

	if _, err := service.AdvanceCommandBarPlanForDelegatedRun(ctx, inMemory); err != nil {
		t.Fatalf("advance delegated linear run: %v", err)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusFailed {
		t.Fatalf("expected in-memory failed status to fail linear plan, got %q", updated.Status)
	}
	if updated.ErrorMessage == nil || *updated.ErrorMessage != errMsg {
		t.Fatalf("expected plan error %q, got %#v", errMsg, updated.ErrorMessage)
	}
}
