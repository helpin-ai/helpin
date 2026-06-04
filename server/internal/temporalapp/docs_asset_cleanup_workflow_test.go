package temporalapp

import (
	"context"
	"errors"
	"sync"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

type fakeDocsAssetReferenceChecker struct {
	referenced bool
	err        error
	calls      []string
}

func (f *fakeDocsAssetReferenceChecker) HasSurvivingReference(_ context.Context, _, _, assetKey string) (bool, error) {
	f.calls = append(f.calls, assetKey)
	return f.referenced, f.err
}

type fakeDocsAssetObjectStore struct {
	err     error
	deleted []string
}

func (f *fakeDocsAssetObjectStore) DeleteObject(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return f.err
}

func TestDocsAssetCleanupActivityDeletesUnreferencedAsset(t *testing.T) {
	t.Parallel()

	refs := &fakeDocsAssetReferenceChecker{}
	store := &fakeDocsAssetObjectStore{}
	activity := NewDocsAssetCleanupActivities(refs, store)

	if err := activity.CleanupAssetActivity(context.Background(), DocsAssetCleanupAssetInput{
		WorkspaceID:       "ws-1",
		DeletedDocumentID: "doc-1",
		AssetKey:          "docs-import/ws-1/import-1/image.png",
	}); err != nil {
		t.Fatalf("CleanupAssetActivity: %v", err)
	}
	if got, want := store.deleted, []string{"docs-import/ws-1/import-1/image.png"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("deleted = %v, want %v", got, want)
	}
}

func TestDocsAssetCleanupActivitySkipsReferencedAsset(t *testing.T) {
	t.Parallel()

	refs := &fakeDocsAssetReferenceChecker{referenced: true}
	store := &fakeDocsAssetObjectStore{}
	activity := NewDocsAssetCleanupActivities(refs, store)

	if err := activity.CleanupAssetActivity(context.Background(), DocsAssetCleanupAssetInput{
		WorkspaceID:       "ws-1",
		DeletedDocumentID: "doc-1",
		AssetKey:          "docs-import/ws-1/import-1/image.png",
	}); err != nil {
		t.Fatalf("CleanupAssetActivity: %v", err)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("deleted referenced asset = %v, want none", store.deleted)
	}
}

func TestDocsAssetCleanupActivityReturnsS3DeleteErrorForRetry(t *testing.T) {
	t.Parallel()

	deleteErr := errors.New("s3 down")
	refs := &fakeDocsAssetReferenceChecker{}
	store := &fakeDocsAssetObjectStore{err: deleteErr}
	activity := NewDocsAssetCleanupActivities(refs, store)

	err := activity.CleanupAssetActivity(context.Background(), DocsAssetCleanupAssetInput{
		WorkspaceID:       "ws-1",
		DeletedDocumentID: "doc-1",
		AssetKey:          "docs-import/ws-1/import-1/image.png",
	})
	if !errors.Is(err, deleteErr) {
		t.Fatalf("CleanupAssetActivity error = %v, want %v", err, deleteErr)
	}
	if len(store.deleted) != 1 {
		t.Fatalf("delete attempts = %d, want 1", len(store.deleted))
	}
}

func TestDocsAssetCleanupWorkflowSchedulesLaterAssetsBeforeFailedAssetRetriesExhaust(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	deleteErr := errors.New("s3 down")
	failedAttempts := 0
	failedAttemptsWhenNextRan := 0
	nextRan := false
	var attemptsMu sync.Mutex
	env.RegisterActivityWithOptions(func(_ context.Context, input DocsAssetCleanupAssetInput) error {
		switch input.AssetKey {
		case "docs-import/ws-1/import-1/fail.png":
			attemptsMu.Lock()
			failedAttempts++
			attemptsMu.Unlock()
			return deleteErr
		case "docs-import/ws-1/import-1/next.png":
			attemptsMu.Lock()
			nextRan = true
			failedAttemptsWhenNextRan = failedAttempts
			attemptsMu.Unlock()
			return nil
		default:
			t.Fatalf("unexpected asset key: %s", input.AssetKey)
			return nil
		}
	}, activity.RegisterOptions{Name: docsAssetCleanupActivityName})

	env.ExecuteWorkflow(DocsAssetCleanupWorkflow, DocsAssetCleanupInput{
		WorkspaceID:        "ws-1",
		DeletedDocumentID:  "doc-1",
		CandidateAssetKeys: []string{"docs-import/ws-1/import-1/fail.png", "docs-import/ws-1/import-1/next.png"},
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	attemptsMu.Lock()
	defer attemptsMu.Unlock()
	if failedAttempts != 5 {
		t.Fatalf("failed asset attempts = %d, want 5", failedAttempts)
	}
	if !nextRan {
		t.Fatal("next asset cleanup did not run")
	}
	if failedAttemptsWhenNextRan >= failedAttempts {
		t.Fatalf("next asset ran after failed asset exhausted retries; failed attempts at next = %d, total failed attempts = %d", failedAttemptsWhenNextRan, failedAttempts)
	}
}
