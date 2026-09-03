package aipolicy

import (
	"errors"
	"testing"
)

func TestResolveExecutionRequiresSpecificCoverageAction(t *testing.T) {
	_, err := ResolveExecution(DefaultRegistry(), ExecutionContext{
		FeatureKey: "coverage_gap_analysis", WorkspaceID: "ws-1", IdempotencyKey: "item-1",
	}, Route{Provider: "openai", Model: "gpt-5.5"})
	if !errors.Is(err, ErrActionRequired) {
		t.Fatalf("ResolveExecution() error = %v, want ErrActionRequired", err)
	}
}

func TestResolveExecutionDerivesNonCoverageFeatureAction(t *testing.T) {
	action, err := ResolveExecution(DefaultRegistry(), ExecutionContext{
		FeatureKey: "crm_summary", WorkspaceID: "ws-1", IdempotencyKey: "summary-1",
	}, Route{Provider: "anthropic", Model: "claude-sonnet-4-6"})
	if err != nil {
		t.Fatalf("ResolveExecution() error = %v", err)
	}
	if action.Key != "feature.crm_summary.v1" {
		t.Fatalf("action key = %q", action.Key)
	}
}

func TestResolveExecutionRejectsMissingWorkspaceForChargeableAction(t *testing.T) {
	_, err := ResolveExecution(DefaultRegistry(), ExecutionContext{
		ActionKey: ActionSupportCoverageAnalyze, IdempotencyKey: "item-1",
	}, Route{Provider: "openai", Model: "gpt-5.5"})
	if !errors.Is(err, ErrInvalidExecutionContext) {
		t.Fatalf("ResolveExecution() error = %v, want ErrInvalidExecutionContext", err)
	}
}
