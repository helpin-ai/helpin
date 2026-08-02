package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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

func (r *CommandBarPlanRepository) ListRecent(ctx context.Context, workspaceID, actorID string, limit int) ([]model.CommandBarPlanRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar plan repository is not configured")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var plans []model.CommandBarPlanRecord
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if actorID != "" {
		query = query.Where("actor_id = ?", actorID)
	}
	if err := query.Order("created_at DESC").Limit(limit).Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("list command bar plans: %w", err)
	}
	return plans, nil
}

// ListRunningUpdatedBefore returns running plans (across workspaces) whose
// last update is older than the cutoff. It backs the stalled-plan sweep that
// re-kicks the local step scheduler for plans no terminal-run finalizer has
// advanced.
func (r *CommandBarPlanRepository) ListRunningUpdatedBefore(ctx context.Context, cutoff time.Time, limit int) ([]model.CommandBarPlanRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar plan repository is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var plans []model.CommandBarPlanRecord
	if err := r.db.WithContext(ctx).
		Where("status = ?", model.CommandBarPlanStatusRunning).
		Where("updated_at < ?", cutoff).
		Order("updated_at ASC").
		Limit(limit).
		Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("list running command bar plans: %w", err)
	}
	return plans, nil
}

// ListByEntity returns plans whose page_context targets the given entity
// (e.g. an epic), regardless of which actor triggered them. This powers the
// epic-visible delivery view, where anyone who can read the epic should see
// its command-bar deliveries — unlike ListRecent, which is actor-scoped.
func (r *CommandBarPlanRepository) ListByEntity(ctx context.Context, workspaceID, entityType, entityID string, limit int) ([]model.CommandBarPlanRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar plan repository is not configured")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var plans []model.CommandBarPlanRecord
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Where("page_context->>'entity_type' = ?", entityType).
		Where("page_context->>'entity_id' = ?", entityID).
		Order("created_at DESC").
		Limit(limit).
		Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("list command bar plans by entity: %w", err)
	}
	return plans, nil
}

func (r *CommandBarPlanRepository) SetStepRun(ctx context.Context, workspaceID, id string, stepIndex int, runID string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	if err := r.db.WithContext(ctx).
		Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]any{
			"current_step_index": stepIndex,
			"run_ids_by_step": gorm.Expr(
				"COALESCE(run_ids_by_step, '{}'::jsonb) || jsonb_build_object(?::text, ?::text)",
				strconv.Itoa(stepIndex),
				runID,
			),
			"status": model.CommandBarPlanStatusRunning,
		}).Error; err != nil {
		return fmt.Errorf("set command bar plan step run: %w", err)
	}
	return nil
}

func (r *CommandBarPlanRepository) RestartStepRun(ctx context.Context, workspaceID, id string, stepIndex int, runIDsByStep []byte) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	// Write the JSON through an explicit text→jsonb cast (mirroring
	// SetStepRun): a bare []byte parameter renders as a bytea literal under
	// the Postgres driver, which cannot be coerced into the jsonb column
	// (SQLSTATE 22P02).
	payload := string(runIDsByStep)
	if strings.TrimSpace(payload) == "" {
		payload = "{}"
	}
	if err := r.db.WithContext(ctx).
		Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]any{
			"current_step_index": stepIndex,
			"run_ids_by_step":    gorm.Expr("?::jsonb", payload),
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
		Where("workspace_id = ? AND id = ? AND status NOT IN ?", workspaceID, id, []string{model.CommandBarPlanStatusCancelled, model.CommandBarPlanStatusFailed}).
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
		Where("workspace_id = ? AND id = ? AND status NOT IN ?", workspaceID, id, []string{model.CommandBarPlanStatusCancelled, model.CommandBarPlanStatusCompleted}).
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

// ListSettledUnnotifiedDockPlans returns terminal plans launched from a dock
// chat whose result has not yet been delivered back into the chat.
func (r *CommandBarPlanRepository) ListSettledUnnotifiedDockPlans(ctx context.Context, limit int) ([]model.CommandBarPlanRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar plan repository is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var plans []model.CommandBarPlanRecord
	if err := r.db.WithContext(ctx).
		Where("parent_chat_run_id IS NOT NULL AND parent_notified_at IS NULL AND status IN ?", []string{
			model.CommandBarPlanStatusCompleted,
			model.CommandBarPlanStatusFailed,
			model.CommandBarPlanStatusCancelled,
		}).
		Order("updated_at ASC").
		Limit(limit).
		Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("list settled unnotified dock plans: %w", err)
	}
	return plans, nil
}

// MarkParentNotified records that the plan's result was delivered to (or is
// permanently undeliverable for) its parent dock chat run.
func (r *CommandBarPlanRepository) MarkParentNotified(ctx context.Context, workspaceID, id string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND id = ? AND parent_notified_at IS NULL", workspaceID, id).
		Update("parent_notified_at", now).Error; err != nil {
		return fmt.Errorf("mark command bar plan parent notified: %w", err)
	}
	return nil
}

// ListByDockChat returns plans launched from a dock chat, newest first.
func (r *CommandBarPlanRepository) ListByDockChat(ctx context.Context, workspaceID, dockChatID string, limit int) ([]model.CommandBarPlanRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar plan repository is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var plans []model.CommandBarPlanRecord
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND dock_chat_id = ?", workspaceID, dockChatID).
		Order("created_at DESC").
		Limit(limit).
		Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("list dock chat plans: %w", err)
	}
	return plans, nil
}
