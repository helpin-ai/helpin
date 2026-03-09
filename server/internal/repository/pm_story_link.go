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
		Where("workspace_id = ? AND source_story_id = ? AND target_story_id = ? AND link_type = ?",
			link.WorkspaceID, link.SourceStoryID, link.TargetStoryID, link.LinkType).
		First(&existing).Error
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("check story link: %w", err)
	}

	if err := r.db.WithContext(ctx).Create(link).Error; err != nil {
		return fmt.Errorf("create story link: %w", err)
	}
	return nil
}

// ListByStory returns links where the story is either the source or target.
func (r *PMStoryLinkRepository) ListByStory(ctx context.Context, workspaceID, storyID string) ([]model.PMStoryLink, error) {
	var links []model.PMStoryLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND (source_story_id = ? OR target_story_id = ?)", workspaceID, storyID, storyID).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list story links: %w", err)
	}
	return links, nil
}
