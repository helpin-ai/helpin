package model

const (
	AgentRunArtifactTypeAgentRepairState = "agent_repair_state"
)

type AgentRepairState struct {
	Source       string `json:"source"`
	RepairClass  string `json:"repair_class"`
	ToolName     string `json:"tool_name,omitempty"`
	RepairHint   string `json:"repair_hint,omitempty"`
	ErrorSummary string `json:"error_summary,omitempty"`
}
