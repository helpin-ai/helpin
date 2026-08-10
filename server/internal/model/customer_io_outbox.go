package model

import (
	"encoding/json"
	"time"
)

// CustomerIOOutboxStatus is the delivery state of a Customer.io lifecycle event.
type CustomerIOOutboxStatus string

const (
	// CustomerIOOutboxStatusPending marks an event ready for delivery.
	CustomerIOOutboxStatusPending CustomerIOOutboxStatus = "pending"
	// CustomerIOOutboxStatusProcessing marks an event claimed by a delivery worker.
	CustomerIOOutboxStatusProcessing CustomerIOOutboxStatus = "processing"
	// CustomerIOOutboxStatusDelivered marks an event successfully delivered to Customer.io.
	CustomerIOOutboxStatusDelivered CustomerIOOutboxStatus = "delivered"
	// CustomerIOOutboxStatusFailed marks an event whose delivery has permanently failed.
	CustomerIOOutboxStatusFailed CustomerIOOutboxStatus = "failed"
)

// CustomerIOOutboxRecipient captures event-time workspace context for one recipient.
type CustomerIOOutboxRecipient struct {
	UserID        string `json:"user_id"`
	WorkspaceRole string `json:"workspace_role"`
}

// CustomerIOOutbox stores a durable Customer.io lifecycle event and its delivery state.
type CustomerIOOutbox struct {
	ID                string                 `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SemanticKey       string                 `json:"semantic_key" gorm:"not null;uniqueIndex"`
	WorkspaceID       *string                `json:"workspace_id,omitempty" gorm:"type:uuid;index"`
	EventName         string                 `json:"event_name" gorm:"not null"`
	OccurredAt        time.Time              `json:"occurred_at" gorm:"not null"`
	Attributes        json.RawMessage        `json:"attributes" gorm:"type:jsonb;not null;default:'{}'"`
	RecipientSnapshot json.RawMessage        `json:"recipient_snapshot" gorm:"type:jsonb;not null;default:'{}'"`
	Status            CustomerIOOutboxStatus `json:"status" gorm:"not null;default:pending;index"`
	Attempts          int                    `json:"attempts" gorm:"not null;default:0"`
	NextAttemptAt     time.Time              `json:"next_attempt_at" gorm:"not null;index"`
	ClaimToken        *string                `json:"claim_token,omitempty"`
	ClaimedAt         *time.Time             `json:"claimed_at,omitempty"`
	LeaseExpiresAt    *time.Time             `json:"lease_expires_at,omitempty" gorm:"index"`
	LastError         *string                `json:"last_error,omitempty"`
	CreatedAt         time.Time              `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time              `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the Customer.io lifecycle outbox table name.
func (CustomerIOOutbox) TableName() string { return "customer_io_outbox" }
