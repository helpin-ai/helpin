package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DecidePending records a legacy status-only decision or a dismissal, never an
// execution. It cannot reset an accepted action or race an executor's claim.
func (r *CRMSuggestionRepository) DecidePending(ctx context.Context, ws string, suggestion *model.CRMSuggestion, status string, reason *string) (bool, error) {
	decided := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		guarded := status == model.CRMSuggestionStatusAccepted && tx.Migrator().HasTable(&model.CRMSituationReference{})
		if guarded {
			if err := lockSituationWorkspace(tx, ws); err != nil {
				return err
			}
		}
		var err error
		decided, err = decidePendingSuggestion(tx, ws, suggestion, status, reason, guarded)
		return err
	})
	return decided, err
}

func decidePendingSuggestion(tx *gorm.DB, ws string, suggestion *model.CRMSuggestion, status string, reason *string, guarded bool) (bool, error) {
	query := tx.Model(&model.CRMSuggestion{}).
		Where("workspace_id = ? AND id = ? AND status = 'pending'", ws, suggestion.ID)
	if guarded {
		query = query.Where(`NOT EXISTS (SELECT 1 FROM crm_situation_references ref
			JOIN crm_situations s ON s.workspace_id = ref.workspace_id AND s.id = ref.situation_id
			WHERE ref.workspace_id = ? AND ref.kind = 'suggestion' AND ref.source_id = ? AND s.lifecycle <> 'open')`, ws, suggestion.ID)
	}
	if !suggestion.UpdatedAt.IsZero() {
		query = query.Where("updated_at = ?", suggestion.UpdatedAt)
	}
	updates := map[string]any{"status": status, "dismissal_reason": reason, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}
	if status == model.CRMSuggestionStatusAccepted && suggestion.SuggestionType != model.CRMSuggestionDealCreate && suggestion.SuggestionType != model.CRMSuggestionDealAdvance {
		updates["execution_status"] = model.CRMSuggestionExecutionManualRequired
	}
	result := query.Updates(updates)
	return result.RowsAffected == 1, result.Error
}
