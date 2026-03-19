package temporalapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/worker"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	tworker "go.temporal.io/sdk/worker"
)

func TestPlanningSessionWorkflow_PrepareFailureMarksSessionAbandoned(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(tworker.Options{EnableSessionWorker: true})

	markedAbandoned := false

	env.RegisterWorkflow(PlanningSessionWorkflow)
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID string) (string, error) {
		return "", errors.New("clone failed")
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.PrepareWorkspaceActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID string) error {
		markedAbandoned = true
		return nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.MarkSessionAbandonedActivity"})

	env.ExecuteWorkflow(PlanningSessionWorkflow, PlanningSessionWorkflowInput{SessionID: "session-1"})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if env.GetWorkflowError() == nil {
		t.Fatal("expected workflow error")
	}
	if !markedAbandoned {
		t.Fatal("expected session abandonment activity to run")
	}
}

func TestPlanningSessionWorkflow_InitialTimeoutRetriesOnce(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(tworker.Options{EnableSessionWorker: true})

	initialAttempts := 0
	markedAbandoned := false

	env.RegisterWorkflow(PlanningSessionWorkflow)
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID string) (string, error) {
		return "/tmp/planning", nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.PrepareWorkspaceActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID, workDir string, attempt int) error {
		initialAttempts++
		if attempt == 1 {
			return temporal.NewNonRetryableApplicationError(worker.ErrInitialResponseTimeout.Error(), worker.ErrInitialResponseTimeout.Error(), nil)
		}
		return nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.RunInitialTurnActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID string) error {
		markedAbandoned = true
		return nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.MarkSessionAbandonedActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, workDir string) error {
		return nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.CleanupWorkspaceActivity"})
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(WorkflowSignalPlanningSession, PlanningSessionSignal{Type: PlanningSessionSignalTypeAbandon})
	}, time.Second)

	env.ExecuteWorkflow(PlanningSessionWorkflow, PlanningSessionWorkflowInput{SessionID: "session-1"})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("expected workflow success after retry, got %v", err)
	}
	if initialAttempts != 2 {
		t.Fatalf("expected 2 initial attempts, got %d", initialAttempts)
	}
	if markedAbandoned {
		t.Fatal("expected session to remain active after retry success")
	}
}

func TestPlanningSessionWorkflow_InitialTimeoutTwiceMarksSessionAbandoned(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(tworker.Options{EnableSessionWorker: true})

	initialAttempts := 0
	markedAbandoned := false

	env.RegisterWorkflow(PlanningSessionWorkflow)
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID string) (string, error) {
		return "/tmp/planning", nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.PrepareWorkspaceActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID, workDir string, attempt int) error {
		initialAttempts++
		return temporal.NewNonRetryableApplicationError(worker.ErrInitialResponseTimeout.Error(), worker.ErrInitialResponseTimeout.Error(), nil)
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.RunInitialTurnActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID string) error {
		markedAbandoned = true
		return nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.MarkSessionAbandonedActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, workDir string) error {
		return nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.CleanupWorkspaceActivity"})

	env.ExecuteWorkflow(PlanningSessionWorkflow, PlanningSessionWorkflowInput{SessionID: "session-1"})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if env.GetWorkflowError() == nil {
		t.Fatal("expected workflow error")
	}
	if initialAttempts != 2 {
		t.Fatalf("expected 2 initial attempts, got %d", initialAttempts)
	}
	if !markedAbandoned {
		t.Fatal("expected session abandonment activity to run")
	}
}

func TestPlanningSessionWorkflow_InitialHardFailureDoesNotRetry(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(tworker.Options{EnableSessionWorker: true})

	initialAttempts := 0
	markedAbandoned := false

	env.RegisterWorkflow(PlanningSessionWorkflow)
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID string) (string, error) {
		return "/tmp/planning", nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.PrepareWorkspaceActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID, workDir string, attempt int) error {
		initialAttempts++
		return errors.New("LLM provider not configured for agent")
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.RunInitialTurnActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, sessionID string) error {
		markedAbandoned = true
		return nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.MarkSessionAbandonedActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, workDir string) error {
		return nil
	}, activity.RegisterOptions{Name: "PlanningSessionActivities.CleanupWorkspaceActivity"})

	env.ExecuteWorkflow(PlanningSessionWorkflow, PlanningSessionWorkflowInput{SessionID: "session-1"})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if env.GetWorkflowError() == nil {
		t.Fatal("expected workflow error")
	}
	if initialAttempts != 1 {
		t.Fatalf("expected 1 initial attempt, got %d", initialAttempts)
	}
	if !markedAbandoned {
		t.Fatal("expected session abandonment activity to run")
	}
}
