package model

import "time"

// DockAgentIdentity is the small agent identity projection used by the dock.
type DockAgentIdentity struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IconKey   string `json:"icon_key,omitempty"`
	PresetKey string `json:"preset_key,omitempty"`
}

// DockRunSummary is one user-owned agent run in the dock roster.
type DockRunSummary struct {
	Run            AgentRun          `json:"run"`
	Agent          DockAgentIdentity `json:"agent"`
	AttentionKind  string            `json:"attention_kind,omitempty"`
	LastActivityAt time.Time         `json:"last_activity_at"`
}

// DockRunListResponse is the complete dock roster payload for one user.
type DockRunListResponse struct {
	Runs           []DockRunSummary `json:"runs"`
	AttentionCount int              `json:"attention_count"`
}
