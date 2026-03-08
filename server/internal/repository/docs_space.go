package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsSpaceRepository handles DB operations for docs spaces.
type DocsSpaceRepository struct {
	db *gorm.DB
}

// NewDocsSpaceRepository creates a new DocsSpaceRepository.
func NewDocsSpaceRepository(db *gorm.DB) *DocsSpaceRepository {
	return &DocsSpaceRepository{db: db}
}

// Create inserts a new space.
func (r *DocsSpaceRepository) Create(ctx context.Context, space *model.DocsSpace) (*model.DocsSpace, error) {
	if err := r.db.WithContext(ctx).Create(space).Error; err != nil {
		return nil, fmt.Errorf("create docs space: %w", err)
	}
	return space, nil
}

// GetByID returns a space by ID (excluding soft-deleted).
func (r *DocsSpaceRepository) GetByID(ctx context.Context, id string) (*model.DocsSpace, error) {
	var space model.DocsSpace
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&space).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs space: %w", err)
	}
	return &space, nil
}

// GetBySlug returns a space by workspace_id + slug.
func (r *DocsSpaceRepository) GetBySlug(ctx context.Context, workspaceID, slug string) (*model.DocsSpace, error) {
	var space model.DocsSpace
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND slug = ? AND deleted_at IS NULL", workspaceID, slug).First(&space).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs space by slug: %w", err)
	}
	return &space, nil
}

// ListByWorkspace returns all spaces for a workspace.
func (r *DocsSpaceRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.DocsSpace, error) {
	var spaces []model.DocsSpace
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID).
		Order("position ASC, created_at ASC").
		Find(&spaces).Error; err != nil {
		return nil, fmt.Errorf("list docs spaces: %w", err)
	}
	return spaces, nil
}

// ListByTeam returns spaces owned by a specific team.
func (r *DocsSpaceRepository) ListByTeam(ctx context.Context, workspaceID, teamID string) ([]model.DocsSpace, error) {
	var spaces []model.DocsSpace
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND team_id = ? AND deleted_at IS NULL", workspaceID, teamID).
		Order("position ASC, created_at ASC").
		Find(&spaces).Error; err != nil {
		return nil, fmt.Errorf("list docs spaces by team: %w", err)
	}
	return spaces, nil
}

// Update applies partial updates to a space.
func (r *DocsSpaceRepository) Update(ctx context.Context, id string, updates map[string]interface{}) (*model.DocsSpace, error) {
	if err := r.db.WithContext(ctx).Model(&model.DocsSpace{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update docs space: %w", err)
	}
	return r.GetByID(ctx, id)
}

// Delete soft-deletes a space.
func (r *DocsSpaceRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Exec("UPDATE docs_spaces SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL", id).Error; err != nil {
		return fmt.Errorf("delete docs space: %w", err)
	}
	return nil
}

// Restore un-deletes a space.
func (r *DocsSpaceRepository) Restore(ctx context.Context, id string) (*model.DocsSpace, error) {
	if err := r.db.WithContext(ctx).Exec("UPDATE docs_spaces SET deleted_at = NULL WHERE id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("restore docs space: %w", err)
	}
	return r.GetByID(ctx, id)
}
