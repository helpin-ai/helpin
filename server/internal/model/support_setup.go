package model

// SupportSetupGuide is a read-only snapshot, shared by the UI and external clients.
// Paths are relative to /w/{slug}; MCP expands them using the configured app URL.
type SupportSetupGuide struct {
	WorkspaceID  string             `json:"workspace_id"`
	Steps        []SupportSetupStep `json:"steps"`
	Instructions string             `json:"instructions"`
}

type SupportSetupStep struct {
	Key           string `json:"key"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Path          string `json:"path,omitempty"`
	BlockedReason string `json:"blocked_reason,omitempty"`
	Verification  string `json:"verification"`
}
