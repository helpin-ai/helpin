package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type CommandBarPlanRepository struct {
	db *gorm.DB
}

func NewCommandBarPlanRepository(db *gorm.DB) *CommandBarPlanRepository {
	return &CommandBarPlanRepository{db: db}
}

func (r *CommandBarPlanRepository) Create(ctx context.Context, plan *model.CommandBarPlanRecord) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	if err := r.db.WithContext(ctx).Create(plan).Error; err != nil {
		return fmt.Errorf("create command bar plan: %w", err)
	}
	return nil
}

func (r *CommandBarPlanRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.CommandBarPlanRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar plan repository is not configured")
	}
	var plan model.CommandBarPlanRecord
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&plan).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get command bar plan: %w", err)
	}
	return &plan, nil
}

func (r *CommandBarPlanRepository) ListRecent(ctx context.Context, workspaceID string, limit int) ([]model.CommandBarPlanRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar plan repository is not configured")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var plans []model.CommandBarPlanRecord
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).
		Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("list command bar plans: %w", err)
	}
	return plans, nil
}

func (r *CommandBarPlanRepository) UpdateStepRun(ctx context.Context, workspaceID, id string, stepIndex int, runIDsByStep []byte) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	if err := r.db.WithContext(ctx).
		Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]any{
			"current_step_index": stepIndex,
			"run_ids_by_step":    runIDsByStep,
			"status":             model.CommandBarPlanStatusRunning,
		}).Error; err != nil {
		return fmt.Errorf("update command bar plan step run: %w", err)
	}
	return nil
}

func (r *CommandBarPlanRepository) RestartStepRun(ctx context.Context, workspaceID, id string, stepIndex int, runIDsByStep []byte) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	if err := r.db.WithContext(ctx).
		Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]any{
			"current_step_index": stepIndex,
			"run_ids_by_step":    runIDsByStep,
			"status":             model.CommandBarPlanStatusRunning,
			"error_message":      nil,
			"cancelled_at":       nil,
			"completed_at":       nil,
		}).Error; err != nil {
		return fmt.Errorf("restart command bar plan step run: %w", err)
	}
	return nil
}

func (r *CommandBarPlanRepository) MarkCompleted(ctx context.Context, workspaceID, id string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]any{
			"status":       model.CommandBarPlanStatusCompleted,
			"completed_at": now,
		}).Error; err != nil {
		return fmt.Errorf("mark command bar plan completed: %w", err)
	}
	return nil
}

func (r *CommandBarPlanRepository) MarkFailed(ctx context.Context, workspaceID, id, message string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	if err := r.db.WithContext(ctx).
		Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]any{
			"status":        model.CommandBarPlanStatusFailed,
			"error_message": message,
		}).Error; err != nil {
		return fmt.Errorf("mark command bar plan failed: %w", err)
	}
	return nil
}

func (r *CommandBarPlanRepository) MarkCancelled(ctx context.Context, workspaceID, id string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]any{
			"status":       model.CommandBarPlanStatusCancelled,
			"cancelled_at": now,
		}).Error; err != nil {
		return fmt.Errorf("mark command bar plan cancelled: %w", err)
	}
	return nil
}

type CommandBarUnmetIntentRepository struct {
	db *gorm.DB
}

func NewCommandBarUnmetIntentRepository(db *gorm.DB) *CommandBarUnmetIntentRepository {
	return &CommandBarUnmetIntentRepository{db: db}
}

func (r *CommandBarUnmetIntentRepository) Create(ctx context.Context, intent *model.CommandBarUnmetIntent) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar unmet intent repository is not configured")
	}
	if err := r.db.WithContext(ctx).Create(intent).Error; err != nil {
		return fmt.Errorf("create command bar unmet intent: %w", err)
	}
	return nil
}

func (r *CommandBarUnmetIntentRepository) List(ctx context.Context, workspaceID, status string, limit int) ([]model.CommandBarUnmetIntent, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar unmet intent repository is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var intents []model.CommandBarUnmetIntent
	if err := query.Order("created_at DESC").Limit(limit).Find(&intents).Error; err != nil {
		return nil, fmt.Errorf("list command bar unmet intents: %w", err)
	}
	return intents, nil
}

func (r *CommandBarUnmetIntentRepository) Review(ctx context.Context, workspaceID, id, status string, notes *string) (*model.CommandBarUnmetIntent, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar unmet intent repository is not configured")
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.CommandBarUnmetIntent{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]any{
			"status":       status,
			"review_notes": notes,
			"reviewed_at":  now,
		}).Error; err != nil {
		return nil, fmt.Errorf("review command bar unmet intent: %w", err)
	}
	var intent model.CommandBarUnmetIntent
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&intent).Error; err != nil {
		return nil, fmt.Errorf("get reviewed command bar unmet intent: %w", err)
	}
	return &intent, nil
}
