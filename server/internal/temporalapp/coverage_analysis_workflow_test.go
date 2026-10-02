package temporalapp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

type fakeCoverageDailyAnalyzer struct {
	mu                 sync.Mutex
	reanalysisRequests []string
	workspaces         []string
	runs               []coverageAnalysisRunCall
	err                error
	errorsByWorkspace  map[string]error
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
	// Temporal executes the child activities concurrently.
	f.mu.Lock()
	f.runs = append(f.runs, coverageAnalysisRunCall{workspaceID: workspaceID, windowStart: windowStart, windowEnd: windowEnd})
	f.mu.Unlock()
	if err := f.errorsByWorkspace[workspaceID]; err != nil {
		return err
	}
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *fakeCoverageDailyAnalyzer) recordedRuns() []coverageAnalysisRunCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]coverageAnalysisRunCall(nil), f.runs...)
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
	runs := analyzer.recordedRuns()
	if len(runs) != 2 {
		t.Fatalf("expected 2 workspace runs, got %+v", runs)
	}
	seen := map[string]bool{}
	for _, run := range runs {
		seen[run.workspaceID] = true
	}
	if !seen["ws-1"] || !seen["ws-2"] {
		t.Fatalf("unexpected workspace runs: %+v", runs)
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
	runs := analyzer.recordedRuns()
	if len(runs) != 1 {
		t.Fatalf("expected one workspace run, got %+v", runs)
	}
	run := runs[0]
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

func TestCoverageDailyAnalysisWorkflow_WorkspaceFailureDoesNotStopLaterWorkspaces(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	start := time.Date(2026, 4, 29, 4, 30, 0, 0, time.UTC)
	env.SetStartTime(start)

	analyzer := &fakeCoverageDailyAnalyzer{
		workspaces:        []string{"ws-1", "ws-2"},
		errorsByWorkspace: map[string]error{"ws-1": errors.New("analysis failed")},
	}
	activities := NewCoverageAnalysisActivities(analyzer)
	env.RegisterWorkflow(CoverageWorkspaceAnalysisWorkflow)
	env.RegisterActivityWithOptions(activities.ListWorkspacesActivity, activity.RegisterOptions{Name: CoverageListAnalysisWorkspacesActivity})
	env.RegisterActivityWithOptions(activities.RunWorkspaceAnalysisActivity, activity.RegisterOptions{Name: CoverageRunWorkspaceAnalysisActivityName})

	env.ExecuteWorkflow(CoverageDailyAnalysisWorkflow)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("parent workflow should isolate child failures: %v", err)
	}
	seenWS2 := false
	runs := analyzer.recordedRuns()
	for _, run := range runs {
		seenWS2 = seenWS2 || run.workspaceID == "ws-2"
	}
	if !seenWS2 {
		t.Fatalf("expected later workspace to run despite ws-1 retries, got %+v", runs)
	}
}

func TestCoverageDailyAnalysisWorkflow_UsesThreeHourWindow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	start := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	env.SetStartTime(start)
	analyzer := &fakeCoverageDailyAnalyzer{workspaces: []string{"ws-1"}}
	activities := NewCoverageAnalysisActivities(analyzer)
	env.RegisterWorkflow(CoverageWorkspaceAnalysisWorkflow)
	env.RegisterActivityWithOptions(activities.ListWorkspacesActivity, activity.RegisterOptions{Name: CoverageListAnalysisWorkspacesActivity})
	env.RegisterActivityWithOptions(activities.RunWorkspaceAnalysisActivity, activity.RegisterOptions{Name: CoverageRunWorkspaceAnalysisActivityName})
	env.ExecuteWorkflow(CoverageDailyAnalysisWorkflow)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	runs := analyzer.recordedRuns()
	if len(runs) != 1 || runs[0].windowEnd.Sub(runs[0].windowStart) != 3*time.Hour {
		t.Fatalf("unexpected analysis window: %+v", runs)
	}
}

func TestCoverageDailyAnalysisWorkflow_ConcurrencyBound(t *testing.T) {
	if CoverageAnalysisMaxWorkspaceChildren != 10 {
		t.Fatalf("workspace child concurrency = %d, want 10", CoverageAnalysisMaxWorkspaceChildren)
	}
}

func (f *fakeCoverageDailyAnalyzer) RunWorkspaceReanalysis(ctx context.Context, workspaceID string, start, end time.Time, requestID string) error {
	f.mu.Lock()
	f.reanalysisRequests = append(f.reanalysisRequests, requestID)
	f.mu.Unlock()
	return f.RunWorkspaceDailyAnalysis(ctx, workspaceID, start, end)
}
func TestCoverageWorkspaceAnalysisWorkflow_ExplicitReanalysisReachesAnalyzer(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	analyzer := &fakeCoverageDailyAnalyzer{}
	activities := NewCoverageAnalysisActivities(analyzer)
	env.RegisterActivityWithOptions(activities.RunWorkspaceAnalysisActivity, activity.RegisterOptions{Name: CoverageRunWorkspaceAnalysisActivityName})
	env.ExecuteWorkflow(CoverageWorkspaceAnalysisWorkflow, CoverageWorkspaceAnalysisInput{WorkspaceID: "ws-1", Reanalyze: true, RequestID: "request"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if len(analyzer.reanalysisRequests) != 1 || analyzer.reanalysisRequests[0] != "request" {
		t.Fatalf("missing reassessment flag: %+v", analyzer.reanalysisRequests)
	}
}
