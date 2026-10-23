package model

import "time"

// Support inbox clear scopes name the inbox view a person emptied.
const (
	SupportInboxClearScopeMailbox = "mailbox"
	SupportInboxClearScopeAll     = "all"
	SupportInboxClearScopeMine    = "mine"
)

// SupportInboxClear records one person emptying a support inbox view by
// resolving or marking spam its last conversation.
type SupportInboxClear struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null"`
	UserID         string    `json:"user_id" gorm:"type:uuid;not null"`
	ScopeKey       string    `json:"scope_key" gorm:"not null"`
	MailboxID      *string   `json:"mailbox_id,omitempty" gorm:"type:uuid"`
	ConversationID string    `json:"conversation_id" gorm:"type:uuid;not null"`
	LocalDate      string    `json:"local_date" gorm:"type:varchar(10);not null"`
	ClearedAt      time.Time `json:"cleared_at" gorm:"not null"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the ledger-owned inbox clear table.
func (SupportInboxClear) TableName() string { return "support_inbox_clears" }

// RecordSupportInboxClearRequest is the body of POST /api/support/inbox-clears.
type RecordSupportInboxClearRequest struct {
	ConversationID string  `json:"conversation_id"`
	Scope          string  `json:"scope"`
	MailboxID      *string `json:"mailbox_id"`
	LocalDate      string  `json:"local_date"`
	DayStart       string  `json:"day_start"`
}

// SupportInboxClearStats are the facts the inbox-zero celebration may state.
type SupportInboxClearStats struct {
	StreakDays    int `json:"streak_days"`
	ClearsToday   int `json:"clears_today"`
	ResolvedToday int `json:"resolved_today"`
	ClearedCount  int `json:"cleared_count"`
}
