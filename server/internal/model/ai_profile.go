package model

import (
	"time"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
)

// AIProfileRoute binds model controls to a connection, never to a browser-supplied secret.
type AIProfileRoute struct {
	ConnectionID string       `json:"connection_id"`
	Model        sdk.RunModel `json:"model"`
}

// AIProfile is a workspace-scoped personal or shared model selection.
type AIProfile struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	UserID      *string         `json:"user_id" gorm:"type:uuid;index"`
	Scope       string          `json:"scope" gorm:"not null"`
	Name        string          `json:"name" gorm:"not null"`
	Revision    int64           `json:"revision" gorm:"not null"`
	Primary     AIProfileRoute  `json:"primary" gorm:"serializer:json;type:jsonb;not null"`
	Fallback    *AIProfileRoute `json:"fallback" gorm:"serializer:json;type:jsonb"`
	DeletedAt   *time.Time      `json:"-"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (AIProfile) TableName() string { return "ai_profiles" }

// SaveAIProfileRequest replaces a profile using an expected revision on update.
// Ownership is immutable; changing scope requires a new profile.
type SaveAIProfileRequest struct {
	Name     string          `json:"name"`
	Scope    string          `json:"scope"`
	Revision int64           `json:"revision,omitempty"`
	Primary  AIProfileRoute  `json:"primary"`
	Fallback *AIProfileRoute `json:"fallback,omitempty"`
}

// AIWorkspaceSettings holds the shared default independently of commercial policy.
type AIWorkspaceSettings struct {
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;primaryKey"`
	DefaultProfileID *string   `json:"default_profile_id" gorm:"type:uuid"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (AIWorkspaceSettings) TableName() string { return "ai_workspace_settings" }

// AIExecutionSelection freezes the route accepted at admission. It contains no secrets.
type AIExecutionSelection struct {
	ProfileID       string                     `json:"profile_id,omitempty"`
	ProfileRevision int64                      `json:"profile_revision,omitempty"`
	ConnectionScope string                     `json:"connection_scope"`
	OwnerID         *string                    `json:"owner_id,omitempty"`
	Route           AIProfileRoute             `json:"route"`
	Source          string                     `json:"source"`
	FallbackReason  string                     `json:"fallback_reason,omitempty"`
	Policy          *AIExecutionPolicySnapshot `json:"policy,omitempty"`
}

// AIExecutionPolicySnapshot is supplied by trusted edition policy at admission.
// It travels with the accepted route and is never supplied by a launch DTO.
type AIExecutionPolicySnapshot struct {
	Mode        string                   `json:"mode"`
	FundingMode aiusage.FundingMode      `json:"funding_mode"`
	FlatTariff  *aiusage.FlatTokenTariff `json:"flat_tariff,omitempty"`
}
