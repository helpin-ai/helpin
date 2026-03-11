package model

import (
	"time"
)

// SupportConversation represents a support conversation (renamed from SupportTicket).
type SupportConversation struct {
	ID              string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DisplayID       int        `json:"display_id" gorm:"not null;index"`
	Subject         string     `json:"subject" gorm:"not null"`
	Status          string     `json:"status" gorm:"not null;default:'open'"`     // open, in_progress, waiting, resolved, closed
	Priority        string     `json:"priority" gorm:"not null;default:'medium'"` // low, medium, high, urgent
	Channel         string     `json:"channel" gorm:"not null;default:'widget'"`  // widget, internal, email, api
	CustomerName    *string    `json:"customer_name"`
	CustomerEmail   *string    `json:"customer_email"`
	OpenedByUserID  *string    `json:"opened_by_user_id" gorm:"type:uuid"`
	AssignedAgentID *string    `json:"assigned_agent_id" gorm:"type:uuid"`
	LinkedStoryID   *string    `json:"linked_story_id" gorm:"type:uuid"`
	Source          string     `json:"source" gorm:"not null;default:'internal'"` // widget, internal, email, api - kept for backward compat
	CRMContactID    *string    `json:"crm_contact_id" gorm:"type:uuid;index"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	ClosedAt        *time.Time `json:"closed_at"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportConversation) TableName() string { return "support_conversations" }

// SupportMessage represents a message within a support conversation.
type SupportMessage struct {
	ID                string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID    string    `json:"conversation_id" gorm:"type:uuid;not null;index"`
	SenderType        string    `json:"sender_type" gorm:"not null"`                     // customer, user, agent, ai
	MessageType       string    `json:"message_type" gorm:"not null;default:'reply'"`    // reply, csat_survey, system
	SenderUserID      *string   `json:"sender_user_id" gorm:"type:uuid"`
	SenderAgentID     *string   `json:"sender_agent_id" gorm:"type:uuid"`
	SenderDisplayName *string   `json:"sender_display_name"`
	Content           string    `json:"content" gorm:"not null"`
	IsInternal        bool      `json:"is_internal" gorm:"not null;default:false"`
	Metadata          string    `json:"metadata" gorm:"type:jsonb"` // JSONB for CSAT ratings, AI sources, etc.
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportMessage) TableName() string { return "support_messages" }

// SupportCannedResponse represents a canned response for quick replies.
type SupportCannedResponse struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ShortCode   string    `json:"short_code" gorm:"not null"` // e.g., "greeting", "thanks"
	Title       string    `json:"title" gorm:"not null"`
	Content     string    `json:"content" gorm:"not null"`
	CreatedByID string    `json:"created_by_id" gorm:"type:uuid"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCannedResponse) TableName() string { return "support_canned_responses" }

// SupportWidgetInstallation holds workspace-level widget configuration.
type SupportWidgetInstallation struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex"`
	WidgetKey   string    `json:"widget_key" gorm:"not null"` // public key for embedding
	SecretKey   string    `json:"-" gorm:"not null"`          // for signing session tokens
	Settings    string    `json:"settings" gorm:"type:jsonb;not null;default:'{}'"`
	Active      bool      `json:"active" gorm:"not null;default:true"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportWidgetInstallation) TableName() string { return "support_widget_installations" }

// SupportWidgetSession represents a short-lived external chat session.
type SupportWidgetSession struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID *string   `json:"conversation_id" gorm:"type:uuid;index"`
	SessionToken   string    `json:"session_token" gorm:"not null;uniqueIndex"`
	CustomerName   *string   `json:"customer_name"`
	CustomerEmail  *string   `json:"customer_email"`
	ExpiresAt      time.Time `json:"expires_at" gorm:"not null"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportWidgetSession) TableName() string { return "support_widget_sessions" }

// CreateConversationRequest is the payload for creating a support conversation.
type CreateConversationRequest struct {
	WorkspaceID   string  `json:"workspace_id"`
	Subject       string  `json:"subject"`
	Priority      string  `json:"priority"`
	CustomerName  *string `json:"customer_name"`
	CustomerEmail *string `json:"customer_email"`
	Source        string  `json:"source"`
}

// CreateMessageRequest is the payload for creating a support message.
type CreateMessageRequest struct {
	Content     string `json:"content"`
	IsInternal  bool   `json:"is_internal"`
	MessageType string `json:"message_type"` // reply, csat_survey, system
}

// LinkStoryRequest links a conversation to a story.
type LinkStoryRequest struct {
	StoryID string `json:"story_id"`
}

// AssignConversationAgentRequest assigns an agent to a conversation.
type AssignConversationAgentRequest struct {
	AgentID string `json:"agent_id"`
}

// UpdateConversationStatusRequest changes conversation status.
type UpdateConversationStatusRequest struct {
	Status string `json:"status"`
}

// WidgetSessionRequest creates a new widget session.
type WidgetSessionRequest struct {
	WorkspaceSlug string  `json:"workspace_slug"`
	WidgetKey     string  `json:"widget_key"`
	CustomerName  *string `json:"customer_name"`
	CustomerEmail *string `json:"customer_email"`
}

// WidgetMessageRequest sends a message via widget.
type WidgetMessageRequest struct {
	SessionToken string `json:"session_token"`
	Content      string `json:"content"`
}

// CannedResponseRequest is the payload for CRUD operations on canned responses.
type CannedResponseRequest struct {
	ShortCode string `json:"short_code"`
	Title     string `json:"title"`
	Content   string `json:"content"`
}

// TypingIndicatorRequest represents a typing indicator event.
type TypingIndicatorRequest struct {
	IsTyping bool `json:"is_typing"`
}

// CsatSurveyRequest represents a CSAT rating submission.
type CsatSurveyRequest struct {
	Rating   int    `json:"rating"`
	Feedback string `json:"feedback"`
}
