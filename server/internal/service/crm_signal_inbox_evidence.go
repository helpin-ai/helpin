package service

import (
	"context"
	"regexp"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ComposeCRMInboxSignalGroups uses the same grouping, decay and visibility floor
// as the evidence feed. Shadow rules and absent automation policies do not hide
// evidence; this function never grants execution eligibility or persists work.
func ComposeCRMInboxSignalGroups(ctx context.Context, repo *repository.CRMSignalRepository, ws string, signals []model.CRMSignal) ([]model.CRMSignalAccountStory, error) {
	service := NewCRMSignalService(repo, nil)
	profile := service.loadSignalScoringProfile(ctx, ws)
	now := time.Now().UTC()
	for i := range signals {
		profile.scoreSignal(&signals[i], now)
	}
	groups := composeSignalStories(signals, profile, now)
	settings, err := repo.GetSignalRoutingSettings(ctx, ws)
	if err != nil {
		return nil, err
	}
	visible := groups[:0]
	for _, group := range groups {
		if group.Priority >= settings.MinimumLanePriority {
			visible = append(visible, group)
		}
	}
	return visible, nil
}

var inboxSignalGroupID = regexp.MustCompile(`^[a-f0-9]{24}$`)

// InboxSignalGroup reads source evidence without importing work or starting agents.
func (s *CRMSituationService) InboxSignalGroup(ctx context.Context, ws, id string) (*model.CRMSignalAccountStory, error) {
	if _, err := s.authorize(ctx, ws, authorization.PermCRMRead); err != nil {
		return nil, err
	}
	if !inboxSignalGroupID.MatchString(id) {
		return nil, ErrCRMSituationInput
	}
	group, err := s.store.InboxSignalGroup(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrCRMSituationNotFound
	}
	return group, nil
}
