package service

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// QualifySituationSignals reuses rule/version/trust/priority and workspace gates.
// Recording customer work is independent of notification deduplication and an
// unrelated task on the account. This grants no permission to execute an action.
func (s *CRMSignalService) QualifySituationSignals(ctx context.Context, ws string, signals []model.CRMSignal) ([]model.CRMSignal, error) {
	rollout, err := s.signalRepo.GetSignalRolloutSettings(ctx, ws)
	if err != nil {
		return nil, err
	}
	if rollout.Mode != model.CRMSignalRolloutLive {
		return nil, nil
	}
	policy, err := s.signalRepo.GetActiveRoutingPolicy(ctx, ws)
	if err != nil || policy == nil {
		return nil, err
	}
	profile := s.loadSignalScoringProfile(ctx, ws)
	// Unlike presentation scoring, enrollment must report unavailable policy.
	rules, err := s.signalRepo.ListLatestRuleScoringConfigs(ctx, ws)
	if err != nil {
		return nil, err
	}
	profile.rules = rules
	var qualified []model.CRMSignal
	for _, signal := range signals {
		if signal.WorkspaceID != ws || signal.SupersededAt != nil || signal.ActedAt != nil ||
			model.CRMSituationCategoryForMotion(signal.CommercialMotion) == "" ||
			(signal.CompanyID == nil && signal.ContactID == nil && signal.DealID == nil) {
			continue
		}
		profile.scoreSignal(&signal, time.Now().UTC())
		signal.ActivationBlockers = signalPolicyBlockers(&signal, policy, profile)
		signal.ActivationEligible = len(signal.ActivationBlockers) == 0
		if signal.ActivationEligible {
			qualified = append(qualified, signal)
		}
	}
	return qualified, nil
}
