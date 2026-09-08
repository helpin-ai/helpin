package model

import "time"

const (
	// AutomationEventScheduled waits for its persisted due time or retry time.
	AutomationEventScheduled = "scheduled"
	// AutomationEventProcessing is protected by a bounded, fenced delivery lease.
	AutomationEventProcessing = "processing"
	// AutomationEventDelivered confirms event processing, never a business outcome.
	AutomationEventDelivered = "delivered"
	// AutomationEventCancelled confirms that further delivery of this event is revoked.
	AutomationEventCancelled = "cancelled"
	// AutomationEventFailed requires intervention after bounded delivery attempts.
	AutomationEventFailed = "failed"
	// CRMCheckpointEvent requests reevaluation only; it grants no execution authority.
	CRMCheckpointEvent = "crm.checkpoint_due"
)

// AutomationScheduledEvent is a durable product event, not an agent run or a Flow.
// Identity and due-time fields are immutable. Consumers must acknowledge under the
// lease and commit their idempotent domain effect in the same transaction.
type AutomationScheduledEvent struct {
	ID               string     `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID      string     `json:"workspace_id" gorm:"type:uuid;not null"`
	EventKey         string     `json:"-" gorm:"not null"`
	Kind             string     `json:"kind" gorm:"not null"`
	TargetType       string     `json:"target_type" gorm:"not null"`
	TargetID         string     `json:"target_id" gorm:"type:uuid;not null"`
	ExpectedRevision int64      `json:"-" gorm:"not null"`
	DueAt            time.Time  `json:"due_at" gorm:"not null"`
	AvailableAt      time.Time  `json:"-" gorm:"not null"`
	Status           string     `json:"status" gorm:"not null"`
	Attempts         int        `json:"attempts" gorm:"not null"`
	MaxAttempts      int        `json:"-" gorm:"not null"`
	LeaseToken       *string    `json:"-" gorm:"type:uuid"`
	LeaseUntil       *time.Time `json:"-"`
	ResultCode       string     `json:"result_code" gorm:"not null"`
	CompletedAt      *time.Time `json:"completed_at"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName identifies the shared Automation event outbox.
func (AutomationScheduledEvent) TableName() string { return "automation_scheduled_events" }
