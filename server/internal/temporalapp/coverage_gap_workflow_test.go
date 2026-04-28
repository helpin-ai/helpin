package temporalapp

import (
	"context"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

type fakeCoverageGapEnricher struct {
	topicIDs []string
}

func (f *fakeCoverageGapEnricher) EnrichTopic(ctx context.Context, topicID string) error {
	f.topicIDs = append(f.topicIDs, topicID)
	return nil
}

type fakeCoverageGapBatcher struct {
	workspaces []string
	topics     map[string][]string
}

func (f *fakeCoverageGapBatcher) ListWorkspacesWithOpenGaps(ctx context.Context) ([]string, error) {
	return f.workspaces, nil
}

func (f *fakeCoverageGapBatcher) ListTopicsDueForEnrichment(ctx context.Context, workspaceID string, olderThan time.Duration, minEvidence int) ([]string, error) {
	return f.topics[workspaceID], nil
}

func TestCoverageGapEnrichmentWorkflow_ExecutesActivity(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	enricher := &fakeCoverageGapEnricher{}
	activities := newCoverageGapActivitiesForTest(enricher, &fakeCoverageGapBatcher{})
	env.RegisterActivityWithOptions(activities.EnrichTopicActivity, activity.RegisterOptions{
		Name: CoverageGapEnrichmentActivityName,
	})

	env.ExecuteWorkflow(CoverageGapEnrichmentWorkflow, "topic-1")

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	if len(enricher.topicIDs) != 1 || enricher.topicIDs[0] != "topic-1" {
		t.Fatalf("enriched topics=%v, want [topic-1]", enricher.topicIDs)
	}
}

func TestCoverageGapPerWorkspaceWorkflow_EnrichesDueTopics(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	enricher := &fakeCoverageGapEnricher{}
	batcher := &fakeCoverageGapBatcher{topics: map[string][]string{"ws-1": {"topic-1", "topic-2"}}}
	activities := newCoverageGapActivitiesForTest(enricher, batcher)
	env.RegisterActivityWithOptions(activities.ListTopicsForBatchActivity, activity.RegisterOptions{
		Name: CoverageGapListBatchActivityName,
	})
	env.RegisterWorkflow(CoverageGapEnrichmentWorkflow)
	env.RegisterActivityWithOptions(activities.EnrichTopicActivity, activity.RegisterOptions{
		Name: CoverageGapEnrichmentActivityName,
	})

	env.ExecuteWorkflow(CoverageGapPerWorkspaceWorkflow, "ws-1")

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	if len(enricher.topicIDs) != 2 {
		t.Fatalf("enriched topics=%v, want 2 topics", enricher.topicIDs)
	}
}
