package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type taskTemplateTestEnv struct {
	svc       *PMTaskTemplateService
	wsID      string
	teamAID   string
	teamBID   string
	memberID  string
	ownerID   string
	adminID   string
	viewerID  string
	createdAt time.Time
}

func newTaskTemplateTestEnv(t *testing.T) taskTemplateTestEnv {
	t.Helper()
	db := newTestDB(t)
	now := time.Now()

	mustExec(t, db, `CREATE TABLE IF NOT EXISTS pm_task_templates (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		team_id TEXT,
		name TEXT NOT NULL,
		description TEXT,
		task_type TEXT,
		priority TEXT,
		severity TEXT,
		estimate INTEGER,
		label_ids TEXT,
		owner_member_id TEXT,
		epic_id TEXT,
		sprint_id TEXT,
		workflow_state_id TEXT,
		deadline TEXT,
		checklist_items TEXT,
		external_links TEXT,
		archived BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	)`)

	wsID := "ws-template-001"
	userID := "user-template-001"
	ownerUserID := "user-template-owner-001"
	adminUserID := "user-template-admin-001"
	viewerUserID := "user-template-viewer-001"
	memberID := "member-template-001"
	ownerID := "member-template-owner-001"
	adminID := "member-template-admin-001"
	viewerID := "member-template-viewer-001"
	teamAID := "team-template-a"
	teamBID := "team-template-b"

	seedUser(t, db, userID, "template-member@test.com", "Template Member", "hash")
	seedUser(t, db, ownerUserID, "template-owner@test.com", "Template Owner", "hash")
	seedUser(t, db, adminUserID, "template-admin@test.com", "Template Admin", "hash")
	seedUser(t, db, viewerUserID, "template-viewer@test.com", "Template Viewer", "hash")
	seedWorkspace(t, db, wsID, "Template Workspace", "template-ws", adminUserID)
	seedWorkspaceMember(t, db, memberID, wsID, userID, "template-member@test.com", "Template Member", model.RoleMember)
	seedWorkspaceMember(t, db, ownerID, wsID, ownerUserID, "template-owner@test.com", "Template Owner", model.RoleMember)
	seedWorkspaceMember(t, db, adminID, wsID, adminUserID, "template-admin@test.com", "Template Admin", model.RoleAdmin)
	seedWorkspaceMember(t, db, viewerID, wsID, viewerUserID, "template-viewer@test.com", "Template Viewer", model.RoleViewer)

	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		teamAID, wsID, "Engineering", now, now)
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		teamBID, wsID, "Support", now, now)
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"twm-template-member-a", teamAID, memberID, "member", now, now)
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"twm-template-owner-a", teamAID, ownerID, "owner", now, now)
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"twm-template-viewer-a", teamAID, viewerID, "member", now, now)

	mustExec(t, db, `INSERT INTO pm_task_templates (id, workspace_id, team_id, name, archived, created_at, updated_at) VALUES (?, ?, ?, ?, 0, ?, ?)`,
		"tmpl-shared", wsID, nil, "Shared", now, now)
	mustExec(t, db, `INSERT INTO pm_task_templates (id, workspace_id, team_id, name, archived, created_at, updated_at) VALUES (?, ?, ?, ?, 0, ?, ?)`,
		"tmpl-team-a", wsID, teamAID, "Team A", now, now)
	mustExec(t, db, `INSERT INTO pm_task_templates (id, workspace_id, team_id, name, archived, created_at, updated_at) VALUES (?, ?, ?, ?, 0, ?, ?)`,
		"tmpl-team-b", wsID, teamBID, "Team B", now, now)

	return taskTemplateTestEnv{
		svc:       NewPMTaskTemplateService(repository.NewPMTaskTemplateRepository(db), nil),
		wsID:      wsID,
		teamAID:   teamAID,
		teamBID:   teamBID,
		memberID:  memberID,
		ownerID:   ownerID,
		adminID:   adminID,
		viewerID:  viewerID,
		createdAt: now,
	}
}

func taskTemplateActorContext(role, workspaceID, memberID string, teams []authorization.TeamRole) context.Context {
	return authorization.WithActor(context.Background(), &authorization.Actor{
		WorkspaceID:       workspaceID,
		WorkspaceMemberID: memberID,
		Role:              role,
		TeamMemberships:   teams,
	})
}

func TestPMTaskTemplateServiceTeamAccess(t *testing.T) {
	env := newTaskTemplateTestEnv(t)

	t.Run("member only sees shared and own team templates", func(t *testing.T) {
		ctx := taskTemplateActorContext(model.RoleMember, env.wsID, env.memberID, []authorization.TeamRole{{TeamID: env.teamAID, Role: "member"}})

		templates, err := env.svc.ListByWorkspace(ctx, env.wsID, nil, true, nil)
		if err != nil {
			t.Fatalf("list templates: %v", err)
		}

		got := map[string]bool{}
		for _, tmpl := range templates {
			got[tmpl.ID] = true
		}
		if !got["tmpl-shared"] || !got["tmpl-team-a"] {
			t.Fatalf("expected shared and own team templates, got %#v", got)
		}
		if got["tmpl-team-b"] {
			t.Fatalf("did not expect other team template, got %#v", got)
		}
	})

	t.Run("only team owners can mutate team templates", func(t *testing.T) {
		memberCtx := taskTemplateActorContext(model.RoleMember, env.wsID, env.memberID, []authorization.TeamRole{{TeamID: env.teamAID, Role: "member"}})
		ownerCtx := taskTemplateActorContext(model.RoleMember, env.wsID, env.ownerID, []authorization.TeamRole{{TeamID: env.teamAID, Role: "owner"}})

		if _, err := env.svc.Create(memberCtx, model.CreateTaskTemplateRequest{WorkspaceID: env.wsID, TeamID: &env.teamAID, Name: "Member template"}); err == nil {
			t.Fatal("expected regular team member create to be forbidden")
		} else if _, ok := err.(*model.ErrForbidden); !ok {
			t.Fatalf("expected *model.ErrForbidden, got %T: %v", err, err)
		}

		if _, err := env.svc.Create(ownerCtx, model.CreateTaskTemplateRequest{WorkspaceID: env.wsID, TeamID: &env.teamAID, Name: "Owner template"}); err != nil {
			t.Fatalf("expected team owner create to succeed: %v", err)
		}
	})

	t.Run("shared template mutation is admin only", func(t *testing.T) {
		ownerCtx := taskTemplateActorContext(model.RoleMember, env.wsID, env.ownerID, []authorization.TeamRole{{TeamID: env.teamAID, Role: "owner"}})
		adminCtx := taskTemplateActorContext(model.RoleAdmin, env.wsID, env.adminID, nil)

		if _, err := env.svc.Create(ownerCtx, model.CreateTaskTemplateRequest{WorkspaceID: env.wsID, Name: "Team owner shared"}); err == nil {
			t.Fatal("expected team owner shared create to be forbidden")
		} else if _, ok := err.(*model.ErrForbidden); !ok {
			t.Fatalf("expected *model.ErrForbidden, got %T: %v", err, err)
		}

		if _, err := env.svc.Create(adminCtx, model.CreateTaskTemplateRequest{WorkspaceID: env.wsID, Name: "Admin shared"}); err != nil {
			t.Fatalf("expected admin shared create to succeed: %v", err)
		}
	})
}
