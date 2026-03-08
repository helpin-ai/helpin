package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMStoryTemplateRepository handles DB operations for story templates.
type PMStoryTemplateRepository struct {
	db *gorm.DB
}

// PMStoryTemplateListOptions aliases ScopeFilterOptions for story templates.
type PMStoryTemplateListOptions = ScopeFilterOptions

// NewPMStoryTemplateRepository creates a new PMStoryTemplateRepository.
func NewPMStoryTemplateRepository(db *gorm.DB) *PMStoryTemplateRepository {
	return &PMStoryTemplateRepository{db: db}
}

// ListByWorkspace lists story templates by workspace.
func (r *PMStoryTemplateRepository) ListByWorkspace(ctx context.Context, workspaceID string, opts PMStoryTemplateListOptions) ([]model.PMStoryTemplate, error) {
	var templates []model.PMStoryTemplate
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	query = ApplyScopeFilter(query, opts)
	if err := query.Order("COALESCE(team_id::text, ''), name ASC").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("list story templates: %w", err)
	}
	return templates, nil
}

// GetByID returns a story template by ID.
func (r *PMStoryTemplateRepository) GetByID(ctx context.Context, id string) (*model.PMStoryTemplate, error) {
	var tmpl model.PMStoryTemplate
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&tmpl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get story template: %w", err)
	}
	return &tmpl, nil
}

// GetByName returns a story template by workspace/name within a scope.
func (r *PMStoryTemplateRepository) GetByName(ctx context.Context, workspaceID string, teamID *string, name string) (*model.PMStoryTemplate, error) {
	var tmpl model.PMStoryTemplate
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
func (r *PMStoryTemplateRepository) Create(ctx context.Context, tmpl *model.PMStoryTemplate) error {
	if err := r.db.WithContext(ctx).Create(tmpl).Error; err != nil {
		return fmt.Errorf("create story template: %w", err)
	}
	return nil
}

// Update updates a story template.
func (r *PMStoryTemplateRepository) Update(ctx context.Context, tmpl *model.PMStoryTemplate) error {
	if err := r.db.WithContext(ctx).Save(tmpl).Error; err != nil {
		return fmt.Errorf("update story template: %w", err)
	}
	return nil
}

// Delete hard-deletes a story template.
func (r *PMStoryTemplateRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMStoryTemplate{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete story template: %w", err)
	}
	return nil
}

