package model

import "time"

const (
	DocsHelpcenterSearchEntryTypeTitle   = "title"
	DocsHelpcenterSearchEntryTypeExcerpt = "excerpt"
	DocsHelpcenterSearchEntryTypeHeading = "heading"
	DocsHelpcenterSearchEntryTypeBody    = "body"
)

// DocsHelpcenterSearchEntry is a section-level public help-center search record.
type DocsHelpcenterSearchEntry struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DocumentID   string    `json:"document_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_hc_search_doc_locale_key,priority:1;index"`
	Locale       string    `json:"locale" gorm:"not null;size:16;uniqueIndex:idx_docs_hc_search_doc_locale_key,priority:2;index"`
	EntryKey     string    `json:"entry_key" gorm:"not null;uniqueIndex:idx_docs_hc_search_doc_locale_key,priority:3"`
	EntryType    string    `json:"entry_type" gorm:"not null;size:16;index"`
	Content      string    `json:"content" gorm:"type:text;not null"`
	SectionTitle *string   `json:"section_title,omitempty" gorm:"type:text"`
	Anchor       *string   `json:"anchor,omitempty" gorm:"type:text"`
	Position     int       `json:"position" gorm:"not null;default:0"`
	RankWeight   float64   `json:"rank_weight" gorm:"not null;default:1"`
	SearchConfig string    `json:"search_config" gorm:"not null;size:32;default:simple"`
	SearchVector string    `json:"-" gorm:"type:tsvector;not null;default:''"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsHelpcenterSearchEntry) TableName() string {
	return "docs_helpcenter_search_entries"
}

type PublicSearchMatchResponse struct {
	EntryType    string  `json:"entry_type"`
	SectionTitle *string `json:"section_title,omitempty"`
	Anchor       *string `json:"anchor,omitempty"`
	Snippet      string  `json:"snippet"`
}
