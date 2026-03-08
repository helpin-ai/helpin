package repository

import (
	"context"
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
