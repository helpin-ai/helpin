package model

import "time"

// Support inbox clear scopes name the inbox view a person emptied.
const (
	// SupportInboxClearScopeMailbox is one team inbox.
	SupportInboxClearScopeMailbox = "mailbox"
	// SupportInboxClearScopeShared is the main shared inbox: conversations
	// that are not in a team inbox.
	SupportInboxClearScopeShared = "shared"
	// SupportInboxClearScopeMine is the conversations assigned to the person.
	SupportInboxClearScopeMine = "mine"
)

// SupportInboxClearCount limits which resolved conversations a clear counts.
// The zero value counts every inbox.
type SupportInboxClearCount struct {
	// MailboxID counts only conversations in this team inbox.
	MailboxID *string
	// SharedOnly counts only conversations that are in no team inbox.
	SharedOnly bool
}

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
