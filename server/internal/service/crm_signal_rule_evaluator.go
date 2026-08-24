package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	signalRuleLeaseDuration     = 20 * time.Minute
	signalRuleDailyOverlap      = 48 * time.Hour
	signalRuleMicroBatchOverlap = 15 * time.Minute
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
	if config.Cadence == model.CRMSignalRuleCadenceDaily {
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
		volume, err := reader.SmokeCount(ctx, start, end)
		if err != nil {
			return nil, err
		}
		if volume == 0 {
			continue
		}
		candidates, err := reader.EvaluateBehavioralSignalRule(ctx, config, start, end)
		if err != nil {
			return nil, err
		}
		result = append(result, candidates...)
	}
	return result, nil
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
	if config.Cadence == model.CRMSignalRuleCadenceMicroBatch {
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
	signal := &model.CRMBuyerSignal{
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
