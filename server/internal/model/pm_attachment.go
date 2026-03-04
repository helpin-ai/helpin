package model

import "time"

// PMAttachment represents a file attachment on a story, epic, or comment.
type PMAttachment struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	EntityType   string    `json:"entity_type" gorm:"not null"` // "story" | "epic" | "comment"
	EntityID     string    `json:"entity_id" gorm:"type:uuid;not null;index"`
	FileName     string    `json:"file_name" gorm:"not null"`
	FileSize     int64     `json:"file_size" gorm:"not null"`
	ContentType  string    `json:"content_type" gorm:"not null"`
	StorageKey   string    `json:"storage_key" gorm:"not null"`
	IsUploaded   bool      `json:"is_uploaded" gorm:"default:false"`
	UploadedByID string    `json:"uploaded_by_id" gorm:"type:uuid;not null;index"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMAttachment) TableName() string { return "pm_attachments" }

// CreateAttachmentRequest is the payload for initiating an attachment upload.
type CreateAttachmentRequest struct {
	EntityType  string `json:"entity_type"`
	EntityID    string `json:"entity_id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

// AttachmentResponse is returned after creating or listing attachments.
type AttachmentResponse struct {
	Attachment PMAttachment `json:"attachment"`
	URL        string       `json:"url,omitempty"` // presigned URL (PUT for create, GET for list)
}
