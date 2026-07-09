package model

import (
	"encoding/json"
	"time"
)

const (
	ContentSourceTypeWebsite = "website"
	ContentSourceTypeFile    = "file"

	ContentSourceFormatHTML     = "html"
	ContentSourceFormatMarkdown = "markdown"
	ContentSourceFormatJSON     = "json"

	ContentSourceDiscoveryAll      = "all"
	ContentSourceDiscoverySitemaps = "sitemaps"
	ContentSourceDiscoveryLinks    = "links"

	ContentSourcePurposeSearch  = "search"
	ContentSourcePurposeAIInput = "ai-input"
	ContentSourcePurposeAITrain = "ai-train"
)

// SupportContentSource stores a crawlable web content source for support AI.
type SupportContentSource struct {
	ID                   string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID          string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_content_sources_ws_name,priority:1"`
	Name                 string          `json:"name" gorm:"not null;index:idx_support_content_sources_ws_name,priority:2"`
	SourceType           string          `json:"source_type" gorm:"not null;default:'website';index"`
	StartURL             string          `json:"start_url" gorm:"type:text;not null"`
	FileName             *string         `json:"file_name,omitempty"`
	FileSize             int64           `json:"file_size" gorm:"not null;default:0"`
	ContentType          *string         `json:"content_type,omitempty"`
	StorageKey           *string         `json:"storage_key,omitempty"`
	CrawlLimit           int             `json:"crawl_limit" gorm:"not null;default:100"`
	CrawlDepth           int             `json:"crawl_depth" gorm:"not null;default:2"`
	CrawlSource          string          `json:"crawl_source" gorm:"not null;default:'all'"`
	Formats              DocsStringArray `json:"formats" gorm:"type:text[]"`
	Render               bool            `json:"render" gorm:"not null;default:true"`
	IncludeExternalLinks bool            `json:"include_external_links" gorm:"not null;default:false"`
	IncludeSubdomains    bool            `json:"include_subdomains" gorm:"not null;default:false"`
	IncludePatterns      DocsStringArray `json:"include_patterns" gorm:"type:text[]"`
	ExcludePatterns      DocsStringArray `json:"exclude_patterns" gorm:"type:text[]"`
	CrawlPurposes        DocsStringArray `json:"crawl_purposes" gorm:"type:text[]"`
	MaxAgeSeconds        int             `json:"max_age_seconds" gorm:"not null;default:86400"`
	ModifiedSince        *time.Time      `json:"modified_since"`
	JSONPrompt           *string         `json:"json_prompt"`
	JSONResponseFormat   json.RawMessage `json:"json_response_format" gorm:"type:jsonb"`
	SyncStatus           string          `json:"sync_status" gorm:"not null;default:'queued';index"`
	SyncProgress         int             `json:"sync_progress" gorm:"not null;default:0"`
	IndexedPages         int             `json:"indexed_pages" gorm:"not null;default:0"`
	IndexedChunks        int             `json:"indexed_chunks" gorm:"not null;default:0"`
	LastSyncError        *string         `json:"last_sync_error"`
	LastCrawlJobID       *string         `json:"last_crawl_job_id"`
	LastSyncStartedAt    *time.Time      `json:"last_sync_started_at" gorm:"type:timestamptz"`
	LastSyncCompletedAt  *time.Time      `json:"last_sync_completed_at" gorm:"type:timestamptz"`
	CreatedAt            time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportContentSource) TableName() string { return "support_content_sources" }

// AgentContentSource links a support agent to a content source for RAG retrieval.
type AgentContentSource struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AgentID         string    `json:"agent_id" gorm:"type:uuid;not null;uniqueIndex:idx_agent_content_source,priority:1"`
	ContentSourceID string    `json:"content_source_id" gorm:"type:uuid;not null;uniqueIndex:idx_agent_content_source,priority:2;index"`
	WorkspaceID     string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AgentContentSource) TableName() string { return "agent_content_sources" }

// SupportContentPage stores one crawled page from a content source.
type SupportContentPage struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_content_pages_ws_source,priority:1"`
	ContentSourceID string          `json:"content_source_id" gorm:"type:uuid;not null;index:idx_support_content_pages_ws_source,priority:2;uniqueIndex:idx_support_content_page_source_url,priority:1"`
	URL             string          `json:"url" gorm:"type:text;not null;uniqueIndex:idx_support_content_page_source_url,priority:2"`
	Title           string          `json:"title" gorm:"not null"`
	HTTPStatus      int             `json:"http_status" gorm:"not null;default:0"`
	ContentFormat   string          `json:"content_format" gorm:"not null;default:'markdown'"`
	ContentText     string          `json:"content_text" gorm:"type:text;not null"`
	ContentHash     string          `json:"content_hash" gorm:"size:64;not null;index"`
	ContentLength   int             `json:"content_length" gorm:"-"` // computed via SQL, not persisted
	Metadata        json.RawMessage `json:"metadata" gorm:"type:jsonb"`
	LastCrawledAt   time.Time       `json:"last_crawled_at" gorm:"not null"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportContentPage) TableName() string { return "support_content_pages" }

// SupportContentChunk stores chunked crawled content and its vector embedding.
type SupportContentChunk struct {
	ID                  string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string    `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_content_chunk_ws_source_page,priority:1"`
	ContentSourceID     string    `json:"content_source_id" gorm:"type:uuid;not null;index:idx_support_content_chunk_ws_source_page,priority:2;index"`
	PageID              string    `json:"page_id" gorm:"type:uuid;not null;index:idx_support_content_chunk_ws_source_page,priority:3;uniqueIndex:idx_support_content_chunk_page_order,priority:1"`
	ChunkIndex          int       `json:"chunk_index" gorm:"not null;uniqueIndex:idx_support_content_chunk_page_order,priority:2"`
	Title               string    `json:"title" gorm:"not null"`
	URL                 string    `json:"url" gorm:"type:text;not null"`
	Content             string    `json:"content" gorm:"type:text;not null"`
	ContentHash         string    `json:"content_hash" gorm:"size:64;not null;index"`
	Embedding           string    `json:"-" gorm:"type:vector(1536);not null"`
	EmbeddingProvider   string    `json:"embedding_provider" gorm:"not null;default:'openai'"`
	EmbeddingModel      string    `json:"embedding_model" gorm:"not null;default:'text-embedding-3-small'"`
	EmbeddingVersion    string    `json:"embedding_version" gorm:"not null;default:'content-chunk-v1'"`
	EmbeddingDimensions int       `json:"embedding_dimensions" gorm:"not null;default:1536"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportContentChunk) TableName() string { return "support_content_chunks" }

type CreateSupportContentSourceRequest struct {
	Name                 string          `json:"name"`
	StartURL             string          `json:"start_url"`
	CrawlLimit           int             `json:"crawl_limit"`
	CrawlDepth           int             `json:"crawl_depth"`
	CrawlSource          string          `json:"crawl_source"`
	Formats              []string        `json:"formats"`
	Render               *bool           `json:"render"`
	IncludeExternalLinks *bool           `json:"include_external_links"`
	IncludeSubdomains    *bool           `json:"include_subdomains"`
	IncludePatterns      []string        `json:"include_patterns"`
	ExcludePatterns      []string        `json:"exclude_patterns"`
	CrawlPurposes        []string        `json:"crawl_purposes"`
	MaxAgeSeconds        int             `json:"max_age_seconds"`
	ModifiedSince        *time.Time      `json:"modified_since"`
	JSONPrompt           *string         `json:"json_prompt"`
	JSONResponseFormat   json.RawMessage `json:"json_response_format"`
}

type CreateSupportContentSourceFileUploadRequest struct {
	Name        string `json:"name"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	StorageKey  string `json:"-"`
}

type CreateSupportContentSourceFileUploadResponse struct {
	Source    SupportContentSource `json:"source"`
	UploadURL string               `json:"upload_url"`
}

type UpdateSupportContentSourceRequest struct {
	Name                 *string         `json:"name"`
	StartURL             *string         `json:"start_url"`
	CrawlLimit           *int            `json:"crawl_limit"`
	CrawlDepth           *int            `json:"crawl_depth"`
	CrawlSource          *string         `json:"crawl_source"`
	Formats              []string        `json:"formats"`
	Render               *bool           `json:"render"`
	IncludeExternalLinks *bool           `json:"include_external_links"`
	IncludeSubdomains    *bool           `json:"include_subdomains"`
	IncludePatterns      []string        `json:"include_patterns"`
	ExcludePatterns      []string        `json:"exclude_patterns"`
	CrawlPurposes        []string        `json:"crawl_purposes"`
	MaxAgeSeconds        *int            `json:"max_age_seconds"`
	ModifiedSince        *time.Time      `json:"modified_since"`
	JSONPrompt           *string         `json:"json_prompt"`
	JSONResponseFormat   json.RawMessage `json:"json_response_format"`
}

type UpdateAgentContentSourcesRequest struct {
	ContentSourceIDs []string `json:"content_source_ids"`
}
