package model

import "time"

// CRMSignalRoutingSettings stores mutable workspace defaults separate from autonomy.
type CRMSignalRoutingSettings struct {
	ID                         string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID                string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex"`
	DefaultSignalOwnerMemberID *string   `json:"default_signal_owner_member_id,omitempty" gorm:"type:uuid;index"`
	MinimumLanePriority        float64   `json:"minimum_lane_priority" gorm:"not null;default:5"`
	CreatedAt                  time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                  time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMSignalRoutingSettings) TableName() string { return "crm_signal_routing_settings" }

// UpdateCRMSignalRoutingSettingsRequest updates mutable workspace routing defaults.
type UpdateCRMSignalRoutingSettingsRequest struct {
	DefaultSignalOwnerMemberID *string  `json:"default_signal_owner_member_id"`
	ClearDefaultSignalOwner    bool     `json:"clear_default_signal_owner"`
	MinimumLanePriority        *float64 `json:"minimum_lane_priority"`
}

const (
	CRMSignalRolloutShadow = "shadow"
	CRMSignalRolloutLive   = "live"
)

// CRMSignalRolloutSettings gates the motion spine and dependent customer rules.
type CRMSignalRolloutSettings struct {
	ID                  string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string     `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex"`
	Mode                string     `json:"mode" gorm:"not null;default:'shadow';index"`
	ActivatedAt         *time.Time `json:"activated_at,omitempty"`
	ActivatedByMemberID *string    `json:"activated_by_member_id,omitempty" gorm:"type:uuid"`
	CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMSignalRolloutSettings) TableName() string { return "crm_signal_rollout_settings" }
