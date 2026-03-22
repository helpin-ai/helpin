package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportContentSourceRepository handles CRUD for crawled support content sources.
type SupportContentSourceRepository struct {
	db *gorm.DB
}

func NewSupportContentSourceRepository(db *gorm.DB) *SupportContentSourceRepository {
	return &SupportContentSourceRepository{db: db}
}

func (r *SupportContentSourceRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.SupportContentSource, error) {
	var sources []model.SupportContentSource
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("created_at ASC").
		Find(&sources).Error; err != nil {
		return nil, fmt.Errorf("list content sources: %w", err)
	}
	return sources, nil
}

func (r *SupportContentSourceRepository) GetByID(ctx context.Context, id string) (*model.SupportContentSource, error) {
	var source model.SupportContentSource
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&source).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get content source: %w", err)
	}
	return &source, nil
}

func (r *SupportContentSourceRepository) Create(ctx context.Context, source *model.SupportContentSource) error {
	if err := r.db.WithContext(ctx).Create(source).Error; err != nil {
		return fmt.Errorf("create content source: %w", err)
	}
	return nil
}

func (r *SupportContentSourceRepository) Update(ctx context.Context, id string, updates map[string]any) (*model.SupportContentSource, error) {
	if err := r.db.WithContext(ctx).
		Model(&model.SupportContentSource{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update content source: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *SupportContentSourceRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.SupportContentSource{}).Error; err != nil {
		return fmt.Errorf("delete content source: %w", err)
	}
	return nil
}

func (r *SupportContentSourceRepository) UpdateSyncState(
	ctx context.Context,
	id string,
	status string,
	progress int,
	pageCount int,
	chunkCount int,
	errMessage *string,
	jobID *string,
	startedAt *time.Time,
	completedAt *time.Time,
) error {
	updates := map[string]any{
		"sync_status":            status,
		"sync_progress":          progress,
		"indexed_pages":          pageCount,
		"indexed_chunks":         chunkCount,
		"last_sync_error":        errMessage,
		"last_crawl_job_id":      jobID,
		"last_sync_started_at":   startedAt,
		"last_sync_completed_at": completedAt,
		"updated_at":             time.Now(),
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportContentSource{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update content source sync state: %w", err)
	}
	return nil
}

func (r *SupportContentSourceRepository) MarkSyncQueued(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.SupportContentSource{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"sync_status":     model.KnowledgeSourceSyncQueued,
			"sync_progress":   0,
			"last_sync_error": nil,
			"updated_at":      time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("mark content source queued: %w", err)
	}
	return nil
}
