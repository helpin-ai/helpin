package model

import "time"

// PMLabel represents labels that can be attached to stories/epics/sprints.
type PMLabel struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TeamID      *string   `json:"team_id" gorm:"type:uuid;index"`
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
	TeamID      *string `json:"team_id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
}

// UpdateLabelRequest is the payload for updating a label.
type UpdateLabelRequest struct {
	TeamID      *string `json:"team_id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	Archived    *bool   `json:"archived"`
}

// LabelStats holds completion metrics for tasks and epics associated with a label.
type LabelStats struct {
	TaskCount     int `json:"task_count"`
	DoneTaskCount int `json:"done_task_count"`
	TotalPoints   int `json:"total_points"`
	DonePoints    int `json:"done_points"`
	EpicCount     int `json:"epic_count"`
	DoneEpicCount int `json:"done_epic_count"`
}

// LabelWithStats pairs a label with its computed stats.
type LabelWithStats struct {
	Label PMLabel    `json:"label"`
	Stats LabelStats `json:"stats"`
}
