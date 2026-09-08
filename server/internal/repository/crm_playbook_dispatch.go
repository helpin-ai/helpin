package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ReserveRun freezes one normal run identity before any metering or remote call.
// The supplied compiler may only inspect the records provided by this transaction.
func (r *CRMPlaybookExecutionRepository) ReserveRun(ctx context.Context, claim model.AutomationScheduledEvent, now time.Time,
	compile func(model.CRMPlaybookExecutionSource, string) (model.AutomationRunBinding, error),
) (*model.AutomationRunBinding, error) {
	var result *model.AutomationRunBinding
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, claim.WorkspaceID); err != nil {
			return err
		}
		events := NewAutomationScheduledEventRepository(tx)
		event, err := events.LockClaim(ctx, claim, now)
		if err != nil {
			return err
		}
		if event.Kind != model.CRMPlaybookWorkDue || event.TargetType != "crm_situation" {
			return ErrCRMPlaybookExecutionBlocked
		}
		source, err := playbookExecutionSource(tx, event.WorkspaceID, event.TargetID)
		if err != nil {
			return err
		}
		if source != nil && source.StopReason != "" {
			if err := setPlaybookExecutionBlocker(tx, source.Binding, source.StopReason, now); err != nil {
				return err
			}
			return events.Complete(ctx, *event, "stopped", now)
		}
		if !playbookExecutionAllowed(source) || source.Binding.Generation != event.ExpectedRevision {
			return events.Complete(ctx, *event, "stale", now)
		}
		var existing model.AutomationRunBinding
		err = tx.Where("workspace_id = ? AND event_id = ?", event.WorkspaceID, event.ID).Take(&existing).Error
		if err == nil {
			if existing.Generation != source.Binding.Generation || existing.SituationRevision != source.Item.Situation.Revision {
				return events.Complete(ctx, *event, "superseded", now)
			}
			result = &existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var active int64
		if err := tx.Table("automation_run_bindings AS binding").Joins("JOIN agent_runs run ON run.id = binding.run_id AND run.workspace_id = binding.workspace_id").
			Where("binding.workspace_id = ? AND binding.situation_id = ? AND run.status IN ?", event.WorkspaceID, event.TargetID, []string{"queued", "running", "paused"}).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			if err := enqueuePlaybookWork(tx, source.Binding, "running:"+event.ID, now.Add(15*time.Minute)); err != nil {
				return err
			}
			return events.Complete(ctx, *event, "run_in_progress", now)
		}
		// Waiting for a human or an already-created deliverable needs a cheap
		// check, not another model run or a replacement recommendation.
		waiting := ""
		for _, action := range source.Item.Actions {
			if action.Status == "pending" {
				waiting = "awaiting_approval"
			}
			if action.Status == "accepted" && (action.ExecutionStatus == "in_progress" || action.ExecutionStatus == "manual_required") {
				waiting = "awaiting_result"
				break
			}
		}
		if waiting == "" && tx.Migrator().HasTable(&model.CRMPlaybookActionIntent{}) {
			facts, err := playbookActionFacts(tx, source.Item.Situation)
			if err != nil {
				return err
			}
			for _, task := range facts.LinkedTasks {
				if task.CompletedAt == nil {
					waiting = "awaiting_work"
					break
				}
			}
		}
		if waiting != "" {
			if err := setPlaybookExecutionBlocker(tx, source.Binding, waiting, now); err != nil {
				return err
			}
			if err := enqueuePlaybookWork(tx, source.Binding, "waiting:"+event.ID, now.Add(time.Hour)); err != nil {
				return err
			}
			return events.Complete(ctx, *event, waiting, now)
		}
		var today int64
		if err := tx.Model(&model.AutomationRunBinding{}).Where("workspace_id = ? AND situation_id = ? AND created_at >= ?", event.WorkspaceID, event.TargetID, now.Add(-24*time.Hour)).Count(&today).Error; err != nil {
			return err
		}
		blocker := ""
		if today >= int64(source.Settings.MaxRunsPerDay) {
			blocker = "daily_run_limit"
		}
		if source.Binding.NoProgressRuns >= source.Settings.MaxNoProgressRuns {
			blocker = "no_progress"
		}
		if !source.Item.OwnerAvailable {
			blocker = "needs_owner"
		}
		if blocker != "" {
			if err := setPlaybookExecutionBlocker(tx, source.Binding, blocker, now); err != nil {
				return err
			}
			if blocker == "daily_run_limit" {
				if err := enqueuePlaybookWork(tx, source.Binding, "limit:"+event.ID, now.Add(time.Hour)); err != nil {
					return err
				}
			}
			return events.Complete(ctx, *event, blocker, now)
		}
		runID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("helpin.automation.run:"+event.WorkspaceID+":"+event.ID)).String()
		binding, err := compile(*source, runID)
		if err != nil {
			return err
		}
		binding.RunID, binding.WorkspaceID, binding.EventID = runID, event.WorkspaceID, event.ID
		binding.SituationID, binding.ConnectionID = event.TargetID, source.Connection.ID
		binding.Generation, binding.SituationRevision = source.Binding.Generation, source.Item.Situation.Revision
		binding.CreatedAt = now
		if err := tx.Create(&binding).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CRMPlaybookAutomationBinding{}).Where("workspace_id = ? AND situation_id = ?", event.WorkspaceID, event.TargetID).
			Updates(map[string]any{"last_run_id": runID, "blocker": "", "updated_at": now}).Error; err != nil {
			return err
		}
		result = &binding
		return nil
	})
	return result, err
}

// RunBinding resolves only a server-owned dispatch receipt, never request metadata.
func (r *CRMPlaybookExecutionRepository) RunBinding(ctx context.Context, ws, runID string) (*model.AutomationRunBinding, error) {
	var binding model.AutomationRunBinding
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND run_id = ?", ws, runID).Take(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &binding, err
}

// ValidateRun reloads the live gate and material revision for a stored normal run.
func (r *CRMPlaybookExecutionRepository) ValidateRun(ctx context.Context, ws, runID string) (*model.AutomationRunBinding, *model.CRMPlaybookExecutionSource, error) {
	var binding *model.AutomationRunBinding
	var source *model.CRMPlaybookExecutionSource
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		var err error
		binding, err = NewCRMPlaybookExecutionRepository(tx).RunBinding(ctx, ws, runID)
		if err != nil {
			return err
		}
		if binding == nil {
			return ErrCRMPlaybookExecutionBlocked
		}
		source, err = playbookExecutionSource(tx, ws, binding.SituationID)
		if err != nil {
			return err
		}
		if !playbookExecutionAllowed(source) || source.Binding.Generation != binding.Generation || source.Binding.ConnectionID != binding.ConnectionID || source.Item.Situation.Revision != binding.SituationRevision || !source.Item.OwnerAvailable {
			return ErrCRMPlaybookExecutionBlocked
		}
		return nil
	})
	return binding, source, err
}

// FinishDispatch acknowledges a wake-up without claiming a customer outcome.
func (r *CRMPlaybookExecutionRepository) FinishDispatch(ctx context.Context, claim model.AutomationScheduledEvent, code string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, claim.WorkspaceID); err != nil {
			return err
		}
		events := NewAutomationScheduledEventRepository(tx)
		if _, err := events.LockClaim(ctx, claim, now); err != nil {
			return err
		}
		if code != "started" {
			binding, err := NewCRMPlaybookExecutionRepository(tx).Binding(ctx, claim.WorkspaceID, claim.TargetID)
			if err != nil {
				return err
			}
			if binding != nil && binding.Generation == claim.ExpectedRevision {
				if err := setPlaybookExecutionBlocker(tx, *binding, code, now); err != nil {
					return err
				}
			}
		}
		return events.Complete(ctx, claim, code, now)
	})
}

// ObserveTerminalRun records a check once and schedules bounded follow-through.
// It never marks a milestone complete or moves an existing business deadline.
func (r *CRMPlaybookExecutionRepository) ObserveTerminalRun(ctx context.Context, run model.AgentRun, now time.Time) error {
	if model.IsAgentRunActiveStatus(run.Status) {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, run.WorkspaceID); err != nil {
			return err
		}
		receipt, err := NewCRMPlaybookExecutionRepository(tx).RunBinding(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.ObservedTerminalAt != nil {
			return nil
		}
		if err := tx.Model(&model.AutomationRunBinding{}).Where("run_id = ?", run.ID).Update("observed_terminal_at", now).Error; err != nil {
			return err
		}
		source, err := playbookExecutionSource(tx, run.WorkspaceID, receipt.SituationID)
		if err != nil {
			return err
		}
		if !playbookExecutionAllowed(source) || source.Binding.Generation != receipt.Generation {
			return nil
		}
		updates := map[string]any{"last_checked_at": now, "updated_at": now}
		if source.Binding.Blocker == "start_uncertain" || source.Binding.Blocker == "launch_uncertain" {
			updates["blocker"] = ""
		}
		if source.Item.Situation.Revision == receipt.SituationRevision {
			updates["no_progress_runs"] = source.Binding.NoProgressRuns + 1
		} else {
			updates["no_progress_runs"] = 0
		}
		if run.Status != model.AgentRunStatusCompleted {
			updates["blocker"] = "run_failed"
		}
		if err := tx.Model(&model.CRMPlaybookAutomationBinding{}).Where("workspace_id = ? AND situation_id = ?", run.WorkspaceID, receipt.SituationID).Updates(updates).Error; err != nil {
			return err
		}
		if run.Status != model.AgentRunStatusCompleted {
			return nil
		}
		due := now.Add(time.Duration(source.Policy.Definition.Policy.CheckAfterHours) * time.Hour)
		if checkpoint := source.Item.Situation.NextCheckpointAt; checkpoint != nil && checkpoint.After(now) && checkpoint.Before(due) {
			due = *checkpoint
		}
		return enqueuePlaybookWork(tx, source.Binding, "after:"+run.ID, due)
	})
}

// ActiveRunIDs supports stop/recovery through the existing Agent cancellation path.
func (r *CRMPlaybookExecutionRepository) ActiveRunIDs(ctx context.Context, ws, pb, situationID string) ([]string, error) {
	var ids []string
	q := r.db.WithContext(ctx).Table("automation_run_bindings AS r").Joins("JOIN crm_playbook_automation_bindings b ON b.workspace_id = r.workspace_id AND b.situation_id = r.situation_id").
		Joins("JOIN agent_runs a ON a.workspace_id = r.workspace_id AND a.id = r.run_id").Where("r.workspace_id = ? AND a.status IN ?", ws, []string{"queued", "running", "paused"}).
		Where("b.enabled = false OR b.generation <> r.generation")
	if pb != "" {
		q = q.Where("b.playbook_id = ?", pb)
	}
	if situationID != "" {
		q = q.Where("r.situation_id = ?", situationID)
	}
	err := q.Pluck("r.run_id", &ids).Error
	return ids, err
}

func playbookExecutionAllowed(source *model.CRMPlaybookExecutionSource) bool {
	return source != nil && source.StopReason == "" && source.Settings.Enabled && source.Binding.Enabled && source.Item.Situation.Lifecycle == model.CRMSituationOpen &&
		source.Item.Situation.PlaybookID != nil && *source.Item.Situation.PlaybookID == source.Binding.PlaybookID &&
		source.Item.Situation.PlaybookVersionID != nil && *source.Item.Situation.PlaybookVersionID == source.Policy.ID
}

func setPlaybookExecutionBlocker(tx *gorm.DB, b model.CRMPlaybookAutomationBinding, code string, now time.Time) error {
	if code == "" || len(code) > 100 {
		return fmt.Errorf("invalid automation blocker")
	}
	return tx.Model(&model.CRMPlaybookAutomationBinding{}).Where("workspace_id = ? AND situation_id = ?", b.WorkspaceID, b.SituationID).
		Updates(map[string]any{"blocker": code, "updated_at": now}).Error
}
