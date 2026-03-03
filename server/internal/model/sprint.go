package model

import "time"

// Sprint represents a row in the sprints table.
type Sprint struct {
	ID           string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	QuarterID    string     `json:"quarter_id" gorm:"type:uuid;not null;index"`
	WorkspaceID  string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SprintNumber int        `json:"sprint_number" gorm:"not null"`
	StartDate    string     `json:"start_date" gorm:"not null"`
	EndDate      string     `json:"end_date" gorm:"not null"`
	Status       string     `json:"status" gorm:"not null;default:'active'"`
	LockedAt     *time.Time `json:"locked_at"`
	LockedBy     *string    `json:"locked_by" gorm:"type:uuid"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Sprint) TableName() string { return "sprints" }

// SprintWithGoals is a sprint together with its sprint goals.
type SprintWithGoals struct {
	Sprint
	Goals []SprintGoal `json:"goals,omitempty"`
}
