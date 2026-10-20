package model

import "time"

// SupportInvestigation is an optional, independent job committed with handover.
// Stable IDs deduplicate a workspace/conversation/issue across retries.
type SupportInvestigation struct {
	ID             string `gorm:"type:uuid;primaryKey"`
	WorkspaceID    string `gorm:"type:uuid;not null"`
	ConversationID string `gorm:"type:uuid;not null"`
	RunID          string `gorm:"type:uuid;not null;uniqueIndex"`
	Question       string
	Brief          string
	Status         string    `gorm:"not null;index:idx_support_investigation_due"`
	DueAt          time.Time `gorm:"index:idx_support_investigation_due"`
	LeaseUntil     *time.Time
	StartedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TableName returns the ledger-owned investigation queue table.
func (SupportInvestigation) TableName() string { return "support_investigations" }
