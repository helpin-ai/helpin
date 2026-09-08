package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// DecideAction binds one decision to one linked canonical suggestion revision.
// Other actions and the customer outcome are never implicitly completed.
func (s *CRMSituationService) DecideAction(ctx context.Context, ws, id, actionID, revision, decision, reason string, edits map[string]interface{}) (*model.CRMSuggestion, error) {
	if _, err := s.authorize(ctx, ws, authorization.PermCRMEdit); err != nil {
		return nil, err
	}
	if !validSituationID(actionID) || revision == "" || s.actions == nil {
		return nil, ErrCRMSituationInput
	}
	if decision == "dismiss" && !validSignalDismissalReason(reason) {
		return nil, ErrCRMSituationInput
	}
	item, err := s.GetByID(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	if item.Situation.Lifecycle != model.CRMSituationOpen {
		return nil, ErrCRMSuggestionStale
	}
	var action *model.CRMSuggestion
	for i := range item.Actions {
		if item.Actions[i].ID == actionID {
			action = &item.Actions[i]
			break
		}
	}
	if action == nil {
		return nil, ErrCRMSituationNotFound
	}
	if action.Revision != revision {
		return nil, ErrCRMSuggestionStale
	}
	switch decision {
	case "accept":
		return s.actions.AcceptSuggestionRevision(ctx, ws, actionID, revision, edits)
	case "dismiss":
		return s.actions.DismissSuggestionRevision(ctx, ws, actionID, revision, reason)
	default:
		return nil, ErrCRMSituationInput
	}
}
