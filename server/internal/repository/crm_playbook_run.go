package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// HasRuntimeProfile prevents stripped early callback metadata from taking an ordinary-run path.
func (r *CRMPlaybookExecutionRepository) HasRuntimeProfile(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AutomationRunBinding{}).Where("runtime_profile_id = ?", id).Count(&count).Error
	return count > 0, err
}

// CreateBoundRun persists the normal Agent row once, within current activation and host limits.
// AI usage preflight occurs before this call with the same idempotent run identity.
func (r *CRMPlaybookExecutionRepository) CreateBoundRun(ctx context.Context, run model.AgentRun) (*model.AgentRun, bool, error) {
	var result *model.AgentRun
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, run.WorkspaceID); err != nil {
			return err
		}
		store := NewCRMPlaybookExecutionRepository(tx)
		receipt, err := store.RunBinding(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Input.CRMPlaybook == nil {
			return ErrCRMPlaybookExecutionBlocked
		}
		source, err := playbookExecutionSource(tx, run.WorkspaceID, receipt.SituationID)
		if err != nil {
			return err
		}
		if !playbookExecutionAllowed(source) || source.Binding.Generation != receipt.Generation || source.Item.Situation.Revision != receipt.SituationRevision {
			return ErrCRMPlaybookExecutionBlocked
		}
		var input model.AgentRunInputPayload
		if json.Unmarshal(run.Input, &input) != nil || input.CRMPlaybook == nil || input.CRMPlaybook.ConnectionID != receipt.ConnectionID ||
			run.AgentID != receipt.AgentID || run.TargetType != receipt.Input.CRMPlaybook.Target.TargetType || run.TargetID != receipt.Input.CRMPlaybook.Target.TargetID {
			return ErrCRMPlaybookExecutionBlocked
		}
		var existing model.AgentRun
		err = tx.Where("workspace_id = ? AND id = ?", run.WorkspaceID, run.ID).Take(&existing).Error
		if err == nil {
			result = &existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		agent, err := NewAgentRepository(tx).GetByID(ctx, run.WorkspaceID, run.AgentID)
		if err != nil {
			return err
		}
		if agent == nil {
			return ErrCRMPlaybookExecutionBlocked
		}
		var active int64
		if err := tx.Model(&model.AgentRun{}).Where("workspace_id = ? AND agent_id = ? AND status IN ?", run.WorkspaceID, run.AgentID, []string{"queued", "running", "paused"}).Count(&active).Error; err != nil {
			return err
		}
		limit := agent.MaxConcurrentRuns
		if frozen := source.Connection.Snapshot.Agent.MaxConcurrentRuns; frozen > 0 && (limit <= 0 || frozen < limit) {
			limit = frozen
		}
		if limit > 0 && active >= int64(limit) {
			return ErrCRMPlaybookExecutionBlocked
		}
		for _, budget := range []*int{agent.MonthlyTokenBudget, source.Connection.Snapshot.Agent.MonthlyTokenBudget} {
			if budget != nil && *budget > 0 && agent.TokensUsedThisMonth >= *budget {
				return ErrCRMPlaybookExecutionBlocked
			}
		}
		if err := NewAgentRunRepository(tx).Create(ctx, &run); err != nil {
			return err
		}
		result = &run
		created = true
		return nil
	})
	return result, created, err
}

// SetBoundRuntimeMapping records correlation without overwriting a newer lifecycle projection.
func (r *CRMPlaybookExecutionRepository) SetBoundRuntimeMapping(ctx context.Context, ws, id, runtimeID string) error {
	if runtimeID == "" {
		return ErrCRMPlaybookExecutionBlocked
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		result := tx.Model(&model.AgentRun{}).Where("workspace_id = ? AND id = ? AND (external_runtime_id IS NULL OR external_runtime_id = '' OR external_runtime_id = ?)", ws, id, runtimeID).
			Updates(map[string]any{"external_runtime": "agent-runtime", "external_runtime_id": runtimeID, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCRMPlaybookExecutionBlocked
		}
		// A confirmed correlation resolves only this generation's launch warning.
		// It must not erase a newer pause, permission failure or customer blocker.
		return tx.Model(&model.CRMPlaybookAutomationBinding{}).
			Where("workspace_id = ? AND last_run_id = ? AND blocker IN ?", ws, id, []string{"start_uncertain", "launch_uncertain"}).
			Where("EXISTS (SELECT 1 FROM automation_run_bindings run WHERE run.workspace_id = crm_playbook_automation_bindings.workspace_id AND run.run_id = ? AND run.situation_id = crm_playbook_automation_bindings.situation_id AND run.generation = crm_playbook_automation_bindings.generation)", id).
			Updates(map[string]any{"blocker": "", "updated_at": now}).Error
	})
}

// ClaimBoundRuntimeStart prevents concurrent workers from issuing duplicate launch calls.
// An interrupted claim is reconciled by host-run identity, never blindly restarted.
func (r *CRMPlaybookExecutionRepository) ClaimBoundRuntimeStart(ctx context.Context, ws, id string) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.AgentRun{}).Where("workspace_id = ? AND id = ? AND status = ? AND execution_stage = ?", ws, id, "queued", "prepared").
		Updates(map[string]any{"execution_stage": "starting", "last_heartbeat_at": time.Now().UTC()})
	return result.RowsAffected == 1, result.Error
}
