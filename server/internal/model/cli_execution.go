package model

import (
	"encoding/json"
	"time"
)

// CLIGeneration records a single provider attempt before dispatch for replay safety.
type CLIGeneration struct {
	ID                    string   `gorm:"primaryKey"`
	ExecutionID           string   `gorm:"not null;index"`
	RequestHash           string   `gorm:"not null"`
	Response              JSONBlob `gorm:"type:jsonb"`
	InputTokens           int64
	OutputTokens          int64
	CachedInputTokens     int64
	ReasoningOutputTokens int64
	CreatedAt             time.Time
}

// TableName returns the provider attempt journal.
func (CLIGeneration) TableName() string { return "cli_generations" }

// CLINativeMessage is the versioned, credential-free local model message.
type CLINativeMessage struct {
	Role             string           `json:"role"`
	Content          string           `json:"content,omitempty"`
	Blocks           []CLINativeBlock `json:"blocks,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ProviderState    json.RawMessage  `json:"provider_state,omitempty"`
	ContextSummary   bool             `json:"context_summary,omitempty"`
	Provenance       string           `json:"provenance,omitempty"`
}

// CLINativeBlock carries native tool calls and results.
type CLINativeBlock struct {
	Type       string          `json:"type"`
	Text       string          `json:"text,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Output     string          `json:"output,omitempty"`
	IsError    bool            `json:"is_error,omitempty"`
}

// CLIToolDefinition is intersected with the admitted local tool policy.
type CLIToolDefinition struct {
	Category             string         `json:"category,omitempty"`
	RiskLevel            string         `json:"risk_level,omitempty"`
	SupportedTargetTypes []string       `json:"supported_target_types,omitempty"`
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	InputSchema          map[string]any `json:"input_schema"`
	Mutating             bool           `json:"mutating"`
}

// CLINativeRequest is the inference request carried by the generic CLI protocol.
type CLINativeRequest struct {
	SystemPrompt string              `json:"system_prompt,omitempty"`
	Messages     []CLINativeMessage  `json:"messages"`
	Tools        []CLIToolDefinition `json:"tools,omitempty"`
	Step         int                 `json:"step"`
}

// CLINativeResponse contains only the provider response and server-observed usage.
type CLINativeResponse struct {
	Incomplete bool             `json:"incomplete,omitempty"`
	Message    CLINativeMessage `json:"message"`
	Usage      CLIUsage         `json:"usage"`
}

// CLIUsage is provider-observed telemetry, never accepted from local reports.
type CLIUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
}

// CLIModelRequest binds a stable inference request to one leased execution.
type CLIModelRequest struct {
	RequestID  string           `json:"request_id"`
	Epoch      int64            `json:"epoch"`
	LocalRunID string           `json:"local_run_id"`
	Request    CLINativeRequest `json:"request"`
}

// CLIResultRequest contains untrusted local reports, not billing or cloud events.
type CLIResultRequest struct {
	Epoch         int64                `json:"epoch"`
	LocalRunID    string               `json:"local_run_id"`
	Status        string               `json:"status"`
	Events        []json.RawMessage    `json:"events"`
	Messages      []CLIReportedMessage `json:"messages"`
	OutputSummary json.RawMessage      `json:"output_summary"`
}

// CLIReportedMessage is projected as locally reported conversation content.
type CLIReportedMessage struct {
	ID              string          `json:"id"`
	Role            string          `json:"role"`
	Content         string          `json:"content"`
	ContentBlocks   json.RawMessage `json:"content_blocks,omitempty"`
	ToolInvocations json.RawMessage `json:"tool_invocations,omitempty"`
}

// CLIArtifactRequest describes a bounded local text artifact.
type CLIArtifactRequest struct {
	Epoch      int64  `json:"epoch"`
	LocalRunID string `json:"local_run_id"`
	RequestID  string `json:"request_id"`
	Kind       string `json:"kind"`
	Content    string `json:"content"`
}
