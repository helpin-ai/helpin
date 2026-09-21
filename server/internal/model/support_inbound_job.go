package model

import "time"

// SupportInboundJob retains accepted mail until its processing stage completes.
// IDs are stable across deliveries and attachment retries.
type SupportInboundJob struct {
	ID             string    `gorm:"type:uuid;primaryKey"`
	Kind           string    `gorm:"not null;index:idx_support_inbound_due"`
	WorkspaceID    *string   `gorm:"type:uuid;index"`
	ConversationID *string   `gorm:"type:uuid;index"`
	MessageID      *string   `gorm:"type:uuid;index"`
	Payload        string    `gorm:"type:text;not null"`
	Status         string    `gorm:"not null;index:idx_support_inbound_due"`
	Attempts       int       `gorm:"not null;default:0"`
	AvailableAt    time.Time `gorm:"not null;index:idx_support_inbound_due"`
	LeaseToken     string    `gorm:"not null;default:''"`
	LastError      string    `gorm:"type:text;not null;default:''"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (SupportInboundJob) TableName() string { return "support_inbound_jobs" }
