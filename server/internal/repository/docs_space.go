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

// SetTeamIDs replaces the team associations for a space.
func (r *DocsSpaceRepository) SetTeamIDs(ctx context.Context, spaceID string, teamIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("space_id = ?", spaceID).Delete(&model.DocsSpaceTeam{}).Error; err != nil {
			return fmt.Errorf("clear space teams: %w", err)
		}
		for _, tid := range teamIDs {
			st := model.DocsSpaceTeam{SpaceID: spaceID, TeamID: tid}
			if err := tx.Create(&st).Error; err != nil {
				return fmt.Errorf("add space team: %w", err)
			}
		}
		return nil
	})
}

// GetTeamIDs returns the team IDs associated with a space.
func (r *DocsSpaceRepository) GetTeamIDs(ctx context.Context, spaceID string) ([]string, error) {
	var teams []model.DocsSpaceTeam
	if err := r.db.WithContext(ctx).Where("space_id = ?", spaceID).Find(&teams).Error; err != nil {
		return nil, fmt.Errorf("get space teams: %w", err)
	}
	ids := make([]string, len(teams))
	for i, t := range teams {
		ids[i] = t.TeamID
	}
	return ids, nil
}

// GetTeamIDsForSpaces returns a map of space_id → team_ids for multiple spaces.
func (r *DocsSpaceRepository) GetTeamIDsForSpaces(ctx context.Context, spaceIDs []string) (map[string][]string, error) {
	if len(spaceIDs) == 0 {
		return map[string][]string{}, nil
	}
	var teams []model.DocsSpaceTeam
	if err := r.db.WithContext(ctx).Where("space_id IN ?", spaceIDs).Find(&teams).Error; err != nil {
		return nil, fmt.Errorf("get space teams batch: %w", err)
	}
	result := make(map[string][]string)
	for _, t := range teams {
		result[t.SpaceID] = append(result[t.SpaceID], t.TeamID)
	}
	return result, nil
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

// UpdateSlug changes the slug of a space.
func (r *DocsSpaceRepository) UpdateSlug(ctx context.Context, id, slug string) error {
	if err := r.db.WithContext(ctx).Model(&model.DocsSpace{}).Where("id = ? AND deleted_at IS NULL", id).Update("slug", slug).Error; err != nil {
		return fmt.Errorf("update docs space slug: %w", err)
	}
	return nil
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

// ListPublicByWorkspace returns external-capable spaces for the public help center.
func (r *DocsSpaceRepository) ListPublicByWorkspace(ctx context.Context, workspaceID string) ([]model.DocsSpace, error) {
	var spaces []model.DocsSpace
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND type = ? AND deleted_at IS NULL", workspaceID, model.SpaceTypeExternalCapable).
		Order("position ASC, created_at ASC").
		Find(&spaces).Error; err != nil {
		return nil, fmt.Errorf("list public docs spaces: %w", err)
	}
	return spaces, nil
}

// AccessibleSpaceIDs returns the IDs of spaces a user can access based on team memberships.
// workspace_wide spaces are accessible to everyone.
// team_only spaces are accessible if the user is a member of at least one associated team.
func (r *DocsSpaceRepository) AccessibleSpaceIDs(ctx context.Context, workspaceID string, teamIDs []string) ([]string, error) {
	// workspace_wide spaces are always accessible
	var ids []string
	query := r.db.WithContext(ctx).
		Model(&model.DocsSpace{}).
		Select("id").
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID).
		Where("visibility = ? OR (visibility = ? AND id IN (SELECT space_id FROM docs_space_teams WHERE team_id IN ?))",
			model.SpaceVisibilityWorkspaceWide,
			model.SpaceVisibilityTeamOnly,
			teamIDs,
		)

	if len(teamIDs) == 0 {
		// No team memberships — only workspace_wide spaces
		query = r.db.WithContext(ctx).
			Model(&model.DocsSpace{}).
			Select("id").
			Where("workspace_id = ? AND deleted_at IS NULL AND visibility = ?", workspaceID, model.SpaceVisibilityWorkspaceWide)
	}

	if err := query.Find(&ids).Error; err != nil {
		return nil, fmt.Errorf("accessible space ids: %w", err)
	}
	return ids, nil
}

// NextPosition returns the next position for a space in the given section.
func (r *DocsSpaceRepository) NextPosition(ctx context.Context, workspaceID, section string) (int, error) {
	if err := r.NormalizeSection(ctx, workspaceID, section); err != nil {
		return 0, err
	}
	var maxPos *int
	err := r.db.WithContext(ctx).
		Model(&model.DocsSpace{}).
		Where("workspace_id = ? AND type = ? AND deleted_at IS NULL", workspaceID, section).
		Select("COALESCE(MAX(position), -1)").
		Scan(&maxPos).Error
	if err != nil {
		return 0, fmt.Errorf("next space position: %w", err)
	}
	if maxPos == nil {
		return 0, nil
	}
	return *maxPos + 1, nil
}

// Reorder sets contiguous positions for the given space IDs within a section.
func (r *DocsSpaceRepository) Reorder(ctx context.Context, workspaceID, section string, orderedIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range orderedIDs {
			if err := tx.Model(&model.DocsSpace{}).
				Where("id = ? AND workspace_id = ? AND type = ? AND deleted_at IS NULL", id, workspaceID, section).
				UpdateColumn("position", i).Error; err != nil {
				return fmt.Errorf("reorder space %s: %w", id, err)
			}
		}
		return nil
	})
}

// NormalizeSection re-numbers positions in a workspace/type bucket to be contiguous starting from 0.
func (r *DocsSpaceRepository) NormalizeSection(ctx context.Context, workspaceID, section string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return normalizeSpaceSectionTx(tx, workspaceID, section)
	})
}

func normalizeSpaceSectionTx(tx *gorm.DB, workspaceID, section string) error {
	var spaces []model.DocsSpace
	if err := tx.
		Where("workspace_id = ? AND type = ? AND deleted_at IS NULL", workspaceID, section).
		Order("position ASC, created_at ASC, id ASC").
		Find(&spaces).Error; err != nil {
		return fmt.Errorf("list space section for normalization: %w", err)
	}

	for i, space := range spaces {
		if space.Position == i {
			continue
		}
		if err := tx.Model(&model.DocsSpace{}).
			Where("id = ? AND deleted_at IS NULL", space.ID).
			UpdateColumn("position", i).Error; err != nil {
			return fmt.Errorf("normalize space %s: %w", space.ID, err)
		}
	}

	return nil
}
