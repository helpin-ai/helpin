package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMTaskTemplateRepository handles DB operations for story templates.
type PMTaskTemplateRepository struct {
	db *gorm.DB
}

// PMTaskTemplateListOptions aliases ScopeFilterOptions for story templates.
type PMTaskTemplateListOptions = ScopeFilterOptions

// NewPMTaskTemplateRepository creates a new PMTaskTemplateRepository.
func NewPMTaskTemplateRepository(db *gorm.DB) *PMTaskTemplateRepository {
	return &PMTaskTemplateRepository{db: db}
}

// ListByWorkspace lists story templates by workspace.
func (r *PMTaskTemplateRepository) ListByWorkspace(ctx context.Context, workspaceID string, opts PMTaskTemplateListOptions) ([]model.PMTaskTemplate, error) {
	var templates []model.PMTaskTemplate
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	query = ApplyScopeFilter(query, opts)
	if err := query.Order("team_id IS NOT NULL ASC, team_id ASC, name ASC").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("list story templates: %w", err)
	}
	return templates, nil
}

// GetByID returns a story template by ID.
func (r *PMTaskTemplateRepository) GetByID(ctx context.Context, id string) (*model.PMTaskTemplate, error) {
	var tmpl model.PMTaskTemplate
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&tmpl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get story template: %w", err)
	}
	return &tmpl, nil
}

// GetByName returns a story template by workspace/name within a scope.
func (r *PMTaskTemplateRepository) GetByName(ctx context.Context, workspaceID string, teamID *string, name string) (*model.PMTaskTemplate, error) {
	var tmpl model.PMTaskTemplate
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND LOWER(name) = LOWER(?)", workspaceID, strings.TrimSpace(name))
	if teamID == nil || strings.TrimSpace(*teamID) == "" {
		query = query.Where("team_id IS NULL")
	} else {
		query = query.Where("team_id = ?", *teamID)
	}
	if err := query.First(&tmpl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get story template by name: %w", err)
	}
	return &tmpl, nil
}

// Create inserts a story template.
func (r *PMTaskTemplateRepository) Create(ctx context.Context, tmpl *model.PMTaskTemplate) error {
	if err := r.db.WithContext(ctx).Create(tmpl).Error; err != nil {
		return fmt.Errorf("create story template: %w", err)
	}
	return nil
}

// Update updates a story template.
func (r *PMTaskTemplateRepository) Update(ctx context.Context, tmpl *model.PMTaskTemplate) error {
	if err := r.db.WithContext(ctx).Save(tmpl).Error; err != nil {
		return fmt.Errorf("update story template: %w", err)
	}
	return nil
}

// Delete hard-deletes a story template.
func (r *PMTaskTemplateRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMTaskTemplate{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete story template: %w", err)
	}
	return nil
}
