package repository

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// InspectAction records the approving teammate's explicit destination inspection.
// It cannot repeat the operation or replace an already-confirmed executor result.
func (r *CRMPlaybookExecutionRepository) InspectAction(ctx context.Context, ws, id, member string, req model.CRMPlaybookActionInspection, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		if err := requirePlaybookExecutionMember(tx, ws, member); err != nil {
			return err
		}
		intent, err := NewCRMPlaybookExecutionRepository(tx).ActionIntent(ctx, ws, id)
		if err != nil {
			return err
		}
		if intent == nil || intent.ApprovedByMemberID == nil || *intent.ApprovedByMemberID != member || intent.ApprovedAt == nil || now.Before(intent.ApprovedAt.Add(5*time.Minute)) {
			return ErrCRMPlaybookExecutionBlocked
		}
		action, err := NewCRMSuggestionRepository(tx).GetByID(ctx, ws, id)
		if err != nil {
			return err
		}
		if action == nil || action.Revision != req.Revision || action.Status != "accepted" || action.ExecutionStatus != "in_progress" {
			return ErrCRMPlaybookStale
		}
		status := "failed"
		if req.Outcome == "completed" {
			status = "succeeded"
		}
		action.Context["playbook_action_inspection"] = model.JSONB{"outcome": req.Outcome, "evidence": req.Evidence, "member_id": member, "recorded_at": now}
		if err := tx.Model(&model.CRMSuggestion{}).Where("workspace_id=? AND id=?", ws, id).Update("context", action.Context).Error; err != nil {
			return err
		}
		message := "Result recorded by the approving teammate after inspecting the destination."
		return NewCRMPlaybookExecutionRepository(tx).FinishAction(ctx, *intent, status, "human_verified", nil, &message, now)
	})
}
