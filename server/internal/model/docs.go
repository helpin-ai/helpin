package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ─── Doc type and status constants ──────────────────────────────────────────

// Canonical doc types.
const (
	DocTypeWiki              = "wiki"
	DocTypeSOP               = "sop"
	DocTypeFeatureDoc        = "feature_doc"
	DocTypeSupportArticle    = "support_article"
	DocTypeHelpCenterArticle = "help_center_article"
)

// AllDocTypes returns every valid doc_type value.
func AllDocTypes() []string {
	return []string{DocTypeWiki, DocTypeSOP, DocTypeFeatureDoc, DocTypeSupportArticle, DocTypeHelpCenterArticle}
}

// IsValidDocType checks whether a doc_type string is canonical.
func IsValidDocType(t string) bool {
	for _, v := range AllDocTypes() {
		if v == t {
			return true
		}
	}
	return false
}

// IsExternalCapableDocType returns true if the doc type can be published externally.
func IsExternalCapableDocType(t string) bool {
	return t == DocTypeHelpCenterArticle
}

// OwnerRequired returns true if the doc type requires an owner.
func OwnerRequired(docType string) bool {
	switch docType {
	case DocTypeSOP, DocTypeFeatureDoc, DocTypeSupportArticle, DocTypeHelpCenterArticle:
		return true
	default:
		return false
	}
}

// AllowedTemplateKeys returns the canonical template_key values.
func AllowedTemplateKeys() []string {
	return AllDocTypes()
}

// IsValidTemplateKey checks whether a template_key is canonical.
func IsValidTemplateKey(k string) bool {
	for _, v := range AllowedTemplateKeys() {
		if v == k {
			return true
		}
	}
	return false
}

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
	LinkedObjectEpic          = "epic"
	LinkedObjectStory         = "story"
	LinkedObjectProject       = "project"
	LinkedObjectObjective     = "objective"
	LinkedObjectSprint        = "sprint"
	LinkedObjectSupportTicket = "support_ticket"
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
	RestrictToOwners  bool       `json:"restrict_to_owners" gorm:"not null;default:false"`
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

// DocsCollection groups documents inside a space.
type DocsCollection struct {
	ID          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SpaceID     string     `json:"space_id" gorm:"type:uuid;not null;index:idx_docs_collection_space_pos,priority:1"`
	WorkspaceID string     `json:"workspace_id" gorm:"type:uuid;not null"`
	Name        string     `json:"name" gorm:"not null"`
	Description *string    `json:"description"`
	Icon        *string    `json:"icon"`
	Position    int        `json:"position" gorm:"not null;default:0;index:idx_docs_collection_space_pos,priority:2"`
	CreatedBy   string     `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   *time.Time `json:"deleted_at" gorm:"index"`
}

func (DocsCollection) TableName() string { return "docs_collections" }

// DocsDocument is the core document metadata.
type DocsDocument struct {
	ID               string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_docs_doc_ws_space_status,priority:1;index:idx_docs_doc_ws_type_status,priority:1;index:idx_docs_doc_ws_team_updated,priority:1"`
	SpaceID          string          `json:"space_id" gorm:"type:uuid;not null;index:idx_docs_doc_ws_space_status,priority:2"`
	CollectionID     *string         `json:"collection_id" gorm:"type:uuid"`
	Title            string          `json:"title" gorm:"not null"`
	DocType          string          `json:"doc_type" gorm:"not null;index:idx_docs_doc_ws_type_status,priority:2"`
	Status           string          `json:"status" gorm:"not null;default:'draft';index:idx_docs_doc_ws_space_status,priority:3;index:idx_docs_doc_ws_type_status,priority:3"`
	Visibility       string          `json:"visibility" gorm:"not null;default:'workspace_wide'"`
	OwnerID          *string         `json:"owner_id" gorm:"type:uuid;index:idx_docs_doc_owner_review,priority:1"`
	TeamID           *string         `json:"team_id" gorm:"type:uuid;index:idx_docs_doc_ws_team_updated,priority:2"`
	TemplateKey      *string         `json:"template_key"`
	Excerpt          *string         `json:"excerpt"`
	Icon             *string         `json:"icon"`
	Tags             DocsStringArray `json:"tags" gorm:"type:text[]"`
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
}

func (DocsDocument) TableName() string { return "docs_documents" }

// DocsContent stores document content separately from metadata.
type DocsContent struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DocumentID  string          `json:"document_id" gorm:"type:uuid;not null;uniqueIndex"`
	Content     json.RawMessage `json:"content" gorm:"type:jsonb"`
	ContentText string          `json:"content_text" gorm:"type:text"`
	WordCount   int             `json:"word_count" gorm:"not null;default:0"`
	CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
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
}

func (DocsLink) TableName() string { return "docs_links" }

// DocsHelpcenterConfig stores workspace-level help center configuration.
type DocsHelpcenterConfig struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex"`
	Subdomain      string    `json:"subdomain" gorm:"not null"`
	CustomDomain   *string   `json:"custom_domain"`
	BrandName      string    `json:"brand_name" gorm:"not null"`
	BrandLogoURL   *string   `json:"brand_logo_url"`
	BrandColor     string    `json:"brand_color" gorm:"not null;default:'#000000'"`
	IsPublished    bool      `json:"is_published" gorm:"not null;default:false"`
	SEOTitle       *string   `json:"seo_title"`
	SEODescription *string   `json:"seo_description"`
	SupportEmail   *string   `json:"support_email"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsHelpcenterConfig) TableName() string { return "docs_helpcenter_configs" }

// DocsHelpcenterArticle is a 1:1 extension table for help center articles.
type DocsHelpcenterArticle struct {
	ID                string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DocumentID        string     `json:"document_id" gorm:"type:uuid;not null;uniqueIndex"`
	SEOTitle          *string    `json:"seo_title"`
	SEODescription    *string    `json:"seo_description"`
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
	RestrictToOwners  bool     `json:"restrict_to_owners"`
	DefaultReviewDays *int     `json:"default_review_days"`
}

// UpdateDocsSpaceRequest is the payload for updating a space.
type UpdateDocsSpaceRequest struct {
	Name              *string  `json:"name"`
	Slug              *string  `json:"slug"`
	Icon              *string  `json:"icon"`
	Type              *string  `json:"type"`
	Visibility        *string  `json:"visibility"`
	RestrictToOwners  *bool    `json:"restrict_to_owners"`
	DefaultReviewDays *int     `json:"default_review_days"`
	TeamIDs           []string `json:"team_ids"`
	SetTeamIDs        bool     `json:"set_team_ids"`
}

// CreateDocsCollectionRequest is the payload for creating a collection.
type CreateDocsCollectionRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
}

// UpdateDocsCollectionRequest is the payload for updating a collection.
type UpdateDocsCollectionRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Position    *int    `json:"position"`
}

// CreateDocsDocumentRequest is the payload for creating a document.
type CreateDocsDocumentRequest struct {
	SpaceID      string   `json:"space_id"`
	CollectionID *string  `json:"collection_id"`
	Title        string   `json:"title"`
	DocType      string   `json:"doc_type"`
	OwnerID      *string  `json:"owner_id"`
	TemplateKey  *string  `json:"template_key"`
	Icon         *string  `json:"icon"`
	Tags         []string `json:"tags"`
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
	Subdomain      *string `json:"subdomain"`
	CustomDomain   *string `json:"custom_domain"`
	BrandName      *string `json:"brand_name"`
	BrandLogoURL   *string `json:"brand_logo_url"`
	BrandColor     *string `json:"brand_color"`
	IsPublished    *bool   `json:"is_published"`
	SEOTitle       *string `json:"seo_title"`
	SEODescription *string `json:"seo_description"`
	SupportEmail   *string `json:"support_email"`
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
