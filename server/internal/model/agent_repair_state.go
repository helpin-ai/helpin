package model

const (
	AgentRunArtifactTypeNativeRepairState = "native_repair_state"
)

type NativeRepairState struct {
	Source       string `json:"source"`
	RepairClass  string `json:"repair_class"`
	ToolName     string `json:"tool_name,omitempty"`
	RepairHint   string `json:"repair_hint,omitempty"`
	ErrorSummary string `json:"error_summary,omitempty"`
}
