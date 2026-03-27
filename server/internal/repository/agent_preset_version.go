package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type WorkspaceAgentPresetVersionRepository struct {
	db *gorm.DB
}

func NewWorkspaceAgentPresetVersionRepository(db *gorm.DB) *WorkspaceAgentPresetVersionRepository {
	return &WorkspaceAgentPresetVersionRepository{db: db}
}

func (r *WorkspaceAgentPresetVersionRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.WorkspaceAgentPresetVersion, error) {
	var versions []model.WorkspaceAgentPresetVersion
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("family_key ASC, created_at ASC").
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list workspace preset versions: %w", err)
	}
	return versions, nil
}

func (r *WorkspaceAgentPresetVersionRepository) GetByVersionKey(ctx context.Context, workspaceID, familyKey, versionKey string) (*model.WorkspaceAgentPresetVersion, error) {
	var version model.WorkspaceAgentPresetVersion
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND family_key = ? AND version_key = ?", workspaceID, familyKey, versionKey).
		First(&version).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace preset version: %w", err)
	}
	return &version, nil
}

func (r *WorkspaceAgentPresetVersionRepository) Create(ctx context.Context, version *model.WorkspaceAgentPresetVersion) error {
	if err := r.db.WithContext(ctx).Create(version).Error; err != nil {
		return fmt.Errorf("create workspace preset version: %w", err)
	}
	return nil
}
