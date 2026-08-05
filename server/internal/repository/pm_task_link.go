package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMTaskLinkRepository handles DB operations for task dependency links.
type PMTaskLinkRepository struct {
	db *gorm.DB
}

// NewPMTaskLinkRepository creates a new PMTaskLinkRepository.
func NewPMTaskLinkRepository(db *gorm.DB) *PMTaskLinkRepository {
	return &PMTaskLinkRepository{db: db}
}

// WithTransaction runs a batch of task-link mutations atomically.
func (r *PMTaskLinkRepository) WithTransaction(ctx context.Context, fn func(*PMTaskLinkRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&PMTaskLinkRepository{db: tx})
	})
}

// Create inserts a new task link if it does not already exist.
func (r *PMTaskLinkRepository) Create(ctx context.Context, link *model.PMTaskLink) error {
	var existing model.PMTaskLink
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND source_task_id = ? AND target_task_id = ? AND link_type = ?",
			link.WorkspaceID, link.SourceTaskID, link.TargetTaskID, link.LinkType).
		First(&existing).Error
	switch {
	case err == nil:
		link.ID = existing.ID
		link.CreatedAt = existing.CreatedAt
		link.UpdatedAt = existing.UpdatedAt
		return nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("check task link: %w", err)
	}

	if err := r.db.WithContext(ctx).Create(link).Error; err != nil {
		return fmt.Errorf("create task link: %w", err)
	}
	return nil
}

// ListByTask returns links where the task is either the source or target.
func (r *PMTaskLinkRepository) ListByTask(ctx context.Context, workspaceID, taskID string) ([]model.PMTaskLink, error) {
	var links []model.PMTaskLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND (source_task_id = ? OR target_task_id = ?)", workspaceID, taskID, taskID).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list task links: %w", err)
	}
	return links, nil
}

// ListByTasks returns links where either endpoint belongs to the provided task set.
func (r *PMTaskLinkRepository) ListByTasks(ctx context.Context, workspaceID string, taskIDs []string) ([]model.PMTaskLink, error) {
	if len(taskIDs) == 0 {
		return []model.PMTaskLink{}, nil
	}

	var links []model.PMTaskLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND (source_task_id IN ? OR target_task_id IN ?)", workspaceID, taskIDs, taskIDs).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list task links by tasks: %w", err)
	}
	return links, nil
}

// ListByWorkspaceAndType returns all links of a given type for a workspace.
func (r *PMTaskLinkRepository) ListByWorkspaceAndType(ctx context.Context, workspaceID, linkType string) ([]model.PMTaskLink, error) {
	var links []model.PMTaskLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND link_type = ?", workspaceID, linkType).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list task links by type: %w", err)
	}
	return links, nil
}

// GetByID returns a single task link by ID.
func (r *PMTaskLinkRepository) GetByID(ctx context.Context, id string) (*model.PMTaskLink, error) {
	var link model.PMTaskLink
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get task link: %w", err)
	}
	return &link, nil
}

// Delete removes a task link by ID.
func (r *PMTaskLinkRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.PMTaskLink{}).Error; err != nil {
		return fmt.Errorf("delete task link: %w", err)
	}
	return nil
}
