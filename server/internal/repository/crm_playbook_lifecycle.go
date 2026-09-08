package repository

import (
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// syncSituationPlaybookExecution runs under the canonical lifecycle transaction.
// Old runs lose authority before a changed/pause/close command becomes visible.
func syncSituationPlaybookExecution(tx *gorm.DB, ws, id string, state model.CRMSituationWorkState, now time.Time) error {
	// Manual CRM work remains compatible while the additive execution schema is absent.
	if !tx.Migrator().HasTable(&model.CRMPlaybookAutomationBinding{}) {
		return nil
	}
	binding, err := NewCRMPlaybookExecutionRepository(tx).Binding(tx.Statement.Context, ws, id)
	if err != nil || binding == nil || !binding.Enabled {
		return err
	}
	if state.Lifecycle != model.CRMSituationOpen {
		return pausePlaybookBinding(tx, *binding, "signal_"+state.Lifecycle, now)
	}
	if err := supersedePlaybookActions(tx, ws, id, now); err != nil {
		return err
	}
	if err := NewAutomationScheduledEventRepository(tx).CancelTarget(tx.Statement.Context, ws, model.CRMPlaybookWorkDue, "crm_situation", id, now); err != nil {
		return err
	}
	binding.Generation++
	if err := tx.Model(&model.CRMPlaybookAutomationBinding{}).Where("workspace_id = ? AND situation_id = ?", ws, id).
		Updates(map[string]any{"generation": binding.Generation, "blocker": "", "no_progress_runs": 0, "context_fingerprint": "", "progress_observed_at": now, "escalated_at": nil, "updated_at": now}).Error; err != nil {
		return err
	}
	return enqueuePlaybookWork(tx, *binding, fmt.Sprintf("changed:%d", binding.Generation), now.Add(30*time.Second))
}

func supersedePlaybookActions(tx *gorm.DB, ws, situationID string, now time.Time) error {
	if !tx.Migrator().HasTable(&model.CRMPlaybookActionIntent{}) {
		return nil
	}
	return tx.Model(&model.CRMSuggestion{}).Where("workspace_id = ? AND status = 'pending' AND suggestion_type = 'playbook_action'", ws).
		Where("id IN (SELECT suggestion_id FROM crm_playbook_action_intents WHERE workspace_id = ? AND situation_id = ?)", ws, situationID).
		Updates(map[string]any{"status": model.CRMSuggestionStatusSuperseded, "updated_at": now}).Error
}
