package authorization

import "context"

// SupportAgentScope is a resolved agent identity, never a synthetic human role.
// An empty team list denotes an agent configured for the whole workspace.
// Only the authenticated command host may derive this from persisted agent data.
type SupportAgentScope struct {
	WorkspaceID string
	AgentID     string
	TeamIDs     []string
}
type supportAgentScopeKey struct{}

func WithSupportAgentScope(ctx context.Context, scope SupportAgentScope) context.Context {
	scope.TeamIDs = append([]string(nil), scope.TeamIDs...)
	return context.WithValue(ctx, supportAgentScopeKey{}, scope)
}
func GetSupportAgentScope(ctx context.Context) *SupportAgentScope {
	scope, ok := ctx.Value(supportAgentScopeKey{}).(SupportAgentScope)
	if !ok || scope.WorkspaceID == "" || scope.AgentID == "" {
		return nil
	}
	return &scope
}
