package model

import (
	"encoding/json"
	"time"
)

const (
	// DocsAPIReferenceSourceURL identifies a remotely hosted OpenAPI document.
	DocsAPIReferenceSourceURL = "url"
	// DocsAPIReferenceSourceUpload identifies an OpenAPI document uploaded through Helpin.
	DocsAPIReferenceSourceUpload = "upload"

	// DocsAPIReferenceSyncReady indicates the most recent source import succeeded.
	DocsAPIReferenceSyncReady = "ready"
	// DocsAPIReferenceSyncFailed indicates the most recent source import failed.
	DocsAPIReferenceSyncFailed = "failed"
)

// DocsAPIReference stores the publishing and source configuration for an OpenAPI reference.
type DocsAPIReference struct {
	ID                  string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string     `json:"workspace_id" gorm:"type:uuid;not null;index:idx_docs_api_reference_workspace"`
	SpaceID             string     `json:"space_id" gorm:"type:uuid;not null;index:idx_docs_api_reference_space_position,priority:1;uniqueIndex:idx_docs_api_reference_space_slug,priority:1"`
	Name                string     `json:"name" gorm:"not null"`
	Slug                string     `json:"slug" gorm:"not null;uniqueIndex:idx_docs_api_reference_space_slug,priority:2"`
	SourceType          string     `json:"source_type" gorm:"not null"`
	SourceURL           *string    `json:"source_url,omitempty"`
	SyncEnabled         bool       `json:"sync_enabled" gorm:"not null;default:false"`
	SyncStatus          string     `json:"sync_status" gorm:"not null;default:'ready'"`
	LastSyncError       *string    `json:"last_sync_error,omitempty" gorm:"type:text"`
	LastSyncedAt        *time.Time `json:"last_synced_at,omitempty"`
	DraftRevisionID     *string    `json:"draft_revision_id,omitempty" gorm:"type:uuid"`
	PublishedRevisionID *string    `json:"published_revision_id,omitempty" gorm:"type:uuid"`
	PublishedAt         *time.Time `json:"published_at,omitempty"`
	Position            int        `json:"position" gorm:"not null;default:0;index:idx_docs_api_reference_space_position,priority:2"`
	CreatedBy           string     `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the API reference table name.
func (DocsAPIReference) TableName() string { return "docs_api_references" }

// DocsAPIReferenceRevision is an immutable validated OpenAPI snapshot.
type DocsAPIReferenceRevision struct {
	ID             string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	APIReferenceID string          `json:"api_reference_id" gorm:"type:uuid;not null;index:idx_docs_api_reference_revision_ref_created,priority:1"`
	WorkspaceID    string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SourceHash     string          `json:"source_hash" gorm:"not null"`
	OpenAPIVersion string          `json:"openapi_version" gorm:"column:openapi_version;not null"`
	APIVersion     string          `json:"api_version" gorm:"not null;default:''"`
	Specification  json.RawMessage `json:"specification" gorm:"type:jsonb;not null"`
	Warnings       DocsStringArray `json:"warnings" gorm:"type:text[]"`
	OperationCount int             `json:"operation_count" gorm:"not null;default:0"`
	SchemaCount    int             `json:"schema_count" gorm:"not null;default:0"`
	CreatedBy      string          `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt      time.Time       `json:"created_at" gorm:"autoCreateTime;index:idx_docs_api_reference_revision_ref_created,priority:2"`
}

// TableName returns the API reference revision table name.
func (DocsAPIReferenceRevision) TableName() string { return "docs_api_reference_revisions" }

// DocsAPIReferenceResponse includes revision summaries for management screens.
type DocsAPIReferenceResponse struct {
	DocsAPIReference
	DraftRevision     *DocsAPIReferenceRevision `json:"draft_revision,omitempty"`
	PublishedRevision *DocsAPIReferenceRevision `json:"published_revision,omitempty"`
}

// CreateDocsAPIReferenceRequest creates an API reference and its first draft revision.
type CreateDocsAPIReferenceRequest struct {
	Name              string `json:"name"`
	Slug              string `json:"slug"`
	SourceType        string `json:"source_type"`
	SourceURL         string `json:"source_url"`
	SpecificationText string `json:"specification_text"`
	SyncEnabled       bool   `json:"sync_enabled"`
}

// UpdateDocsAPIReferenceRequest updates metadata and may create a new draft revision.
type UpdateDocsAPIReferenceRequest struct {
	Name              *string `json:"name"`
	Slug              *string `json:"slug"`
	SourceURL         *string `json:"source_url"`
	SpecificationText *string `json:"specification_text"`
	SyncEnabled       *bool   `json:"sync_enabled"`
}

// PublicDocsAPIReferenceSummary is the public navigation representation of a reference.
type PublicDocsAPIReferenceSummary struct {
	ID             string `json:"id"`
	SpaceID        string `json:"space_id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	APIVersion     string `json:"api_version"`
	OperationCount int    `json:"operation_count"`
}

// PublicDocsAPIReferenceResponse contains the published OpenAPI snapshot.
type PublicDocsAPIReferenceResponse struct {
	PublicDocsAPIReferenceSummary
	OpenAPIVersion string          `json:"openapi_version"`
	Specification  json.RawMessage `json:"specification"`
	PublishedAt    *time.Time      `json:"published_at,omitempty"`
}
