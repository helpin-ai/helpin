package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CommandBarPlanDismissalRepository persists per-user dismissals of command bar
// plans. Used by the command runs rail "Clear all" and per-card hide actions.
type CommandBarPlanDismissalRepository struct {
	db *gorm.DB
}

// NewCommandBarPlanDismissalRepository constructs a CommandBarPlanDismissalRepository.
func NewCommandBarPlanDismissalRepository(db *gorm.DB) *CommandBarPlanDismissalRepository {
	return &CommandBarPlanDismissalRepository{db: db}
}

// Dismiss records that the user has dismissed the given plan IDs from their rail.
// Existing rows are left intact so dismissed_at reflects the first dismissal.
func (r *CommandBarPlanDismissalRepository) Dismiss(ctx context.Context, workspaceID, userID string, planIDs []string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar plan dismissal repository is not configured")
	}
	if workspaceID == "" || userID == "" || len(planIDs) == 0 {
		return nil
	}
	rows := make([]model.CommandBarPlanDismissal, 0, len(planIDs))
	for _, planID := range planIDs {
		if planID == "" {
			continue
		}
		rows = append(rows, model.CommandBarPlanDismissal{
			WorkspaceID: workspaceID,
			PlanID:      planID,
			UserID:      userID,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&rows).Error; err != nil {
		return fmt.Errorf("dismiss command bar plans: %w", err)
	}
	return nil
}

// ListDismissedPlanIDs returns the IDs of plans the user has dismissed in the
// workspace. Returns nil rather than an error when there are none, so the
// service layer can use the result directly as a NOT IN filter.
func (r *CommandBarPlanDismissalRepository) ListDismissedPlanIDs(ctx context.Context, workspaceID, userID string) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("command bar plan dismissal repository is not configured")
	}
	if workspaceID == "" || userID == "" {
		return nil, nil
	}
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&model.CommandBarPlanDismissal{}).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Pluck("plan_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list command bar plan dismissals: %w", err)
	}
	return ids, nil
}
