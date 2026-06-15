package model

import "encoding/json"

const AgentRunArtifactTypeToolCall = "tool_call"

type AgentRunToolCatalogEntry struct {
	Name                 string      `json:"name"`
	Description          string      `json:"description"`
	Category             string      `json:"category"`
	InputSchema          interface{} `json:"input_schema"`
	Mutating             bool        `json:"mutating"`
	ApprovalMode         string      `json:"approval_mode"`
	SupportedTargetTypes []string    `json:"supported_target_types,omitempty"`
}

type AgentRunToolListResponse struct {
	RunID string                     `json:"run_id"`
	Tools []AgentRunToolCatalogEntry `json:"tools"`
}

type AgentRunToolCallRequest struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
}

type AgentRunToolCallResponse struct {
	ToolName         string           `json:"tool_name"`
	Content          []MCPContentItem `json:"content"`
	IsError          bool             `json:"is_error,omitempty"`
	ApprovalRequired bool             `json:"approval_required,omitempty"`
	InteractionID    string           `json:"interaction_id,omitempty"`
}

type MCPContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
