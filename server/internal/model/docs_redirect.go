package model

import "time"

const (
	RedirectTypeImported   = "imported"
	RedirectTypeSlugChange = "slug_change"
	RedirectTypeManual     = "manual"
)

// DocsRedirect stores URL redirect rules for the public help center.
type DocsRedirect struct {
	ID                   string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID          string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_redirects_ws_source,priority:1;index:idx_docs_redirects_ws"`
	SourcePath           string    `json:"source_path" gorm:"not null;uniqueIndex:idx_docs_redirects_ws_source,priority:2"`
	TargetCollectionSlug string    `json:"target_collection_slug" gorm:"not null"`
	TargetArticleSlug    *string   `json:"target_article_slug"`
	Type                 string    `json:"type" gorm:"not null;default:'manual'"`
	SourceSystem         *string   `json:"source_system"`
	SourceObjectType     *string   `json:"source_object_type"`
	SourceObjectID       *string   `json:"source_object_id"`
	CreatedAt            time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (DocsRedirect) TableName() string { return "docs_redirects" }

// CreateDocsRedirectRequest is the payload for creating a manual redirect.
type CreateDocsRedirectRequest struct {
	SourcePath           string  `json:"source_path"`
	TargetCollectionSlug string  `json:"target_collection_slug"`
	TargetArticleSlug    *string `json:"target_article_slug"`
}

// UpdateDocsRedirectRequest is the payload for updating a redirect.
type UpdateDocsRedirectRequest struct {
	SourcePath           *string `json:"source_path"`
	TargetCollectionSlug *string `json:"target_collection_slug"`
	TargetArticleSlug    *string `json:"target_article_slug"`
}

// DocsRedirectFilter controls list query filtering and pagination.
type DocsRedirectFilter struct {
	Search  string `json:"search"`
	Type    string `json:"type"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

// DocsRedirectListResponse is the paginated response for listing redirects.
type DocsRedirectListResponse struct {
	Items []DocsRedirect `json:"items"`
	Total int64          `json:"total"`
}
