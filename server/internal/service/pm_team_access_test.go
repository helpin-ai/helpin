package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func actorContext(role string, teams []authorization.TeamRole) context.Context {
	return authorization.WithActor(context.Background(), &authorization.Actor{
		Role:            role,
		TeamMemberships: teams,
	})
}

func expectForbidden(t *testing.T, err error) {
	t.Helper()
	var forbidden *model.ErrForbidden
	if !errors.As(err, &forbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestRequireCanEditTeamEpics_MemberOfTeam(t *testing.T) {
	teamID := "team-a"
	ctx := actorContext(model.RoleMember, []authorization.TeamRole{{TeamID: teamID, Role: "member"}})

	if err := requireCanEditTeamEpics(ctx, &teamID); err != nil {
		t.Fatalf("requireCanEditTeamEpics() error = %v, want nil", err)
	}
}

func TestRequireCanEditTeamEpics_NotMemberOfTeam(t *testing.T) {
	teamID := "team-b"
	ctx := actorContext(model.RoleMember, []authorization.TeamRole{{TeamID: "team-a", Role: "member"}})

	expectForbidden(t, requireCanEditTeamEpics(ctx, &teamID))
}

func TestRequireCanEditTeamEpics_AdminCanEditAnyTeam(t *testing.T) {
	teamID := "team-b"
	ctx := actorContext(model.RoleAdmin, nil)

	if err := requireCanEditTeamEpics(ctx, &teamID); err != nil {
		t.Fatalf("requireCanEditTeamEpics() error = %v, want nil", err)
	}
}

func TestRequireCanEditTeamEpics_NullTeamID(t *testing.T) {
	memberCtx := actorContext(model.RoleMember, []authorization.TeamRole{{TeamID: "team-a", Role: "member"}})
	expectForbidden(t, requireCanEditTeamEpics(memberCtx, nil))

	adminCtx := actorContext(model.RoleAdmin, nil)
	if err := requireCanEditTeamEpics(adminCtx, nil); err != nil {
		t.Fatalf("requireCanEditTeamEpics(admin) error = %v, want nil", err)
	}
}

func TestRequireCanManage_Unchanged(t *testing.T) {
	teamID := "team-a"
	memberCtx := actorContext(model.RoleMember, []authorization.TeamRole{{TeamID: teamID, Role: "member"}})
	expectForbidden(t, requireCanManage(memberCtx, &teamID))

	managerCtx := actorContext(model.RoleMember, []authorization.TeamRole{{TeamID: teamID, Role: "owner"}})
	if err := requireCanManage(managerCtx, &teamID); err != nil {
		t.Fatalf("requireCanManage(team owner) error = %v, want nil", err)
	}
}
