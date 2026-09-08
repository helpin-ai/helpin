package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Apply pins an explicitly confirmed policy to existing work under the lifecycle lock.
// It preserves ownership, next-step commitments, source evidence and canonical actions.
func (r *CRMPlaybookRepository) Apply(ctx context.Context, ws, id, actor string, req model.ApplyCRMPlaybookRequest, fingerprint string) (*model.CRMSituationCommandResult, error) {
	return r.applyWithRouting(ctx, ws, id, actor, req, fingerprint, nil)
}

func (r *CRMPlaybookRepository) applyWithRouting(ctx context.Context, ws, id, actor string, req model.ApplyCRMPlaybookRequest, fingerprint string, owner *string) (*model.CRMSituationCommandResult, error) {
	return r.changeParticipant(ctx, ws, id, req.SituationID, actor, req.CommandKey, fingerprint, req.ExpectedSituationRevision, "apply_playbook", "Playbook applied",
		func(tx *gorm.DB, current model.CRMSituation) (model.CRMSituationWorkState, error) {
			state := model.CRMSituationState(current)
			if !req.Confirmed || current.PlaybookID != nil {
				return state, ErrCRMPlaybookConflict
			}
			var pb model.CRMPlaybook
			err := tx.Where("workspace_id = ? AND id = ?", ws, id).Take(&pb).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return state, ErrCRMPlaybookUnavailable
			}
			if err != nil {
				return state, err
			}
			if pb.Revision != req.ExpectedPlaybookRevision || pb.PublishedVersionID == nil || *pb.PublishedVersionID != req.VersionID {
				return state, ErrCRMPlaybookStale
			}
			if !pb.AcceptingCustomers {
				return state, ErrCRMPlaybookUnavailable
			}
			var version model.CRMPlaybookVersion
			if err := tx.Where("workspace_id = ? AND playbook_id = ? AND id = ?", ws, id, req.VersionID).Take(&version).Error; err != nil {
				return state, err
			}
			if err := validatePlaybookReferences(tx, ws, version.Definition); err != nil {
				return state, err
			}
			query, err := eligiblePlaybookSignals(tx, ws, version.Definition)
			if err != nil {
				return state, err
			}
			var eligible int64
			if err := query.Where("s.id = ?", current.ID).Select("COUNT(*)").Scan(&eligible).Error; err != nil {
				return state, err
			}
			if eligible != 1 {
				return state, ErrCRMPlaybookUnavailable
			}
			now := time.Now().UTC().Truncate(time.Microsecond)
			state.PlaybookID, state.PlaybookVersionID, state.PlaybookAppliedByMemberID, state.PlaybookAppliedAt = &id, &version.ID, &actor, &now
			if owner != nil {
				if err := requirePlaybookExecutionMember(tx, ws, *owner); err != nil {
					return state, err
				}
				state.OwnerMemberID, state.NextActionOwnerMemberID = owner, owner
			}
			state.PlaybookMilestones = make([]model.CRMPlaybookMilestoneProgress, 0, len(version.Definition.Milestones))
			for _, milestone := range version.Definition.Milestones {
				state.PlaybookMilestones = append(state.PlaybookMilestones, model.CRMPlaybookMilestoneProgress{Key: milestone.Key, Status: "pending"})
			}
			return state, nil
		})
}

// AssessMilestone records a human assessment without approving work or closing the Signal.
func (r *CRMPlaybookRepository) AssessMilestone(ctx context.Context, ws, id, situationID, actor string, req model.CRMPlaybookMilestoneRequest, fingerprint string) (*model.CRMSituationCommandResult, error) {
	return r.changeParticipant(ctx, ws, id, situationID, actor, req.CommandKey, fingerprint, req.ExpectedRevision, "update_milestone", req.Summary,
		func(tx *gorm.DB, current model.CRMSituation) (model.CRMSituationWorkState, error) {
			state := model.CRMSituationState(current)
			if current.PlaybookID == nil || *current.PlaybookID != id || current.PlaybookVersionID == nil {
				return state, ErrCRMPlaybookUnavailable
			}
			var version model.CRMPlaybookVersion
			if err := tx.Where("workspace_id = ? AND playbook_id = ? AND id = ?", ws, id, *current.PlaybookVersionID).Take(&version).Error; err != nil {
				return state, err
			}
			valid := false
			for _, milestone := range version.Definition.Milestones {
				valid = valid || milestone.Key == req.MilestoneKey
			}
			if !valid {
				return state, ErrCRMPlaybookUnavailable
			}
			for i := range state.PlaybookMilestones {
				if state.PlaybookMilestones[i].Key != req.MilestoneKey {
					continue
				}
				now, basis := time.Now().UTC().Truncate(time.Microsecond), "human_assessment"
				state.PlaybookMilestones[i] = model.CRMPlaybookMilestoneProgress{Key: req.MilestoneKey, Status: req.Status, Summary: req.Summary, AssessedByMemberID: &actor, AssessedAt: &now, Basis: &basis}
				return state, nil
			}
			return state, ErrCRMPlaybookUnavailable
		})
}

func (r *CRMPlaybookRepository) changeParticipant(ctx context.Context, ws, playbookID, id, actor, key, fingerprint string, revision int64, operation, reason string,
	transition func(*gorm.DB, model.CRMSituation) (model.CRMSituationWorkState, error),
) (*model.CRMSituationCommandResult, error) {
	var result *model.CRMSituationCommandResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		if err := situationSourceExists(tx, "workspace_members", ws, actor); err != nil {
			return err
		}
		var current model.CRMSituation
		err := tx.Where("workspace_id = ? AND id = ?", ws, id).Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var receipt model.CRMSituationChange
		err = tx.Where("workspace_id = ? AND situation_id = ? AND command_key = ?", ws, id, key).Take(&receipt).Error
		if err == nil {
			if receipt.CommandFingerprint != fingerprint || receipt.After.PlaybookID == nil || *receipt.After.PlaybookID != playbookID {
				return ErrCRMPlaybookConflict
			}
			result = &model.CRMSituationCommandResult{Change: receipt, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if current.Revision != revision {
			return ErrCRMSituationStale
		}
		if current.Lifecycle != model.CRMSituationOpen {
			return ErrCRMPlaybookUnavailable
		}
		after, err := transition(tx, current)
		if err != nil {
			return err
		}
		before := model.CRMSituationState(current)
		inFlight, err := situationInFlightCount(tx, ws, id)
		if err != nil {
			return err
		}
		receipt = model.CRMSituationChange{ID: uuid.NewString(), WorkspaceID: ws, SituationID: id, Revision: current.Revision + 1,
			CommandKey: key, CommandFingerprint: fingerprint, Operation: operation, ActorKind: "member", ActorMemberID: &actor,
			Reason: reason, Before: &before, After: after, InFlightActionCount: inFlight, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
		if err := saveSituationChange(tx, current, &receipt); err != nil {
			return err
		}
		result = &model.CRMSituationCommandResult{Change: receipt}
		return nil
	})
	return result, err
}
