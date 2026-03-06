package model

import (
	"time"
)

// AgentJob represents a queued job for the agent worker.
type AgentJob struct {
	ID          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	RunID       string     `json:"run_id" gorm:"type:uuid;not null;uniqueIndex"`
	Status      string     `json:"status" gorm:"not null;default:'pending'"` // pending, claimed, completed, failed
	Attempt     int        `json:"attempt" gorm:"not null;default:0"`
	MaxAttempts int        `json:"max_attempts" gorm:"not null;default:3"`
	AvailableAt time.Time  `json:"available_at" gorm:"not null;default:CURRENT_TIMESTAMP;index"`
	LockedBy    *string    `json:"locked_by"`
	LockedAt    *time.Time `json:"locked_at"`
	LastError   *string    `json:"last_error"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AgentJob) TableName() string { return "agent_jobs" }
