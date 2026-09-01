package model

import "time"

// CRMEmailAttachment is a private, outgoing attachment staged for a CRM email draft.
type CRMEmailAttachment struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DraftID      string    `json:"draft_id" gorm:"type:uuid;not null;index"`
	MessageID    *string   `json:"message_id,omitempty" gorm:"type:uuid;index"`
	UploadedByID string    `json:"uploaded_by_id" gorm:"type:uuid;not null;index"`
	FileName     string    `json:"file_name" gorm:"not null"`
	FileSize     int64     `json:"file_size" gorm:"not null"`
	ContentType  string    `json:"content_type" gorm:"not null"`
	StorageKey   string    `json:"-" gorm:"not null;default:''"`
	IsUploaded   bool      `json:"is_uploaded" gorm:"default:false"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

func (CRMEmailAttachment) TableName() string { return "crm_email_attachments" }

type CreateCRMEmailAttachmentRequest struct {
	DraftID     string `json:"draft_id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

type CRMEmailAttachmentUploadResponse struct {
	Attachment CRMEmailAttachment `json:"attachment"`
	UploadURL  string             `json:"upload_url"`
}
