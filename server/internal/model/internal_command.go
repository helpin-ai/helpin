package model

type InternalCommandContext struct {
	WorkspaceID  string   `json:"workspace_id"`
	ActorID      string   `json:"actor_id,omitempty"`
	AuditActorID string   `json:"audit_actor_id,omitempty"`
	ActorRole    string   `json:"actor_role,omitempty"`
	ActorTeamIDs []string `json:"actor_team_ids,omitempty"`
	AgentID      string   `json:"agent_id,omitempty"`
	AgentTeamIDs []string `json:"agent_team_ids,omitempty"`
	// AgentScopeResolved distinguishes a workspace-scoped agent (true with an
	// empty team list) from command contexts whose agent scope is unknown.
	AgentScopeResolved bool   `json:"agent_scope_resolved,omitempty"`
	RunID              string `json:"run_id,omitempty"`
	TargetType         string `json:"target_type,omitempty"`
	TargetID           string `json:"target_id,omitempty"`
}
