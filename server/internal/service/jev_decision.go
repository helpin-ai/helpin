package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/decision"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Independently controlled Jev features; CRM signal classification is excluded.
const (
	JevMeetingRouting         = "meeting_routing"
	JevCoverageClassification = "coverage_classification"
	JevCoverageTopicMatching  = "coverage_topic_matching"
	JevAutomationCondition    = "automation_condition"
	JevAnswerEvidence         = "answer_evidence"
)

// JevDecisionStore provides admission and content-free settlement for decisions.
type JevDecisionStore interface {
	Reserve(context.Context, model.JevDecisionAttempt, int) (*repository.JevDecisionAdmission, error)
	Finish(context.Context, string, string, string, model.JSONB) error
}

// JevDecisionService applies shared bounds, metering and audit to domain decisions.
// Domain callers own authorization, evidence preparation and guarded mutations.
type JevDecisionService struct {
	provider   decision.Provider
	store      JevDecisionStore
	usage      AIExecutionUsageStore
	policies   map[string]decision.Policy
	workspaces map[string]bool
}

// NewJevDecisionService creates shared infrastructure without calling the provider.
func NewJevDecisionService(provider decision.Provider, store JevDecisionStore, usage AIExecutionUsageStore, policies map[string]decision.Policy, workspaceIDs []string) (*JevDecisionService, error) {
	if store == nil || usage == nil {
		return nil, errors.New("decision audit and usage stores required")
	}
	copied := make(map[string]decision.Policy, len(policies))
	for feature, policy := range policies {
		switch feature {
		case JevMeetingRouting, JevCoverageClassification, JevCoverageTopicMatching, JevAutomationCondition, JevAnswerEvidence:
		default:
			return nil, errors.New("unsupported decision feature")
		}
		if err := policy.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", feature, err)
		}
		copied[feature] = policy
	}
	workspaces := map[string]bool{}
	for _, id := range workspaceIDs {
		if id = strings.TrimSpace(id); id != "" {
			workspaces[id] = true
		}
	}
	return &JevDecisionService{provider: provider, store: store, usage: usage, policies: copied, workspaces: workspaces}, nil
}

// JevDecisionRequest contains already-authorized evidence and closed choices.
type JevDecisionRequest struct {
	WorkspaceID string
	Feature     string
	SourceID    string
	Version     string
	State       string
	Questions   map[string]decision.Question
}

// JevDecisionResult exposes decisions with their rollout status and policy.
type JevDecisionResult struct {
	ID        string
	Status    string
	Mode      string
	Threshold float64
	Result    *decision.Result
}

// Selected returns an accepted primary choice only. Callers still handle explicit
// uncertain choices and preserve their own deterministic eligibility checks.
func (r *JevDecisionResult) Selected(key string) (string, float64, bool) {
	if r == nil || r.Status != "ready" || r.Mode != "primary" || r.Result == nil {
		return "", 0, false
	}
	answer, ok := r.Result.Answers[key]
	probability := answer.Probabilities[answer.Choice]
	return answer.Choice, probability, ok && probability >= r.Threshold
}

// Enabled reports whether a feature can evaluate this workspace (including shadow).
func (s *JevDecisionService) Enabled(workspaceID, feature string) bool {
	if s == nil || s.provider == nil {
		return false
	}
	policy, exists := s.policies[feature]
	return exists && policy.Mode != "off" && (len(s.workspaces) == 0 || s.workspaces[workspaceID])
}

// SemanticConditionAvailability reports whether a workspace can author actionable
// conditions. Shadow can evaluate evidence but cannot allow a Flow action.
func (s *JevDecisionService) SemanticConditionAvailability(workspaceID string) model.SemanticConditionAvailability {
	if s == nil || s.provider == nil {
		return model.SemanticConditionAvailability{Reason: "not_configured"}
	}
	if len(s.workspaces) > 0 && !s.workspaces[workspaceID] {
		return model.SemanticConditionAvailability{Reason: "workspace_disabled"}
	}
	switch s.policies[JevAutomationCondition].Mode {
	case "primary":
		return model.SemanticConditionAvailability{Available: true}
	case "shadow":
		return model.SemanticConditionAvailability{Reason: "shadow"}
	default:
		return model.SemanticConditionAvailability{Reason: "disabled"}
	}
}

// Decide evaluates bounded evidence, deduplicates attempts and accounts for usage.
// Rejected/failed/limited decisions never return an actionable selection.
func (s *JevDecisionService) Decide(ctx context.Context, req JevDecisionRequest) (*JevDecisionResult, error) {
	output := &JevDecisionResult{Status: "disabled"}
	if !s.Enabled(req.WorkspaceID, req.Feature) {
		return output, nil
	}
	policy := s.policies[req.Feature]
	output.Mode, output.Threshold = policy.Mode, policy.Threshold
	if req.WorkspaceID == "" || req.SourceID == "" || len(req.SourceID) > 255 || req.Version == "" {
		return nil, errors.New("invalid decision identity")
	}
	if err := validateJevDecisionInput(req); err != nil {
		output.Status = "input_limit"
		return output, nil
	}
	identity, err := json.Marshal(struct {
		Request JevDecisionRequest
		Policy  decision.Policy
		Model   string
	}{Request: req, Policy: policy, Model: decision.Model})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(identity)
	admission, err := s.store.Reserve(ctx, model.JevDecisionAttempt{WorkspaceID: req.WorkspaceID, Feature: req.Feature, SourceID: req.SourceID, InputHash: hex.EncodeToString(hash[:]), Mode: policy.Mode}, policy.DailyLimit)
	if err != nil {
		return nil, err
	}
	if admission.Limited {
		output.Status = "daily_limit"
		return output, nil
	}
	if admission.Attempt == nil {
		return nil, errors.New("missing decision admission")
	}
	output.ID, output.Status = admission.Attempt.ID, admission.Attempt.Status
	if !admission.CallProvider {
		if output.Status != "ready" {
			return output, nil
		}
		data, err := json.Marshal(admission.Attempt.Outcome["result"])
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &output.Result); err != nil {
			return nil, err
		}
		if err := decision.ValidateResult(output.Result, req.Questions); err != nil {
			return nil, err
		}
		return output, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	result, callErr := s.provider.DecideMany(callCtx, req.State, req.Questions)
	cancel()
	if callErr == nil {
		callErr = decision.ValidateResult(result, req.Questions)
	}
	settle, done := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer done()
	if result != nil && result.InputTokens >= 0 && result.OutputTokens >= 0 {
		entry := model.AIExecutionUsage{WorkspaceID: req.WorkspaceID, IdempotencyKey: "jev-product:" + output.ID, FeatureKey: "jev_" + req.Feature, Provider: "typesafe", Model: result.Model, InputTokens: result.InputTokens, OutputTokens: result.OutputTokens, MeasurementStatus: "provider_reported", PaidTools: model.JSONBlob(`[]`)}
		if err := s.usage.RecordExecutionUsage(settle, entry, nil); err != nil {
			callErr = errors.Join(callErr, fmt.Errorf("record decision usage: %w", err))
		}
	}
	output.Status = "failed"
	outcome := model.JSONB{"version": req.Version, "mode": policy.Mode, "threshold": policy.Threshold, "cost_policy": "operator_funded_pilot", "confidence_kind": "provider_probability_not_locally_calibrated"}
	if callErr == nil {
		output.Status = "ready"
		outcome["result"] = result
	}
	if err := s.store.Finish(settle, req.WorkspaceID, output.ID, output.Status, outcome); err != nil {
		return nil, errors.Join(callErr, err)
	}
	if callErr != nil {
		return nil, callErr
	}
	output.Result = result
	return output, nil
}

func validateJevDecisionInput(req JevDecisionRequest) error {
	if strings.TrimSpace(req.State) == "" || len(req.State) > 16000 || len(req.Questions) < 1 || len(req.Questions) > 64 {
		return errors.New("decision input bounds exceeded")
	}
	for name, q := range req.Questions {
		if name == "" || strings.TrimSpace(q.Instructions) == "" || len(q.Choices) < 2 || len(q.Choices) > 255 {
			return errors.New("invalid decision question")
		}
		for id, description := range q.Choices {
			if id == "" || strings.TrimSpace(description) == "" {
				return errors.New("invalid decision choice")
			}
		}
	}
	encoded, err := json.Marshal(req.Questions)
	if err != nil {
		return err
	}
	if len(encoded) > 64000 {
		return errors.New("decision questions exceed bounds")
	}
	return nil
}
