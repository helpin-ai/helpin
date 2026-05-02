package model

import "time"

const (
	SupportSystemTagAIHandoff  = "ai_handoff"
	SupportSystemTagAIResolved = "ai_resolved"
)

// SupportTag is a workspace-wide tag for support conversations.
type SupportTag struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"not null"`
	Color       *string   `json:"color"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportTag) TableName() string { return "support_tags" }

// SupportConversationTag links support tags to conversations.
type SupportConversationTag struct {
	ConversationID string    `json:"conversation_id" gorm:"type:uuid;primaryKey"`
	TagID          string    `json:"tag_id" gorm:"type:uuid;primaryKey"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportConversationTag) TableName() string { return "support_conversation_tags" }

type CreateSupportTagRequest struct {
	Name  string  `json:"name"`
	Color *string `json:"color"`
}

type UpdateSupportTagRequest struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}
