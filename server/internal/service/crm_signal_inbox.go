package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ListInbox reads one globally filtered daily queue, including standalone proposals.
func (s *CRMSituationService) ListInbox(ctx context.Context, ws string, filters model.CRMSignalInboxFilters) (*model.CRMSignalInboxList, error) {
	actor, err := s.authorize(ctx, ws, authorization.PermCRMRead)
	if err != nil {
		return nil, err
	}
	approvals := filters.Navigation.State == "needs_approval"
	if approvals {
		filters.Navigation.State = "all"
	}
	filters.Navigation, err = normalizeSituationFilters(filters.Navigation)
	if err != nil {
		return nil, err
	}
	if approvals {
		filters.Navigation.State = "needs_approval"
	}
	if filters.Sort == "" {
		filters.Sort = "priority"
	}
	if filters.Sort != "priority" && filters.Sort != "recommended" && filters.Sort != "newest" && filters.Sort != "oldest" {
		return nil, ErrCRMSituationInput
	}
	return s.store.ListInbox(ctx, ws, actor.WorkspaceMemberID, filters)
}

// InboxRecommendation reads the original revision and evidence, including after a decision.
func (s *CRMSituationService) InboxRecommendation(ctx context.Context, ws, id string) (*model.CRMInboxRecommendation, error) {
	if _, err := s.authorize(ctx, ws, authorization.PermCRMRead); err != nil {
		return nil, err
	}
	if !validSituationID(id) {
		return nil, ErrCRMSituationInput
	}
	item, err := s.store.InboxRecommendation(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrCRMSituationNotFound
	}
	return item, nil
}

// DecideInboxRecommendation uses the original suggestion's atomic claim and guard.
// Approval cannot bypass a concurrently linked situation's paused/closed state.
func (s *CRMSituationService) DecideInboxRecommendation(ctx context.Context, ws, id, revision, decision, reason string, edits map[string]interface{}) (*model.CRMSuggestion, error) {
	if _, err := s.authorize(ctx, ws, authorization.PermCRMEdit); err != nil {
		return nil, err
	}
	if !validSituationID(id) || revision == "" || s.actions == nil {
		return nil, ErrCRMSituationInput
	}
	item, err := s.store.InboxRecommendation(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrCRMSituationNotFound
	}
	// Once linked, use the same decision path as the canonical drawer.
	if len(item.LinkedSituations) > 0 {
		return s.DecideAction(ctx, ws, item.LinkedSituations[0], id, revision, decision, reason, edits)
	}
	switch decision {
	case "accept":
		return s.actions.AcceptSuggestionRevision(ctx, ws, id, revision, edits)
	case "dismiss":
		if !validSignalDismissalReason(reason) {
			return nil, ErrCRMSituationInput
		}
		return s.actions.DismissSuggestionRevision(ctx, ws, id, revision, reason)
	default:
		return nil, ErrCRMSituationInput
	}
}
