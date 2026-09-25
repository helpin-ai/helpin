package model

import "time"

// JevDecisionAttempt records a bounded semantic decision without source content.
// Schema is owned by the versioned SQL migration, not AutoMigrate.
type JevDecisionAttempt struct {
	ID          string `gorm:"type:uuid;primaryKey"`
	WorkspaceID string `gorm:"type:uuid;not null"`
	Feature     string `gorm:"not null"`
	SourceID    string `gorm:"not null"`
	InputHash   string `gorm:"not null"`
	Mode        string `gorm:"not null"`
	Status      string `gorm:"not null"`
	Outcome     JSONB  `gorm:"type:jsonb;not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName identifies the SQL-owned audit table.
func (JevDecisionAttempt) TableName() string { return "jev_decision_attempts" }
