package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	// ErrCRMSituationStale means the caller did not observe the current work revision.
	ErrCRMSituationStale = errors.New("customer work revision changed")
	// ErrCRMSituationCommandConflict rejects different intent under an existing retry key.
	ErrCRMSituationCommandConflict = errors.New("customer work command key conflicts")
	// ErrCRMSituationExecutionPending prevents closure from hiding an unconfirmed operation.
	ErrCRMSituationExecutionPending = errors.New("customer work has unconfirmed execution")
)

// ApplyCommand commits work state and its immutable receipt in one transaction.
// The pure transition callback owns business rules; it must not perform I/O.
func (r *CRMSituationRepository) ApplyCommand(
	ctx context.Context, ws, id, member string, req model.CRMSituationCommandRequest, fingerprint string,
	transition func(model.CRMSituation) (model.CRMSituationWorkState, error),
) (*model.CRMSituationCommandResult, error) {
	var result *model.CRMSituationCommandResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		var current model.CRMSituation
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", ws, id).Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read work for change: %w", err)
		}
		if err := situationSourceExists(tx, "workspace_members", ws, member); err != nil {
			return err
		}
		var receipt model.CRMSituationChange
		err = tx.Where("workspace_id = ? AND situation_id = ? AND command_key = ?", ws, id, req.CommandKey).Take(&receipt).Error
		if err == nil {
			if receipt.CommandFingerprint != fingerprint {
				return ErrCRMSituationCommandConflict
			}
			result = &model.CRMSituationCommandResult{Change: receipt, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("read work command receipt: %w", err)
		}
		if current.Revision != req.ExpectedRevision {
			return ErrCRMSituationStale
		}
		after, err := transition(current)
		if err != nil {
			return err
		}
		if err := validateSituationChange(tx, current, after, req); err != nil {
			return err
		}
		inFlight, err := situationInFlightCount(tx, ws, id)
		if err != nil {
			return err
		}
		if req.Operation == "close" && inFlight > 0 {
			return ErrCRMSituationExecutionPending
		}
		before := model.CRMSituationState(current)
		receipt = model.CRMSituationChange{
			ID: uuid.NewString(), WorkspaceID: ws, SituationID: id, Revision: current.Revision + 1,
			CommandKey: req.CommandKey, CommandFingerprint: fingerprint, Operation: req.Operation,
			ActorKind: "member", ActorMemberID: &member, Reason: req.Reason,
			Before: &before, After: after, InFlightActionCount: inFlight,
			CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		}
		if err := saveSituationChange(tx, current, &receipt); err != nil {
			return err
		}
		result = &model.CRMSituationCommandResult{Change: receipt}
		return nil
	})
	return result, err
}

// History reads bounded, newest-first changes. Existing records without earlier
// audit entries stay honest: no migration fabricates historical actors or events.
func (r *CRMSituationRepository) History(ctx context.Context, ws, id string, before int64, limit int) (*model.CRMSituationHistory, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMSituation{}).Where("workspace_id = ? AND id = ?", ws, id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	result := &model.CRMSituationHistory{Data: []model.CRMSituationChange{}}
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND situation_id = ?", ws, id)
	if before > 0 {
		query = query.Where("revision < ?", before)
	}
	if err := query.Order("revision DESC").Limit(limit + 1).Find(&result.Data).Error; err != nil {
		return nil, fmt.Errorf("read customer work history: %w", err)
	}
	if len(result.Data) > limit {
		result.Data = result.Data[:limit]
		revision := result.Data[limit-1].Revision
		result.NextBeforeRevision = &revision
	}
	return result, nil
}

func saveSituationChange(tx *gorm.DB, current model.CRMSituation, receipt *model.CRMSituationChange) error {
	state := receipt.After
	var milestones any
	if state.PlaybookMilestones != nil {
		encoded, err := json.Marshal(state.PlaybookMilestones)
		if err != nil {
			return err
		}
		milestones = string(encoded)
	}
	updates := map[string]any{
		"playbook_id": state.PlaybookID, "playbook_version_id": state.PlaybookVersionID,
		"playbook_applied_by_member_id": state.PlaybookAppliedByMemberID, "playbook_applied_at": state.PlaybookAppliedAt,
		"playbook_milestones": milestones,
		"owner_member_id":     state.OwnerMemberID, "next_action_owner_member_id": state.NextActionOwnerMemberID,
		"lifecycle": state.Lifecycle, "attention": state.Attention, "next_step": state.NextStep,
		"next_checkpoint_at": state.NextCheckpointAt, "outcome_kind": state.OutcomeKind,
		"outcome_summary": state.OutcomeSummary, "outcome_basis": state.OutcomeBasis,
		"duplicate_of_situation_id": state.DuplicateOfSituationID,
		"closed_at":                 state.ClosedAt, "closed_by_member_id": state.ClosedByMemberID,
		"revision": receipt.Revision, "updated_at": receipt.CreatedAt,
	}
	result := tx.Model(&model.CRMSituation{}).
		Where("workspace_id = ? AND id = ? AND revision = ?", current.WorkspaceID, current.ID, current.Revision).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("save customer work change: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrCRMSituationStale
	}
	if err := tx.Clauses(clause.Returning{}).Create(receipt).Error; err != nil {
		return fmt.Errorf("append customer work history: %w", err)
	}
	if err := syncSituationCheckpoint(tx, current.WorkspaceID, current.ID, receipt.Revision, state, receipt.CreatedAt); err != nil {
		return err
	}
	return syncSituationPlaybookExecution(tx, current.WorkspaceID, current.ID, state, receipt.CreatedAt)
}

func validateSituationChange(tx *gorm.DB, current model.CRMSituation, after model.CRMSituationWorkState, req model.CRMSituationCommandRequest) error {
	if req.Changes != nil {
		for _, owner := range []*model.CRMSituationOwnerChange{req.Changes.Owner, req.Changes.NextActionOwner} {
			if owner != nil && owner.MemberID != nil {
				if err := situationSourceExists(tx, "workspace_members", current.WorkspaceID, *owner.MemberID); err != nil {
					return err
				}
			}
		}
	}
	if after.Lifecycle == model.CRMSituationOpen &&
		(after.Attention == model.CRMSituationWaitingCustomer || after.Attention == model.CRMSituationWaitingWork) {
		owner := after.NextActionOwnerMemberID
		if owner == nil {
			owner = after.OwnerMemberID
		}
		if owner == nil {
			return ErrCRMSituationInvalidReference
		}
		if err := situationSourceExists(tx, "workspace_members", current.WorkspaceID, *owner); err != nil {
			return err
		}
	}
	if after.DuplicateOfSituationID != nil {
		var count int64
		if err := tx.Model(&model.CRMSituation{}).
			Where("workspace_id = ? AND id = ? AND id <> ?", current.WorkspaceID, *after.DuplicateOfSituationID, current.ID).
			Where("outcome_kind IS NULL OR outcome_kind <> 'duplicate'").Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrCRMSituationInvalidReference
		}
	}
	return nil
}

func situationInFlightCount(tx *gorm.DB, ws, id string) (int64, error) {
	var count int64
	err := tx.Table("crm_situation_references ref").
		Joins("JOIN crm_suggestions a ON a.workspace_id = ref.workspace_id AND a.id = ref.source_id").
		Where("ref.workspace_id = ? AND ref.situation_id = ? AND ref.kind = 'suggestion'", ws, id).
		Where("a.status = 'accepted' AND (a.execution_status IS NULL OR a.execution_status IN ('','pending','in_progress'))").
		Count(&count).Error
	return count, err
}

func lockSituationWorkspace(tx *gorm.DB, ws string) error {
	// Source enrollment, lifecycle changes and action admission share this short
	// lock. No external execution is performed under it. It also prevents phantom
	// source links or opposite duplicate links from racing a pause/close.
	var workspace struct{ ID string }
	if err := tx.Table("workspaces").Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", ws).Take(&workspace).Error; err != nil {
		return fmt.Errorf("lock customer work workspace: %w", err)
	}
	return nil
}

func recordSituationCreation(tx *gorm.DB, situation model.CRMSituation) error {
	kind := situation.OriginKind
	if kind == "manual" || kind == "" {
		kind = "member"
	}
	receipt := model.CRMSituationChange{
		ID: uuid.NewString(), WorkspaceID: situation.WorkspaceID, SituationID: situation.ID,
		Revision: situation.Revision, CommandKey: "system:created", CommandFingerprint: situation.CreationFingerprint,
		Operation: "created", ActorKind: kind, ActorMemberID: situation.CreatedByMemberID,
		After: model.CRMSituationState(situation), CreatedAt: situation.CreatedAt,
	}
	if err := tx.Create(&receipt).Error; err != nil {
		return fmt.Errorf("record customer work creation: %w", err)
	}
	return syncSituationCheckpoint(tx, situation.WorkspaceID, situation.ID, situation.Revision, receipt.After, situation.CreatedAt)
}
