package model

import "time"

// SupportTeammateStatusOverride stores a manual status override for a workspace teammate.
type SupportTeammateStatusOverride struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_teammate_status_override_workspace_user,unique"`
	UserID       string    `json:"user_id" gorm:"type:uuid;not null;index:idx_support_teammate_status_override_workspace_user,unique"`
	ManualStatus string    `json:"manual_status" gorm:"not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportTeammateStatusOverride) TableName() string { return "support_teammate_status_overrides" }
