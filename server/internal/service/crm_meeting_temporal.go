package service

import (
	"context"
	"errors"
	"fmt"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"

	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

type temporalMeetingProcessingRunner struct {
	client tclient.Client
}

// NewTemporalMeetingProcessingRunner creates the durable meeting processor launcher.
func NewTemporalMeetingProcessingRunner(client tclient.Client) meetingProcessingRunner {
	return &temporalMeetingProcessingRunner{client: client}
}

func (r *temporalMeetingProcessingRunner) StartMeetingProcessing(ctx context.Context, workspaceID, meetingID string) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("meeting processing is unavailable")
	}
	_, err := r.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:                    "crm-meeting-processing-" + meetingID,
		TaskQueue:             temporalapp.QueueAutomation,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, temporalapp.CRMMeetingProcessingWorkflow, temporalapp.CRMMeetingProcessingInput{
		WorkspaceID: workspaceID,
		MeetingID:   meetingID,
	})
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start meeting processing workflow: %w", err)
	}
	return nil
}
