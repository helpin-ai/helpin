package authorization

import "context"

type actorContextKey struct{}

// Actor represents the authenticated user's identity and role within a workspace.
type Actor struct {
	UserID            string
	WorkspaceID       string
	WorkspaceMemberID string
	Role              string
	Status            string
	TeamMemberships   []TeamRole
}

// TeamRole captures a user's role within a specific team.
type TeamRole struct {
	TeamID string
	Role   string
}

// IsTeamOwner returns true if the actor is an owner of the given team.
func (a *Actor) IsTeamOwner(teamID string) bool {
	for _, tm := range a.TeamMemberships {
		if tm.TeamID == teamID && tm.Role == "owner" {
			return true
		}
	}
	return false
}

// TeamIDs returns the IDs of all teams the actor belongs to.
func (a *Actor) TeamIDs() []string {
	ids := make([]string, len(a.TeamMemberships))
	for i, tm := range a.TeamMemberships {
		ids[i] = tm.TeamID
	}
	return ids
}

// IsMemberOfTeam returns true if the actor belongs to the given team (any role).
func (a *Actor) IsMemberOfTeam(teamID string) bool {
	for _, tm := range a.TeamMemberships {
		if tm.TeamID == teamID {
			return true
		}
	}
	return false
}

// WithActor stores the Actor in the request context.
func WithActor(ctx context.Context, actor *Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// GetActor retrieves the Actor from the request context.
// Returns nil if no actor is present.
func GetActor(ctx context.Context) *Actor {
	v, _ := ctx.Value(actorContextKey{}).(*Actor)
	return v
}
