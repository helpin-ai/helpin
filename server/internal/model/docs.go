package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ─── Doc status constants ───────────────────────────────────────────────────

// Document status values.
const (
	DocStatusDraft     = "draft"
	DocStatusPublished = "published"
	DocStatusArchived  = "archived"
)

// Space type values.
const (
	SpaceTypeInternal        = "internal"
	SpaceTypeExternalCapable = "external_capable"
)

// Space visibility values.
const (
	SpaceVisibilityWorkspaceWide = "workspace_wide"
	SpaceVisibilityTeamOnly      = "team_only"
)

// Help center public URL modes.
const (
	HelpcenterPublicURLModeHostedSubdomain = "hosted_subdomain"
	HelpcenterPublicURLModeCustomDomain    = "custom_domain"
	HelpcenterPublicURLModeReverseProxy    = "reverse_proxy"
)

// Version type values.
const (
	VersionTypeManual  = "manual"
	VersionTypeAuto    = "auto"
	VersionTypePublish = "publish"
	VersionTypeRevert  = "revert"
)

// Link context values.
const (
	LinkContextAttached        = "attached"
	LinkContextMentioned       = "mentioned"
	LinkContextCreatedFrom     = "created_from"
	LinkContextLinkedInContent = "linked_in_content"
)

// Linked object type values.
const (
	LinkedObjectEpic                = "epic"
	LinkedObjectTask                = "task"
	LinkedObjectProject             = "project"
	LinkedObjectObjective           = "objective"
	LinkedObjectSprint              = "sprint"
	LinkedObjectSupportConversation = "support_conversation"
)

// ─── Helper types ───────────────────────────────────────────────────────────

// DocsStringArray is a PostgreSQL text[] compatible type for GORM.
type DocsStringArray []string

// Value implements driver.Valuer for PostgreSQL text[].
func (a DocsStringArray) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	escaped := make([]string, len(a))
	for i, s := range a {
		escaped[i] = `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return "{" + strings.Join(escaped, ",") + "}", nil
}

// Scan implements sql.Scanner for PostgreSQL text[].
func (a *DocsStringArray) Scan(src interface{}) error {
	if src == nil {
		*a = DocsStringArray{}
		return nil
	}
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("unsupported type for DocsStringArray: %T", src)
	}
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		*a = DocsStringArray{}
		return nil
	}
	parts := strings.Split(s, ",")
	result := make(DocsStringArray, len(parts))
	for i, p := range parts {
		result[i] = strings.Trim(p, `"`)
	}
	*a = result
	return nil
}

// ─── Core models ────────────────────────────────────────────────────────────

// DocsSpace is the top-level container owned by a team.
type DocsSpace struct {
	ID                string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string     `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_space_ws_slug,priority:1;index:idx_docs_space_ws_team,priority:1;index:idx_docs_space_ws_pos,priority:1"`
	TeamID            *string    `json:"team_id" gorm:"type:uuid;index:idx_docs_space_ws_team,priority:2"`
	Name              string     `json:"name" gorm:"not null"`
	Slug              string     `json:"slug" gorm:"not null;uniqueIndex:idx_docs_space_ws_slug,priority:2"`
	Icon              *string    `json:"icon"`
	Visibility        string     `json:"visibility" gorm:"not null;default:'workspace_wide'"`
	Type              string     `json:"type" gorm:"not null;default:'internal'"`
	DefaultReviewDays *int       `json:"default_review_days"`
	IsSystem          bool       `json:"is_system" gorm:"not null;default:false"`
	Position          int        `json:"position" gorm:"not null;default:0;index:idx_docs_space_ws_pos,priority:2"`
	CreatedBy         string     `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt         *time.Time `json:"deleted_at" gorm:"index"`
}

func (DocsSpace) TableName() string { return "docs_spaces" }

// DocsSpaceTeam is the many-to-many join between spaces and teams.
type DocsSpaceTeam struct {
	SpaceID   string    `json:"space_id" gorm:"type:uuid;primaryKey"`
	TeamID    string    `json:"team_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (DocsSpaceTeam) TableName() string { return "docs_space_teams" }

// DocsSpaceWithTeams is a space with its associated team IDs.
type DocsSpaceWithTeams struct {
	DocsSpace
	TeamIDs []string `json:"team_ids" gorm:"-"`
}

// DocsCollection groups documents inside a space. Collections form a bounded
// tree: each collection has an optional parent collection within the same
// space, with a maximum depth of 0..2 enforced at the service layer.
//
// Ordering is scoped to the (space_id, parent_collection_id) sibling bucket.
// The partial unique index on (workspace_id, slug) for non-deleted rows is
// enforced via dbmigrate SQL (GORM tags cannot express partial uniqueness).
type DocsCollection struct {
	ID                 string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SpaceID            string     `json:"space_id" gorm:"type:uuid;not null;index:idx_docs_collections_space_parent_pos,priority:1"`
	WorkspaceID        string     `json:"workspace_id" gorm:"type:uuid;not null"`
	ParentCollectionID *string    `json:"parent_collection_id" gorm:"type:uuid;index:idx_docs_collections_space_parent_pos,priority:2;index:idx_docs_collections_parent"`
	Depth              int        `json:"depth" gorm:"not null;default:0"`
	Name               string     `json:"name" gorm:"not null"`
	PublicID           string     `json:"public_id" gorm:"not null;default:''"`
	Slug               string     `json:"slug" gorm:"not null;default:''"`
	Description        *string    `json:"description"`
	Icon               *string    `json:"icon"`
	Position           int        `json:"position" gorm:"not null;default:0;index:idx_docs_collections_space_parent_pos,priority:3"`
	CreatedBy          string     `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt          time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt          *time.Time `json:"deleted_at" gorm:"index"`
}

func (DocsCollection) TableName() string { return "docs_collections" }

// DocsDocument is the core document metadata.
type DocsDocument struct {
	ID               string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_docs_doc_ws_space_status,priority:1;index:idx_docs_doc_ws_team_updated,priority:1"`
	SpaceID          string          `json:"space_id" gorm:"type:uuid;not null;index:idx_docs_doc_ws_space_status,priority:2"`
	CollectionID     *string         `json:"collection_id" gorm:"type:uuid"`
	Title            string          `json:"title" gorm:"not null"`
	Status           string          `json:"status" gorm:"not null;default:'draft';index:idx_docs_doc_ws_space_status,priority:3"`
	Visibility       string          `json:"visibility" gorm:"not null;default:'workspace_wide'"`
	OwnerID          *string         `json:"owner_id" gorm:"type:uuid;index:idx_docs_doc_owner_review,priority:1"`
	TeamID           *string         `json:"team_id" gorm:"type:uuid;index:idx_docs_doc_ws_team_updated,priority:2"`
	TemplateKey      *string         `json:"template_key"`
	Excerpt          *string         `json:"excerpt"`
	Icon             *string         `json:"icon"`
	Tags             DocsStringArray `json:"tags" gorm:"type:text[]"`
	Position         int             `json:"position" gorm:"not null;default:0"`
	IsPinned         bool            `json:"is_pinned" gorm:"not null;default:false"`
	IsPubliclyShared bool            `json:"is_publicly_shared" gorm:"not null;default:false"`
	ShareToken       *string         `json:"share_token" gorm:"uniqueIndex"`
	IsLocked         bool            `json:"is_locked" gorm:"not null;default:false"`
	LockedBy         *string         `json:"locked_by" gorm:"type:uuid"`
	LastReviewedAt   *time.Time      `json:"last_reviewed_at"`
	NextReviewAt     *time.Time      `json:"next_review_at" gorm:"index:idx_docs_doc_owner_review,priority:2"`
	PublishedAt      *time.Time      `json:"published_at"`
	CreatedBy        string          `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt        time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time       `json:"updated_at" gorm:"autoUpdateTime;index:idx_docs_doc_ws_team_updated,priority:3"`
	DeletedAt        *time.Time      `json:"deleted_at" gorm:"index"`

	// Transient fields (not stored in docs_documents, populated by handlers)
	HCSlug                string     `json:"hc_slug,omitempty" gorm:"-"`
	HCOGTitle             *string    `json:"hc_og_title,omitempty" gorm:"-"`
	HCOGDescription       *string    `json:"hc_og_description,omitempty" gorm:"-"`
	HCOGImageURL          *string    `json:"hc_og_image_url,omitempty" gorm:"-"`
	HCOGImageAlt          *string    `json:"hc_og_image_alt,omitempty" gorm:"-"`
	HasUnpublishedChanges bool       `json:"has_unpublished_changes" gorm:"-"`
	LivePublishedAt       *time.Time `json:"live_published_at,omitempty" gorm:"-"`
	LiveSlug              *string    `json:"live_slug,omitempty" gorm:"-"`
}

func (DocsDocument) TableName() string { return "docs_documents" }

// DocsContent stores document content separately from metadata.
type DocsContent struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DocumentID  string          `json:"document_id" gorm:"type:uuid;not null;uniqueIndex"`
	Content     json.RawMessage `json:"content" gorm:"type:jsonb"`
	ContentText string          `json:"content_text" gorm:"type:text"`
	WordCount   int             `json:"word_count" gorm:"not null;default:0"`

	// Import provenance — snapshot for reconversion/debugging, not the live source of truth.
	// Stored as post-image-rewrite, pre-conversion HTML so reconversion works even if
	// the original external asset URLs die.
	ImportSourceHTML     *string `json:"-" gorm:"type:text"`
	ImportSourceSystem   *string `json:"-" gorm:"type:text"`
	ImportSourceObjectID *string `json:"-" gorm:"type:text"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsContent) TableName() string { return "docs_contents" }

// DocsVersion stores saved snapshots.
type DocsVersion struct {
	ID            string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DocumentID    string          `json:"document_id" gorm:"type:uuid;not null;index:idx_docs_version_doc_created,priority:1"`
	Content       json.RawMessage `json:"content" gorm:"type:jsonb"`
	ContentText   string          `json:"content_text" gorm:"type:text"`
	SnapshotLabel *string         `json:"snapshot_label"`
	VersionType   string          `json:"version_type" gorm:"not null;default:'manual'"`
	WordCount     int             `json:"word_count" gorm:"not null;default:0"`
	CreatedBy     string          `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt     time.Time       `json:"created_at" gorm:"autoCreateTime;index:idx_docs_version_doc_created,priority:2,sort:desc"`
}

func (DocsVersion) TableName() string { return "docs_versions" }

// DocsLink links documents to PM and Support objects.
type DocsLink struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;index:idx_docs_link_ws_obj,priority:1"`
	DocumentID       string    `json:"document_id" gorm:"type:uuid;not null;index:idx_docs_link_doc_type,priority:1"`
	LinkedObjectType string    `json:"linked_object_type" gorm:"not null;index:idx_docs_link_doc_type,priority:2;index:idx_docs_link_obj,priority:1;index:idx_docs_link_ws_obj,priority:2"`
	LinkedObjectID   string    `json:"linked_object_id" gorm:"type:uuid;not null;index:idx_docs_link_obj,priority:2;index:idx_docs_link_ws_obj,priority:3"`
	LinkContext      string    `json:"link_context" gorm:"not null;default:'attached'"`
	CreatedBy        string    `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`

	// Transient fields — enriched by the service layer, not stored in DB.
	LinkedObjectName      string `json:"linked_object_name,omitempty" gorm:"-"`
	LinkedObjectDisplayID int    `json:"linked_object_display_id,omitempty" gorm:"-"`
	DocumentTitle         string `json:"document_title,omitempty" gorm:"-"`
}

func (DocsLink) TableName() string { return "docs_links" }

// HelpcenterHeaderLink is a single header navigation link.
type HelpcenterHeaderLink struct {
	Label    string `json:"label"`
	URL      string `json:"url"`
	External bool   `json:"external"`
	Style    string `json:"style"`    // "text" (default) or "button"
	Position int    `json:"position"` // display order (0-based)
}

// HelpcenterFooterLink is a single footer link.
type HelpcenterFooterLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// HelpcenterFooterConfig stores footer customization.
type HelpcenterFooterConfig struct {
	CopyrightText string                 `json:"copyright_text"`
	Links         []HelpcenterFooterLink `json:"links"`
}

// HomepageFeaturedCard is a single card on the help center homepage.
type HomepageFeaturedCard struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	LinkType    string `json:"link_type"`
	LinkValue   string `json:"link_value"`
	SpaceSlug   string `json:"space_slug"`
	PublicID    string `json:"public_id"`
}

// HelpcenterHomepageConfig stores homepage hero and featured cards.
type HelpcenterHomepageConfig struct {
	HeroTitle     string                 `json:"hero_title"`
	HeroSubtitle  string                 `json:"hero_subtitle"`
	FeaturedCards []HomepageFeaturedCard `json:"featured_cards"`
}

// HelpcenterSpaceNavConfig stores client-side space ordering and hiding.
type HelpcenterSpaceNavConfig struct {
	Order  []string `json:"order"`
	Hidden []string `json:"hidden"`
}

// DocsHelpcenterConfig stores workspace-level help center configuration.
type DocsHelpcenterConfig struct {
	ID                      string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID             string          `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex"`
	Subdomain               string          `json:"subdomain" gorm:"not null"`
	CustomDomain            *string         `json:"custom_domain"`
	PublicURLMode           string          `json:"public_url_mode" gorm:"not null;default:'hosted_subdomain'"`
	ReverseProxyHost        *string         `json:"reverse_proxy_host"`
	ReverseProxyBasePath    *string         `json:"reverse_proxy_base_path"`
	BrandName               string          `json:"brand_name" gorm:"not null"`
	BrandLogoURL            *string         `json:"brand_logo_url"`
	BrandLogoDarkURL        *string         `json:"brand_logo_dark_url"`
	BrandColor              string          `json:"brand_color" gorm:"not null;default:'#000000'"`
	FaviconURL              *string         `json:"favicon_url"`
	ThemeMode               string          `json:"theme_mode" gorm:"not null;default:'system'"`
	HeaderLinks             json.RawMessage `json:"header_links" gorm:"type:jsonb;default:'[]'"`
	FooterConfig            json.RawMessage `json:"footer_config" gorm:"type:jsonb;default:'{}'"`
	HomepageConfig          json.RawMessage `json:"homepage_config" gorm:"type:jsonb;default:'{}'"`
	SpaceNavConfig          json.RawMessage `json:"space_nav_config" gorm:"type:jsonb;default:'{}'"`
	SearchPlaceholder       *string         `json:"search_placeholder"`
	DefaultLocale           string          `json:"default_locale" gorm:"not null;default:'en'"`
	EnabledLocales          DocsStringArray `json:"enabled_locales" gorm:"type:text[]"`
	ProtectedTerms          DocsStringArray `json:"protected_terms" gorm:"type:text[]"`
	ShowLanguageSwitcher    bool            `json:"show_language_switcher" gorm:"not null;default:false"`
	FallbackToDefaultLocale bool            `json:"fallback_to_default_locale" gorm:"not null;default:true"`
	IsPublished             bool            `json:"is_published" gorm:"not null;default:false"`
	SEOTitle                *string         `json:"seo_title"`
	SEODescription          *string         `json:"seo_description"`
	OGTitle                 *string         `json:"og_title"`
	OGDescription           *string         `json:"og_description"`
	OGImageURL              *string         `json:"og_image_url"`
	OGImageAlt              *string         `json:"og_image_alt"`
	SupportEmail            *string         `json:"support_email"`
	CreatedAt               time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt               time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsHelpcenterConfig) TableName() string { return "docs_helpcenter_configs" }

// DocsHelpcenterArticle is a 1:1 extension table for help center articles.
type DocsHelpcenterArticle struct {
	ID                string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DocumentID        string     `json:"document_id" gorm:"type:uuid;not null;uniqueIndex"`
	PublicID          string     `json:"public_id" gorm:"uniqueIndex"`
	Slug              string     `json:"slug" gorm:"not null;default:''"`
	SEOTitle          *string    `json:"seo_title"`
	SEODescription    *string    `json:"seo_description"`
	OGTitle           *string    `json:"og_title"`
	OGDescription     *string    `json:"og_description"`
	OGImageURL        *string    `json:"og_image_url"`
	OGImageAlt        *string    `json:"og_image_alt"`
	HelpfulCount      int        `json:"helpful_count" gorm:"not null;default:0"`
	NotHelpfulCount   int        `json:"not_helpful_count" gorm:"not null;default:0"`
	ViewCount         int        `json:"view_count" gorm:"not null;default:0"`
	PublicPublishedAt *time.Time `json:"public_published_at" gorm:"index"`
	CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsHelpcenterArticle) TableName() string { return "docs_helpcenter_articles" }

// DocsSlugAlias stores old public slugs for redirect.
type DocsSlugAlias struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_docs_slug_alias_ws_slug,priority:1"`
	DocumentID  string    `json:"document_id" gorm:"type:uuid;not null;index"`
	OldSlug     string    `json:"old_slug" gorm:"not null;uniqueIndex:idx_docs_slug_alias_ws_slug,priority:2"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (DocsSlugAlias) TableName() string { return "docs_slug_aliases" }

// DocsReviewQueue is schema-only for v1.
type DocsReviewQueue struct {
	ID          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DocumentID  string     `json:"document_id" gorm:"type:uuid;not null;index"`
	AssignedTo  *string    `json:"assigned_to" gorm:"type:uuid"`
	Status      string     `json:"status" gorm:"not null;default:'pending'"`
	DueAt       *time.Time `json:"due_at"`
	CreatedBy   string     `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsReviewQueue) TableName() string { return "docs_review_queue" }

// DocsArticleFeedback is schema-only for v1.
type DocsArticleFeedback struct {
	ID         string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DocumentID string    `json:"document_id" gorm:"type:uuid;not null;index"`
	IsHelpful  bool      `json:"is_helpful" gorm:"not null"`
	Comment    *string   `json:"comment"`
	SessionID  *string   `json:"session_id"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (DocsArticleFeedback) TableName() string { return "docs_article_feedback" }

// DocsComment is schema-only for v1 (page-level comments).
type DocsComment struct {
	ID         string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DocumentID string     `json:"document_id" gorm:"type:uuid;not null;index"`
	ParentID   *string    `json:"parent_id" gorm:"type:uuid"`
	AuthorID   string     `json:"author_id" gorm:"type:uuid;not null"`
	Content    string     `json:"content" gorm:"type:text;not null"`
	CreatedAt  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt  *time.Time `json:"deleted_at" gorm:"index"`
}

func (DocsComment) TableName() string { return "docs_comments" }

// ─── Request/Response DTOs ──────────────────────────────────────────────────

// CreateDocsSpaceRequest is the payload for creating a space.
type CreateDocsSpaceRequest struct {
	TeamID            *string  `json:"team_id"`
	TeamIDs           []string `json:"team_ids"`
	Name              string   `json:"name"`
	Slug              string   `json:"slug"`
	Icon              *string  `json:"icon"`
	Visibility        string   `json:"visibility"`
	Type              string   `json:"type"`
	DefaultReviewDays *int     `json:"default_review_days"`
}

// UpdateDocsSpaceRequest is the payload for updating a space.
type UpdateDocsSpaceRequest struct {
	Name              *string  `json:"name"`
	Slug              *string  `json:"slug"`
	Icon              *string  `json:"icon"`
	Type              *string  `json:"type"`
	Visibility        *string  `json:"visibility"`
	DefaultReviewDays *int     `json:"default_review_days"`
	TeamIDs           []string `json:"team_ids"`
	SetTeamIDs        bool     `json:"set_team_ids"`
}

// CreateDocsCollectionRequest is the payload for creating a collection.
//
// ParentCollectionID is optional. When nil the collection is created at the
// top of the space. Service-layer validation enforces the depth cap.
type CreateDocsCollectionRequest struct {
	Name               string  `json:"name"`
	Slug               *string `json:"slug"`
	Description        *string `json:"description"`
	Icon               *string `json:"icon"`
	ParentCollectionID *string `json:"parent_collection_id"`
}

// UpdateDocsCollectionRequest is the payload for updating a collection.
//
// ParentCollectionID semantics: a nil pointer leaves the parent unchanged;
// a pointer to the empty string reparents the collection to the top of the
// space; any other value reparents under the referenced collection within
// the same space. The depth cap is enforced at the service layer.
type UpdateDocsCollectionRequest struct {
	Name               *string `json:"name"`
	Description        *string `json:"description"`
	Icon               *string `json:"icon"`
	Position           *int    `json:"position"`
	ParentCollectionID *string `json:"parent_collection_id"`
}

// DocsCollectionDeleteImpact summarizes what permanent collection deletion affects.
type DocsCollectionDeleteImpact struct {
	CollectionID           string `json:"collection_id"`
	CollectionName         string `json:"collection_name"`
	SpaceID                string `json:"space_id"`
	CollectionCount        int    `json:"collection_count"`
	DocumentCount          int    `json:"document_count"`
	ArchivedDocumentCount  int    `json:"archived_document_count"`
	PublishedDocumentCount int    `json:"published_document_count"`
	PublicDocumentCount    int    `json:"public_document_count"`
}

// DocsSpaceDeleteImpact summarizes what permanent space deletion affects.
// CollectionCount is the total number of collections in the space (no
// self-counting; unlike collection impact, the space isn't a collection).
type DocsSpaceDeleteImpact struct {
	SpaceID                string `json:"space_id"`
	SpaceName              string `json:"space_name"`
	CollectionCount        int    `json:"collection_count"`
	DocumentCount          int    `json:"document_count"`
	ArchivedDocumentCount  int    `json:"archived_document_count"`
	PublishedDocumentCount int    `json:"published_document_count"`
	PublicDocumentCount    int    `json:"public_document_count"`
}

// CreateDocsDocumentRequest is the payload for creating a document.
type CreateDocsDocumentRequest struct {
	SpaceID      string   `json:"space_id"`
	CollectionID *string  `json:"collection_id"`
	Title        string   `json:"title"`
	OwnerID      *string  `json:"owner_id"`
	TemplateKey  *string  `json:"template_key"`
	Icon         *string  `json:"icon"`
	Tags         []string `json:"tags"`
}

// ─── Reorder request DTOs ───────────────────────────────────────────────────

// ReorderDocsSpacesRequest reorders spaces within a section (internal or external_capable).
type ReorderDocsSpacesRequest struct {
	Section  string   `json:"section"`   // "internal" | "external_capable"
	SpaceIDs []string `json:"space_ids"` // full ordered sibling list
}

// ReorderDocsCollectionsRequest reorders collections within a space.
// ReorderDocsCollectionsRequest reorders one (space_id, parent_collection_id)
// sibling bucket. ParentCollectionID is optional and defaults to the
// top-level bucket when nil or an empty string.
type ReorderDocsCollectionsRequest struct {
	CollectionIDs      []string `json:"collection_ids"`                 // full ordered list for one sibling bucket
	ParentCollectionID *string  `json:"parent_collection_id,omitempty"` // nil / "" = top-level bucket
}

// ReorderDocsDocumentsRequest reorders documents within a bucket (collection or uncategorized).
type ReorderDocsDocumentsRequest struct {
	CollectionID *string  `json:"collection_id"` // nil => uncategorized bucket
	DocumentIDs  []string `json:"document_ids"`  // full ordered list for one bucket
}

// ReorderDocsChildItem is a single entry in a mixed-children reorder
// payload. Kind must be "collection" or "article".
type ReorderDocsChildItem struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// ReorderDocsChildrenRequest reorders a mixed list of collections and
// articles that share the same parent (or sit at the space root).
// Positions are assigned sequentially across both types in one
// transaction so cross-type drag-and-drop persists correctly.
type ReorderDocsChildrenRequest struct {
	ParentCollectionID *string                `json:"parent_collection_id"` // nil / "" => space root
	Items              []ReorderDocsChildItem `json:"items"`
}

// UpdateDocsDocumentRequest is the payload for updating a document.
type UpdateDocsDocumentRequest struct {
	Title        *string  `json:"title"`
	CollectionID *string  `json:"collection_id"`
	OwnerID      *string  `json:"owner_id"`
	TemplateKey  *string  `json:"template_key"`
	Excerpt      *string  `json:"excerpt"`
	Icon         *string  `json:"icon"`
	Tags         []string `json:"tags"`
	IsPinned     *bool    `json:"is_pinned"`
}

// MoveDocsDocumentRequest is the payload for moving a document.
type MoveDocsDocumentRequest struct {
	SpaceID      string  `json:"space_id"`
	CollectionID *string `json:"collection_id"`
}

// SaveDocsContentRequest is the payload for saving document content.
type SaveDocsContentRequest struct {
	Content json.RawMessage `json:"content"`
}

// SaveDocsMarkdownRequest is the payload for saving document content from Markdown.
// The backend wraps the markdown in a JSON envelope so the frontend can auto-convert.
type SaveDocsMarkdownRequest struct {
	Markdown string `json:"markdown"`
}

type ImportDocsExternalImageRequest struct {
	ImageURL string `json:"image_url"`
}

type ImportDocsExternalImageResponse struct {
	URL string `json:"url"`
}

// CreateDocsVersionRequest is the payload for manually creating a version snapshot.
type CreateDocsVersionRequest struct {
	SnapshotLabel *string `json:"snapshot_label"`
}

// UpdateDocsVersionRequest is the payload for renaming a manual version's label.
type UpdateDocsVersionRequest struct {
	SnapshotLabel *string `json:"snapshot_label"`
}

// CreateDocsLinkRequest is the payload for creating a document link.
type CreateDocsLinkRequest struct {
	LinkedObjectType string `json:"linked_object_type"`
	LinkedObjectID   string `json:"linked_object_id"`
	LinkContext      string `json:"link_context"`
}

// UpdateDocsHelpcenterConfigRequest is the payload for updating help center config.
type UpdateDocsHelpcenterConfigRequest struct {
	Subdomain               *string         `json:"subdomain"`
	CustomDomain            *string         `json:"custom_domain"`
	PublicURLMode           *string         `json:"public_url_mode"`
	ReverseProxyHost        *string         `json:"reverse_proxy_host"`
	ReverseProxyBasePath    *string         `json:"reverse_proxy_base_path"`
	BrandName               *string         `json:"brand_name"`
	BrandLogoURL            *string         `json:"brand_logo_url"`
	BrandLogoDarkURL        *string         `json:"brand_logo_dark_url"`
	BrandColor              *string         `json:"brand_color"`
	FaviconURL              *string         `json:"favicon_url"`
	ThemeMode               *string         `json:"theme_mode"`
	HeaderLinks             json.RawMessage `json:"header_links,omitempty"`
	FooterConfig            json.RawMessage `json:"footer_config,omitempty"`
	HomepageConfig          json.RawMessage `json:"homepage_config,omitempty"`
	SpaceNavConfig          json.RawMessage `json:"space_nav_config,omitempty"`
	SearchPlaceholder       *string         `json:"search_placeholder"`
	DefaultLocale           *string         `json:"default_locale"`
	EnabledLocales          []string        `json:"enabled_locales"`
	ProtectedTerms          []string        `json:"protected_terms"`
	ShowLanguageSwitcher    *bool           `json:"show_language_switcher"`
	FallbackToDefaultLocale *bool           `json:"fallback_to_default_locale"`
	IsPublished             *bool           `json:"is_published"`
	SEOTitle                *string         `json:"seo_title"`
	SEODescription          *string         `json:"seo_description"`
	OGTitle                 *string         `json:"og_title"`
	OGDescription           *string         `json:"og_description"`
	OGImageURL              *string         `json:"og_image_url"`
	OGImageAlt              *string         `json:"og_image_alt"`
	SupportEmail            *string         `json:"support_email"`
}

// UpdateDocsHelpcenterArticleMetadataRequest updates source-locale social metadata.
type UpdateDocsHelpcenterArticleMetadataRequest struct {
	OGTitle       *string `json:"og_title"`
	OGDescription *string `json:"og_description"`
	OGImageURL    *string `json:"og_image_url"`
	OGImageAlt    *string `json:"og_image_alt"`
}

// DocsArticleFeedbackRequest is the payload for submitting article feedback.
type DocsArticleFeedbackRequest struct {
	IsHelpful bool    `json:"is_helpful"`
	Comment   *string `json:"comment"`
	SessionID *string `json:"session_id"`
}

// ToggleDocShareRequest is the payload for toggling public share on a document.
type ToggleDocShareRequest struct {
	IsPubliclyShared bool `json:"is_publicly_shared"`
}

// ToggleDocLockRequest is the payload for locking/unlocking a document.
type ToggleDocLockRequest struct {
	IsLocked bool `json:"is_locked"`
}

// PublicDocResponse is the response for a publicly shared document.
type PublicDocResponse struct {
	Document *DocsDocument `json:"document"`
	Content  *DocsContent  `json:"content"`
}

// ─── Public Help Center Response DTOs ────────────────────────────────────────

// PublicSpaceResponse is the public-facing space for help center top nav.
type PublicSpaceResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Icon        *string `json:"icon"`
	Description *string `json:"description"`
}

// PublicNavArticle is a published article within a collection for sidebar navigation.
type PublicNavArticle struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	PublicID    string  `json:"public_id"`
	Position    int     `json:"position"`
	PublishedAt *string `json:"published_at"`
}

// PublicNavCollection is a collection with its published articles for
// sidebar navigation. The nav tree is returned as a flat list; callers
// build the nested structure using ParentCollectionID and Depth.
type PublicNavCollection struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	Slug               string             `json:"slug"`
	PublicID           string             `json:"public_id"`
	SpaceSlug          string             `json:"space_slug,omitempty"`
	Icon               *string            `json:"icon"`
	ParentCollectionID *string            `json:"parent_collection_id"`
	Depth              int                `json:"depth"`
	Position           int                `json:"position"`
	Articles           []PublicNavArticle `json:"articles"`
}

// PublicNavBreadcrumbEntry is one segment of a localized collection
// breadcrumb path. The segments are ordered from the top-level ancestor
// down to the active collection itself, so the public help center can
// render them as "Root > Parent > Current".
type PublicNavBreadcrumbEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	PublicID string `json:"public_id"`
}

// PublicArticleResponse is the full article detail for the help center content area.
type PublicArticleResponse struct {
	ID                 string  `json:"id"`
	Title              string  `json:"title"`
	Slug               string  `json:"slug"`
	PublicID           string  `json:"public_id"`
	Locale             string  `json:"locale,omitempty"`
	RequestedLocale    string  `json:"requested_locale,omitempty"`
	IsFallback         bool    `json:"is_fallback,omitempty"`
	Excerpt            *string `json:"excerpt"`
	Icon               *string `json:"icon"`
	Status             string  `json:"status"`
	SpaceSlug          string  `json:"space_slug,omitempty"`
	CollectionID       *string `json:"collection_id"`
	CollectionName     *string `json:"collection_name"`
	CollectionSlug     *string `json:"collection_slug,omitempty"`
	CollectionPublicID *string `json:"collection_public_id,omitempty"`
	PublishedAt        *string `json:"published_at"`
	SEOTitle           *string `json:"seo_title"`
	SEODescription     *string `json:"seo_description"`
	OGTitle            *string `json:"og_title"`
	OGDescription      *string `json:"og_description"`
	OGImageURL         *string `json:"og_image_url"`
	OGImageAlt         *string `json:"og_image_alt"`
	HelpfulCount       int     `json:"helpful_count"`
	NotHelpfulCount    int     `json:"not_helpful_count"`
	ViewCount          int     `json:"view_count"`
	ContentHTML        *string `json:"content_html"`
}

// PreviewArticleResponse contains rendered HTML for article preview (any status).
type PreviewArticleResponse struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Excerpt        *string `json:"excerpt"`
	Icon           *string `json:"icon"`
	Status         string  `json:"status"`
	CollectionID   *string `json:"collection_id"`
	CollectionName *string `json:"collection_name"`
	SpaceName      string  `json:"space_name"`
	SpaceSlug      string  `json:"space_slug"`
	ContentHTML    string  `json:"content_html"`
}

// PublicSearchResultResponse is a search result with space + collection
// tree context.
//
// CollectionAncestorPath is a human-readable breadcrumb string such as
// "Root / Middle / Current" built from the localized ancestor names of
// the owning collection. It is nil when the article lives directly in
// the space (no owning collection) or when the collection has no
// ancestors beyond itself. The frontend can render it unchanged above
// the title to give nested search hits obvious context.
type PublicSearchResultResponse struct {
	ID                     string  `json:"id"`
	Title                  string  `json:"title"`
	Slug                   string  `json:"slug"`
	PublicID               string  `json:"public_id"`
	Locale                 string  `json:"locale,omitempty"`
	RequestedLocale        string  `json:"requested_locale,omitempty"`
	IsFallback             bool    `json:"is_fallback,omitempty"`
	Excerpt                *string `json:"excerpt"`
	CollectionID           *string `json:"collection_id,omitempty"`
	CollectionName         *string `json:"collection_name"`
	CollectionSlug         *string `json:"collection_slug,omitempty"`
	CollectionPublicID     *string `json:"collection_public_id,omitempty"`
	CollectionAncestorPath *string `json:"collection_ancestor_path,omitempty"`
	SpaceSlug              string  `json:"space_slug"`
	SpaceName              string  `json:"space_name"`
}
