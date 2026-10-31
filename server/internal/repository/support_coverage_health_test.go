package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestListFailedAttemptsHidesFailuresOfAnOlderConversationSnapshot(t *testing.T) {
	db := setupCoverageV2TestDB(t, "recovered_snapshot")
	repo := NewCoverageV2Repository(db)
	now := time.Now().UTC()
	for _, row := range []struct {
		id, source, segment, hash, status string
		at                                time.Time
	}{
		{"old-failure", "c-1", "old-segment", "old-hash", model.CoverageAttemptDeadLetter, now.Add(-time.Hour)},
		{"recovered", "c-1", "new-segment", "new-hash", model.CoverageAttemptSucceeded, now},
		{"unresolved", "c-2", "segment", "hash", model.CoverageAttemptDeadLetter, now},
	} {
		attempt := &model.CoverageAnalysisAttempt{ID: row.id, WorkspaceID: "ws-1", BatchID: row.id,
			LogicalWorkKey: row.id, SourceKind: "conversation", SourceID: row.source, SegmentID: row.segment, ContentHash: row.hash,
			AnalyzerVersion: "v4", PolicyVersion: "v1", Attempt: 1, Status: row.status, CreatedAt: row.at}
		if err := repo.EnqueueAttempt(context.Background(), attempt); err != nil {
			t.Fatal(err)
		}
	}
	failures, err := repo.ListFailedAttempts(context.Background(), "ws-1", 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || failures[0].ID != "unresolved" {
		t.Fatalf("failures = %+v, want only the conversation that has not recovered", failures)
	}
}

func TestCoverageClaimDoesNotRetryWorkSupersededByReanalysis(t *testing.T) {
	db := setupCoverageV2TestDB(t, "superseded_retry")
	repo := NewCoverageV2Repository(db)
	now := time.Now().UTC()
	for i, status := range []string{model.CoverageAttemptRetryable, model.CoverageAttemptSucceeded} {
		attempt := &model.CoverageAnalysisAttempt{ID: fmt.Sprint(i), WorkspaceID: "ws-1", BatchID: "batch",
			LogicalWorkKey: fmt.Sprint(i), SourceKind: "conversation", SourceID: "c-1", SegmentID: fmt.Sprint(i), ContentHash: fmt.Sprint(i),
			AnalyzerVersion: "v4", PolicyVersion: "v1", Attempt: 1, Status: status, CreatedAt: now.Add(time.Duration(i-2) * time.Hour)}
		if err := repo.EnqueueAttempt(context.Background(), attempt); err != nil {
			t.Fatal(err)
		}
	}
	claimed, err := repo.ClaimAttemptsForWorkspace(context.Background(), "ws-1", "worker", now, time.Minute, 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 0 {
		t.Fatalf("claimed superseded failure: %+v", claimed)
	}
	workspaces, err := repo.ListQueuedWorkspaces(context.Background(), 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 0 {
		t.Fatalf("superseded work kept workspace scheduled: %v", workspaces)
	}
}

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

func TestListFailedAttemptsKeepsUnresolvedHistoryButHidesRecoveredWork(t *testing.T) {
	db := setupCoverageV2TestDB(t, "unresolved_failures")
	repo := NewCoverageV2Repository(db)
	now := time.Now().UTC()
	for _, row := range []struct {
		id, ws, source, status string
		at                     time.Time
	}{
		{"old-unresolved", "ws-1", "c-1", "dead_letter", now.Add(-time.Hour)},
		{"old-recovered", "ws-1", "c-2", "dead_letter", now.Add(-time.Hour)},
		{"reassessed", "ws-1", "c-2", "succeeded", now},
		{"old-retry", "ws-1", "c-3", "retryable", now.Add(-time.Hour)},
		{"new-retry", "ws-1", "c-3", "retryable", now},
		{"other-workspace", "ws-2", "c-1", "succeeded", now},
	} {
		attempt := &model.CoverageAnalysisAttempt{ID: row.id, WorkspaceID: row.ws, BatchID: row.id,
			LogicalWorkKey: row.id, SourceKind: "conversation", SourceID: row.source, SegmentID: "segment", ContentHash: "hash",
			AnalyzerVersion: "v4", PolicyVersion: "v1", Attempt: 1, Status: row.status, CreatedAt: row.at}
		if err := repo.EnqueueAttempt(context.Background(), attempt); err != nil {
			t.Fatal(err)
		}
	}
	failures, err := repo.ListFailedAttempts(context.Background(), "ws-1", 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 2 || failures[0].ID != "old-unresolved" || failures[1].ID != "new-retry" {
		t.Fatalf("want unresolved terminal failure first then current retry, got %+v", failures)
	}
}

func TestCoverageSchedulesPendingAssignmentsWithoutNewConversations(t *testing.T) {
	db := setupCoverageV2TestDB(t, "pending_assignments")
	repo := NewCoverageV2Repository(db)
	for _, row := range []struct {
		ws, status string
		current    bool
	}{
		{"ws-pending", "pending", true}, {"ws-retry", "retryable", true},
		{"ws-done", "assigned", true}, {"ws-old", "pending", false},
	} {
		finding := &model.CoverageFinding{ID: row.ws, WorkspaceID: row.ws, AnalysisAttemptID: row.ws,
			SourceKind: "conversation", SourceID: row.ws, LogicalWorkKey: row.ws, IsCurrent: true,
			AssignmentStatus: row.status, Metadata: []byte(`{}`)}
		if err := db.Create(finding).Error; err != nil {
			t.Fatal(err)
		}
		if !row.current {
			if err := db.Model(finding).Update("is_current", false).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	workspaces, err := repo.ListQueuedWorkspaces(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 2 || workspaces[0] != "ws-pending" || workspaces[1] != "ws-retry" {
		t.Fatalf("assignment backlog not scheduled: %v", workspaces)
	}
}
