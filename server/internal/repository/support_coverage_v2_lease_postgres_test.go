//go:build integration

package repository

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// The production claim locks rows with FOR UPDATE SKIP LOCKED and relies on the
// partial one-lease index, which SQLite tests only approximate.
func TestCoverageV2ClaimOnPostgresSkipsOlderAttemptsAndStaleLeases(t *testing.T) {
	tx := setupCoverageAnalysisPostgres(t)
	ddl, err := os.ReadFile("../dbmigrate/sql/202609020002_coverage_topics_v2.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Batches, attempts and their indexes come first in the migration.
	attemptsDDL, _, found := strings.Cut(string(ddl), "CREATE TABLE IF NOT EXISTS coverage_findings")
	if !found {
		t.Fatal("coverage_findings table not found in the coverage v2 migration")
	}
	if err := tx.Exec(attemptsDDL).Error; err != nil {
		t.Fatalf("apply coverage v2 schema: %v", err)
	}
	repo := NewCoverageV2Repository(tx)
	ctx := context.Background()
	now := time.Now().UTC()
	batch, err := repo.CreateBatch(ctx, &model.CoverageBatch{WorkspaceID: "11111111-1111-1111-1111-111111111111", WindowStart: now.Add(-time.Hour), WindowEnd: now, AnalyzerVersion: "v4", PolicyVersion: "v1", Status: model.CoverageBatchRunning})
	if err != nil {
		t.Fatal(err)
	}
	expired, due := now.Add(-time.Hour), now.Add(-time.Minute)
	fixtures := []model.CoverageAnalysisAttempt{
		coverageAttemptFixture(1, model.CoverageAttemptRetryable),
		coverageAttemptFixture(2, model.CoverageAttemptLeased),
		coverageAttemptFixture(3, model.CoverageAttemptRetryable),
	}
	fixtures[0].RetryAt = &due
	fixtures[1].LeaseOwner, fixtures[1].LeaseExpiresAt = "stopped-worker", &expired
	fixtures[2].RetryAt = &due
	for index := range fixtures {
		fixtures[index].WorkspaceID, fixtures[index].BatchID = batch.WorkspaceID, batch.ID
	}
	enqueueCoverageAttempts(t, repo, fixtures...)

	claimed, err := repo.ClaimAttemptsForWorkspace(ctx, batch.WorkspaceID, "worker", now, time.Minute, 10)
	if err != nil {
		t.Fatalf("ClaimAttemptsForWorkspace: %v", err)
	}
	if len(claimed) != 1 || claimed[0].Attempt != 3 {
		t.Fatalf("claimed = %+v, want only attempt 3", claimed)
	}
	var leased int64
	if err := tx.Model(&model.CoverageAnalysisAttempt{}).Where("logical_work_key = ? AND status = ?", "work-1", model.CoverageAttemptLeased).Count(&leased).Error; err != nil {
		t.Fatal(err)
	}
	if leased != 1 {
		t.Fatalf("leased attempts = %d, want 1", leased)
	}
}
