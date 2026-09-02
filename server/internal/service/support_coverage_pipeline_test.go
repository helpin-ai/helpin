package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCoveragePipelineTransitions(t *testing.T) {
	tests := []struct {
		name       string
		state      CoverageAttemptState
		event      CoverageAttemptEvent
		wantStatus string
		wantBudget int
	}{
		{"claim", CoverageAttemptState{Status: model.CoverageAttemptQueued}, CoverageAttemptEvent{Kind: CoverageEventClaim}, model.CoverageAttemptLeased, 0},
		{"success", CoverageAttemptState{Status: model.CoverageAttemptLeased}, CoverageAttemptEvent{Kind: CoverageEventSuccess}, model.CoverageAttemptSucceeded, 0},
		{"retryable", CoverageAttemptState{Status: model.CoverageAttemptLeased}, CoverageAttemptEvent{Kind: CoverageEventFailure, FailureClass: model.CoverageFailureLLMProvider, MaxRetries: 3}, model.CoverageAttemptRetryable, 1},
		{"terminal", CoverageAttemptState{Status: model.CoverageAttemptLeased, RetryBudgetUsed: 2}, CoverageAttemptEvent{Kind: CoverageEventFailure, FailureClass: model.CoverageFailureLLMProvider, MaxRetries: 3}, model.CoverageAttemptDeadLetter, 3},
		{"configuration", CoverageAttemptState{Status: model.CoverageAttemptLeased, RetryBudgetUsed: 1}, CoverageAttemptEvent{Kind: CoverageEventFailure, FailureClass: model.CoverageFailureConfiguration, MaxRetries: 3}, model.CoverageAttemptPausedConfiguration, 1},
		{"lease lost", CoverageAttemptState{Status: model.CoverageAttemptLeased}, CoverageAttemptEvent{Kind: CoverageEventLeaseLost, MaxRetries: 3}, model.CoverageAttemptRetryable, 1},
		{"replay", CoverageAttemptState{Status: model.CoverageAttemptDeadLetter, RetryBudgetUsed: 3}, CoverageAttemptEvent{Kind: CoverageEventReplay}, model.CoverageAttemptQueued, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := TransitionCoverageAttempt(test.state, test.event)
			if err != nil {
				t.Fatalf("TransitionCoverageAttempt: %v", err)
			}
			if got.Status != test.wantStatus || got.RetryBudgetUsed != test.wantBudget {
				t.Fatalf("state = %#v, want status=%s budget=%d", got, test.wantStatus, test.wantBudget)
			}
		})
	}
}

func TestCoveragePipelineRejectsInvalidTransition(t *testing.T) {
	if _, err := TransitionCoverageAttempt(CoverageAttemptState{Status: model.CoverageAttemptSucceeded}, CoverageAttemptEvent{Kind: CoverageEventClaim}); err == nil {
		t.Fatal("expected succeeded attempt claim to fail")
	}
}
