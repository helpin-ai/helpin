package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMAutomationRepository handles DB operations for automations.
type PMAutomationRepository struct {
	db *gorm.DB
}

// NewPMAutomationRepository creates a new PMAutomationRepository.
func NewPMAutomationRepository(db *gorm.DB) *PMAutomationRepository {
	return &PMAutomationRepository{db: db}
}

// ListByWorkspace returns all automations for a workspace.
func (r *PMAutomationRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.PMAutomation, error) {
	var automations []model.PMAutomation
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("automation_type ASC, team_id ASC").
		Find(&automations).Error; err != nil {
		return nil, fmt.Errorf("list automations: %w", err)
	}
	return automations, nil
}

// ListEnabledByType returns all enabled automations of a given type across all workspaces.
func (r *PMAutomationRepository) ListEnabledByType(ctx context.Context, automationType string) ([]model.PMAutomation, error) {
	var automations []model.PMAutomation
	if err := r.db.WithContext(ctx).
		Where("automation_type = ? AND enabled = true", automationType).
		Find(&automations).Error; err != nil {
		return nil, fmt.Errorf("list enabled automations: %w", err)
	}
	return automations, nil
}

// GetByType returns automations of a type for a workspace, optionally filtered by team.
func (r *PMAutomationRepository) GetByType(ctx context.Context, workspaceID, automationType string, teamID *string) (*model.PMAutomation, error) {
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND automation_type = ?", workspaceID, automationType)
	if teamID != nil && *teamID != "" {
		query = query.Where("team_id = ?", *teamID)
	} else {
		query = query.Where("team_id IS NULL")
	}

	var automation model.PMAutomation
	if err := query.First(&automation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get automation: %w", err)
	}
	return &automation, nil
}

// Upsert creates or updates an automation based on (workspace_id, automation_type, team_id).
func (r *PMAutomationRepository) Upsert(ctx context.Context, automation *model.PMAutomation) error {
	// Find existing by unique key
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND automation_type = ?", automation.WorkspaceID, automation.AutomationType)
	if automation.TeamID != nil && *automation.TeamID != "" {
		query = query.Where("team_id = ?", *automation.TeamID)
	} else {
		query = query.Where("team_id IS NULL")
	}

	var existing model.PMAutomation
	if err := query.First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new
			if err := r.db.WithContext(ctx).Create(automation).Error; err != nil {
				return fmt.Errorf("create automation: %w", err)
			}
			return nil
		}
		return fmt.Errorf("upsert automation lookup: %w", err)
	}

	// Update existing
	automation.ID = existing.ID
	if err := r.db.WithContext(ctx).Model(&existing).Clauses(clause.Returning{}).Updates(map[string]interface{}{
		"enabled":         automation.Enabled,
		"config_state_id": automation.ConfigStateID,
		"config_int":      automation.ConfigInt,
		"config_int2":     automation.ConfigInt2,
		"config_int3":     automation.ConfigInt3,
	}).Error; err != nil {
		return fmt.Errorf("update automation: %w", err)
	}
	// Returning clause populates existing; copy timestamps back to automation
	automation.CreatedAt = existing.CreatedAt
	automation.UpdatedAt = existing.UpdatedAt
	return nil
}

// Delete removes an automation by workspace, type, and optional team.
func (r *PMAutomationRepository) Delete(ctx context.Context, workspaceID, automationType string, teamID *string) error {
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND automation_type = ?", workspaceID, automationType)
	if teamID != nil && *teamID != "" {
		query = query.Where("team_id = ?", *teamID)
	} else {
		query = query.Where("team_id IS NULL")
	}
	if err := query.Delete(&model.PMAutomation{}).Error; err != nil {
		return fmt.Errorf("delete automation: %w", err)
	}
	return nil
}
