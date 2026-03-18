package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type PMRecurringTemplateRepository struct {
	db *gorm.DB
}

func NewPMRecurringTemplateRepository(db *gorm.DB) *PMRecurringTemplateRepository {
	return &PMRecurringTemplateRepository{db: db}
}

func (r *PMRecurringTemplateRepository) Create(ctx context.Context, tmpl *model.PMRecurringTemplate) error {
	if err := r.db.WithContext(ctx).Create(tmpl).Error; err != nil {
		return fmt.Errorf("create recurring template: %w", err)
	}
	return nil
}

func (r *PMRecurringTemplateRepository) Update(ctx context.Context, tmpl *model.PMRecurringTemplate) error {
	if err := r.db.WithContext(ctx).Save(tmpl).Error; err != nil {
		return fmt.Errorf("update recurring template: %w", err)
	}
	return nil
}

func (r *PMRecurringTemplateRepository) GetByID(ctx context.Context, id string) (*model.PMRecurringTemplate, error) {
	var tmpl model.PMRecurringTemplate
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&tmpl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get recurring template: %w", err)
	}
	return &tmpl, nil
}

func (r *PMRecurringTemplateRepository) GetByStoryID(ctx context.Context, storyID string) (*model.PMRecurringTemplate, error) {
	var tmpl model.PMRecurringTemplate
	if err := r.db.WithContext(ctx).
		Table("pm_recurring_templates t").
		Select("t.*").
		Joins("JOIN pm_stories s ON s.recurring_template_id = t.id").
		Where("s.id = ?", storyID).
		First(&tmpl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get recurring template by story: %w", err)
	}
	return &tmpl, nil
}

func (r *PMRecurringTemplateRepository) ListByWorkspace(ctx context.Context, workspaceID string, status, teamID, search *string) ([]model.PMRecurringTemplate, error) {
	query := r.db.WithContext(ctx).Model(&model.PMRecurringTemplate{}).Where("workspace_id = ?", workspaceID)
	if status != nil && *status != "" {
		query = query.Where("status = ?", *status)
	}
	if teamID != nil && *teamID != "" {
		query = query.Where("team_id = ?", *teamID)
	}
	if search != nil && strings.TrimSpace(*search) != "" {
		pattern := "%" + strings.TrimSpace(*search) + "%"
		query = query.Where("title ILIKE ?", pattern)
	}

	var templates []model.PMRecurringTemplate
	if err := query.Order("COALESCE(next_run_at, updated_at) ASC, created_at DESC").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("list recurring templates: %w", err)
	}
	return templates, nil
}

func (r *PMRecurringTemplateRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]model.PMRecurringTemplate, error) {
	query := r.db.WithContext(ctx).
		Model(&model.PMRecurringTemplate{}).
		Where("status = ?", model.PMRecurringTemplateStatusActive).
		Where("next_run_at IS NOT NULL AND next_run_at <= ?", now).
		Order("next_run_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	var templates []model.PMRecurringTemplate
	if err := query.Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("list due recurring templates: %w", err)
	}
	return templates, nil
}

func (r *PMRecurringTemplateRepository) CreateRun(ctx context.Context, run *model.PMRecurringRun) error {
	if err := r.db.WithContext(ctx).Create(run).Error; err != nil {
		return fmt.Errorf("create recurring run: %w", err)
	}
	return nil
}

func (r *PMRecurringTemplateRepository) UpdateRun(ctx context.Context, run *model.PMRecurringRun) error {
	if err := r.db.WithContext(ctx).Save(run).Error; err != nil {
		return fmt.Errorf("update recurring run: %w", err)
	}
	return nil
}

func (r *PMRecurringTemplateRepository) GetRunByDedupeKey(ctx context.Context, dedupeKey string) (*model.PMRecurringRun, error) {
	var run model.PMRecurringRun
	if err := r.db.WithContext(ctx).Where("dedupe_key = ?", dedupeKey).First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get recurring run by dedupe key: %w", err)
	}
	return &run, nil
}

func (r *PMRecurringTemplateRepository) ListRuns(ctx context.Context, templateID string, limit int) ([]model.PMRecurringRun, error) {
	query := r.db.WithContext(ctx).Model(&model.PMRecurringRun{}).Where("template_id = ?", templateID).Order("occurrence_number DESC, created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	var runs []model.PMRecurringRun
	if err := query.Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list recurring runs: %w", err)
	}
	return runs, nil
}
