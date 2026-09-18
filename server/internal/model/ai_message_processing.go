package model

import "time"

// AIMessageProcessing tracks AI consumer processing state for durable idempotency.
type AIMessageProcessing struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string    `json:"workspace_id" gorm:"type:uuid;not null"`
	SourceMessageID string    `json:"source_message_id" gorm:"type:uuid;not null;uniqueIndex"`
	ConversationID  string    `json:"conversation_id" gorm:"type:uuid;not null"`
	ReplyMessageID  *string   `json:"reply_message_id" gorm:"type:uuid"`
	Status          string    `json:"status" gorm:"not null;default:'processing'"` // processing, waiting_for_result, deferred, completed, failed
	Attempts        int       `json:"attempts" gorm:"not null;default:1"`
	TokensUsed      int       `json:"tokens_used" gorm:"not null;default:0"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AIMessageProcessing) TableName() string { return "ai_message_processing" }
