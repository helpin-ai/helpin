package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// AcceptHandoff atomically assigns responsibility and records the receiver's explicit assessment.
// It does not close the Signal or infer that onboarding has finished.
func (r *CRMPlaybookExecutionRepository) AcceptHandoff(ctx context.Context, intent model.CRMPlaybookActionIntent, action model.CRMPlaybookHandoffAction, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, intent.WorkspaceID); err != nil {
			return err
		}
		var previous model.CRMSituationChange
		err := tx.Where("workspace_id = ? AND situation_id = ? AND command_key = ?", intent.WorkspaceID, intent.SituationID, "action:"+intent.SuggestionID).Take(&previous).Error
		if err == nil {
			if previous.CommandFingerprint != intent.ApprovedFingerprint {
				return ErrCRMPlaybookConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		_, source, err := lockedPlaybookRunSource(tx, intent.WorkspaceID, intent.RunID)
		if err != nil {
			return err
		}
		currentIntent, err := NewCRMPlaybookExecutionRepository(tx).ActionIntent(ctx, intent.WorkspaceID, intent.SuggestionID)
		if err != nil {
			return err
		}
		if source.Policy.Definition.Journey != "sales_handoff" || currentIntent == nil || currentIntent.ApprovedAt == nil || currentIntent.ApprovedByMemberID == nil || *currentIntent.ApprovedByMemberID != action.ReceivingMemberID || currentIntent.ApprovedFingerprint != intent.ApprovedFingerprint {
			return ErrCRMPlaybookExecutionBlocked
		}
		if err := requirePlaybookExecutionMember(tx, intent.WorkspaceID, action.ReceivingMemberID); err != nil {
			return err
		}
		current := source.Item.Situation
		before := model.CRMSituationState(current)
		after := model.CRMSituationState(current)
		after.OwnerMemberID = &action.ReceivingMemberID
		after.NextActionOwnerMemberID = &action.ReceivingMemberID
		basis := "human_assessment"
		found := false
		for i := range after.PlaybookMilestones {
			if after.PlaybookMilestones[i].Key == "handoff_accepted" {
				after.PlaybookMilestones[i] = model.CRMPlaybookMilestoneProgress{Key: "handoff_accepted", Status: "achieved", Summary: action.Summary, AssessedByMemberID: &action.ReceivingMemberID, AssessedAt: &now, Basis: &basis}
				found = true
			}
		}
		if !found {
			return ErrCRMPlaybookExecutionBlocked
		}
		after.NextStep = "Confirm the customer's onboarding plan"
		after.Attention = model.CRMSituationNeedsContext
		receipt := model.CRMSituationChange{ID: uuid.NewString(), WorkspaceID: intent.WorkspaceID, SituationID: intent.SituationID, Revision: current.Revision + 1,
			CommandKey: "action:" + intent.SuggestionID, CommandFingerprint: intent.ApprovedFingerprint, Operation: "update", ActorKind: "member", ActorMemberID: &action.ReceivingMemberID,
			Reason: "Accepted the sales-to-success handoff", Before: &before, After: after, CreatedAt: now}
		return saveSituationChange(tx, current, &receipt)
	})
}

// ActionWorkChange reads the canonical command receipt when reconciling a local effect.
func (r *CRMPlaybookExecutionRepository) ActionWorkChange(ctx context.Context, ws, situationID, commandKey string) (*model.CRMSituationChange, error) {
	var change model.CRMSituationChange
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND situation_id = ? AND command_key = ?", ws, situationID, commandKey).Take(&change).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &change, err
}

// ActionTask reads a task created with the canonical approval's stable external identity.
func (r *CRMPlaybookExecutionRepository) ActionTask(ctx context.Context, ws, externalID string) (*model.PMTask, error) {
	var tasks []model.PMTask
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND external_id = ?", ws, externalID).Limit(2).Find(&tasks).Error; err != nil {
		return nil, err
	}
	if len(tasks) > 1 {
		return nil, ErrCRMPlaybookConflict
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	return &tasks[0], nil
}
