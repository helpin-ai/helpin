package service

import (
	"context"
	"encoding/json"
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
	response string
	err      error
	requests []llm.ChatRequest
}

func (s *scriptedCommandBarLLM) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	s.requests = append(s.requests, req)
	if s.err != nil {
		return nil, s.err
	}
	return &llm.ChatResponse{Content: s.response}, nil
}

func TestParseExplicitNamedAgentsPreservesRequestOrder(t *testing.T) {
	pageContext := model.CommandBarPageContext{
		EntityType:   "task",
		EntityID:     "task-1",
		DisplayTitle: "Task 1",
	}
	candidates := []model.CommandBarAgent{
		{ID: "agent-forge", Name: "Forge", PresetKey: model.AgentPresetCodeBuilder, AllowedTargets: []string{"task"}},
		{ID: "agent-lens", Name: "Lens", PresetKey: model.AgentPresetReviewAgent, AllowedTargets: []string{"task"}},
	}

	resp := parseExplicitNamedAgents("run forge and then lens", pageContext, candidates)
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil {
		t.Fatalf("expected plan response, got %#v", resp)
	}
	if resp.Plan.RunCount != 2 {
		t.Fatalf("expected run count 2, got %d", resp.Plan.RunCount)
	}
	if got := resp.Plan.Steps[0].AgentName; got != "Forge" {
		t.Fatalf("expected first step Forge, got %q", got)
	}
	if got := resp.Plan.Steps[1].AgentName; got != "Lens" {
		t.Fatalf("expected second step Lens, got %q", got)
	}
	if got := resp.Plan.Steps[0].Instructions; strings.Contains(strings.ToLower(got), "run forge and then lens") {
		t.Fatalf("expected step-scoped Forge instructions, got %q", got)
	}
	if got := resp.Plan.Steps[0].Instructions; !strings.Contains(got, "Do not invoke or run Lens") {
		t.Fatalf("expected Forge step to leave Lens to scheduler, got %q", got)
	}
}

func TestParseExplicitNamedAgentsUsesWholeWords(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "task", EntityID: "task-1"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-lens", Name: "Lens", PresetKey: model.AgentPresetReviewAgent, AllowedTargets: []string{"task"}},
	}

	if resp := parseExplicitNamedAgents("check camera lenses", pageContext, candidates); resp != nil {
		t.Fatalf("expected no explicit agent match, got %#v", resp)
	}
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

func TestParseOneShotCommandIntentForDocumentUpdate(t *testing.T) {
	pageContext := model.CommandBarPageContext{
		EntityType:   "document",
		EntityID:     "doc-1",
		DisplayTitle: "Setup guide",
	}
	candidates := []model.CommandBarAgent{
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"document"},
			AllowedTools:   []string{"update_plan", "request_user_input", "request_approval", "web_search_exa", "web_search_brave", "fetch_url", "crawl_url", "list_documents", "list_collections", "read_document", "search_documents", "write_document_content", "create_document"},
		},
	}

	resp := parseOneShotCommandIntent("check the web and update stale doc sections", pageContext, candidates)
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil {
		t.Fatalf("expected one-shot command plan, got %#v", resp)
	}
	if resp.Plan.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected one-shot plan kind, got %q", resp.Plan.PlanKind)
	}
	step := resp.Plan.Steps[0]
	if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected one-shot step kind, got %q", step.PlanKind)
	}
	for _, required := range []string{"web_search_exa", "read_document", "write_document_content"} {
		if !slices.Contains(step.AllowedTools, required) {
			t.Fatalf("expected tool %q in %#v", required, step.AllowedTools)
		}
	}
	if !strings.Contains(step.Instructions, "Do not create or save a reusable agent") {
		t.Fatalf("expected one-shot instruction guardrail, got %q", step.Instructions)
	}
	if !strings.Contains(step.Instructions, "Goal:") || !strings.Contains(step.Instructions, "Plan:") || !strings.Contains(step.Instructions, "Constraints:") {
		t.Fatalf("expected structured execution brief, got %q", step.Instructions)
	}
	if !strings.Contains(step.Instructions, "Read the current document") {
		t.Fatalf("expected document-specific execution plan, got %q", step.Instructions)
	}
}

func TestParseIntentDeterministicallyPrefersKnownAgentBeforeOneShot(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-task", Name: "Task Planner", PresetKey: model.AgentPresetTaskPlanner, AllowedTargets: []string{"workspace"}},
		{ID: "agent-command", Name: "Command Agent", PresetKey: model.AgentPresetCommandAgent, AllowedTargets: []string{"workspace"}},
	}

	resp := parseIntentDeterministically("break down this initiative into tasks", pageContext, commandBarNarrowCandidates(candidates))
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected known-agent plan, got %#v", resp)
	}
	if got := resp.Plan.Steps[0].AgentID; got != "agent-task" {
		t.Fatalf("expected task planner, got %q", got)
	}
}

func TestWorkspaceTaskQuestionPrefersOneShotCommandAgent(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-task", Name: "Atlas", PresetKey: model.AgentPresetTaskPlanner, AllowedTargets: []string{"workspace"}},
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"workspace"},
			AllowedTools:   []string{"update_plan", "request_user_input", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks", "get_task_context"},
		},
	}

	resp := parsePreferredOneShotCommandIntent("how many tasks in engineering team needs attention?", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot command plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.AgentID != "agent-command" {
		t.Fatalf("expected Command Agent, got %q", step.AgentID)
	}
	if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected one-shot plan kind, got %q", step.PlanKind)
	}
	for _, required := range []string{"list_workspace_teams", "list_team_workflows_with_stages", "list_tasks"} {
		if !slices.Contains(step.AllowedTools, required) {
			t.Fatalf("expected tool %q in %#v", required, step.AllowedTools)
		}
	}
	if strings.Contains(strings.ToLower(strings.Join(step.AllowedTools, ",")), "create_task") {
		t.Fatalf("did not expect create_task for read-only task question: %#v", step.AllowedTools)
	}
	if !strings.Contains(step.Instructions, "Do not create, update, or move tasks") {
		t.Fatalf("expected read-only PM analysis guardrail, got %q", step.Instructions)
	}
}

func TestParseIntentWithLLMRoutesOneShotAndNarrowsTools(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-atlas", Name: "Atlas", Description: "Task planning agent", PresetKey: model.AgentPresetTaskPlanner, AllowedTargets: []string{"workspace"}},
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			Description:    "One-shot workspace operator",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"workspace"},
			AllowedTools:   []string{"update_plan", "request_user_input", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks", "create_task"},
		},
	}
	fakeLLM := &scriptedCommandBarLLM{response: `{
		"status":"plan",
		"route_kind":"one_shot_command",
		"agent_id":"agent-command",
		"instructions":"Count engineering tasks that need attention.",
		"one_shot_tools":["list_workspace_teams","list_tasks","create_task"],
		"rationale":"This is an ad hoc data question, not planning.",
		"confidence":0.91
	}`}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetLLMRouterConfig("openai", "gpt-5.5", 777, time.Second)

	resp := service.parseIntentWithLLM(context.Background(), "how many tasks in engineering team needs attention?", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.AgentID != "agent-command" || step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected command one-shot step, got %#v", step)
	}
	if !slices.Contains(step.AllowedTools, "list_tasks") || !slices.Contains(step.AllowedTools, "list_workspace_teams") {
		t.Fatalf("expected read tools, got %#v", step.AllowedTools)
	}
	if slices.Contains(step.AllowedTools, "create_task") {
		t.Fatalf("did not expect mutation tool for read-only question, got %#v", step.AllowedTools)
	}
	if len(fakeLLM.requests) != 1 {
		t.Fatalf("expected one LLM request, got %d", len(fakeLLM.requests))
	}
	if got := fakeLLM.requests[0].Provider; got != "openai" {
		t.Fatalf("expected provider openai, got %q", got)
	}
	if got := fakeLLM.requests[0].Model; got != "gpt-5.5" {
		t.Fatalf("expected model gpt-5.5, got %q", got)
	}
	if got := fakeLLM.requests[0].MaxTokens; got != 777 {
		t.Fatalf("expected max tokens 777, got %d", got)
	}
	if fakeLLM.requests[0].JSONSchema == nil {
		t.Fatalf("expected command router JSON schema for schema-forced providers")
	}
}

func TestParseIntentWithLLMRoutesMultiStepSavedAgents(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "task", EntityID: "task-1"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-forge", Name: "Forge", PresetKey: model.AgentPresetCodeBuilder, AllowedTargets: []string{"task"}},
		{ID: "agent-lens", Name: "Lens", PresetKey: model.AgentPresetReviewAgent, AllowedTargets: []string{"task"}},
		{ID: "agent-command", Name: "Command Agent", PresetKey: model.AgentPresetCommandAgent, AllowedTargets: []string{"task"}, AllowedTools: []string{"get_task_context"}},
	}
	fakeLLM := &scriptedCommandBarLLM{response: `{
		"status":"plan",
		"route_kind":"multi_step",
		"steps":[
			{"agent_id":"agent-forge","instructions":"Implement the requested task."},
			{"agent_id":"agent-lens","instructions":"Review Forge's result."}
		],
		"rationale":"The user asked for implementation followed by review.",
		"confidence":0.93
	}`}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM)

	resp := service.parseIntentWithLLM(context.Background(), "have Forge implement then Lens review", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 2 {
		t.Fatalf("expected two-step saved-agent plan, got %#v", resp)
	}
	if resp.Plan.Steps[0].AgentID != "agent-forge" || resp.Plan.Steps[1].AgentID != "agent-lens" {
		t.Fatalf("unexpected step order: %#v", resp.Plan.Steps)
	}
	if resp.Plan.PlanKind != model.CommandBarPlanKindKnownAgent {
		t.Fatalf("expected known-agent plan kind, got %q", resp.Plan.PlanKind)
	}
}

func TestParseOneShotCommandIntentRejectsUnsupportedMutation(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"workspace"},
			AllowedTools:   []string{"update_plan", "request_user_input"},
		},
	}

	if resp := parseOneShotCommandIntent("delete workspace", pageContext, candidates); resp != nil {
		t.Fatalf("expected unsupported mutation to stay unmatched, got %#v", resp)
	}
}

func TestCRMResearchUpdatePrefersOneShotCommandAgent(t *testing.T) {
	pageContext := model.CommandBarPageContext{
		EntityType:   "crm_contact",
		EntityID:     "contact-1",
		DisplayTitle: "Ada Lovelace",
	}
	candidates := []model.CommandBarAgent{
		{
			ID:             "agent-crm",
			Name:           "CRM Operator",
			PresetKey:      model.AgentPresetCRMOperator,
			AllowedTargets: []string{"crm_contact"},
			AllowedTools:   []string{"list_deals", "list_contacts", "list_buyer_signals"},
		},
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"crm_contact"},
			AllowedTools:   []string{"update_plan", "request_user_input", "request_approval", "web_search_exa", "web_search_brave", "fetch_url", "crawl_url", "list_deals", "list_contacts", "list_buyer_signals", "ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company"},
		},
	}

	resp := parsePreferredOneShotCommandIntent("find info about this contact and update contact and company", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot command plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.AgentID != "agent-command" {
		t.Fatalf("expected Command Agent, got %q", step.AgentID)
	}
	if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected one-shot plan kind, got %q", step.PlanKind)
	}
	for _, required := range []string{"web_search_exa", "fetch_url", "list_contacts", "request_approval", "ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company"} {
		if !slices.Contains(step.AllowedTools, required) {
			t.Fatalf("expected tool %q in %#v", required, step.AllowedTools)
		}
	}
	if !strings.Contains(step.Instructions, "Research the CRM target") || !strings.Contains(step.Instructions, "protected server-side") {
		t.Fatalf("expected CRM-specific execution brief, got %q", step.Instructions)
	}

	if deterministic := parseIntentDeterministically("find info about this contact and update contact and company", pageContext, commandBarNarrowCandidates(candidates)); deterministic == nil || deterministic.Plan.Steps[0].AgentID != "agent-crm" {
		t.Fatalf("expected deterministic fallback alone to choose CRM Operator, got %#v", deterministic)
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
}

func TestParseFanOutIntentBuildsConcreteTargetPlan(t *testing.T) {
	pageContext := model.CommandBarPageContext{
		EntityType: "epic",
		EntityID:   "epic-1",
		RelatedIDs: map[string][]string{
			"task_ids": {"task-1", "task-2", "task-3"},
		},
	}
	candidates := []model.CommandBarAgent{
		{ID: "agent-lens", Name: "Lens", PresetKey: model.AgentPresetReviewAgent, AllowedTargets: []string{"task"}},
	}

	resp := parseFanOutIntent("run lens across all child tasks", pageContext, candidates, candidates)
	if resp == nil || resp.Plan == nil {
		t.Fatalf("expected fan-out plan, got %#v", resp)
	}
	if resp.Plan.PlanKind != model.CommandBarPlanKindFanOut {
		t.Fatalf("expected fan-out plan kind, got %q", resp.Plan.PlanKind)
	}
	if resp.Plan.RunCount != 3 {
		t.Fatalf("expected 3 fan-out runs, got %d", resp.Plan.RunCount)
	}
	for i, step := range resp.Plan.Steps {
		if step.PlanKind != model.CommandBarPlanKindFanOut {
			t.Fatalf("expected step %d fan-out kind, got %q", i, step.PlanKind)
		}
		if step.Target.EntityType != "task" {
			t.Fatalf("expected task target, got %#v", step.Target)
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

func TestCommandBarUnmetIntentSummaryRedactsPromptByDefault(t *testing.T) {
	pageContext, _ := json.Marshal(model.CommandBarPageContext{EntityType: "document", EntityID: "doc-1"})
	candidates, _ := json.Marshal([]model.CommandBarAgent{{ID: "agent-1", Name: "Command Agent"}})
	intent := model.CommandBarUnmetIntent{
		ID:              "intent-1",
		WorkspaceID:     "workspace-1",
		Prompt:          "check the web and update stale doc sections with sensitive customer details",
		PageContext:     pageContext,
		CandidateAgents: candidates,
		Reason:          "No matching agent.",
		Status:          "open",
	}

	summary := commandBarUnmetIntentSummary(intent, false)
	if summary.Prompt != "" || !summary.PromptRedacted {
		t.Fatalf("expected redacted prompt, got prompt=%q redacted=%v", summary.Prompt, summary.PromptRedacted)
	}
	if summary.PromptPreview == "" || !strings.Contains(summary.PromptPreview, "check the web") {
		t.Fatalf("expected useful prompt preview, got %q", summary.PromptPreview)
	}
	if summary.PageContext.EntityType != "document" || len(summary.CandidateAgents) != 1 {
		t.Fatalf("expected decoded context and candidates, got %#v", summary)
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
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			workflow_id TEXT,
			workflow_run_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
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
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			source_template_id TEXT,
			source_template_key TEXT,
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
