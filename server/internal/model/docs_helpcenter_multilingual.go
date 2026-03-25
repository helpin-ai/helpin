package model

import (
	"encoding/json"
	"time"
)

const (
	DocsHelpcenterTranslationStatusDraft       = "draft"
	DocsHelpcenterTranslationStatusPublished   = "published"
	DocsHelpcenterTranslationStatusNeedsReview = "needs_review"
)

// DocsHelpcenterSpaceTranslation stores localized public-facing space metadata.
type DocsHelpcenterSpaceTranslation struct {
	ID              string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SpaceID         string     `json:"space_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_hc_space_locale,priority:1;index:idx_docs_hc_space_translation_space_locale,priority:1"`
	WorkspaceID     string     `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_hc_space_ws_locale_slug,priority:1;index:idx_docs_hc_space_translation_space_locale,priority:2"`
	Locale          string     `json:"locale" gorm:"not null;size:16;uniqueIndex:idx_docs_hc_space_locale,priority:2;uniqueIndex:idx_docs_hc_space_ws_locale_slug,priority:2;index:idx_docs_hc_space_translation_space_locale,priority:3"`
	Name            string     `json:"name" gorm:"not null"`
	Slug            string     `json:"slug" gorm:"not null;uniqueIndex:idx_docs_hc_space_ws_locale_slug,priority:3"`
	Description     *string    `json:"description"`
	Status          string     `json:"status" gorm:"not null;default:'draft'"`
	SourceUpdatedAt *time.Time `json:"source_updated_at"`
	SourceSynced    bool       `json:"source_synced" gorm:"not null;default:false"`
	PublishedAt     *time.Time `json:"published_at"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsHelpcenterSpaceTranslation) TableName() string {
	return "docs_helpcenter_space_translations"
}

// DocsHelpcenterCollectionTranslation stores localized public-facing collection metadata.
type DocsHelpcenterCollectionTranslation struct {
	ID              string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CollectionID    string     `json:"collection_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_hc_collection_locale,priority:1;index:idx_docs_hc_collection_translation_space_locale,priority:1"`
	WorkspaceID     string     `json:"workspace_id" gorm:"type:uuid;not null"`
	SpaceID         string     `json:"space_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_hc_collection_space_locale_slug,priority:1;index:idx_docs_hc_collection_translation_space_locale,priority:2"`
	Locale          string     `json:"locale" gorm:"not null;size:16;uniqueIndex:idx_docs_hc_collection_locale,priority:2;uniqueIndex:idx_docs_hc_collection_space_locale_slug,priority:2;index:idx_docs_hc_collection_translation_space_locale,priority:3"`
	Name            string     `json:"name" gorm:"not null"`
	Description     *string    `json:"description"`
	Slug            string     `json:"slug" gorm:"not null;uniqueIndex:idx_docs_hc_collection_space_locale_slug,priority:3"`
	Status          string     `json:"status" gorm:"not null;default:'draft'"`
	SourceUpdatedAt *time.Time `json:"source_updated_at"`
	SourceSynced    bool       `json:"source_synced" gorm:"not null;default:false"`
	PublishedAt     *time.Time `json:"published_at"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsHelpcenterCollectionTranslation) TableName() string {
	return "docs_helpcenter_collection_translations"
}

// DocsHelpcenterArticleTranslation stores localized public-facing article metadata and content.
type DocsHelpcenterArticleTranslation struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DocumentID      string          `json:"document_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_hc_article_locale,priority:1;index:idx_docs_hc_article_translation_space_locale,priority:1"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null"`
	SpaceID         string          `json:"space_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_hc_article_space_locale_slug,priority:1;index:idx_docs_hc_article_translation_space_locale,priority:2"`
	CollectionID    *string         `json:"collection_id" gorm:"type:uuid"`
	Locale          string          `json:"locale" gorm:"not null;size:16;uniqueIndex:idx_docs_hc_article_locale,priority:2;uniqueIndex:idx_docs_hc_article_space_locale_slug,priority:2;index:idx_docs_hc_article_translation_space_locale,priority:3"`
	Title           string          `json:"title" gorm:"not null"`
	Slug            string          `json:"slug" gorm:"not null;uniqueIndex:idx_docs_hc_article_space_locale_slug,priority:3"`
	Excerpt         *string         `json:"excerpt"`
	Content         json.RawMessage `json:"content" gorm:"type:jsonb"`
	ContentText     string          `json:"content_text" gorm:"type:text"`
	SEOTitle        *string         `json:"seo_title"`
	SEODescription  *string         `json:"seo_description"`
	Status          string          `json:"status" gorm:"not null;default:'draft'"`
	SourceUpdatedAt *time.Time      `json:"source_updated_at"`
	SourceSynced    bool            `json:"source_synced" gorm:"not null;default:false"`
	PublishedAt     *time.Time      `json:"published_at"`
	ViewCount       int             `json:"view_count" gorm:"not null;default:0"`
	HelpfulCount    int             `json:"helpful_count" gorm:"not null;default:0"`
	NotHelpfulCount int             `json:"not_helpful_count" gorm:"not null;default:0"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsHelpcenterArticleTranslation) TableName() string {
	return "docs_helpcenter_article_translations"
}

type UpdateDocsHelpcenterLocalesRequest struct {
	DefaultLocale           string   `json:"default_locale"`
	EnabledLocales          []string `json:"enabled_locales"`
	ShowLanguageSwitcher    bool     `json:"show_language_switcher"`
	FallbackToDefaultLocale bool     `json:"fallback_to_default_locale"`
}

type UpsertDocsHelpcenterSpaceTranslationRequest struct {
	Locale      string  `json:"locale"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
}

type UpsertDocsHelpcenterCollectionTranslationRequest struct {
	Locale      string  `json:"locale"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Slug        string  `json:"slug"`
	Status      string  `json:"status"`
}

type UpsertDocsHelpcenterArticleTranslationRequest struct {
	Locale         string          `json:"locale"`
	Title          string          `json:"title"`
	Slug           string          `json:"slug"`
	Excerpt        *string         `json:"excerpt"`
	Content        json.RawMessage `json:"content"`
	SEOTitle       *string         `json:"seo_title"`
	SEODescription *string         `json:"seo_description"`
	Status         string          `json:"status"`
}
