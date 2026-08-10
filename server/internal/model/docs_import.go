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
	ID               string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_docs_import_jobs_ws"`
	SpaceID          *string         `json:"space_id" gorm:"type:uuid"`
	Source           string          `json:"source" gorm:"not null;default:'helpscout'"`
	Status           string          `json:"status" gorm:"not null;default:'pending'"`
	Total            int             `json:"total" gorm:"not null;default:0"`
	Completed        int             `json:"completed" gorm:"not null;default:0"`
	Failed           int             `json:"failed" gorm:"not null;default:0"`
	Failures         json.RawMessage `json:"failures" gorm:"type:jsonb;not null;default:'[]'"`
	Config           json.RawMessage `json:"config" gorm:"type:jsonb;not null;default:'{}'"`
	RedirectMap      json.RawMessage `json:"redirect_map" gorm:"type:jsonb"`
	Summary          json.RawMessage `json:"summary" gorm:"type:jsonb"`
	Error            *string         `json:"error"`
	PayloadEncrypted *string         `json:"-" gorm:"type:text"`
	WorkflowID       *string         `json:"workflow_id,omitempty" gorm:"index"`
	StartedBy        string          `json:"started_by" gorm:"type:uuid;not null"`
	StartedAt        *time.Time      `json:"started_at"`
	CompletedAt      *time.Time      `json:"completed_at"`
	CreatedAt        time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the database table name for DocsImportJob.
func (DocsImportJob) TableName() string { return "docs_import_jobs" }

// ImportFailure records a single article that failed during import.
type ImportFailure struct {
	ArticleID string `json:"article_id"`
	Title     string `json:"title"`
	Error     string `json:"error"`
}

// ImportSummary holds high-level stats shown to the user when an import completes.
type ImportSummary struct {
	CollectionsCreated             int    `json:"collections_created"`
	ArticlesPublished              int    `json:"articles_published"`
	ArticlesDrafted                int    `json:"articles_drafted"`
	RedirectsCreated               int    `json:"redirects_created"`
	ArticlesUncategorized          int    `json:"articles_uncategorized"`
	ArticlesWithConversionWarnings int    `json:"articles_with_conversion_warnings"`
	HTMLBlockFallbacks             int    `json:"html_block_fallbacks"`
	ImageRewriteFailures           int    `json:"image_rewrite_failures"`
	NormalizedNoteBlocks           int    `json:"normalized_note_blocks"`
	SourceSystem                   string `json:"source_system,omitempty"`
	UnsupportedComponents          int    `json:"unsupported_components,omitempty"`
	BrokenLinks                    int    `json:"broken_links,omitempty"`
	AssetRewriteFailures           int    `json:"asset_rewrite_failures,omitempty"`
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

// --- Nextra Import DTOs ---

// DocsImportJobConfig stores source-agnostic configuration for an
// import job. It is serialised into the job's Config JSON column.
type DocsImportJobConfig struct {
	// HelpScout fields (existing, kept for backward compatibility).
	APIKey                string `json:"api_key,omitempty"`
	HelpscoutCollectionID string `json:"helpscout_collection_id,omitempty"`
	TargetSpaceID         string `json:"target_space_id,omitempty"`
	NewSpaceName          string `json:"new_space_name,omitempty"`
	ImportStatus          string `json:"import_status,omitempty"`

	// Source-agnostic fields.
	SourceSystem string `json:"source_system,omitempty"`
	ArchiveKey   string `json:"archive_key,omitempty"`
	ArchiveName  string `json:"archive_name,omitempty"`
	DetectedRoot string `json:"detected_root,omitempty"`
	SourceCommit string `json:"source_commit,omitempty"`
}

// DocsNextraImportPreviewResponse is returned after previewing a
// Nextra archive upload.
type DocsNextraImportPreviewResponse struct {
	JobID                 string                                   `json:"job_id"`
	ArchiveName           string                                   `json:"archive_name"`
	SourceCommit          string                                   `json:"source_commit,omitempty"`
	DetectedRoot          string                                   `json:"detected_root"`
	Spaces                []DocsNextraImportSpacePreview           `json:"spaces"`
	Collections           int                                      `json:"collections"`
	Articles              int                                      `json:"articles"`
	Assets                int                                      `json:"assets"`
	Redirects             int                                      `json:"redirects"`
	Warnings              []DocsImportWarningResponse              `json:"warnings"`
	BrokenLinks           []DocsImportBrokenLinkResponse           `json:"broken_links"`
	UnsupportedComponents []DocsImportUnsupportedComponentResponse `json:"unsupported_components"`
}

// DocsNextraImportSpacePreview summarises one target space in a
// Nextra import preview.
type DocsNextraImportSpacePreview struct {
	SourceID        string `json:"source_id"`
	Name            string `json:"name"`
	CollectionCount int    `json:"collection_count"`
	ArticleCount    int    `json:"article_count"`
}

// DocsNextraImportStartRequest is the payload for starting a
// previously previewed Nextra import.
type DocsNextraImportStartRequest struct {
	JobID           string  `json:"job_id"`
	TargetSpaceID   *string `json:"target_space_id"`
	NewSpaceName    *string `json:"new_space_name"`
	ImportStatus    string  `json:"import_status"`
	CreateRedirects bool    `json:"create_redirects"`
}

// DocsImportWarningResponse represents one warning in an import
// preview or report.
type DocsImportWarningResponse struct {
	Type       string `json:"type"`
	SourcePath string `json:"source_path,omitempty"`
	Message    string `json:"message"`
}

// DocsImportBrokenLinkResponse represents an internal link that could
// not be resolved.
type DocsImportBrokenLinkResponse struct {
	SourcePath string `json:"source_path"`
	Target     string `json:"target"`
	Message    string `json:"message"`
}

// DocsImportUnsupportedComponentResponse tallies how many times an
// unknown MDX component appeared across all articles.
type DocsImportUnsupportedComponentResponse struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
