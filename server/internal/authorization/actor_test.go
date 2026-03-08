package authorization

import (
	"context"
	"testing"
)

func TestActor_IsTeamOwner(t *testing.T) {
	actor := &Actor{
		TeamMemberships: []TeamRole{
			{TeamID: "team-1", Role: "owner"},
			{TeamID: "team-2", Role: "member"},
		},
	}

	if !actor.IsTeamOwner("team-1") {
		t.Error("should be team owner of team-1")
	}
	if actor.IsTeamOwner("team-2") {
		t.Error("should NOT be team owner of team-2")
	}
	if actor.IsTeamOwner("team-3") {
		t.Error("should NOT be team owner of team-3 (not a member)")
	}
}

func TestActor_IsTeamOwner_NoTeams(t *testing.T) {
	actor := &Actor{}
	if actor.IsTeamOwner("any-team") {
		t.Error("actor with no team memberships should not be team owner")
	}
}

func TestWithActor_GetActor(t *testing.T) {
	actor := &Actor{
		UserID:            "user-1",
		WorkspaceID:       "ws-1",
		WorkspaceMemberID: "wm-1",
		Role:              "admin",
		Status:            "active",
	}

	ctx := WithActor(context.Background(), actor)
	got := GetActor(ctx)
	if got == nil {
		t.Fatal("expected actor in context")
	}
	if got.UserID != "user-1" || got.WorkspaceMemberID != "wm-1" || got.Role != "admin" {
		t.Error("actor fields mismatch")
	}
}

func TestGetActor_EmptyContext(t *testing.T) {
	got := GetActor(context.Background())
	if got != nil {
		t.Error("expected nil actor from empty context")
	}
}
