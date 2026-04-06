package service

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// addSettingsExtraTables creates tables required by SettingsRepository.GetAll
// that are not part of newTestDB (which covers the core PM/notification schema).
func addSettingsExtraTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	extras := []string{
		`CREATE TABLE IF NOT EXISTS team_workspace_memberships (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			team_id TEXT NOT NULL,
			workspace_member_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(team_id, workspace_member_id)
		)`,
		`CREATE TABLE IF NOT EXISTS reward_profiles (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_member_id TEXT NOT NULL UNIQUE,
			manager_member_id TEXT,
			role TEXT NOT NULL DEFAULT 'employee',
			job_role TEXT NOT NULL DEFAULT '',
			hire_date TEXT,
			base_salary REAL NOT NULL DEFAULT 0,
			active_for_bonus BOOLEAN NOT NULL DEFAULT 1,
			active_for_evaluation BOOLEAN NOT NULL DEFAULT 1,
			is_account_owner BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS workspace_managers (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			person_id TEXT NOT NULL,
			can_create_goals BOOLEAN NOT NULL DEFAULT 0,
			can_score_performance BOOLEAN NOT NULL DEFAULT 0,
			reporting_to TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS job_role_criteria (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			job_role TEXT NOT NULL,
			criteria_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			question TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			weight INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS bonus_tiers (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			tier TEXT NOT NULL,
			min_score INTEGER NOT NULL DEFAULT 0,
			max_score INTEGER NOT NULL DEFAULT 100,
			salary_multiplier REAL NOT NULL DEFAULT 0,
			description TEXT,
			editable BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(workspace_id, tier)
		)`,
		`CREATE TABLE IF NOT EXISTS pm_team_estimate_settings (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			team_id TEXT NOT NULL UNIQUE,
			enabled BOOLEAN NOT NULL DEFAULT 0,
			scale TEXT NOT NULL DEFAULT 'linear',
			extended BOOLEAN NOT NULL DEFAULT 0,
			allow_zero BOOLEAN NOT NULL DEFAULT 0,
			count_unestimated_as_one BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS pm_team_field_visibility (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			team_id TEXT NOT NULL UNIQUE,
			priority BOOLEAN NOT NULL DEFAULT 1,
			task_type BOOLEAN NOT NULL DEFAULT 1,
			severity BOOLEAN NOT NULL DEFAULT 1,
			labels BOOLEAN NOT NULL DEFAULT 1,
			epic BOOLEAN NOT NULL DEFAULT 1,
			sprint BOOLEAN NOT NULL DEFAULT 1,
			estimate BOOLEAN NOT NULL DEFAULT 1,
			due_date BOOLEAN NOT NULL DEFAULT 1,
			blocked BOOLEAN NOT NULL DEFAULT 1,
			delivery BOOLEAN NOT NULL DEFAULT 1,
			dev_history BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS pm_team_repo_defaults (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			team_id TEXT NOT NULL UNIQUE,
			repository_id TEXT NOT NULL,
			base_branch TEXT NOT NULL DEFAULT 'main',
			branch_template TEXT NOT NULL DEFAULT '{task_key}-{slug}',
			auto_sync_states BOOLEAN NOT NULL DEFAULT 1,
			review_state_id TEXT,
			done_state_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range extras {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create extra table: %v", err)
		}
	}
}

// newSettingsService creates a SettingsService wired to a fresh in-memory DB
// that has all required tables. Returns the service, the raw DB, and the
// SettingsRepository (useful for seeding).
func newSettingsService(t *testing.T) (*SettingsService, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	addSettingsExtraTables(t, db)
	repo := repository.NewSettingsRepository(db)
	svc := NewSettingsService(repo, nil, nil)
	return svc, db
}

// ---------------------------------------------------------------------------
// GetAll
// ---------------------------------------------------------------------------

func TestGetAll_WithSeededSettings(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")
	seedSettings(t, db, "s1", "ws1")

	cfg, err := svc.GetAll(ctx, "ws1")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Settings == nil {
		t.Fatal("expected non-nil settings")
	}
	if cfg.Settings.WorkspaceID != "ws1" {
		t.Fatalf("expected workspace_id ws1, got %q", cfg.Settings.WorkspaceID)
	}
	// Verify defaults come through from the table DDL.
	if cfg.Settings.SprintDurationWeeks != 2 {
		t.Fatalf("expected sprint_duration_weeks=2, got %d", cfg.Settings.SprintDurationWeeks)
	}
	if !cfg.Settings.NotificationsEnabled {
		t.Fatal("expected notifications_enabled=true by default")
	}
	if cfg.Settings.AutoCalculateBonuses {
		t.Fatal("expected auto_calculate_bonuses=false by default")
	}
	if cfg.Settings.TeamWeight != 50 {
		t.Fatalf("expected team_weight=50, got %d", cfg.Settings.TeamWeight)
	}
	// Empty workspace should have zero teams, people, etc.
	if len(cfg.Teams) != 0 {
		t.Fatalf("expected 0 teams, got %d", len(cfg.Teams))
	}
}

func TestGetAll_AutoInitializesWhenNoSettings(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")
	// Deliberately do NOT seed settings.

	cfg, err := svc.GetAll(ctx, "ws1")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if cfg == nil || cfg.Settings == nil {
		t.Fatal("expected GetAll to auto-initialize settings")
	}
	if cfg.Settings.WorkspaceID != "ws1" {
		t.Fatalf("expected workspace_id ws1, got %q", cfg.Settings.WorkspaceID)
	}
	// Auto-initialized row should carry defaults.
	if cfg.Settings.SprintDurationWeeks != 2 {
		t.Fatalf("expected sprint_duration_weeks=2, got %d", cfg.Settings.SprintDurationWeeks)
	}
}

func TestGetAll_WithTeamsAndMembers(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")
	seedSettings(t, db, "s1", "ws1")
	seedWorkspaceMember(t, db, "wm1", "ws1", "u1", "owner@test.com", "Owner", "admin")

	// Insert a team directly.
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle, created_at, updated_at) VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"t1", "ws1", "Engineering", "engineering")

	cfg, err := svc.GetAll(ctx, "ws1")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(cfg.Teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(cfg.Teams))
	}
	if cfg.Teams[0].Name != "Engineering" {
		t.Fatalf("expected team name Engineering, got %q", cfg.Teams[0].Name)
	}
	if cfg.Teams[0].Handle == nil || *cfg.Teams[0].Handle != "engineering" {
		t.Fatal("expected team handle 'engineering'")
	}
	// People should include the workspace member.
	if len(cfg.People) != 1 {
		t.Fatalf("expected 1 person, got %d", len(cfg.People))
	}
	if cfg.People[0].Email != "owner@test.com" {
		t.Fatalf("expected person email owner@test.com, got %q", cfg.People[0].Email)
	}
}

// ---------------------------------------------------------------------------
// Initialize
// ---------------------------------------------------------------------------

func TestInitialize_CreatesDefaultSettings(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	settings, err := svc.Initialize(ctx, "ws1")
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if settings == nil {
		t.Fatal("expected non-nil settings")
	}
	if settings.WorkspaceID != "ws1" {
		t.Fatalf("expected workspace_id ws1, got %q", settings.WorkspaceID)
	}
	if settings.SprintDurationWeeks != 2 {
		t.Fatalf("expected sprint_duration_weeks=2, got %d", settings.SprintDurationWeeks)
	}
}

func TestInitialize_IdempotentOnDuplicate(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	first, err := svc.Initialize(ctx, "ws1")
	if err != nil {
		t.Fatalf("first Initialize: %v", err)
	}
	second, err := svc.Initialize(ctx, "ws1")
	if err != nil {
		t.Fatalf("second Initialize: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected idempotent init (same ID), got %q vs %q", first.ID, second.ID)
	}
}

// ---------------------------------------------------------------------------
// UpdateSystem
// ---------------------------------------------------------------------------

func TestUpdateSystem_ChangeSprintDuration(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")
	seedSettings(t, db, "s1", "ws1")

	weeks := 3
	updated, err := svc.UpdateSystem(ctx, "ws1", model.UpdateSystemSettingsRequest{
		SprintDurationWeeks: &weeks,
	})
	if err != nil {
		t.Fatalf("UpdateSystem: %v", err)
	}
	if updated.SprintDurationWeeks != 3 {
		t.Fatalf("expected sprint_duration_weeks=3, got %d", updated.SprintDurationWeeks)
	}
	// Other defaults should be untouched.
	if !updated.NotificationsEnabled {
		t.Fatal("expected notifications_enabled to remain true")
	}
	if updated.TeamWeight != 50 {
		t.Fatalf("expected team_weight=50, got %d", updated.TeamWeight)
	}
}

func TestUpdateSystem_MultipleFields(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")
	seedSettings(t, db, "s1", "ws1")

	weeks := 4
	notifications := false
	autoBonuses := true
	weight := 75

	updated, err := svc.UpdateSystem(ctx, "ws1", model.UpdateSystemSettingsRequest{
		SprintDurationWeeks:  &weeks,
		NotificationsEnabled: &notifications,
		AutoCalculateBonuses: &autoBonuses,
		TeamWeight:           &weight,
	})
	if err != nil {
		t.Fatalf("UpdateSystem: %v", err)
	}
	if updated.SprintDurationWeeks != 4 {
		t.Fatalf("expected sprint_duration_weeks=4, got %d", updated.SprintDurationWeeks)
	}
	if updated.NotificationsEnabled {
		t.Fatal("expected notifications_enabled=false")
	}
	if !updated.AutoCalculateBonuses {
		t.Fatal("expected auto_calculate_bonuses=true")
	}
	if updated.TeamWeight != 75 {
		t.Fatalf("expected team_weight=75, got %d", updated.TeamWeight)
	}
}

func TestUpdateSystem_NoRowReturnsError(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")
	// No settings seeded -- UpdateSystem should fail because there is no row to update.

	weeks := 3
	_, err := svc.UpdateSystem(ctx, "ws1", model.UpdateSystemSettingsRequest{
		SprintDurationWeeks: &weeks,
	})
	if err == nil {
		t.Fatal("expected error when updating non-existent settings")
	}
}

// ---------------------------------------------------------------------------
// CreateTeam
// ---------------------------------------------------------------------------

func TestCreateTeam_Success(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")
	seedWorkspaceMember(t, db, "wm1", "ws1", "u1", "owner@test.com", "Owner", "admin")

	team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
	}, "u1")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if team == nil {
		t.Fatal("expected non-nil team")
	}
	if team.Name != "Backend" {
		t.Fatalf("expected team name Backend, got %q", team.Name)
	}
	if team.Handle == nil || *team.Handle != "backend" {
		t.Fatal("expected auto-generated handle 'backend'")
	}
	if team.WorkspaceID != "ws1" {
		t.Fatalf("expected workspace_id ws1, got %q", team.WorkspaceID)
	}
}

func TestCreateTeam_CustomHandle(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	handle := "be-team"
	team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
		Handle:      &handle,
	}, "")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if team.Handle == nil || *team.Handle != "be-team" {
		t.Fatalf("expected handle be-team, got %v", team.Handle)
	}
}

func TestCreateTeam_DuplicateHandleFails(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	_, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
	}, "")
	if err != nil {
		t.Fatalf("first CreateTeam: %v", err)
	}

	_, err = svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
	}, "")
	if err == nil {
		t.Fatal("expected error for duplicate team handle")
	}
}

func TestCreateTeam_MissingNameFails(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	_, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "",
	}, "")
	if err == nil {
		t.Fatal("expected error for empty team name")
	}
}

func TestCreateTeam_MissingWorkspaceIDFails(t *testing.T) {
	svc, _ := newSettingsService(t)
	ctx := context.Background()

	_, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "",
		Name:        "Backend",
	}, "")
	if err == nil {
		t.Fatal("expected error for empty workspace_id")
	}
}

// ---------------------------------------------------------------------------
// UpdateTeam
// ---------------------------------------------------------------------------

func TestUpdateTeam_ChangeName(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
	}, "")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	newName := "Platform"
	updated, err := svc.UpdateTeam(ctx, team.ID, model.UpdateTeamRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("UpdateTeam: %v", err)
	}
	if updated.Name != "Platform" {
		t.Fatalf("expected name Platform, got %q", updated.Name)
	}
}

func TestUpdateTeam_EmptyNameFails(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
	}, "")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	empty := "   "
	_, err = svc.UpdateTeam(ctx, team.ID, model.UpdateTeamRequest{
		Name: &empty,
	})
	if err == nil {
		t.Fatal("expected error for empty team name")
	}
}

func TestUpdateTeam_NotFoundFails(t *testing.T) {
	svc, _ := newSettingsService(t)
	ctx := context.Background()

	newName := "Whatever"
	_, err := svc.UpdateTeam(ctx, "nonexistent-team-id", model.UpdateTeamRequest{
		Name: &newName,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent team")
	}
}

// ---------------------------------------------------------------------------
// DeleteTeam
// ---------------------------------------------------------------------------

func TestDeleteTeam_Success(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
	}, "")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	if err := svc.DeleteTeam(ctx, team.ID); err != nil {
		t.Fatalf("DeleteTeam: %v", err)
	}

	// Verify team is gone by checking GetAll.
	seedSettings(t, db, "s1", "ws1")
	cfg, err := svc.GetAll(ctx, "ws1")
	if err != nil {
		t.Fatalf("GetAll after delete: %v", err)
	}
	if len(cfg.Teams) != 0 {
		t.Fatalf("expected 0 teams after delete, got %d", len(cfg.Teams))
	}
}

// ---------------------------------------------------------------------------
// AddTeamMember / UpdateTeamMember / RemoveTeamMember
// ---------------------------------------------------------------------------

func TestAddTeamMember_DefaultRole(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")
	seedWorkspaceMember(t, db, "wm1", "ws1", "u1", "owner@test.com", "Owner", "admin")

	team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
	}, "")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	membership, err := svc.AddTeamMember(ctx, team.ID, model.AddTeamMemberRequest{
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("AddTeamMember: %v", err)
	}
	if membership == nil {
		t.Fatal("expected non-nil membership")
	}
	if membership.Role != "member" {
		t.Fatalf("expected default role 'member', got %q", membership.Role)
	}
}

func TestAddTeamMember_InvalidRoleFails(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
	}, "")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	_, err = svc.AddTeamMember(ctx, team.ID, model.AddTeamMemberRequest{
		UserID: "u1",
		Role:   "superadmin",
	})
	if err == nil {
		t.Fatal("expected error for invalid team member role")
	}
}

func TestAddTeamMember_EmptyUserIDFails(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
	}, "")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	_, err = svc.AddTeamMember(ctx, team.ID, model.AddTeamMemberRequest{
		UserID: "",
	})
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
}

func TestUpdateTeamMember_InvalidRoleFails(t *testing.T) {
	svc, _ := newSettingsService(t)
	ctx := context.Background()

	badRole := "superadmin"
	_, err := svc.UpdateTeamMember(ctx, "t1", "u1", nil, model.UpdateTeamMemberRequest{
		Role: &badRole,
	})
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestUpdateTeamMember_EmptyUserIDFails(t *testing.T) {
	svc, _ := newSettingsService(t)
	ctx := context.Background()

	role := "member"
	_, err := svc.UpdateTeamMember(ctx, "t1", "", nil, model.UpdateTeamMemberRequest{
		Role: &role,
	})
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
}

func TestRemoveTeamMember_EmptyUserIDFails(t *testing.T) {
	svc, _ := newSettingsService(t)
	ctx := context.Background()

	err := svc.RemoveTeamMember(ctx, "t1", "", nil)
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
}

// ---------------------------------------------------------------------------
// TeamEstimateSettings
// ---------------------------------------------------------------------------

func TestUpdateTeamEstimateSettings_InvalidScaleFails(t *testing.T) {
	svc, _ := newSettingsService(t)
	ctx := context.Background()

	bad := "unknown_scale"
	_, err := svc.UpdateTeamEstimateSettings(ctx, "t1", model.UpdateTeamEstimateSettingsRequest{
		Scale: &bad,
	})
	if err == nil {
		t.Fatal("expected error for invalid estimate scale")
	}
}

func TestUpdateTeamEstimateSettings_ValidScales(t *testing.T) {
	validScales := []string{"exponential", "fibonacci", "linear", "tshirt", "hours"}
	for _, scale := range validScales {
		t.Run(scale, func(t *testing.T) {
			svc, db := newSettingsService(t)
			ctx := context.Background()

			seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
			seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

			// Create a team to have a valid team_id.
			team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
				WorkspaceID: "ws1",
				Name:        "Team " + scale,
			}, "")
			if err != nil {
				t.Fatalf("CreateTeam: %v", err)
			}

			s := scale
			result, err := svc.UpdateTeamEstimateSettings(ctx, team.ID, model.UpdateTeamEstimateSettingsRequest{
				Scale: &s,
			})
			if err != nil {
				t.Fatalf("UpdateTeamEstimateSettings(%s): %v", scale, err)
			}
			if result.Scale != scale {
				t.Fatalf("expected scale %q, got %q", scale, result.Scale)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TeamRepoDefault
// ---------------------------------------------------------------------------

func TestUpdateTeamRepoDefault_EmptyRepoIDFails(t *testing.T) {
	svc, _ := newSettingsService(t)
	ctx := context.Background()

	_, err := svc.UpdateTeamRepoDefault(ctx, "t1", model.UpdateTeamRepoDefaultRequest{
		RepositoryID: "",
	})
	if err == nil {
		t.Fatal("expected error for empty repository_id")
	}
}

func TestUpdateTeamRepoDefault_TrimsBranch(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()

	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

	team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws1",
		Name:        "Backend",
	}, "")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	// Insert a fake repository row so the FK doesn't block (SQLite doesn't enforce FK by default).
	mustExec(t, db, `INSERT INTO git_repositories (id, workspace_id, git_integration_id, repo_full_name, repo_name, repo_owner, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"repo1", "ws1", "gi1", "org/repo", "repo", "org")

	branch := "  develop  "
	tmpl := "  feat-{display_id}  "
	result, err := svc.UpdateTeamRepoDefault(ctx, team.ID, model.UpdateTeamRepoDefaultRequest{
		RepositoryID:   "repo1",
		BaseBranch:     &branch,
		BranchTemplate: &tmpl,
	})
	if err != nil {
		t.Fatalf("UpdateTeamRepoDefault: %v", err)
	}
	if result.BaseBranch != "develop" {
		t.Fatalf("expected trimmed base_branch 'develop', got %q", result.BaseBranch)
	}
	if result.BranchTemplate != "feat-{display_id}" {
		t.Fatalf("expected trimmed branch_template, got %q", result.BranchTemplate)
	}
}

// ---------------------------------------------------------------------------
// Slugify / Handle generation (indirect via CreateTeam)
// ---------------------------------------------------------------------------

func TestCreateTeam_HandleSlugification(t *testing.T) {
	tests := []struct {
		name           string
		teamName       string
		expectedHandle string
	}{
		{"simple", "Backend", "backend"},
		{"with spaces", "My Great Team", "my-great-team"},
		{"with special chars", "Team #1!", "team-1"},
		{"mixed case", "FrontEnd Dev", "frontend-dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db := newSettingsService(t)
			ctx := context.Background()

			seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
			seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")

			team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{
				WorkspaceID: "ws1",
				Name:        tt.teamName,
			}, "")
			if err != nil {
				t.Fatalf("CreateTeam(%q): %v", tt.teamName, err)
			}
			if team.Handle == nil || *team.Handle != tt.expectedHandle {
				got := "<nil>"
				if team.Handle != nil {
					got = *team.Handle
				}
				t.Fatalf("expected handle %q, got %q", tt.expectedHandle, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetAll for unknown workspace (no workspace row, no settings)
// ---------------------------------------------------------------------------

func TestGetAll_UnknownWorkspaceAutoInitializes(t *testing.T) {
	svc, _ := newSettingsService(t)
	ctx := context.Background()

	// GetAll for a workspace_id that has no rows should auto-initialize settings
	// and return an empty config (no teams, people, etc.).
	cfg, err := svc.GetAll(ctx, "nonexistent-ws")
	if err != nil {
		t.Fatalf("GetAll for unknown workspace: %v", err)
	}
	if cfg == nil || cfg.Settings == nil {
		t.Fatal("expected auto-initialized config for unknown workspace")
	}
	if cfg.Settings.WorkspaceID != "nonexistent-ws" {
		t.Fatalf("expected workspace_id nonexistent-ws, got %q", cfg.Settings.WorkspaceID)
	}
	if len(cfg.Teams) != 0 {
		t.Fatalf("expected 0 teams, got %d", len(cfg.Teams))
	}
}
