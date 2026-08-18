package model

import "time"

// SupportAttachment represents a file attachment in a support conversation.
type SupportAttachment struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID *string   `json:"conversation_id" gorm:"type:uuid;index"`
	MessageID      *string   `json:"message_id" gorm:"type:uuid;index"`
	FileName       string    `json:"file_name" gorm:"not null"`
	FileSize       int64     `json:"file_size" gorm:"not null"`
	ContentType    string    `json:"content_type" gorm:"not null"`
	StorageKey     string    `json:"storage_key" gorm:"not null;default:''"`
	PublicURL      string    `json:"public_url" gorm:"not null;default:''"`
	IsUploaded     bool      `json:"is_uploaded" gorm:"default:false"`
	UploadedByType string    `json:"uploaded_by_type" gorm:"not null"` // "user" | "customer"
	UploadedByID   *string   `json:"uploaded_by_id" gorm:"type:uuid"`
	SessionID      *string   `json:"session_id" gorm:"type:uuid"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportAttachment) TableName() string { return "support_attachments" }

// CreateSupportAttachmentRequest is the payload for initiating a support attachment upload.
type CreateSupportAttachmentRequest struct {
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

// SupportAttachmentResponse is returned after creating a support attachment.
type SupportAttachmentResponse struct {
	Attachment SupportAttachment `json:"attachment"`
	UploadURL  string            `json:"upload_url,omitempty"`
	PublicURL  string            `json:"public_url,omitempty"`
}

// SupportAttachmentPayload is a compact attachment shape for message responses.
type SupportAttachmentPayload struct {
	ID       string `json:"id"`
	FileKey  string `json:"file_key"`
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
	FileSize int64  `json:"file_size"`
	URL      string `json:"url"`
}
