package temporalapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

type fakeCoverageDailyAnalyzer struct {
	workspaces []string
	runs       []coverageAnalysisRunCall
	err        error
}

type coverageAnalysisRunCall struct {
	workspaceID string
	windowStart time.Time
	windowEnd   time.Time
}

func (f *fakeCoverageDailyAnalyzer) ListWorkspacesForDailyAnalysis(ctx context.Context) ([]string, error) {
	return f.workspaces, nil
}

func (f *fakeCoverageDailyAnalyzer) RunWorkspaceDailyAnalysis(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time) error {
	if f.err != nil {
		return f.err
	}
	f.runs = append(f.runs, coverageAnalysisRunCall{workspaceID: workspaceID, windowStart: windowStart, windowEnd: windowEnd})
	return nil
}

func TestCoverageDailyAnalysisWorkflow_FansOutWorkspaces(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	start := time.Date(2026, 4, 29, 4, 30, 0, 0, time.UTC)
	env.SetStartTime(start)

	analyzer := &fakeCoverageDailyAnalyzer{workspaces: []string{"ws-1", "ws-2"}}
	activities := NewCoverageAnalysisActivities(analyzer)
	env.RegisterWorkflow(CoverageWorkspaceAnalysisWorkflow)
	env.RegisterActivityWithOptions(activities.ListWorkspacesActivity, activity.RegisterOptions{Name: CoverageListAnalysisWorkspacesActivity})
	env.RegisterActivityWithOptions(activities.RunWorkspaceAnalysisActivity, activity.RegisterOptions{Name: CoverageRunWorkspaceAnalysisActivityName})

	env.ExecuteWorkflow(CoverageDailyAnalysisWorkflow)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	if len(analyzer.runs) != 2 {
		t.Fatalf("expected 2 workspace runs, got %+v", analyzer.runs)
	}
	seen := map[string]bool{}
	for _, run := range analyzer.runs {
		seen[run.workspaceID] = true
	}
	if !seen["ws-1"] || !seen["ws-2"] {
		t.Fatalf("unexpected workspace runs: %+v", analyzer.runs)
	}
}

func TestCoverageWorkspaceAnalysisWorkflow_CallsActivityWithWindow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	analyzer := &fakeCoverageDailyAnalyzer{}
	activities := NewCoverageAnalysisActivities(analyzer)
	env.RegisterActivityWithOptions(activities.RunWorkspaceAnalysisActivity, activity.RegisterOptions{Name: CoverageRunWorkspaceAnalysisActivityName})
	windowStart := time.Date(2026, 4, 28, 4, 30, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 4, 29, 4, 30, 0, 0, time.UTC)

	env.ExecuteWorkflow(CoverageWorkspaceAnalysisWorkflow, CoverageWorkspaceAnalysisInput{
		WorkspaceID: "ws-1",
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	if len(analyzer.runs) != 1 {
		t.Fatalf("expected one workspace run, got %+v", analyzer.runs)
	}
	run := analyzer.runs[0]
	if run.workspaceID != "ws-1" || !run.windowStart.Equal(windowStart) || !run.windowEnd.Equal(windowEnd) {
		t.Fatalf("unexpected activity input: %+v", run)
	}
}

func TestCoverageWorkspaceAnalysisWorkflow_FailsOnActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	analyzer := &fakeCoverageDailyAnalyzer{err: errors.New("analysis failed")}
	activities := NewCoverageAnalysisActivities(analyzer)
	env.RegisterActivityWithOptions(activities.RunWorkspaceAnalysisActivity, activity.RegisterOptions{Name: CoverageRunWorkspaceAnalysisActivityName})

	env.ExecuteWorkflow(CoverageWorkspaceAnalysisWorkflow, CoverageWorkspaceAnalysisInput{WorkspaceID: "ws-1"})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected workflow error")
	}
}

func TestCoverageDailyAnalysisWorkflow_FailsWhenWorkspaceChildFails(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	start := time.Date(2026, 4, 29, 4, 30, 0, 0, time.UTC)
	env.SetStartTime(start)

	analyzer := &fakeCoverageDailyAnalyzer{
		workspaces: []string{"ws-1"},
		err:        errors.New("analysis failed"),
	}
	activities := NewCoverageAnalysisActivities(analyzer)
	env.RegisterWorkflow(CoverageWorkspaceAnalysisWorkflow)
	env.RegisterActivityWithOptions(activities.ListWorkspacesActivity, activity.RegisterOptions{Name: CoverageListAnalysisWorkspacesActivity})
	env.RegisterActivityWithOptions(activities.RunWorkspaceAnalysisActivity, activity.RegisterOptions{Name: CoverageRunWorkspaceAnalysisActivityName})

	env.ExecuteWorkflow(CoverageDailyAnalysisWorkflow)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected workflow error")
	}
}

func TestCoverageDailyAnalysisWorkflow_ConcurrencyBound(t *testing.T) {
	if CoverageAnalysisMaxWorkspaceChildren != 10 {
		t.Fatalf("workspace child concurrency = %d, want 10", CoverageAnalysisMaxWorkspaceChildren)
	}
}
