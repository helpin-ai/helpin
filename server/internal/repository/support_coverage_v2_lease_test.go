package repository

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupCoverageLeaseTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := setupCoverageV2TestDB(t, name)
	// Production allows one live lease per piece of work.
	if err := db.Exec(`CREATE UNIQUE INDEX uq_coverage_attempt_active_lease ON coverage_analysis_attempts (workspace_id, logical_work_key) WHERE status = 'leased'`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func enqueueCoverageAttempts(t *testing.T, repo *CoverageV2Repository, attempts ...model.CoverageAnalysisAttempt) {
	t.Helper()
	for index := range attempts {
		attempt := attempts[index]
		if err := repo.EnqueueAttempt(context.Background(), &attempt); err != nil {
			t.Fatalf("EnqueueAttempt %d: %v", attempt.Attempt, err)
		}
	}
}

func coverageAttemptFixture(attempt int, status string) model.CoverageAnalysisAttempt {
	return model.CoverageAnalysisAttempt{
		WorkspaceID: "ws-1", BatchID: "batch-1", LogicalWorkKey: "work-1", SourceKind: "conversation",
		SourceID: "conversation-1", ContentHash: "hash", AnalyzerVersion: "v4", PolicyVersion: "v1",
		Attempt: attempt, Status: status,
	}
}

func TestCoverageV2ClaimLeasesOnlyTheNewestAttemptForWork(t *testing.T) {
	db := setupCoverageLeaseTestDB(t, "claim_newest")
	repo := NewCoverageV2Repository(db)
	now := time.Now().UTC()
	due := now.Add(-time.Hour)
	older, newer := coverageAttemptFixture(1, model.CoverageAttemptRetryable), coverageAttemptFixture(2, model.CoverageAttemptRetryable)
	older.RetryAt, newer.RetryAt = &due, &due
	enqueueCoverageAttempts(t, repo, older, newer)

	claimed, err := repo.ClaimAttemptsForWorkspace(context.Background(), "ws-1", "worker", now, time.Minute, 10)
	if err != nil {
		t.Fatalf("ClaimAttemptsForWorkspace: %v", err)
	}
	if len(claimed) != 1 || claimed[0].Attempt != 2 {
		t.Fatalf("claimed = %+v, want only attempt 2", claimed)
	}
}

func TestCoverageV2ClaimReleasesAnOlderExpiredLease(t *testing.T) {
	db := setupCoverageLeaseTestDB(t, "claim_stale_lease")
	repo := NewCoverageV2Repository(db)
	now := time.Now().UTC()
	expired, due := now.Add(-time.Hour), now.Add(-time.Minute)
	stale, newer := coverageAttemptFixture(1, model.CoverageAttemptLeased), coverageAttemptFixture(2, model.CoverageAttemptRetryable)
	stale.LeaseOwner, stale.LeaseExpiresAt, newer.RetryAt = "stopped-worker", &expired, &due
	enqueueCoverageAttempts(t, repo, stale, newer)

	claimed, err := repo.ClaimAttemptsForWorkspace(context.Background(), "ws-1", "worker", now, time.Minute, 10)
	if err != nil {
		t.Fatalf("ClaimAttemptsForWorkspace: %v", err)
	}
	if len(claimed) != 1 || claimed[0].Attempt != 2 {
		t.Fatalf("claimed = %+v, want attempt 2", claimed)
	}
	var released model.CoverageAnalysisAttempt
	if err := db.First(&released, "logical_work_key = ? AND attempt = 1", "work-1").Error; err != nil {
		t.Fatal(err)
	}
	if released.Status != model.CoverageAttemptRetryable || released.FailureClass != model.CoverageFailureLeaseLost {
		t.Fatalf("stale attempt = %s/%s, want retryable/lease_lost", released.Status, released.FailureClass)
	}
}

func TestCoverageV2StartAttemptReplacesAnExpiredLease(t *testing.T) {
	db := setupCoverageLeaseTestDB(t, "start_stale_lease")
	repo := NewCoverageV2Repository(db)
	ctx := context.Background()
	batch, err := repo.CreateBatch(ctx, &model.CoverageBatch{WorkspaceID: "ws-1", WindowStart: time.Now().Add(-time.Hour), WindowEnd: time.Now(), AnalyzerVersion: "v4", PolicyVersion: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.StartAnalysisAttempt(ctx, batch.ID, "ws-1", "work-1", "conversation", "conversation-1", "segment-1", "hash", "v4", "v1", "stopped-worker", time.Nanosecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)

	second, err := repo.StartAnalysisAttempt(ctx, batch.ID, "ws-1", "work-1", "conversation", "conversation-1", "segment-1", "hash", "v4", "v1", "worker", time.Minute)
	if err != nil {
		t.Fatalf("StartAnalysisAttempt after expired lease: %v", err)
	}
	if second.Attempt != 2 || second.LeaseOwner != "worker" {
		t.Fatalf("second attempt = %+v", second)
	}
	var released model.CoverageAnalysisAttempt
	if err := db.First(&released, "id = ?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if released.Status != model.CoverageAttemptRetryable {
		t.Fatalf("expired attempt status = %s, want retryable", released.Status)
	}
}
