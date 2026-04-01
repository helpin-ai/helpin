package model

import (
	"encoding/json"
	"time"
)

const (
	AgentRunInteractionKindRequestUserInput         = "request_user_input"
	AgentRunInteractionKindCommandExecutionApproval = "command_execution_approval"
	AgentRunInteractionKindFileChangeApproval       = "file_change_approval"
	AgentRunInteractionKindPermissionsApproval      = "permissions_approval"
	AgentRunInteractionKindReviewCheckpoint         = "review_checkpoint"
	AgentRunInteractionKindAuthRequired             = "auth_required"
	AgentRunInteractionStatusPending                = "pending"
	AgentRunInteractionStatusResolved               = "resolved"
	AgentRunInteractionStatusCancelled              = "cancelled"
	AgentRunInteractionSchemaVersionCodexV2         = "codex.v2"
	AgentRunInteractionSchemaVersionHelpinV1        = "helpin.v1"
)

// AgentRunInteraction stores first-class interactive requests and responses for a run.
type AgentRunInteraction struct {
	ID                         string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID                string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	RunID                      string          `json:"run_id" gorm:"type:uuid;not null;index"`
	RuntimeKind                string          `json:"runtime_kind" gorm:"not null;index"`
	InteractionKind            string          `json:"interaction_kind" gorm:"not null;index"`
	Status                     string          `json:"status" gorm:"not null;default:'pending';index"`
	RequestSchemaVersion       string          `json:"request_schema_version" gorm:"not null"`
	ResponseSchemaVersion      *string         `json:"response_schema_version,omitempty"`
	RequestID                  *string         `json:"request_id,omitempty" gorm:"index"`
	ThreadID                   *string         `json:"thread_id,omitempty"`
	TurnID                     *string         `json:"turn_id,omitempty"`
	ItemID                     *string         `json:"item_id,omitempty"`
	ApprovalID                 *string         `json:"approval_id,omitempty"`
	AssistantMessageSequenceNo *int            `json:"assistant_message_sequence_no,omitempty"`
	Title                      *string         `json:"title,omitempty"`
	Summary                    *string         `json:"summary,omitempty"`
	RequestPayload             json.RawMessage `json:"request_payload" gorm:"type:jsonb;not null;default:'{}'"`
	ResponsePayload            json.RawMessage `json:"response_payload,omitempty" gorm:"type:jsonb"`
	RuntimeMetadata            json.RawMessage `json:"runtime_metadata" gorm:"type:jsonb;not null;default:'{}'"`
	ResolvedBy                 *string         `json:"resolved_by,omitempty" gorm:"type:uuid"`
	ResolvedAt                 *time.Time      `json:"resolved_at,omitempty"`
	CreatedAt                  time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                  time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AgentRunInteraction) TableName() string { return "agent_run_interactions" }

type ResolveAgentRunInteractionRequest struct {
	ResponsePayload json.RawMessage `json:"response_payload"`
	FollowupMessage *string         `json:"followup_message,omitempty"`
}
