package service

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

func TestCreateAgentDefaultsToCodeBuilderPreset(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	created, err := svc.CreateAgent(context.Background(), modelCreateAgentRequest(nil), "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	if created.PresetKey != "" {
		t.Fatalf("expected custom agent preset_key to remain empty, got %q", created.PresetKey)
	}
	if created.PresetVersionKey != "" {
		t.Fatalf("expected custom agent preset_version_key to remain empty, got %q", created.PresetVersionKey)
	}
	if created.SourcePresetKey != "" {
		t.Fatalf("expected custom agent source preset to remain empty, got %q", created.SourcePresetKey)
	}
	if created.SourcePresetVersionKey != "" {
		t.Fatalf("expected custom agent source preset version to remain empty, got %q", created.SourcePresetVersionKey)
	}
	if created.Role != "Custom Agent" {
		t.Fatalf("expected default role Custom Agent, got %q", created.Role)
	}
	if created.RuntimeKind != "opencode" {
		t.Fatalf("expected default runtime opencode, got %q", created.RuntimeKind)
	}
}

func TestCreatePlannerPersistsDefaultSystemPrompt(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	req := modelCreateAgentRequest(nil)
	req.SystemPrompt = agentTestStringPtr("Plan carefully and ask follow-up questions when needed.")
	created, err := svc.CreateAgent(context.Background(), req, "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	if created.SystemPrompt == nil || *created.SystemPrompt != "Plan carefully and ask follow-up questions when needed." {
		t.Fatalf("expected custom system prompt to be persisted, got %+v", created.SystemPrompt)
	}
	if created.PlanningNotes != nil {
		t.Fatalf("expected planner planning notes to be empty after create, got %+v", created.PlanningNotes)
	}
	if created.PresetKey != "" {
		t.Fatalf("expected custom agent preset_key to remain empty, got %q", created.PresetKey)
	}
	if created.PresetVersionKey != "" {
		t.Fatalf("expected custom agent preset_version_key to remain empty, got %q", created.PresetVersionKey)
	}
	if created.SourcePresetKey != "" {
		t.Fatalf("expected custom agent source preset to remain empty, got %q", created.SourcePresetKey)
	}
	if created.SourcePresetVersionKey != "" {
		t.Fatalf("expected custom agent source preset version to remain empty, got %q", created.SourcePresetVersionKey)
	}
}

func TestEnsureSystemProductPlannerAgentRefreshesLegacyPrompt(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	now := time.Now().UTC()
	legacyPrompt := "1. `prd_draft`\n2. `awaiting_prd_approval`\n3. `persist_prd`\n4. `story_plan`\n5. `awaiting_story_approval`\n6. `create_stories`"
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, system_prompt, allowed_tools, allowed_commands, allowed_targets,
		approval_mode, max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-system", "ws-test", true, "Epic Planner", model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", legacyPrompt, []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	).Error; err != nil {
		t.Fatalf("insert system agent: %v", err)
	}

	updated, err := svc.ensureSystemProductPlannerAgent(context.Background(), "ws-test", "user-1")
	if err != nil {
		t.Fatalf("ensureSystemProductPlannerAgent returned error: %v", err)
	}
	if updated.SystemPrompt == nil || *updated.SystemPrompt == "" {
		t.Fatal("expected refreshed system prompt")
	}
	if strings.Contains(*updated.SystemPrompt, "awaiting_prd_approval") || strings.Contains(*updated.SystemPrompt, "awaiting_story_approval") {
		t.Fatalf("expected refreshed prompt to remove legacy approval phases, got %q", *updated.SystemPrompt)
	}
	if !strings.Contains(*updated.SystemPrompt, "There is no hidden planner phase machine deciding the next step for you.") {
		t.Fatalf("expected refreshed prompt to include inline approval guidance, got %q", *updated.SystemPrompt)
	}
	if updated.Name != defaultSystemEpicPlannerName {
		t.Fatalf("expected renamed system planner %q, got %q", defaultSystemEpicPlannerName, updated.Name)
	}
}

func TestEnsureBuiltInTaskPlannerRefreshesLegacyPrompt(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	now := time.Now().UTC()
	legacyPrompt := "Use `publish_preview` and `request_human_approval` once the runtime tells you the current planner phase."
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, system_prompt, allowed_tools, allowed_commands, allowed_targets,
		approval_mode, max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-task-planner", "ws-test", true, "Task Planner", model.AgentPresetTaskPlanner, "Task Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", legacyPrompt, []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	).Error; err != nil {
		t.Fatalf("insert task planner system agent: %v", err)
	}

	updated, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetTaskPlanner)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if updated.SystemPrompt == nil || *updated.SystemPrompt == "" {
		t.Fatal("expected refreshed task planner prompt")
	}
	if strings.Contains(*updated.SystemPrompt, "`publish_preview`") || strings.Contains(*updated.SystemPrompt, "`request_human_approval`") {
		t.Fatalf("expected refreshed task planner prompt to remove legacy preview/approval tools, got %q", *updated.SystemPrompt)
	}
	if !strings.Contains(*updated.SystemPrompt, "`publish_task_plan_doc`") {
		t.Fatalf("expected refreshed task planner prompt to include publish_task_plan_doc, got %q", *updated.SystemPrompt)
	}
	if updated.Name != "Scribe" {
		t.Fatalf("expected renamed task planner %q, got %q", "Scribe", updated.Name)
	}
}

func TestEnsureBuiltInReviewAgentRefreshesPromptVersionAndTools(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	now := time.Now().UTC()
	legacyPrompt := "You are Review Agent.\n\n- Inspect the relevant code and run targeted validation when possible.\n- Focus on correctness, regressions, missing tests, and delivery risk.\n- Report findings first, ordered by severity, with concrete file references when available.\n- Avoid low-signal commentary and avoid proposing unnecessary rewrites."
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, preset_version_key, role, status, runtime_kind,
		skills, trigger_mode, system_prompt, allowed_tools, allowed_commands, allowed_targets,
		approval_mode, max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-reviewer", "ws-test", true, "Lens", model.AgentPresetReviewAgent, "review_agent_default", "Review Agent", "idle", "codex",
		[]byte("[]"), "manual", legacyPrompt, mustJSONStringSlice([]string{"read_file", "run_command"}), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeAutonomous, now, now,
	).Error; err != nil {
		t.Fatalf("insert review agent: %v", err)
	}

	updated, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetReviewAgent)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if updated.PresetVersionKey != defaultPresetVersionKeyForPresetKey(model.AgentPresetReviewAgent) {
		t.Fatalf("expected review preset version %q, got %q", defaultPresetVersionKeyForPresetKey(model.AgentPresetReviewAgent), updated.PresetVersionKey)
	}
	if updated.SystemPrompt == nil || !strings.Contains(*updated.SystemPrompt, "`request_user_input`") {
		t.Fatalf("expected refreshed review prompt with request_user_input, got %+v", updated.SystemPrompt)
	}
	var tools []string
	if err := json.Unmarshal(updated.AllowedTools, &tools); err != nil {
		t.Fatalf("unmarshal allowed tools: %v", err)
	}
	if !slices.Contains(tools, worker.ToolRequestUserInput) {
		t.Fatalf("expected review agent tools to include %q, got %v", worker.ToolRequestUserInput, tools)
	}
}

func TestCreateAgentRejectsUnknownPreset(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	badPreset := "human"
	if _, err := svc.CreateAgent(context.Background(), modelCreateAgentRequest(&badPreset), "user-1"); err == nil {
		t.Fatal("expected custom agent preset input to be rejected")
	}
}

func TestUpdateAgentRejectsPresetFieldsForCustomAgents(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	created, err := svc.CreateAgent(context.Background(), modelCreateAgentRequest(nil), "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	presetKey := model.AgentPresetTaskPlanner
	if _, err := svc.UpdateAgent(context.Background(), "ws-test", created.ID, model.UpdateAgentRequest{
		PresetKey: &presetKey,
	}, "user-1"); err == nil {
		t.Fatal("expected preset update on custom agent to be rejected")
	}
}

func TestListAgents_DoesNotMutateWorkspaceAgents(t *testing.T) {
	db := newAgentServiceTestDB(t)
	now := time.Now().UTC()
	defaultPrompt := defaultSystemPromptForPreset(model.AgentPresetCodeBuilder)
	if defaultPrompt == nil {
		t.Fatal("expected code builder default prompt")
	}
	preset, ok := agentPresetDefinition(model.AgentPresetCodeBuilder)
	if !ok {
		t.Fatal("expected code builder preset definition")
	}
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, preset_version_key, role, status, runtime_kind,
		skills, trigger_mode, system_prompt, allowed_tools, allowed_commands, allowed_targets,
		approval_mode, max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-legacy-code-builder", "ws-test", false, "Code Builder", model.AgentPresetCodeBuilder, "",
		"Code Builder", "idle", "opencode", []byte("[]"), "manual", *defaultPrompt,
		mustJSONStringSlice(preset.AllowedTools),
		mustJSONStringSlice(preset.AllowedCommands),
		mustJSONStringSlice(preset.AllowedTargetTypes),
		"never", 1, model.InvocationModeAutonomous, now, now,
	).Error; err != nil {
		t.Fatalf("insert legacy built-in candidate: %v", err)
	}

	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	agents, err := svc.ListAgents(context.Background(), "ws-test")
	if err != nil {
		t.Fatalf("ListAgents returned error: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected read-only list to return 1 existing agent, got %d", len(agents))
	}

	custom, err := agentRepo.GetByID(context.Background(), "ws-test", "agent-legacy-code-builder")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if custom == nil {
		t.Fatal("expected custom agent to remain present")
	}
	if custom.IsSystem {
		t.Fatal("expected list path to avoid promoting custom agent to system")
	}
	if custom.Name != "Code Builder" {
		t.Fatalf("expected list path to preserve custom agent name, got %q", custom.Name)
	}
}

func TestSeedWorkspaceDefaults_CreatesMissingSystemAgentsWithoutPromotingCustomAgents(t *testing.T) {
	db := newAgentServiceTestDB(t)
	now := time.Now().UTC()
	defaultPrompt := defaultSystemPromptForPreset(model.AgentPresetCodeBuilder)
	if defaultPrompt == nil {
		t.Fatal("expected code builder default prompt")
	}
	preset, ok := agentPresetDefinition(model.AgentPresetCodeBuilder)
	if !ok {
		t.Fatal("expected code builder preset definition")
	}
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, preset_version_key, role, status, runtime_kind,
		skills, trigger_mode, system_prompt, allowed_tools, allowed_commands, allowed_targets,
		approval_mode, max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-custom-code-builder", "ws-test", false, "Code Builder", model.AgentPresetCodeBuilder, "",
		"Code Builder", "idle", "opencode", []byte("[]"), "manual", *defaultPrompt,
		mustJSONStringSlice(preset.AllowedTools),
		mustJSONStringSlice(preset.AllowedCommands),
		mustJSONStringSlice(preset.AllowedTargetTypes),
		"never", 1, model.InvocationModeAutonomous, now, now,
	).Error; err != nil {
		t.Fatalf("insert default-looking custom agent: %v", err)
	}

	agentRepo := repository.NewAgentRepository(db)
	svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "", "", false, "", "")

	if err := svc.SeedWorkspaceDefaults(context.Background(), "ws-test", "user-1"); err != nil {
		t.Fatalf("SeedWorkspaceDefaults returned error: %v", err)
	}

	agents, err := agentRepo.List(context.Background(), "ws-test")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	systemPresets := make([]string, 0)
	for _, agent := range agents {
		if agent.IsSystem {
			systemPresets = append(systemPresets, agent.PresetKey)
		}
	}
	for _, presetKey := range builtInPresetKeys() {
		if !slices.Contains(systemPresets, presetKey) {
			t.Fatalf("expected seeded system agent for preset %q", presetKey)
		}
	}
	forge, err := agentRepo.GetSystemByPreset(context.Background(), "ws-test", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("GetSystemByPreset returned error: %v", err)
	}
	if forge == nil {
		t.Fatal("expected system forge agent")
	}
	if forge.RuntimeKind != "codex" {
		t.Fatalf("expected forge runtime codex, got %q", forge.RuntimeKind)
	}
	if forge.Provider == nil || *forge.Provider != model.AgentModelProviderOpenAI {
		t.Fatalf("expected forge provider openai, got %+v", forge.Provider)
	}
	if forge.Model == nil || *forge.Model != "gpt-5.4" {
		t.Fatalf("expected forge model gpt-5.4, got %+v", forge.Model)
	}
	lens, err := agentRepo.GetSystemByPreset(context.Background(), "ws-test", model.AgentPresetReviewAgent)
	if err != nil {
		t.Fatalf("GetSystemByPreset returned error: %v", err)
	}
	if lens == nil {
		t.Fatal("expected system lens agent")
	}
	if lens.RuntimeKind != "codex" {
		t.Fatalf("expected lens runtime codex, got %q", lens.RuntimeKind)
	}
	if lens.Provider == nil || *lens.Provider != model.AgentModelProviderOpenAI {
		t.Fatalf("expected lens provider openai, got %+v", lens.Provider)
	}
	if lens.Model == nil || *lens.Model != "gpt-5.4" {
		t.Fatalf("expected lens model gpt-5.4, got %+v", lens.Model)
	}

	custom, err := agentRepo.GetByID(context.Background(), "ws-test", "agent-custom-code-builder")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if custom == nil {
		t.Fatal("expected custom code builder to remain present")
	}
	if custom.IsSystem {
		t.Fatal("expected custom code builder to remain non-system")
	}
	if custom.Name != "Code Builder" {
		t.Fatalf("expected custom agent name to remain unchanged, got %q", custom.Name)
	}
}

func TestSeedWorkspaceDefaults_ReconcilesAndDedupesExistingSystemPresetAgents(t *testing.T) {
	db := newAgentServiceTestDB(t)
	now := time.Now().UTC()
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, preset_version_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets,
		approval_mode, max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-system-old", "ws-test", true, "Epic Planner", model.AgentPresetEpicPlanner, defaultPresetVersionKeyForPresetKey(model.AgentPresetEpicPlanner),
		"Epic Planner", "idle", "native_sdk", []byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now.Add(-time.Hour), now.Add(-time.Hour),
		"agent-system-new", "ws-test", true, "Epic Planner", model.AgentPresetEpicPlanner, defaultPresetVersionKeyForPresetKey(model.AgentPresetEpicPlanner),
		"Epic Planner", "idle", "native_sdk", []byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	).Error; err != nil {
		t.Fatalf("insert duplicate system agents: %v", err)
	}

	agentRepo := repository.NewAgentRepository(db)
	svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "", "", false, "", "")

	if err := svc.SeedWorkspaceDefaults(context.Background(), "ws-test", "user-1"); err != nil {
		t.Fatalf("SeedWorkspaceDefaults returned error: %v", err)
	}

	agents, err := agentRepo.List(context.Background(), "ws-test")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	var epicSystemAgents []model.Agent
	for _, agent := range agents {
		if agent.IsSystem && agent.PresetKey == model.AgentPresetEpicPlanner {
			epicSystemAgents = append(epicSystemAgents, agent)
		}
	}
	if len(epicSystemAgents) != 1 {
		t.Fatalf("expected exactly one epic planner system agent after reconciliation, got %d", len(epicSystemAgents))
	}
	if epicSystemAgents[0].Name != defaultSystemEpicPlannerName {
		t.Fatalf("expected reconciled system agent name %q, got %q", defaultSystemEpicPlannerName, epicSystemAgents[0].Name)
	}

	deleted, err := agentRepo.GetByID(context.Background(), "ws-test", "agent-system-new")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if deleted != nil {
		t.Fatal("expected duplicate system agent to be deleted")
	}
}

func TestUpdateAgent_PreservesSystemAgentPresetFamily(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}
	svc.SetModelProviderConfig("", "test-openai-key", "", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}

	newName := "Forge Prime"
	newModel := "gpt-5-mini"
	runtime := defaultRuntimeKindForPresetKey(model.AgentPresetCodeBuilder)
	presetKey := model.AgentPresetCodeBuilder
	updated, err := svc.UpdateAgent(context.Background(), "ws-test", systemAgent.ID, model.UpdateAgentRequest{
		Name:        &newName,
		Model:       &newModel,
		RuntimeKind: &runtime,
		PresetKey:   &presetKey,
	}, "user-1")
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	if updated.PresetKey != model.AgentPresetCodeBuilder {
		t.Fatalf("expected system agent preset to remain %q, got %q", model.AgentPresetCodeBuilder, updated.PresetKey)
	}
	if updated.PresetVersionKey != defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder) {
		t.Fatalf("expected system agent preset version to remain default %q, got %q", defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder), updated.PresetVersionKey)
	}
	if updated.Name != newName {
		t.Fatalf("expected updated name %q, got %q", newName, updated.Name)
	}
	if updated.Model == nil || *updated.Model != newModel {
		t.Fatalf("expected updated model %q, got %+v", newModel, updated.Model)
	}
}

func TestUpdateAgent_PreservesSelectedSystemPresetVersion(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
		activitySvc:                activitySvc,
		wsPublisher:                nil,
	}
	svc.SetModelProviderConfig("", "test-openai-key", "", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if err := workspacePresetVersionRepo.Create(context.Background(), &model.WorkspaceAgentPresetVersion{
		WorkspaceID:           "ws-test",
		FamilyKey:             model.AgentPresetCodeBuilder,
		VersionKey:            "code_builder_workspace_v2",
		Label:                 "Workspace v2",
		RuntimeKind:           "opencode",
		SystemPrompt:          agentTestStringPtr("Workspace tuned code builder."),
		AllowedTools:          mustJSONStringSlice([]string{"read_file", "run_command"}),
		SupportedModes:        mustJSONStringSlice([]string{model.InvocationModeAutonomous}),
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}); err != nil {
		t.Fatalf("Create workspace preset version returned error: %v", err)
	}

	versionKey := "code_builder_workspace_v2"
	updated, err := svc.UpdateAgent(context.Background(), "ws-test", systemAgent.ID, model.UpdateAgentRequest{
		PresetVersionKey: &versionKey,
	}, "user-1")
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	if updated.PresetVersionKey != versionKey {
		t.Fatalf("expected selected preset version %q, got %q", versionKey, updated.PresetVersionKey)
	}
	if updated.ApprovalMode != "never" {
		t.Fatalf("expected system agent approval mode never, got %q", updated.ApprovalMode)
	}
}

func TestEnsureBuiltInAgent_UpgradesLegacyCodeBuilderRuntimeToCodex(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}
	svc.SetModelProviderConfig("", "test-openai-key", "", "", false, "", "")

	defaultPrompt := defaultSystemPromptForPreset(model.AgentPresetCodeBuilder)
	if defaultPrompt == nil {
		t.Fatal("expected code builder default prompt")
	}
	now := time.Now().UTC()
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, preset_version_key, role, status, runtime_kind,
		skills, trigger_mode, system_prompt, allowed_tools, allowed_commands, allowed_targets,
		approval_mode, max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-system-legacy-forge", "ws-test", true, "Code Builder", model.AgentPresetCodeBuilder, "code_builder_default",
		"Code Builder", "idle", "opencode", []byte("[]"), "manual", *defaultPrompt,
		mustJSONStringSlice([]string{"read_file", "run_command"}),
		mustJSONStringSlice([]string{}),
		mustJSONStringSlice([]string{"task"}),
		"never", 1, model.InvocationModeAutonomous, now, now,
	).Error; err != nil {
		t.Fatalf("insert legacy forge system agent: %v", err)
	}

	updated, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if updated.RuntimeKind != "codex" {
		t.Fatalf("expected legacy forge runtime to upgrade to codex, got %q", updated.RuntimeKind)
	}
	if updated.PresetVersionKey != defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder) {
		t.Fatalf("expected preset version %q, got %q", defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder), updated.PresetVersionKey)
	}
}

func TestUpdateAgent_AllowsSystemPresetVersionRuntimeFromSelectedVersion(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
		activitySvc:                activitySvc,
		wsPublisher:                nil,
	}
	svc.SetModelProviderConfig("", "test-openai-key", "", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if err := workspacePresetVersionRepo.Create(context.Background(), &model.WorkspaceAgentPresetVersion{
		WorkspaceID:           "ws-test",
		FamilyKey:             model.AgentPresetCodeBuilder,
		VersionKey:            "code_builder_workspace_codex",
		Label:                 "Workspace Codex",
		RuntimeKind:           "codex",
		SystemPrompt:          agentTestStringPtr("Use Codex for code builder runs."),
		AllowedTools:          mustJSONStringSlice([]string{"read_file", "run_command"}),
		SupportedModes:        mustJSONStringSlice([]string{model.InvocationModeAutonomous, model.InvocationModeInteractive}),
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}); err != nil {
		t.Fatalf("Create workspace preset version returned error: %v", err)
	}

	versionKey := "code_builder_workspace_codex"
	runtimeKind := "codex"
	updated, err := svc.UpdateAgent(context.Background(), "ws-test", systemAgent.ID, model.UpdateAgentRequest{
		PresetVersionKey: &versionKey,
		RuntimeKind:      &runtimeKind,
	}, "user-1")
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	if updated.PresetVersionKey != versionKey {
		t.Fatalf("expected selected preset version %q, got %q", versionKey, updated.PresetVersionKey)
	}
	if updated.RuntimeKind != runtimeKind {
		t.Fatalf("expected runtime kind %q, got %q", runtimeKind, updated.RuntimeKind)
	}
}

func TestDeleteAgentRejectsSystemAgent(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	now := "2026-03-21T00:00:00Z"
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-system", "ws-test", true, "Epic Planner", model.AgentPresetEpicPlanner, "Epic Planner", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	).Error; err != nil {
		t.Fatalf("insert system agent: %v", err)
	}

	if err := svc.DeleteAgent(context.Background(), "ws-test", "agent-system", "user-1"); err == nil {
		t.Fatal("expected delete to reject system agent")
	}
}

func TestCreateWorkspacePresetVersion(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
	}

	req := model.CreateWorkspaceAgentPresetVersionRequest{
		WorkspaceID:      "ws-test",
		FamilyKey:        model.AgentPresetCodeBuilder,
		Label:            "Engineering v2",
		SourceVersionKey: agentTestStringPtr(defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder)),
		RuntimeKind:      agentTestStringPtr("codex"),
		Model:            agentTestStringPtr("gpt-5-mini"),
		ExecutionConfig:  json.RawMessage(`{"reasoning_effort":"high","service_tier":"fast"}`),
		SystemPrompt:     agentTestStringPtr("Use the repo conventions and keep changes incremental."),
		AllowedTools:     mustJSONStringSlice([]string{"read_file", "run_command"}),
		SupportedModes:   mustJSONStringSlice([]string{model.InvocationModeAutonomous}),
		ApprovalMode:     agentTestStringPtr("always"),
	}

	version, err := svc.CreateWorkspacePresetVersion(context.Background(), req, "user-1")
	if err != nil {
		t.Fatalf("CreateWorkspacePresetVersion returned error: %v", err)
	}
	if version.Scope != "workspace" {
		t.Fatalf("expected workspace scope, got %q", version.Scope)
	}
	if version.FamilyKey != model.AgentPresetCodeBuilder {
		t.Fatalf("expected family key %q, got %q", model.AgentPresetCodeBuilder, version.FamilyKey)
	}
	if version.VersionKey == "" || version.VersionKey == defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder) {
		t.Fatalf("expected a new workspace version key, got %q", version.VersionKey)
	}
	if version.Model == nil || *version.Model != "gpt-5-mini" {
		t.Fatalf("expected persisted model override, got %+v", version.Model)
	}
	if strings.TrimSpace(string(version.ExecutionConfig)) != `{"reasoning_effort":"high","service_tier":"fast"}` {
		t.Fatalf("expected persisted execution config override, got %s", version.ExecutionConfig)
	}
	if version.RuntimeKind != "codex" {
		t.Fatalf("expected persisted runtime override, got %q", version.RuntimeKind)
	}
	if !slices.Equal(version.SupportedModes, []string{model.InvocationModeAutonomous}) {
		t.Fatalf("expected persisted supported modes override, got %v", version.SupportedModes)
	}
	if version.SystemPrompt == nil || *version.SystemPrompt != "Use the repo conventions and keep changes incremental." {
		t.Fatalf("expected persisted system prompt override, got %+v", version.SystemPrompt)
	}
	if !slices.Equal(version.AllowedTools, []string{"read_file", "run_command"}) {
		t.Fatalf("expected allowed tools override, got %v", version.AllowedTools)
	}
	if version.ApprovalMode != "never" {
		t.Fatalf("expected workspace preset version approval mode to be forced to never, got %q", version.ApprovalMode)
	}
}

func TestEnsureBuiltInAgent_AppliesDefaultExecutionConfigForForgeAndLens(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}
	svc.SetModelProviderConfig("", "test-openai-key", "", "", false, "", "")

	for _, presetKey := range []string{model.AgentPresetCodeBuilder, model.AgentPresetReviewAgent} {
		agent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", presetKey)
		if err != nil {
			t.Fatalf("ensureBuiltInAgent(%s) returned error: %v", presetKey, err)
		}
		if strings.TrimSpace(string(agent.ExecutionConfig)) != `{"reasoning_effort":"high","service_tier":"fast"}` {
			t.Fatalf("expected %s execution config to default to fast+high, got %s", presetKey, agent.ExecutionConfig)
		}
	}
}

func TestListAgentPresetsIncludesWorkspaceVersions(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
	}

	if err := workspacePresetVersionRepo.Create(context.Background(), &model.WorkspaceAgentPresetVersion{
		WorkspaceID:           "ws-test",
		FamilyKey:             model.AgentPresetEpicPlanner,
		VersionKey:            "epic_planner_workspace_v2",
		Label:                 "Ops Variant",
		RuntimeKind:           "native_sdk",
		SystemPrompt:          agentTestStringPtr("Plan with explicit operational checkpoints."),
		AllowedTools:          mustJSONStringSlice([]string{"publish_prd_draft", "request_user_input"}),
		SupportedModes:        mustJSONStringSlice([]string{model.InvocationModeAutonomous, model.InvocationModeInteractive}),
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeInteractive,
	}); err != nil {
		t.Fatalf("Create workspace preset version returned error: %v", err)
	}

	presets := svc.ListAgentPresets(context.Background(), "ws-test")
	found := false
	for _, preset := range presets {
		if preset.VersionKey != "epic_planner_workspace_v2" {
			continue
		}
		found = true
		if preset.Scope != "workspace" {
			t.Fatalf("expected workspace scope, got %q", preset.Scope)
		}
		if preset.WorkspaceID == nil || *preset.WorkspaceID != "ws-test" {
			t.Fatalf("expected workspace id ws-test, got %+v", preset.WorkspaceID)
		}
		if preset.SystemPrompt == nil || *preset.SystemPrompt != "Plan with explicit operational checkpoints." {
			t.Fatalf("expected workspace system prompt, got %+v", preset.SystemPrompt)
		}
	}
	if !found {
		t.Fatal("expected workspace preset version in catalog")
	}
}

func modelCreateAgentRequest(presetKey *string) model.CreateAgentRequest {
	return model.CreateAgentRequest{
		WorkspaceID: "ws-test",
		Name:        "Example",
		PresetKey:   presetKey,
	}
}

func agentTestStringPtr(value string) *string {
	return &value
}

func newAgentServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	registerAgentTestUUIDCallback(t, db)

	statements := []string{
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			role TEXT,
			status TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			skills BLOB NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			system_prompt TEXT,
			planning_notes TEXT,
			tools BLOB NOT NULL DEFAULT '[]',
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			allowed_commands BLOB NOT NULL DEFAULT '[]',
			allowed_targets BLOB NOT NULL DEFAULT '[]',
			schedule TEXT,
			target_selector TEXT,
			trigger_events BLOB NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'never',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_agent_preset_versions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			family_key TEXT NOT NULL,
			version_key TEXT NOT NULL,
			label TEXT NOT NULL,
			description TEXT,
			source_version_key TEXT,
			runtime_kind TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			system_prompt TEXT,
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			supported_modes BLOB NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_activity_log (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			actor_id TEXT,
			action TEXT NOT NULL,
			field_name TEXT,
			old_value TEXT,
			new_value TEXT,
			metadata TEXT,
			created_at DATETIME
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}

	return db
}

func registerAgentTestUUIDCallback(t *testing.T, db *gorm.DB) {
	t.Helper()
	err := db.Callback().Create().Before("gorm:before_create").Register("test:assign_uuid", func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		field := tx.Statement.Schema.LookUpField("ID")
		if field == nil || field.FieldType.Kind() != reflect.String {
			return
		}

		assignID := func(value reflect.Value) {
			if _, zero := field.ValueOf(tx.Statement.Context, value); !zero {
				return
			}
			_ = field.Set(tx.Statement.Context, value, uuid.NewString())
		}

		switch tx.Statement.ReflectValue.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < tx.Statement.ReflectValue.Len(); i++ {
				assignID(tx.Statement.ReflectValue.Index(i))
			}
		default:
			assignID(tx.Statement.ReflectValue)
		}
	})
	if err != nil {
		t.Fatalf("register uuid callback: %v", err)
	}
}
