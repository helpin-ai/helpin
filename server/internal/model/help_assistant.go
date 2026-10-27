package model

import "time"

// HelpAssistantTurnTarget is the agent run target of one help assistant
// turn. Its ID is the pending assistant message the run answers.
const HelpAssistantTurnTarget = "help_assistant_turn"

// Help assistant surfaces.
const (
	HelpAssistantSurfaceHelpcenter = "helpcenter"
	HelpAssistantSurfacePortal     = "portal"
)

// Help assistant message roles and statuses.
const (
	HelpAssistantRoleUser      = "user"
	HelpAssistantRoleAssistant = "assistant"

	HelpAssistantStatusPending   = "pending"
	HelpAssistantStatusAnswered  = "answered"
	HelpAssistantStatusHandedOff = "handed_off"
	HelpAssistantStatusFailed    = "failed"
)

// HelpAssistantSession is one conversation with the help assistant. Its
// audience decides which help spaces the assistant may read: "public", or
// "customers" for a customer signed in to the portal.
type HelpAssistantSession struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null"`
	Surface          string    `json:"surface" gorm:"not null"`
	Audience         string    `json:"audience" gorm:"not null"`
	PortalIdentityID *string   `json:"portal_identity_id,omitempty" gorm:"type:uuid"`
	VisitorID        string    `json:"-" gorm:"not null"`
	Locale           string    `json:"locale" gorm:"not null;default:''"`
	TurnCount        int       `json:"turn_count" gorm:"not null;default:0"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the help assistant sessions table name.
func (HelpAssistantSession) TableName() string { return "help_assistant_sessions" }

// HelpAssistantMessage is a question or an answer in a session.
type HelpAssistantMessage struct {
	ID            string  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SessionID     string  `json:"session_id" gorm:"type:uuid;not null"`
	WorkspaceID   string  `json:"workspace_id" gorm:"type:uuid;not null"`
	Role          string  `json:"role" gorm:"not null"`
	Content       string  `json:"content" gorm:"not null;default:''"`
	Status        string  `json:"status" gorm:"not null"`
	ProgressLabel *string `json:"progress_label,omitempty"`
	// Citations is a JSON array of HelpAssistantCitation.
	Citations     *string    `json:"citations,omitempty" gorm:"type:jsonb"`
	PageArticleID *string    `json:"page_article_id,omitempty"`
	RunID         *string    `json:"run_id,omitempty" gorm:"type:uuid"`
	Helpful       *bool      `json:"helpful,omitempty"`
	CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// TableName returns the help assistant messages table name.
func (HelpAssistantMessage) TableName() string { return "help_assistant_messages" }

// HelpAssistantCitation is an article an answer is based on. Public articles
// carry their help center URL; articles for signed-in customers carry their
// public ID, which the portal links to.
type HelpAssistantCitation struct {
	ArticleID       string `json:"article_id"`
	Title           string `json:"title"`
	URL             string `json:"url,omitempty"`
	PortalArticleID string `json:"portal_article_id,omitempty"`
}
