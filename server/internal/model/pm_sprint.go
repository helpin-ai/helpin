package model

import "time"

const (
	PMSprintStatusUnstarted = "unstarted"
	PMSprintStatusStarted   = "started"
	PMSprintStatusDone      = "done"
)

// PMSprint represents a sprint planning period.
type PMSprint struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"not null"`
	Description *string   `json:"description"`
	StartDate   time.Time `json:"start_date" gorm:"type:date;not null"`
	EndDate     time.Time `json:"end_date" gorm:"type:date;not null"`
	Status      string    `json:"status" gorm:"->"`
	TeamID      *string   `json:"team_id" gorm:"type:uuid;index"`
	Archived    bool      `json:"archived" gorm:"not null;default:false"`
	CreatedBy   *string   `json:"created_by" gorm:"type:uuid"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMSprint) TableName() string { return "pm_sprints" }

// PMSprintLabel is the many-to-many join between sprints and labels.
type PMSprintLabel struct {
	SprintID  string    `json:"sprint_id" gorm:"type:uuid;primaryKey"`
	LabelID   string    `json:"label_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMSprintLabel) TableName() string { return "pm_sprint_labels" }

// PMSprintListFilters applies filters when listing sprints.
type PMSprintListFilters struct {
	TeamID   *string
	Status   *string
	Archived *bool
}

// CreateSprintRequest is the payload for creating a sprint.
type CreateSprintRequest struct {
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	TeamID      *string   `json:"team_id"`
	LabelIDs    []string  `json:"label_ids"`
}

// UpdateSprintRequest is the payload for updating a sprint.
type UpdateSprintRequest struct {
	Name        *string    `json:"name"`
	Description *string    `json:"description"`
	StartDate   *time.Time `json:"start_date"`
	EndDate     *time.Time `json:"end_date"`
	TeamID      *string    `json:"team_id"`
	Archived    *bool      `json:"archived"`
	LabelIDs    []string   `json:"label_ids"`
}

// PMSprintStats contains derived sprint progress metrics.
type PMSprintStats struct {
	StoryCount     int `json:"story_count"`
	DoneStoryCount int `json:"done_story_count"`
	TotalPoints    int `json:"total_points"`
	DonePoints     int `json:"done_points"`
}

// SprintWithStats is a sprint with computed progress metrics.
type SprintWithStats struct {
	Sprint PMSprint      `json:"sprint"`
	Labels []PMLabel     `json:"labels"`
	Stats  PMSprintStats `json:"stats"`
}
