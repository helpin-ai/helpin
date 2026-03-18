package model

import (
	"encoding/json"
	"time"
)

// Docs import job status values.
const (
	DocsImportStatusPending     = "pending"
	DocsImportStatusRunning     = "running"
	DocsImportStatusDone        = "done"
	DocsImportStatusFailed      = "failed"
	DocsImportStatusInterrupted = "interrupted"
)

// DocsImportJob tracks a docs import operation from an external source.
type DocsImportJob struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_docs_import_jobs_ws"`
	SpaceID     *string         `json:"space_id" gorm:"type:uuid"`
	Source      string          `json:"source" gorm:"not null;default:'helpscout'"`
	Status      string          `json:"status" gorm:"not null;default:'pending'"`
	Total       int             `json:"total" gorm:"not null;default:0"`
	Completed   int             `json:"completed" gorm:"not null;default:0"`
	Failed      int             `json:"failed" gorm:"not null;default:0"`
	Failures    json.RawMessage `json:"failures" gorm:"type:jsonb;not null;default:'[]'"`
	Config      json.RawMessage `json:"config" gorm:"type:jsonb;not null;default:'{}'"`
	RedirectMap json.RawMessage `json:"redirect_map" gorm:"type:jsonb"`
	Error       *string         `json:"error"`
	StartedBy   string          `json:"started_by" gorm:"type:uuid;not null"`
	StartedAt   *time.Time      `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at"`
	CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the database table name for DocsImportJob.
func (DocsImportJob) TableName() string { return "docs_import_jobs" }

// ImportFailure records a single article that failed during import.
type ImportFailure struct {
	ArticleID string `json:"article_id"`
	Title     string `json:"title"`
	Error     string `json:"error"`
}

// DocsImportPreviewRequest is the payload for previewing available collections from an external source.
type DocsImportPreviewRequest struct {
	APIKey string `json:"api_key"`
}

// DocsImportStartRequest is the payload for starting a docs import job.
type DocsImportStartRequest struct {
	APIKey                string  `json:"api_key"`
	HelpscoutCollectionID string  `json:"helpscout_collection_id"`
	TargetSpaceID         *string `json:"target_space_id"`
	NewSpaceName          *string `json:"new_space_name"`
	ImportStatus          string  `json:"import_status"`
}

// DocsImportPreviewResponse is the response for a docs import preview.
type DocsImportPreviewResponse struct {
	Collections []HelpscoutCollectionPreview `json:"collections"`
}

// HelpscoutCollectionPreview represents a single HelpScout collection in a preview listing.
type HelpscoutCollectionPreview struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	CategoryCount int    `json:"category_count"`
	ArticleCount  int    `json:"article_count"`
}
