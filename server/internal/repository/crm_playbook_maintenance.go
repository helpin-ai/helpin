package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// MaintenanceRuns rotates a bounded set of interrupted/active runs. Completed
// runs disappear only after their canonical follow-through receipt is observed.
func (r *CRMPlaybookExecutionRepository) MaintenanceRuns(ctx context.Context, now time.Time) ([]model.AgentRun, error) {
	var runs []model.AgentRun
	err := r.db.WithContext(ctx).Table("agent_runs a").Select("a.*").
		Joins("JOIN automation_run_bindings b ON b.workspace_id = a.workspace_id AND b.run_id = a.id").
		Where("(b.observed_terminal_at IS NULL OR a.status IN ('queued','running','paused')) AND a.created_at < ? AND (b.last_maintenance_at IS NULL OR b.last_maintenance_at < ?)", now.Add(-time.Minute), now.Add(-time.Minute)).
		Order("b.last_maintenance_at ASC NULLS FIRST, a.created_at, a.id").Limit(20).Scan(&runs).Error
	return runs, err
}

func (r *CRMPlaybookExecutionRepository) ClaimRunMaintenance(ctx context.Context, run model.AgentRun, now time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.AutomationRunBinding{}).Where("workspace_id = ? AND run_id = ? AND (last_maintenance_at IS NULL OR last_maintenance_at < ?)", run.WorkspaceID, run.ID, now.Add(-time.Minute)).Update("last_maintenance_at", now)
	return result.RowsAffected == 1, result.Error
}

func (r *CRMPlaybookExecutionRepository) MaintenanceBindings(ctx context.Context, now time.Time) ([]model.CRMPlaybookAutomationBinding, error) {
	var bindings []model.CRMPlaybookAutomationBinding
	err := r.db.WithContext(ctx).Where("enabled = true AND (last_maintenance_at IS NULL OR last_maintenance_at < ?)", now.Add(-time.Minute)).Order("last_maintenance_at ASC NULLS FIRST, updated_at, situation_id").Limit(50).Find(&bindings).Error
	return bindings, err
}

// MaintainBinding performs cheap database checks, not an AI run. New customer
// facts fence obsolete proposals; action decisions wake without erasing rejection.
func (r *CRMPlaybookExecutionRepository) MaintainBinding(ctx context.Context, ws, id string, now time.Time, fingerprint func(model.CRMPlaybookExecutionSource, model.CRMPlaybookActionFacts) (string, error)) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		source, err := playbookExecutionSource(tx, ws, id)
		if err != nil {
			return err
		}
		if source == nil || !source.Binding.Enabled {
			return nil
		}
		b := source.Binding
		if b.LastMaintenanceAt != nil && b.LastMaintenanceAt.After(now.Add(-time.Minute)) {
			return nil
		}
		// Expiry is an explicit canonical status, not a fabricated rejection.
		if err := tx.Model(&model.CRMSuggestion{}).Where("workspace_id = ? AND status = 'pending' AND suggestion_type = 'playbook_action'", ws).
			Where("id IN (SELECT suggestion_id FROM crm_playbook_action_intents WHERE workspace_id = ? AND situation_id = ? AND expires_at <= ?)", ws, id, now).
			Updates(map[string]any{"status": "expired", "updated_at": now}).Error; err != nil {
			return err
		}
		source, err = playbookExecutionSource(tx, ws, id)
		if err != nil {
			return err
		}
		facts, err := playbookActionFacts(tx, source.Item.Situation)
		if err != nil {
			return err
		}
		key, err := fingerprint(*source, *facts)
		if err != nil {
			return err
		}
		type decision struct{ ID, Status, Execution string }
		decisions := make([]decision, 0, len(source.Item.Actions))
		for _, action := range source.Item.Actions {
			decisions = append(decisions, decision{action.ID, action.Status, action.ExecutionStatus})
		}
		encoded, err := json.Marshal(decisions)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(encoded)
		actionKey := hex.EncodeToString(hash[:])
		updates := map[string]any{"last_maintenance_at": now, "context_fingerprint": key, "action_fingerprint": actionKey}
		if source.StopReason != "" {
			if err := supersedePlaybookActions(tx, ws, id, now); err != nil {
				return err
			}
			updates["blocker"] = source.StopReason
			return tx.Model(&model.CRMPlaybookAutomationBinding{}).Where("workspace_id = ? AND situation_id = ?", ws, id).Updates(updates).Error
		}
		factsChanged := b.ContextFingerprint != "" && key != b.ContextFingerprint
		actionsChanged := b.ActionFingerprint != "" && actionKey != b.ActionFingerprint
		if factsChanged {
			if err := supersedePlaybookActions(tx, ws, id, now); err != nil {
				return err
			}
			if err := NewAutomationScheduledEventRepository(tx).CancelTarget(ctx, ws, model.CRMPlaybookWorkDue, "crm_situation", id, now); err != nil {
				return err
			}
			b.Generation++
			updates["generation"], updates["no_progress_runs"], updates["blocker"] = b.Generation, 0, ""
			updates["progress_observed_at"], updates["escalated_at"] = now, nil
		} else {
			progressAt := b.ProgressObservedAt
			if progressAt == nil {
				progressAt = source.Item.Situation.PlaybookAppliedAt
			}
			if b.NoProgressRuns >= source.Settings.MaxNoProgressRuns || progressAt != nil && !now.Before(progressAt.Add(time.Duration(source.Policy.Definition.Policy.EscalateAfterHours)*time.Hour)) {
				if b.EscalatedAt == nil {
					updates["escalated_at"] = now
				}
				if b.Blocker == "" || b.Blocker == "awaiting_work" || b.Blocker == "awaiting_approval" {
					updates["blocker"] = "no_progress"
				}
			}
		}
		if err := tx.Model(&model.CRMPlaybookAutomationBinding{}).Where("workspace_id = ? AND situation_id = ?", ws, id).Updates(updates).Error; err != nil {
			return err
		}
		if playbookExecutionAllowed(source) && (factsChanged || actionsChanged) {
			return enqueuePlaybookWork(tx, b, "context:"+key[:16]+":"+actionKey[:16], now.Add(30*time.Second))
		}
		return nil
	})
}

// LinkActionTask uses native CRM associations. A retry only repairs links to the
// task correlated with this approved action; it never creates another task.
func (r *CRMPlaybookExecutionRepository) LinkActionTask(ctx context.Context, intent model.CRMPlaybookActionIntent, task model.PMTask) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, intent.WorkspaceID); err != nil {
			return err
		}
		if task.WorkspaceID != intent.WorkspaceID || task.ExternalID == nil || *task.ExternalID != "crm-action:"+intent.SuggestionID {
			return ErrCRMPlaybookExecutionBlocked
		}
		var situation model.CRMSituation
		if err := tx.Where("workspace_id = ? AND id = ?", intent.WorkspaceID, intent.SituationID).Take(&situation).Error; err != nil {
			return err
		}
		label := "Created from CRM playbook"
		for kind, id := range map[string]*string{"company": situation.CompanyID, "contact": situation.ContactID, "deal": situation.DealID} {
			if id == nil {
				continue
			}
			if err := NewCRMAssociationRepository(tx).Create(ctx, &model.CRMAssociation{WorkspaceID: intent.WorkspaceID, FromObjectType: kind, FromObjectID: *id, ToObjectType: "task", ToObjectID: task.ID, AssociationLabel: &label}); err != nil {
				return err
			}
		}
		return nil
	})
}
