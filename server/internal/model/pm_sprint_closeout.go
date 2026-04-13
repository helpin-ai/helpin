package model

import "time"

const (
	PMSprintCloseoutOutcomeCompleted           = "completed"
	PMSprintCloseoutOutcomeUnfinishedNotRolled = "unfinished_not_rolled"
	PMSprintCloseoutOutcomeRolledOver          = "rolled_over"
)

type PMSprintCloseout struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SprintID         string    `json:"sprint_id" gorm:"type:uuid;not null;uniqueIndex"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TeamID           *string   `json:"team_id,omitempty" gorm:"type:uuid;index"`
	RolledToSprintID *string   `json:"rolled_to_sprint_id,omitempty" gorm:"type:uuid;index"`
	CommittedCount   int       `json:"committed_count" gorm:"not null;default:0"`
	CompletedCount   int       `json:"completed_count" gorm:"not null;default:0"`
	UnfinishedCount  int       `json:"unfinished_count" gorm:"not null;default:0"`
	RolledOverCount  int       `json:"rolled_over_count" gorm:"not null;default:0"`
	CommittedPoints  int       `json:"committed_points" gorm:"not null;default:0"`
	CompletedPoints  int       `json:"completed_points" gorm:"not null;default:0"`
	UnfinishedPoints int       `json:"unfinished_points" gorm:"not null;default:0"`
	RolledOverPoints int       `json:"rolled_over_points" gorm:"not null;default:0"`
	ClosedAt         time.Time `json:"closed_at" gorm:"not null"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMSprintCloseout) TableName() string { return "pm_sprint_closeouts" }

type PMSprintCloseoutTask struct {
	ID         string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CloseoutID string    `json:"closeout_id" gorm:"type:uuid;not null;uniqueIndex:idx_pm_sprint_closeout_task_unique,priority:1;index"`
	TaskID     string    `json:"task_id" gorm:"type:uuid;not null;uniqueIndex:idx_pm_sprint_closeout_task_unique,priority:2"`
	Outcome    string    `json:"outcome" gorm:"not null"`
	Estimate   int       `json:"estimate" gorm:"not null;default:0"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMSprintCloseoutTask) TableName() string { return "pm_sprint_closeout_tasks" }

type SprintInboundRolloverSummary struct {
	SourceSprintID   string `json:"source_sprint_id"`
	SourceSprintName string `json:"source_sprint_name"`
	RolledOverCount  int    `json:"rolled_over_count"`
	RolledOverPoints int    `json:"rolled_over_points"`
}

type SprintCloseoutResponse struct {
	Closeout     *PMSprintCloseout              `json:"closeout"`
	RolledInFrom []SprintInboundRolloverSummary `json:"rolled_in_from"`
}

type SprintCloseoutListResponse struct {
	Items []SprintCloseoutListItem `json:"items"`
}

type SprintCloseoutListItem struct {
	CloseoutID         string     `json:"closeout_id"`
	SprintID           string     `json:"sprint_id"`
	SprintName         string     `json:"sprint_name"`
	TeamID             *string    `json:"team_id,omitempty"`
	TeamName           *string    `json:"team_name,omitempty"`
	StartDate          *time.Time `json:"start_date,omitempty"`
	EndDate            *time.Time `json:"end_date,omitempty"`
	CommittedCount     int        `json:"committed_count"`
	CompletedCount     int        `json:"completed_count"`
	UnfinishedCount    int        `json:"unfinished_count"`
	RolledOverCount    int        `json:"rolled_over_count"`
	CommittedPoints    int        `json:"committed_points"`
	CompletedPoints    int        `json:"completed_points"`
	UnfinishedPoints   int        `json:"unfinished_points"`
	RolledOverPoints   int        `json:"rolled_over_points"`
	CompletionRate     float64    `json:"completion_rate"`
	RolledToSprintID   *string    `json:"rolled_to_sprint_id,omitempty"`
	RolledToSprintName *string    `json:"rolled_to_sprint_name,omitempty"`
	ClosedAt           time.Time  `json:"closed_at"`
}
