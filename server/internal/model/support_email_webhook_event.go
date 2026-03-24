package model

import "time"

// SupportEmailWebhookEvent stores raw Postmark webhook requests for audit/debugging.
type SupportEmailWebhookEvent struct {
	ID                string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       *string    `json:"workspace_id,omitempty" gorm:"type:uuid;index"`
	ConversationID    *string    `json:"conversation_id,omitempty" gorm:"type:uuid;index"`
	EmailLogID        *string    `json:"email_log_id,omitempty" gorm:"type:uuid;index"`
	Provider          string     `json:"provider" gorm:"size:20;not null;default:'postmark'"`
	EventType         string     `json:"event_type" gorm:"size:30;not null;index"`
	PostmarkMessageID *string    `json:"postmark_message_id,omitempty" gorm:"size:255;index"`
	MessageStream     *string    `json:"message_stream,omitempty" gorm:"size:50"`
	RawPayload        string     `json:"raw_payload" gorm:"type:jsonb;not null"`
	ReceivedAt        *time.Time `json:"received_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportEmailWebhookEvent) TableName() string { return "support_email_webhook_events" }
