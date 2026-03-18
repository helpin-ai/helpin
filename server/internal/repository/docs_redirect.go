package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsRedirectRepository handles DB operations for docs URL redirects.
type DocsRedirectRepository struct {
	db *gorm.DB
}

// NewDocsRedirectRepository creates a new DocsRedirectRepository.
func NewDocsRedirectRepository(db *gorm.DB) *DocsRedirectRepository {
	return &DocsRedirectRepository{db: db}
}

// Create inserts a single redirect record.
func (r *DocsRedirectRepository) Create(ctx context.Context, redirect *model.DocsRedirect) error {
	if err := r.db.WithContext(ctx).Create(redirect).Error; err != nil {
		return fmt.Errorf("create docs redirect: %w", err)
	}
	return nil
}

// BulkCreate inserts multiple redirect records, skipping duplicates on
// (workspace_id, source_path) conflicts.
func (r *DocsRedirectRepository) BulkCreate(ctx context.Context, redirects []model.DocsRedirect) error {
	if len(redirects) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&redirects).Error; err != nil {
		return fmt.Errorf("bulk create docs redirects: %w", err)
	}
	return nil
}

// GetBySourcePath returns the redirect matching the given workspace and source
// path. Returns nil (no error) when no record is found.
func (r *DocsRedirectRepository) GetBySourcePath(ctx context.Context, workspaceID, sourcePath string) (*model.DocsRedirect, error) {
	var redirect model.DocsRedirect
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND source_path = ?", workspaceID, sourcePath).
		First(&redirect).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs redirect by source path: %w", err)
	}
	return &redirect, nil
}

// List returns a paginated list of redirects for a workspace with optional
// search and type filters. Returns the items and total matching count.
func (r *DocsRedirectRepository) List(ctx context.Context, workspaceID string, filter model.DocsRedirectFilter) ([]model.DocsRedirect, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	q := r.db.WithContext(ctx).Model(&model.DocsRedirect{}).Where("workspace_id = ?", workspaceID)

	if filter.Search != "" {
		q = q.Where("source_path ILIKE ?", "%"+filter.Search+"%")
	}
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count docs redirects: %w", err)
	}

	var items []model.DocsRedirect
	if err := q.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list docs redirects: %w", err)
	}

	return items, total, nil
}

// Delete removes a redirect by ID.
func (r *DocsRedirectRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.DocsRedirect{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete docs redirect: %w", err)
	}
	return nil
}
