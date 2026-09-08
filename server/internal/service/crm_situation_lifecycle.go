package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrCRMSituationTransition rejects an operation that does not apply to current work.
var ErrCRMSituationTransition = errors.New("customer work transition is not applicable")

// Command applies a human-authorized change through the same atomic lifecycle
// boundary that future Flow adapters must use. It never launches an executor.
func (s *CRMSituationService) Command(ctx context.Context, ws, id string, req model.CRMSituationCommandRequest) (*model.CRMSituationCommandResult, error) {
	actor, err := s.authorize(ctx, ws, authorization.PermCRMEdit)
	if err != nil {
		return nil, err
	}
	if !validSituationID(id) {
		return nil, ErrCRMSituationInput
	}
	req, err = normalizeSituationCommand(req)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(struct {
		Request model.CRMSituationCommandRequest
		Member  string
	}{Request: req, Member: actor.WorkspaceMemberID})
	if err != nil {
		return nil, fmt.Errorf("encode customer work command: %w", err)
	}
	fingerprint := sha256.Sum256(encoded)
	result, err := s.store.ApplyCommand(ctx, ws, id, actor.WorkspaceMemberID, req,
		hex.EncodeToString(fingerprint[:]), func(current model.CRMSituation) (model.CRMSituationWorkState, error) {
			return transitionSituation(current, req, actor.WorkspaceMemberID, time.Now().UTC().Truncate(time.Microsecond))
		})
	if err == nil && result == nil {
		return nil, ErrCRMSituationNotFound
	}
	if err == nil && s.automation != nil {
		if stopErr := s.automation.AfterSituationCommand(ctx, ws, id, actor.UserID, *result); stopErr != nil {
			return result, stopErr
		}
	}
	return result, err
}

// History returns immutable changes without reading or modifying source feedback.
func (s *CRMSituationService) History(ctx context.Context, ws, id string, before int64, limit int) (*model.CRMSituationHistory, error) {
	if _, err := s.authorize(ctx, ws, authorization.PermCRMRead); err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = 50
	}
	if !validSituationID(id) || before < 0 || limit < 1 || limit > 100 {
		return nil, ErrCRMSituationInput
	}
	result, err := s.store.History(ctx, ws, id, before, limit)
	if err == nil && result == nil {
		return nil, ErrCRMSituationNotFound
	}
	return result, err
}

func normalizeSituationCommand(req model.CRMSituationCommandRequest) (model.CRMSituationCommandRequest, error) {
	req.CommandKey, req.Reason = strings.TrimSpace(req.CommandKey), strings.TrimSpace(req.Reason)
	if req.CommandKey == "" || len(req.CommandKey) > 200 || strings.HasPrefix(req.CommandKey, "system:") ||
		req.ExpectedRevision < 1 || len(req.Reason) > 2000 {
		return req, ErrCRMSituationInput
	}
	switch req.Operation {
	case "update":
		if req.Changes == nil || req.Outcome != nil {
			return req, ErrCRMSituationInput
		}
		changes := *req.Changes
		if err := normalizeSituationChanges(&changes); err != nil {
			return req, err
		}
		req.Changes = &changes
	case "pause", "resume":
		if req.Changes != nil || req.Outcome != nil || (req.Operation == "pause" && req.Reason == "") {
			return req, ErrCRMSituationInput
		}
	case "close":
		if req.Changes != nil || req.Outcome == nil {
			return req, ErrCRMSituationInput
		}
		outcome := *req.Outcome
		outcome.Summary = strings.TrimSpace(outcome.Summary)
		if outcome.Summary == "" || len(outcome.Summary) > 4000 ||
			(outcome.Kind != "achieved" && outcome.Kind != "not_pursued" && outcome.Kind != "invalid" && outcome.Kind != "duplicate") ||
			(outcome.Kind == "duplicate") != (outcome.DuplicateOfSituationID != nil) ||
			(outcome.DuplicateOfSituationID != nil && !validSituationID(*outcome.DuplicateOfSituationID)) {
			return req, ErrCRMSituationInput
		}
		req.Outcome = &outcome
	default:
		return req, ErrCRMSituationInput
	}
	return req, nil
}

func normalizeSituationChanges(changes *model.CRMSituationChanges) error {
	if changes.Owner == nil && changes.NextActionOwner == nil && changes.NextStep == nil && changes.Attention == nil && changes.Checkpoint == nil {
		return ErrCRMSituationInput
	}
	for _, owner := range []*model.CRMSituationOwnerChange{changes.Owner, changes.NextActionOwner} {
		if owner != nil && owner.MemberID != nil && !validSituationID(*owner.MemberID) {
			return ErrCRMSituationInput
		}
	}
	if changes.NextStep != nil {
		step := strings.TrimSpace(*changes.NextStep)
		if len(step) > 1000 {
			return ErrCRMSituationInput
		}
		changes.NextStep = &step
	}
	if changes.Attention != nil {
		switch *changes.Attention {
		case model.CRMSituationNeedsContext, model.CRMSituationFollowUpDue,
			model.CRMSituationWaitingCustomer, model.CRMSituationWaitingWork:
		default:
			// Approval and failure are derived from canonical actions, not a manual dropdown.
			return ErrCRMSituationInput
		}
	}
	if changes.Checkpoint != nil && changes.Checkpoint.At != nil {
		at := changes.Checkpoint.At.UTC().Truncate(time.Microsecond)
		if at.Year() < 2000 || at.Year() > 9999 {
			return ErrCRMSituationInput
		}
		changes.Checkpoint = &model.CRMSituationCheckpointChange{At: &at}
	}
	return nil
}

func transitionSituation(current model.CRMSituation, req model.CRMSituationCommandRequest, member string, now time.Time) (model.CRMSituationWorkState, error) {
	state := model.CRMSituationState(current)
	if current.Lifecycle == model.CRMSituationClosed {
		return state, ErrCRMSituationTransition
	}
	switch req.Operation {
	case "update":
		applySituationChanges(&state, *req.Changes)
		if reflect.DeepEqual(state, model.CRMSituationState(current)) {
			return state, ErrCRMSituationTransition
		}
	case "pause":
		if state.Lifecycle != model.CRMSituationOpen {
			return state, ErrCRMSituationTransition
		}
		state.Lifecycle = model.CRMSituationPaused
	case "resume":
		if state.Lifecycle != model.CRMSituationPaused {
			return state, ErrCRMSituationTransition
		}
		state.Lifecycle = model.CRMSituationOpen
	case "close":
		state.Lifecycle = model.CRMSituationClosed
		state.OutcomeKind, state.OutcomeSummary = &req.Outcome.Kind, &req.Outcome.Summary
		basis := "human_assessment"
		state.OutcomeBasis, state.DuplicateOfSituationID = &basis, req.Outcome.DuplicateOfSituationID
		state.ClosedAt, state.ClosedByMemberID = &now, &member
		state.NextCheckpointAt = nil
	}
	if state.Lifecycle == model.CRMSituationOpen &&
		(state.Attention == model.CRMSituationWaitingCustomer || state.Attention == model.CRMSituationWaitingWork) &&
		(state.NextCheckpointAt == nil || state.NextStep == "" || (state.OwnerMemberID == nil && state.NextActionOwnerMemberID == nil)) {
		return state, ErrCRMSituationInput
	}
	return state, nil
}

func applySituationChanges(state *model.CRMSituationWorkState, changes model.CRMSituationChanges) {
	if changes.Owner != nil {
		state.OwnerMemberID = changes.Owner.MemberID
	}
	if changes.NextActionOwner != nil {
		state.NextActionOwnerMemberID = changes.NextActionOwner.MemberID
	}
	if changes.NextStep != nil {
		state.NextStep = *changes.NextStep
	}
	if changes.Attention != nil {
		state.Attention = *changes.Attention
	}
	if changes.Checkpoint != nil {
		state.NextCheckpointAt = changes.Checkpoint.At
	}
}
