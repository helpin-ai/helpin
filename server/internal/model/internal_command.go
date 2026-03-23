package model

type InternalCommandContext struct {
	WorkspaceID   string `json:"workspace_id"`
	ActorID       string `json:"actor_id,omitempty"`
	AgentID       string `json:"agent_id,omitempty"`
	RunID         string `json:"run_id,omitempty"`
	TargetType    string `json:"target_type,omitempty"`
	TargetID      string `json:"target_id,omitempty"`
}
