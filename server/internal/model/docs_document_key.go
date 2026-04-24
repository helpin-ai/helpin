package model

import "time"

const (
	DocsDocumentKeyTypeReleaseNotes = "release_notes"
)

type DocsDocumentKey struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	KeyType     string    `json:"key_type" gorm:"not null;default:'release_notes'"`
	Key         string    `json:"key" gorm:"not null"`
	DocumentID  *string   `json:"document_id,omitempty" gorm:"type:uuid;index"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsDocumentKey) TableName() string { return "docs_document_keys" }
