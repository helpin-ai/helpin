package model

import "time"

// SupportTranslation is a private snapshot, never a public widget message.
// Source hashes identify exact text revisions without changing canonical messages.
type SupportTranslation struct {
	RetryAttempt    int        `json:"-" gorm:"-"`
	PolicyRevision  int64      `json:"-"`
	ID              string     `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID     string     `json:"-" gorm:"type:uuid;not null"`
	ConversationID  string     `json:"conversation_id" gorm:"type:uuid;not null"`
	Purpose         string     `json:"purpose" gorm:"not null"`
	SourceMessageID *string    `json:"source_message_id,omitempty" gorm:"type:uuid"`
	CreatedByUserID *string    `json:"-" gorm:"type:uuid"`
	SentMessageID   *string    `json:"sent_message_id,omitempty" gorm:"type:uuid"`
	SourceText      string     `json:"source_text" gorm:"not null"`
	SourceHash      string     `json:"source_hash" gorm:"not null"`
	SourceLanguage  string     `json:"source_language" gorm:"not null;default:''"`
	TargetLanguage  string     `json:"target_language" gorm:"not null"`
	TranslatedText  string     `json:"translated_text" gorm:"not null;default:''"`
	SendKey         string     `json:"-" gorm:"not null;default:''"`
	CacheKey        string     `json:"-" gorm:"not null"`
	Provider        string     `json:"provider,omitempty" gorm:"not null;default:''"`
	Model           string     `json:"model,omitempty" gorm:"not null;default:''"`
	PipelineVersion string     `json:"-" gorm:"not null"`
	Attempts        int        `json:"-" gorm:"type:integer;not null;default:1"`
	Status          string     `json:"status" gorm:"not null"`
	ReviewStatus    string     `json:"review_status" gorm:"not null;default:'not_requested'"`
	JevAssessmentID *string    `json:"-" gorm:"type:uuid"`
	SentByUserID    *string    `json:"-" gorm:"type:uuid"`
	SentAt          *time.Time `json:"-"`
	ErrorCode       string     `json:"error_code,omitempty" gorm:"not null;default:''"`
	CreatedAt       time.Time  `json:"created_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
}

func (SupportTranslation) TableName() string { return "support_translations" }

type SupportTranslationPreference struct {
	WorkspaceID           string    `json:"-" gorm:"primaryKey;type:uuid"`
	UserID                string    `json:"-" gorm:"primaryKey;type:uuid"`
	ReadingLanguage       string    `json:"reading_language" gorm:"not null;default:'en'"`
	AutoTranslateOutgoing bool      `json:"auto_translate_outgoing" gorm:"not null"`
	AutoTranslateIncoming bool      `json:"auto_translate_incoming" gorm:"not null"`
	UpdatedAt             time.Time `json:"-" gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (SupportTranslationPreference) TableName() string { return "support_translation_preferences" }

type SupportTranslationConversation struct {
	Revision         int64     `json:"revision" gorm:"not null;default:1"`
	WorkspaceID      string    `json:"-" gorm:"primaryKey;type:uuid"`
	ConversationID   string    `json:"-" gorm:"primaryKey;type:uuid"`
	CustomerLanguage string    `json:"customer_language" gorm:"not null;default:''"`
	TranslationMode  string    `json:"translation_mode" gorm:"not null;default:'inherit'"`
	UpdatedAt        time.Time `json:"-" gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (SupportTranslationConversation) TableName() string { return "support_translation_conversations" }

type SupportTranslationOptions struct {
	DetectedCustomerLanguage string                         `json:"detected_customer_language,omitempty"`
	Available                bool                           `json:"available"`
	UnavailableReason        string                         `json:"unavailable_reason,omitempty"`
	JevReview                bool                           `json:"jev_review"`
	Languages                map[string]string              `json:"languages"`
	Preference               SupportTranslationPreference   `json:"preference"`
	Conversation             SupportTranslationConversation `json:"conversation"`
}

type SupportTranslateRequest struct {
	MessageID      string `json:"message_id,omitempty"`
	Content        string `json:"content,omitempty"`
	DraftID        string `json:"draft_id,omitempty"`
	TargetLanguage string `json:"target_language"`
	Live           bool   `json:"-"`
}

// SupportLiveMessage is durable work recorded in the message insertion transaction.
type SupportLiveMessage struct {
	SourceHash     string `json:"-"`
	WorkspaceID    string
	ConversationID string
	MessageID      string `gorm:"primaryKey"`
	Enabled        bool
	Revision       int64
	TargetLanguage string
	Status         string
	Attempts       int
	UpdatedAt      time.Time
}

func (SupportLiveMessage) TableName() string { return "support_live_messages" }

// SupportPendingSend is private to its author and is never a widget projection.
type SupportPendingSend struct {
	TargetLanguage string    `json:"-"`
	RecipientEmail string    `json:"-"`
	RequestHash    string    `json:"-"`
	Attempts       int       `json:"-"`
	ID             string    `json:"id" gorm:"primaryKey"`
	WorkspaceID    string    `json:"-"`
	ConversationID string    `json:"conversation_id"`
	UserID         string    `json:"-"`
	Request        string    `json:"-"`
	Revision       int64     `json:"-"`
	Status         string    `json:"status"`
	Failure        string    `json:"failure,omitempty"`
	MessageID      string    `json:"message_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (SupportPendingSend) TableName() string { return "support_pending_sends" }
