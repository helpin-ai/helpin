package model

import (
	"time"
)

// SupportTicket represents a support conversation.
type SupportTicket struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DisplayID       int       `json:"display_id" gorm:"not null;index"`
	Subject         string    `json:"subject" gorm:"not null"`
	Status          string    `json:"status" gorm:"not null;default:'open'"` // open, in_progress, waiting, resolved, closed
	Priority        string    `json:"priority" gorm:"not null;default:'medium'"` // low, medium, high, urgent
	CustomerName    *string   `json:"customer_name"`
	CustomerEmail   *string   `json:"customer_email"`
	OpenedByUserID  *string   `json:"opened_by_user_id" gorm:"type:uuid"`
	AssignedAgentID *string   `json:"assigned_agent_id" gorm:"type:uuid"`
	LinkedStoryID   *string   `json:"linked_story_id" gorm:"type:uuid"`
	Source          string    `json:"source" gorm:"not null;default:'internal'"` // widget, internal, email, api
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportTicket) TableName() string { return "support_tickets" }

// SupportMessage represents a message within a support ticket.
type SupportMessage struct {
	ID                string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TicketID          string    `json:"ticket_id" gorm:"type:uuid;not null;index"`
	SenderType        string    `json:"sender_type" gorm:"not null"` // customer, user, agent
	SenderUserID      *string   `json:"sender_user_id" gorm:"type:uuid"`
	SenderAgentID     *string   `json:"sender_agent_id" gorm:"type:uuid"`
	SenderDisplayName *string   `json:"sender_display_name"`
	Content           string    `json:"content" gorm:"not null"`
	IsInternal        bool      `json:"is_internal" gorm:"not null;default:false"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportMessage) TableName() string { return "support_messages" }

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
	ID            string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TicketID      *string   `json:"ticket_id" gorm:"type:uuid"`
	SessionToken  string    `json:"session_token" gorm:"not null;uniqueIndex"`
	CustomerName  *string   `json:"customer_name"`
	CustomerEmail *string   `json:"customer_email"`
	ExpiresAt     time.Time `json:"expires_at" gorm:"not null"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportWidgetSession) TableName() string { return "support_widget_sessions" }

// CreateTicketRequest is the payload for creating a support ticket.
type CreateTicketRequest struct {
	WorkspaceID   string  `json:"workspace_id"`
	Subject       string  `json:"subject"`
	Priority      string  `json:"priority"`
	CustomerName  *string `json:"customer_name"`
	CustomerEmail *string `json:"customer_email"`
	Source        string  `json:"source"`
}

// CreateMessageRequest is the payload for creating a support message.
type CreateMessageRequest struct {
	Content    string `json:"content"`
	IsInternal bool   `json:"is_internal"`
}

// LinkStoryRequest links a ticket to a story.
type LinkStoryRequest struct {
	StoryID string `json:"story_id"`
}

// AssignTicketAgentRequest assigns an agent to a ticket.
type AssignTicketAgentRequest struct {
	AgentID string `json:"agent_id"`
}

// UpdateTicketStatusRequest changes ticket status.
type UpdateTicketStatusRequest struct {
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
