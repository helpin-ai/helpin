package model

import "time"

// RewardIndividualCheck represents a row in the reward_individual_checks table.
type RewardIndividualCheck struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SprintID    string    `json:"sprint_id" gorm:"type:uuid;not null;uniqueIndex:idx_check_sprint_emp_criteria"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	EmployeeID  string    `json:"employee_id" gorm:"type:uuid;not null;uniqueIndex:idx_check_sprint_emp_criteria"`
	ScoredBy    *string   `json:"scored_by" gorm:"type:uuid"`
	CriteriaID  string    `json:"criteria_id" gorm:"not null;uniqueIndex:idx_check_sprint_emp_criteria"`
	Answer      bool      `json:"answer" gorm:"not null;default:false"`
	Notes       *string   `json:"notes"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (RewardIndividualCheck) TableName() string { return "reward_individual_checks" }

// UpsertRewardCheckRequest is the payload for upserting an individual check.
type UpsertRewardCheckRequest struct {
	SprintID    string  `json:"sprint_id"`
	WorkspaceID string  `json:"workspace_id"`
	EmployeeID  string  `json:"employee_id"`
	CriteriaID  string  `json:"criteria_id"`
	Answer      bool    `json:"answer"`
	Notes       *string `json:"notes"`
}
