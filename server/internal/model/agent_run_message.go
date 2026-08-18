package model

import (
	"encoding/json"
	"time"
)

// AgentRunMessage persists the conversation history for an interactive or autonomous run.
type AgentRunMessage struct {
	ID               string               `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string               `json:"workspace_id" gorm:"type:uuid;not null;index;index:idx_agent_run_messages_runtime_message_id,priority:1"`
	RunID            string               `json:"run_id" gorm:"type:uuid;not null;index;index:idx_agent_run_messages_runtime_message_id,priority:2"`
	DockChatID       *string              `json:"dock_chat_id,omitempty" gorm:"type:uuid"`
	DockChatSequence *int64               `json:"dock_chat_sequence,omitempty"`
	ClientMessageID  *string              `json:"client_message_id,omitempty" gorm:"type:uuid"`
	DeliveryStatus   string               `json:"delivery_status" gorm:"type:text;not null;default:'sent'"`
	ActorUserID      *string              `json:"actor_user_id,omitempty" gorm:"type:uuid;index"`
	RuntimeMessageID string               `json:"runtime_message_id,omitempty" gorm:"index:idx_agent_run_messages_runtime_message_id,priority:3"`
	Role             string               `json:"role" gorm:"not null"`
	Content          string               `json:"content" gorm:"not null"`
	MessageType      string               `json:"message_type" gorm:"not null;default:'message'"`
	ContentBlocks    json.RawMessage      `json:"content_blocks,omitempty" gorm:"type:jsonb"`
	TurnSegments     json.RawMessage      `json:"turn_segments,omitempty" gorm:"type:jsonb"`
	ToolInvocations  json.RawMessage      `json:"tool_invocations,omitempty" gorm:"type:jsonb"`
	TokenUsage       json.RawMessage      `json:"token_usage,omitempty" gorm:"type:jsonb"`
	SequenceNo       int                  `json:"sequence_no" gorm:"not null;default:0"`
	CreatedAt        time.Time            `json:"created_at" gorm:"autoCreateTime"`
	DockWorkSummary  *DockChatWorkSummary `json:"dock_work_summary,omitempty" gorm:"-"`
}

// DockChatWorkSummary is the lightweight completed-turn disclosure returned
// in compact Dock chat history. Full work details are fetched on demand.
type DockChatWorkSummary struct {
	MessageID     string `json:"message_id"`
	DurationMs    int64  `json:"duration_ms"`
	ActivityCount int    `json:"activity_count"`
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
