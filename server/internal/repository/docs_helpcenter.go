package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsHelpcenterRepository handles DB operations for help center config, articles, slugs, and feedback.
type DocsHelpcenterRepository struct {
	db *gorm.DB
}

// NewDocsHelpcenterRepository creates a new DocsHelpcenterRepository.
func NewDocsHelpcenterRepository(db *gorm.DB) *DocsHelpcenterRepository {
	return &DocsHelpcenterRepository{db: db}
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
	return &cfg, nil
}

// GetConfigByCustomDomain returns the help center config by custom domain.
func (r *DocsHelpcenterRepository) GetConfigByCustomDomain(ctx context.Context, domain string) (*model.DocsHelpcenterConfig, error) {
	var cfg model.DocsHelpcenterConfig
	if err := r.db.WithContext(ctx).Where("custom_domain = ?", domain).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get helpcenter config by custom domain: %w", err)
	}
	return &cfg, nil
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
	if v, ok := updates["brand_logo_url"].(*string); ok {
		cfg.BrandLogoURL = v
	}
	if v, ok := updates["brand_logo_dark_url"].(*string); ok {
		cfg.BrandLogoDarkURL = v
	}
	if v, ok := updates["is_published"].(bool); ok {
		cfg.IsPublished = v
	}
	if v, ok := updates["seo_title"].(*string); ok {
		cfg.SEOTitle = v
	}
	if v, ok := updates["seo_description"].(*string); ok {
		cfg.SEODescription = v
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

// CreateArticle creates a help center article extension.
func (r *DocsHelpcenterRepository) CreateArticle(ctx context.Context, art *model.DocsHelpcenterArticle) (*model.DocsHelpcenterArticle, error) {
	if err := r.db.WithContext(ctx).Create(art).Error; err != nil {
		return nil, fmt.Errorf("create helpcenter article: %w", err)
	}
	return art, nil
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

// ListSpaceNavigation returns collections with their published articles for sidebar navigation.
func (r *DocsHelpcenterRepository) ListSpaceNavigation(ctx context.Context, spaceID string) ([]model.PublicNavCollection, error) {
	// 1. Get collections in this space, ordered by position.
	var collections []model.DocsCollection
	if err := r.db.WithContext(ctx).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Order("position ASC, created_at ASC").
		Find(&collections).Error; err != nil {
		return nil, fmt.Errorf("list space collections: %w", err)
	}

	// 2. Get all published articles in this space that are externally published.
	type navArticleRow struct {
		ID           string  `gorm:"column:id"`
		Title        string  `gorm:"column:title"`
		Slug         string  `gorm:"column:slug"`
		CollectionID *string `gorm:"column:collection_id"`
	}
	var articles []navArticleRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT d.id, d.title, ha.slug, d.collection_id
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE d.space_id = ?
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND ha.slug != ''
		ORDER BY d.collection_id, d.position ASC, d.created_at ASC
	`, spaceID).Scan(&articles).Error; err != nil {
		return nil, fmt.Errorf("list space nav articles: %w", err)
	}

	// 3. Group articles by collection_id.
	articlesByCollection := map[string][]model.PublicNavArticle{}
	var uncategorized []model.PublicNavArticle
	for _, a := range articles {
		na := model.PublicNavArticle{ID: a.ID, Title: a.Title, Slug: a.Slug}
		if a.CollectionID != nil {
			articlesByCollection[*a.CollectionID] = append(articlesByCollection[*a.CollectionID], na)
		} else {
			uncategorized = append(uncategorized, na)
		}
	}

	// 4. Build response — only include collections that have published articles.
	var result []model.PublicNavCollection
	for _, c := range collections {
		arts, ok := articlesByCollection[c.ID]
		if !ok || len(arts) == 0 {
			continue
		}
		result = append(result, model.PublicNavCollection{
			ID:       c.ID,
			Name:     c.Name,
			Slug:     c.ID, // collections don't have slugs; use ID as identifier
			Icon:     c.Icon,
			Articles: arts,
		})
	}

	// Add uncategorized articles if any.
	if len(uncategorized) > 0 {
		result = append(result, model.PublicNavCollection{
			ID:       "uncategorized",
			Name:     "General",
			Slug:     "uncategorized",
			Articles: uncategorized,
		})
	}

	return result, nil
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

// ListWidgetCollections returns widget help collections for a space, including article counts.
func (r *DocsHelpcenterRepository) ListWidgetCollections(ctx context.Context, spaceID string) ([]model.WidgetHelpCollection, error) {
	type collectionRow struct {
		ID           string  `gorm:"column:id"`
		Name         string  `gorm:"column:name"`
		Icon         *string `gorm:"column:icon"`
		ArticleCount int     `gorm:"column:article_count"`
	}

	var rows []collectionRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT c.id, c.name, c.icon, COUNT(d.id) AS article_count
		FROM docs_collections c
		JOIN docs_documents d ON d.collection_id = c.id
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE c.space_id = ?
		  AND c.deleted_at IS NULL
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND ha.slug != ''
		GROUP BY c.id, c.name, c.icon, c.position, c.created_at
		ORDER BY c.position ASC, c.created_at ASC
	`, spaceID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list widget collections: %w", err)
	}

	result := make([]model.WidgetHelpCollection, 0, len(rows)+1)
	for _, row := range rows {
		result = append(result, model.WidgetHelpCollection{
			ID:           row.ID,
			Name:         row.Name,
			Slug:         row.ID,
			Icon:         row.Icon,
			ArticleCount: row.ArticleCount,
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
		ID      string  `gorm:"column:id"`
		Title   string  `gorm:"column:title"`
		Excerpt *string `gorm:"column:excerpt"`
		Icon    *string `gorm:"column:icon"`
	}

	var rows []articleRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT d.id, d.title, d.excerpt, d.icon
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE d.collection_id = ?
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND ha.slug != ''
		ORDER BY d.position ASC, d.created_at ASC
	`, collectionID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list widget articles by collection: %w", err)
	}

	result := make([]model.WidgetHelpArticleSummary, len(rows))
	for i, row := range rows {
		result[i] = model.WidgetHelpArticleSummary{
			ID:      row.ID,
			Title:   row.Title,
			Slug:    row.ID,
			Excerpt: row.Excerpt,
			Icon:    row.Icon,
		}
	}
	return result, nil
}

// ListWidgetArticlesBySpaceUncategorized returns externally published uncategorized articles in a space.
func (r *DocsHelpcenterRepository) ListWidgetArticlesBySpaceUncategorized(ctx context.Context, spaceID string) ([]model.WidgetHelpArticleSummary, error) {
	type articleRow struct {
		ID      string  `gorm:"column:id"`
		Title   string  `gorm:"column:title"`
		Excerpt *string `gorm:"column:excerpt"`
		Icon    *string `gorm:"column:icon"`
	}

	var rows []articleRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT d.id, d.title, d.excerpt, d.icon
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE d.space_id = ?
		  AND d.collection_id IS NULL
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND ha.slug != ''
		ORDER BY d.position ASC, d.created_at ASC
	`, spaceID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list uncategorized widget articles: %w", err)
	}

	result := make([]model.WidgetHelpArticleSummary, len(rows))
	for i, row := range rows {
		result[i] = model.WidgetHelpArticleSummary{
			ID:      row.ID,
			Title:   row.Title,
			Slug:    row.ID,
			Excerpt: row.Excerpt,
			Icon:    row.Icon,
		}
	}
	return result, nil
}

// GetPublicArticleBySlug finds a publicly published article by space and slug.
func (r *DocsHelpcenterRepository) GetPublicArticleBySlug(ctx context.Context, spaceID, slug string) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent, error) {
	// Find the helpcenter article by slug.
	var ha model.DocsHelpcenterArticle
	if err := r.db.WithContext(ctx).
		Joins("JOIN docs_documents d ON d.id = docs_helpcenter_articles.document_id").
		Where("d.space_id = ? AND docs_helpcenter_articles.slug = ? AND d.status = 'published' AND d.deleted_at IS NULL AND docs_helpcenter_articles.public_published_at IS NOT NULL",
			spaceID, slug).
		First(&ha).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("get public article by slug: %w", err)
	}

	// Get the document.
	var doc model.DocsDocument
	if err := r.db.WithContext(ctx).Where("id = ?", ha.DocumentID).First(&doc).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("get article document: %w", err)
	}

	// Get the content.
	var content model.DocsContent
	if err := r.db.WithContext(ctx).Where("document_id = ?", ha.DocumentID).First(&content).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, fmt.Errorf("get article content: %w", err)
		}
		return &doc, &ha, nil, nil
	}

	return &doc, &ha, &content, nil
}

// GetPublicArticleByDocumentIDInSpaces finds a public article by document ID constrained to allowed spaces.
func (r *DocsHelpcenterRepository) GetPublicArticleByDocumentIDInSpaces(ctx context.Context, spaceIDs []string, documentID string) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent, error) {
	if len(spaceIDs) == 0 {
		return nil, nil, nil, nil
	}

	var ha model.DocsHelpcenterArticle
	if err := r.db.WithContext(ctx).
		Joins("JOIN docs_documents d ON d.id = docs_helpcenter_articles.document_id").
		Where(`
			docs_helpcenter_articles.document_id = ?
			AND d.space_id IN ?
			AND d.status = 'published'
			AND d.deleted_at IS NULL
			AND docs_helpcenter_articles.public_published_at IS NOT NULL
		`, documentID, spaceIDs).
		First(&ha).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("get public article by document id: %w", err)
	}

	var doc model.DocsDocument
	if err := r.db.WithContext(ctx).Where("id = ?", ha.DocumentID).First(&doc).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("get article document: %w", err)
	}

	var content model.DocsContent
	if err := r.db.WithContext(ctx).Where("document_id = ?", ha.DocumentID).First(&content).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &doc, &ha, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("get article content: %w", err)
	}

	return &doc, &ha, &content, nil
}

// GetPublicArticleByCollectionSlug finds a publicly published article by collection slug and article slug.
func (r *DocsHelpcenterRepository) GetPublicArticleByCollectionSlug(ctx context.Context, workspaceID, collectionSlug, articleSlug string) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent, error) {
	var ha model.DocsHelpcenterArticle
	if err := r.db.WithContext(ctx).
		Joins("JOIN docs_documents d ON d.id = docs_helpcenter_articles.document_id").
		Joins("JOIN docs_collections c ON c.id = d.collection_id").
		Where("c.workspace_id = ? AND c.slug = ? AND docs_helpcenter_articles.slug = ? AND d.status = 'published' AND d.deleted_at IS NULL AND docs_helpcenter_articles.public_published_at IS NOT NULL",
			workspaceID, collectionSlug, articleSlug).
		First(&ha).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("get public article by collection slug: %w", err)
	}

	var doc model.DocsDocument
	if err := r.db.WithContext(ctx).Where("id = ?", ha.DocumentID).First(&doc).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("get article document: %w", err)
	}

	var content model.DocsContent
	if err := r.db.WithContext(ctx).Where("document_id = ?", ha.DocumentID).First(&content).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, fmt.Errorf("get article content: %w", err)
		}
		return &doc, &ha, nil, nil
	}

	return &doc, &ha, &content, nil
}

// GetPublicCollectionBySlug returns a collection and its published articles by workspace and collection slug.
func (r *DocsHelpcenterRepository) GetPublicCollectionBySlug(ctx context.Context, workspaceID, collectionSlug string) (*model.DocsCollection, []model.PublicNavArticle, error) {
	var coll model.DocsCollection
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND slug = ? AND deleted_at IS NULL", workspaceID, collectionSlug).
		First(&coll).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("get public collection by slug: %w", err)
	}

	type navArticleRow struct {
		ID    string `gorm:"column:id"`
		Title string `gorm:"column:title"`
		Slug  string `gorm:"column:slug"`
	}
	var rows []navArticleRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT d.id, d.title, ha.slug
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE d.collection_id = ?
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND ha.slug != ''
		ORDER BY d.position ASC, d.created_at ASC
	`, coll.ID).Scan(&rows).Error; err != nil {
		return nil, nil, fmt.Errorf("list public collection articles: %w", err)
	}

	articles := make([]model.PublicNavArticle, len(rows))
	for i, row := range rows {
		articles[i] = model.PublicNavArticle{ID: row.ID, Title: row.Title, Slug: row.Slug}
	}

	return &coll, articles, nil
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
