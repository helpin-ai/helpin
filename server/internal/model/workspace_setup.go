package model

// WorkspaceSetupGuide is a read-only snapshot of essentials and selected goals.
// Paths are relative to /w/{slug}; MCP expands them to the configured app URL.
type WorkspaceSetupGuide struct {
	WorkspaceID  string                  `json:"workspace_id"`
	Goals        []string                `json:"goals"`
	Sections     []WorkspaceSetupSection `json:"sections"`
	Instructions string                  `json:"instructions"`
}

type WorkspaceSetupSection struct {
	Key   string             `json:"key"`
	Title string             `json:"title"`
	Steps []SupportSetupStep `json:"steps"`
}
