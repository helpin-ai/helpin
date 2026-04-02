package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMStoryLinkRepository handles DB operations for story dependency links.
type PMStoryLinkRepository struct {
	db *gorm.DB
}

// NewPMStoryLinkRepository creates a new PMStoryLinkRepository.
func NewPMStoryLinkRepository(db *gorm.DB) *PMStoryLinkRepository {
	return &PMStoryLinkRepository{db: db}
}

// Create inserts a new story link if it does not already exist.
func (r *PMStoryLinkRepository) Create(ctx context.Context, link *model.PMStoryLink) error {
	var existing model.PMStoryLink
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND source_task_id = ? AND target_task_id = ? AND link_type = ?",
			link.WorkspaceID, link.SourceStoryID, link.TargetStoryID, link.LinkType).
		First(&existing).Error
	switch {
	case err == nil:
		link.ID = existing.ID
		link.CreatedAt = existing.CreatedAt
		link.UpdatedAt = existing.UpdatedAt
		return nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("check story link: %w", err)
	}

	if err := r.db.WithContext(ctx).Create(link).Error; err != nil {
		return fmt.Errorf("create story link: %w", err)
	}
	return nil
}

// ListByStory returns links where the task is either the source or target.
func (r *PMStoryLinkRepository) ListByStory(ctx context.Context, workspaceID, storyID string) ([]model.PMStoryLink, error) {
	var links []model.PMStoryLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND (source_task_id = ? OR target_task_id = ?)", workspaceID, storyID, storyID).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list story links: %w", err)
	}
	return links, nil
}

// ListByStories returns links where either endpoint belongs to the provided task set.
func (r *PMStoryLinkRepository) ListByStories(ctx context.Context, workspaceID string, storyIDs []string) ([]model.PMStoryLink, error) {
	if len(storyIDs) == 0 {
		return []model.PMStoryLink{}, nil
	}

	var links []model.PMStoryLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND (source_task_id IN ? OR target_task_id IN ?)", workspaceID, storyIDs, storyIDs).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list story links by stories: %w", err)
	}
	return links, nil
}

// ListByWorkspaceAndType returns all links of a given type for a workspace.
func (r *PMStoryLinkRepository) ListByWorkspaceAndType(ctx context.Context, workspaceID, linkType string) ([]model.PMStoryLink, error) {
	var links []model.PMStoryLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND link_type = ?", workspaceID, linkType).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list story links by workspace/type: %w", err)
	}
	return links, nil
}

// GetByID returns a story relationship by ID.
func (r *PMStoryLinkRepository) GetByID(ctx context.Context, id string) (*model.PMStoryLink, error) {
	var link model.PMStoryLink
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get story link: %w", err)
	}
	return &link, nil
}

// Delete removes a story relationship by ID.
func (r *PMStoryLinkRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.PMStoryLink{}).Error; err != nil {
		return fmt.Errorf("delete story link: %w", err)
	}
	return nil
}
