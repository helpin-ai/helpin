package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"


	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type scriptedCommandBarLLM struct {
	response  string
	responses []string
	err       error
	errors    []error
	requests  []llm.ChatRequest
	deadlines []time.Time
}

func (s *scriptedCommandBarLLM) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	s.requests = append(s.requests, req)
	if deadline, ok := ctx.Deadline(); ok {
		s.deadlines = append(s.deadlines, deadline)
	} else {
		s.deadlines = append(s.deadlines, time.Time{})
	}
	if s.err != nil {
		return nil, s.err
	}
	if len(s.errors) > 0 {
		err := s.errors[0]
		s.errors = s.errors[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(s.responses) > 0 {
		response := s.responses[0]
		s.responses = s.responses[1:]
		return &llm.ChatResponse{Content: response}, nil
	}
	return &llm.ChatResponse{Content: s.response}, nil
}





















func TestCommandBarAdditionalContextDoesNotIncludeRawUserRequest(t *testing.T) {
	context := commandBarAdditionalContext(
		"Execute your normal Forge role for the current target.",
		model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
		0,
		2,
	)
	if strings.Contains(strings.ToLower(context), "run forge and then lens") {
		t.Fatalf("expected raw prompt to stay out of execution context, got %q", context)
	}
	if !strings.Contains(context, "Step instruction:") {
		t.Fatalf("expected execution context to include step instruction, got %q", context)
	}
	if !strings.Contains(context, "Other command-bar plan steps are scheduled separately") {
		t.Fatalf("expected multi-step scheduler guidance, got %q", context)
	}
}














func TestValidateCommandBarStepDependenciesRejectsCycle(t *testing.T) {
	steps := []model.CommandBarPlanStep{
		{PlanKind: model.CommandBarPlanKindDAG, DependsOnStepIndexes: []int{1}},
		{PlanKind: model.CommandBarPlanKindDAG, DependsOnStepIndexes: []int{0}},
	}
	err := validateCommandBarStepDependencies(steps, maxCommandBarDAGInitialFanOut)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle validation error, got %v", err)
	}
}

func TestValidateCommandBarStepDependenciesRejectsInitialFanOutCap(t *testing.T) {
	steps := make([]model.CommandBarPlanStep, 0, maxCommandBarDAGInitialFanOut+1)
	for i := 0; i < maxCommandBarDAGInitialFanOut+1; i++ {
		steps = append(steps, model.CommandBarPlanStep{PlanKind: model.CommandBarPlanKindDAG})
	}
	err := validateCommandBarStepDependencies(steps, maxCommandBarDAGInitialFanOut)
	if err == nil || !strings.Contains(err.Error(), "initially runnable") {
		t.Fatalf("expected fan-out validation error, got %v", err)
	}
}

func TestCommandBarSchedulerReadinessRequiresCompletedDependencies(t *testing.T) {
	completedRunID := "run-completed"
	runningRunID := "run-running"
	runsByID := map[string]model.AgentRun{
		completedRunID: {ID: completedRunID, Status: model.AgentRunStatusCompleted},
		runningRunID:   {ID: runningRunID, Status: model.AgentRunStatusRunning},
	}

	linearSteps := []model.CommandBarPlanStep{
		{},
		{DependsOnStepIndexes: []int{0}},
	}
	if !commandBarStepDependenciesSatisfied(linearSteps[1], map[int]string{0: completedRunID}, runsByID) {
		t.Fatalf("expected completed dependency to make step ready")
	}
	parentRunID := commandBarParentRunIDForStep(linearSteps, 1, map[int]string{0: completedRunID}, runsByID)
	if parentRunID == nil || *parentRunID != completedRunID {
		t.Fatalf("expected one-to-one dependency to provide parent run %q, got %v", completedRunID, parentRunID)
	}

	if commandBarStepDependenciesSatisfied(
		model.CommandBarPlanStep{DependsOnStepIndexes: []int{1}},
		map[int]string{1: runningRunID},
		runsByID,
	) {
		t.Fatalf("expected running dependency to keep step blocked")
	}

	if commandBarStepDependenciesSatisfied(
		model.CommandBarPlanStep{DependsOnStepIndexes: []int{2}},
		map[int]string{},
		runsByID,
	) {
		t.Fatalf("expected missing dependency run to keep step blocked")
	}

	sharedDependencySteps := []model.CommandBarPlanStep{
		{},
		{DependsOnStepIndexes: []int{0}},
		{DependsOnStepIndexes: []int{0}},
	}
	if parent := commandBarParentRunIDForStep(sharedDependencySteps, 1, map[int]string{0: completedRunID}, runsByID); parent != nil {
		t.Fatalf("expected shared dependency fan-out to avoid parent_run_id, got %v", *parent)
	}
}

func TestCommandBarExistingChildRunForParent(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	service := &AgentService{runRepo: runRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	parentRunID := "22222222-2222-2222-2222-222222222222"
	childRunID := "33333333-3333-3333-3333-333333333333"
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             childRunID,
		WorkspaceID:    workspaceID,
		AgentID:        "44444444-4444-4444-4444-444444444444",
		TargetType:     "task",
		TargetID:       "55555555-5555-5555-5555-555555555555",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ParentRunID:    &parentRunID,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusQueued,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create child run: %v", err)
	}

	existing := service.commandBarExistingChildRunForParent(ctx, workspaceID, &parentRunID)
	if existing == nil || existing.ID != childRunID {
		t.Fatalf("expected existing child run %q, got %#v", childRunID, existing)
	}
	if got := service.commandBarExistingChildRunForParent(ctx, workspaceID, nil); got != nil {
		t.Fatalf("expected nil for nil parent run id, got %#v", got)
	}
}

func TestCommandBarExistingRunForStepRequiresExactCommandBarStep(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	service := &AgentService{runRepo: runRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	parentRunID := "33333333-3333-3333-3333-333333333333"
	childRunID := "44444444-4444-4444-4444-444444444444"
	taskID := "55555555-5555-5555-5555-555555555555"
	steps := []model.CommandBarPlanStep{
		{
			AgentID:   "agent-forge",
			AgentName: "Forge",
			Target:    model.CommandBarPageContext{EntityType: "task", EntityID: taskID, DisplayTitle: "Task"},
		},
		{
			AgentID:              "agent-lens",
			AgentName:            "Lens",
			Target:               model.CommandBarPageContext{EntityType: "task", EntityID: taskID, DisplayTitle: "Task"},
			DependsOnStepIndexes: []int{0},
		},
	}
	trigger, err := buildCommandBarTriggerContext("run forge then lens", model.CommandBarPageContext{EntityType: "task", EntityID: taskID}, steps, 1, planID)
	if err != nil {
		t.Fatalf("build trigger: %v", err)
	}
	input, err := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             childRunID,
		WorkspaceID:    workspaceID,
		AgentID:        "agent-lens",
		TargetType:     "task",
		TargetID:       taskID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ParentRunID:    &parentRunID,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusQueued,
		Input:          input,
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create child run: %v", err)
	}

	existing := service.commandBarExistingRunForStep(ctx, workspaceID, &parentRunID, planID, steps, 1)
	if existing == nil || existing.ID != childRunID {
		t.Fatalf("expected exact existing child run %q, got %#v", childRunID, existing)
	}
	if got := service.commandBarExistingRunForStep(ctx, workspaceID, &parentRunID, planID, steps, 0); got != nil {
		t.Fatalf("expected mismatched step not to reuse child run, got %#v", got)
	}
}

func TestCreateRunAllowsConflictChildWhenActiveRunIsParent(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	service := &AgentService{runRepo: runRepo, agentRepo: agentRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	taskID := "22222222-2222-2222-2222-222222222222"
	parentRunID := "33333333-3333-3333-3333-333333333333"
	forgeAgent := &model.Agent{
		ID:                    "44444444-4444-4444-4444-444444444444",
		WorkspaceID:           workspaceID,
		Name:                  "Forge",
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             parentRunID,
		WorkspaceID:    workspaceID,
		AgentID:        "55555555-5555-5555-5555-555555555555",
		TargetType:     "task",
		TargetID:       taskID,
		RuntimeKind:    "internal",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create parent run: %v", err)
	}

	_, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          forgeAgent,
		targetType:     "task",
		targetID:       taskID,
		parentRunID:    &parentRunID,
		taskID:         &taskID,
		input:          []byte("{}"),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err == nil || !strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected active parent to block normal child run, got %v", err)
	}

	_, err = service.createRun(ctx, createRunParams{
		workspaceID:          workspaceID,
		agent:                forgeAgent,
		targetType:           "task",
		targetID:             taskID,
		parentRunID:          &parentRunID,
		allowActiveParentRun: true,
		taskID:               &taskID,
		input:                []byte("{}"),
		invocationMode:       model.InvocationModeAutonomous,
	})
	if err == nil || strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected active parent exception to bypass duplicate-run guard, got %v", err)
	}
}

func TestCreateRunAllowsDifferentAgentsOnWorkspaceTarget(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	service := &AgentService{runRepo: runRepo, agentRepo: agentRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	existingAgentID := "22222222-2222-2222-2222-222222222222"
	nextAgent := &model.Agent{
		ID:                    "33333333-3333-3333-3333-333333333333",
		WorkspaceID:           workspaceID,
		Name:                  "Competitors Changelog Tracking Report",
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}

	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             "44444444-4444-4444-4444-444444444444",
		WorkspaceID:    workspaceID,
		AgentID:        existingAgentID,
		TargetType:     "workspace",
		TargetID:       workspaceID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonHumanInput,
		Status:         model.AgentRunStatusPaused,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create existing workspace run: %v", err)
	}

	_, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          nextAgent,
		targetType:     "workspace",
		targetID:       workspaceID,
		input:          []byte("{}"),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err == nil || strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected different workspace agent to bypass duplicate-run guard, got %v", err)
	}

	runs, err := runRepo.ListByTarget(ctx, workspaceID, "workspace", workspaceID)
	if err != nil {
		t.Fatalf("list workspace runs: %v", err)
	}
	foundNextAgentRun := false
	for _, run := range runs {
		if run.AgentID == nextAgent.ID {
			foundNextAgentRun = true
			break
		}
	}
	if !foundNextAgentRun {
		t.Fatalf("expected new workspace run for agent %q to be created alongside existing run, got %#v", nextAgent.ID, runs)
	}
}

func TestCreateRunPreflightsAICreditsBeforeQueueingRun(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	consumer := &recordingAIUsageConsumer{preflightErr: fmt.Errorf("AI usage exhausted")}
	service := (&AgentService{runRepo: runRepo, agentRepo: agentRepo}).SetAIUsageMeter(NewAIUsageMeter(consumer))

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	taskID := "22222222-2222-2222-2222-222222222222"
	agent := &model.Agent{
		ID:                    "33333333-3333-3333-3333-333333333333",
		WorkspaceID:           workspaceID,
		Name:                  "Forge",
		PresetKey:             model.AgentPresetCodeBuilder,
		IsSystem:              true,
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}

	_, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "task",
		targetID:       taskID,
		taskID:         &taskID,
		input:          []byte("{}"),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err == nil || !strings.Contains(err.Error(), "AI usage exhausted") {
		t.Fatalf("createRun() error = %v, want AI usage exhausted", err)
	}
	if consumer.preflight.WorkspaceID != workspaceID || consumer.preflight.FeatureKey != BillingFeatureForgeRun || consumer.preflight.Credits != 100 {
		t.Fatalf("preflight = %#v, want Forge run preflight", consumer.preflight)
	}
	runs, total, err := runRepo.ListByWorkspace(ctx, workspaceID, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if total != 0 || len(runs) != 0 {
		t.Fatalf("expected no queued run after failed preflight, total=%d runs=%#v", total, runs)
	}
}

func seedCreateRunAgentRow(t *testing.T, db *gorm.DB, agent *model.Agent) {
	t.Helper()
	now := time.Now().UTC()
	execConfig := strings.TrimSpace(string(agent.ExecutionConfig))
	if execConfig == "" {
		execConfig = "{}"
	}
	allowedTools := strings.TrimSpace(string(agent.AllowedTools))
	if allowedTools == "" {
		allowedTools = "[]"
	}
	allowedTargets := strings.TrimSpace(string(agent.AllowedTargets))
	if allowedTargets == "" {
		allowedTargets = "[]"
	}
	allowedCommands := strings.TrimSpace(string(agent.AllowedCommands))
	if allowedCommands == "" {
		allowedCommands = "[]"
	}
	skills, err := json.Marshal(agent.Skills.Normalize())
	if err != nil {
		t.Fatalf("marshal agent skills: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO agents (
			id, workspace_id, is_system, name, preset_key, status, runtime_kind,
			skills, trigger_mode, provider, model, execution_config, system_prompt,
			allowed_tools, allowed_commands, allowed_targets, approval_mode,
			max_concurrent_runs, default_invocation_mode, tokens_used_this_month,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agent.ID,
		agent.WorkspaceID,
		agent.IsSystem,
		agent.Name,
		agent.PresetKey,
		agent.Status,
		agent.RuntimeKind,
		string(skills),
		defaultString(agent.TriggerMode, "manual"),
		agent.Provider,
		agent.Model,
		execConfig,
		agent.SystemPrompt,
		allowedTools,
		allowedCommands,
		allowedTargets,
		agent.ApprovalMode,
		agent.MaxConcurrentRuns,
		agent.DefaultInvocationMode,
		agent.TokensUsedThisMonth,
		now,
		now,
	).Error; err != nil {
		t.Fatalf("seed agent row: %v", err)
	}
}

func TestCreateRunDelegatesMiraWorkspaceRunToAgentRuntime(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	service := (&AgentService{
		runRepo:            runRepo,
		agentRepo:          agentRepo,
		agentRuntimeClient: runtimeClient,
	}).SetAgentRuntimeLaunchEnabled(true)

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	prompt := "You are Mira."
	provider := "openai"
	modelName := "gpt-5.5"
	agent := &model.Agent{
		ID:                    "33333333-3333-3333-3333-333333333333",
		WorkspaceID:           workspaceID,
		Name:                  "Mira",
		PresetKey:             model.AgentPresetMarketer,
		RuntimeKind:           "native_sdk",
		Provider:              &provider,
		Model:                 &modelName,
		SystemPrompt:          &prompt,
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeInteractive,
		AllowedTargets:        json.RawMessage(`["workspace","document"]`),
		AllowedTools:          json.RawMessage(`["update_plan","request_user_input"]`),
		AllowedCommands:       json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{{Key: "marketing_context_setup"}},
		ExecutionConfig:       model.JSONBlob(`{"reasoning_effort":"medium"}`),
		MaxConcurrentRuns:     1,
	}
	seedCreateRunAgentRow(t, db, agent)
	additionalContext := "Prepare a short launch plan."
	payload, err := buildAgentRunInputPayload("workspace", workspaceID, manualRunTriggerContext(), nil, nil, &additionalContext, []string{"update_plan"})
	if err != nil {
		t.Fatalf("build payload: %v", err)
	}

	run, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "workspace",
		targetID:       workspaceID,
		actorID:        &actorID,
		input:          payload,
		invocationMode: model.InvocationModeInteractive,
	})
	if err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if len(runtimeClient.upsertAgents) != 1 {
		t.Fatalf("expected one runtime agent upsert, got %d", len(runtimeClient.upsertAgents))
	}
	upsert := runtimeClient.upsertAgents[0]
	if upsert.ID != agent.ID || upsert.AppID != "helpin" || upsert.Name != "Mira" || upsert.SystemPrompt != prompt {
		t.Fatalf("unexpected upserted agent: %#v", upsert)
	}
	if !slices.Equal(upsert.AllowedTargets, []string{"workspace", "document"}) || !slices.Equal(upsert.AllowedTools, []string{"update_plan", "request_user_input"}) {
		t.Fatalf("unexpected upserted permissions: targets=%#v tools=%#v", upsert.AllowedTargets, upsert.AllowedTools)
	}
	if len(upsert.Skills) != 1 || upsert.Skills[0].Key != "marketing_context_setup" {
		t.Fatalf("expected runtime skill refs, got %#v", upsert.Skills)
	}
	if len(runtimeClient.startRunCalls) != 1 {
		t.Fatalf("expected one runtime start call, got %d", len(runtimeClient.startRunCalls))
	}
	start := runtimeClient.startRunCalls[0]
	if start.HostRunID != run.ID || start.AgentID != agent.ID || start.Target.Type != "workspace" || start.Target.ID != workspaceID {
		t.Fatalf("unexpected runtime start request: %#v", start)
	}
	if start.ExternalActorID != actorID || start.Mode != model.InvocationModeInteractive || start.ExecutionMode != agentRuntimeExecutionModeDurable || start.Instructions != additionalContext {
		t.Fatalf("unexpected runtime start mode/actor/instructions: %#v", start)
	}
	if !slices.Equal(start.AllowedTools, []string{"update_plan"}) {
		t.Fatalf("unexpected start allowed tools: %#v", start.AllowedTools)
	}
	if start.TurnPolicy.Mode != agentRuntimeTurnCompleteOnFinish {
		t.Fatalf("expected completion turn policy, got %#v", start.TurnPolicy)
	}
	if start.Metadata["workspace_id"] != workspaceID || start.Metadata["helpin_run_id"] != run.ID || start.Target.Metadata["workspace_id"] != workspaceID {
		t.Fatalf("unexpected runtime metadata: metadata=%#v target=%#v", start.Metadata, start.Target.Metadata)
	}

	reloaded, err := runRepo.GetByID(ctx, workspaceID, run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.ExternalRuntime == nil || *reloaded.ExternalRuntime != agentRuntimeName || reloaded.ExternalRuntimeID == nil || *reloaded.ExternalRuntimeID != "run_runtime_1" {
		t.Fatalf("expected delegated runtime mapping, got external_runtime=%v external_runtime_id=%v", reloaded.ExternalRuntime, reloaded.ExternalRuntimeID)
	}
	if reloaded.WorkflowID != nil || reloaded.WorkflowRunID != nil {
		t.Fatalf("expected no Helpin Temporal workflow IDs, got %v/%v", reloaded.WorkflowID, reloaded.WorkflowRunID)
	}
}

func TestCreateRunRetriesDelegatedRuntimeStartOnce(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	runtimeClient := &fakeAgentRuntimeSignalClient{startRunErrs: []error{errors.New("runtime timeout")}}
	service := (&AgentService{
		runRepo:            runRepo,
		agentRepo:          agentRepo,
		agentRuntimeClient: runtimeClient,
	}).SetAgentRuntimeLaunchEnabled(true)

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	agent := &model.Agent{
		ID:                    "22222222-2222-2222-2222-222222222222",
		WorkspaceID:           workspaceID,
		Name:                  "Mira",
		PresetKey:             model.AgentPresetMarketer,
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
		AllowedTargets:        json.RawMessage(`["workspace"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		MaxConcurrentRuns:     1,
	}
	seedCreateRunAgentRow(t, db, agent)

	run, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "workspace",
		targetID:       workspaceID,
		input:          []byte(`{"target":{"target_type":"workspace","target_id":"11111111-1111-1111-1111-111111111111"}}`),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if len(runtimeClient.startRunCalls) != 2 {
		t.Fatalf("expected runtime start retry, got %d calls", len(runtimeClient.startRunCalls))
	}
	if runtimeClient.startRunCalls[0].HostRunID != run.ID || runtimeClient.startRunCalls[1].HostRunID != run.ID {
		t.Fatalf("retry should preserve host_run_id, calls=%#v", runtimeClient.startRunCalls)
	}
	reloaded, err := runRepo.GetByID(ctx, workspaceID, run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.ExternalRuntimeID == nil || *reloaded.ExternalRuntimeID != "run_runtime_1" || reloaded.Status != model.AgentRunStatusQueued {
		t.Fatalf("expected delegated mapping after retry, got %#v", reloaded)
	}
}

func TestCreateRunMarksDelegatedMiraRunFailedWhenRuntimeStartFails(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	runtimeClient := &fakeAgentRuntimeSignalClient{startRunErr: errors.New("runtime unavailable")}
	service := (&AgentService{
		runRepo:            runRepo,
		agentRepo:          agentRepo,
		agentRuntimeClient: runtimeClient,
	}).SetAgentRuntimeLaunchEnabled(true)

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	agent := &model.Agent{
		ID:                    "22222222-2222-2222-2222-222222222222",
		WorkspaceID:           workspaceID,
		Name:                  "Mira",
		PresetKey:             model.AgentPresetMarketer,
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
		AllowedTargets:        json.RawMessage(`["workspace"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		MaxConcurrentRuns:     1,
	}
	seedCreateRunAgentRow(t, db, agent)
	_, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "workspace",
		targetID:       workspaceID,
		input:          []byte(`{"target":{"target_type":"workspace","target_id":"11111111-1111-1111-1111-111111111111"}}`),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err == nil || !strings.Contains(err.Error(), "runtime unavailable") {
		t.Fatalf("expected runtime start error, got %v", err)
	}
	runs, total, err := runRepo.ListByWorkspace(ctx, workspaceID, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if total != 1 || len(runs) != 1 {
		t.Fatalf("expected one failed run, total=%d runs=%#v", total, runs)
	}
	failed := runs[0]
	if failed.Status != model.AgentRunStatusFailed || failed.ExecutionStage == nil || *failed.ExecutionStage != "failed_to_start" || failed.ErrorMessage == nil || !strings.Contains(*failed.ErrorMessage, "runtime unavailable") {
		t.Fatalf("unexpected failed run state: %#v", failed)
	}
	if failed.ExternalRuntime != nil || failed.ExternalRuntimeID != nil {
		t.Fatalf("failed runtime start should not stamp external mapping, got %v/%v", failed.ExternalRuntime, failed.ExternalRuntimeID)
	}
	var status string
	if err := db.WithContext(ctx).Raw("SELECT status FROM agents WHERE workspace_id = ? AND id = ?", workspaceID, agent.ID).Scan(&status).Error; err != nil {
		t.Fatalf("query agent status: %v", err)
	}
	if status != "idle" {
		t.Fatalf("expected agent to be idle after failed runtime start, got %q", status)
	}
}

func TestCreateRunDedupesSameAgentOnWorkspaceTarget(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	service := &AgentService{runRepo: runRepo, agentRepo: agentRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	agent := &model.Agent{
		ID:                    "22222222-2222-2222-2222-222222222222",
		WorkspaceID:           workspaceID,
		Name:                  "Competitors Changelog Tracking Report",
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	existingRunID := "33333333-3333-3333-3333-333333333333"
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             existingRunID,
		WorkspaceID:    workspaceID,
		AgentID:        agent.ID,
		TargetType:     "workspace",
		TargetID:       workspaceID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonHumanInput,
		Status:         model.AgentRunStatusPaused,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create existing workspace run: %v", err)
	}

	run, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "workspace",
		targetID:       workspaceID,
		input:          []byte("{}"),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err != nil {
		t.Fatalf("expected same workspace agent to reuse existing active run, got error %v", err)
	}
	if run == nil || run.ID != existingRunID {
		t.Fatalf("expected existing workspace run %q, got %#v", existingRunID, run)
	}
}

func TestCreateRunStillBlocksDifferentAgentsOnTaskTarget(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	service := &AgentService{runRepo: runRepo, agentRepo: agentRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	taskID := "22222222-2222-2222-2222-222222222222"
	nextAgent := &model.Agent{
		ID:                    "33333333-3333-3333-3333-333333333333",
		WorkspaceID:           workspaceID,
		Name:                  "Forge",
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             "44444444-4444-4444-4444-444444444444",
		WorkspaceID:    workspaceID,
		AgentID:        "55555555-5555-5555-5555-555555555555",
		TargetType:     "task",
		TargetID:       taskID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create existing task run: %v", err)
	}

	_, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          nextAgent,
		targetType:     "task",
		targetID:       taskID,
		taskID:         &taskID,
		input:          []byte("{}"),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err == nil || !strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected different task agent to remain blocked by duplicate-run guard, got %v", err)
	}
}
























func TestCRMEnrichmentToolsAreCommandAgentOnlyPresetTools(t *testing.T) {
	presets := ListAgentPresets()
	var commandAgent, crmOperator *model.AgentPresetDefinition
	for idx := range presets {
		switch presets[idx].Key {
		case model.AgentPresetCommandAgent:
			commandAgent = &presets[idx]
		case model.AgentPresetCRMOperator:
			crmOperator = &presets[idx]
		}
	}
	if commandAgent == nil || crmOperator == nil {
		t.Fatalf("missing command or CRM operator preset")
	}
	for _, tool := range []string{"ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company"} {
		if !slices.Contains(commandAgent.AllowedTools, tool) {
			t.Fatalf("expected Command Agent to allow %q", tool)
		}
		if slices.Contains(crmOperator.AllowedTools, tool) {
			t.Fatalf("did not expect CRM Operator to allow %q in first slice", tool)
		}
	}
	for _, tool := range []string{"web_search_exa", "fetch_url", "read_document", "get_document_blocks", "write_document_content", "update_document_block", "link_document_to_object"} {
		if !slices.Contains(commandAgent.AllowedTools, tool) {
			t.Fatalf("expected Command Agent to allow document one-shot tool %q", tool)
		}
	}
}





func TestValidateDispatchStepsRequiresOneShotKindForCommandAgent(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	service := &CommandBarService{agentService: &AgentService{agentRepo: agentRepo}}
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	agentID := "22222222-2222-2222-2222-222222222222"

	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, status, runtime_kind,
		allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, 1, 'Command Agent', ?, 'idle', 'native_sdk', ?, ?, ?, 'never', 1, 'interactive', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		agentID,
		workspaceID,
		model.AgentPresetCommandAgent,
		[]byte(`["read_document","write_document_content"]`),
		[]byte(`[]`),
		[]byte(`["document"]`),
	).Error; err != nil {
		t.Fatalf("seed command agent: %v", err)
	}

	target := model.CommandBarPageContext{EntityType: "document", EntityID: "33333333-3333-3333-3333-333333333333"}
	err := service.validateDispatchSteps(ctx, workspaceID, []model.CommandBarPlanStep{{
		AgentID:      agentID,
		Target:       target,
		Instructions: "Update this doc.",
		AllowedTools: []string{"read_document"},
	}})
	if err == nil || !strings.Contains(err.Error(), "must be dispatched as a one-shot command") {
		t.Fatalf("expected one-shot kind validation error, got %v", err)
	}

	err = service.validateDispatchSteps(ctx, workspaceID, []model.CommandBarPlanStep{{
		AgentID:      agentID,
		PlanKind:     model.CommandBarPlanKindOneShotCommand,
		Target:       target,
		Instructions: "Update this doc.",
		AllowedTools: []string{"read_document"},
	}})
	if err != nil {
		t.Fatalf("expected one-shot command step to validate: %v", err)
	}
}

func TestAdvanceCommandBarPlanMarksFailedRunPlanFailed(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &AgentService{runRepo: runRepo, commandBarPlanRepo: planRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	runID := "33333333-3333-3333-3333-333333333333"
	agentID := "44444444-4444-4444-4444-444444444444"
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
	plan.CurrentStepIndex = 0
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	trigger, err := buildCommandBarTriggerContext("run forge and lens", pageContext, steps, 0, planID)
	if err != nil {
		t.Fatalf("build trigger: %v", err)
	}
	input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	errMsg := "forge failed"
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
		Status:         model.AgentRunStatusFailed,
		Input:          input,
		OutputSummary:  json.RawMessage("{}"),
		ErrorMessage:   &errMsg,
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}

	nextRun, err := service.AdvanceCommandBarPlanAfterRun(ctx, runID)
	if err != nil {
		t.Fatalf("advance failed run: %v", err)
	}
	if nextRun != nil {
		t.Fatalf("expected no next run after failed step, got %#v", nextRun)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusFailed {
		t.Fatalf("expected failed plan status, got %q", updated.Status)
	}
	if updated.ErrorMessage == nil || *updated.ErrorMessage != errMsg {
		t.Fatalf("expected plan error %q, got %#v", errMsg, updated.ErrorMessage)
	}
}

func TestAdvanceFanOutCommandBarPlanWaitsForAllRuns(t *testing.T) {
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
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create first run: %v", err)
	}
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
		Status:         model.AgentRunStatusCompleted,
		Input:          input,
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create second run: %v", err)
	}

	nextRun, err := service.AdvanceCommandBarPlanAfterRun(ctx, runTwoID)
	if err != nil {
		t.Fatalf("advance fan-out run: %v", err)
	}
	if nextRun != nil {
		t.Fatalf("expected no sequential next run for fan-out, got %#v", nextRun)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusRunning {
		t.Fatalf("expected plan to wait for active fan-out run, got %q", updated.Status)
	}

	trigger, err = buildCommandBarTriggerContext("run lens across all child tasks", pageContext, steps, 0, planID)
	if err != nil {
		t.Fatalf("build second trigger: %v", err)
	}
	input, _ = json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	if err := db.Model(&model.AgentRun{}).
		Where("id = ?", runOneID).
		Updates(map[string]any{"status": model.AgentRunStatusCompleted, "input": input}).Error; err != nil {
		t.Fatalf("complete first run: %v", err)
	}
	if _, err := service.AdvanceCommandBarPlanAfterRun(ctx, runOneID); err != nil {
		t.Fatalf("advance final fan-out run: %v", err)
	}
	updated, err = planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get completed plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusCompleted {
		t.Fatalf("expected completed plan after all fan-out runs complete, got %q", updated.Status)
	}
}

func TestAdvanceTaskPipelinePlanSchedulesFallbackWithoutRunEngine(t *testing.T) {
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
			AgentName:    "Forge",
			PlanKind:     model.CommandBarPlanKindTaskPipeline,
			Target:       model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
			Instructions: "Build task 1.",
		},
		{
			AgentID:              agentID,
			AgentName:            "Lens",
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			Target:               model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
			Instructions:         "Review task 1.",
			DependsOnStepIndexes: []int{0},
		},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run forge then lens", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: runOneID, 1: runTwoID})
	plan.RunIDsByStep = runIDs
	plan.RunCount = 2
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	for index, runID := range []string{runOneID, runTwoID} {
		trigger, err := buildCommandBarTriggerContext("run forge then lens", pageContext, steps, index, planID)
		if err != nil {
			t.Fatalf("build trigger %d: %v", index, err)
		}
		input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
		if err := runRepo.Create(ctx, &model.AgentRun{
			ID:             runID,
			WorkspaceID:    workspaceID,
			AgentID:        agentID,
			TargetType:     "task",
			TargetID:       "task-1",
			RuntimeKind:    "native_sdk",
			InvocationMode: model.InvocationModeAutonomous,
			ApprovalState:  "not_required",
			PauseReason:    model.AgentRunPauseReasonNone,
			Status:         model.AgentRunStatusCompleted,
			Input:          input,
			OutputSummary:  json.RawMessage("{}"),
		}); err != nil {
			t.Fatalf("create run %d: %v", index, err)
		}
	}

	if _, err := service.AdvanceCommandBarPlanAfterRun(ctx, runTwoID); err != nil {
		t.Fatalf("advance task pipeline run: %v", err)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusCompleted {
		t.Fatalf("expected direct fallback to complete task pipeline plan, got %q", updated.Status)
	}
}

func TestAdvanceTaskPipelinePlanCompletesWhenAllRunsTerminal(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &AgentService{
		runRepo:            runRepo,
		commandBarPlanRepo: planRepo,
	}

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
			AgentName:    "Forge",
			PlanKind:     model.CommandBarPlanKindTaskPipeline,
			Target:       model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
			Instructions: "Build task 1.",
		},
		{
			AgentID:              agentID,
			AgentName:            "Lens",
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			Target:               model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
			Instructions:         "Review task 1.",
			DependsOnStepIndexes: []int{0},
		},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run forge then lens", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: runOneID, 1: runTwoID})
	plan.RunIDsByStep = runIDs
	plan.RunCount = 2
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	for index, runID := range []string{runOneID, runTwoID} {
		trigger, err := buildCommandBarTriggerContext("run forge then lens", pageContext, steps, index, planID)
		if err != nil {
			t.Fatalf("build trigger %d: %v", index, err)
		}
		input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
		if err := runRepo.Create(ctx, &model.AgentRun{
			ID:             runID,
			WorkspaceID:    workspaceID,
			AgentID:        agentID,
			TargetType:     "task",
			TargetID:       "task-1",
			RuntimeKind:    "native_sdk",
			InvocationMode: model.InvocationModeAutonomous,
			ApprovalState:  "not_required",
			PauseReason:    model.AgentRunPauseReasonNone,
			Status:         model.AgentRunStatusCompleted,
			Input:          input,
			OutputSummary:  json.RawMessage("{}"),
		}); err != nil {
			t.Fatalf("create run %d: %v", index, err)
		}
	}

	if _, err := service.AdvanceCommandBarPlanAfterRun(ctx, runTwoID); err != nil {
		t.Fatalf("advance task pipeline run: %v", err)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusCompleted {
		t.Fatalf("expected local advancement to complete task pipeline plan, got %q", updated.Status)
	}
}

func TestDecodeCommandBarPlanRunIDsUsesStringKeys(t *testing.T) {
	raw := json.RawMessage(`{"0":"run-0","2":"run-2","bad":"ignored"}`)
	got := decodeCommandBarPlanRunIDs(raw)
	if got[0] != "run-0" || got[2] != "run-2" {
		t.Fatalf("unexpected decoded run ids: %#v", got)
	}
	if _, ok := got[1]; ok {
		t.Fatalf("did not expect step 1 in %#v", got)
	}
}


func TestCommandBarPlanOwnedByActor(t *testing.T) {
	actorID := "11111111-1111-1111-1111-111111111111"
	otherID := "22222222-2222-2222-2222-222222222222"
	if !commandBarPlanOwnedByActor(&model.CommandBarPlanRecord{ActorID: &actorID}, actorID) {
		t.Fatal("expected owner to access plan")
	}
	if commandBarPlanOwnedByActor(&model.CommandBarPlanRecord{ActorID: &otherID}, actorID) {
		t.Fatal("expected other actor to be denied")
	}
}


func TestValidatePromotedAgentTargetsRejectsOutsideSourceAllowlist(t *testing.T) {
	sourceAgent := &model.Agent{
		Name:           "Command Agent",
		AllowedTargets: json.RawMessage(`["document"]`),
	}
	if err := validatePromotedAgentTargets([]string{"document"}, sourceAgent); err != nil {
		t.Fatalf("expected document target to be accepted: %v", err)
	}
	if err := validatePromotedAgentTargets([]string{"crm_deal"}, sourceAgent); err == nil {
		t.Fatal("expected crm_deal target to be rejected")
	}
}




// The retry handler pre-fetches the plan for per-step authorization; that
// lookup must not be owner-gated or cross-actor retries from the epic page
// die with "not found" before the team-actionable service method runs.
func TestGetWorkspacePlanBypassesOwnerGate(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &CommandBarService{
		planRepo:     planRepo,
		agentService: &AgentService{runRepo: repository.NewAgentRunRepository(db)},
	}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "66666666-6666-6666-6666-666666666666"
	pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: "epic-1", DisplayTitle: "Epic"}
	steps := []model.CommandBarPlanStep{{AgentID: "agent-1", AgentName: "Forge", Target: pageContext, Instructions: "Build."}}
	plan, err := newCommandBarPlanRecord(workspaceID, "actor-owner", planID, "run all tasks", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	if _, err := service.GetPlan(ctx, workspaceID, "actor-other", planID); err == nil {
		t.Fatalf("expected owner-gated GetPlan to hide another actor's plan")
	}
	detail, err := service.GetWorkspacePlan(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("GetWorkspacePlan: %v", err)
	}
	if detail.Plan.ID != planID {
		t.Fatalf("expected plan %q, got %q", planID, detail.Plan.ID)
	}
}

// A plan can be left status "running" with all child runs failed/cancelled —
// a zombie that resume cannot revive. Retry must accept it (rejecting only
// while runs are genuinely active).
func TestRetryPlanFromStepAcceptsRunningPlanWithNoActiveRuns(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &CommandBarService{
		planRepo:     planRepo,
		agentService: &AgentService{runRepo: runRepo, commandBarPlanRepo: planRepo},
	}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	runID := "33333333-3333-3333-3333-333333333333"
	agentID := "44444444-4444-4444-4444-444444444444"
	targetID := "55555555-5555-5555-5555-555555555555"
	pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: targetID, DisplayTitle: "Epic"}
	steps := []model.CommandBarPlanStep{
		{
			AgentID:      agentID,
			AgentName:    "Forge",
			PlanKind:     model.CommandBarPlanKindTaskPipeline,
			Target:       pageContext,
			Instructions: "Build it.",
		},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run all tasks", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: runID})
	plan.RunIDsByStep = runIDs
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if plan.Status != model.CommandBarPlanStatusRunning {
		t.Fatalf("expected plan to start running, got %q", plan.Status)
	}
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             runID,
		WorkspaceID:    workspaceID,
		AgentID:        agentID,
		TargetType:     "epic",
		TargetID:       targetID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusCancelled,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}

	// Zombie (running plan, cancelled run): must pass the running gate and
	// only fail later in the retry pipeline in this partial test harness.
	_, err = service.RetryPlanFromStep(ctx, workspaceID, "actor-2", planID, model.CommandBarRetryPlanRequest{StepIndex: 0})
	if err != nil && strings.Contains(err.Error(), "active runs") {
		t.Fatalf("expected zombie plan to pass the running gate, got %v", err)
	}

	// Genuinely active run: retry must be rejected.
	if err := db.Exec(`UPDATE agent_runs SET status = 'running' WHERE id = ?`, runID).Error; err != nil {
		t.Fatalf("activate run: %v", err)
	}
	_, err = service.RetryPlanFromStep(ctx, workspaceID, "actor-2", planID, model.CommandBarRetryPlanRequest{StepIndex: 0})
	if err == nil || !strings.Contains(err.Error(), "active runs") {
		t.Fatalf("expected active-run rejection, got %v", err)
	}
}

func setupCommandBarPlanTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:command_bar_plan_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	for _, stmt := range []string{
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'task',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			parent_run_id TEXT,
			dock_chat_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			workflow_id TEXT,
			workflow_run_id TEXT,
			external_runtime TEXT,
			external_runtime_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			agent_version_id TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input TEXT NOT NULL DEFAULT '{}',
			output_summary TEXT NOT NULL DEFAULT '{}',
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE command_bar_plans (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			parent_chat_run_id TEXT,
			dock_chat_id TEXT,
			parent_notified_at DATETIME,
			status TEXT NOT NULL DEFAULT 'running',
			prompt TEXT NOT NULL,
			page_context TEXT NOT NULL DEFAULT '{}',
			steps TEXT NOT NULL DEFAULT '[]',
			run_ids_by_step TEXT NOT NULL DEFAULT '{}',
			current_step_index INTEGER NOT NULL DEFAULT 0,
			run_count INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			cancelled_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE command_bar_threads (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE command_bar_messages (
			id TEXT PRIMARY KEY,
			thread_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			page_context TEXT,
			proposal_json TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			icon_key TEXT NOT NULL DEFAULT '',
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			source_template_id TEXT,
			source_template_key TEXT,
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
		active_version_id TEXT,
			role TEXT,
			status TEXT NOT NULL DEFAULT 'idle',
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			skills BLOB NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL DEFAULT 'manual',
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT '{}',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			allowed_commands BLOB NOT NULL DEFAULT '[]',
			allowed_targets BLOB NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create command bar test table: %v", err)
		}
	}
	return db
}

func createCommandBarChatTablesForTest(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, stmt := range []string{
		`CREATE TABLE command_bar_threads (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE command_bar_messages (
			id TEXT PRIMARY KEY,
			thread_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			page_context TEXT,
			proposal_json TEXT,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create command bar chat test table: %v", err)
		}
	}
}


