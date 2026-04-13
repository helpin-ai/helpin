package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestWorkspaceServiceRemoveMemberRevokesMembership(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	const (
		workspaceID    = "ws-remove"
		ownerUserID    = "user-owner"
		ownerMemberID  = "member-owner"
		targetUserID   = "user-target"
		targetMemberID = "member-target"
		teamID         = "team-1"
	)

	seedUser(t, db, ownerUserID, "owner@example.com", "Owner User", "hash")
	seedUser(t, db, targetUserID, "member@example.com", "Member User", "hash")
	seedWorkspace(t, db, workspaceID, "Workspace", "workspace", ownerUserID)
	seedWorkspaceMember(t, db, ownerMemberID, workspaceID, ownerUserID, "owner@example.com", "Owner User", model.RoleOwner)
	seedWorkspaceMember(t, db, targetMemberID, workspaceID, targetUserID, "member@example.com", "Member User", model.RoleMember)
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle, created_at, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		teamID, workspaceID, "Team Alpha", "alpha")
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"twm-1", teamID, targetMemberID, "member")
	mustExec(t, db, `UPDATE users SET default_workspace_id = ? WHERE id = ?`, workspaceID, targetUserID)
	mustExec(t, db, `INSERT INTO support_teammate_status_overrides (id, workspace_id, user_id, manual_status, created_at, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"override-1", workspaceID, targetUserID, "away")

	svc := NewWorkspaceService(repository.NewWorkspaceRepository(db), nil, nil)

	if err := svc.RemoveMember(ctx, workspaceID, ownerUserID, targetMemberID); err != nil {
		t.Fatalf("RemoveMember error = %v", err)
	}

	member, err := repository.NewWorkspaceRepository(db).GetMembershipByID(ctx, workspaceID, targetMemberID)
	if err != nil {
		t.Fatalf("GetMembershipByID error = %v", err)
	}
	if member == nil {
		t.Fatal("expected revoked membership row to remain")
	}
	if member.Status != model.WorkspaceMemberStatusRevoked {
		t.Fatalf("member.Status = %q, want %q", member.Status, model.WorkspaceMemberStatusRevoked)
	}

	activeMembership, err := repository.NewWorkspaceRepository(db).GetMembership(ctx, workspaceID, targetUserID)
	if err != nil {
		t.Fatalf("GetMembership error = %v", err)
	}
	if activeMembership != nil {
		t.Fatalf("expected active membership lookup to return nil, got %#v", activeMembership)
	}

	var teamMembershipCount int64
	if err := db.Table("team_workspace_memberships").Where("workspace_member_id = ?", targetMemberID).Count(&teamMembershipCount).Error; err != nil {
		t.Fatalf("count team memberships: %v", err)
	}
	if teamMembershipCount != 0 {
		t.Fatalf("team membership count = %d, want 0", teamMembershipCount)
	}

	var overrideCount int64
	if err := db.Table("support_teammate_status_overrides").Where("workspace_id = ? AND user_id = ?", workspaceID, targetUserID).Count(&overrideCount).Error; err != nil {
		t.Fatalf("count status overrides: %v", err)
	}
	if overrideCount != 0 {
		t.Fatalf("status override count = %d, want 0", overrideCount)
	}

	var defaultWorkspaceID *string
	if err := db.Raw(`SELECT default_workspace_id FROM users WHERE id = ?`, targetUserID).Scan(&defaultWorkspaceID).Error; err != nil {
		t.Fatalf("load default workspace: %v", err)
	}
	if defaultWorkspaceID != nil {
		t.Fatalf("default_workspace_id = %v, want nil", *defaultWorkspaceID)
	}
}

func TestWorkspaceServiceRemoveMemberGuards(t *testing.T) {
	tests := []struct {
		name              string
		actorRole         string
		targetRole        string
		actorSameAsTarget bool
		wantErr           string
	}{
		{
			name:              "cannot remove yourself",
			actorRole:         model.RoleOwner,
			targetRole:        model.RoleOwner,
			actorSameAsTarget: true,
			wantErr:           "cannot remove yourself",
		},
		{
			name:       "admin cannot remove admin",
			actorRole:  model.RoleAdmin,
			targetRole: model.RoleAdmin,
			wantErr:    "only owners can remove admins",
		},
		{
			name:       "owner cannot remove owner",
			actorRole:  model.RoleOwner,
			targetRole: model.RoleOwner,
			wantErr:    "cannot remove an owner",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()

			const (
				workspaceID    = "ws-guard"
				ownerUserID    = "user-owner"
				ownerMemberID  = "member-owner"
				actorUserID    = "user-actor"
				actorMemberID  = "member-actor"
				targetUserID   = "user-target"
				targetMemberID = "member-target"
			)

			seedUser(t, db, ownerUserID, "owner@example.com", "Owner User", "hash")
			seedWorkspace(t, db, workspaceID, "Workspace", "workspace", ownerUserID)
			seedWorkspaceMember(t, db, ownerMemberID, workspaceID, ownerUserID, "owner@example.com", "Owner User", model.RoleOwner)

			if tc.actorSameAsTarget {
				svc := NewWorkspaceService(repository.NewWorkspaceRepository(db), nil, nil)
				err := svc.RemoveMember(ctx, workspaceID, ownerUserID, ownerMemberID)
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("RemoveMember error = %v, want substring %q", err, tc.wantErr)
				}
				return
			}

			seedUser(t, db, actorUserID, "actor@example.com", "Actor User", "hash")
			seedUser(t, db, targetUserID, "target@example.com", "Target User", "hash")
			seedWorkspaceMember(t, db, actorMemberID, workspaceID, actorUserID, "actor@example.com", "Actor User", tc.actorRole)
			seedWorkspaceMember(t, db, targetMemberID, workspaceID, targetUserID, "target@example.com", "Target User", tc.targetRole)

			svc := NewWorkspaceService(repository.NewWorkspaceRepository(db), nil, nil)
			err := svc.RemoveMember(ctx, workspaceID, actorUserID, targetMemberID)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("RemoveMember error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}
