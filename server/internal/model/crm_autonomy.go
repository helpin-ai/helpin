package model

import "time"

// CRMAutonomySettings defines the automation behavior for the CRM module.
type CRMAutonomySettings struct {
	ID                   string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID          string    `json:"workspace_id" gorm:"type:uuid;uniqueIndex;not null"`
	Enabled              bool      `json:"enabled" gorm:"not null;default:true"`
	AutoCreateDeals      bool      `json:"auto_create_deals" gorm:"not null;default:true"`
	AutoProgressDeals    bool      `json:"auto_progress_deals" gorm:"not null;default:true"`
	AutoExecuteThreshold float64   `json:"auto_execute_threshold" gorm:"not null;default:0.9"`
	ReviewThreshold      float64   `json:"review_threshold" gorm:"not null;default:0.7"`
	CreatedAt            time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMAutonomySettings) TableName() string { return "crm_autonomy_settings" }

// DefaultAutonomySettings returns the default settings.
func DefaultAutonomySettings() CRMAutonomySettings {
	return CRMAutonomySettings{
		Enabled:              true,
		AutoCreateDeals:      true,
		AutoProgressDeals:    true,
		AutoExecuteThreshold: 0.9,
		ReviewThreshold:      0.7,
	}
}

// UpdateCRMAutonomySettingsRequest is the payload for updating autonomy settings.
type UpdateCRMAutonomySettingsRequest struct {
	Enabled              *bool    `json:"enabled"`
	AutoCreateDeals      *bool    `json:"auto_create_deals"`
	AutoProgressDeals    *bool    `json:"auto_progress_deals"`
	AutoExecuteThreshold *float64 `json:"auto_execute_threshold"`
	ReviewThreshold      *float64 `json:"review_threshold"`
}
