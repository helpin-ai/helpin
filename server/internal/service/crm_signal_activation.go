package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

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
		return fmt.Errorf("buyer signal not found")
	}
	return s.signalRepo.RecordSignalFeedback(ctx, signal, memberID, action, reasonPtr, time.Now().UTC())
}

func (s *CRMSignalService) SignalPrecisionReport(ctx context.Context, workspaceID string) ([]model.CRMSignalPrecisionRow, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.signalRepo.ListSignalPrecision(ctx, workspaceID)
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

func (s *CRMSignalService) setSignalActivation(ctx context.Context, signal *model.CRMBuyerSignal, policy *model.CRMSignalRoutingPolicy, profile signalScoringProfile) {
	blockers := []string{}
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

func (s *CRMSignalService) ListActivationSignals(ctx context.Context, workspaceID string, limit int) ([]model.CRMBuyerSignal, error) {
	if limit < 1 || limit > 50 {
		limit = 20
	}
	policy, err := s.signalRepo.GetActiveRoutingPolicy(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return []model.CRMBuyerSignal{}, nil
	}
	filters := model.CRMBuyerSignalListFilters{}
	signals, err := s.signalRepo.ListWorkspaceSignalCandidates(ctx, workspaceID, filters, time.Now().UTC(), 500)
	if err != nil {
		return nil, err
	}
	profile := s.loadSignalScoringProfile(ctx, workspaceID)
	eligible := make([]model.CRMBuyerSignal, 0, limit)
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
	policy, err := s.signalRepo.GetActiveRoutingPolicy(ctx, workspaceID)
	if err != nil || policy == nil {
		return 0, err
	}
	signals, err := s.ListActivationSignals(ctx, workspaceID, 50)
	if err != nil {
		return 0, err
	}
	var channels []string
	if err := json.Unmarshal(policy.Channels, &channels); err != nil {
		return 0, fmt.Errorf("decode routing policy channels: %w", err)
	}
	routed := 0
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
				Status: "routed", DeliveredAt: &now,
			}
			created, createErr := s.signalRepo.CreateSignalDelivery(ctx, delivery)
			if createErr != nil {
				return routed, createErr
			}
			if !created {
				continue
			}
			routed++
			if channel == model.CRMSignalDeliveryFeed || s.notifications == nil {
				continue
			}
			recipients, recipientErr := s.signalRepo.RoutingRecipientUserIDs(ctx, workspaceID, ownerID, policy.DestinationTeamID)
			if recipientErr != nil {
				return routed, recipientErr
			}
			event := model.NotificationEventInput{
				WorkspaceID: workspaceID, EventType: "crm.signal_ready", EntityType: "crm_signal", EntityID: signal.ID,
				Title: "Buyer signal ready for review", Body: signal.Summary, Category: "crm_signal",
				Priority: signalNotificationPriority(signal.Severity), TeamID: signalStringValue(policy.DestinationTeamID),
				ExplicitRecipients: recipients, SkipFollowers: true, SkipEmailDelivery: channel == model.CRMSignalDeliveryNotification,
				Metadata: model.JSONB{"rule_key": signalStringValue(signal.RuleKey), "rule_version": signalIntValue(signal.RuleVersion), "score": signal.BusinessPriority},
			}
			if channel == model.CRMSignalDeliveryDigest {
				event.DelayedEmailChannel = "digest"
			}
			if err := s.notifications.Emit(ctx, event); err != nil {
				return routed, err
			}
		}
	}
	return routed, nil
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

func (s *CRMSignalService) GetSignalBrief(ctx context.Context, workspaceID string, filters model.CRMBuyerSignalListFilters) (*model.CRMSignalBrief, error) {
	feed, err := s.ListWorkspaceSignalFeed(ctx, workspaceID, filters, model.PMPagination{Page: 1, PerPage: 1})
	if err != nil {
		return nil, err
	}
	if len(feed.Data) == 0 {
		return &model.CRMSignalBrief{GeneratedAt: time.Now().UTC(), WhatChanged: []string{}, Sources: []model.CRMBuyerSignal{}}, nil
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
