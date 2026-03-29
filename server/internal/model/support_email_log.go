package model

import "time"

// SupportEmailLog records inbound and outbound email activity for support conversations.
type SupportEmailLog struct {
	ID                string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_sel_workspace"`
	ConversationID    string          `json:"conversation_id" gorm:"type:uuid;not null;index:idx_sel_conversation"`
	EmailRouteID      *string         `json:"email_route_id,omitempty" gorm:"type:uuid;index"`
	Direction         string          `json:"direction" gorm:"size:10;not null"`
	MessageIDs        DocsStringArray `json:"message_ids" gorm:"type:text[]"`
	FromEmail         string          `json:"from_email" gorm:"size:255"`
	ToEmail           string          `json:"to_email" gorm:"size:255"`
	RecipientAddress  string          `json:"recipient_address,omitempty" gorm:"size:255"`
	Subject           string          `json:"subject" gorm:"size:500"`
	RFCMessageID      string          `json:"rfc_message_id" gorm:"size:255"`
	InReplyTo         string          `json:"in_reply_to" gorm:"size:255"`
	ReferencesHeader  string          `json:"references_header,omitempty" gorm:"type:text"`
	PostmarkMessageID *string         `json:"postmark_message_id,omitempty" gorm:"size:255"`
	RawBody           string          `json:"-" gorm:"type:text"`
	StrippedText      string          `json:"stripped_text,omitempty" gorm:"type:text"`
	Status            string          `json:"status" gorm:"size:20;not null;default:'sent'"`
	OpenedAt          *time.Time      `json:"opened_at,omitempty"`
	ErrorMessage      string          `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt         time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportEmailLog) TableName() string { return "support_email_logs" }
