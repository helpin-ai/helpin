package service

import (
	"context"
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
		`CREATE TABLE IF NOT EXISTS reward_company_goals (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reward_goal_team_contributions (id TEXT PRIMARY KEY, goal_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reward_sprints (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reward_sprint_goals (id TEXT PRIMARY KEY, sprint_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reward_profiles (id TEXT PRIMARY KEY, workspace_member_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reward_bonus_calculations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reward_individual_checks (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reward_finance_settings (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reward_audit_log (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reward_goal_drafts (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reward_quarters (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS support_messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS support_widget_sessions (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS support_widget_installations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pm_import_jobs (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pm_team_estimate_settings (id TEXT PRIMARY KEY, team_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pm_team_field_visibility (id TEXT PRIMARY KEY, team_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pm_team_repo_defaults (id TEXT PRIMARY KEY, team_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS story_delivery_targets (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS story_git_links (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS git_integrations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS agent_run_artifacts (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS agents (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS agent_handoffs (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS team_workspace_memberships (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			workspace_member_id TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS workspace_managers (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS job_role_criteria (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS bonus_tiers (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
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
	wsRepo := repository.NewWorkspaceRepository(db)
	attachRepo := repository.NewPMAttachmentRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	automationRepo := repository.NewPMAutomationRepository(db)

	pmWorkflowService := NewPMWorkflowService(workflowRepo, nil, labelRepo)
	pmAutomationService := NewPMAutomationService(automationRepo, nil, nil, nil, workflowRepo, nil, nil)
	defaults := NewCompositeDefaultsInitializer(pmWorkflowService, pmAutomationService)
	svc := NewWorkspaceService(wsRepo, attachRepo, nil, defaults)

	seedUser(t, db, "owner-1", "owner@test.com", "Test Owner", "hashed")
	return db, svc, automationRepo, workflowRepo
}

func TestWorkspaceService_Create(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	req := model.CreateWorkspaceRequest{
		Name:     "My Workspace",
		Slug:     "my-workspace",
		Timezone: "America/New_York",
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
	if ws.Role != "owner" {
		t.Errorf("Role = %q, want %q", ws.Role, "owner")
	}
	if ws.ID == "" {
		t.Error("ID should be non-empty")
	}
}

func TestWorkspaceService_Create_SeedsDefaultEpicAutomations(t *testing.T) {
	_, svc, automationRepo, workflowRepo := newWorkspaceDefaultsTestHarness(t)
	ctx := context.Background()

	ws, err := svc.Create(ctx, model.CreateWorkspaceRequest{
		Name: "Automation Defaults",
		Slug: "automation-defaults",
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

func TestWorkspaceService_Create_EmptyNameFails(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	req := model.CreateWorkspaceRequest{
		Name: "",
		Slug: "some-slug",
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
		Name: "Valid Name",
		Slug: "",
	}
	_, err := svc.Create(ctx, req, "owner-1")
	if err == nil {
		t.Fatal("expected error for empty slug, got nil")
	}
}

func TestWorkspaceService_Create_DuplicateSlugFails(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	req := model.CreateWorkspaceRequest{
		Name: "First",
		Slug: "dup-slug",
	}
	_, err := svc.Create(ctx, req, "owner-1")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	req2 := model.CreateWorkspaceRequest{
		Name: "Second",
		Slug: "dup-slug",
	}
	_, err = svc.Create(ctx, req2, "owner-1")
	if err == nil {
		t.Fatal("expected error for duplicate slug, got nil")
	}
}

func TestWorkspaceService_List(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "WS A", Slug: "ws-a"}, "owner-1")
	if err != nil {
		t.Fatalf("Create WS A: %v", err)
	}
	_, err = svc.Create(ctx, model.CreateWorkspaceRequest{Name: "WS B", Slug: "ws-b"}, "owner-1")
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

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Slug Test", Slug: "slug-test"}, "owner-1")
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

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "ID Test", Slug: "id-test"}, "owner-1")
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

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Before", Slug: "update-test"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "After"
	updated, err := svc.Update(ctx, created.ID, model.UpdateWorkspaceRequest{
		Name: &newName,
	})
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

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Desc Test", Slug: "desc-test"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	desc := "A new description"
	updated, err := svc.Update(ctx, created.ID, model.UpdateWorkspaceRequest{
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description == nil || *updated.Description != "A new description" {
		t.Errorf("updated Description = %v, want %q", updated.Description, "A new description")
	}
}

func TestWorkspaceService_Update_Timezone(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "TZ Test", Slug: "tz-test", Timezone: "UTC"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	tz := "Europe/London"
	updated, err := svc.Update(ctx, created.ID, model.UpdateWorkspaceRequest{
		Timezone: &tz,
	})
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

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Delete Me", Slug: "delete-me"}, "owner-1")
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

func TestWorkspaceService_Create_WithOrganizationID(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	ctx := context.Background()

	req := model.CreateWorkspaceRequest{
		Name:           "Org WS",
		Slug:           "org-ws",
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
		Name:        "Desc WS",
		Slug:        "desc-ws",
		Description: &desc,
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

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Role Test", Slug: "role-test"}, "owner-1")
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

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Role Test 2", Slug: "role-test-2"}, "owner-1")
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

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Membership Test", Slug: "membership-test"}, "owner-1")
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

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Membership NF", Slug: "membership-nf"}, "owner-1")
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

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Logo Test", Slug: "logo-test"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.UploadLogo(ctx, created.ID, nil, 0, "image/png")
	if err == nil {
		t.Fatal("expected error when s3Client is nil, got nil")
	}
}
