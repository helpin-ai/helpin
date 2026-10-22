package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	enums "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
)

const coverageRunErrorMaxLen = 300

// coverageRunState is what the Coverage page needs to explain a manual
// reassessment: its status, the attempt in progress, and why it failed.
type coverageRunState struct {
	Status  string
	Attempt int32
	Error   string
}

func coverageReanalysisWorkflowID(workspaceID string) string {
	return "coverage-reanalysis-" + workspaceID
}

func (s *SupportCoverageV2Service) reanalysisState(ctx context.Context, workspaceID string) (coverageRunState, error) {
	if s.temporal == nil {
		return coverageRunState{Status: "unavailable"}, nil
	}
	workflowID := coverageReanalysisWorkflowID(workspaceID)
	description, err := s.temporal.DescribeWorkflowExecution(ctx, workflowID, "")
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return coverageRunState{}, nil
	}
	if err != nil {
		return coverageRunState{}, err
	}
	if description == nil || description.WorkflowExecutionInfo == nil {
		return coverageRunState{Status: "unavailable"}, nil
	}
	info := description.WorkflowExecutionInfo
	switch info.Status {
	case enums.WORKFLOW_EXECUTION_STATUS_RUNNING:
		return pendingCoverageRunState(description.PendingActivities), nil
	case enums.WORKFLOW_EXECUTION_STATUS_FAILED, enums.WORKFLOW_EXECUTION_STATUS_TIMED_OUT,
		enums.WORKFLOW_EXECUTION_STATUS_TERMINATED, enums.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return coverageRunState{Status: "failed", Error: s.coverageRunCloseReason(ctx, workflowID, info)}, nil
	default:
		return coverageRunState{}, nil
	}
}

func pendingCoverageRunState(pending []*workflowpb.PendingActivityInfo) coverageRunState {
	for _, activity := range pending {
		if activity == nil {
			continue
		}
		state := coverageRunState{Status: "queued", Attempt: activity.GetAttempt()}
		if activity.GetState() == enums.PENDING_ACTIVITY_STATE_STARTED {
			state.Status = "running"
		}
		if activity.GetAttempt() > 1 && activity.GetLastFailure() != nil {
			if state.Status == "queued" {
				state.Status = "retrying"
			}
			state.Error = coverageFailureMessage(activity.GetLastFailure())
		}
		return state
	}
	return coverageRunState{Status: "queued"}
}

// coverageRunCloseReason reads the close event of a finished reassessment so
// the page can show why it stopped without anyone opening Temporal.
func (s *SupportCoverageV2Service) coverageRunCloseReason(ctx context.Context, workflowID string, info *workflowpb.WorkflowExecutionInfo) string {
	switch info.Status {
	case enums.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return "The analysis ran out of time."
	case enums.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return "The analysis was cancelled."
	}
	runID := ""
	if info.GetExecution() != nil {
		runID = info.GetExecution().GetRunId()
	}
	events := s.temporal.GetWorkflowHistory(ctx, workflowID, runID, false, enums.HISTORY_EVENT_FILTER_TYPE_CLOSE_EVENT)
	for events != nil && events.HasNext() {
		event, err := events.Next()
		if err != nil {
			slog.WarnContext(ctx, "coverage reanalysis close event unavailable", "workflow_id", workflowID, "error", err)
			return ""
		}
		if failed := event.GetWorkflowExecutionFailedEventAttributes(); failed != nil {
			return coverageFailureMessage(failed.GetFailure())
		}
		if terminated := event.GetWorkflowExecutionTerminatedEventAttributes(); terminated != nil {
			if reason := strings.TrimSpace(terminated.GetReason()); reason != "" {
				return truncateCoverageRunError("The analysis was stopped: " + reason)
			}
			return "The analysis was stopped."
		}
	}
	return ""
}

// coverageFailureMessage returns the innermost failure message, which is the
// application error rather than Temporal's activity wrapper.
func coverageFailureMessage(failure *failurepb.Failure) string {
	message := ""
	for current := failure; current != nil; current = current.GetCause() {
		if info := current.GetTimeoutFailureInfo(); info != nil {
			if info.GetTimeoutType() == enums.TIMEOUT_TYPE_HEARTBEAT {
				return "The analysis worker stopped responding."
			}
			return "The analysis ran out of time."
		}
		if text := strings.TrimSpace(current.GetMessage()); text != "" {
			message = text
		}
	}
	return truncateCoverageRunError(message)
}

func truncateCoverageRunError(message string) string {
	runes := []rune(message)
	if len(runes) <= coverageRunErrorMaxLen {
		return message
	}
	return strings.TrimSpace(string(runes[:coverageRunErrorMaxLen])) + "…"
}

// scheduleStatus reports whether the automatic analysis cron exists. A missing
// cron otherwise looks identical to a quiet workspace.
func (s *SupportCoverageV2Service) scheduleStatus(ctx context.Context) string {
	if s.temporal == nil {
		return ""
	}
	description, err := s.temporal.DescribeWorkflowExecution(ctx, coverageAnalysisWorkflowID, "")
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return "missing"
	}
	if err != nil {
		slog.WarnContext(ctx, "coverage analysis schedule unavailable", "error", err)
		return ""
	}
	if description.GetWorkflowExecutionInfo().GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_RUNNING {
		return "scheduled"
	}
	return "missing"
}
