package model

import (
	"encoding/json"
	"time"
)

const (
	DockActionProposalKindExecution  = "execution"
	DockActionProposalKindAgentDraft = "agent_draft"

	DockActionProposalStatusPrepared  = "prepared"
	DockActionProposalStatusActive    = "active"
	DockActionProposalStatusCompleted = "completed"
	DockActionProposalStatusExpired   = "expired"
)

// DockActionProposal stores an immutable, user-reviewable execution scope for
// one dock chat. Mutating commands consume the bounded operations in Spec.
type DockActionProposal struct {
	ID                    string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID           string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DockChatRunID         string          `json:"dock_chat_run_id" gorm:"type:uuid;not null;index"`
	ActorID               string          `json:"actor_id" gorm:"type:uuid;not null;index"`
	Kind                  string          `json:"kind" gorm:"not null;index"`
	Status                string          `json:"status" gorm:"not null;index"`
	Summary               string          `json:"summary" gorm:"not null"`
	Spec                  json.RawMessage `json:"spec" gorm:"type:jsonb;not null;default:'{}'"`
	Usage                 json.RawMessage `json:"usage" gorm:"type:jsonb;not null;default:'{}'"`
	ApprovalInteractionID *string         `json:"approval_interaction_id,omitempty" gorm:"type:uuid;index"`
	ExpiresAt             time.Time       `json:"expires_at" gorm:"not null;index"`
	ActivatedAt           *time.Time      `json:"activated_at,omitempty"`
	CompletedAt           *time.Time      `json:"completed_at,omitempty"`
	CreatedAt             time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the database table used for dock action proposals.
func (DockActionProposal) TableName() string { return "dock_action_proposals" }
