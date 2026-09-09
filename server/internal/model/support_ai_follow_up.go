package model

import "time"

// SupportAIFollowUp is durable work for one unanswered public AI message.
type SupportAIFollowUp struct {
	StartedAt       *time.Time `json:"-"`
	ID              string     `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID     string     `json:"-" gorm:"type:uuid;not null"`
	ConversationID  string     `json:"-" gorm:"type:uuid;not null"`
	SourceMessageID string     `json:"-" gorm:"type:uuid;not null"`
	RunID           string     `json:"run_id" gorm:"type:uuid;not null"`
	Status          string     `json:"status"`
	Reason          string     `json:"reason,omitempty"`
	DueAt           time.Time  `json:"due_at"`
	CloseHours      int        `json:"-"`
	SentMessageID   *string    `json:"sent_message_id,omitempty" gorm:"type:uuid"`
	SentAt          *time.Time `json:"sent_at,omitempty"`
	CloseAt         *time.Time `json:"close_at,omitempty"`
	LeaseUntil      *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TableName returns the durable follow-up queue table.
func (SupportAIFollowUp) TableName() string { return "support_ai_follow_ups" }

// SupportFollowUpPreview describes potential assessments; eligibility still requires Runtime context review.
type SupportFollowUpPreview struct {
	Candidates   int64                 `json:"candidates"`
	DailyLimit   int                   `json:"daily_limit"`
	LookbackDays int                   `json:"lookback_days"`
	Sample       []SupportConversation `json:"sample"`
}
