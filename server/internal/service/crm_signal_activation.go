package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const signalDeliveryClaimTimeout = 5 * time.Minute

func (s *CRMSignalService) ListRuleConfigs(ctx context.Context, workspaceID string) ([]model.CRMSignalRuleConfig, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	configs, err := s.signalRepo.ListLatestRuleConfigs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	rows := make([]model.CRMSignalRuleConfig, 0, len(configs))
	for _, config := range configs {
		rows = append(rows, config)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].RuleKey < rows[j].RuleKey })
	return rows, nil
}

func validSignalDismissalReason(reason string) bool {
	switch reason {
	case model.CRMSignalDismissIncorrectEvidence, model.CRMSignalDismissWrongEntity,
		model.CRMSignalDismissDuplicate, model.CRMSignalDismissIrrelevant,
		model.CRMSignalDismissHandled, model.CRMSignalDismissBadTiming:
		return true
	default:
		return false
	}
}

func (s *CRMSignalService) RecordSignalFeedback(ctx context.Context, workspaceID, signalID, memberID, action, reason string) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(signalID) == "" || strings.TrimSpace(memberID) == "" {
		return fmt.Errorf("workspace_id, signal_id, and member_id are required")
	}
	if action != model.CRMSignalFeedbackReviewed && action != model.CRMSignalFeedbackActed && action != model.CRMSignalFeedbackDismissed {
		return fmt.Errorf("invalid feedback action")
	}
	var reasonPtr *string
	if action == model.CRMSignalFeedbackDismissed {
		reason = strings.TrimSpace(reason)
		if !validSignalDismissalReason(reason) {
			return fmt.Errorf("a valid dismissal reason is required")
		}
		reasonPtr = &reason
	}
	signal, err := s.signalRepo.GetSignal(ctx, workspaceID, signalID)
	if err != nil {
		return err
	}
	if signal == nil {
		return fmt.Errorf("CRM signal not found")
	}
	return s.signalRepo.RecordSignalFeedback(ctx, signal, memberID, action, reasonPtr, time.Now().UTC())
}

// GetSignalRoutingSettings returns mutable workspace routing defaults.
func (s *CRMSignalService) GetSignalRoutingSettings(ctx context.Context, workspaceID string) (*model.CRMSignalRoutingSettings, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.signalRepo.GetSignalRoutingSettings(ctx, workspaceID)
}

// UpdateSignalRoutingSettings saves mutable workspace routing defaults.
func (s *CRMSignalService) UpdateSignalRoutingSettings(
	ctx context.Context,
	workspaceID string,
	req model.UpdateCRMSignalRoutingSettingsRequest,
) (*model.CRMSignalRoutingSettings, error) {
	settings, err := s.signalRepo.GetSignalRoutingSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if req.ClearDefaultSignalOwner {
		settings.DefaultSignalOwnerMemberID = nil
	} else if req.DefaultSignalOwnerMemberID != nil {
		settings.DefaultSignalOwnerMemberID = req.DefaultSignalOwnerMemberID
	}
	if req.MinimumLanePriority != nil {
		if *req.MinimumLanePriority < 0 || *req.MinimumLanePriority > 100 {
			return nil, fmt.Errorf("minimum_lane_priority must be between 0 and 100")
		}
		settings.MinimumLanePriority = *req.MinimumLanePriority
	}
	if err := s.signalRepo.SaveSignalRoutingSettings(ctx, settings); err != nil {
		return nil, err
	}
	return s.signalRepo.GetSignalRoutingSettings(ctx, workspaceID)
}

// GetSignalRolloutSettings returns the current shadow/live state.
func (s *CRMSignalService) GetSignalRolloutSettings(ctx context.Context, workspaceID string) (*model.CRMSignalRolloutSettings, error) {
	return s.signalRepo.GetSignalRolloutSettings(ctx, workspaceID)
}

// ActivateSignalRollout returns a workspace to live. Workspaces are live by
// default, so this is the recovery path for an explicit shadow opt-out and is
// deliberately not gated. Readiness stays observable through SignalShadowGate.
func (s *CRMSignalService) ActivateSignalRollout(ctx context.Context, workspaceID, memberID string) (*model.CRMSignalRolloutSettings, error) {
	if gate, err := s.signalRepo.GetSignalShadowGate(ctx, workspaceID); err != nil {
		slog.WarnContext(ctx, "signal shadow gate unavailable during activation", "error", err, "workspace_id", workspaceID)
	} else if !gate.Eligible {
		slog.WarnContext(ctx, "activating motion-aware signals below gate thresholds",
			"workspace_id", workspaceID, "observations", gate.ObservationCount,
			"unmapped_rate", gate.UnmappedObservationRate, "duplicate_rate", gate.DuplicateFingerprintRate,
			"immutable_violations", gate.ImmutableMeaningViolations)
	}
	if err := s.signalRepo.ActivateSignalRollout(ctx, workspaceID, memberID, time.Now().UTC()); err != nil {
		return nil, err
	}
	return s.signalRepo.GetSignalRolloutSettings(ctx, workspaceID)
}

func (s *CRMSignalService) SignalPrecisionReport(ctx context.Context, workspaceID string) ([]model.CRMSignalPrecisionRow, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.signalRepo.ListSignalPrecision(ctx, workspaceID)
}

// SignalOutcomeCalibrationReport joins signals to subsequent subscription state.
func (s *CRMSignalService) SignalOutcomeCalibrationReport(ctx context.Context, workspaceID string, horizonDays int) ([]model.CRMSignalOutcomeCalibrationRow, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.signalRepo.ListSignalOutcomeCalibration(ctx, workspaceID, horizonDays)
}

func (s *CRMSignalService) CreateRoutingPolicy(ctx context.Context, workspaceID, memberID string, req model.CreateCRMSignalRoutingPolicyRequest) (*model.CRMSignalRoutingPolicy, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(memberID) == "" {
		return nil, fmt.Errorf("workspace_id and member_id are required")
	}
	if req.MinimumPriority <= 0 || req.MinimumPriority > 100 {
		return nil, fmt.Errorf("minimum_priority must be between 0 and 100")
	}
	if req.RequiredTrust == "" {
		req.RequiredTrust = "verified"
	}
	if _, ok := signalTrustRank(req.RequiredTrust); !ok {
		return nil, fmt.Errorf("invalid required_trust")
	}
	if len(req.Channels) == 0 {
		req.Channels = []string{model.CRMSignalDeliveryFeed}
	}
	seen := map[string]bool{}
	channels := make([]string, 0, len(req.Channels))
	for _, channel := range req.Channels {
		channel = strings.TrimSpace(channel)
		if channel != model.CRMSignalDeliveryFeed && channel != model.CRMSignalDeliveryNotification && channel != model.CRMSignalDeliveryDigest {
			return nil, fmt.Errorf("invalid routing channel")
		}
		if !seen[channel] {
			seen[channel] = true
			channels = append(channels, channel)
		}
	}
	payload, _ := json.Marshal(channels)
	routeToOwner := true
	if req.RouteToOwner != nil {
		routeToOwner = *req.RouteToOwner
	}
	policy := &model.CRMSignalRoutingPolicy{
		WorkspaceID: workspaceID, Enabled: true, MinimumPriority: req.MinimumPriority,
		RequiredTrust: req.RequiredTrust, RouteToOwner: routeToOwner,
		DestinationTeamID: req.DestinationTeamID, Channels: model.JSONBlob(payload), CreatedByMemberID: memberID,
	}
	if err := s.signalRepo.CreateRoutingPolicy(ctx, policy); err != nil {
		return nil, err
	}
	return policy, nil
}

func (s *CRMSignalService) GetRoutingPolicy(ctx context.Context, workspaceID string) (*model.CRMSignalRoutingPolicy, error) {
	return s.signalRepo.GetActiveRoutingPolicy(ctx, workspaceID)
}

func (s *CRMSignalService) ActivateRoutingPolicyVersion(ctx context.Context, workspaceID string, version int) error {
	if version < 1 {
		return fmt.Errorf("invalid routing policy version")
	}
	return s.signalRepo.ActivateRoutingPolicyVersion(ctx, workspaceID, version)
}

func (s *CRMSignalService) ActivateRuleVersion(ctx context.Context, workspaceID, ruleKey string, version int) error {
	if strings.TrimSpace(ruleKey) == "" || version < 1 {
		return fmt.Errorf("rule_key and a positive version are required")
	}
	return s.signalRepo.ActivateRuleVersion(ctx, workspaceID, ruleKey, version)
}

func signalTrustRank(trust string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(trust)) {
	case "untrusted":
		return 0, true
	case "unknown":
		return 1, true
	case "probabilistic":
		return 2, true
	case "verified":
		return 3, true
	default:
		return 0, false
	}
}

func (s *CRMSignalService) setSignalActivation(ctx context.Context, signal *model.CRMSignal, policy *model.CRMSignalRoutingPolicy, profile signalScoringProfile) {
	blockers := []string{}
	if signal.Metadata["needs_customer_context"] == true || signal.CommercialMotion == model.CRMCommercialMotionNeedsContext {
		blockers = append(blockers, "needs_customer_context")
	}
	if signal.DismissedAt != nil {
		blockers = append(blockers, "dismissed")
	}
	if signal.BusinessPriority < policy.MinimumPriority {
		blockers = append(blockers, "below_priority_threshold")
	}
	actualTrust, actualOK := signalTrustRank(signal.EvidenceIdentityTrust)
	requiredTrust, _ := signalTrustRank(policy.RequiredTrust)
	if !actualOK || actualTrust < requiredTrust {
		blockers = append(blockers, "identity_trust")
	}
	if signal.RuleKey == nil || signal.RuleVersion == nil {
		blockers = append(blockers, "unversioned_rule")
	} else if rule, ok := profile.rules[*signal.RuleKey]; !ok || rule.Version != *signal.RuleVersion || !rule.ActivationEligible || rule.ShadowMode {
		blockers = append(blockers, "rule_not_activation_eligible")
	}
	if delivered, err := s.signalRepo.HasSignalDelivery(ctx, signal.ID, policy.ID); err != nil {
		blockers = append(blockers, "dedupe_check_unavailable")
	} else if delivered {
		blockers = append(blockers, "already_delivered")
	}
	openTaskID, err := s.signalRepo.FindOpenTaskForSignal(ctx, *signal)
	if err != nil {
		blockers = append(blockers, "open_task_check_unavailable")
	} else if openTaskID != nil {
		signal.ExistingOpenTaskID = openTaskID
		blockers = append(blockers, "existing_open_task")
	}
	signal.ActivationBlockers = blockers
	signal.ActivationEligible = len(blockers) == 0
}

func (s *CRMSignalService) ListActivationSignals(ctx context.Context, workspaceID string, limit int) ([]model.CRMSignal, error) {
	if limit < 1 || limit > 50 {
		limit = 20
	}
	rollout, err := s.signalRepo.GetSignalRolloutSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if rollout.Mode != model.CRMSignalRolloutLive {
		return []model.CRMSignal{}, nil
	}
	policy, err := s.signalRepo.GetActiveRoutingPolicy(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return []model.CRMSignal{}, nil
	}
	filters := model.CRMSignalListFilters{}
	signals, err := s.signalRepo.ListWorkspaceSignalCandidates(ctx, workspaceID, filters, time.Now().UTC(), 500)
	if err != nil {
		return nil, err
	}
	profile := s.loadSignalScoringProfile(ctx, workspaceID)
	eligible := make([]model.CRMSignal, 0, limit)
	for index := range signals {
		profile.scoreSignal(&signals[index], time.Now().UTC())
		s.setSignalActivation(ctx, &signals[index], policy, profile)
		if signals[index].ActivationEligible {
			eligible = append(eligible, signals[index])
			if len(eligible) == limit {
				break
			}
		}
	}
	return eligible, nil
}

func (s *CRMSignalService) RouteWorkspaceSignals(ctx context.Context, workspaceID string) (int, error) {
	routed, retryErr := s.retrySignalDeliveries(ctx, workspaceID)
	policy, err := s.signalRepo.GetActiveRoutingPolicy(ctx, workspaceID)
	if err != nil || policy == nil {
		return routed, errors.Join(retryErr, err)
	}
	signals, err := s.ListActivationSignals(ctx, workspaceID, 50)
	if err != nil {
		return routed, errors.Join(retryErr, err)
	}
	var channels []string
	if err := json.Unmarshal(policy.Channels, &channels); err != nil {
		return routed, errors.Join(retryErr, fmt.Errorf("decode routing policy channels: %w", err))
	}
	var routeErrs []error
	if retryErr != nil {
		routeErrs = append(routeErrs, retryErr)
	}
	for _, signal := range signals {
		ownerID := signal.OwnerMemberID
		if !policy.RouteToOwner {
			ownerID = nil
		}
		for _, channel := range channels {
			now := time.Now().UTC()
			delivery := &model.CRMSignalDelivery{
				WorkspaceID: workspaceID, SignalID: signal.ID, PolicyID: policy.ID, PolicyVersion: policy.Version,
				Channel: channel, RecipientMemberID: ownerID, DestinationTeamID: policy.DestinationTeamID,
				Status: model.CRMSignalDeliveryPending,
			}
			if channel == model.CRMSignalDeliveryFeed {
				delivery.Status = model.CRMSignalDeliverySent
				delivery.DeliveredAt = &now
			}
			created, createErr := s.signalRepo.CreateSignalDelivery(ctx, delivery)
			if createErr != nil {
				routeErrs = append(routeErrs, createErr)
				continue
			}
			if !created {
				continue
			}
			if channel == model.CRMSignalDeliveryFeed {
				routed++
				continue
			}
			delivered, deliveryErr := s.emitSignalDelivery(ctx, &signal, delivery)
			if delivered {
				routed++
			}
			if deliveryErr != nil {
				routeErrs = append(routeErrs, deliveryErr)
			}
		}
	}
	return routed, errors.Join(routeErrs...)
}

func (s *CRMSignalService) retrySignalDeliveries(ctx context.Context, workspaceID string) (int, error) {
	now := time.Now().UTC()
	deliveries, err := s.signalRepo.ListRetryableSignalDeliveries(
		ctx, workspaceID, now.Add(-signalDeliveryClaimTimeout), 100,
	)
	if err != nil {
		return 0, err
	}
	routed := 0
	var retryErrs []error
	for index := range deliveries {
		signal, loadErr := s.signalRepo.GetSignal(ctx, workspaceID, deliveries[index].SignalID)
		if loadErr != nil {
			retryErrs = append(retryErrs, loadErr)
			continue
		}
		if signal == nil {
			retryErrs = append(retryErrs, fmt.Errorf("signal %s for queued delivery was not found", deliveries[index].SignalID))
			continue
		}
		delivered, deliveryErr := s.emitSignalDelivery(ctx, signal, &deliveries[index])
		if delivered {
			routed++
		}
		if deliveryErr != nil {
			retryErrs = append(retryErrs, deliveryErr)
		}
	}
	return routed, errors.Join(retryErrs...)
}

func (s *CRMSignalService) emitSignalDelivery(
	ctx context.Context,
	signal *model.CRMSignal,
	delivery *model.CRMSignalDelivery,
) (bool, error) {
	now := time.Now().UTC()
	claimed, err := s.signalRepo.ClaimSignalDelivery(
		ctx, delivery.ID, now.Add(-signalDeliveryClaimTimeout), now,
	)
	if err != nil || !claimed {
		return false, err
	}
	fail := func(cause error) (bool, error) {
		message := cause.Error()
		if len(message) > 1000 {
			message = message[:1000]
		}
		return false, errors.Join(cause, s.signalRepo.MarkSignalDeliveryFailed(ctx, delivery.ID, message))
	}
	if s.notifications == nil {
		return fail(fmt.Errorf("notification emitter is unavailable"))
	}
	recipients, err := s.signalRepo.RoutingRecipientUserIDs(
		ctx, delivery.WorkspaceID, delivery.RecipientMemberID, delivery.DestinationTeamID,
	)
	if err != nil {
		return fail(err)
	}
	event := signalNotificationEvent(*signal, *delivery, recipients)
	if err := s.notifications.Emit(ctx, event); err != nil {
		return fail(err)
	}
	if err := s.signalRepo.MarkSignalDeliverySent(ctx, delivery.ID, time.Now().UTC()); err != nil {
		return false, err
	}
	return true, nil
}

func signalNotificationEvent(
	signal model.CRMSignal,
	delivery model.CRMSignalDelivery,
	recipients []string,
) model.NotificationEventInput {
	event := model.NotificationEventInput{
		WorkspaceID: delivery.WorkspaceID, EventType: "crm.signal_ready",
		EntityType: "crm_signal", EntityID: signal.ID,
		Title: "CRM signal ready for review", Body: signal.Summary, Category: "crm_signal",
		Priority: signalNotificationPriority(signal.Severity),
		TeamID:   signalStringValue(delivery.DestinationTeamID), ExplicitRecipients: recipients,
		SkipFollowers: true, SkipEmailDelivery: delivery.Channel == model.CRMSignalDeliveryNotification,
		Metadata: model.JSONB{
			"rule_key": signalStringValue(signal.RuleKey), "rule_version": signalIntValue(signal.RuleVersion),
			"score": signal.BusinessPriority,
		},
	}
	if delivery.Channel == model.CRMSignalDeliveryDigest {
		event.DelayedEmailChannel = "digest"
	}
	return event
}

func signalNotificationPriority(severity string) string {
	if severity == "high" {
		return "high"
	}
	return "normal"
}

func signalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func signalIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func (s *CRMSignalService) GetSignalBrief(ctx context.Context, workspaceID string, filters model.CRMSignalListFilters) (*model.CRMSignalBrief, error) {
	feed, err := s.ListWorkspaceSignalFeed(ctx, workspaceID, filters, model.PMPagination{Page: 1, PerPage: 1})
	if err != nil {
		return nil, err
	}
	if len(feed.Data) == 0 {
		return &model.CRMSignalBrief{GeneratedAt: time.Now().UTC(), WhatChanged: []string{}, Sources: []model.CRMSignal{}}, nil
	}
	story := feed.Data[0]
	activationReady := false
	if policy, policyErr := s.signalRepo.GetActiveRoutingPolicy(ctx, workspaceID); policyErr == nil && policy != nil {
		profile := s.loadSignalScoringProfile(ctx, workspaceID)
		for index := range story.Signals {
			s.setSignalActivation(ctx, &story.Signals[index], policy, profile)
			activationReady = activationReady || story.Signals[index].ActivationEligible
		}
	}
	return &model.CRMSignalBrief{
		GeneratedAt: time.Now().UTC(), WhatChanged: []string{story.ChangeSummary}, Priority: story.Priority,
		Severity: story.Severity, ScoreVersion: story.ScoreVersion, Sources: story.Signals, ActivationReady: activationReady,
	}, nil
}

func (s *CRMSignalService) GetMeetingSignalBrief(ctx context.Context, workspaceID, meetingID string) (*model.CRMSignalBrief, error) {
	if strings.TrimSpace(meetingID) == "" {
		return nil, fmt.Errorf("meeting_id is required")
	}
	filters, err := s.signalRepo.SignalFiltersForMeeting(ctx, workspaceID, meetingID)
	if err != nil {
		return nil, err
	}
	return s.GetSignalBrief(ctx, workspaceID, filters)
}
