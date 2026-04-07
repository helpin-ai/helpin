package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

func TestListWidgetTeammates_ShowsSupportAccessibleMembersIncludingOffline(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	mustExec(t, db, `CREATE TABLE workspace_module_grants (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		module TEXT NOT NULL,
		subject_type TEXT NOT NULL,
		subject_id TEXT NOT NULL,
		access_level TEXT NOT NULL DEFAULT 'member',
		created_by_id TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)

	const (
		workspaceID = "ws-widget-team"
		ownerID     = "user-owner"
		memberID    = "user-member"
		supportID   = "user-support"
		awayID      = "user-away"
		offlineID   = "user-offline"
		teamID      = "team-support"
	)

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hash")
	seedUser(t, db, memberID, "member@example.com", "Regular Member", "hash")
	seedUser(t, db, supportID, "support@example.com", "Support Member", "hash")
	seedUser(t, db, awayID, "away@example.com", "Away Member", "hash")
	seedUser(t, db, offlineID, "offline@example.com", "Offline Member", "hash")
	seedWorkspace(t, db, workspaceID, "Widget Team", "widget-team", ownerID)

	seedWorkspaceMember(t, db, "wm-owner", workspaceID, ownerID, "owner@example.com", "Owner User", model.RoleAdmin)
	seedWorkspaceMember(t, db, "wm-member", workspaceID, memberID, "member@example.com", "Regular Member", model.RoleMember)
	seedWorkspaceMember(t, db, "wm-support", workspaceID, supportID, "support@example.com", "Support Member", model.RoleMember)
	seedWorkspaceMember(t, db, "wm-away", workspaceID, awayID, "away@example.com", "Away Member", model.RoleMember)
	seedWorkspaceMember(t, db, "wm-offline", workspaceID, offlineID, "offline@example.com", "Offline Member", model.RoleMember)

	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle) VALUES (?, ?, ?, ?)`,
		teamID, workspaceID, "Support", "support")
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role) VALUES (?, ?, ?, ?)`,
		"twm-away", teamID, "wm-away", "member")

	mustExec(t, db, `INSERT INTO workspace_module_grants (id, workspace_id, module, subject_type, subject_id, access_level) VALUES (?, ?, ?, ?, ?, ?)`,
		"grant-support", workspaceID, model.ModuleSupport, model.ModuleGrantSubjectWorkspaceMember, "wm-support", model.ModuleGrantAccessLevelMember)
	mustExec(t, db, `INSERT INTO workspace_module_grants (id, workspace_id, module, subject_type, subject_id, access_level) VALUES (?, ?, ?, ?, ?, ?)`,
		"grant-team", workspaceID, model.ModuleSupport, model.ModuleGrantSubjectTeam, teamID, model.ModuleGrantAccessLevelMember)
	mustExec(t, db, `INSERT INTO workspace_module_grants (id, workspace_id, module, subject_type, subject_id, access_level) VALUES (?, ?, ?, ?, ?, ?)`,
		"grant-offline", workspaceID, model.ModuleSupport, model.ModuleGrantSubjectWorkspaceMember, "wm-offline", model.ModuleGrantAccessLevelMember)

	presence := websocket.NewPresenceState()
	if _, err := presence.SetAgentOnline(ctx, workspaceID, ownerID, "conn-owner"); err != nil {
		t.Fatalf("SetAgentOnline owner: %v", err)
	}
	if _, err := presence.SetAgentOnline(ctx, workspaceID, supportID, "conn-support"); err != nil {
		t.Fatalf("SetAgentOnline support: %v", err)
	}
	if _, err := presence.SetAgentOnline(ctx, workspaceID, awayID, "conn-away"); err != nil {
		t.Fatalf("SetAgentOnline away: %v", err)
	}
	if _, err := presence.SetAgentOffline(ctx, workspaceID, awayID, "conn-away"); err != nil {
		t.Fatalf("SetAgentOffline away: %v", err)
	}

	svc := NewSupportInboxService(
		repository.NewSupportConversationRepository(db),
		repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db),
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsHelpcenterRepository(db),
	)
	svc.SetWorkspaceRepo(repository.NewWorkspaceRepository(db))
	svc.SetPresenceProvider(presence)

	teammates := svc.listWidgetTeammates(ctx, workspaceID, 10)
	if len(teammates) != 4 {
		t.Fatalf("teammate count = %d, want 4", len(teammates))
	}

	if teammates[0].UserID != ownerID || teammates[0].Status != model.SupportTeammateStatusOnline {
		t.Fatalf("first teammate = %#v, want owner online", teammates[0])
	}
	if teammates[1].UserID != supportID || teammates[1].Status != model.SupportTeammateStatusOnline {
		t.Fatalf("second teammate = %#v, want support online", teammates[1])
	}
	if teammates[2].UserID != awayID || teammates[2].Status != model.SupportTeammateStatusAway {
		t.Fatalf("third teammate = %#v, want away teammate", teammates[2])
	}
	if teammates[3].UserID != offlineID || teammates[3].Status != model.SupportTeammateStatusOffline {
		t.Fatalf("fourth teammate = %#v, want offline teammate", teammates[3])
	}
}
