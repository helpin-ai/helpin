package temporalapp

import (
	"context"
	"log/slog"
	"time"

	"go.temporal.io/sdk/workflow"
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
	return a.executor.ExecuteScheduledRule(ctx, input.WorkspaceID, input.RuleID)
}
