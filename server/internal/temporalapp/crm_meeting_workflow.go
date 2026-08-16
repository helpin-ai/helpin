package temporalapp

import (
	"context"
	"errors"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMMeetingProcessingInput identifies one Helpin-owned meeting artifact.
type CRMMeetingProcessingInput struct {
	WorkspaceID string `json:"workspace_id"`
	MeetingID   string `json:"meeting_id"`
}

// CRMMeetingProcessingWorkflow runs the fixed meeting intelligence pipeline.
func CRMMeetingProcessingWorkflow(ctx workflow.Context, input CRMMeetingProcessingInput) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 45 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    15 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    5,
		},
	})
	return workflow.ExecuteActivity(ctx, "CRMMeetingActivities.ProcessMeetingActivity", input).Get(ctx, nil)
}

type meetingProcessor interface {
	Process(ctx context.Context, workspaceID, meetingID string) error
}

// CRMMeetingActivities bridges Temporal to the product-owned processor.
type CRMMeetingActivities struct {
	processor       meetingProcessor
	captureLauncher scheduledMeetingCaptureLauncher
}

// NewCRMMeetingActivities creates meeting processing activities.
func NewCRMMeetingActivities(processor meetingProcessor) *CRMMeetingActivities {
	return &CRMMeetingActivities{processor: processor}
}

// ProcessMeetingActivity creates canonical transcript and intelligence artifacts.
func (a *CRMMeetingActivities) ProcessMeetingActivity(ctx context.Context, input CRMMeetingProcessingInput) error {
	err := a.processor.Process(ctx, input.WorkspaceID, input.MeetingID)
	if errors.Is(err, model.ErrAIUsageExhausted) || errors.Is(err, model.ErrExtraAIUsageUnavailable) ||
		errors.Is(err, model.ErrExtraAIUsageDisabled) || errors.Is(err, model.ErrBillingWorkspaceLocked) {
		return temporal.NewNonRetryableApplicationError(err.Error(), "meeting_ai_usage_blocked", err)
	}
	return err
}
