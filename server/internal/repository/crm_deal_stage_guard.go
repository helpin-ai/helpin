package repository

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrCRMDealChanged requires review against the current deal before mutation.
var ErrCRMDealChanged = errors.New("deal changed; review the current stage")

// UpdateStageIfUnchanged updates only the approved stage, without overwriting other deal fields.
func (r *CRMDealRepository) UpdateStageIfUnchanged(ctx context.Context, expected model.CRMDeal, stageID string) error {
	q := r.db.WithContext(ctx).Model(&model.CRMDeal{}).Where("workspace_id = ? AND id = ? AND stage_id = ? AND pipeline_id = ?", expected.WorkspaceID, expected.ID, expected.StageID, expected.PipelineID).
		Where("EXISTS (SELECT 1 FROM crm_pipeline_stages s WHERE s.id = ? AND s.pipeline_id = ?)", stageID, expected.PipelineID)
	if expected.UpdatedAt.IsZero() {
		q = q.Where("updated_at IS NULL")
	} else {
		q = q.Where("updated_at = ?", expected.UpdatedAt)
	}
	result := q.Updates(map[string]any{"stage_id": stageID, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrCRMDealChanged
	}
	return nil
}

// UpdateStageForPlaybook commits the stage and its existing action receipt together.
// A lost response can then be inspected without inferring success from today's stage.
func (r *CRMDealRepository) UpdateStageForPlaybook(ctx context.Context, expected model.CRMDeal, stageID string, intent model.CRMPlaybookActionIntent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if intent.WorkspaceID != expected.WorkspaceID {
			return ErrCRMPlaybookExecutionBlocked
		}
		if err := lockSituationWorkspace(tx, intent.WorkspaceID); err != nil {
			return err
		}
		store := NewCRMPlaybookExecutionRepository(tx)
		current, err := store.ActionIntent(ctx, intent.WorkspaceID, intent.SuggestionID)
		if err != nil {
			return err
		}
		if current == nil || current.ApprovedAt == nil || current.ApprovedFingerprint == "" || current.ApprovedFingerprint != intent.ApprovedFingerprint {
			return ErrCRMPlaybookExecutionBlocked
		}
		if current.ResultType == "crm_deal" && current.ResultID != nil && *current.ResultID == expected.ID {
			return nil
		}
		if _, _, err := lockedPlaybookRunSource(tx, intent.WorkspaceID, intent.RunID); err != nil {
			return err
		}
		if err := NewCRMDealRepository(tx).UpdateStageIfUnchanged(ctx, expected, stageID); err != nil {
			return err
		}
		updated := expected
		updated.StageID = stageID
		if err := enterWonDeal(tx, updated, expected.StageID, time.Now().UTC()); err != nil {
			return err
		}
		return store.FinishAction(ctx, intent, "succeeded", "crm_deal", &expected.ID, nil, time.Now().UTC())
	})
}
