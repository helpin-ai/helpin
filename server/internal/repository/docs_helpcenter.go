package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsHelpcenterRepository handles DB operations for help center config, articles, slugs, and feedback.
type DocsHelpcenterRepository struct {
	db         *gorm.DB
	useSortKey bool
}

// NewDocsHelpcenterRepository creates a new DocsHelpcenterRepository.
func NewDocsHelpcenterRepository(db *gorm.DB, useSortKey ...bool) *DocsHelpcenterRepository {
	enabled := false
	if len(useSortKey) > 0 {
		enabled = useSortKey[0]
	}
	return &DocsHelpcenterRepository{db: db, useSortKey: enabled}
}

// hcDocOrderBy returns the canonical ORDER BY for articles/docs in help center queries.
func (r *DocsHelpcenterRepository) hcDocOrderBy() string {
	if r.useSortKey {
		return "d.sort_key ASC, d.id ASC"
	}
	return "d.position ASC, d.created_at ASC"
}

// hcCollOrderBy returns the canonical ORDER BY for collections in help center queries.
func (r *DocsHelpcenterRepository) hcCollOrderBy() string {
	if r.useSortKey {
		return "sort_key ASC, id ASC"
	}
	return "position ASC, created_at ASC"
}

func normalizeDocsHelpcenterConfig(cfg *model.DocsHelpcenterConfig) {
	if cfg == nil {
		return
	}
	if cfg.DefaultLocale == "" {
		cfg.DefaultLocale = "en"
	}
	if len(cfg.EnabledLocales) == 0 {
		cfg.EnabledLocales = model.DocsStringArray{cfg.DefaultLocale}
	}
	if cfg.ProtectedTerms == nil {
		cfg.ProtectedTerms = model.DocsStringArray{}
	}
	if cfg.PublicURLMode == "" {
		if cfg.CustomDomain != nil && strings.TrimSpace(*cfg.CustomDomain) != "" {
			cfg.PublicURLMode = model.HelpcenterPublicURLModeCustomDomain
		} else {
			cfg.PublicURLMode = model.HelpcenterPublicURLModeHostedSubdomain
		}
	}
}

// ─── Config ─────────────────────────────────────────────────────────────────

// GetConfig returns the help center config for a workspace.
func (r *DocsHelpcenterRepository) GetConfig(ctx context.Context, workspaceID string) (*model.DocsHelpcenterConfig, error) {
	var cfg model.DocsHelpcenterConfig
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get helpcenter config: %w", err)
	}
	normalizeDocsHelpcenterConfig(&cfg)
	return &cfg, nil
}

// GetConfigBySubdomain returns the help center config by subdomain.
func (r *DocsHelpcenterRepository) GetConfigBySubdomain(ctx context.Context, subdomain string) (*model.DocsHelpcenterConfig, error) {
	var cfg model.DocsHelpcenterConfig
	if err := r.db.WithContext(ctx).Where("subdomain = ?", subdomain).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get helpcenter config by subdomain: %w", err)
	}
	normalizeDocsHelpcenterConfig(&cfg)
	return &cfg, nil
}

// GetConfigByCustomDomain returns the help center config by custom domain.
func (r *DocsHelpcenterRepository) GetConfigByCustomDomain(ctx context.Context, domain string) (*model.DocsHelpcenterConfig, error) {
	var cfg model.DocsHelpcenterConfig
	// Only a verified domain serves a help center.
	if err := r.db.WithContext(ctx).Where("custom_domain = ? AND custom_domain_status IN ?", domain, liveHelpcenterDomainStatuses).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get helpcenter config by custom domain: %w", err)
	}
	normalizeDocsHelpcenterConfig(&cfg)
	return &cfg, nil
}

// CustomDomainRegistered reports whether a help center serves the verified
// domain. It selects nothing but existence so the deny path stays a single
// hit on the partial unique index idx_docs_hc_config_custom_domain.
func (r *DocsHelpcenterRepository) CustomDomainRegistered(ctx context.Context, domain string) (bool, error) {
	var exists bool
	err := r.db.WithContext(ctx).
		Raw("SELECT EXISTS(SELECT 1 FROM docs_helpcenter_configs WHERE custom_domain = ? AND custom_domain_status IN ?)", domain, liveHelpcenterDomainStatuses).
		Scan(&exists).Error
	if err != nil {
		return false, fmt.Errorf("check helpcenter custom domain: %w", err)
	}
	return exists, nil
}

// UpsertConfig creates or updates the help center config.
func (r *DocsHelpcenterRepository) UpsertConfig(ctx context.Context, workspaceID string, updates map[string]interface{}) (*model.DocsHelpcenterConfig, error) {
	existing, err := r.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := r.db.WithContext(ctx).Model(&model.DocsHelpcenterConfig{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update helpcenter config: %w", err)
		}
		return r.GetConfig(ctx, workspaceID)
	}

	cfg := &model.DocsHelpcenterConfig{WorkspaceID: workspaceID}
	if v, ok := updates["subdomain"].(string); ok {
		cfg.Subdomain = v
	}
	if v, ok := updates["brand_name"].(string); ok {
		cfg.BrandName = v
	}
	if v, ok := updates["brand_color"].(string); ok {
		cfg.BrandColor = v
	}
	if v, ok := updates["custom_domain"].(*string); ok {
		cfg.CustomDomain = v
	}
	if v, ok := updates["public_url_mode"].(string); ok {
		cfg.PublicURLMode = v
	}
	if v, ok := updates["custom_domain_status"].(string); ok {
		cfg.CustomDomainStatus = &v
	}
	if v, ok := updates["custom_domain_token"].(string); ok {
		cfg.CustomDomainToken = &v
	}
	if v, ok := updates["custom_domain_verified_at"].(time.Time); ok {
		cfg.CustomDomainVerifiedAt = &v
	}
	if v, ok := updates["reverse_proxy_host"].(*string); ok {
		cfg.ReverseProxyHost = v
	}
	if v, ok := updates["reverse_proxy_base_path"].(*string); ok {
		cfg.ReverseProxyBasePath = v
	}
	if v, ok := updates["brand_logo_url"].(*string); ok {
		cfg.BrandLogoURL = v
	}
	if v, ok := updates["brand_logo_dark_url"].(*string); ok {
		cfg.BrandLogoDarkURL = v
	}
	if v, ok := updates["is_published"].(bool); ok {
		cfg.IsPublished = v
	}
	if v, ok := updates["chat_widget_enabled"].(bool); ok {
		cfg.ChatWidgetEnabled = v
	} else {
		cfg.ChatWidgetEnabled = true
	}
	if v, ok := updates["seo_title"].(*string); ok {
		cfg.SEOTitle = v
	}
	if v, ok := updates["seo_description"].(*string); ok {
		cfg.SEODescription = v
	}
	if v, ok := updates["og_title"].(string); ok {
		cfg.OGTitle = &v
	}
	if v, ok := updates["og_description"].(string); ok {
		cfg.OGDescription = &v
	}
	if v, ok := updates["og_image_url"].(string); ok {
		cfg.OGImageURL = &v
	}
	if v, ok := updates["og_image_alt"].(string); ok {
		cfg.OGImageAlt = &v
	}
	if v, ok := updates["support_email"].(*string); ok {
		cfg.SupportEmail = v
	}
	if v, ok := updates["favicon_url"].(*string); ok {
		cfg.FaviconURL = v
	}
	if v, ok := updates["theme_mode"].(string); ok {
		cfg.ThemeMode = v
	}
	if v, ok := updates["header_links"].(json.RawMessage); ok {
		cfg.HeaderLinks = v
	}
	if v, ok := updates["footer_config"].(json.RawMessage); ok {
		cfg.FooterConfig = v
	}
	if v, ok := updates["homepage_config"].(json.RawMessage); ok {
		cfg.HomepageConfig = v
	}
	if v, ok := updates["space_nav_config"].(json.RawMessage); ok {
		cfg.SpaceNavConfig = v
	}
	if v, ok := updates["search_placeholder"].(*string); ok {
		cfg.SearchPlaceholder = v
	}
	if v, ok := updates["default_locale"].(string); ok {
		cfg.DefaultLocale = v
	}
	if v, ok := updates["enabled_locales"].(model.DocsStringArray); ok {
		cfg.EnabledLocales = v
	}
	if v, ok := updates["protected_terms"].(model.DocsStringArray); ok {
		cfg.ProtectedTerms = v
	}
	if v, ok := updates["show_language_switcher"].(bool); ok {
		cfg.ShowLanguageSwitcher = v
	}
	if v, ok := updates["fallback_to_default_locale"].(bool); ok {
		cfg.FallbackToDefaultLocale = v
	}
	if cfg.DefaultLocale == "" {
		cfg.DefaultLocale = "en"
	}
	if len(cfg.EnabledLocales) == 0 {
		cfg.EnabledLocales = model.DocsStringArray{cfg.DefaultLocale}
	}
	if _, ok := updates["fallback_to_default_locale"]; !ok {
		cfg.FallbackToDefaultLocale = true
	}
	if err := r.db.WithContext(ctx).Create(cfg).Error; err != nil {
		return nil, fmt.Errorf("create helpcenter config: %w", err)
	}
	return cfg, nil
}

// ─── Articles ───────────────────────────────────────────────────────────────

// GetArticle returns the help center article extension for a document.
func (r *DocsHelpcenterRepository) GetArticle(ctx context.Context, documentID string) (*model.DocsHelpcenterArticle, error) {
	var art model.DocsHelpcenterArticle
	if err := r.db.WithContext(ctx).Where("document_id = ?", documentID).First(&art).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get helpcenter article: %w", err)
	}
	return &art, nil
}

// DeleteArticlesByDocumentIDs hard-deletes help center article extensions.
func (r *DocsHelpcenterRepository) DeleteArticlesByDocumentIDs(ctx context.Context, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Where("document_id IN ?", documentIDs).Delete(&model.DocsHelpcenterArticle{}).Error; err != nil {
		return fmt.Errorf("delete helpcenter articles by documents: %w", err)
	}
	return nil
}

// CountPublicArticlesByDocumentIDs returns how many documents are live in the public help center.
func (r *DocsHelpcenterRepository) CountPublicArticlesByDocumentIDs(ctx context.Context, documentIDs []string) (int, error) {
	if len(documentIDs) == 0 {
		return 0, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticle{}).
		Where("document_id IN ? AND public_published_at IS NOT NULL", documentIDs).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count public helpcenter articles: %w", err)
	}
	return int(count), nil
}

// CreateArticle creates a help center article extension.
func (r *DocsHelpcenterRepository) CreateArticle(ctx context.Context, art *model.DocsHelpcenterArticle) (*model.DocsHelpcenterArticle, error) {
	q := r.db.WithContext(ctx)
	if !r.useSortKey {
		q = q.Omit("sort_key")
	}
	if err := q.Create(art).Error; err != nil {
		return nil, fmt.Errorf("create helpcenter article: %w", err)
	}
	return art, nil
}

func (r *DocsHelpcenterRepository) UpdateArticleMetadata(ctx context.Context, documentID string, updates map[string]interface{}) (*model.DocsHelpcenterArticle, error) {
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticle{}).
		Where("document_id = ?", documentID).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update helpcenter article metadata: %w", err)
	}
	return r.GetArticle(ctx, documentID)
}

func (r *DocsHelpcenterRepository) PublicIDExists(ctx context.Context, publicID, excludeDocumentID string) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticle{}).
		Where("public_id = ?", publicID)
	if excludeDocumentID != "" {
		query = query.Where("document_id != ?", excludeDocumentID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("check helpcenter public id exists: %w", err)
	}
	return count > 0, nil
}

func (r *DocsHelpcenterRepository) SetPublicID(ctx context.Context, documentID, publicID string) error {
	if err := r.db.WithContext(ctx).Model(&model.DocsHelpcenterArticle{}).
		Where("document_id = ?", documentID).
		Update("public_id", publicID).Error; err != nil {
		return fmt.Errorf("set helpcenter public id: %w", err)
	}
	return nil
}

// SetPublicPublishedAt sets or clears the public_published_at timestamp.
func (r *DocsHelpcenterRepository) SetPublicPublishedAt(ctx context.Context, documentID string, publishedAt *time.Time) error {
	if err := r.db.WithContext(ctx).Model(&model.DocsHelpcenterArticle{}).
		Where("document_id = ?", documentID).
		Update("public_published_at", publishedAt).Error; err != nil {
		return fmt.Errorf("set public_published_at: %w", err)
	}
	return nil
}

// IncrementViewCount increments the view count for an article.
func (r *DocsHelpcenterRepository) IncrementViewCount(ctx context.Context, documentID string) error {
	if err := r.db.WithContext(ctx).Exec(
		"UPDATE docs_helpcenter_articles SET view_count = view_count + 1 WHERE document_id = ?", documentID,
	).Error; err != nil {
		return fmt.Errorf("increment view count: %w", err)
	}
	return nil
}

// IncrementFeedbackCount increments helpful or not_helpful count.
func (r *DocsHelpcenterRepository) IncrementFeedbackCount(ctx context.Context, documentID string, helpful bool) error {
	col := "not_helpful_count"
	if helpful {
		col = "helpful_count"
	}
	if err := r.db.WithContext(ctx).Exec(
		fmt.Sprintf("UPDATE docs_helpcenter_articles SET %s = %s + 1 WHERE document_id = ?", col, col), documentID,
	).Error; err != nil {
		return fmt.Errorf("increment feedback count: %w", err)
	}
	return nil
}

// ─── Public Help Center Queries ──────────────────────────────────────────────

func (r *DocsHelpcenterRepository) ListPublicSpaceTranslations(ctx context.Context, workspaceID, locale string) ([]model.DocsHelpcenterSpaceTranslation, error) {
	var translations []model.DocsHelpcenterSpaceTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_space_translations st").
		Select("st.*").
		Joins("JOIN docs_spaces s ON s.id = st.space_id").
		Where(`
			st.workspace_id = ?
			AND st.locale = ?
			AND st.status = ?
			AND st.published_at IS NOT NULL
			AND s.deleted_at IS NULL
			AND s.type = ?
		`, workspaceID, locale, model.DocsHelpcenterTranslationStatusPublished, model.SpaceTypeExternalCapable).
		Order("s.position ASC, s.created_at ASC").
		Scan(&translations).Error; err != nil {
		return nil, fmt.Errorf("list public space translations: %w", err)
	}
	return translations, nil
}

func (r *DocsHelpcenterRepository) GetPublicSpaceTranslationBySlug(ctx context.Context, workspaceID, locale, slug string) (*model.DocsHelpcenterSpaceTranslation, error) {
	var translation model.DocsHelpcenterSpaceTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_space_translations st").
		Select("st.*").
		Joins("JOIN docs_spaces s ON s.id = st.space_id").
		Where(`
			st.workspace_id = ?
			AND st.locale = ?
			AND st.slug = ?
			AND st.status = ?
			AND st.published_at IS NOT NULL
			AND s.deleted_at IS NULL
			AND s.type = ?
		`, workspaceID, locale, slug, model.DocsHelpcenterTranslationStatusPublished, model.SpaceTypeExternalCapable).
		First(&translation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get public space translation by slug: %w", err)
	}
	return &translation, nil
}

func (r *DocsHelpcenterRepository) ListPublicCollectionTranslations(ctx context.Context, spaceID, locale string) ([]model.DocsHelpcenterCollectionTranslation, error) {
	var translations []model.DocsHelpcenterCollectionTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_collection_translations ct").
		Select("ct.*").
		Joins("JOIN docs_collections c ON c.id = ct.collection_id").
		Where(`
			ct.space_id = ?
			AND ct.locale = ?
			AND ct.status = ?
			AND ct.published_at IS NOT NULL
			AND c.deleted_at IS NULL
		`, spaceID, locale, model.DocsHelpcenterTranslationStatusPublished).
		Order("c." + r.hcCollOrderBy()).
		Scan(&translations).Error; err != nil {
		return nil, fmt.Errorf("list public collection translations: %w", err)
	}
	return translations, nil
}

func (r *DocsHelpcenterRepository) GetPublicCollectionTranslationBySlug(ctx context.Context, spaceID, locale, slug string) (*model.DocsHelpcenterCollectionTranslation, error) {
	var translations []model.DocsHelpcenterCollectionTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_collection_translations ct").
		Select("ct.*").
		Joins("JOIN docs_collections c ON c.id = ct.collection_id").
		Where(`
			ct.space_id = ?
			AND ct.locale = ?
			AND ct.slug = ?
			AND ct.status = ?
			AND ct.published_at IS NOT NULL
			AND c.deleted_at IS NULL
		`, spaceID, locale, slug, model.DocsHelpcenterTranslationStatusPublished).
		Limit(2).Find(&translations).Error; err != nil {
		return nil, fmt.Errorf("get public collection translation by slug: %w", err)
	}
	if len(translations) != 1 {
		return nil, nil
	}
	return &translations[0], nil
}

func (r *DocsHelpcenterRepository) GetPublicCollectionTranslationByWorkspaceSlug(ctx context.Context, workspaceID, locale, slug string) (*model.DocsHelpcenterCollectionTranslation, error) {
	var translations []model.DocsHelpcenterCollectionTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_collection_translations ct").
		Select("ct.*").
		Joins("JOIN docs_collections c ON c.id = ct.collection_id").
		Joins("JOIN docs_spaces s ON s.id = ct.space_id").
		Where(`
			ct.workspace_id = ?
			AND ct.locale = ?
			AND ct.slug = ?
			AND ct.status = ?
			AND ct.published_at IS NOT NULL
			AND c.deleted_at IS NULL
			AND s.deleted_at IS NULL
			AND s.type = ?
		`, workspaceID, locale, slug, model.DocsHelpcenterTranslationStatusPublished, model.SpaceTypeExternalCapable).
		Limit(2).Find(&translations).Error; err != nil {
		return nil, fmt.Errorf("get public collection translation by workspace slug: %w", err)
	}
	if len(translations) != 1 {
		return nil, nil
	}
	return &translations[0], nil
}

func (r *DocsHelpcenterRepository) GetPublicCollectionTranslationByCollectionID(ctx context.Context, collectionID, locale string) (*model.DocsHelpcenterCollectionTranslation, error) {
	var translation model.DocsHelpcenterCollectionTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_collection_translations ct").
		Select("ct.*").
		Joins("JOIN docs_collections c ON c.id = ct.collection_id").
		Where(`
			ct.collection_id = ?
			AND ct.locale = ?
			AND ct.status = ?
			AND ct.published_at IS NOT NULL
			AND c.deleted_at IS NULL
		`, collectionID, locale, model.DocsHelpcenterTranslationStatusPublished).
		First(&translation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get public collection translation by collection id: %w", err)
	}
	return &translation, nil
}

func (r *DocsHelpcenterRepository) ListPublicArticleTranslationsBySpace(ctx context.Context, spaceID, locale string) ([]model.DocsHelpcenterArticleTranslation, error) {
	var translations []model.DocsHelpcenterArticleTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_translations hat").
		Select(`
			hat.id,
			hat.document_id,
			COALESCE(p.workspace_id, hat.workspace_id) AS workspace_id,
			COALESCE(p.space_id, hat.space_id) AS space_id,
			COALESCE(p.collection_id, hat.collection_id) AS collection_id,
			hat.locale,
			COALESCE(p.title, hat.title) AS title,
			COALESCE(p.slug, hat.slug) AS slug,
			COALESCE(p.excerpt, hat.excerpt) AS excerpt,
			COALESCE(p.content, hat.content) AS content,
			COALESCE(p.content_text, hat.content_text) AS content_text,
			COALESCE(p.seo_title, hat.seo_title) AS seo_title,
			COALESCE(p.seo_description, hat.seo_description) AS seo_description,
			CASE
				WHEN p.document_id IS NOT NULL AND hat.locale = cfg.default_locale THEN 'published'
				ELSE hat.status
			END AS status,
			hat.source_updated_at,
			hat.source_synced,
			hat.published_at,
			hat.view_count,
			hat.helpful_count,
			hat.not_helpful_count,
			hat.created_at,
			hat.updated_at
		`).
		Joins("JOIN docs_documents d ON d.id = hat.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = hat.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = hat.workspace_id").
		Joins("LEFT JOIN docs_helpcenter_article_publications p ON p.document_id = hat.document_id AND p.locale = hat.locale").
		Where(`
			hat.space_id = ?
			AND hat.locale = ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
			AND (
				(
					hat.status = ? AND hat.published_at IS NOT NULL
					AND (p.document_id IS NOT NULL OR hat.locale = cfg.default_locale)
				)
				OR (hat.locale = cfg.default_locale AND p.document_id IS NOT NULL)
			)
		`, spaceID, locale, model.DocsHelpcenterTranslationStatusPublished, model.DocStatusPublished).
		Order(r.hcDocOrderBy()).
		Scan(&translations).Error; err != nil {
		return nil, fmt.Errorf("list public article translations by space: %w", err)
	}
	return translations, nil
}

func (r *DocsHelpcenterRepository) GetPublicArticleTranslationBySlug(ctx context.Context, spaceID string, collectionID *string, locale, slug string) (*model.DocsHelpcenterArticleTranslation, error) {
	query := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_translations hat").
		Select(`
			hat.id,
			hat.document_id,
			COALESCE(p.workspace_id, hat.workspace_id) AS workspace_id,
			COALESCE(p.space_id, hat.space_id) AS space_id,
			COALESCE(p.collection_id, hat.collection_id) AS collection_id,
			hat.locale,
			COALESCE(p.title, hat.title) AS title,
			COALESCE(p.slug, hat.slug) AS slug,
			COALESCE(p.excerpt, hat.excerpt) AS excerpt,
			COALESCE(p.content, hat.content) AS content,
			COALESCE(p.content_text, hat.content_text) AS content_text,
			COALESCE(p.seo_title, hat.seo_title) AS seo_title,
			COALESCE(p.seo_description, hat.seo_description) AS seo_description,
			CASE
				WHEN p.document_id IS NOT NULL AND hat.locale = cfg.default_locale THEN 'published'
				ELSE hat.status
			END AS status,
			hat.source_updated_at,
			hat.source_synced,
			hat.published_at,
			hat.view_count,
			hat.helpful_count,
			hat.not_helpful_count,
			hat.created_at,
			hat.updated_at
		`).
		Joins("JOIN docs_documents d ON d.id = hat.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = hat.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = hat.workspace_id").
		Joins("LEFT JOIN docs_helpcenter_article_publications p ON p.document_id = hat.document_id AND p.locale = hat.locale").
		Where(`
			hat.space_id = ?
			AND hat.locale = ?
			AND COALESCE(p.slug, hat.slug) = ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
			AND (
				(
					hat.status = ? AND hat.published_at IS NOT NULL
					AND (p.document_id IS NOT NULL OR hat.locale = cfg.default_locale)
				)
				OR (hat.locale = cfg.default_locale AND p.document_id IS NOT NULL)
			)
		`, spaceID, locale, slug, model.DocsHelpcenterTranslationStatusPublished, model.DocStatusPublished)

	if collectionID != nil {
		query = query.Where("hat.collection_id = ?", *collectionID)
	}

	var translations []model.DocsHelpcenterArticleTranslation
	if err := query.Limit(2).Find(&translations).Error; err != nil {
		return nil, fmt.Errorf("get public article translation by slug: %w", err)
	}
	if len(translations) != 1 {
		return nil, nil
	}
	return &translations[0], nil
}

func (r *DocsHelpcenterRepository) GetPublicArticleTranslationByPublicID(ctx context.Context, workspaceID, locale, publicID string) (*model.DocsHelpcenterArticleTranslation, error) {
	var translation model.DocsHelpcenterArticleTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_translations hat").
		Select(`
			hat.id,
			hat.document_id,
			COALESCE(p.workspace_id, hat.workspace_id) AS workspace_id,
			COALESCE(p.space_id, hat.space_id) AS space_id,
			COALESCE(p.collection_id, hat.collection_id) AS collection_id,
			hat.locale,
			COALESCE(p.title, hat.title) AS title,
			COALESCE(p.slug, hat.slug) AS slug,
			COALESCE(p.excerpt, hat.excerpt) AS excerpt,
			COALESCE(p.content, hat.content) AS content,
			COALESCE(p.content_text, hat.content_text) AS content_text,
			COALESCE(p.seo_title, hat.seo_title) AS seo_title,
			COALESCE(p.seo_description, hat.seo_description) AS seo_description,
			CASE
				WHEN p.document_id IS NOT NULL AND hat.locale = cfg.default_locale THEN 'published'
				ELSE hat.status
			END AS status,
			hat.source_updated_at,
			hat.source_synced,
			hat.published_at,
			hat.view_count,
			hat.helpful_count,
			hat.not_helpful_count,
			hat.created_at,
			hat.updated_at
		`).
		Joins("JOIN docs_documents d ON d.id = hat.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = hat.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = hat.workspace_id").
		Joins("LEFT JOIN docs_helpcenter_article_publications p ON p.document_id = hat.document_id AND p.locale = hat.locale").
		Where(`
			hat.workspace_id = ?
			AND hat.locale = ?
			AND ha.public_id = ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
			AND (
				(
					hat.status = ? AND hat.published_at IS NOT NULL
					AND (p.document_id IS NOT NULL OR hat.locale = cfg.default_locale)
				)
				OR (hat.locale = cfg.default_locale AND p.document_id IS NOT NULL)
			)
		`, workspaceID, locale, publicID, model.DocStatusPublished, model.DocsHelpcenterTranslationStatusPublished).
		First(&translation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get public article translation by public id: %w", err)
	}
	return &translation, nil
}

func (r *DocsHelpcenterRepository) ListPublicArticleTranslationsByCollection(ctx context.Context, collectionID, locale string) ([]model.DocsHelpcenterArticleTranslation, error) {
	var translations []model.DocsHelpcenterArticleTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_translations hat").
		Select(`
			hat.id,
			hat.document_id,
			COALESCE(p.workspace_id, hat.workspace_id) AS workspace_id,
			COALESCE(p.space_id, hat.space_id) AS space_id,
			COALESCE(p.collection_id, hat.collection_id) AS collection_id,
			hat.locale,
			COALESCE(p.title, hat.title) AS title,
			COALESCE(p.slug, hat.slug) AS slug,
			COALESCE(p.excerpt, hat.excerpt) AS excerpt,
			COALESCE(p.content, hat.content) AS content,
			COALESCE(p.content_text, hat.content_text) AS content_text,
			COALESCE(p.seo_title, hat.seo_title) AS seo_title,
			COALESCE(p.seo_description, hat.seo_description) AS seo_description,
			CASE
				WHEN p.document_id IS NOT NULL AND hat.locale = cfg.default_locale THEN 'published'
				ELSE hat.status
			END AS status,
			hat.source_updated_at,
			hat.source_synced,
			hat.published_at,
			hat.view_count,
			hat.helpful_count,
			hat.not_helpful_count,
			hat.created_at,
			hat.updated_at
		`).
		Joins("JOIN docs_documents d ON d.id = hat.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = hat.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = hat.workspace_id").
		Joins("LEFT JOIN docs_helpcenter_article_publications p ON p.document_id = hat.document_id AND p.locale = hat.locale").
		Where(`
			hat.collection_id = ?
			AND hat.locale = ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
			AND (
				(
					hat.status = ? AND hat.published_at IS NOT NULL
					AND (p.document_id IS NOT NULL OR hat.locale = cfg.default_locale)
				)
				OR (hat.locale = cfg.default_locale AND p.document_id IS NOT NULL)
			)
		`, collectionID, locale, model.DocsHelpcenterTranslationStatusPublished, model.DocStatusPublished).
		Order(r.hcDocOrderBy()).
		Scan(&translations).Error; err != nil {
		return nil, fmt.Errorf("list public article translations by collection: %w", err)
	}
	return translations, nil
}

// ListPublicCollectionNavigationArticles returns the minimal published article
// projection needed by a public collection page. When fallbackLocale is set,
// the requested locale wins per document and the fallback fills only missing
// translations. Unlike the full translation query, this deliberately avoids
// loading article content and other detail-only fields.
func (r *DocsHelpcenterRepository) ListPublicCollectionNavigationArticles(
	ctx context.Context,
	collectionID string,
	requestedLocale string,
	fallbackLocale string,
) ([]model.PublicNavArticle, error) {
	locales := []string{requestedLocale}
	if fallbackLocale != "" && fallbackLocale != requestedLocale {
		locales = append(locales, fallbackLocale)
	}

	type navigationArticleRow struct {
		ID          string     `gorm:"column:id"`
		Locale      string     `gorm:"column:locale"`
		Title       string     `gorm:"column:title"`
		Slug        string     `gorm:"column:slug"`
		PublicID    string     `gorm:"column:public_id"`
		Position    int        `gorm:"column:position"`
		SortKey     string     `gorm:"column:sort_key"`
		PublishedAt *time.Time `gorm:"column:published_at"`
	}

	var rows []navigationArticleRow
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_translations hat").
		Select(`
			d.id,
			hat.locale,
			COALESCE(p.title, hat.title) AS title,
			COALESCE(p.slug, hat.slug) AS slug,
			ha.public_id,
			d.position,
			d.sort_key,
			hat.published_at
		`).
		Joins("JOIN docs_documents d ON d.id = hat.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = hat.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = hat.workspace_id").
		Joins("LEFT JOIN docs_helpcenter_article_publications p ON p.document_id = hat.document_id AND p.locale = hat.locale").
		Where(`
			hat.collection_id = ?
			AND hat.locale IN ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
			AND (
				(
					hat.status = ? AND hat.published_at IS NOT NULL
					AND (p.document_id IS NOT NULL OR hat.locale = cfg.default_locale)
				)
				OR (hat.locale = cfg.default_locale AND p.document_id IS NOT NULL)
			)
		`, collectionID, locales, model.DocStatusPublished, model.DocsHelpcenterTranslationStatusPublished).
		Order(r.hcDocOrderBy()).
		Order("hat.locale ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list public collection navigation articles: %w", err)
	}

	articlesByDocument := make(map[string]model.PublicNavArticle, len(rows))
	documentOrder := make([]string, 0, len(rows))
	selectedLocale := make(map[string]string, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Slug) == "" {
			continue
		}
		if _, exists := articlesByDocument[row.ID]; !exists {
			documentOrder = append(documentOrder, row.ID)
		}
		if selectedLocale[row.ID] == requestedLocale && row.Locale != requestedLocale {
			continue
		}
		var publishedAt *string
		if row.PublishedAt != nil {
			formatted := row.PublishedAt.Format(time.RFC3339)
			publishedAt = &formatted
		}
		articlesByDocument[row.ID] = model.PublicNavArticle{
			ID:          row.ID,
			Title:       row.Title,
			Slug:        row.Slug,
			PublicID:    row.PublicID,
			Position:    row.Position,
			SortKey:     row.SortKey,
			PublishedAt: publishedAt,
		}
		selectedLocale[row.ID] = row.Locale
	}

	articles := make([]model.PublicNavArticle, 0, len(articlesByDocument))
	for _, documentID := range documentOrder {
		if article, ok := articlesByDocument[documentID]; ok {
			articles = append(articles, article)
		}
	}
	return articles, nil
}

// ListPublicCollectionTranslationsByCollection returns every published locale
// variant for a collection that still belongs to a public docs space.
func (r *DocsHelpcenterRepository) ListPublicCollectionTranslationsByCollection(
	ctx context.Context,
	collectionID string,
) ([]model.DocsHelpcenterCollectionTranslation, error) {
	var translations []model.DocsHelpcenterCollectionTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_collection_translations ct").
		Select("ct.*").
		Joins("JOIN docs_collections c ON c.id = ct.collection_id").
		Joins("JOIN docs_spaces s ON s.id = ct.space_id").
		Where(`
			ct.collection_id = ?
			AND ct.status = ?
			AND ct.published_at IS NOT NULL
			AND c.deleted_at IS NULL
			AND s.deleted_at IS NULL
			AND s.type = ?
		`, collectionID, model.DocsHelpcenterTranslationStatusPublished, model.SpaceTypeExternalCapable).
		Order("ct.locale ASC").
		Scan(&translations).Error; err != nil {
		return nil, fmt.Errorf("list public collection translations by collection: %w", err)
	}
	return translations, nil
}
func (r *DocsHelpcenterRepository) GetPublicArticleTranslationByCollectionSlug(ctx context.Context, collectionID, locale, slug string) (*model.DocsHelpcenterArticleTranslation, error) {
	var translations []model.DocsHelpcenterArticleTranslation
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_translations hat").
		Select(`
			hat.id,
			hat.document_id,
			COALESCE(p.workspace_id, hat.workspace_id) AS workspace_id,
			COALESCE(p.space_id, hat.space_id) AS space_id,
			COALESCE(p.collection_id, hat.collection_id) AS collection_id,
			hat.locale,
			COALESCE(p.title, hat.title) AS title,
			COALESCE(p.slug, hat.slug) AS slug,
			COALESCE(p.excerpt, hat.excerpt) AS excerpt,
			COALESCE(p.content, hat.content) AS content,
			COALESCE(p.content_text, hat.content_text) AS content_text,
			COALESCE(p.seo_title, hat.seo_title) AS seo_title,
			COALESCE(p.seo_description, hat.seo_description) AS seo_description,
			CASE
				WHEN p.document_id IS NOT NULL AND hat.locale = cfg.default_locale THEN 'published'
				ELSE hat.status
			END AS status,
			hat.source_updated_at,
			hat.source_synced,
			hat.published_at,
			hat.view_count,
			hat.helpful_count,
			hat.not_helpful_count,
			hat.created_at,
			hat.updated_at
		`).
		Joins("JOIN docs_documents d ON d.id = hat.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = hat.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = hat.workspace_id").
		Joins("LEFT JOIN docs_helpcenter_article_publications p ON p.document_id = hat.document_id AND p.locale = hat.locale").
		Where(`
			hat.collection_id = ?
			AND hat.locale = ?
			AND COALESCE(p.slug, hat.slug) = ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
			AND (
				(
					hat.status = ? AND hat.published_at IS NOT NULL
					AND (p.document_id IS NOT NULL OR hat.locale = cfg.default_locale)
				)
				OR (hat.locale = cfg.default_locale AND p.document_id IS NOT NULL)
			)
		`, collectionID, locale, slug, model.DocsHelpcenterTranslationStatusPublished, model.DocStatusPublished).
		Limit(2).Find(&translations).Error; err != nil {
		return nil, fmt.Errorf("get public article translation by collection slug: %w", err)
	}
	if len(translations) != 1 {
		return nil, nil
	}
	return &translations[0], nil
}

type sourceArticleRow struct {
	model.DocsDocument
	HelpcenterArticleID string          `gorm:"column:helpcenter_article_id"`
	HelpcenterPublicID  string          `gorm:"column:helpcenter_public_id"`
	HelpcenterSlug      string          `gorm:"column:helpcenter_slug"`
	PublicationContent  json.RawMessage `gorm:"column:publication_content"`
	PublicationTitle    string          `gorm:"column:publication_title"`
	PublicationExcerpt  *string         `gorm:"column:publication_excerpt"`
	PublicationSEOTitle *string         `gorm:"column:publication_seo_title"`
	PublicationSEODesc  *string         `gorm:"column:publication_seo_description"`
	PublicationOGTitle  *string         `gorm:"column:publication_og_title"`
	PublicationOGDesc   *string         `gorm:"column:publication_og_description"`
	PublicationOGImage  *string         `gorm:"column:publication_og_image_url"`
	PublicationOGAlt    *string         `gorm:"column:publication_og_image_alt"`
	PublicPublishedAt   *time.Time      `gorm:"column:public_published_at"`
	HelpfulCount        int             `gorm:"column:helpful_count"`
	NotHelpfulCount     int             `gorm:"column:not_helpful_count"`
	ViewCount           int             `gorm:"column:view_count"`
}

func sourceArticleRowToModels(row sourceArticleRow) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent) {
	doc := row.DocsDocument
	doc.Title = row.PublicationTitle
	doc.Excerpt = row.PublicationExcerpt
	ha := &model.DocsHelpcenterArticle{
		ID:                row.HelpcenterArticleID,
		DocumentID:        row.ID,
		PublicID:          row.HelpcenterPublicID,
		Slug:              row.HelpcenterSlug,
		SEOTitle:          row.PublicationSEOTitle,
		SEODescription:    row.PublicationSEODesc,
		OGTitle:           row.PublicationOGTitle,
		OGDescription:     row.PublicationOGDesc,
		OGImageURL:        row.PublicationOGImage,
		OGImageAlt:        row.PublicationOGAlt,
		HelpfulCount:      row.HelpfulCount,
		NotHelpfulCount:   row.NotHelpfulCount,
		ViewCount:         row.ViewCount,
		PublicPublishedAt: row.PublicPublishedAt,
	}
	var content *model.DocsContent
	if len(row.PublicationContent) > 0 {
		content = &model.DocsContent{DocumentID: row.ID, Content: row.PublicationContent}
	}
	return &doc, ha, content
}

// ListSpaceNavigation returns collections with their published articles for sidebar navigation.
func (r *DocsHelpcenterRepository) ListSpaceNavigation(ctx context.Context, spaceID string) ([]model.PublicNavCollection, error) {
	// 1. Get collections in this space. The result is a flat list, but
	//    the ordering — (depth, parent, position) — keeps siblings
	//    contiguous and ancestors before descendants so callers can fold
	//    the response into a tree in a single pass.
	var collections []model.DocsCollection
	if err := r.db.WithContext(ctx).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Order("depth ASC, parent_collection_id ASC, " + r.hcCollOrderBy()).
		Find(&collections).Error; err != nil {
		return nil, fmt.Errorf("list space collections: %w", err)
	}

	// 2. Get all published articles in this space that are externally published.
	type navArticleRow struct {
		ID           string  `gorm:"column:id"`
		Title        string  `gorm:"column:title"`
		Slug         string  `gorm:"column:slug"`
		PublicID     string  `gorm:"column:public_id"`
		Position     int     `gorm:"column:position"`
		SortKey      string  `gorm:"column:sort_key"`
		CollectionID *string `gorm:"column:collection_id"`
	}
	orderBy := "d.collection_id, d.position ASC, d.created_at ASC"
	if r.useSortKey {
		orderBy = "d.collection_id, d.sort_key ASC, d.id ASC"
	}
	var articles []navArticleRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT d.id, d.title, ha.slug, ha.public_id, d.position, d.sort_key, d.collection_id
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE d.space_id = ?
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND ha.slug != ''
		ORDER BY `+orderBy+`
	`, spaceID).Scan(&articles).Error; err != nil {
		return nil, fmt.Errorf("list space nav articles: %w", err)
	}

	// 3. Group articles by collection_id.
	articlesByCollection := map[string][]model.PublicNavArticle{}
	var uncategorized []model.PublicNavArticle
	for _, a := range articles {
		na := model.PublicNavArticle{
			ID:       a.ID,
			Title:    a.Title,
			Slug:     a.Slug,
			PublicID: a.PublicID,
			Position: a.Position,
			SortKey:  a.SortKey,
		}
		if a.CollectionID != nil {
			articlesByCollection[*a.CollectionID] = append(articlesByCollection[*a.CollectionID], na)
		} else {
			uncategorized = append(uncategorized, na)
		}
	}

	// 4. Compute the set of collections that should appear in public nav:
	//    every collection with at least one direct published article,
	//    plus every ancestor of such a collection. A parent with no
	//    direct articles but a child that has some must still render as
	//    a container node so the tree is connected.
	collectionByID := make(map[string]model.DocsCollection, len(collections))
	for _, c := range collections {
		collectionByID[c.ID] = c
	}
	includedSet := make(map[string]bool, len(collections))
	for id := range articlesByCollection {
		cursor := id
		for {
			node, exists := collectionByID[cursor]
			if !exists || includedSet[cursor] {
				break
			}
			includedSet[cursor] = true
			if node.ParentCollectionID == nil {
				break
			}
			cursor = *node.ParentCollectionID
		}
	}

	// 5. Emit included collections in (depth, parent, position) order so
	//    the frontend can fold them into a tree without an extra sort.
	var result []model.PublicNavCollection
	for _, c := range collections {
		if !includedSet[c.ID] {
			continue
		}
		slug := c.Slug
		if slug == "" {
			slug = c.ID // legacy fallback for rows seeded before the tree migration
		}
		result = append(result, model.PublicNavCollection{
			ID:                 c.ID,
			Name:               c.Name,
			Slug:               slug,
			PublicID:           c.PublicID,
			Icon:               c.Icon,
			ParentCollectionID: c.ParentCollectionID,
			Depth:              c.Depth,
			Position:           c.Position,
			SortKey:            c.SortKey,
			Articles:           articlesByCollection[c.ID],
		})
	}

	// Uncategorized articles are intentionally excluded from public
	// navigation. The publish endpoint requires a collection for
	// external-capable spaces, so this branch should not be reached
	// in normal operation. If legacy data has published uncategorized
	// docs, they silently drop from the nav rather than showing as a
	// fake "General" collection.

	return result, nil
}

// DocsHelpcenterPublishedArticleSlug is a minimal projection of a
// published help center article returned by
// ListPublishedArticleSlugsInCollection.
type DocsHelpcenterPublishedArticleSlug struct {
	DocumentID string
	Slug       string
	PublicID   string
}

// ListPublishedArticleSlugsInCollection returns (document_id, slug) for
// every externally published help center article whose owning collection
// is the given collection. The slug is the canonical source slug stored
// on docs_helpcenter_articles, not a localized publication slug. Used by
// collection-rename redirect emission.
func (r *DocsHelpcenterRepository) ListPublishedArticleSlugsInCollection(ctx context.Context, collectionID string) ([]DocsHelpcenterPublishedArticleSlug, error) {
	type row struct {
		DocumentID string `gorm:"column:document_id"`
		Slug       string `gorm:"column:slug"`
		PublicID   string `gorm:"column:public_id"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Raw(`
		SELECT d.id AS document_id, ha.slug, ha.public_id
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE d.collection_id = ?
		  AND d.deleted_at IS NULL
		  AND d.status = 'published'
		  AND ha.public_published_at IS NOT NULL
		  AND ha.slug != ''
	`, collectionID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list published article slugs in collection: %w", err)
	}
	out := make([]DocsHelpcenterPublishedArticleSlug, len(rows))
	for i, r := range rows {
		out[i] = DocsHelpcenterPublishedArticleSlug{DocumentID: r.DocumentID, Slug: r.Slug, PublicID: r.PublicID}
	}
	return out, nil
}

// ListCollectionAncestors returns the ancestor chain of a collection
// ordered top-down (root first, immediate parent last). The chain does
// not include the collection itself. It is iterative and capped at the
// collection tree depth so a dangling parent reference cannot loop.
func (r *DocsHelpcenterRepository) ListCollectionAncestors(ctx context.Context, collectionID string) ([]model.DocsCollection, error) {
	var current model.DocsCollection
	if err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", collectionID).
		First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load collection for ancestor walk: %w", err)
	}

	const maxDepth = 3
	chain := make([]model.DocsCollection, 0, maxDepth)
	for i := 0; i < maxDepth; i++ {
		if current.ParentCollectionID == nil {
			break
		}
		var parent model.DocsCollection
		if err := r.db.WithContext(ctx).
			Where("id = ? AND deleted_at IS NULL", *current.ParentCollectionID).
			First(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			return nil, fmt.Errorf("load ancestor collection: %w", err)
		}
		chain = append([]model.DocsCollection{parent}, chain...)
		current = parent
	}
	return chain, nil
}

// ListPublicDocumentsBySpace returns internally published + externally published docs for a help-center space.
func (r *DocsHelpcenterRepository) ListPublicDocumentsBySpace(ctx context.Context, workspaceID, spaceID string) ([]model.DocsDocument, error) {
	var docs []model.DocsDocument
	if err := r.db.WithContext(ctx).
		Table("docs_documents d").
		Select("d.*").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = d.id").
		Where("d.workspace_id = ? AND d.space_id = ? AND d.status = 'published' AND d.deleted_at IS NULL AND ha.public_published_at IS NOT NULL", workspaceID, spaceID).
		Order("d.updated_at DESC").
		Scan(&docs).Error; err != nil {
		return nil, fmt.Errorf("list public docs by space: %w", err)
	}
	return docs, nil
}

// ListWidgetCollections returns widget help collections for a space,
// including tree metadata and per-collection direct article counts. The
// result is flat; the widget renderer folds it into a drilldown tree
// using ParentCollectionID.
//
// A collection is included when it has at least one directly published
// article OR is an ancestor of such a collection, so container nodes
// that hold content in descendants remain navigable.
func (r *DocsHelpcenterRepository) ListWidgetCollections(ctx context.Context, spaceID string) ([]model.WidgetHelpCollection, error) {
	var collections []model.DocsCollection
	if err := r.db.WithContext(ctx).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Order("depth ASC, parent_collection_id ASC, " + r.hcCollOrderBy()).
		Find(&collections).Error; err != nil {
		return nil, fmt.Errorf("list widget collections: %w", err)
	}

	// Direct article counts per collection.
	type articleCountRow struct {
		CollectionID string `gorm:"column:collection_id"`
		Count        int    `gorm:"column:count"`
	}
	var counts []articleCountRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT d.collection_id, COUNT(d.id) AS count
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE d.space_id = ?
		  AND d.collection_id IS NOT NULL
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND ha.slug != ''
		GROUP BY d.collection_id
	`, spaceID).Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("count widget articles per collection: %w", err)
	}
	countByID := make(map[string]int, len(counts))
	for _, c := range counts {
		countByID[c.CollectionID] = c.Count
	}

	collectionByID := make(map[string]model.DocsCollection, len(collections))
	for _, c := range collections {
		collectionByID[c.ID] = c
	}

	// Include every collection that has direct articles OR is an ancestor
	// of one that does.
	included := make(map[string]bool, len(collections))
	for id, n := range countByID {
		if n == 0 {
			continue
		}
		cursor := id
		for {
			node, exists := collectionByID[cursor]
			if !exists || included[cursor] {
				break
			}
			included[cursor] = true
			if node.ParentCollectionID == nil {
				break
			}
			cursor = *node.ParentCollectionID
		}
	}

	result := make([]model.WidgetHelpCollection, 0, len(collections)+1)
	for _, c := range collections {
		if !included[c.ID] {
			continue
		}
		slug := c.Slug
		if slug == "" {
			slug = c.ID
		}
		result = append(result, model.WidgetHelpCollection{
			ID:                 c.ID,
			Name:               c.Name,
			Slug:               slug,
			PublicID:           c.PublicID,
			Icon:               c.Icon,
			ParentCollectionID: c.ParentCollectionID,
			Depth:              c.Depth,
			ArticleCount:       countByID[c.ID],
		})
	}

	var uncategorizedCount int64
	if err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(d.id)
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE d.space_id = ?
		  AND d.collection_id IS NULL
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND ha.slug != ''
	`, spaceID).Scan(&uncategorizedCount).Error; err != nil {
		return nil, fmt.Errorf("count uncategorized widget articles: %w", err)
	}

	if uncategorizedCount > 0 {
		slug := fmt.Sprintf("uncategorized:%s", spaceID)
		result = append(result, model.WidgetHelpCollection{
			ID:           slug,
			Name:         "General",
			Slug:         slug,
			ArticleCount: int(uncategorizedCount),
		})
	}

	return result, nil
}

// ListWidgetArticlesByCollectionID returns externally published articles in a collection.
func (r *DocsHelpcenterRepository) ListWidgetArticlesByCollectionID(ctx context.Context, collectionID string) ([]model.WidgetHelpArticleSummary, error) {
	type articleRow struct {
		ID       string  `gorm:"column:id"`
		Title    string  `gorm:"column:title"`
		Excerpt  *string `gorm:"column:excerpt"`
		Slug     string  `gorm:"column:slug"`
		PublicID string  `gorm:"column:public_id"`
		Icon     *string `gorm:"column:icon"`
	}

	var rows []articleRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT d.id, p.title, p.excerpt, p.slug, ha.public_id, d.icon
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		JOIN docs_helpcenter_article_publications p ON p.document_id = d.id
		JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id
		WHERE d.collection_id = ?
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND p.locale = cfg.default_locale
		  AND p.slug != ''
		ORDER BY d.position ASC, d.created_at ASC
	`, collectionID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list widget articles by collection: %w", err)
	}

	result := make([]model.WidgetHelpArticleSummary, len(rows))
	for i, row := range rows {
		result[i] = model.WidgetHelpArticleSummary{
			ID:       row.ID,
			Title:    row.Title,
			Slug:     row.Slug,
			PublicID: row.PublicID,
			Excerpt:  row.Excerpt,
			Icon:     row.Icon,
		}
	}
	return result, nil
}

// ListWidgetArticlesBySpaceUncategorized returns externally published uncategorized articles in a space.
func (r *DocsHelpcenterRepository) ListWidgetArticlesBySpaceUncategorized(ctx context.Context, spaceID string) ([]model.WidgetHelpArticleSummary, error) {
	type articleRow struct {
		ID       string  `gorm:"column:id"`
		Title    string  `gorm:"column:title"`
		Excerpt  *string `gorm:"column:excerpt"`
		Slug     string  `gorm:"column:slug"`
		PublicID string  `gorm:"column:public_id"`
		Icon     *string `gorm:"column:icon"`
	}

	var rows []articleRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT d.id, p.title, p.excerpt, p.slug, ha.public_id, d.icon
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		JOIN docs_helpcenter_article_publications p ON p.document_id = d.id
		JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id
		WHERE d.space_id = ?
		  AND d.collection_id IS NULL
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND p.locale = cfg.default_locale
		  AND p.slug != ''
		ORDER BY d.position ASC, d.created_at ASC
	`, spaceID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list uncategorized widget articles: %w", err)
	}

	result := make([]model.WidgetHelpArticleSummary, len(rows))
	for i, row := range rows {
		result[i] = model.WidgetHelpArticleSummary{
			ID:       row.ID,
			Title:    row.Title,
			Slug:     row.Slug,
			PublicID: row.PublicID,
			Excerpt:  row.Excerpt,
			Icon:     row.Icon,
		}
	}
	return result, nil
}

// GetPublicArticleBySlug finds a publicly published article by space and slug.
func (r *DocsHelpcenterRepository) GetPublicArticleBySlug(ctx context.Context, spaceID, slug string) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent, error) {
	var rows []sourceArticleRow
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_publications p").
		Select(`
			d.*,
			ha.id AS helpcenter_article_id,
			ha.public_id AS helpcenter_public_id,
			p.slug AS helpcenter_slug,
			p.content AS publication_content,
			p.title AS publication_title,
			p.excerpt AS publication_excerpt,
			p.seo_title AS publication_seo_title,
			p.seo_description AS publication_seo_description,
			p.og_title AS publication_og_title,
			p.og_description AS publication_og_description,
			p.og_image_url AS publication_og_image_url,
			p.og_image_alt AS publication_og_image_alt,
			ha.public_published_at,
			ha.helpful_count,
			ha.not_helpful_count,
			ha.view_count
		`).
		Joins("JOIN docs_documents d ON d.id = p.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id").
		Where("d.space_id = ? AND p.slug = ? AND p.locale = cfg.default_locale AND d.status = 'published' AND d.deleted_at IS NULL AND ha.public_published_at IS NOT NULL",
			spaceID, slug).
		Limit(2).Find(&rows).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("get public article by slug: %w", err)
	}
	if len(rows) != 1 {
		return nil, nil, nil, nil
	}
	doc, ha, content := sourceArticleRowToModels(rows[0])
	return doc, ha, content, nil
}

// GetPublicArticleByDocumentIDInSpaces finds a public article by slug or document ID constrained to allowed spaces.
func (r *DocsHelpcenterRepository) GetPublicArticleByDocumentIDInSpaces(ctx context.Context, spaceIDs []string, slugOrID string) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent, error) {
	if len(spaceIDs) == 0 {
		return nil, nil, nil, nil
	}

	// Determine whether the caller passed a UUID (document ID) or a slug.
	isUUID := len(slugOrID) == 36 && slugOrID[8] == '-' && slugOrID[13] == '-'
	matchClause := "p.slug = ?"
	if isUUID {
		matchClause = "p.document_id = ?"
	}

	var rows []sourceArticleRow
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_publications p").
		Select(`
			d.*,
			ha.id AS helpcenter_article_id,
			ha.public_id AS helpcenter_public_id,
			p.slug AS helpcenter_slug,
			p.content AS publication_content,
			p.title AS publication_title,
			p.excerpt AS publication_excerpt,
			p.seo_title AS publication_seo_title,
			p.seo_description AS publication_seo_description,
			p.og_title AS publication_og_title,
			p.og_description AS publication_og_description,
			p.og_image_url AS publication_og_image_url,
			p.og_image_alt AS publication_og_image_alt,
			ha.public_published_at,
			ha.helpful_count,
			ha.not_helpful_count,
			ha.view_count
		`).
		Joins("JOIN docs_documents d ON d.id = p.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id").
		Where(`
			`+matchClause+`
			AND d.space_id IN ?
			AND p.locale = cfg.default_locale
			AND d.status = 'published'
			AND d.deleted_at IS NULL
			AND ha.public_published_at IS NOT NULL
		`, slugOrID, spaceIDs).
		Limit(2).
		Find(&rows).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("get public article by document id: %w", err)
	}
	if len(rows) != 1 {
		return nil, nil, nil, nil // not found or ambiguous
	}
	doc, ha, content := sourceArticleRowToModels(rows[0])
	return doc, ha, content, nil
}

// GetPublicArticleByPublicIDInSpaces finds a public article by canonical public ID constrained to allowed spaces.
func (r *DocsHelpcenterRepository) GetPublicArticleByPublicIDInSpaces(ctx context.Context, spaceIDs []string, publicID string) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent, error) {
	if len(spaceIDs) == 0 {
		return nil, nil, nil, nil
	}

	var row sourceArticleRow
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_publications p").
		Select(`
			d.*,
			ha.id AS helpcenter_article_id,
			ha.public_id AS helpcenter_public_id,
			p.slug AS helpcenter_slug,
			p.content AS publication_content,
			p.title AS publication_title,
			p.excerpt AS publication_excerpt,
			p.seo_title AS publication_seo_title,
			p.seo_description AS publication_seo_description,
			p.og_title AS publication_og_title,
			p.og_description AS publication_og_description,
			p.og_image_url AS publication_og_image_url,
			p.og_image_alt AS publication_og_image_alt,
			ha.public_published_at,
			ha.helpful_count,
			ha.not_helpful_count,
			ha.view_count
		`).
		Joins("JOIN docs_documents d ON d.id = p.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id").
		Where(`
			ha.public_id = ?
			AND d.space_id IN ?
			AND p.locale = cfg.default_locale
			AND d.status = 'published'
			AND d.deleted_at IS NULL
			AND ha.public_published_at IS NOT NULL
		`, publicID, spaceIDs).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("get public article by public id in spaces: %w", err)
	}
	doc, ha, content := sourceArticleRowToModels(row)
	return doc, ha, content, nil
}

func (r *DocsHelpcenterRepository) GetPublicArticleByPublicID(ctx context.Context, workspaceID, publicID string) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent, error) {
	var row sourceArticleRow
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_publications p").
		Select(`
			d.*,
			ha.id AS helpcenter_article_id,
			ha.public_id AS helpcenter_public_id,
			p.slug AS helpcenter_slug,
			p.content AS publication_content,
			p.title AS publication_title,
			p.excerpt AS publication_excerpt,
			p.seo_title AS publication_seo_title,
			p.seo_description AS publication_seo_description,
			p.og_title AS publication_og_title,
			p.og_description AS publication_og_description,
			p.og_image_url AS publication_og_image_url,
			p.og_image_alt AS publication_og_image_alt,
			ha.public_published_at,
			ha.helpful_count,
			ha.not_helpful_count,
			ha.view_count
		`).
		Joins("JOIN docs_documents d ON d.id = p.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id").
		Where(`
			d.workspace_id = ?
			AND ha.public_id = ?
			AND p.locale = cfg.default_locale
			AND d.status = 'published'
			AND d.deleted_at IS NULL
			AND ha.public_published_at IS NOT NULL
		`, workspaceID, publicID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("get public article by public id: %w", err)
	}
	doc, ha, content := sourceArticleRowToModels(row)
	return doc, ha, content, nil
}

// GetPublicArticleByCollectionSlug finds a publicly published article by collection slug and article slug.
func (r *DocsHelpcenterRepository) GetPublicArticleByCollectionSlug(ctx context.Context, workspaceID, collectionSlug, articleSlug string) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent, error) {
	var rows []sourceArticleRow
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_publications p").
		Select(`
			d.*,
			ha.id AS helpcenter_article_id,
			ha.public_id AS helpcenter_public_id,
			p.slug AS helpcenter_slug,
			p.content AS publication_content,
			p.title AS publication_title,
			p.excerpt AS publication_excerpt,
			p.seo_title AS publication_seo_title,
			p.seo_description AS publication_seo_description,
			p.og_title AS publication_og_title,
			p.og_description AS publication_og_description,
			p.og_image_url AS publication_og_image_url,
			p.og_image_alt AS publication_og_image_alt,
			ha.public_published_at,
			ha.helpful_count,
			ha.not_helpful_count,
			ha.view_count
		`).
		Joins("JOIN docs_documents d ON d.id = p.document_id").
		Joins("JOIN docs_collections c ON c.id = d.collection_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id").
		Where("c.workspace_id = ? AND c.slug = ? AND p.slug = ? AND p.locale = cfg.default_locale AND d.status = 'published' AND d.deleted_at IS NULL AND ha.public_published_at IS NOT NULL",
			workspaceID, collectionSlug, articleSlug).
		Limit(2).Find(&rows).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("get public article by collection slug: %w", err)
	}
	if len(rows) != 1 {
		return nil, nil, nil, nil
	}
	doc, ha, content := sourceArticleRowToModels(rows[0])
	return doc, ha, content, nil
}

func (r *DocsHelpcenterRepository) GetPublicArticleByCollectionIDAndSlug(ctx context.Context, collectionID, articleSlug string) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent, error) {
	var row sourceArticleRow
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_publications p").
		Select(`
			d.*,
			ha.id AS helpcenter_article_id,
			ha.public_id AS helpcenter_public_id,
			p.slug AS helpcenter_slug,
			p.content AS publication_content,
			p.title AS publication_title,
			p.excerpt AS publication_excerpt,
			p.seo_title AS publication_seo_title,
			p.seo_description AS publication_seo_description,
			p.og_title AS publication_og_title,
			p.og_description AS publication_og_description,
			p.og_image_url AS publication_og_image_url,
			p.og_image_alt AS publication_og_image_alt,
			ha.public_published_at,
			ha.helpful_count,
			ha.not_helpful_count,
			ha.view_count
		`).
		Joins("JOIN docs_documents d ON d.id = p.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id").
		Where("d.collection_id = ? AND p.slug = ? AND p.locale = cfg.default_locale AND d.status = 'published' AND d.deleted_at IS NULL AND ha.public_published_at IS NOT NULL",
			collectionID, articleSlug).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("get public article by collection id and slug: %w", err)
	}
	doc, ha, content := sourceArticleRowToModels(row)
	return doc, ha, content, nil
}

// GetPublicCollectionBySlug returns a collection and its published articles by workspace and collection slug.
func (r *DocsHelpcenterRepository) GetPublicCollectionBySlug(ctx context.Context, workspaceID, collectionSlug string) (*model.DocsCollection, []model.PublicNavArticle, error) {
	var colls []model.DocsCollection
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND slug = ? AND deleted_at IS NULL", workspaceID, collectionSlug).
		Limit(2).Find(&colls).Error; err != nil {
		return nil, nil, fmt.Errorf("get public collection by slug: %w", err)
	}
	if len(colls) != 1 {
		return nil, nil, nil
	}

	articles, err := r.ListPublicCollectionArticles(ctx, colls[0].ID)
	if err != nil {
		return nil, nil, err
	}

	return &colls[0], articles, nil
}

// ListPublicCollectionArticles returns published article nav rows for a collection.
func (r *DocsHelpcenterRepository) ListPublicCollectionArticles(ctx context.Context, collectionID string) ([]model.PublicNavArticle, error) {
	type navArticleRow struct {
		ID       string `gorm:"column:id"`
		Title    string `gorm:"column:title"`
		Slug     string `gorm:"column:slug"`
		PublicID string `gorm:"column:public_id"`
	}
	var rows []navArticleRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT d.id, p.title, p.slug, ha.public_id
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		JOIN docs_helpcenter_article_publications p ON p.document_id = d.id
		JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id
		WHERE d.collection_id = ?
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND p.locale = cfg.default_locale
		  AND p.slug != ''
		ORDER BY d.position ASC, d.created_at ASC
	`, collectionID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list public collection articles: %w", err)
	}

	articles := make([]model.PublicNavArticle, len(rows))
	for i, row := range rows {
		articles[i] = model.PublicNavArticle{
			ID:       row.ID,
			Title:    row.Title,
			Slug:     row.Slug,
			PublicID: row.PublicID,
		}
	}

	return articles, nil
}

func (r *DocsHelpcenterRepository) IncrementTranslatedViewCount(ctx context.Context, documentID, locale string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticleTranslation{}).
		Where("document_id = ? AND locale = ?", documentID, locale).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
		return fmt.Errorf("increment translated view count: %w", err)
	}
	return nil
}

func (r *DocsHelpcenterRepository) IncrementTranslatedFeedbackCount(ctx context.Context, documentID, locale string, helpful bool) error {
	column := "not_helpful_count"
	if helpful {
		column = "helpful_count"
	}
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticleTranslation{}).
		Where("document_id = ? AND locale = ?", documentID, locale).
		UpdateColumn(column, gorm.Expr(column+" + 1")).Error; err != nil {
		return fmt.Errorf("increment translated feedback count: %w", err)
	}
	return nil
}

// SetSlug updates the public slug for a helpcenter article.
func (r *DocsHelpcenterRepository) SetSlug(ctx context.Context, documentID, slug string) error {
	if err := r.db.WithContext(ctx).Model(&model.DocsHelpcenterArticle{}).
		Where("document_id = ?", documentID).
		Update("slug", slug).Error; err != nil {
		return fmt.Errorf("set helpcenter article slug: %w", err)
	}
	return nil
}

// SlugExistsInSpace checks if a slug is already used by another published article in the same space.
// Returns true if the slug is taken by a different document.
func (r *DocsHelpcenterRepository) SlugExistsInSpace(ctx context.Context, spaceID, slug, excludeDocumentID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticle{}).
		Joins("JOIN docs_documents d ON d.id = docs_helpcenter_articles.document_id").
		Where("d.space_id = ? AND docs_helpcenter_articles.slug = ? AND docs_helpcenter_articles.document_id != ? AND d.deleted_at IS NULL",
			spaceID, slug, excludeDocumentID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check slug exists: %w", err)
	}
	return count > 0, nil
}

// ─── Slug Aliases ───────────────────────────────────────────────────────────

// CreateSlugAlias inserts a slug alias record.
func (r *DocsHelpcenterRepository) CreateSlugAlias(ctx context.Context, alias *model.DocsSlugAlias) error {
	if err := r.db.WithContext(ctx).Create(alias).Error; err != nil {
		return fmt.Errorf("create slug alias: %w", err)
	}
	return nil
}

// FindBySlug finds a document ID by slug, checking both docs_documents slug field
// (via the caller) and the alias table.
func (r *DocsHelpcenterRepository) FindAliasBySlug(ctx context.Context, workspaceID, slug string) (*model.DocsSlugAlias, error) {
	var alias model.DocsSlugAlias
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND old_slug = ?", workspaceID, slug).First(&alias).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find slug alias: %w", err)
	}
	return &alias, nil
}

// ─── Feedback ───────────────────────────────────────────────────────────────

// CreateFeedback inserts article feedback.
func (r *DocsHelpcenterRepository) CreateFeedback(ctx context.Context, fb *model.DocsArticleFeedback) (*model.DocsArticleFeedback, error) {
	if err := r.db.WithContext(ctx).Create(fb).Error; err != nil {
		return nil, fmt.Errorf("create article feedback: %w", err)
	}
	return fb, nil
}
