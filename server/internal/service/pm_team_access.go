package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// isPrivileged returns true if the actor has admin or owner role (full access, no team filtering).
func isPrivileged(actor *authorization.Actor) bool {
	return actor.Role == model.RoleAdmin || actor.Role == model.RoleOwner
}

// accessibleTeamIDs returns the team IDs the current actor can access.
// Returns nil for admin/owner (no filtering needed).
// Returns the actor's team IDs for members/viewers.
func accessibleTeamIDs(ctx context.Context) []string {
	actor := authorization.GetActor(ctx)
	if actor == nil {
		return nil
	}
	if isPrivileged(actor) {
		return nil
	}
	return actor.TeamIDs()
}

// canAccessTeam checks whether the current actor can access an entity with the given team ID.
// Admins/owners can access everything. Members/viewers can only access their own teams.
// NULL team_id entities are admin/owner-only.
func canAccessTeam(ctx context.Context, teamID *string) bool {
	actor := authorization.GetActor(ctx)
	if actor == nil {
		return true
	}
	if isPrivileged(actor) {
		return true
	}
	if teamID == nil || *teamID == "" {
		return false // NULL team = admin-only
	}
	return actor.IsMemberOfTeam(*teamID)
}

// canAccessTeams checks whether the current actor can access an entity linked to the given teams.
// Used for objectives which have many-to-many team relationships.
// Returns true if the actor is in at least one of the given teams.
func canAccessTeams(ctx context.Context, teamIDs []string) bool {
	actor := authorization.GetActor(ctx)
	if actor == nil {
		return true
	}
	if isPrivileged(actor) {
		return true
	}
	if len(teamIDs) == 0 {
		return false // No teams = admin-only
	}
	for _, tid := range teamIDs {
		if actor.IsMemberOfTeam(tid) {
			return true
		}
	}
	return false
}

// requireTeamAccess returns an ErrForbidden if the actor cannot access the given team.
func requireTeamAccess(ctx context.Context, teamID *string) error {
	if !canAccessTeam(ctx, teamID) {
		return &model.ErrForbidden{Message: "you do not have access to this team's resources"}
	}
	return nil
}

// requireTeamMembershipForCreate validates that a non-admin actor can create in the given team.
func requireTeamMembershipForCreate(ctx context.Context, teamID *string) error {
	actor := authorization.GetActor(ctx)
	if actor == nil {
		return nil
	}
	if isPrivileged(actor) {
		return nil
	}
	if teamID == nil || *teamID == "" {
		return &model.ErrForbidden{Message: "team_id is required"}
	}
	if !actor.IsMemberOfTeam(*teamID) {
		return &model.ErrForbidden{Message: "you can only create items in your own teams"}
	}
	return nil
}

// requireCanManageTeams checks that the actor is admin/owner OR a team manager for at least one of the target teams.
// Used for planning entities (epics, sprints, objectives) that regular members cannot create/edit.
func requireCanManageTeams(ctx context.Context, teamIDs []string) error {
	actor := authorization.GetActor(ctx)
	if actor == nil {
		return nil
	}
	if isPrivileged(actor) {
		return nil
	}
	if actor.Role == model.RoleViewer {
		return &model.ErrForbidden{Message: "management access required"}
	}
	if len(teamIDs) == 0 {
		return &model.ErrForbidden{Message: "at least one team is required"}
	}
	for _, tid := range teamIDs {
		if actor.IsTeamOwner(tid) {
			return nil
		}
	}
	return &model.ErrForbidden{Message: "only team managers can manage epics, sprints, and objectives"}
}

// requireCanManage checks that the actor is admin/owner OR a team manager (team owner) for the target team.
// Convenience wrapper around requireCanManageTeams for single-team entities.
func requireCanManage(ctx context.Context, teamID *string) error {
	if teamID == nil || *teamID == "" {
		// NULL team — only privileged roles can manage
		actor := authorization.GetActor(ctx)
		if actor == nil {
			return nil
		}
		if isPrivileged(actor) {
			return nil
		}
		return &model.ErrForbidden{Message: "team_id is required"}
	}
	return requireCanManageTeams(ctx, []string{*teamID})
}
