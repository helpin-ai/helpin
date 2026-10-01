package model

import "time"

// AIExecutionUsage records raw usage without a subscription, price, or credit ledger.
type AIExecutionUsage struct {
	ID                string    `json:"id" gorm:"primaryKey"`
	WorkspaceID       string    `json:"workspace_id" gorm:"uniqueIndex:idx_ai_execution_usage_identity"`
	IdempotencyKey    string    `json:"idempotency_key" gorm:"uniqueIndex:idx_ai_execution_usage_identity"`
	RunID             string    `json:"run_id"`
	FeatureKey        string    `json:"feature_key"`
	Provider          string    `json:"provider"`
	Model             string    `json:"model"`
	InputTokens       int64     `json:"input_tokens"`
	OutputTokens      int64     `json:"output_tokens"`
	ReasoningTokens   int64     `json:"reasoning_tokens"`
	CacheReadTokens   int64     `json:"cache_read_tokens"`
	CacheWriteTokens  int64     `json:"cache_write_tokens"`
	AudioMilliseconds int64     `json:"audio_milliseconds" gorm:"not null;default:0"`
	MeasurementStatus string    `json:"measurement_status"`
	PaidTools         JSONBlob  `json:"paid_tools" gorm:"type:jsonb"`
	CreatedAt         time.Time `json:"created_at"`
}

// TableName returns the core usage telemetry table.
func (AIExecutionUsage) TableName() string { return "ai_execution_usage" }
