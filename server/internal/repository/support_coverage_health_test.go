package repository

import (
	"context"
	"testing"
	"time"
)

func TestListFailedAttemptsForBatchIgnoresOlderBatches(t *testing.T) {
	db := setupCoverageV2TestDB(t, "failed_attempts_for_batch")
	repo := NewCoverageV2Repository(db)
	now := time.Now().UTC()
	for _, statement := range []string{
		`INSERT INTO coverage_analysis_attempts (id, workspace_id, batch_id, logical_work_key, source_kind, source_id, content_hash, analyzer_version, policy_version, attempt, status, stage, created_at, metadata) VALUES ('old-failure', 'ws-1', 'batch-old', 'work-old', 'conversation', 'c-1', 'h', 'v4', 'v1', 1, 'dead_letter', 'llm_preflight', ?, CAST('{}' AS BLOB))`,
		`INSERT INTO coverage_analysis_attempts (id, workspace_id, batch_id, logical_work_key, source_kind, source_id, content_hash, analyzer_version, policy_version, attempt, status, stage, created_at, metadata) VALUES ('new-retry', 'ws-1', 'batch-new', 'work-new', 'conversation', 'c-2', 'h', 'v4', 'v1', 1, 'retryable', 'llm_preflight', ?, CAST('{}' AS BLOB))`,
		`INSERT INTO coverage_analysis_attempts (id, workspace_id, batch_id, logical_work_key, source_kind, source_id, content_hash, analyzer_version, policy_version, attempt, status, stage, created_at, metadata) VALUES ('new-done', 'ws-1', 'batch-new', 'work-done', 'conversation', 'c-3', 'h', 'v4', 'v1', 1, 'succeeded', 'llm_preflight', ?, CAST('{}' AS BLOB))`,
		`INSERT INTO coverage_analysis_attempts (id, workspace_id, batch_id, logical_work_key, source_kind, source_id, content_hash, analyzer_version, policy_version, attempt, status, stage, created_at, metadata) VALUES ('other-workspace', 'ws-2', 'batch-new', 'work-other', 'conversation', 'c-4', 'h', 'v4', 'v1', 1, 'dead_letter', 'llm_preflight', ?, CAST('{}' AS BLOB))`,
	} {
		if err := db.Exec(statement, now).Error; err != nil {
			t.Fatal(err)
		}
	}

	failures, err := repo.ListFailedAttemptsForBatch(context.Background(), "ws-1", "batch-new", 25)
	if err != nil {
		t.Fatalf("ListFailedAttemptsForBatch: %v", err)
	}
	if len(failures) != 1 || failures[0].ID != "new-retry" {
		t.Fatalf("failures = %+v, want only new-retry", failures)
	}
}
