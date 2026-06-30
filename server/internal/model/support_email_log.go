package model

import "time"

// SupportEmailLog records inbound and outbound email activity for support conversations.
type SupportEmailLog struct {
	ID                 string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_sel_workspace"`
	ConversationID     string          `json:"conversation_id" gorm:"type:uuid;not null;index:idx_sel_conversation"`
	EmailRouteID       *string         `json:"email_route_id,omitempty" gorm:"type:uuid;index"`
	Direction          string          `json:"direction" gorm:"size:10;not null"`
	MessageIDs         DocsStringArray `json:"message_ids" gorm:"type:text[]"`
	FromEmail          string          `json:"from_email" gorm:"size:255"`
	FromDisplayName    string          `json:"from_display_name,omitempty" gorm:"size:255"`
	FromSource         string          `json:"from_source,omitempty" gorm:"size:80"`
	FromFallbackReason string          `json:"from_fallback_reason,omitempty" gorm:"size:120"`
	ToEmail            string          `json:"to_email" gorm:"size:255"`
	ReplyTo            string          `json:"reply_to,omitempty" gorm:"size:255"`
	RecipientAddress   string          `json:"recipient_address,omitempty" gorm:"size:255"`
	CCEmails           DocsStringArray `json:"cc_emails,omitempty" gorm:"type:text[]"`
	BCCEmails          DocsStringArray `json:"bcc_emails,omitempty" gorm:"type:text[]"`
	Subject            string          `json:"subject" gorm:"size:500"`
	RFCMessageID       string          `json:"rfc_message_id" gorm:"size:255"`
	InReplyTo          string          `json:"in_reply_to" gorm:"size:255"`
	ReferencesHeader   string          `json:"references_header,omitempty" gorm:"type:text"`
	PostmarkMessageID  *string         `json:"postmark_message_id,omitempty" gorm:"size:255"`
	RawBody            string          `json:"-" gorm:"type:text"`
	StrippedText       string          `json:"stripped_text,omitempty" gorm:"type:text"`
	HTMLBody           string          `json:"html_body,omitempty" gorm:"type:text"`
	Status             string          `json:"status" gorm:"size:20;not null;default:'sent'"`
	DeliveredAt        *time.Time      `json:"delivered_at,omitempty" gorm:"index"`
	OpenedAt           *time.Time      `json:"opened_at,omitempty"`
	BouncedAt          *time.Time      `json:"bounced_at,omitempty"`
	ErrorMessage       string          `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt          time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportEmailLog) TableName() string { return "support_email_logs" }

// SupportMessageEmailDetail is the API response returned for a single message's email details.
type SupportMessageEmailDetail struct {
	ID                   string                       `json:"id"`
	MessageID            string                       `json:"message_id"`
	Direction            string                       `json:"direction"`
	Subject              string                       `json:"subject"`
	FromEmail            string                       `json:"from_email"`
	ToEmail              string                       `json:"to_email"`
	ReplyTo              string                       `json:"reply_to,omitempty"`
	CCEmails             DocsStringArray              `json:"cc_emails,omitempty"`
	BCCEmails            DocsStringArray              `json:"bcc_emails,omitempty"`
	RFCMessageID         string                       `json:"rfc_message_id,omitempty"`
	InReplyTo            string                       `json:"in_reply_to,omitempty"`
	ReferencesHeader     string                       `json:"references_header,omitempty"`
	StrippedText         string                       `json:"stripped_text,omitempty"`
	HTMLBody             string                       `json:"html_body,omitempty"`
	Status               string                       `json:"status"`
	DeliveredAt          *time.Time                   `json:"delivered_at,omitempty"`
	OpenedAt             *time.Time                   `json:"opened_at,omitempty"`
	BouncedAt            *time.Time                   `json:"bounced_at,omitempty"`
	ErrorMessage         string                       `json:"error_message,omitempty"`
	CreatedAt            time.Time                    `json:"created_at"`
	ForwardedAttribution *SupportForwardedAttribution `json:"forwarded_attribution,omitempty"`
}

// SupportForwardedAttribution describes an inbound email whose customer identity
// was inferred from a forwarded-message block while preserving the real forwarder.
type SupportForwardedAttribution struct {
	OriginalSenderEmail string `json:"original_sender_email"`
	OriginalSenderName  string `json:"original_sender_name,omitempty"`
	ForwardedByEmail    string `json:"forwarded_by_email"`
	ForwardedByName     string `json:"forwarded_by_name,omitempty"`
	Confidence          int    `json:"confidence"`
	ConfidenceLevel     string `json:"confidence_level"`
	Source              string `json:"source"`
}
