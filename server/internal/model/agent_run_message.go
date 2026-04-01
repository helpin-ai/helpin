package model

import (
	"encoding/json"
	"time"
)

// AgentRunMessage persists the conversation history for an interactive or autonomous run.
type AgentRunMessage struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	RunID           string          `json:"run_id" gorm:"type:uuid;not null;index"`
	Role            string          `json:"role" gorm:"not null"`
	Content         string          `json:"content" gorm:"not null"`
	MessageType     string          `json:"message_type" gorm:"not null;default:'message'"`
	ContentBlocks   json.RawMessage `json:"content_blocks,omitempty" gorm:"type:jsonb"`
	TurnSegments    json.RawMessage `json:"turn_segments,omitempty" gorm:"type:jsonb"`
	ToolInvocations json.RawMessage `json:"tool_invocations,omitempty" gorm:"type:jsonb"`
	TokenUsage      json.RawMessage `json:"token_usage,omitempty" gorm:"type:jsonb"`
	SequenceNo      int             `json:"sequence_no" gorm:"not null;default:0"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentRunMessage) TableName() string { return "agent_run_messages" }

// AgentRunStreamEvent is the live streaming payload for run token/tool updates.
type AgentRunStreamEvent struct {
	EventID         string    `json:"event_id,omitempty"`
	SentAt          time.Time `json:"sent_at,omitempty"`
	Type            string    `json:"type"`
	RunID           string    `json:"run_id"`
	MessageID       string    `json:"message_id,omitempty"`
	ParentMessageID string    `json:"parent_message_id,omitempty"`
	ResultMessageID string    `json:"result_message_id,omitempty"`
	Text            string    `json:"text,omitempty"`
	Content         string    `json:"content,omitempty"`
	ToolCallID      string    `json:"tool_call_id,omitempty"`
	ToolName        string    `json:"tool_name,omitempty"`
	ToolInput       string    `json:"tool_input,omitempty"`
	ArgsDelta       string    `json:"args_delta,omitempty"`
	ArgsText        string    `json:"args_text,omitempty"`
	ActivityID      string    `json:"activity_id,omitempty"`
	ActivityType    string    `json:"activity_type,omitempty"`
	EncryptedValue  string    `json:"encrypted_value,omitempty"`
	OutputSummary   string    `json:"output_summary,omitempty"`
	DurationMs      int64     `json:"duration_ms,omitempty"`
	Error           string    `json:"error,omitempty"`
}
