package model

import (
	"encoding/json"
	"time"
)

// CompanyGoal represents a row in the company_goals table.
type CompanyGoal struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	QuarterID    string    `json:"quarter_id" gorm:"type:uuid;not null;index"`
	Title        string    `json:"title" gorm:"not null"`
	Description  *string   `json:"description"`
	GoalType     string    `json:"goal_type" gorm:"not null"`
	Baseline     *float64  `json:"baseline"`
	Target       *float64  `json:"target"`
	CurrentValue *float64  `json:"current_value"`
	Unit         *string   `json:"unit"`
	Status       string    `json:"status" gorm:"not null;default:'active'"`
	CreatedBy    *string   `json:"created_by" gorm:"type:uuid"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CompanyGoal) TableName() string { return "company_goals" }

// CompanyGoalWithContributions is a company goal with its team contributions.
type CompanyGoalWithContributions struct {
	CompanyGoal
	TeamContributions []GoalTeamContribution `json:"team_contributions"`
}

// GoalTeamContribution represents a row in the goal_team_contributions table.
type GoalTeamContribution struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	GoalID          string    `json:"goal_id" gorm:"type:uuid;not null;index"`
	TeamID          string    `json:"team_id" gorm:"type:uuid;not null"`
	ContributionPct float64   `json:"contribution_pct" gorm:"not null"`
	TargetValue     *float64  `json:"target_value"`
	CurrentValue    *float64  `json:"current_value"`
	Rationale       *string   `json:"rationale"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GoalTeamContribution) TableName() string { return "goal_team_contributions" }

// SprintGoal represents a row in the sprint_goals table.
type SprintGoal struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SprintID    string    `json:"sprint_id" gorm:"type:uuid;not null;uniqueIndex:idx_sprint_goal_unique"`
	TeamID      string    `json:"team_id" gorm:"type:uuid;not null;uniqueIndex:idx_sprint_goal_unique"`
	Title       string    `json:"title" gorm:"not null;uniqueIndex:idx_sprint_goal_unique"`
	Description *string   `json:"description"`
	Weight      int       `json:"weight" gorm:"not null;default:0"`
	Done        bool      `json:"done" gorm:"not null;default:false"`
	KRID        *string   `json:"kr_id" gorm:"column:kr_id;type:uuid"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SprintGoal) TableName() string { return "sprint_goals" }

// GoalDraft represents a row in the goal_drafts table.
type GoalDraft struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	QuarterID   string          `json:"quarter_id" gorm:"type:uuid;not null;index"`
	CreatedBy   *string         `json:"created_by" gorm:"type:uuid"`
	DraftData   json.RawMessage `json:"draft_data" gorm:"type:jsonb"`
	Status      string          `json:"status" gorm:"not null;default:'draft'"`
	CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GoalDraft) TableName() string { return "goal_drafts" }

// CreateGoalRequest is the payload for creating a company goal.
type CreateGoalRequest struct {
	WorkspaceID       string                          `json:"workspace_id"`
	QuarterID         string                          `json:"quarter_id"`
	Title             string                          `json:"title"`
	Description       *string                         `json:"description"`
	GoalType          string                          `json:"goal_type"`
	Baseline          *float64                        `json:"baseline"`
	Target            *float64                        `json:"target"`
	Unit              *string                         `json:"unit"`
	TeamContributions []CreateTeamContributionRequest `json:"team_contributions"`
}

// CreateTeamContributionRequest is a nested payload for goal team contributions.
type CreateTeamContributionRequest struct {
	TeamID          string   `json:"team_id"`
	ContributionPct float64  `json:"contribution_pct"`
	TargetValue     *float64 `json:"target_value"`
	Rationale       *string  `json:"rationale"`
}

// UpsertSprintGoalRequest is the payload for upserting a sprint goal.
type UpsertSprintGoalRequest struct {
	SprintID    string  `json:"sprint_id"`
	TeamID      string  `json:"team_id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Weight      int     `json:"weight"`
	Done        bool    `json:"done"`
	KRID        *string `json:"kr_id"`
}

// CreateDraftRequest is the payload for creating a goal draft.
type CreateDraftRequest struct {
	WorkspaceID string          `json:"workspace_id"`
	QuarterID   string          `json:"quarter_id"`
	DraftData   json.RawMessage `json:"draft_data"`
}

// UpdateDraftRequest is the payload for updating a goal draft.
type UpdateDraftRequest struct {
	DraftData json.RawMessage `json:"draft_data"`
	Status    *string         `json:"status"`
}
