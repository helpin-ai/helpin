package model

import "time"

// CRMMailboxSendingPolicy shares limits across connections to one physical mailbox.
type CRMMailboxSendingPolicy struct {
	MailboxKey         string     `json:"-" gorm:"primaryKey"`
	DailyLimit         int        `json:"daily_limit"`
	ManualReserve      int        `json:"manual_reserve"`
	MinIntervalSeconds int        `json:"min_interval_seconds"`
	CooldownUntil      *time.Time `json:"cooldown_until,omitempty"`
}

// TableName is the stable policy table.
func (CRMMailboxSendingPolicy) TableName() string { return "crm_mailbox_sending_policies" }

// CRMEmailSendReservation durably counts attempts before contacting the provider.
type CRMEmailSendReservation struct {
	ID          string    `json:"-" gorm:"primaryKey"`
	MailboxKey  string    `json:"-" gorm:"index"`
	WorkspaceID string    `json:"-"`
	AccountID   string    `json:"-"`
	SequenceID  string    `json:"-" gorm:"index"`
	Automated   bool      `json:"-"`
	FirstEmail  bool      `json:"-"`
	Status      string    `json:"-"`
	CreatedAt   time.Time `json:"-" gorm:"index"`
}

// TableName is the stable reservation table.
func (CRMEmailSendReservation) TableName() string { return "crm_email_send_reservations" }

// CRMMailboxCapacity is the owner's compact sending usage summary.
type CRMMailboxCapacity struct {
	CRMMailboxSendingPolicy
	AccountID         string     `json:"account_id"`
	Email             string     `json:"email"`
	Used              int        `json:"used"`
	Sent              int        `json:"sent"`
	Remaining         int        `json:"remaining"`
	SequenceRemaining int        `json:"sequence_remaining"`
	Queued            int64      `json:"queued"`
	NextAvailableAt   *time.Time `json:"next_available_at,omitempty"`
}
