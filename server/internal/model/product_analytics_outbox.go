package model

import (
	"encoding/json"
	"time"
)

// ProductAnalyticsOutboxStatus is the delivery state of a canonical product event.
type ProductAnalyticsOutboxStatus string

const (
	// ProductAnalyticsOutboxStatusPending marks an event ready for delivery.
	ProductAnalyticsOutboxStatusPending ProductAnalyticsOutboxStatus = "pending"
	// ProductAnalyticsOutboxStatusProcessing marks an event claimed by a delivery worker.
	ProductAnalyticsOutboxStatusProcessing ProductAnalyticsOutboxStatus = "processing"
	// ProductAnalyticsOutboxStatusDelivered marks an event successfully delivered.
	ProductAnalyticsOutboxStatusDelivered ProductAnalyticsOutboxStatus = "delivered"
	// ProductAnalyticsOutboxStatusFailed marks an event whose delivery permanently failed.
	ProductAnalyticsOutboxStatusFailed ProductAnalyticsOutboxStatus = "failed"
)

// ProductAnalyticsOutbox stores one canonical product event for durable delivery.
type ProductAnalyticsOutbox struct {
	ID             string                       `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SemanticKey    string                       `json:"semantic_key" gorm:"not null;uniqueIndex"`
	UserID         *string                      `json:"user_id,omitempty" gorm:"type:uuid;index"`
	AnonymousID    *string                      `json:"anonymous_id,omitempty"`
	WorkspaceID    *string                      `json:"workspace_id,omitempty" gorm:"type:uuid;index"`
	EventName      string                       `json:"event_name" gorm:"not null"`
	Source         string                       `json:"source" gorm:"not null"`
	OccurredAt     time.Time                    `json:"occurred_at" gorm:"not null"`
	Attributes     json.RawMessage              `json:"attributes" gorm:"type:jsonb;not null;default:'{}'"`
	Status         ProductAnalyticsOutboxStatus `json:"status" gorm:"not null;default:pending;index"`
	Attempts       int                          `json:"attempts" gorm:"not null;default:0"`
	NextAttemptAt  time.Time                    `json:"next_attempt_at" gorm:"not null;index"`
	ClaimToken     *string                      `json:"claim_token,omitempty"`
	ClaimedAt      *time.Time                   `json:"claimed_at,omitempty"`
	LeaseExpiresAt *time.Time                   `json:"lease_expires_at,omitempty" gorm:"index"`
	LastError      *string                      `json:"last_error,omitempty"`
	CreatedAt      time.Time                    `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time                    `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the canonical product analytics outbox table name.
func (ProductAnalyticsOutbox) TableName() string { return "product_analytics_outbox" }
