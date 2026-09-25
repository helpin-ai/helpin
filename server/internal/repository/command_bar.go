package repository

import (
	"context"
	"encoding/json"
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

// FindByDockChatAndRunID returns the plan in one dock chat that owns runID.
// Run IDs are persisted inside run_ids_by_step, so this deliberately decodes
// the chat's plans instead of relying on database-specific JSON operators.
func (r *CommandBarPlanRepository) FindByDockChatAndRunID(ctx context.Context, workspaceID, dockChatID, runID string) (*model.CommandBarPlanRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar plan repository is not configured")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	dockChatID = strings.TrimSpace(dockChatID)
	runID = strings.TrimSpace(runID)
	if workspaceID == "" || dockChatID == "" || runID == "" {
		return nil, nil
	}
	var plans []model.CommandBarPlanRecord
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND dock_chat_id = ?", workspaceID, dockChatID).
		Order("created_at DESC").
		Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("find dock plan for run: %w", err)
	}
	for index := range plans {
		var runIDs map[string]string
		if err := json.Unmarshal(plans[index].RunIDsByStep, &runIDs); err != nil {
			continue
		}
		for _, candidate := range runIDs {
			if strings.TrimSpace(candidate) == runID {
				return &plans[index], nil
			}
		}
	}
	return nil, nil
}

// FindBySupportConversationAndRunID returns the plan launched from one support
// conversation's chat run that owns runID (same decode strategy as
// FindByDockChatAndRunID).
func (r *CommandBarPlanRepository) FindBySupportConversationAndRunID(ctx context.Context, workspaceID, conversationID, runID string) (*model.CommandBarPlanRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar plan repository is not configured")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	conversationID = strings.TrimSpace(conversationID)
	runID = strings.TrimSpace(runID)
	if workspaceID == "" || conversationID == "" || runID == "" {
		return nil, nil
	}
	var plans []model.CommandBarPlanRecord
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND support_conversation_id = ?", workspaceID, conversationID).
		Order("created_at DESC").
		Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("find support plan for run: %w", err)
	}
	for index := range plans {
		var runIDs map[string]string
		if err := json.Unmarshal(plans[index].RunIDsByStep, &runIDs); err != nil {
			continue
		}
		for _, candidate := range runIDs {
			if strings.TrimSpace(candidate) == runID {
				return &plans[index], nil
			}
		}
	}
	return nil, nil
}

// CountPlansForSupportConversation returns how many child plans a support
// conversation has launched; activeOnly restricts to still-running plans.
func (r *CommandBarPlanRepository) CountPlansForSupportConversation(ctx context.Context, workspaceID, conversationID string, activeOnly bool) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("command bar plan repository is not configured")
	}
	query := r.db.WithContext(ctx).
		Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND support_conversation_id = ?", workspaceID, conversationID)
	if activeOnly {
		query = query.Where("status = ?", model.CommandBarPlanStatusRunning)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count support conversation plans: %w", err)
	}
	return count, nil
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

// BeginPausedStepRestart atomically claims the exact old run. A second editor
// cannot replace the same step after its mapping has already been removed.
func (r *CommandBarPlanRepository) BeginPausedStepRestart(ctx context.Context, workspaceID, id string, stepIndex int, expectedRunID string, binding []byte) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	var profile any
	if len(binding) > 0 {
		profile = gorm.Expr("?::jsonb", string(binding))
	}
	result := r.db.WithContext(ctx).Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND id = ? AND run_ids_by_step ->> ? = ?", workspaceID, id, strconv.Itoa(stepIndex), expectedRunID).
		Where("status IN ?", []string{model.CommandBarPlanStatusRunning, model.CommandBarPlanStatusFailed}).
		Updates(map[string]any{
			"run_ids_by_step":    gorm.Expr("run_ids_by_step - ?", strconv.Itoa(stepIndex)),
			"profile_binding":    profile,
			"status":             model.CommandBarPlanStatusFailed,
			"current_step_index": stepIndex,
		})
	if result.Error != nil {
		return fmt.Errorf("claim paused step restart: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("delivery step changed; reload before restarting")
	}
	return nil
}

// RollbackPausedStepRestart restores a cancelled mapping when launch fails,
// leaving the plan retryable rather than stranded with an empty step.
func (r *CommandBarPlanRepository) RollbackPausedStepRestart(ctx context.Context, workspaceID, id string, stepIndex int, oldRunID string, binding []byte) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan repository is not configured")
	}
	var profile any
	if len(binding) > 0 {
		profile = gorm.Expr("?::jsonb", string(binding))
	}
	result := r.db.WithContext(ctx).Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND id = ? AND run_ids_by_step ->> ? IS NULL", workspaceID, id, strconv.Itoa(stepIndex)).
		Updates(map[string]any{
			"run_ids_by_step": gorm.Expr("COALESCE(run_ids_by_step, '{}'::jsonb) || jsonb_build_object(?::text, ?::text)", strconv.Itoa(stepIndex), oldRunID),
			"profile_binding": profile,
			"status":          model.CommandBarPlanStatusFailed,
		})
	if result.Error != nil {
		return fmt.Errorf("restore cancelled step: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("delivery step changed before recovery")
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

// HasPendingSupportResult reports child work whose result has not yet reached
// the parent. Completed-but-undelivered plans count as pending too.
func (r *CommandBarPlanRepository) HasPendingSupportResult(ctx context.Context, workspaceID, conversationID, parentRunID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.CommandBarPlanRecord{}).
		Where("workspace_id = ? AND support_conversation_id = ? AND parent_chat_run_id = ? AND parent_notified_at IS NULL", workspaceID, conversationID, parentRunID).
		Count(&count).Error
	return count > 0, err
}
