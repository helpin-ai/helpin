package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportContentPageRepository stores crawled content pages.
type SupportContentPageRepository struct {
	db *gorm.DB
}

func NewSupportContentPageRepository(db *gorm.DB) *SupportContentPageRepository {
	return &SupportContentPageRepository{db: db}
}

func (r *SupportContentPageRepository) Upsert(ctx context.Context, page *model.SupportContentPage) (*model.SupportContentPage, error) {
	var existing model.SupportContentPage
	err := r.db.WithContext(ctx).
		Where("content_source_id = ? AND url = ?", page.ContentSourceID, page.URL).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := r.db.WithContext(ctx).Create(page).Error; err != nil {
			return nil, fmt.Errorf("create content page: %w", err)
		}
		return page, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get content page for upsert: %w", err)
	}

	updates := map[string]any{
		"title":           page.Title,
		"http_status":     page.HTTPStatus,
		"content_format":  page.ContentFormat,
		"content_text":    page.ContentText,
		"content_hash":    page.ContentHash,
		"metadata":        page.Metadata,
		"last_crawled_at": page.LastCrawledAt,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportContentPage{}).
		Where("id = ?", existing.ID).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update content page: %w", err)
	}
	return r.GetByID(ctx, existing.ID)
}

func (r *SupportContentPageRepository) GetByID(ctx context.Context, id string) (*model.SupportContentPage, error) {
	var page model.SupportContentPage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get content page: %w", err)
	}
	return &page, nil
}

func (r *SupportContentPageRepository) DeleteByContentSourceID(ctx context.Context, contentSourceID string) error {
	if err := r.db.WithContext(ctx).
		Where("content_source_id = ?", contentSourceID).
		Delete(&model.SupportContentPage{}).Error; err != nil {
		return fmt.Errorf("delete content pages by source: %w", err)
	}
	return nil
}

// ListByContentSourceID returns all pages for a content source, ordered by
// last_crawled_at DESC. The heavy content_text column is excluded.
func (r *SupportContentPageRepository) ListByContentSourceID(ctx context.Context, contentSourceID string) ([]model.SupportContentPage, error) {
	var pages []model.SupportContentPage
	if err := r.db.WithContext(ctx).
		Select("id", "workspace_id", "content_source_id", "url", "title", "http_status", "content_format", "content_hash", "LENGTH(content_text) AS content_length", "last_crawled_at", "created_at", "updated_at").
		Where("content_source_id = ?", contentSourceID).
		Order("last_crawled_at DESC").
		Find(&pages).Error; err != nil {
		return nil, fmt.Errorf("list content pages by source: %w", err)
	}
	return pages, nil
}

// ListIndexedByContentSourceID excludes crawled pages without searchable chunks.
func (r *SupportContentPageRepository) ListIndexedByContentSourceID(ctx context.Context, workspaceID, contentSourceID string) ([]model.SupportContentPage, error) {
	pages := []model.SupportContentPage{}
	err := r.db.WithContext(ctx).Table("support_content_pages AS p").
		Select("p.id, p.workspace_id, p.content_source_id, p.url, p.title, p.http_status, p.content_format, p.content_hash, LENGTH(p.content_text) AS content_length, p.last_crawled_at, p.created_at, p.updated_at").
		Where("p.workspace_id = ? AND p.content_source_id = ?", workspaceID, contentSourceID).
		Where("p.http_status >= 200 AND p.http_status < 300").
		Where(`EXISTS (SELECT 1 FROM support_content_chunks c WHERE c.page_id = p.id AND c.workspace_id = p.workspace_id AND c.content_source_id = p.content_source_id)`).
		Order("LOWER(p.title), p.id").Find(&pages).Error
	if err != nil {
		return nil, fmt.Errorf("list indexed content pages: %w", err)
	}
	return pages, nil
}

// ListByContentSourceIDWithContent returns all pages for a content source
// including the content_text column. Use this for reindex operations that need
// to re-chunk and re-embed existing content without re-crawling.
func (r *SupportContentPageRepository) ListByContentSourceIDWithContent(ctx context.Context, contentSourceID string) ([]model.SupportContentPage, error) {
	var pages []model.SupportContentPage
	if err := r.db.WithContext(ctx).
		Where("content_source_id = ?", contentSourceID).
		Order("last_crawled_at DESC").
		Find(&pages).Error; err != nil {
		return nil, fmt.Errorf("list content pages with content: %w", err)
	}
	return pages, nil
}

// GetByIDWithContent returns a single page including its content_text.
func (r *SupportContentPageRepository) GetByIDWithContent(ctx context.Context, id string) (*model.SupportContentPage, error) {
	var page model.SupportContentPage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get content page with content: %w", err)
	}
	return &page, nil
}

func (r *SupportContentPageRepository) DeleteByContentSourceExceptURLs(ctx context.Context, workspaceID, contentSourceID string, keepURLs []string) error {
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND content_source_id = ?", workspaceID, contentSourceID)
	if len(keepURLs) > 0 {
		query = query.Where("url NOT IN ?", keepURLs)
	}
	if err := query.Delete(&model.SupportContentPage{}).Error; err != nil {
		return fmt.Errorf("delete stale content pages: %w", err)
	}
	return nil
}
