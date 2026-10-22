package service

import (
	"context"
	"testing"

	commonpb "go.temporal.io/api/common/v1"
	enums "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	tclient "go.temporal.io/sdk/client"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type coverageRunStateClient struct {
	tclient.Client
	executions map[string]*workflowservice.DescribeWorkflowExecutionResponse
	closeEvent *historypb.HistoryEvent
}

func (c *coverageRunStateClient) DescribeWorkflowExecution(ctx context.Context, id, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
	if execution, ok := c.executions[id]; ok {
		return execution, nil
	}
	return nil, &serviceerror.NotFound{}
}

func (c *coverageRunStateClient) GetWorkflowHistory(ctx context.Context, workflowID, runID string, isLongPoll bool, filterType enums.HistoryEventFilterType) tclient.HistoryEventIterator {
	return &coverageHistoryIterator{events: []*historypb.HistoryEvent{c.closeEvent}}
}

type coverageHistoryIterator struct {
	events []*historypb.HistoryEvent
}

func (i *coverageHistoryIterator) HasNext() bool { return len(i.events) > 0 && i.events[0] != nil }

func (i *coverageHistoryIterator) Next() (*historypb.HistoryEvent, error) {
	event := i.events[0]
	i.events = i.events[1:]
	return event, nil
}

func describedCoverageRun(status enums.WorkflowExecutionStatus, pending ...*workflowpb.PendingActivityInfo) *workflowservice.DescribeWorkflowExecutionResponse {
	return &workflowservice.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: status, Execution: &commonpb.WorkflowExecution{RunId: "run-1"}},
		PendingActivities:     pending,
	}
}

func TestCoverageReanalysisStateExplainsFailure(t *testing.T) {
	activityFailure := &failurepb.Failure{
		Message: "activity error",
		Cause:   &failurepb.Failure{Message: "coverage daily analyzer dependencies are not configured"},
	}
	client := &coverageRunStateClient{
		executions: map[string]*workflowservice.DescribeWorkflowExecutionResponse{
			"coverage-reanalysis-ws-1": describedCoverageRun(enums.WORKFLOW_EXECUTION_STATUS_FAILED),
		},
		closeEvent: &historypb.HistoryEvent{Attributes: &historypb.HistoryEvent_WorkflowExecutionFailedEventAttributes{
			WorkflowExecutionFailedEventAttributes: &historypb.WorkflowExecutionFailedEventAttributes{Failure: activityFailure},
		}},
	}
	svc := NewSupportCoverageV2Service(nil).SetTemporalClient(client)

	state, err := svc.reanalysisState(context.Background(), "ws-1")

	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "failed" || state.Error != "coverage daily analyzer dependencies are not configured" {
		t.Fatalf("state = %+v", state)
	}
}

func TestCoverageReanalysisStateShowsAutomaticRetry(t *testing.T) {
	client := &coverageRunStateClient{executions: map[string]*workflowservice.DescribeWorkflowExecutionResponse{
		"coverage-reanalysis-ws-1": describedCoverageRun(enums.WORKFLOW_EXECUTION_STATUS_RUNNING, &workflowpb.PendingActivityInfo{
			State: enums.PENDING_ACTIVITY_STATE_SCHEDULED, Attempt: 2,
			LastFailure: &failurepb.Failure{Message: "heartbeat", FailureInfo: &failurepb.Failure_TimeoutFailureInfo{
				TimeoutFailureInfo: &failurepb.TimeoutFailureInfo{TimeoutType: enums.TIMEOUT_TYPE_HEARTBEAT},
			}},
		}),
	}}
	svc := NewSupportCoverageV2Service(nil).SetTemporalClient(client)

	state, err := svc.reanalysisState(context.Background(), "ws-1")

	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "retrying" || state.Attempt != 2 || state.Error != "The analysis worker stopped responding." {
		t.Fatalf("state = %+v", state)
	}
}

func TestCoverageScheduleStatusReportsMissingCron(t *testing.T) {
	missing := NewSupportCoverageV2Service(nil).SetTemporalClient(&coverageRunStateClient{})
	if status := missing.scheduleStatus(context.Background()); status != "missing" {
		t.Fatalf("schedule status = %q, want missing", status)
	}
	scheduled := NewSupportCoverageV2Service(nil).SetTemporalClient(&coverageRunStateClient{executions: map[string]*workflowservice.DescribeWorkflowExecutionResponse{
		coverageAnalysisWorkflowID: describedCoverageRun(enums.WORKFLOW_EXECUTION_STATUS_RUNNING),
	}})
	if status := scheduled.scheduleStatus(context.Background()); status != "scheduled" {
		t.Fatalf("schedule status = %q, want scheduled", status)
	}
}

func TestCoverageFailuresNeedPersonOnlyAfterRetriesRunOut(t *testing.T) {
	retrying := []model.CoverageAnalysisAttempt{{Status: model.CoverageAttemptRetryable}}
	if coverageFailuresNeedPerson(retrying) {
		t.Fatal("failures that retry automatically should not need a person")
	}
	stuck := append(retrying, model.CoverageAnalysisAttempt{Status: model.CoverageAttemptDeadLetter})
	if !coverageFailuresNeedPerson(stuck) {
		t.Fatal("a failure that used all its retries needs a person")
	}
}
