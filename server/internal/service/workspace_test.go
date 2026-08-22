package service

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// createDeleteStubTables creates the extra tables referenced by the workspace
// cascade-delete transaction that are not present in the shared newTestDB helper.
func createDeleteStubTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	stubs := []string{
		`CREATE TABLE IF NOT EXISTS reward_profiles (id TEXT PRIMARY KEY, workspace_member_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS support_messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS support_widget_sessions (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS support_widget_installations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pm_import_jobs (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pm_team_estimate_settings (id TEXT PRIMARY KEY, team_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pm_team_field_visibility (id TEXT PRIMARY KEY, team_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pm_team_repo_defaults (id TEXT PRIMARY KEY, team_id TEXT NOT NULL, repository_id TEXT)`,
		`CREATE TABLE IF NOT EXISTS task_delivery_targets (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, repository_id TEXT, integration_id TEXT)`,
		`CREATE TABLE IF NOT EXISTS task_git_links (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, repository_id TEXT, integration_id TEXT)`,
		`CREATE TABLE IF NOT EXISTS git_integrations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			organization_id TEXT,
			provider TEXT NOT NULL DEFAULT 'github',
			display_name TEXT NOT NULL DEFAULT '',
			credential_mode TEXT NOT NULL DEFAULT 'github_app',
			credential_id TEXT,
			account_login TEXT,
			base_url TEXT,
			installation_id TEXT,
			app_id TEXT,
			webhook_secret TEXT,
			access_token TEXT NOT NULL DEFAULT '',
			default_commit_author_name TEXT,
			default_commit_author_email TEXT,
			active BOOLEAN NOT NULL DEFAULT 1,
			deleted_at DATETIME,
			last_synced_at DATETIME,
			last_sync_error TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS git_repositories (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			integration_id TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT 'github',
			base_url TEXT,
			external_id TEXT NOT NULL DEFAULT '',
			full_name TEXT NOT NULL DEFAULT '',
			default_branch TEXT NOT NULL DEFAULT 'main',
			permissions TEXT NOT NULL DEFAULT '{}',
			private BOOLEAN NOT NULL DEFAULT 1,
			archived BOOLEAN NOT NULL DEFAULT 0,
			selected BOOLEAN NOT NULL DEFAULT 1,
			active BOOLEAN NOT NULL DEFAULT 1,
			deleted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS agent_run_artifacts (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, cached_input_tokens INTEGER NOT NULL DEFAULT 0, input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS agents (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS agent_handoffs (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS workspace_key_history (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS team_workspace_memberships (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			workspace_member_id TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS workspace_managers (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS job_role_criteria (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS workspace_billing (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL UNIQUE,
			plan TEXT NOT NULL DEFAULT 'free',
			status TEXT NOT NULL DEFAULT 'active',
			stripe_customer_id TEXT,
			stripe_subscription_id TEXT,
			stripe_price_id TEXT,
			billing_interval TEXT NOT NULL DEFAULT 'monthly',
			included_credits INTEGER NOT NULL DEFAULT 1000,
			credits_used INTEGER NOT NULL DEFAULT 0,
			on_demand_enabled BOOLEAN NOT NULL DEFAULT 0,
			on_demand_blocks_invoiced INTEGER NOT NULL DEFAULT 0,
			current_period_start DATETIME,
			current_period_end DATETIME,
			trial_ends_at DATETIME,
			pending_plan TEXT,
			pending_billing_interval TEXT,
			pending_change_at DATETIME,
			cancel_at_period_end BOOLEAN NOT NULL DEFAULT 0,
			canceled_at DATETIME,
			billing_notice_type TEXT,
			billing_notice_message TEXT,
			billing_notice_at DATETIME,
			payment_failed_at DATETIME,
			trial_will_end_at DATETIME,
			last_stripe_event_id TEXT,
			payment_method_id TEXT,
			billing_owner_user_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS organization_billing (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL UNIQUE,
			stripe_customer_id TEXT,
			default_payment_method_id TEXT,
			founder_plan_enabled BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range stubs {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create stub table: %v", err)
		}
	}
}

// newWorkspaceTestHarness creates a fresh DB, repos, and WorkspaceService for
// a single test. It also seeds a default owner user.
func newWorkspaceTestHarness(t *testing.T) (*gorm.DB, *WorkspaceService) {
	t.Helper()
	db := newTestDB(t)
	wsRepo := repository.NewWorkspaceRepository(db)
	attachRepo := repository.NewPMAttachmentRepository(db)
	svc := NewWorkspaceService(wsRepo, attachRepo, nil)
	seedUser(t, db, "owner-1", "owner@test.com", "Test Owner", "hashed")
	return db, svc
}

func newWorkspaceDefaultsTestHarness(t *testing.T) (*gorm.DB, *WorkspaceService, *repository.PMAutomationRepository, *repository.PMWorkflowRepository) {
	t.Helper()
	db := newTestDB(t)
	for _, stmt := range []string{
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
			model_tier TEXT NOT NULL DEFAULT '',
			skills TEXT NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools TEXT NOT NULL DEFAULT '[]',
			allowed_commands TEXT NOT NULL DEFAULT '[]',
			allowed_targets TEXT NOT NULL DEFAULT '[]',
			schedule TEXT,
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create agents table: %v", err)
		}
	}
	wsRepo := repository.NewWorkspaceRepository(db)
	attachRepo := repository.NewPMAttachmentRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	automationRepo := repository.NewPMAutomationRepository(db)
	agentRepo := repository.NewAgentRepository(db)

	pmWorkflowService := NewPMWorkflowService(workflowRepo, nil, labelRepo, nil)
	pmAutomationService := NewPMAutomationService(automationRepo, nil, nil, nil, workflowRepo, nil, nil, nil)
	agentService := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: NewPMActivityService(repository.NewPMActivityRepository(db)),
		wsPublisher: nil,
	}
	agentService.SetModelProviderConfig("", "test-openai-key", "test-openrouter-key", "", false, "", "")
	defaults := NewCompositeDefaultsInitializer(pmWorkflowService, pmAutomationService, agentService)
	svc := NewWorkspaceService(wsRepo, attachRepo, nil, defaults)

	seedUser(t, db, "owner-1", "owner@test.com", "Test Owner", "hashed")
	return db, svc, automationRepo, workflowRepo
}

func TestWorkspaceService_Create(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	websiteURL := "example.com"
	req := model.CreateWorkspaceRequest{
		Name:         "My Workspace",
		Slug:         "my-workspace",
		WorkspaceKey: "MYW",
		WebsiteURL:   &websiteURL,
		Timezone:     "America/New_York",
	}
	ws, err := svc.Create(ctx, req, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if ws.Name != "My Workspace" {
		t.Errorf("Name = %q, want %q", ws.Name, "My Workspace")
	}
	if ws.Slug != "my-workspace" {
		t.Errorf("Slug = %q, want %q", ws.Slug, "my-workspace")
	}
	if ws.OwnerID != "owner-1" {
		t.Errorf("OwnerID = %q, want %q", ws.OwnerID, "owner-1")
	}
	if ws.Timezone != "America/New_York" {
		t.Errorf("Timezone = %q, want %q", ws.Timezone, "America/New_York")
	}
	if ws.WebsiteURL == nil || *ws.WebsiteURL != "https://example.com" {
		t.Errorf("WebsiteURL = %v, want %q", ws.WebsiteURL, "https://example.com")
	}
	if ws.Role != "owner" {
		t.Errorf("Role = %q, want %q", ws.Role, "owner")
	}
	if ws.ID == "" {
		t.Error("ID should be non-empty")
	}
}

type failingWorkspaceSetupInitializer struct{}

func (failingWorkspaceSetupInitializer) InitializeSetupGoals(context.Context, string, string, []string) error {
	return errors.New("temporary setup persistence failure")
}

func TestWorkspaceService_CreateDoesNotReportFailureAfterWorkspaceCommit(t *testing.T) {
	db, svc := newWorkspaceTestHarness(t)
	if err := db.Exec(`CREATE TABLE setup_intents (workspace_id TEXT PRIMARY KEY, goal_keys TEXT NOT NULL, created_at DATETIME, updated_at DATETIME)`).Error; err != nil {
		t.Fatalf("create setup intents: %v", err)
	}
	svc.SetSetupInitializer(failingWorkspaceSetupInitializer{})

	created, err := svc.Create(context.Background(), model.CreateWorkspaceRequest{
		Name: "Committed Workspace", Slug: "committed-workspace", WorkspaceKey: "COM", SetupGoals: []string{model.SetupGoalInternalDocs},
	}, "owner-1")
	if err != nil {
		t.Fatalf("Create returned an error after setup initialization failure: %v", err)
	}
	if created == nil {
		t.Fatal("expected committed workspace response")
	}
	var count int64
	if err := db.Model(&model.Workspace{}).Where("id = ?", created.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("committed workspace count = %d, err = %v", count, err)
	}
	if err := db.Table("setup_intents").Where("workspace_id = ?", created.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("recoverable setup intent count = %d, err = %v", count, err)
	}
}

func TestWorkspaceService_Create_SeedsDefaultEpicAutomations(t *testing.T) {
	_, svc, automationRepo, workflowRepo := newWorkspaceDefaultsTestHarness(t)
	ctx := context.Background()

	ws, err := svc.Create(ctx, model.CreateWorkspaceRequest{
		Name:         "Automation Defaults",
		Slug:         "automation-defaults",
		WorkspaceKey: "AUTO",
	}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	epicStates, err := workflowRepo.ListEpicStates(ctx, ws.ID)
	if err != nil {
		t.Fatalf("ListEpicStates: %v", err)
	}

	stateByType := map[string]string{}
	for _, state := range epicStates {
		if _, exists := stateByType[state.StateType]; !exists {
			stateByType[state.StateType] = state.ID
		}
	}

	automations, err := automationRepo.ListByWorkspace(ctx, ws.ID)
	if err != nil {
		t.Fatalf("ListByWorkspace: %v", err)
	}

	automationByType := map[string]model.PMAutomation{}
	for _, automation := range automations {
		automationByType[automation.AutomationType] = automation
	}

	autoStart, ok := automationByType[model.PMAutomationTypeEpicAutoStart]
	if !ok {
		t.Fatal("expected epic_auto_start automation to be seeded")
	}
	if !autoStart.Enabled {
		t.Fatal("expected epic_auto_start automation to be enabled")
	}
	if autoStart.TeamID != nil {
		t.Fatalf("expected epic_auto_start automation to be workspace-scoped, got team_id=%v", autoStart.TeamID)
	}
	if autoStart.ConfigStateID == nil || *autoStart.ConfigStateID != stateByType[model.PMStateTypeStarted] {
		t.Fatalf("epic_auto_start config_state_id = %v, want %q", autoStart.ConfigStateID, stateByType[model.PMStateTypeStarted])
	}

	autoComplete, ok := automationByType[model.PMAutomationTypeEpicAutoComplete]
	if !ok {
		t.Fatal("expected epic_auto_complete automation to be seeded")
	}
	if !autoComplete.Enabled {
		t.Fatal("expected epic_auto_complete automation to be enabled")
	}
	if autoComplete.TeamID != nil {
		t.Fatalf("expected epic_auto_complete automation to be workspace-scoped, got team_id=%v", autoComplete.TeamID)
	}
	if autoComplete.ConfigStateID == nil || *autoComplete.ConfigStateID != stateByType[model.PMStateTypeDone] {
		t.Fatalf("epic_auto_complete config_state_id = %v, want %q", autoComplete.ConfigStateID, stateByType[model.PMStateTypeDone])
	}
}

func TestWorkspaceService_Create_SeedsSystemPresetAgents(t *testing.T) {
	db, svc, _, _ := newWorkspaceDefaultsTestHarness(t)
	ctx := context.Background()

	ws, err := svc.Create(ctx, model.CreateWorkspaceRequest{
		Name:         "Planner Defaults",
		Slug:         "planner-defaults",
		WorkspaceKey: "PLAN",
	}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	agentRepo := repository.NewAgentRepository(db)
	agents, err := agentRepo.List(ctx, ws.ID)
	if err != nil {
		t.Fatalf("List agents: %v", err)
	}
	if len(agents) == 0 {
		t.Fatal("expected at least one seeded agent")
	}

	systemByPreset := map[string]model.Agent{}
	for _, agent := range agents {
		if !agent.IsSystem {
			continue
		}
		systemByPreset[agent.PresetKey] = agent
	}
	if len(systemByPreset) != len(builtInPresetKeys()) {
		t.Fatalf("expected %d system preset agents, got %d", len(builtInPresetKeys()), len(systemByPreset))
	}

	planner, ok := systemByPreset[model.AgentPresetEpicPlanner]
	if !ok {
		t.Fatal("expected epic planner system agent")
	}
	if planner.Name != defaultSystemEpicPlannerName {
		t.Fatalf("name = %q, want %q", planner.Name, defaultSystemEpicPlannerName)
	}
	// Managed system agents store SystemPrompt as nil; it's materialized on read via materializeAgentSystemPrompt.
	// Verify the instruction template version is set, indicating the prompt is managed.
	if planner.InstructionTemplateVersion == "" {
		t.Fatal("expected seeded planner to have an instruction template version")
	}
	if planner.DefaultInvocationMode != model.InvocationModeInteractive {
		t.Fatalf("default_invocation_mode = %q, want %q", planner.DefaultInvocationMode, model.InvocationModeInteractive)
	}
	if planner.PresetVersionKey != defaultPresetVersionKeyForPresetKey(model.AgentPresetEpicPlanner) {
		t.Fatalf("preset_version_key = %q, want %q", planner.PresetVersionKey, defaultPresetVersionKeyForPresetKey(model.AgentPresetEpicPlanner))
	}
}

func TestWorkspaceService_Create_EmptyNameFails(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	req := model.CreateWorkspaceRequest{
		Name:         "",
		Slug:         "some-slug",
		WorkspaceKey: "SOM",
	}
	_, err := svc.Create(ctx, req, "owner-1")
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}

func TestWorkspaceService_Create_EmptySlugFails(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	req := model.CreateWorkspaceRequest{
		Name:         "Valid Name",
		Slug:         "",
		WorkspaceKey: "VAL",
	}
	_, err := svc.Create(ctx, req, "owner-1")
	if err == nil {
		t.Fatal("expected error for empty slug, got nil")
	}
}

func TestWorkspaceService_Create_DuplicateSlugDeduplicates(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	req := model.CreateWorkspaceRequest{
		Name:         "First",
		Slug:         "dup-slug",
		WorkspaceKey: "DUP",
	}
	_, err := svc.Create(ctx, req, "owner-1")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	req2 := model.CreateWorkspaceRequest{
		Name:         "Second",
		Slug:         "dup-slug",
		WorkspaceKey: "DUPE",
	}
	ws, err := svc.Create(ctx, req2, "owner-1")
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if ws.Slug != "dup-slug-2" {
		t.Fatalf("expected deduplicated slug dup-slug-2, got %q", ws.Slug)
	}
}

func TestWorkspaceService_List(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "WS A", Slug: "ws-a", WorkspaceKey: "WSA"}, "owner-1")
	if err != nil {
		t.Fatalf("Create WS A: %v", err)
	}
	_, err = svc.Create(ctx, model.CreateWorkspaceRequest{Name: "WS B", Slug: "ws-b", WorkspaceKey: "WSB"}, "owner-1")
	if err != nil {
		t.Fatalf("Create WS B: %v", err)
	}

	list, err := svc.List(ctx, "owner-1", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List returned %d workspaces, want 2", len(list))
	}

	slugs := map[string]bool{}
	for _, ws := range list {
		slugs[ws.Slug] = true
	}
	if !slugs["ws-a"] || !slugs["ws-b"] {
		t.Errorf("expected slugs ws-a and ws-b, got %v", slugs)
	}
}

func TestWorkspaceService_List_Empty(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	list, err := svc.List(ctx, "owner-1", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List returned %d workspaces, want 0", len(list))
	}
}

func TestWorkspaceService_GetBySlug(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Slug Test", Slug: "slug-test", WorkspaceKey: "SLG"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ws, err := svc.GetBySlug(ctx, "slug-test")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if ws.ID != created.ID {
		t.Errorf("GetBySlug ID = %q, want %q", ws.ID, created.ID)
	}
	if ws.Name != "Slug Test" {
		t.Errorf("GetBySlug Name = %q, want %q", ws.Name, "Slug Test")
	}
}

func TestWorkspaceService_GetBySlug_NotFound(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	_, err := svc.GetBySlug(ctx, "nonexistent-slug")
	if err == nil {
		t.Fatal("expected error for unknown slug, got nil")
	}
}

func TestWorkspaceService_GetByID(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "ID Test", Slug: "id-test", WorkspaceKey: "IDT"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ws, err := svc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if ws.ID != created.ID {
		t.Errorf("GetByID ID = %q, want %q", ws.ID, created.ID)
	}
	if ws.Name != "ID Test" {
		t.Errorf("GetByID Name = %q, want %q", ws.Name, "ID Test")
	}
	if ws.Slug != "id-test" {
		t.Errorf("GetByID Slug = %q, want %q", ws.Slug, "id-test")
	}
}

func TestWorkspaceService_GetByID_NotFound(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	_, err := svc.GetByID(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for unknown ID, got nil")
	}
}

func TestWorkspaceService_Update(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Before", Slug: "update-test", WorkspaceKey: "UPD"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "After"
	updated, err := svc.Update(ctx, created.ID, model.UpdateWorkspaceRequest{
		Name: &newName,
	}, "owner-1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "After" {
		t.Errorf("updated Name = %q, want %q", updated.Name, "After")
	}

	// Verify the change persisted via GetByID
	ws, err := svc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if ws.Name != "After" {
		t.Errorf("persisted Name = %q, want %q", ws.Name, "After")
	}
}

func TestWorkspaceService_Update_Description(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Desc Test", Slug: "desc-test", WorkspaceKey: "DSC"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	desc := "A new description"
	updated, err := svc.Update(ctx, created.ID, model.UpdateWorkspaceRequest{
		Description: &desc,
	}, "owner-1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description == nil || *updated.Description != "A new description" {
		t.Errorf("updated Description = %v, want %q", updated.Description, "A new description")
	}
}

func TestWorkspaceService_Update_WebsiteURL(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Website Test", Slug: "website-test", WorkspaceKey: "WEB"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	websiteURL := "helpin.ai"
	updated, err := svc.Update(ctx, created.ID, model.UpdateWorkspaceRequest{
		WebsiteURL: &websiteURL,
	}, "owner-1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.WebsiteURL == nil || *updated.WebsiteURL != "https://helpin.ai" {
		t.Errorf("updated WebsiteURL = %v, want %q", updated.WebsiteURL, "https://helpin.ai")
	}

	clearWebsite := ""
	cleared, err := svc.Update(ctx, created.ID, model.UpdateWorkspaceRequest{
		WebsiteURL: &clearWebsite,
	}, "owner-1")
	if err != nil {
		t.Fatalf("Clear website URL: %v", err)
	}
	if cleared.WebsiteURL != nil {
		t.Errorf("cleared WebsiteURL = %v, want nil", cleared.WebsiteURL)
	}
}

func TestWorkspaceService_Update_Timezone(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "TZ Test", Slug: "tz-test", WorkspaceKey: "TZT", Timezone: "UTC"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	tz := "Europe/London"
	updated, err := svc.Update(ctx, created.ID, model.UpdateWorkspaceRequest{
		Timezone: &tz,
	}, "owner-1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Timezone != "Europe/London" {
		t.Errorf("updated Timezone = %q, want %q", updated.Timezone, "Europe/London")
	}
}

func TestWorkspaceService_Delete(t *testing.T) {
	db, svc := newWorkspaceTestHarness(t)
	createDeleteStubTables(t, db)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Delete Me", Slug: "delete-me", WorkspaceKey: "DEL"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = svc.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify workspace is gone
	_, err = svc.GetByID(ctx, created.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}

	// Verify it no longer appears in list
	list, err := svc.List(ctx, "owner-1", "")
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List returned %d workspaces after delete, want 0", len(list))
	}
}

func TestWorkspaceService_DeleteCancelsActiveSubscriptionImmediately(t *testing.T) {
	db, svc := newWorkspaceTestHarness(t)
	createDeleteStubTables(t, db)
	ctx := context.Background()
	gateway := &fakeBillingGateway{}
	billingRepo := repository.NewBillingRepository(db)
	billingSvc := NewBillingService(billingRepo, gateway, nil)
	svc.SetBillingService(billingSvc)

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Paid Delete", Slug: "paid-delete", WorkspaceKey: "PDL"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := billingRepo.UpsertWorkspaceBilling(ctx, &model.WorkspaceBilling{
		WorkspaceID:          created.ID,
		Plan:                 model.BillingPlanStarter,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_paid"),
		StripeSubscriptionID: billingStringPtr("sub_paid"),
		BillingInterval:      "monthly",
		IncludedCredits:      5000,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(gateway.immediateCancels) != 1 || gateway.immediateCancels[0].SubscriptionID != "sub_paid" {
		t.Fatalf("unexpected immediate cancels: %#v", gateway.immediateCancels)
	}
	var count int64
	if err := db.Model(&model.WorkspaceBilling{}).Where("workspace_id = ?", created.ID).Count(&count).Error; err != nil {
		t.Fatalf("count workspace billing: %v", err)
	}
	if count != 0 {
		t.Fatalf("workspace billing rows after delete = %d, want 0", count)
	}
}

func TestWorkspaceService_DeleteStopsWhenSubscriptionCancellationFails(t *testing.T) {
	db, svc := newWorkspaceTestHarness(t)
	createDeleteStubTables(t, db)
	ctx := context.Background()
	gateway := &fakeBillingGateway{cancelErr: errors.New("stripe unavailable")}
	billingRepo := repository.NewBillingRepository(db)
	billingSvc := NewBillingService(billingRepo, gateway, nil)
	svc.SetBillingService(billingSvc)

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Paid Delete Fail", Slug: "paid-delete-fail", WorkspaceKey: "PDF"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := billingRepo.UpsertWorkspaceBilling(ctx, &model.WorkspaceBilling{
		WorkspaceID:          created.ID,
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_paid"),
		StripeSubscriptionID: billingStringPtr("sub_paid"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	err = svc.Delete(ctx, created.ID)
	if err == nil {
		t.Fatal("expected delete to fail when subscription cancellation fails")
	}
	if _, getErr := svc.GetByID(ctx, created.ID); getErr != nil {
		t.Fatalf("workspace should remain after failed billing cancellation: %v", getErr)
	}
}

func TestWorkspaceService_Delete_NonexistentDoesNotError(t *testing.T) {
	db, svc := newWorkspaceTestHarness(t)
	createDeleteStubTables(t, db)
	ctx := context.Background()

	// Deleting a workspace that doesn't exist should not return an error
	// because the DELETE statements simply affect zero rows.
	err := svc.Delete(ctx, "does-not-exist")
	if err != nil {
		t.Fatalf("Delete nonexistent: %v", err)
	}
}

func TestWorkspaceService_Delete_ReleasesRepoClaimsButKeepsSharedIntegrationForSiblingWorkspace(t *testing.T) {
	db, svc := newWorkspaceTestHarness(t)
	createDeleteStubTables(t, db)
	ctx := context.Background()

	seedUser(t, db, "owner-2", "owner2@test.com", "Owner Two", "hashed")
	orgID := "org-shared"
	if err := db.Create(&model.Organization{
		ID:      orgID,
		Name:    "Shared Org",
		Slug:    "shared-org",
		OwnerID: "owner-1",
	}).Error; err != nil {
		t.Fatalf("create organization: %v", err)
	}

	wsA, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Workspace A", Slug: "workspace-a", WorkspaceKey: "WSA", OrganizationID: orgID}, "owner-1")
	if err != nil {
		t.Fatalf("create workspace A: %v", err)
	}
	wsB, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Workspace B", Slug: "workspace-b", WorkspaceKey: "WSB", OrganizationID: orgID}, "owner-2")
	if err != nil {
		t.Fatalf("create workspace B: %v", err)
	}

	if err := db.Exec(`INSERT INTO git_integrations (
		id, workspace_id, organization_id, provider, display_name, credential_mode, access_token, active, created_at, updated_at
	) VALUES (?, ?, ?, 'github', 'GitHub Shared', 'github_app', '', 1, datetime('now'), datetime('now'))`, "gi-1", wsA.ID, orgID).Error; err != nil {
		t.Fatalf("insert integration: %v", err)
	}
	if err := db.Exec(`INSERT INTO git_repositories (
		id, workspace_id, integration_id, provider, external_id, full_name, default_branch, permissions, private, archived, selected, active, created_at, updated_at
	) VALUES
		(?, ?, ?, 'github', '101', 'acme/repo-a', 'main', '{}', 1, 0, 1, 1, datetime('now'), datetime('now')),
		(?, ?, ?, 'github', '102', 'acme/repo-b', 'main', '{}', 1, 0, 1, 1, datetime('now'), datetime('now'))`,
		"repo-a", wsA.ID, "gi-1", "repo-b", wsB.ID, "gi-1").Error; err != nil {
		t.Fatalf("insert repositories: %v", err)
	}

	if err := svc.Delete(ctx, wsA.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var integration struct {
		Active    bool
		DeletedAt *string
	}
	if err := db.Raw(`SELECT active, deleted_at FROM git_integrations WHERE id = ?`, "gi-1").Scan(&integration).Error; err != nil {
		t.Fatalf("load integration: %v", err)
	}
	if !integration.Active {
		t.Fatalf("expected integration to stay active for sibling workspace")
	}
	if integration.DeletedAt != nil {
		t.Fatalf("expected integration deleted_at to remain nil, got %v", *integration.DeletedAt)
	}

	var repoCountA int64
	if err := db.Raw(`SELECT COUNT(*) FROM git_repositories WHERE workspace_id = ?`, wsA.ID).Scan(&repoCountA).Error; err != nil {
		t.Fatalf("count workspace A repos: %v", err)
	}
	if repoCountA != 0 {
		t.Fatalf("expected workspace A repo claims to be removed, got %d", repoCountA)
	}

	var repoCountB int64
	if err := db.Raw(`SELECT COUNT(*) FROM git_repositories WHERE workspace_id = ?`, wsB.ID).Scan(&repoCountB).Error; err != nil {
		t.Fatalf("count workspace B repos: %v", err)
	}
	if repoCountB != 1 {
		t.Fatalf("expected workspace B repo claims to remain, got %d", repoCountB)
	}
}

func TestWorkspaceService_Delete_DeactivatesIntegrationWhenDeletingLastWorkspaceInOrg(t *testing.T) {
	db, svc := newWorkspaceTestHarness(t)
	createDeleteStubTables(t, db)
	ctx := context.Background()

	orgID := "org-last"
	if err := db.Create(&model.Organization{
		ID:      orgID,
		Name:    "Last Org",
		Slug:    "last-org",
		OwnerID: "owner-1",
	}).Error; err != nil {
		t.Fatalf("create organization: %v", err)
	}

	ws, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Solo Workspace", Slug: "solo-workspace", WorkspaceKey: "SLO", OrganizationID: orgID}, "owner-1")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	if err := db.Exec(`INSERT INTO git_integrations (
		id, workspace_id, organization_id, provider, display_name, credential_mode, access_token, active, created_at, updated_at
	) VALUES (?, ?, ?, 'github', 'GitHub Solo', 'github_app', '', 1, datetime('now'), datetime('now'))`, "gi-last", ws.ID, orgID).Error; err != nil {
		t.Fatalf("insert integration: %v", err)
	}
	if err := db.Exec(`INSERT INTO git_repositories (
		id, workspace_id, integration_id, provider, external_id, full_name, default_branch, permissions, private, archived, selected, active, created_at, updated_at
	) VALUES (?, ?, ?, 'github', '201', 'acme/solo', 'main', '{}', 1, 0, 1, 1, datetime('now'), datetime('now'))`,
		"repo-last", ws.ID, "gi-last").Error; err != nil {
		t.Fatalf("insert repository: %v", err)
	}

	if err := svc.Delete(ctx, ws.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var repoCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM git_repositories WHERE workspace_id = ?`, ws.ID).Scan(&repoCount).Error; err != nil {
		t.Fatalf("count repo claims: %v", err)
	}
	if repoCount != 0 {
		t.Fatalf("expected repo claims to be removed, got %d", repoCount)
	}

	var integration struct {
		Active    bool
		DeletedAt *string
	}
	if err := db.Raw(`SELECT active, deleted_at FROM git_integrations WHERE id = ?`, "gi-last").Scan(&integration).Error; err != nil {
		t.Fatalf("load integration: %v", err)
	}
	if integration.Active {
		t.Fatalf("expected integration to be deactivated")
	}
	if integration.DeletedAt == nil || *integration.DeletedAt == "" {
		t.Fatalf("expected integration deleted_at to be set")
	}
}

func TestWorkspaceService_Create_WithOrganizationID(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	req := model.CreateWorkspaceRequest{
		Name:           "Org WS",
		Slug:           "org-ws",
		WorkspaceKey:   "ORG",
		OrganizationID: "org-123",
	}
	ws, err := svc.Create(ctx, req, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ws.OrganizationID == nil || *ws.OrganizationID != "org-123" {
		t.Errorf("OrganizationID = %v, want %q", ws.OrganizationID, "org-123")
	}
}

func TestWorkspaceService_Create_WithDescription(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	desc := "A workspace description"
	req := model.CreateWorkspaceRequest{
		Name:         "Desc WS",
		Slug:         "desc-ws",
		WorkspaceKey: "DWS",
		Description:  &desc,
	}
	ws, err := svc.Create(ctx, req, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ws.Description == nil || *ws.Description != "A workspace description" {
		t.Errorf("Description = %v, want %q", ws.Description, "A workspace description")
	}
}

func TestWorkspaceService_GetMyRole(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Role Test", Slug: "role-test", WorkspaceKey: "ROL"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	role, err := svc.GetMyRole(ctx, created.ID, "owner-1")
	if err != nil {
		t.Fatalf("GetMyRole: %v", err)
	}
	if role != "owner" {
		t.Errorf("role = %q, want %q", role, "owner")
	}
}

func TestWorkspaceService_GetMyRole_NonMember(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()
	seedUser(t, newTestDB(t), "other-user", "other@test.com", "Other User", "hashed")

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Role Test 2", Slug: "role-test-2", WorkspaceKey: "RLT"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	role, err := svc.GetMyRole(ctx, created.ID, "not-a-member")
	if err != nil {
		t.Fatalf("GetMyRole: %v", err)
	}
	if role != "" {
		t.Errorf("role = %q, want empty string for non-member", role)
	}
}

func TestWorkspaceService_GetMyMembership(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Membership Test", Slug: "membership-test", WorkspaceKey: "MEM"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	member, err := svc.GetMyMembership(ctx, created.ID, "owner-1")
	if err != nil {
		t.Fatalf("GetMyMembership: %v", err)
	}
	if member.Role != "owner" {
		t.Errorf("member Role = %q, want %q", member.Role, "owner")
	}
	if member.WorkspaceID != created.ID {
		t.Errorf("member WorkspaceID = %q, want %q", member.WorkspaceID, created.ID)
	}
	if member.Email != "owner@test.com" {
		t.Errorf("member Email = %q, want %q", member.Email, "owner@test.com")
	}
}

func TestWorkspaceService_GetMyMembership_NotFound(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Membership NF", Slug: "membership-nf", WorkspaceKey: "MNF"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.GetMyMembership(ctx, created.ID, "not-a-member")
	if err == nil {
		t.Fatal("expected error for non-member, got nil")
	}
}

func TestWorkspaceService_UploadLogo_NilS3Client(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Logo Test", Slug: "logo-test", WorkspaceKey: "LOG"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.UploadLogo(ctx, created.ID, nil, 0, "image/png")
	if err == nil {
		t.Fatal("expected error when s3Client is nil, got nil")
	}
}
