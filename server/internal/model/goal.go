package model

import (
	"encoding/json"
	"time"
)

// RewardCompanyGoal represents a row in the reward_company_goals table.
type RewardCompanyGoal struct {
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

func (RewardCompanyGoal) TableName() string { return "reward_company_goals" }

// RewardCompanyGoalWithContributions is a company goal with its team contributions.
type RewardCompanyGoalWithContributions struct {
	RewardCompanyGoal
	TeamContributions []RewardGoalTeamContribution `json:"team_contributions"`
}

// RewardGoalTeamContribution represents a row in the reward_goal_team_contributions table.
type RewardGoalTeamContribution struct {
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

func (RewardGoalTeamContribution) TableName() string { return "reward_goal_team_contributions" }

// RewardSprintGoal represents a row in the reward_sprint_goals table.
type RewardSprintGoal struct {
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

func (RewardSprintGoal) TableName() string { return "reward_sprint_goals" }

// RewardGoalDraft represents a row in the reward_goal_drafts table.
type RewardGoalDraft struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	QuarterID   string          `json:"quarter_id" gorm:"type:uuid;not null;index"`
	CreatedBy   *string         `json:"created_by" gorm:"type:uuid"`
	DraftData   json.RawMessage `json:"draft_data" gorm:"type:jsonb"`
	Status      string          `json:"status" gorm:"not null;default:'draft'"`
	CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (RewardGoalDraft) TableName() string { return "reward_goal_drafts" }

// CreateRewardGoalRequest is the payload for creating a company goal.
type CreateRewardGoalRequest struct {
	WorkspaceID       string                                    `json:"workspace_id"`
	QuarterID         string                                    `json:"quarter_id"`
	Title             string                                    `json:"title"`
	Description       *string                                   `json:"description"`
	GoalType          string                                    `json:"goal_type"`
	Baseline          *float64                                  `json:"baseline"`
	Target            *float64                                  `json:"target"`
	Unit              *string                                   `json:"unit"`
	TeamContributions []CreateRewardGoalTeamContributionRequest `json:"team_contributions"`
}

// CreateRewardGoalTeamContributionRequest is a nested payload for goal team contributions.
type CreateRewardGoalTeamContributionRequest struct {
	TeamID          string   `json:"team_id"`
	ContributionPct float64  `json:"contribution_pct"`
	TargetValue     *float64 `json:"target_value"`
	Rationale       *string  `json:"rationale"`
}

// UpsertRewardSprintGoalRequest is the payload for upserting a sprint goal.
type UpsertRewardSprintGoalRequest struct {
	SprintID    string  `json:"sprint_id"`
	TeamID      string  `json:"team_id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Weight      int     `json:"weight"`
	Done        bool    `json:"done"`
	KRID        *string `json:"kr_id"`
}

// CreateRewardDraftRequest is the payload for creating a goal draft.
type CreateRewardDraftRequest struct {
	WorkspaceID string          `json:"workspace_id"`
	QuarterID   string          `json:"quarter_id"`
	DraftData   json.RawMessage `json:"draft_data"`
}

// UpdateRewardDraftRequest is the payload for updating a goal draft.
type UpdateRewardDraftRequest struct {
	DraftData json.RawMessage `json:"draft_data"`
	Status    *string         `json:"status"`
}
