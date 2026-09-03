package model

import (
	"encoding/json"
	"time"
)

const (
	AIActionExecutionRunning   = "running"
	AIActionExecutionSucceeded = "succeeded"
	AIActionExecutionFailed    = "failed"
)

// AIActionExecution is the append-only audit identity for one governed AI attempt.
type AIActionExecution struct {
	ID                string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string          `json:"workspace_id" gorm:"type:text;not null;default:'';index"`
	ActionKey         string          `json:"action_key" gorm:"not null;index"`
	PolicyVersion     string          `json:"policy_version" gorm:"not null"`
	FeatureKey        string          `json:"feature_key" gorm:"not null;index"`
	Category          string          `json:"category" gorm:"not null"`
	Origin            string          `json:"origin" gorm:"not null;index"`
	Modality          string          `json:"modality" gorm:"not null"`
	Provider          string          `json:"provider" gorm:"not null"`
	Model             string          `json:"model" gorm:"not null"`
	IdempotencyKey    string          `json:"idempotency_key" gorm:"not null;uniqueIndex:idx_ai_action_attempt"`
	Attempt           int             `json:"attempt" gorm:"not null;uniqueIndex:idx_ai_action_attempt"`
	Status            string          `json:"status" gorm:"not null;index"`
	FailureClass      string          `json:"failure_class" gorm:"not null;default:''"`
	FailureMessage    string          `json:"failure_message" gorm:"type:text;not null;default:''"`
	InputTokens       int             `json:"input_tokens" gorm:"not null;default:0"`
	OutputTokens      int             `json:"output_tokens" gorm:"not null;default:0"`
	ReasoningTokens   int             `json:"reasoning_tokens" gorm:"not null;default:0"`
	CachedInputTokens int             `json:"cached_input_tokens" gorm:"not null;default:0"`
	Metadata          json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	StartedAt         time.Time       `json:"started_at" gorm:"not null"`
	CompletedAt       *time.Time      `json:"completed_at"`
	CreatedAt         time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the governed AI execution audit table.
func (AIActionExecution) TableName() string { return "ai_action_executions" }
