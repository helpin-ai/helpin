package temporalapp

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.temporal.io/sdk/temporal"
)

type scheduledRuleExecutorFunc func(context.Context, string, string) error

func (f scheduledRuleExecutorFunc) ExecuteScheduledRule(
	ctx context.Context,
	workspaceID string,
	ruleID string,
) error {
	return f(ctx, workspaceID, ruleID)
}

func TestScheduledRuleActivityDoesNotRetryPermanentRuleErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "missing", err: ErrScheduledRuleNotFound},
		{name: "disabled", err: ErrScheduledRuleDisabled},
		{name: "not scheduled", err: ErrScheduledRuleNotScheduled},
		{name: "wrapped", err: fmt.Errorf("load rule: %w", ErrScheduledRuleNotFound)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			activities := NewScheduledRuleActivities(scheduledRuleExecutorFunc(
				func(context.Context, string, string) error { return tt.err },
			))
			err := activities.ExecuteScheduledRule(context.Background(), ScheduledRuleInput{
				WorkspaceID: "ws-1",
				RuleID:      "rule-1",
			})

			var applicationErr *temporal.ApplicationError
			if !errors.As(err, &applicationErr) {
				t.Fatalf("error = %T %v, want Temporal application error", err, err)
			}
			if !applicationErr.NonRetryable() {
				t.Fatal("permanent scheduled-rule error must be non-retryable")
			}
			if applicationErr.Type() != "scheduled_rule_unavailable" {
				t.Fatalf("application error type = %q", applicationErr.Type())
			}
		})
	}
}

func TestScheduledRuleActivityKeepsTransientErrorsRetryable(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("database temporarily unavailable")
	activities := NewScheduledRuleActivities(scheduledRuleExecutorFunc(
		func(context.Context, string, string) error { return wantErr },
	))
	err := activities.ExecuteScheduledRule(context.Background(), ScheduledRuleInput{
		WorkspaceID: "ws-1",
		RuleID:      "rule-1",
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want transient executor error", err)
	}
	var applicationErr *temporal.ApplicationError
	if errors.As(err, &applicationErr) {
		t.Fatal("transient errors must retain the workflow retry policy")
	}
}
