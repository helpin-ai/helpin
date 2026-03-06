package model

import "time"

// RewardQuarter represents a row in the reward_quarters table.
type RewardQuarter struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"not null"`
	StartDate   string    `json:"start_date" gorm:"not null"`
	EndDate     string    `json:"end_date" gorm:"not null"`
	Status      string    `json:"status" gorm:"not null;default:'planning'"`
	CreatedBy   *string   `json:"created_by" gorm:"type:uuid"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (RewardQuarter) TableName() string { return "reward_quarters" }

// CreateRewardQuarterRequest is the payload for POST /api/rewards/quarters.
type CreateRewardQuarterRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
}

// UpdateRewardQuarterStatusRequest is the payload for PATCH /api/rewards/quarters/{id}/status.
type UpdateRewardQuarterStatusRequest struct {
	Status string `json:"status"`
}
