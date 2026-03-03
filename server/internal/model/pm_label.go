package model

import "time"

// PMLabel represents labels that can be attached to stories/epics/iterations.
type PMLabel struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"not null"`
	Description *string   `json:"description"`
	Color       *string   `json:"color"`
	Archived    bool      `json:"archived" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMLabel) TableName() string { return "pm_labels" }

// CreateLabelRequest is the payload for creating a label.
type CreateLabelRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
}

// UpdateLabelRequest is the payload for updating a label.
type UpdateLabelRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	Archived    *bool   `json:"archived"`
}
