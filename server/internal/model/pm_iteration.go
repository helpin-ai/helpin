package model

import "time"

const (
	PMIterationStatusUnstarted = "unstarted"
	PMIterationStatusStarted   = "started"
	PMIterationStatusDone      = "done"
)

// PMIteration represents an iteration/sprint-like planning period.
type PMIteration struct {
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

func (PMIteration) TableName() string { return "pm_iterations" }

// PMIterationLabel is the many-to-many join between iterations and labels.
type PMIterationLabel struct {
	IterationID string    `json:"iteration_id" gorm:"type:uuid;primaryKey"`
	LabelID     string    `json:"label_id" gorm:"type:uuid;primaryKey"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMIterationLabel) TableName() string { return "pm_iteration_labels" }

// PMIterationListFilters applies filters when listing iterations.
type PMIterationListFilters struct {
	TeamID   *string
	Status   *string
	Archived *bool
}

// CreateIterationRequest is the payload for creating an iteration.
type CreateIterationRequest struct {
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	TeamID      *string   `json:"team_id"`
	LabelIDs    []string  `json:"label_ids"`
}

// UpdateIterationRequest is the payload for updating an iteration.
type UpdateIterationRequest struct {
	Name        *string    `json:"name"`
	Description *string    `json:"description"`
	StartDate   *time.Time `json:"start_date"`
	EndDate     *time.Time `json:"end_date"`
	TeamID      *string    `json:"team_id"`
	Archived    *bool      `json:"archived"`
	LabelIDs    []string   `json:"label_ids"`
}

// PMIterationStats contains derived iteration progress metrics.
type PMIterationStats struct {
	StoryCount     int `json:"story_count"`
	DoneStoryCount int `json:"done_story_count"`
	TotalPoints    int `json:"total_points"`
	DonePoints     int `json:"done_points"`
}

// IterationWithStats is an iteration with computed progress metrics.
type IterationWithStats struct {
	Iteration PMIteration      `json:"iteration"`
	Labels    []PMLabel        `json:"labels"`
	Stats     PMIterationStats `json:"stats"`
}
