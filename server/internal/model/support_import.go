package model

import (
	"encoding/json"
	"time"
)

// Support import job states.
const (
	SupportImportStatusPending   = "pending"
	SupportImportStatusRunning   = "running"
	SupportImportStatusDone      = "done"
	SupportImportStatusFailed    = "failed"
	SupportImportStatusCancelled = "cancelled"
)

// SupportImportSourceIntercom imports conversations from Intercom.
const SupportImportSourceIntercom = "intercom"

// SupportImportFailureLimit bounds the failures a job keeps.
const SupportImportFailureLimit = 100

// SupportImportJob imports conversations from another help desk. A worker
// processes it page by page from Cursor, so it resumes after restarts.
type SupportImportJob struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Source      string    `json:"source" gorm:"not null"`
	Status      string    `json:"status" gorm:"not null;index"`
	Since       time.Time `json:"since" gorm:"type:timestamptz;not null"`
	// MailboxID files imported conversations in a team inbox; nil is the
	// main inbox.
	MailboxID *string `json:"mailbox_id,omitempty" gorm:"type:uuid"`
	Total     int     `json:"total" gorm:"type:integer;not null;default:0"`
	Imported  int     `json:"imported" gorm:"type:integer;not null;default:0"`
	Skipped   int     `json:"skipped" gorm:"type:integer;not null;default:0"`
	Failed    int     `json:"failed" gorm:"type:integer;not null;default:0"`
	// Failures holds up to SupportImportFailureLimit SupportImportFailure.
	Failures         json.RawMessage `json:"failures" gorm:"type:jsonb;not null;default:'[]'"`
	Cursor           string          `json:"-" gorm:"not null;default:''"`
	Error            string          `json:"error" gorm:"not null;default:''"`
	PayloadEncrypted string          `json:"-" gorm:"not null"`
	LockedUntil      *time.Time      `json:"-" gorm:"type:timestamptz"`
	// LeaseToken names the worker holding the lease; a worker writes progress
	// only while it still holds it.
	LeaseToken string `json:"-" gorm:"not null;default:''"`
	// Attempts counts failed tries of the current page in a row.
	Attempts    int        `json:"-" gorm:"type:integer;not null;default:0"`
	StartedBy   string     `json:"started_by" gorm:"type:uuid;not null"`
	CompletedAt *time.Time `json:"completed_at,omitempty" gorm:"type:timestamptz"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportImportJob) TableName() string { return "support_import_jobs" }

// SupportImportFailure is one conversation that could not be imported.
type SupportImportFailure struct {
	ExternalID string `json:"external_id"`
	Error      string `json:"error"`
}

// SupportImportRecord maps an imported conversation to its source, so a
// conversation is imported once however often an import runs.
type SupportImportRecord struct {
	WorkspaceID    string `json:"workspace_id" gorm:"type:uuid;primaryKey"`
	Source         string `json:"source" gorm:"primaryKey"`
	ExternalID     string `json:"external_id" gorm:"primaryKey"`
	ConversationID string `json:"conversation_id" gorm:"type:uuid;not null"`
	// JobID is the import that created the record.
	JobID     *string   `json:"job_id,omitempty" gorm:"type:uuid"`
	CreatedAt time.Time `json:"created_at" gorm:"type:timestamptz;not null"`
}

func (SupportImportRecord) TableName() string { return "support_import_records" }

// IntercomConversationImportRequest previews or starts an Intercom
// conversation import.
type IntercomConversationImportRequest struct {
	AccessToken string `json:"access_token"`
	Region      string `json:"region"`
	// Since is a date (YYYY-MM-DD); conversations created on or after it
	// are imported.
	Since     string  `json:"since"`
	MailboxID *string `json:"mailbox_id"`
}

// IntercomConversationImportPreview counts conversations to import.
type IntercomConversationImportPreview struct {
	WorkspaceName string `json:"workspace_name"`
	Conversations int    `json:"conversations"`
}
