package model

import (
	"encoding/json"
	"time"
)

// DocsHelpcenterArticlePublication stores the latest live public snapshot for an article locale.
// Draft/source editing continues in the existing docs and translation tables; public reads use this snapshot.
type DocsHelpcenterArticlePublication struct {
	ID             string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DocumentID     string          `json:"document_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_hc_article_pub_doc_locale,priority:1;index:idx_docs_hc_article_pub_space_locale_slug,priority:1;index:idx_docs_hc_article_pub_collection_locale,priority:1"`
	WorkspaceID    string          `json:"workspace_id" gorm:"type:uuid;not null"`
	SpaceID        string          `json:"space_id" gorm:"type:uuid;not null;index:idx_docs_hc_article_pub_space_locale_slug,priority:2"`
	CollectionID   *string         `json:"collection_id" gorm:"type:uuid;index:idx_docs_hc_article_pub_collection_locale,priority:2"`
	Locale         string          `json:"locale" gorm:"not null;size:16;uniqueIndex:idx_docs_hc_article_pub_doc_locale,priority:2;index:idx_docs_hc_article_pub_space_locale_slug,priority:3;index:idx_docs_hc_article_pub_collection_locale,priority:3"`
	Title          string          `json:"title" gorm:"not null"`
	Slug           string          `json:"slug" gorm:"not null;uniqueIndex:idx_docs_hc_article_pub_space_locale_slug,priority:4"`
	Excerpt        *string         `json:"excerpt"`
	Content        json.RawMessage `json:"content" gorm:"type:jsonb"`
	ContentText    string          `json:"content_text" gorm:"type:text"`
	SEOTitle       *string         `json:"seo_title"`
	SEODescription *string         `json:"seo_description"`
	PublishedAt    time.Time       `json:"published_at" gorm:"not null;index"`
	CreatedAt      time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsHelpcenterArticlePublication) TableName() string {
	return "docs_helpcenter_article_publications"
}
