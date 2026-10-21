package model

import "time"

// SupportTagJob is identifier-only automatic tagging work persisted with a reply.
// Its SQL-owned schema and enqueue trigger live in the migration ledger.
type SupportTagJob struct {
	MessageID      string `gorm:"type:uuid;primaryKey"`
	WorkspaceID    string `gorm:"type:uuid;not null"`
	ConversationID string `gorm:"type:uuid;not null"`
	Status         string
	Attempts       int
	AvailableAt    time.Time
	LeaseToken     string
	LeaseUntil     *time.Time
	ResultCode     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TableName identifies the durable automatic tagging queue.
func (SupportTagJob) TableName() string { return "support_tag_jobs" }
