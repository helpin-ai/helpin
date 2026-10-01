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

// ForEachAutoSyncSource visits idle websites in bounded batches. Active sources
// and sources already attempted since the last daily slot are left alone.
func (r *SupportContentSourceRepository) ForEachAutoSyncSource(ctx context.Context, since time.Time, visit func(model.SupportContentSource) error) error {
	var sources []model.SupportContentSource
	err := r.db.WithContext(ctx).
		Where("source_type = ?", model.ContentSourceTypeWebsite).
		Where("sync_status NOT IN ?", []string{model.KnowledgeSourceSyncQueued, model.KnowledgeSourceSyncRunning, model.KnowledgeSourceSyncDisabled}).
		Where("last_sync_started_at IS NULL OR last_sync_started_at < ?", since).
		FindInBatches(&sources, 100, func(_ *gorm.DB, _ int) error {
			for _, source := range sources {
				if err := visit(source); err != nil {
					return err
				}
			}
			return nil
		}).Error
	if err != nil {
		return fmt.Errorf("visit auto-sync websites: %w", err)
	}
	return nil
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

// ClaimAutoSync atomically queues an idle source so a racing manual refresh is
// not queued again by the daily dispatcher.
func (r *SupportContentSourceRepository) ClaimAutoSync(ctx context.Context, id string, since time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.SupportContentSource{}).
		Where("id = ? AND source_type = ?", id, model.ContentSourceTypeWebsite).
		Where("sync_status NOT IN ?", []string{model.KnowledgeSourceSyncQueued, model.KnowledgeSourceSyncRunning, model.KnowledgeSourceSyncDisabled}).
		Where("last_sync_started_at IS NULL OR last_sync_started_at < ?", since).
		Updates(map[string]any{"sync_status": model.KnowledgeSourceSyncQueued, "sync_progress": 0, "last_sync_error": nil, "updated_at": time.Now()})
	if result.Error != nil {
		return false, fmt.Errorf("claim website auto-sync: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// UpdateSyncWarning records non-fatal crawl policy skips separately from errors.
func (r *SupportContentSourceRepository) UpdateSyncWarning(ctx context.Context, id string, warning *string) error {
	if err := r.db.WithContext(ctx).Model(&model.SupportContentSource{}).Where("id = ?", id).Update("last_sync_warning", warning).Error; err != nil {
		return fmt.Errorf("update content source sync warning: %w", err)
	}
	return nil
}
