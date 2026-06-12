package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type WorkspaceAgentPresetVersionRepository struct {
	db *gorm.DB
}

func NewWorkspaceAgentPresetVersionRepository(db *gorm.DB) *WorkspaceAgentPresetVersionRepository {
	return &WorkspaceAgentPresetVersionRepository{db: db}
}

// DB returns the underlying *gorm.DB for transaction support.
func (r *WorkspaceAgentPresetVersionRepository) DB() *gorm.DB {
	return r.db
}

// WithTx returns a repository bound to the provided transaction.
func (r *WorkspaceAgentPresetVersionRepository) WithTx(tx *gorm.DB) *WorkspaceAgentPresetVersionRepository {
	if tx == nil {
		return r
	}
	return &WorkspaceAgentPresetVersionRepository{db: tx}
}

func (r *WorkspaceAgentPresetVersionRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.WorkspaceAgentPresetVersion, error) {
	var versions []model.WorkspaceAgentPresetVersion
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID).
		Order("family_key ASC, created_at DESC, id ASC").
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list workspace preset versions: %w", err)
	}
	return versions, nil
}

func (r *WorkspaceAgentPresetVersionRepository) GetByVersionKey(ctx context.Context, workspaceID, familyKey, versionKey string) (*model.WorkspaceAgentPresetVersion, error) {
	var version model.WorkspaceAgentPresetVersion
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND family_key = ? AND version_key = ? AND deleted_at IS NULL", workspaceID, familyKey, versionKey).
		First(&version).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace preset version: %w", err)
	}
	return &version, nil
}

func (r *WorkspaceAgentPresetVersionRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.WorkspaceAgentPresetVersion, error) {
	var version model.WorkspaceAgentPresetVersion
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ? AND deleted_at IS NULL", workspaceID, id).
		First(&version).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace preset version by id: %w", err)
	}
	return &version, nil
}

func (r *WorkspaceAgentPresetVersionRepository) Create(ctx context.Context, version *model.WorkspaceAgentPresetVersion) error {
	normalizeWorkspacePresetVersionTargets(version)
	if err := r.db.WithContext(ctx).Create(version).Error; err != nil {
		return fmt.Errorf("create workspace preset version: %w", err)
	}
	return nil
}

func (r *WorkspaceAgentPresetVersionRepository) Update(ctx context.Context, version *model.WorkspaceAgentPresetVersion) error {
	// Workspace preset versions currently use last-writer-wins semantics.
	// If the UI later needs conflict banners, this should move to an optimistic updated_at guard.
	normalizeWorkspacePresetVersionTargets(version)
	if err := r.db.WithContext(ctx).Save(version).Error; err != nil {
		return fmt.Errorf("update workspace preset version: %w", err)
	}
	return nil
}

func normalizeWorkspacePresetVersionTargets(version *model.WorkspaceAgentPresetVersion) {
	if version == nil {
		return
	}
	var targets []string
	if err := json.Unmarshal(version.AllowedTargets, &targets); err == nil && len(targets) > 0 {
		return
	}
	version.AllowedTargets = json.RawMessage(`["task"]`)
}

func (r *WorkspaceAgentPresetVersionRepository) Delete(ctx context.Context, workspaceID, id string) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.WorkspaceAgentPresetVersion{}).
		Where("workspace_id = ? AND id = ? AND deleted_at IS NULL", workspaceID, id).
		Updates(map[string]any{
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("delete workspace preset version: %w", err)
	}
	return nil
}
