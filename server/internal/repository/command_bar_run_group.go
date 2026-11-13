package repository

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CommandBarPlanRunGroupRepository stores the run group of each command-bar
// plan that uses Agent Runtime completions.
type CommandBarPlanRunGroupRepository struct {
	db *gorm.DB
}

// NewCommandBarPlanRunGroupRepository returns a repository over db.
func NewCommandBarPlanRunGroupRepository(db *gorm.DB) *CommandBarPlanRunGroupRepository {
	return &CommandBarPlanRunGroupRepository{db: db}
}

// Create records a plan's group; an existing row is kept.
func (r *CommandBarPlanRunGroupRepository) Create(ctx context.Context, link *model.CommandBarPlanRunGroup) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(link).Error
}

// GetByPlan returns a plan's group, or nil when the plan has none.
func (r *CommandBarPlanRunGroupRepository) GetByPlan(ctx context.Context, workspaceID, planID string) (*model.CommandBarPlanRunGroup, error) {
	var link model.CommandBarPlanRunGroup
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND plan_id = ?", workspaceID, planID).Take(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get command bar plan run group %q: %w", planID, err)
	}
	return &link, nil
}

// ListRunning returns groups of running plans, least recently advanced first.
func (r *CommandBarPlanRunGroupRepository) ListRunning(ctx context.Context, limit int) ([]model.CommandBarPlanRunGroup, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var links []model.CommandBarPlanRunGroup
	err := r.db.WithContext(ctx).
		Joins("JOIN command_bar_plans ON command_bar_plans.id = command_bar_plan_run_groups.plan_id").
		Where("command_bar_plans.status = ?", model.CommandBarPlanStatusRunning).
		Order("command_bar_plan_run_groups.updated_at ASC").Limit(limit).
		Find(&links).Error
	if err != nil {
		return nil, fmt.Errorf("list running command bar plan run groups: %w", err)
	}
	return links, nil
}

// AdvanceCursor records completion seq as applied. It reports true only to
// the one caller that moves the cursor past seq, which makes applying a
// completion idempotent per (group, sequence) across replicas and retries.
func (r *CommandBarPlanRunGroupRepository) AdvanceCursor(ctx context.Context, workspaceID, planID string, seq int64) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.CommandBarPlanRunGroup{}).
		Where("workspace_id = ? AND plan_id = ? AND completion_cursor < ?", workspaceID, planID, seq).
		Updates(map[string]any{"completion_cursor": seq, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return false, fmt.Errorf("advance command bar plan cursor %q: %w", planID, result.Error)
	}
	return result.RowsAffected == 1, nil
}

// Touch marks a plan's group as checked, so the consumer rotates through
// plans instead of rereading the same ones.
func (r *CommandBarPlanRunGroupRepository) Touch(ctx context.Context, workspaceID, planID string) error {
	return r.db.WithContext(ctx).Model(&model.CommandBarPlanRunGroup{}).
		Where("workspace_id = ? AND plan_id = ?", workspaceID, planID).
		UpdateColumn("updated_at", time.Now().UTC()).Error
}

// WithPlanLock runs fn while holding the plan's advancement lock, a
// PostgreSQL session advisory lock on a dedicated connection, so the
// completion consumer, the run finalizer and the stalled-plan sweep never
// advance one plan at the same time. It waits up to wait for the lock and
// reports false without running fn when it stays taken. Stores other than
// PostgreSQL (tests) run fn directly.
func (r *CommandBarPlanRunGroupRepository) WithPlanLock(ctx context.Context, planID string, wait time.Duration, fn func() error) (bool, error) {
	if r == nil || r.db == nil || r.db.Dialector.Name() != "postgres" {
		return true, fn()
	}
	sqlDB, err := r.db.DB()
	if err != nil {
		return false, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("plan lock connection: %w", err)
	}
	defer conn.Close()
	key := planLockKey(planID)
	deadline := time.Now().Add(wait)
	for {
		var acquired bool
		if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&acquired); err != nil {
			return false, fmt.Errorf("plan lock: %w", err)
		}
		if acquired {
			break
		}
		if !time.Now().Before(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock($1)`, key)
	}()
	return true, fn()
}

func planLockKey(planID string) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte("command_bar_plan:" + planID))
	return int64(hash.Sum64())
}
