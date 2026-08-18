package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
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
	if created.RuntimeKind != "codex" {
		t.Fatalf("expected default runtime codex, got %q", created.RuntimeKind)
	}
}

func TestCustomAgentIconCanBeCreatedAndUpdated(t *testing.T) {
	db := newAgentServiceTestDB(t)
	svc := &AgentService{agentRepo: repository.NewAgentRepository(db)}

	req := modelCreateAgentRequest(nil)
	req.IconKey = agentTestStringPtr("violet_star")
	created, err := svc.CreateAgent(context.Background(), req, "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}
	if created.IconKey != "violet_star" {
		t.Fatalf("created.IconKey = %q, want violet_star", created.IconKey)
	}

	updated, err := svc.UpdateAgent(context.Background(), "ws-test", created.ID, model.UpdateAgentRequest{
		IconKey: agentTestStringPtr("ocean_orbit"),
	}, "user-1")
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	if updated.IconKey != "ocean_orbit" {
		t.Fatalf("updated.IconKey = %q, want ocean_orbit", updated.IconKey)
	}

	if _, err := svc.UpdateAgent(context.Background(), "ws-test", created.ID, model.UpdateAgentRequest{
		IconKey: agentTestStringPtr("forge"),
	}, "user-1"); err == nil {
		t.Fatal("expected named product-agent icon key to be rejected")
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

func TestCreateAgentPersistsMultipleTeamIDs(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	req := modelCreateAgentRequest(nil)
	req.TeamIDs = []string{"team-a", "team-b"}
	req.TeamID = agentTestStringPtr("legacy-team")

	created, err := svc.CreateAgent(context.Background(), req, "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}
	if got, want := created.TeamIDs, []string{"team-a", "team-b"}; !slices.Equal(got, want) {
		t.Fatalf("created.TeamIDs = %#v, want %#v", got, want)
	}
	if created.TeamID == nil || *created.TeamID != "team-a" {
		t.Fatalf("created.TeamID = %v, want legacy-compatible first team", created.TeamID)
	}
}

func TestCreateAgentLegacyTeamIDBackfillsTeamIDs(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	req := modelCreateAgentRequest(nil)
	req.TeamID = agentTestStringPtr("team-a")

	created, err := svc.CreateAgent(context.Background(), req, "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}
	if got, want := created.TeamIDs, []string{"team-a"}; !slices.Equal(got, want) {
		t.Fatalf("created.TeamIDs = %#v, want %#v", got, want)
	}
}

func TestUpdateAgentReplacesTeamIDsWhenExplicit(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	req := modelCreateAgentRequest(nil)
	req.TeamIDs = []string{"team-a", "team-b"}
	created, err := svc.CreateAgent(context.Background(), req, "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	updated, err := svc.UpdateAgent(context.Background(), "ws-test", created.ID, model.UpdateAgentRequest{
		TeamIDs: &[]string{"team-c"},
	}, "user-1")
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	if got, want := updated.TeamIDs, []string{"team-c"}; !slices.Equal(got, want) {
		t.Fatalf("updated.TeamIDs = %#v, want %#v", got, want)
	}
	if updated.TeamID == nil || *updated.TeamID != "team-c" {
		t.Fatalf("updated.TeamID = %v, want legacy-compatible first team", updated.TeamID)
	}

	updated, err = svc.UpdateAgent(context.Background(), "ws-test", created.ID, model.UpdateAgentRequest{
		TeamIDs: &[]string{},
	}, "user-1")
	if err != nil {
		t.Fatalf("UpdateAgent clear returned error: %v", err)
	}
	if len(updated.TeamIDs) != 0 {
		t.Fatalf("updated.TeamIDs = %#v, want workspace-wide empty team_ids", updated.TeamIDs)
	}
	if updated.TeamID != nil {
		t.Fatalf("updated.TeamID = %v, want nil after clearing team_ids", *updated.TeamID)
	}
}

func TestUpdateAgentRejectsMixedLegacyAndMultiTeamInputs(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	created, err := svc.CreateAgent(context.Background(), modelCreateAgentRequest(nil), "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	legacyTeamID := "team-a"
	if _, err := svc.UpdateAgent(context.Background(), "ws-test", created.ID, model.UpdateAgentRequest{
		TeamID:  &legacyTeamID,
		TeamIDs: &[]string{"team-b"},
	}, "user-1"); err == nil {
		t.Fatal("expected mixed team_id and team_ids update to be rejected")
	}
}

func TestEnsureSystemProductPlannerAgentRefreshesLegacyPrompt(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	now := time.Now().UTC()
	legacyPrompt := "1. `prd_draft`\n2. `awaiting_prd_approval`\n3. `persist_prd`\n4. `task_plan`\n5. `awaiting_task_approval`\n6. `create_tasks`"
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
	if strings.Contains(*updated.SystemPrompt, "awaiting_prd_approval") || strings.Contains(*updated.SystemPrompt, "awaiting_task_approval") {
		t.Fatalf("expected refreshed prompt to remove legacy approval phases, got %q", *updated.SystemPrompt)
	}
	if !strings.Contains(*updated.SystemPrompt, "Approval requests happen inline in the same chat.") {
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
	if !strings.Contains(*updated.SystemPrompt, "`"+agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolPublishTaskPlanDoc)+"`") {
		t.Fatalf("expected refreshed task planner prompt to include publish_task_plan_doc, got %q", *updated.SystemPrompt)
	}
	if updated.Name != "Scribe" {
		t.Fatalf("expected renamed task planner %q, got %q", "Scribe", updated.Name)
	}
}

func TestEnsureBuiltInCommandAgentReconcilesLegacyResearcherPreset(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	now := time.Now().UTC()
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, preset_version_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets,
		approval_mode, max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-command-legacy", "ws-test", true, "Command Agent", model.AgentPresetResearcher, "researcher_default", "Command Agent", "idle", "native_sdk",
		[]byte("[]"), "manual", []byte("[]"), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeAutonomous, now, now,
	).Error; err != nil {
		t.Fatalf("insert legacy command agent: %v", err)
	}

	updated, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCommandAgent)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if updated.ID != "agent-command-legacy" {
		t.Fatalf("expected legacy command agent to be reused, got %q", updated.ID)
	}
	if updated.PresetKey != model.AgentPresetCommandAgent {
		t.Fatalf("expected canonical command agent preset key %q, got %q", model.AgentPresetCommandAgent, updated.PresetKey)
	}
	if updated.PresetVersionKey != defaultPresetVersionKeyForPresetKey(model.AgentPresetCommandAgent) {
		t.Fatalf("expected canonical command agent preset version %q, got %q", defaultPresetVersionKeyForPresetKey(model.AgentPresetCommandAgent), updated.PresetVersionKey)
	}
	if updated.Name != "Sub-agent" {
		t.Fatalf("expected legacy command agent display name to reconcile to Sub-agent, got %q", updated.Name)
	}

	agents, err := agentRepo.List(context.Background(), "ws-test")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	var commandAgents []model.Agent
	for _, agent := range agents {
		if agent.IsSystem && normalizePresetKey(agent.PresetKey) == model.AgentPresetCommandAgent {
			commandAgents = append(commandAgents, agent)
		}
	}
	if len(commandAgents) != 1 {
		t.Fatalf("expected exactly one command system agent after reconciliation, got %d", len(commandAgents))
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
	if updated.SystemPrompt == nil ||
		!strings.Contains(*updated.SystemPrompt, "`"+agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolRequestUserInput)+"`") ||
		!strings.Contains(*updated.SystemPrompt, "`"+agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolRequestReviewCheckpoint)+"`") {
		t.Fatalf("expected refreshed review prompt with interactive loop tools, got %+v", updated.SystemPrompt)
	}
	var tools []string
	if err := json.Unmarshal(updated.AllowedTools, &tools); err != nil {
		t.Fatalf("unmarshal allowed tools: %v", err)
	}
	if !slices.Contains(tools, agentcontract.ToolRequestUserInput) {
		t.Fatalf("expected review agent tools to include %q, got %v", agentcontract.ToolRequestUserInput, tools)
	}
	if !slices.Contains(tools, agentcontract.ToolRequestReviewCheckpoint) {
		t.Fatalf("expected review agent tools to include %q, got %v", agentcontract.ToolRequestReviewCheckpoint, tools)
	}
	if updated.DefaultInvocationMode != model.InvocationModeInteractive {
		t.Fatalf("expected review agent default invocation mode interactive, got %q", updated.DefaultInvocationMode)
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
	svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

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
			wantProvider, wantModel := model.AgentModelProviderOpenAI, "gpt-5.6-terra"
			switch agent.PresetKey {
			case model.AgentPresetEpicPlanner:
				wantProvider, wantModel = model.AgentModelProviderOpenRouter, defaultAtlasAgentModel
			case model.AgentPresetTaskPlanner:
				wantProvider, wantModel = model.AgentModelProviderOpenAI, defaultScribeAgentModel
			case model.AgentPresetDocumentationAgent:
				wantProvider, wantModel = model.AgentModelProviderOpenRouter, defaultQuillAgentModel
			case model.AgentPresetAskAgent, model.AgentPresetSupportAgent:
				// The dock orchestrator and support agent default to a
				// flash-tier OpenRouter model.
				wantProvider, wantModel = model.AgentModelProviderOpenRouter, defaultAskAgentModel
			}
			if agent.Provider == nil || *agent.Provider != wantProvider {
				t.Errorf("system preset %q provider = %+v, want %s", agent.PresetKey, agent.Provider, wantProvider)
			}
			if agent.Model == nil || *agent.Model != wantModel {
				t.Errorf("system preset %q model = %+v, want %s", agent.PresetKey, agent.Model, wantModel)
			}
			runtimeAgent := runtimeAgentFromHelpinAgent(&agent, "helpin")
			if runtimeAgent.Provider != wantProvider || runtimeAgent.Model != wantModel {
				t.Errorf(
					"system preset %q runtime routing = %q/%q, want %s/%s",
					agent.PresetKey,
					runtimeAgent.Provider,
					runtimeAgent.Model,
					wantProvider,
					wantModel,
				)
			}
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
	if forge.Model == nil || *forge.Model != "gpt-5.6-terra" {
		t.Fatalf("expected forge model gpt-5.6-terra, got %+v", forge.Model)
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
	if lens.Model == nil || *lens.Model != "gpt-5.6-terra" {
		t.Fatalf("expected lens model gpt-5.6-terra, got %+v", lens.Model)
	}
	supportAgent, err := agentRepo.GetSystemByPreset(context.Background(), "ws-test", model.AgentPresetSupportAgent)
	if err != nil {
		t.Fatalf("GetSystemByPreset returned error: %v", err)
	}
	if supportAgent == nil {
		t.Fatal("expected system support agent")
	}
	if supportAgent.Provider == nil || *supportAgent.Provider != model.AgentModelProviderOpenRouter {
		t.Fatalf("expected support agent provider openrouter, got %+v", supportAgent.Provider)
	}
	if supportAgent.Model == nil || *supportAgent.Model != defaultAskAgentModel {
		t.Fatalf("expected support agent model %s, got %+v", defaultAskAgentModel, supportAgent.Model)
	}
	docsAgent, err := agentRepo.GetSystemByPreset(context.Background(), "ws-test", model.AgentPresetDocumentationAgent)
	if err != nil {
		t.Fatalf("GetSystemByPreset returned error: %v", err)
	}
	if docsAgent == nil {
		t.Fatal("expected documentation system agent")
	}
	if docsAgent.Name != "Quill" {
		t.Fatalf("expected documentation agent name, got %q", docsAgent.Name)
	}
	if docsAgent.RuntimeKind != "native_sdk" {
		t.Fatalf("expected documentation runtime native_sdk, got %q", docsAgent.RuntimeKind)
	}
	if docsAgent.Provider == nil || *docsAgent.Provider != model.AgentModelProviderOpenRouter {
		t.Fatalf("expected documentation provider openrouter, got %+v", docsAgent.Provider)
	}
	if docsAgent.Model == nil || *docsAgent.Model != defaultQuillAgentModel {
		t.Fatalf("expected documentation model %s, got %+v", defaultQuillAgentModel, docsAgent.Model)
	}
	if docsAgent.DefaultInvocationMode != model.InvocationModeInteractive {
		t.Fatalf("expected documentation default invocation mode interactive, got %q", docsAgent.DefaultInvocationMode)
	}
	if !slices.Contains(parseJSONStringSlice(docsAgent.AllowedTargets), "document") || !slices.Contains(parseJSONStringSlice(docsAgent.AllowedTargets), "support_conversation") {
		t.Fatalf("expected documentation targets to include document and support_conversation, got %s", string(docsAgent.AllowedTargets))
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
	svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

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
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "test-openrouter-key", "", false, "", "")

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
	reconciled, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if reconciled.Model == nil || *reconciled.Model != newModel {
		t.Fatalf("expected custom model %q to survive reconciliation, got %+v", newModel, reconciled.Model)
	}
}

func TestEnsureBuiltInAgent_UpgradesLegacyDefaultModelToGPT56Terra(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	legacyModel := "gpt-5.5"
	systemAgent.Model = &legacyModel
	if err := agentRepo.Update(context.Background(), systemAgent); err != nil {
		t.Fatalf("persist legacy model: %v", err)
	}

	reconciled, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if reconciled.Model == nil || *reconciled.Model != "gpt-5.6-terra" {
		t.Fatalf("expected reconciled model gpt-5.6-terra, got %+v", reconciled.Model)
	}
}

func TestEnsureBuiltInAgent_UpgradesLegacyScribeDefaultRouting(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetTaskPlanner)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	legacyProvider := model.AgentModelProviderOpenRouter
	legacyModel := "deepseek/deepseek-v4-flash"
	systemAgent.RuntimeKind = "native_sdk"
	systemAgent.Provider = &legacyProvider
	systemAgent.Model = &legacyModel
	if err := agentRepo.Update(context.Background(), systemAgent); err != nil {
		t.Fatalf("persist legacy Scribe routing: %v", err)
	}

	reconciled, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetTaskPlanner)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if reconciled.RuntimeKind != "codex" {
		t.Fatalf("expected reconciled runtime codex, got %q", reconciled.RuntimeKind)
	}
	if reconciled.Provider == nil || *reconciled.Provider != model.AgentModelProviderOpenAI {
		t.Fatalf("expected reconciled provider openai, got %+v", reconciled.Provider)
	}
	if reconciled.Model == nil || *reconciled.Model != defaultScribeAgentModel {
		t.Fatalf("expected reconciled model %s, got %+v", defaultScribeAgentModel, reconciled.Model)
	}
}

func TestEnsureBuiltInAgent_UpgradesLegacyQuillDefaultRouting(t *testing.T) {
	legacyModels := []string{"gpt-5.5", defaultOpenAIAgentModel}
	for _, legacyModel := range legacyModels {
		t.Run(legacyModel, func(t *testing.T) {
			db := newAgentServiceTestDB(t)
			agentRepo := repository.NewAgentRepository(db)
			svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

			systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetDocumentationAgent)
			if err != nil {
				t.Fatalf("ensureBuiltInAgent returned error: %v", err)
			}
			legacyProvider := model.AgentModelProviderOpenAI
			systemAgent.RuntimeKind = "codex"
			systemAgent.Provider = &legacyProvider
			systemAgent.Model = &legacyModel
			if err := agentRepo.Update(context.Background(), systemAgent); err != nil {
				t.Fatalf("persist legacy Quill routing: %v", err)
			}

			reconciled, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetDocumentationAgent)
			if err != nil {
				t.Fatalf("ensureBuiltInAgent returned error: %v", err)
			}
			if reconciled.RuntimeKind != "native_sdk" {
				t.Fatalf("expected reconciled runtime native_sdk, got %q", reconciled.RuntimeKind)
			}
			if reconciled.Provider == nil || *reconciled.Provider != model.AgentModelProviderOpenRouter {
				t.Fatalf("expected reconciled provider openrouter, got %+v", reconciled.Provider)
			}
			if reconciled.Model == nil || *reconciled.Model != defaultQuillAgentModel {
				t.Fatalf("expected reconciled model %s, got %+v", defaultQuillAgentModel, reconciled.Model)
			}
		})
	}
}

func TestEnsureBuiltInAgent_UpgradesLegacyAtlasDefaultRouting(t *testing.T) {
	legacyModels := []string{"gpt-5.5", defaultOpenAIAgentModel}
	for _, legacyModel := range legacyModels {
		t.Run(legacyModel, func(t *testing.T) {
			db := newAgentServiceTestDB(t)
			agentRepo := repository.NewAgentRepository(db)
			svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

			systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetEpicPlanner)
			if err != nil {
				t.Fatalf("ensureBuiltInAgent returned error: %v", err)
			}
			legacyProvider := model.AgentModelProviderOpenAI
			systemAgent.Provider = &legacyProvider
			systemAgent.Model = &legacyModel
			if err := agentRepo.Update(context.Background(), systemAgent); err != nil {
				t.Fatalf("persist legacy Atlas routing: %v", err)
			}

			reconciled, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetEpicPlanner)
			if err != nil {
				t.Fatalf("ensureBuiltInAgent returned error: %v", err)
			}
			if reconciled.RuntimeKind != "native_sdk" {
				t.Fatalf("expected reconciled runtime native_sdk, got %q", reconciled.RuntimeKind)
			}
			if reconciled.Provider == nil || *reconciled.Provider != model.AgentModelProviderOpenRouter {
				t.Fatalf("expected reconciled provider openrouter, got %+v", reconciled.Provider)
			}
			if reconciled.Model == nil || *reconciled.Model != defaultAtlasAgentModel {
				t.Fatalf("expected reconciled model %s, got %+v", defaultAtlasAgentModel, reconciled.Model)
			}
		})
	}
}

func TestEnsureBuiltInAgent_PreservesCustomAtlasRouting(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetEpicPlanner)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	customProvider := model.AgentModelProviderOpenAI
	customModel := "gpt-5-mini"
	systemAgent.Provider = &customProvider
	systemAgent.Model = &customModel
	if err := agentRepo.Update(context.Background(), systemAgent); err != nil {
		t.Fatalf("persist custom Atlas routing: %v", err)
	}

	reconciled, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetEpicPlanner)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if reconciled.RuntimeKind != "native_sdk" {
		t.Fatalf("expected reconciled runtime native_sdk, got %q", reconciled.RuntimeKind)
	}
	if reconciled.Provider == nil || *reconciled.Provider != customProvider {
		t.Fatalf("expected custom provider %s, got %+v", customProvider, reconciled.Provider)
	}
	if reconciled.Model == nil || *reconciled.Model != customModel {
		t.Fatalf("expected custom model %s, got %+v", customModel, reconciled.Model)
	}
}

func TestEnsureBuiltInAgent_PreservesCustomScribeRouting(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetTaskPlanner)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	customProvider := model.AgentModelProviderOpenAI
	customModel := "gpt-5-mini"
	systemAgent.Provider = &customProvider
	systemAgent.Model = &customModel
	if err := agentRepo.Update(context.Background(), systemAgent); err != nil {
		t.Fatalf("persist custom Scribe routing: %v", err)
	}

	reconciled, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetTaskPlanner)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if reconciled.Provider == nil || *reconciled.Provider != customProvider {
		t.Fatalf("expected custom provider %s, got %+v", customProvider, reconciled.Provider)
	}
	if reconciled.RuntimeKind != "codex" {
		t.Fatalf("expected Scribe runtime codex, got %q", reconciled.RuntimeKind)
	}
	if reconciled.Model == nil || *reconciled.Model != customModel {
		t.Fatalf("expected custom model %s, got %+v", customModel, reconciled.Model)
	}
}

func TestEnsureBuiltInAgent_PreservesCustomQuillRouting(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := (&AgentService{agentRepo: agentRepo}).SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetDocumentationAgent)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	customProvider := model.AgentModelProviderOpenAI
	customModel := "gpt-5-mini"
	systemAgent.Provider = &customProvider
	systemAgent.Model = &customModel
	if err := agentRepo.Update(context.Background(), systemAgent); err != nil {
		t.Fatalf("persist custom Quill routing: %v", err)
	}

	reconciled, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetDocumentationAgent)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if reconciled.RuntimeKind != "native_sdk" {
		t.Fatalf("expected reconciled runtime native_sdk, got %q", reconciled.RuntimeKind)
	}
	if reconciled.Provider == nil || *reconciled.Provider != customProvider {
		t.Fatalf("expected custom provider %s, got %+v", customProvider, reconciled.Provider)
	}
	if reconciled.Model == nil || *reconciled.Model != customModel {
		t.Fatalf("expected custom model %s, got %+v", customModel, reconciled.Model)
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
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "test-openrouter-key", "", false, "", "")

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
		InstructionSkills:     mustJSONStringSlice(nil),
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
	// The custom version's prompt must survive the update (not be replaced by the built-in family prompt).
	if updated.SystemPrompt == nil || !strings.Contains(*updated.SystemPrompt, "Workspace tuned code builder.") {
		t.Fatalf("expected workspace preset prompt preserved after update, got %v", updated.SystemPrompt)
	}

	// Verify the prompt also survives a re-read through the service (which runs normalizeAgentRecord + materialize).
	reloaded, err := svc.GetAgent(context.Background(), "ws-test", updated.ID)
	if err != nil {
		t.Fatalf("GetAgent returned error: %v", err)
	}
	if reloaded.SystemPrompt == nil || !strings.Contains(*reloaded.SystemPrompt, "Workspace tuned code builder.") {
		t.Fatalf("expected workspace preset prompt preserved after re-read, got %v", reloaded.SystemPrompt)
	}
}

func TestUpdateAgent_ClearsSystemModelWhenBlankStringProvided(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}
	svc.SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if systemAgent.Model == nil || strings.TrimSpace(*systemAgent.Model) == "" {
		t.Fatalf("expected seeded system agent model, got %+v", systemAgent.Model)
	}

	blankModel := ""
	updated, err := svc.UpdateAgent(context.Background(), "ws-test", systemAgent.ID, model.UpdateAgentRequest{
		Model: &blankModel,
	}, "user-1")
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	if updated.Model != nil {
		t.Fatalf("expected cleared model, got %+v", updated.Model)
	}

	reloaded, err := svc.GetAgent(context.Background(), "ws-test", updated.ID)
	if err != nil {
		t.Fatalf("GetAgent returned error: %v", err)
	}
	if reloaded.Model != nil {
		t.Fatalf("expected cleared model after reload, got %+v", reloaded.Model)
	}
}

func TestUpdateAgent_AllowsSystemAgentMonthlyTokenBudgetUpdate(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}

	budget := 250000
	updated, err := svc.UpdateAgent(context.Background(), "ws-test", systemAgent.ID, model.UpdateAgentRequest{
		MonthlyTokenBudget: &budget,
	}, "user-1")
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	if updated.MonthlyTokenBudget == nil || *updated.MonthlyTokenBudget != budget {
		t.Fatalf("expected monthly token budget %d, got %+v", budget, updated.MonthlyTokenBudget)
	}
	if updated.PresetKey != model.AgentPresetCodeBuilder {
		t.Fatalf("expected preset key to remain %q, got %q", model.AgentPresetCodeBuilder, updated.PresetKey)
	}

	clearBudget := 0
	updated, err = svc.UpdateAgent(context.Background(), "ws-test", systemAgent.ID, model.UpdateAgentRequest{
		MonthlyTokenBudget: &clearBudget,
	}, "user-1")
	if err != nil {
		t.Fatalf("UpdateAgent clear returned error: %v", err)
	}
	if updated.MonthlyTokenBudget != nil {
		t.Fatalf("expected monthly token budget to clear, got %+v", updated.MonthlyTokenBudget)
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
	svc.SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

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
	svc.SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

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
		Provider:              agentTestStringPtr(model.AgentModelProviderOpenAI),
		SystemPrompt:          agentTestStringPtr("Use Codex for code builder runs."),
		InstructionSkills:     mustJSONStringSlice(nil),
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
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "test-openrouter-key", "", false, "", "")

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
		t.Fatalf("expected a new custom version key, got %q", version.VersionKey)
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
	if !slices.Equal(version.AllowedTools, []string{"read_files", "run_command"}) {
		t.Fatalf("expected allowed tools override, got %v", version.AllowedTools)
	}
	if version.ApprovalMode != "never" {
		t.Fatalf("expected workspace preset version approval mode to be forced to never, got %q", version.ApprovalMode)
	}
}

func TestCreateWorkspacePresetVersion_PreservesExplicitBlankModel(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
	}

	blankModel := ""
	req := model.CreateWorkspaceAgentPresetVersionRequest{
		WorkspaceID:      "ws-test",
		FamilyKey:        model.AgentPresetCodeBuilder,
		Label:            "Modelless Forge",
		SourceVersionKey: agentTestStringPtr(defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder)),
		RuntimeKind:      agentTestStringPtr("codex"),
		Provider:         agentTestStringPtr(model.AgentModelProviderOpenAI),
		Model:            &blankModel,
		AllowedTools:     mustJSONStringSlice([]string{"read_file", "run_command"}),
		SupportedModes:   mustJSONStringSlice([]string{model.InvocationModeAutonomous}),
	}

	version, err := svc.CreateWorkspacePresetVersion(context.Background(), req, "user-1")
	if err != nil {
		t.Fatalf("CreateWorkspacePresetVersion returned error: %v", err)
	}
	if version.Model == nil {
		t.Fatal("expected explicit blank model to be preserved")
	}
	if *version.Model != "" {
		t.Fatalf("expected explicit blank model, got %q", *version.Model)
	}

	presets := svc.ListAgentPresets(context.Background(), "ws-test")
	for _, preset := range presets {
		if preset.VersionKey != version.VersionKey {
			continue
		}
		if preset.Model == nil || *preset.Model != "" {
			t.Fatalf("expected listed workspace preset to keep blank model, got %+v", preset.Model)
		}
		return
	}
	t.Fatalf("expected workspace preset version %q in preset list", version.VersionKey)
}

func TestEnsureBuiltInAgent_PreservesWorkspacePresetVersionDuringReconcile(t *testing.T) {
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
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetReviewAgent)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	if err := workspacePresetVersionRepo.Create(context.Background(), &model.WorkspaceAgentPresetVersion{
		WorkspaceID:           "ws-test",
		FamilyKey:             model.AgentPresetReviewAgent,
		VersionKey:            "review_agent_workspace_checks",
		Label:                 "Workspace Checks",
		RuntimeKind:           "codex",
		Provider:              agentTestStringPtr(model.AgentModelProviderOpenAI),
		Model:                 agentTestStringPtr("gpt-5.5"),
		SystemPrompt:          agentTestStringPtr("Run extra workspace review checks."),
		InstructionSkills:     mustJSONStringSlice(nil),
		AllowedTools:          mustJSONStringSlice([]string{"read_file", "run_command"}),
		SupportedModes:        mustJSONStringSlice([]string{model.InvocationModeAutonomous, model.InvocationModeInteractive}),
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}); err != nil {
		t.Fatalf("create workspace preset version: %v", err)
	}

	versionKey := "review_agent_workspace_checks"
	if _, err := svc.UpdateAgent(context.Background(), "ws-test", systemAgent.ID, model.UpdateAgentRequest{
		PresetVersionKey: &versionKey,
	}, "user-1"); err != nil {
		t.Fatalf("pin workspace preset version: %v", err)
	}

	reconciled, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetReviewAgent)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent during reconcile returned error: %v", err)
	}
	if reconciled.PresetVersionKey != versionKey {
		t.Fatalf("expected workspace preset version %q to survive reconcile, got %q", versionKey, reconciled.PresetVersionKey)
	}
}

func TestEnsureBuiltInAgent_FallsBackWhenWorkspacePresetVersionDeleted(t *testing.T) {
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
	svc.SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	version := &model.WorkspaceAgentPresetVersion{
		WorkspaceID:           "ws-test",
		FamilyKey:             model.AgentPresetCodeBuilder,
		VersionKey:            "code_builder_workspace_tuned",
		Label:                 "Workspace Tuned",
		RuntimeKind:           "codex",
		Provider:              agentTestStringPtr(model.AgentModelProviderOpenAI),
		Model:                 agentTestStringPtr("gpt-5.5"),
		SystemPrompt:          agentTestStringPtr("Workspace tuned code builder."),
		InstructionSkills:     mustJSONStringSlice(nil),
		AllowedTools:          mustJSONStringSlice([]string{"read_file", "run_command"}),
		SupportedModes:        mustJSONStringSlice([]string{model.InvocationModeAutonomous}),
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	if err := workspacePresetVersionRepo.Create(context.Background(), version); err != nil {
		t.Fatalf("create workspace preset version: %v", err)
	}

	versionKey := version.VersionKey
	if _, err := svc.UpdateAgent(context.Background(), "ws-test", systemAgent.ID, model.UpdateAgentRequest{
		PresetVersionKey: &versionKey,
	}, "user-1"); err != nil {
		t.Fatalf("pin workspace preset version: %v", err)
	}
	if err := workspacePresetVersionRepo.Delete(context.Background(), "ws-test", version.ID); err != nil {
		t.Fatalf("soft delete workspace preset version: %v", err)
	}

	reconciled, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent during reconcile returned error: %v", err)
	}
	expected := defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder)
	if reconciled.PresetVersionKey != expected {
		t.Fatalf("expected fallback preset version %q, got %q", expected, reconciled.PresetVersionKey)
	}
}

func TestUpdateWorkspacePresetVersion(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
	}
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "test-openrouter-key", "", false, "", "")

	created, err := svc.CreateWorkspacePresetVersion(context.Background(), model.CreateWorkspaceAgentPresetVersionRequest{
		WorkspaceID:      "ws-test",
		FamilyKey:        model.AgentPresetCodeBuilder,
		Label:            "Engineering v2",
		SourceVersionKey: agentTestStringPtr(defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder)),
		RuntimeKind:      agentTestStringPtr("codex"),
		Provider:         agentTestStringPtr(model.AgentModelProviderOpenAI),
		Model:            agentTestStringPtr("gpt-5.5"),
		AllowedTools:     mustJSONStringSlice([]string{"read_file", "run_command"}),
		SupportedModes:   mustJSONStringSlice([]string{model.InvocationModeAutonomous}),
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateWorkspacePresetVersion returned error: %v", err)
	}
	if created.ID == nil {
		t.Fatal("expected workspace preset version id")
	}

	newLabel := "Engineering v3"
	newDescription := "Stricter code review rules."
	blankModel := ""
	updateReq := model.UpdateWorkspaceAgentPresetVersionRequest{
		Label:             &newLabel,
		Description:       &newDescription,
		Model:             &blankModel,
		AllowedTools:      mustJSONStringSlice([]string{"read_file"}),
		SupportedModes:    mustJSONStringSlice([]string{model.InvocationModeAutonomous, model.InvocationModeInteractive}),
		InstructionSkills: mustJSONStringSlice([]string{"system/engineering_planner_operating_rules"}),
	}
	updated, err := svc.UpdateWorkspacePresetVersion(context.Background(), "ws-test", *created.ID, updateReq, "user-2")
	if err != nil {
		t.Fatalf("UpdateWorkspacePresetVersion returned error: %v", err)
	}
	if updated.VersionLabel != newLabel {
		t.Fatalf("expected updated label %q, got %q", newLabel, updated.VersionLabel)
	}
	if updated.Description != newDescription {
		t.Fatalf("expected updated description %q, got %q", newDescription, updated.Description)
	}
	if updated.Model == nil || *updated.Model != "" {
		t.Fatalf("expected explicit blank model preserved, got %+v", updated.Model)
	}
	if !slices.Equal(updated.AllowedTools, []string{"read_files"}) {
		t.Fatalf("expected updated allowed tools, got %v", updated.AllowedTools)
	}
	if !slices.Equal(updated.SupportedModes, []string{model.InvocationModeAutonomous, model.InvocationModeInteractive}) {
		t.Fatalf("expected updated supported modes, got %v", updated.SupportedModes)
	}
}

func TestUpdateWorkspacePresetVersion_PersistsPromptOnlyEditAndExplicitEmptyLists(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
	}
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "test-openrouter-key", "", false, "", "")

	created, err := svc.CreateWorkspacePresetVersion(context.Background(), model.CreateWorkspaceAgentPresetVersionRequest{
		WorkspaceID:      "ws-test",
		FamilyKey:        model.AgentPresetEpicPlanner,
		Label:            "Workspace Atlas",
		SourceVersionKey: agentTestStringPtr(defaultPresetVersionKeyForPresetKey(model.AgentPresetEpicPlanner)),
		SystemPrompt:     agentTestStringPtr("Workspace atlas v1"),
		AllowedTools:     mustJSONStringSlice([]string{agentcontract.ToolUpdatePlan}),
		SupportedModes:   mustJSONStringSlice([]string{model.InvocationModeInteractive}),
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateWorkspacePresetVersion returned error: %v", err)
	}
	if created.ID == nil {
		t.Fatal("expected workspace preset version id")
	}

	updatedPrompt := "Workspace atlas v2"
	updated, err := svc.UpdateWorkspacePresetVersion(context.Background(), "ws-test", *created.ID, model.UpdateWorkspaceAgentPresetVersionRequest{
		SystemPrompt: &updatedPrompt,
	}, "user-2")
	if err != nil {
		t.Fatalf("UpdateWorkspacePresetVersion returned error: %v", err)
	}
	if updated.SystemPrompt == nil || *updated.SystemPrompt != updatedPrompt {
		t.Fatalf("expected updated system prompt %q, got %+v", updatedPrompt, updated.SystemPrompt)
	}

	blankPreamble := ""
	cleared, err := svc.UpdateWorkspacePresetVersion(context.Background(), "ws-test", *created.ID, model.UpdateWorkspaceAgentPresetVersionRequest{
		InstructionPreamble: &blankPreamble,
		InstructionSkills:   mustJSONStringSlice([]string{}),
		AllowedTools:        mustJSONStringSlice([]string{}),
	}, "user-2")
	if err != nil {
		t.Fatalf("UpdateWorkspacePresetVersion clear-lists returned error: %v", err)
	}
	if len(cleared.AllowedTools) != 0 {
		t.Fatalf("expected empty allowed tools, got %v", cleared.AllowedTools)
	}
	if len(cleared.InstructionSkills) != 0 {
		t.Fatalf("expected empty instruction skills, got %v", cleared.InstructionSkills)
	}

	stored, err := workspacePresetVersionRepo.GetByID(context.Background(), "ws-test", *created.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if stored == nil {
		t.Fatal("expected stored workspace preset version")
	}
	if len(parseJSONStringSlice(stored.AllowedTools)) != 0 {
		t.Fatalf("expected stored allowed tools to be empty, got %v", parseJSONStringSlice(stored.AllowedTools))
	}
	if len(parseJSONStringSlice(stored.InstructionSkills)) != 0 {
		t.Fatalf("expected stored instruction skills to be empty, got %v", parseJSONStringSlice(stored.InstructionSkills))
	}
}

func TestCreateWorkspacePresetVersion_SystemPromptWinsOverInstructionMetadata(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
	}
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "test-openrouter-key", "", false, "", "")

	rawPrompt := "Use this exact custom system prompt."
	legacyPreamble := "This legacy preamble must not replace the raw prompt."
	created, err := svc.CreateWorkspacePresetVersion(context.Background(), model.CreateWorkspaceAgentPresetVersionRequest{
		WorkspaceID:           "ws-test",
		FamilyKey:             model.AgentPresetEpicPlanner,
		Label:                 "Workspace Atlas Raw Prompt",
		SourceVersionKey:      agentTestStringPtr(defaultPresetVersionKeyForPresetKey(model.AgentPresetEpicPlanner)),
		SystemPrompt:          &rawPrompt,
		InstructionPreamble:   &legacyPreamble,
		InstructionSkills:     mustJSONStringSlice([]string{"engineering_planner_operating_rules"}),
		AllowedTools:          mustJSONStringSlice([]string{agentcontract.ToolUpdatePlan}),
		SupportedModes:        mustJSONStringSlice([]string{model.InvocationModeInteractive}),
		DefaultInvocationMode: agentTestStringPtr(model.InvocationModeInteractive),
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateWorkspacePresetVersion returned error: %v", err)
	}
	if created.SystemPrompt == nil || *created.SystemPrompt != rawPrompt {
		t.Fatalf("expected raw system prompt %q, got %+v", rawPrompt, created.SystemPrompt)
	}
}

func TestUpdateWorkspacePresetVersion_SystemPromptWinsOverInstructionMetadata(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
	}
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "test-openrouter-key", "", false, "", "")

	created, err := svc.CreateWorkspacePresetVersion(context.Background(), model.CreateWorkspaceAgentPresetVersionRequest{
		WorkspaceID:      "ws-test",
		FamilyKey:        model.AgentPresetEpicPlanner,
		Label:            "Workspace Atlas",
		SourceVersionKey: agentTestStringPtr(defaultPresetVersionKeyForPresetKey(model.AgentPresetEpicPlanner)),
		SystemPrompt:     agentTestStringPtr("Workspace atlas v1"),
		AllowedTools:     mustJSONStringSlice([]string{agentcontract.ToolUpdatePlan}),
		SupportedModes:   mustJSONStringSlice([]string{model.InvocationModeInteractive}),
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateWorkspacePresetVersion returned error: %v", err)
	}
	if created.ID == nil {
		t.Fatal("expected workspace preset version id")
	}

	rawPrompt := "Use this exact updated system prompt."
	legacyPreamble := "This update preamble must not replace the raw prompt."
	updated, err := svc.UpdateWorkspacePresetVersion(context.Background(), "ws-test", *created.ID, model.UpdateWorkspaceAgentPresetVersionRequest{
		SystemPrompt:        &rawPrompt,
		InstructionPreamble: &legacyPreamble,
		InstructionSkills:   mustJSONStringSlice([]string{"engineering_planner_operating_rules"}),
	}, "user-2")
	if err != nil {
		t.Fatalf("UpdateWorkspacePresetVersion returned error: %v", err)
	}
	if updated.SystemPrompt == nil || *updated.SystemPrompt != rawPrompt {
		t.Fatalf("expected raw system prompt %q, got %+v", rawPrompt, updated.SystemPrompt)
	}
}

func TestUpdateWorkspacePresetVersion_PropagatesToPinnedSystemAgent(t *testing.T) {
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
	svc.SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}

	created, err := svc.CreateWorkspacePresetVersion(context.Background(), model.CreateWorkspaceAgentPresetVersionRequest{
		WorkspaceID:      "ws-test",
		FamilyKey:        model.AgentPresetCodeBuilder,
		Label:            "Workspace Forge",
		SourceVersionKey: agentTestStringPtr(defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder)),
		RuntimeKind:      agentTestStringPtr("codex"),
		Provider:         agentTestStringPtr(model.AgentModelProviderOpenAI),
		Model:            agentTestStringPtr("gpt-5.5"),
		ExecutionConfig:  json.RawMessage(`{"reasoning_effort":"low","service_tier":"flex"}`),
		SystemPrompt:     agentTestStringPtr("Workspace forge v1"),
		AllowedTools:     mustJSONStringSlice([]string{"read_file", "run_command"}),
		SupportedModes:   mustJSONStringSlice([]string{model.InvocationModeAutonomous}),
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateWorkspacePresetVersion returned error: %v", err)
	}
	if created.ID == nil {
		t.Fatal("expected workspace preset version id")
	}

	if _, err := svc.UpdateAgent(context.Background(), "ws-test", systemAgent.ID, model.UpdateAgentRequest{
		PresetVersionKey: &created.VersionKey,
	}, "user-1"); err != nil {
		t.Fatalf("pin workspace preset version: %v", err)
	}

	updatedPrompt := "Workspace forge v2"
	blankModel := ""
	updated, err := svc.UpdateWorkspacePresetVersion(context.Background(), "ws-test", *created.ID, model.UpdateWorkspaceAgentPresetVersionRequest{
		Model:                 &blankModel,
		SystemPrompt:          &updatedPrompt,
		ExecutionConfig:       json.RawMessage(`{"reasoning_effort":"high","service_tier":"fast"}`),
		AllowedTools:          mustJSONStringSlice([]string{"read_file"}),
		SupportedModes:        mustJSONStringSlice([]string{model.InvocationModeAutonomous, model.InvocationModeInteractive}),
		DefaultInvocationMode: agentTestStringPtr(model.InvocationModeInteractive),
	}, "user-2")
	if err != nil {
		t.Fatalf("UpdateWorkspacePresetVersion returned error: %v", err)
	}
	if updated.Model == nil || *updated.Model != "" {
		t.Fatalf("expected updated preset model to be explicit blank, got %+v", updated.Model)
	}

	refetched, err := agentRepo.GetByID(context.Background(), "ws-test", systemAgent.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if refetched == nil {
		t.Fatal("expected pinned system agent")
	}
	if refetched.Model != nil {
		t.Fatalf("expected pinned agent model to be cleared, got %+v", refetched.Model)
	}
	if refetched.SystemPrompt == nil || *refetched.SystemPrompt != updatedPrompt {
		t.Fatalf("expected pinned agent prompt %q, got %+v", updatedPrompt, refetched.SystemPrompt)
	}
	if strings.TrimSpace(string(refetched.ExecutionConfig)) != `{"reasoning_effort":"high","service_tier":"fast"}` {
		t.Fatalf("expected pinned agent execution config to update, got %s", refetched.ExecutionConfig)
	}
	if !slices.Equal(parseJSONStringSlice(refetched.AllowedTools), []string{"read_files"}) {
		t.Fatalf("expected pinned agent allowed tools to update, got %v", parseJSONStringSlice(refetched.AllowedTools))
	}
	if refetched.DefaultInvocationMode != model.InvocationModeInteractive {
		t.Fatalf("expected pinned agent default invocation mode %q, got %q", model.InvocationModeInteractive, refetched.DefaultInvocationMode)
	}
}

func TestUpdateWorkspacePresetVersion_RejectsInvalidRouting(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
	}
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "test-openrouter-key", "", false, "", "")

	created, err := svc.CreateWorkspacePresetVersion(context.Background(), model.CreateWorkspaceAgentPresetVersionRequest{
		WorkspaceID:      "ws-test",
		FamilyKey:        model.AgentPresetCodeBuilder,
		Label:            "Workspace Forge",
		SourceVersionKey: agentTestStringPtr(defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder)),
		RuntimeKind:      agentTestStringPtr("codex"),
		Provider:         agentTestStringPtr(model.AgentModelProviderOpenAI),
		Model:            agentTestStringPtr("gpt-5.5"),
		AllowedTools:     mustJSONStringSlice([]string{"read_file"}),
		SupportedModes:   mustJSONStringSlice([]string{model.InvocationModeAutonomous}),
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateWorkspacePresetVersion returned error: %v", err)
	}
	if created.ID == nil {
		t.Fatal("expected workspace preset version id")
	}

	invalidProvider := model.AgentModelProviderAnthropic
	if _, err := svc.UpdateWorkspacePresetVersion(context.Background(), "ws-test", *created.ID, model.UpdateWorkspaceAgentPresetVersionRequest{
		Provider: &invalidProvider,
	}, "user-2"); err == nil {
		t.Fatal("expected invalid routing error")
	}

	stored, err := workspacePresetVersionRepo.GetByID(context.Background(), "ws-test", *created.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if stored == nil {
		t.Fatal("expected stored workspace preset version")
	}
	if stored.Provider == nil || *stored.Provider != model.AgentModelProviderOpenAI {
		t.Fatalf("expected provider to remain openai after failed update, got %+v", stored.Provider)
	}
}

func TestDeleteWorkspacePresetVersionRejectsPinnedVersion(t *testing.T) {
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
	svc.SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

	systemAgent, err := svc.ensureBuiltInAgent(context.Background(), "ws-test", "user-1", model.AgentPresetCodeBuilder)
	if err != nil {
		t.Fatalf("ensureBuiltInAgent returned error: %v", err)
	}
	created, err := svc.CreateWorkspacePresetVersion(context.Background(), model.CreateWorkspaceAgentPresetVersionRequest{
		WorkspaceID:      "ws-test",
		FamilyKey:        model.AgentPresetCodeBuilder,
		Label:            "Pinned Version",
		SourceVersionKey: agentTestStringPtr(defaultPresetVersionKeyForPresetKey(model.AgentPresetCodeBuilder)),
		RuntimeKind:      agentTestStringPtr("codex"),
		AllowedTools:     mustJSONStringSlice([]string{"read_file", "run_command"}),
		SupportedModes:   mustJSONStringSlice([]string{model.InvocationModeAutonomous}),
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateWorkspacePresetVersion returned error: %v", err)
	}
	if created.ID == nil {
		t.Fatal("expected workspace preset version id")
	}

	if _, err := svc.UpdateAgent(context.Background(), "ws-test", systemAgent.ID, model.UpdateAgentRequest{
		PresetVersionKey: &created.VersionKey,
	}, "user-1"); err != nil {
		t.Fatalf("pin workspace preset version: %v", err)
	}
	err = svc.DeleteWorkspacePresetVersion(context.Background(), "ws-test", *created.ID, "user-1")
	if !errors.Is(err, ErrWorkspacePresetVersionPinned) {
		t.Fatalf("expected pinned error, got %v", err)
	}
}

func TestDeleteWorkspacePresetVersionSoftDeletesAndHidesFromList(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	svc := &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
	}

	created, err := svc.CreateWorkspacePresetVersion(context.Background(), model.CreateWorkspaceAgentPresetVersionRequest{
		WorkspaceID:           "ws-test",
		FamilyKey:             model.AgentPresetReviewAgent,
		Label:                 "Delete Me",
		SourceVersionKey:      agentTestStringPtr(defaultPresetVersionKeyForPresetKey(model.AgentPresetReviewAgent)),
		RuntimeKind:           agentTestStringPtr("codex"),
		AllowedTools:          mustJSONStringSlice([]string{"read_file", "run_command"}),
		SupportedModes:        mustJSONStringSlice([]string{model.InvocationModeAutonomous, model.InvocationModeInteractive}),
		DefaultInvocationMode: agentTestStringPtr(model.InvocationModeInteractive),
	}, "user-1")
	if err != nil {
		t.Fatalf("CreateWorkspacePresetVersion returned error: %v", err)
	}
	if created.ID == nil {
		t.Fatal("expected workspace preset version id")
	}

	if err := svc.DeleteWorkspacePresetVersion(context.Background(), "ws-test", *created.ID, "user-1"); err != nil {
		t.Fatalf("DeleteWorkspacePresetVersion returned error: %v", err)
	}

	presets := svc.ListAgentPresets(context.Background(), "ws-test")
	for _, preset := range presets {
		if preset.VersionKey == created.VersionKey {
			t.Fatalf("expected deleted preset version %q to be hidden from list", created.VersionKey)
		}
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
	svc.SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")

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
		InstructionSkills:     mustJSONStringSlice(nil),
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
		if preset.Scope == "product" {
			if preset.ID != nil {
				t.Fatalf("expected product preset %q to have nil id, got %q", preset.VersionKey, *preset.ID)
			}
			continue
		}
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
		if preset.ID == nil || strings.TrimSpace(*preset.ID) == "" {
			t.Fatal("expected workspace preset version to include id")
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
			icon_key TEXT NOT NULL DEFAULT '',
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			source_template_id TEXT,
			source_template_key TEXT NOT NULL DEFAULT '',
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			active_version_id TEXT,
			role TEXT,
			status TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			skills BLOB NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			tools BLOB NOT NULL DEFAULT '[]',
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			allowed_commands BLOB NOT NULL DEFAULT '[]',
			allowed_targets BLOB NOT NULL DEFAULT x'5b5d',
			schedule TEXT,
			target_selector TEXT,
			trigger_events BLOB NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'never',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_team_access (
			agent_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (agent_id, team_id)
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
			instruction_preamble TEXT,
			instruction_skills BLOB NOT NULL DEFAULT '[]',
			available_skills BLOB NOT NULL DEFAULT '[]',
			instruction_template_version TEXT NOT NULL DEFAULT '',
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			allowed_targets BLOB NOT NULL DEFAULT '[]',
			supported_modes BLOB NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_by TEXT,
			updated_by TEXT,
			last_edited_at DATETIME,
			deleted_at DATETIME,
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
