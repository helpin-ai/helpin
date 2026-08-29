package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/eventcatalog"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	signalRuleLeaseDuration     = 20 * time.Minute
	signalRuleDailyOverlap      = 48 * time.Hour
	signalRuleMicroBatchOverlap = 15 * time.Minute
	signalMotionStateRetention  = 400 * 24 * time.Hour
	usageBaselineRetention      = 90 * 24 * time.Hour
)

// CRMSignalRuleEvaluator turns bounded first-party evidence into durable signals.
type CRMSignalRuleEvaluator struct {
	signals  *repository.CRMSignalRepository
	projects *repository.EventProjectRepository
	eventsDB *sql.DB
	owner    string
	now      func() time.Time
}

// NewCRMSignalRuleEvaluator creates a global, lease-coordinated evaluator.
func NewCRMSignalRuleEvaluator(
	signals *repository.CRMSignalRepository,
	projects *repository.EventProjectRepository,
	eventsDB *sql.DB,
	owner string,
) *CRMSignalRuleEvaluator {
	if strings.TrimSpace(owner) == "" {
		owner = uuid.NewString()
	}
	return &CRMSignalRuleEvaluator{signals: signals, projects: projects, eventsDB: eventsDB, owner: owner, now: time.Now}
}

// Run starts daily and micro-batch evaluators until the context is cancelled.
func (e *CRMSignalRuleEvaluator) Run(ctx context.Context) {
	daily := time.NewTicker(24 * time.Hour)
	micro := time.NewTicker(10 * time.Minute)
	defer daily.Stop()
	defer micro.Stop()
	e.runAndLog(ctx, model.CRMSignalRuleCadenceDaily)
	if e.eventsDB != nil {
		e.runAndLog(ctx, model.CRMSignalRuleCadenceMicroBatch)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-daily.C:
			e.runAndLog(ctx, model.CRMSignalRuleCadenceDaily)
		case <-micro.C:
			if e.eventsDB != nil {
				e.runAndLog(ctx, model.CRMSignalRuleCadenceMicroBatch)
			}
		}
	}
}

func (e *CRMSignalRuleEvaluator) runAndLog(ctx context.Context, cadence string) {
	result, err := e.RunSweep(ctx, cadence)
	if err != nil {
		slog.ErrorContext(ctx, "CRM signal rule sweep failed", "cadence", cadence, "error", err)
		return
	}
	if result != nil {
		slog.InfoContext(ctx, "CRM signal rule sweep complete", "cadence", cadence,
			"rules", result.RulesEvaluated, "candidates", result.Candidates,
			"inserted", result.Inserted, "failed_rules", result.FailedRules)
	}
}

// RunSweep executes one cadence using a global watermark and overlap.
func (e *CRMSignalRuleEvaluator) RunSweep(ctx context.Context, cadence string) (*model.CRMSignalRuleSweepResult, error) {
	if e == nil || e.signals == nil {
		return nil, fmt.Errorf("signal rule evaluator is not configured")
	}
	now := e.now().UTC()
	windowEnd, initial, overlap := ruleSweepWindow(cadence, now)
	watermark, acquired, err := e.signals.TryAcquireSignalEvaluatorLease(ctx, cadence, e.owner, now, signalRuleLeaseDuration, initial)
	if err != nil || !acquired {
		return nil, err
	}
	windowStart := watermark.Add(-overlap)
	if windowStart.Before(initial) {
		windowStart = initial
	}
	result := &model.CRMSignalRuleSweepResult{Cadence: cadence}
	configs, err := e.signals.ListActiveSignalRuleConfigs(ctx, cadence)
	if err != nil {
		_ = e.signals.AbandonSignalEvaluatorLease(ctx, cadence, e.owner)
		return nil, err
	}
	for _, config := range configs {
		ruleStartedAt := e.now().UTC()
		candidates, evaluateErr := e.evaluateRule(ctx, config, windowStart, windowEnd)
		inserted := 0
		if evaluateErr == nil {
			for index := range candidates {
				created, persistErr := e.persistCandidate(ctx, config, &candidates[index], windowStart, windowEnd)
				if persistErr != nil {
					evaluateErr = persistErr
					break
				}
				if created {
					inserted++
				}
			}
		}
		e.recordRuleRun(ctx, config, windowStart, windowEnd, ruleStartedAt, len(candidates), inserted, evaluateErr)
		result.RulesEvaluated++
		result.Candidates += len(candidates)
		result.Inserted += inserted
		if evaluateErr != nil {
			result.FailedRules++
			slog.WarnContext(ctx, "CRM signal rule failed", "rule_key", config.RuleKey, "rule_version", config.Version, "error", evaluateErr)
		}
	}
	if cadence == model.CRMSignalRuleCadenceDaily {
		motionStates, pruneErr := e.signals.PruneSignalMotionStates(ctx, now.Add(-signalMotionStateRetention))
		if pruneErr != nil {
			slog.WarnContext(ctx, "CRM signal motion-state retention failed", "error", pruneErr)
		} else if motionStates > 0 {
			slog.InfoContext(ctx, "pruned CRM signal motion-state history", "rows", motionStates)
		}
		baselines, pruneErr := e.signals.PruneUsageWeekdayBaselines(ctx, now.Add(-usageBaselineRetention))
		if pruneErr != nil {
			slog.WarnContext(ctx, "CRM usage-baseline retention failed", "error", pruneErr)
		} else if baselines > 0 {
			slog.InfoContext(ctx, "pruned CRM usage-baseline history", "rows", baselines)
		}
	}
	if result.FailedRules > 0 {
		_ = e.signals.AbandonSignalEvaluatorLease(ctx, cadence, e.owner)
		return result, fmt.Errorf("%d signal rules failed", result.FailedRules)
	}
	if err := e.signals.ReleaseSignalEvaluatorLease(ctx, cadence, e.owner, windowEnd); err != nil {
		return result, err
	}
	return result, nil
}

func ruleSweepWindow(cadence string, now time.Time) (time.Time, time.Time, time.Duration) {
	switch cadence {
	case model.CRMSignalRuleCadenceDaily:
		end := now.Truncate(24 * time.Hour)
		return end, end.Add(-72 * time.Hour), signalRuleDailyOverlap
	default:
		end := now.Truncate(10 * time.Minute)
		return end, end.Add(-time.Hour), signalRuleMicroBatchOverlap
	}
}

func (e *CRMSignalRuleEvaluator) evaluateRule(ctx context.Context, config model.CRMSignalRuleConfig, start, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	// External evidence is pushed through the normalized ingestion contract;
	// the config remains active for scoring/version policy, not polling.
	if config.RuleKey == model.CRMSignalRuleExternalEvidence {
		return nil, nil
	}
	if config.Cadence == model.CRMSignalRuleCadenceDaily && !eventBackedCustomerRule(config.RuleKey) {
		return e.signals.EvaluatePostgresSignalRule(ctx, config, start, end)
	}
	if e.eventsDB == nil || e.projects == nil {
		return nil, fmt.Errorf("ClickHouse behavioral reader is not configured")
	}
	workspaceIDs, err := e.projects.ListActiveEventWorkspaceIDs(ctx, 500)
	if err != nil {
		return nil, err
	}
	result := make([]model.CRMSignalRuleCandidate, 0)
	for _, workspaceID := range workspaceIDs {
		reader, err := repository.NewClickHouseEventRepository(e.eventsDB, e.projects, workspaceID)
		if err != nil {
			return nil, err
		}
		timezone, err := e.signals.GetWorkspaceTimezone(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		reader.SetTimezone(timezone)
		if !eventBackedCustomerRule(config.RuleKey) {
			volume, err := reader.SmokeCount(ctx, start, end)
			if err != nil {
				return nil, err
			}
			if volume == 0 {
				continue
			}
		}
		var candidates []model.CRMSignalRuleCandidate
		if eventBackedCustomerRule(config.RuleKey) {
			candidates, err = e.evaluatePersistedBaselineRule(ctx, reader, config, workspaceID, timezone, end)
		} else {
			candidates, err = reader.EvaluateBehavioralSignalRule(ctx, config, start, end)
		}
		if err != nil {
			return nil, err
		}
		if config.RuleKey == model.CRMSignalRuleUsageDecline && len(candidates) > 0 {
			eligible, _ := candidates[0].Metadata["eligible_accounts"].(int)
			if eligible == 0 {
				if value, ok := candidates[0].Metadata["eligible_accounts"].(float64); ok {
					eligible = int(value)
				}
			}
			if ShouldSuppressUsageDeclineBatch(eligible, len(candidates)) {
				if err := e.signals.RecordSignalBatchSuppression(ctx, workspaceID, config.RuleKey, config.Version,
					eligible, len(candidates), start, end, "workspace_wide_usage_anomaly"); err != nil {
					return nil, err
				}
				continue
			}
		}
		result = append(result, candidates...)
	}
	return result, nil
}

func (e *CRMSignalRuleEvaluator) evaluatePersistedBaselineRule(
	ctx context.Context,
	reader *repository.ClickHouseEventRepository,
	config model.CRMSignalRuleConfig,
	workspaceID, timezone string,
	end time.Time,
) ([]model.CRMSignalRuleCandidate, error) {
	metricKey := "feature_used"
	spike := false
	if config.RuleKey == model.CRMSignalRuleWorkflowFailureSpike {
		metricKey, spike = "workflow_failed", true
	}
	baselines, err := e.signals.ListLatestUsageWeekdayBaselines(ctx, workspaceID, metricKey)
	if err != nil {
		return nil, err
	}
	type accountBaseline struct {
		externalID     string
		expected       map[int]float64
		identityMethod string
		completeWeeks  int
	}
	accounts := map[string]*accountBaseline{}
	for _, row := range baselines {
		account := accounts[row.CompanyID]
		if account == nil {
			account = &accountBaseline{externalID: row.CompanyExternalID, expected: map[int]float64{}, identityMethod: row.IdentityMethod, completeWeeks: row.CompleteWeeks}
			accounts[row.CompanyID] = account
		}
		account.expected[row.Weekday] = row.MedianValue
		if row.CompleteWeeks < account.completeWeeks {
			account.completeWeeks = row.CompleteWeeks
		}
		if account.identityMethod != row.IdentityMethod {
			account.identityMethod = "mixed_verified"
		}
	}
	recent, err := reader.ListRecentUsageDays(ctx, metricKey, end, timezone)
	if err != nil {
		return nil, err
	}
	type dailyValue struct {
		value          float64
		identityMethod string
	}
	recentByCompany := map[string]map[string]dailyValue{}
	for _, row := range recent {
		if recentByCompany[row.CompanyExternalID] == nil {
			recentByCompany[row.CompanyExternalID] = map[string]dailyValue{}
		}
		recentByCompany[row.CompanyExternalID][row.Day.Format("2006-01-02")] = dailyValue{value: row.Value, identityMethod: row.IdentityMethod}
	}
	localEndUTC, err := workspaceLocalMidnightUTC(end, timezone)
	if err != nil {
		return nil, fmt.Errorf("load workspace timezone: %w", err)
	}
	location, _ := time.LoadLocation(timezone)
	localEnd := localEndUTC.In(location)
	eligible := 0
	rows := make([]repository.BehavioralRuleEvidence, 0)
	for _, account := range accounts {
		if account.externalID == "" || len(account.expected) != 7 || account.completeWeeks < 8 {
			continue
		}
		if !spike {
			valid := true
			for _, expected := range account.expected {
				if expected <= 0 {
					valid = false
					break
				}
			}
			if !valid {
				continue
			}
		}
		eligible++
		trippedDays, recentTotal, expectedTotal := 0, 0.0, 0.0
		identityMethods := map[string]struct{}{}
		for offset := -7; offset < 0; offset++ {
			day := localEnd.AddDate(0, 0, offset)
			actual := recentByCompany[account.externalID][day.Format("2006-01-02")]
			expected := account.expected[int(day.Weekday())]
			recentTotal += actual.value
			expectedTotal += expected
			if actual.identityMethod != "" {
				identityMethods[actual.identityMethod] = struct{}{}
			}
			if (!spike && actual.value < expected*0.60) || (spike && actual.value >= 3 && actual.value > expected*2.0) {
				trippedDays++
			}
		}
		if trippedDays < 5 {
			continue
		}
		identityMethod := account.identityMethod
		if len(identityMethods) == 1 {
			for value := range identityMethods {
				identityMethod = value
			}
		} else if len(identityMethods) > 1 {
			identityMethod = "mixed_verified"
		}
		rows = append(rows, repository.BehavioralRuleEvidence{
			CompanyExternalID: account.externalID, IdentityMethod: identityMethod,
			IdentityTrust: model.IdentityTrustVerified, ObservedAt: localEnd.UTC(),
			EventCount: int(recentTotal), SessionCount: trippedDays, EventName: metricKey,
			Evidence: fmt.Sprintf("%d of 7 days outside weekday baseline; actual %.0f, expected %.1f", trippedDays, recentTotal, expectedTotal),
		})
	}
	for index := range rows {
		rows[index].EligibleAccounts = eligible
	}
	return repository.BuildBehavioralCandidates(config, workspaceID, rows), nil
}

func (e *CRMSignalRuleEvaluator) persistCandidate(
	ctx context.Context,
	config model.CRMSignalRuleConfig,
	candidate *model.CRMSignalRuleCandidate,
	windowStart, windowEnd time.Time,
) (bool, error) {
	if candidate == nil {
		return false, nil
	}
	if !commercialCandidateOriginAllowed(candidate) {
		return false, nil
	}
	if config.Cadence == model.CRMSignalRuleCadenceMicroBatch || strings.TrimSpace(candidate.CompanyExternalID) != "" {
		contactID, companyID, _, _, err := e.signals.ResolveBehavioralIdentity(ctx, candidate.WorkspaceID,
			candidate.AnonymousID, candidate.ExternalUserID, candidate.CompanyExternalID)
		if err != nil {
			return false, err
		}
		candidate.ContactID, candidate.CompanyID = contactID, companyID
		dealID, err := e.signals.ResolveOpenDealForIdentity(ctx, candidate.WorkspaceID, contactID, companyID)
		if err != nil {
			return false, err
		}
		candidate.DealID = dealID
		if candidate.RuleKey == model.CRMSignalRuleProcurementPageActivity && dealID == nil {
			return false, nil
		}
		if contactID == nil && companyID == nil && dealID == nil {
			return false, nil
		}
	}
	if candidate.Metadata == nil {
		candidate.Metadata = model.JSONB{}
	}
	activationEligible := config.ActivationEligible && !config.ShadowMode && candidate.EvidenceIdentityTrust == model.IdentityTrustVerified
	candidate.Metadata["shadow_mode"] = config.ShadowMode
	candidate.Metadata["activation_eligible"] = activationEligible
	candidate.Metadata["rule_version"] = config.Version
	ruleKey, version := config.RuleKey, config.Version
	evidence := strings.TrimSpace(candidate.EvidenceExcerpt)
	signal := &model.CRMSignal{
		WorkspaceID: candidate.WorkspaceID, ContactID: candidate.ContactID, DealID: candidate.DealID, CompanyID: candidate.CompanyID,
		SignalType: candidate.SignalType, SourceType: candidate.SourceType, SourceID: candidate.SourceID,
		Summary: candidate.Summary, EvidenceExcerpt: &evidence, Metadata: candidate.Metadata,
		Confidence: 1, DetectedAt: candidate.ObservedAt, DetectorKind: model.CRMSignalDetectorRuleDerived,
		SignalDomain: candidate.SignalDomain, Polarity: candidate.Polarity, RuleKey: &ruleKey, RuleVersion: &version,
		WindowStartedAt: &windowStart, WindowEndedAt: &windowEnd,
		EvidenceIdentityMethod: candidate.EvidenceIdentityMethod, EvidenceIdentityTrust: candidate.EvidenceIdentityTrust,
		EvidenceFingerprint: candidate.EvidenceFingerprint,
	}
	return e.signals.CreateRuleSignalIfAbsent(ctx, signal)
}

func eventBackedCustomerRule(ruleKey string) bool {
	return ruleKey == model.CRMSignalRuleUsageDecline || ruleKey == model.CRMSignalRuleWorkflowFailureSpike
}

func commercialCandidateOriginAllowed(candidate *model.CRMSignalRuleCandidate) bool {
	if candidate == nil {
		return false
	}
	eventName, ok := candidate.Metadata["event_type"].(string)
	requiresServerEvent := candidate.RuleKey == model.CRMSignalRulePaymentFailed ||
		candidate.RuleKey == model.CRMSignalRuleDowngradeRequested ||
		candidate.RuleKey == model.CRMSignalRuleWorkflowFailureSpike
	if requiresServerEvent && (!ok || !eventcatalog.IsServerOnlyEvent(eventName)) {
		return false
	}
	return !ok || !eventcatalog.IsServerOnlyEvent(eventName) ||
		candidate.EvidenceIdentityMethod == model.IdentityMethodServerEvent
}

func (e *CRMSignalRuleEvaluator) recordRuleRun(ctx context.Context, config model.CRMSignalRuleConfig, start, end, started time.Time, candidates, inserted int, ruleErr error) {
	run := &model.CRMSignalEvaluationRun{Cadence: config.Cadence, RuleKey: config.RuleKey, RuleVersion: config.Version,
		WindowStartedAt: start, WindowEndedAt: end, Status: "running", StartedAt: started}
	if err := e.signals.CreateSignalEvaluationRun(ctx, run); err != nil {
		slog.WarnContext(ctx, "record CRM signal rule run start failed", "rule_key", config.RuleKey, "error", err)
		return
	}
	status := "completed"
	var message *string
	if ruleErr != nil {
		status = "failed"
		text := ruleErr.Error()
		message = &text
	}
	if err := e.signals.FinishSignalEvaluationRun(ctx, run.ID, status, candidates, inserted, message, e.now().UTC()); err != nil {
		slog.WarnContext(ctx, "record CRM signal rule run completion failed", "rule_key", config.RuleKey, "error", err)
	}
}
