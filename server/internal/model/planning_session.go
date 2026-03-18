package model

import (
	"encoding/json"
	"time"
)

const (
	PlanningSessionStatusActive     = "active"
	PlanningSessionStatusPaused     = "paused"
	PlanningSessionStatusFinalizing = "finalizing"
	PlanningSessionStatusCompleted  = "completed"
	PlanningSessionStatusAbandoned  = "abandoned"

	PlanningMessageTypeQuestion     = "question"
	PlanningMessageTypeAnswer       = "answer"
	PlanningMessageTypeProposal     = "proposal"
	PlanningMessageTypeConfirmation = "confirmation"
	PlanningMessageTypeContext      = "context"
	PlanningMessageTypeSummary      = "summary"
	PlanningMessageTypeFinalSpec    = "final_spec"
	PlanningMessageTypeToolUse      = "tool_use"
	PlanningMessageTypeMessage      = "message"

	EpicPlanningStateInSession = "in_session"
)

// PlanningSessionAllowedTools defines the read-only tools available during planning sessions.
var PlanningSessionAllowedTools = map[string]bool{
	"read_file":       true,
	"read_file_range": true,
	"list_directory":  true,
	"search_files":    true,
	"ripgrep":         true,
	"grep":            true,
	"list_symbols":    true,
	"web_search":      true,
}

// PlanningSession represents an interactive planning conversation between a human and a planning agent.
type PlanningSession struct {
	ID                  string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	EpicID              string          `json:"epic_id" gorm:"type:uuid;not null;index"`
	FlowRunID           *string         `json:"flow_run_id,omitempty" gorm:"type:uuid;index"`
	FlowNodeRunID       *string         `json:"flow_node_run_id,omitempty" gorm:"type:uuid;index"`
	AgentID             string          `json:"agent_id" gorm:"type:uuid;not null"`
	Status              string          `json:"status" gorm:"not null;default:'active'"`
	PlanningMethodology string          `json:"planning_methodology" gorm:"not null;default:'structured_v1'"`
	AllowedTools        json.RawMessage `json:"allowed_tools" gorm:"type:jsonb;not null;default:'[]'"`
	SpecDocumentID      *string         `json:"spec_document_id,omitempty" gorm:"type:uuid"`
	SpecDraft           string          `json:"spec_draft" gorm:"type:text;not null;default:''"`
	SpecSections        json.RawMessage `json:"spec_sections" gorm:"type:jsonb;not null;default:'[]'"` // deprecated: kept for migration compat
	ContextSnapshot     json.RawMessage `json:"context_snapshot" gorm:"type:jsonb;not null;default:'{}'"`
	TokenUsage          json.RawMessage `json:"token_usage" gorm:"type:jsonb;not null;default:'{\"input\":0,\"output\":0}'"`
	StartedBy           *string         `json:"started_by,omitempty" gorm:"type:uuid"`
	StartedAt           time.Time       `json:"started_at" gorm:"not null;default:now()"`
	LastActiveAt        time.Time       `json:"last_active_at" gorm:"not null;default:now()"`
	CompletedAt         *time.Time      `json:"completed_at,omitempty"`
	CreatedAt           time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PlanningSession) TableName() string { return "planning_sessions" }

// PlanningSessionMessage represents a single message in a planning session conversation.
type PlanningSessionMessage struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SessionID       string          `json:"session_id" gorm:"type:uuid;not null;index"`
	Role            string          `json:"role" gorm:"not null"`
	Content         string          `json:"content" gorm:"not null"`
	MessageType     string          `json:"message_type" gorm:"not null;default:'message'"`
	SectionMetadata json.RawMessage `json:"section_metadata,omitempty" gorm:"type:jsonb"`
	ToolInvocations json.RawMessage `json:"tool_invocations,omitempty" gorm:"type:jsonb"`
	ContentBlocks   json.RawMessage `json:"content_blocks,omitempty" gorm:"type:jsonb"`
	TokenUsage      json.RawMessage `json:"token_usage,omitempty" gorm:"type:jsonb"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (PlanningSessionMessage) TableName() string { return "planning_session_messages" }

// ToolInvocation records a single tool use within a planning session turn.
type ToolInvocation struct {
	ToolName      string          `json:"tool_name"`
	Input         json.RawMessage `json:"input"`
	OutputSummary string          `json:"output_summary"`
	DurationMs    int64           `json:"duration_ms"`
}

// PlanningMessageBlock stores provider-neutral assistant/tool content for a session turn.
type PlanningMessageBlock struct {
	Type       string          `json:"type"`
	Text       string          `json:"text,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Output     string          `json:"output,omitempty"`
	IsError    bool            `json:"is_error,omitempty"`
}

// SessionTokenUsage tracks cumulative token consumption for a session.
type SessionTokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// StartPlanningSessionRequest is the request to start a new planning session.
type StartPlanningSessionRequest struct {
	AgentID           string  `json:"agent_id"`
	AdditionalContext *string `json:"additional_context,omitempty"`
	FlowRunID         *string `json:"flow_run_id,omitempty"`
	FlowNodeRunID     *string `json:"flow_node_run_id,omitempty"`
	AllowedTools      json.RawMessage `json:"allowed_tools,omitempty"`
}

// SendPlanningMessageRequest is the request to send a message in a planning session.
type SendPlanningMessageRequest struct {
	Content     string  `json:"content"`
	MessageType *string `json:"message_type,omitempty"`
}

// FinalizePlanningSessionRequest is the request to finalize a planning session.
type FinalizePlanningSessionRequest struct{}

// PlanningStreamEvent represents a real-time streaming event sent via WebSocket.
type PlanningStreamEvent struct {
	EventID       string    `json:"event_id,omitempty"`
	SentAt        time.Time `json:"sent_at,omitempty"`
	Type          string    `json:"type"`
	SessionID     string    `json:"session_id"`
	MessageID     string    `json:"message_id,omitempty"`
	Text          string    `json:"text,omitempty"`
	ToolCallID    string    `json:"tool_call_id,omitempty"`
	ToolName      string    `json:"tool_name,omitempty"`
	ToolInput     string    `json:"tool_input,omitempty"`
	OutputSummary string    `json:"output_summary,omitempty"`
	DurationMs    int64     `json:"duration_ms,omitempty"`
	Error         string    `json:"error,omitempty"`
}
