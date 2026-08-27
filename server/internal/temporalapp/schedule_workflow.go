package temporalapp

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var (
	// ErrScheduledRuleNotFound means a cron workflow outlived its rule record.
	ErrScheduledRuleNotFound = errors.New("scheduled automation rule not found")
	// ErrScheduledRuleDisabled means a cron workflow outlived an enabled rule.
	ErrScheduledRuleDisabled = errors.New("scheduled automation rule is disabled")
	// ErrScheduledRuleNotScheduled means a cron workflow outlived a cron trigger.
	ErrScheduledRuleNotScheduled = errors.New("automation rule is no longer scheduled")
)

// ScheduledRuleInput identifies the automation rule for each cron tick.
type ScheduledRuleInput struct {
	RuleID      string `json:"rule_id"`
	WorkspaceID string `json:"workspace_id"`
}

// ScheduledRuleWorkflow is a Temporal cron workflow that executes one
// automation rule on each tick.
func ScheduledRuleWorkflow(ctx workflow.Context, input ScheduledRuleInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	return workflow.ExecuteActivity(ctx, "ScheduledRuleActivities.ExecuteScheduledRule", input).Get(ctx, nil)
}

// WorkflowIDForRuleSchedule returns the Temporal workflow ID for an automation-rule schedule.
func WorkflowIDForRuleSchedule(ruleID string) string {
	return "automation-rule-schedule-" + ruleID
}

// ScheduledRuleExecutor executes one scheduled automation rule tick.
type ScheduledRuleExecutor interface {
	ExecuteScheduledRule(ctx context.Context, workspaceID, ruleID string) error
}

// ScheduledRuleActivities executes automation-rule-backed cron schedules.
type ScheduledRuleActivities struct {
	executor ScheduledRuleExecutor
}

// NewScheduledRuleActivities creates scheduled rule activities.
func NewScheduledRuleActivities(executor ScheduledRuleExecutor) *ScheduledRuleActivities {
	return &ScheduledRuleActivities{executor: executor}
}

// ExecuteScheduledRule executes one scheduled automation rule tick.
func (a *ScheduledRuleActivities) ExecuteScheduledRule(ctx context.Context, input ScheduledRuleInput) error {
	if a == nil || a.executor == nil {
		slog.WarnContext(ctx, "scheduled rule executor not configured, skipping",
			"workspace_id", input.WorkspaceID,
			"rule_id", input.RuleID,
		)
		return nil
	}
	err := a.executor.ExecuteScheduledRule(ctx, input.WorkspaceID, input.RuleID)
	if isPermanentScheduledRuleError(err) {
		return temporal.NewNonRetryableApplicationError(
			err.Error(),
			"scheduled_rule_unavailable",
			err,
		)
	}
	return err
}

func isPermanentScheduledRuleError(err error) bool {
	return errors.Is(err, ErrScheduledRuleNotFound) ||
		errors.Is(err, ErrScheduledRuleDisabled) ||
		errors.Is(err, ErrScheduledRuleNotScheduled)
}
