package model

import "time"

const (
	PMSprintStatusUnstarted = "unstarted"
	PMSprintStatusStarted   = "started"
	PMSprintStatusDone      = "done"
)

// PMSprint represents a sprint planning period.
type PMSprint struct {
	ID          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string     `json:"name" gorm:"not null"`
	Description *string    `json:"description"`
	ExternalID  *string    `json:"external_id" gorm:"index"`
	StartDate   *time.Time `json:"start_date" gorm:"type:date"`
	EndDate     *time.Time `json:"end_date" gorm:"type:date"`
	Status      string     `json:"status" gorm:"->"`
	TeamID      *string    `json:"team_id" gorm:"type:uuid;index"`
	Archived    bool       `json:"archived" gorm:"not null;default:false"`
	CreatedBy   *string    `json:"created_by" gorm:"type:uuid"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
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
	// AccessibleTeamIDs enforces team-based access boundaries.
	// nil = no filtering (admin/owner), [] = no access, [ids] = filter to these teams.
	AccessibleTeamIDs []string
}

// PMSprintPlanningFilters applies filters when building the sprint planning workspace.
type PMSprintPlanningFilters struct {
	TeamID           *string
	IncludeCompleted bool
	PreviewTaskLimit int
	BacklogLimit     int
	// AccessibleTeamIDs enforces team-based access boundaries.
	// nil = no filtering (admin/owner), [] = no access, [ids] = filter to these teams.
	AccessibleTeamIDs []string
}

// CreateSprintRequest is the payload for creating a sprint.
type CreateSprintRequest struct {
	WorkspaceID   string    `json:"workspace_id"`
	Name          string    `json:"name"`
	Description   *string   `json:"description"`
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
	TeamID        *string   `json:"team_id"`
	LabelIDs      []string  `json:"label_ids"`
	AttachmentIDs []string  `json:"attachment_ids,omitempty"`
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
	TaskCount     int `json:"task_count"`
	DoneTaskCount int `json:"done_task_count"`
	TotalPoints   int `json:"total_points"`
	DonePoints    int `json:"done_points"`
}

// SprintWithStats is a sprint with computed progress metrics.
type SprintWithStats struct {
	Sprint PMSprint      `json:"sprint"`
	Labels []PMLabel     `json:"labels"`
	Stats  PMSprintStats `json:"stats"`
}

// SprintPlanningTaskPreview is the lightweight task payload used by the sprint planning page.
type SprintPlanningTaskPreview struct {
	ID              string   `json:"id"`
	DisplayID       int      `json:"display_id"`
	Name            string   `json:"name"`
	WorkflowStateID string   `json:"workflow_state_id"`
	StateName       *string  `json:"state_name,omitempty"`
	StateType       *string  `json:"state_type,omitempty"`
	OwnerMemberID   *string  `json:"owner_member_id,omitempty"`
	OwnerMemberIDs  []string `json:"owner_member_ids,omitempty"`
	Estimate        *int     `json:"estimate,omitempty"`
	Priority        string   `json:"priority"`
	SprintID        *string  `json:"sprint_id,omitempty"`
	TeamID          *string  `json:"team_id,omitempty"`
}

// SprintPlanningCard is the sprint card payload for the planning page.
type SprintPlanningCard struct {
	Sprint              PMSprint                    `json:"sprint"`
	Stats               PMSprintStats               `json:"stats"`
	PreviewTasks        []SprintPlanningTaskPreview `json:"preview_tasks"`
	TaskPreviewOverflow int                         `json:"task_preview_overflow"`
}

// SprintPlanningBucket groups planning cards by temporal bucket.
type SprintPlanningBucket struct {
	Key     string               `json:"key"`
	Label   string               `json:"label"`
	Sprints []SprintPlanningCard `json:"sprints"`
}

// SprintPlanningWorkspace is the top-level response for the sprint planning page.
type SprintPlanningWorkspace struct {
	Buckets      []SprintPlanningBucket      `json:"buckets"`
	BacklogTasks []SprintPlanningTaskPreview `json:"backlog_tasks"`
	BacklogTotal int                         `json:"backlog_total"`
}
