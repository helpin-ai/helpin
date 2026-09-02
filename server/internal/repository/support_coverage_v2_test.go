package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCoverageV2ClaimIsOwnerCheckedAndLeaseIsReclaimable(t *testing.T) {
	db := setupCoverageV2TestDB(t, "claim")
	repo := NewCoverageV2Repository(db)
	now := time.Now().UTC()
	attempt := &model.CoverageAnalysisAttempt{
		WorkspaceID: "ws-1", BatchID: "batch-1", LogicalWorkKey: "work-1", SourceKind: "conversation",
		SourceID: "conversation-1", ContentHash: "hash", AnalyzerVersion: "v2", PolicyVersion: "v1", Attempt: 1,
	}
	if err := repo.EnqueueAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("EnqueueAttempt: %v", err)
	}
	claimed, err := repo.ClaimAttempts(context.Background(), "worker-1", now, time.Minute, 10)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("ClaimAttempts = %d, %v", len(claimed), err)
	}
	if err := repo.CompleteAttempt(context.Background(), claimed[0].ID, "worker-2", now.Add(time.Second)); err == nil {
		t.Fatal("expected wrong owner completion to fail")
	}
	if err := repo.MarkRetryable(context.Background(), claimed[0].ID, "worker-1", model.CoverageFailureLLMProvider, "temporary", now.Add(-time.Second), 1); err != nil {
		t.Fatalf("MarkRetryable: %v", err)
	}
	claimed, err = repo.ClaimAttempts(context.Background(), "worker-2", now, time.Minute, 10)
	if err != nil || len(claimed) != 1 || claimed[0].LeaseOwner != "worker-2" {
		t.Fatalf("reclaim = %#v, %v", claimed, err)
	}
}

func TestCoverageV2CurrentFindingAndMembershipAreVersioned(t *testing.T) {
	db := setupCoverageV2TestDB(t, "versioning")
	repo := NewCoverageV2Repository(db)
	ctx := context.Background()
	first := coverageFindingFixture("finding-1", "attempt-1")
	second := coverageFindingFixture("finding-2", "attempt-2")
	if err := repo.ReplaceCurrentFinding(ctx, first); err != nil {
		t.Fatalf("ReplaceCurrentFinding first: %v", err)
	}
	if err := repo.ReplaceCurrentFinding(ctx, second); err != nil {
		t.Fatalf("ReplaceCurrentFinding second: %v", err)
	}
	var current int64
	if err := db.Model(&model.CoverageFinding{}).Where("is_current = ?", true).Count(&current).Error; err != nil || current != 1 {
		t.Fatalf("current findings = %d, %v", current, err)
	}
	if err := repo.SetCurrentMembership(ctx, &model.CoverageTopicMembership{WorkspaceID: "ws-1", FindingID: second.ID, TopicID: "topic-1", DecisionSource: model.CoverageMembershipAutomatic, Confidence: .9, PolicyVersion: "v1"}); err != nil {
		t.Fatalf("SetCurrentMembership first: %v", err)
	}
	if err := repo.SetCurrentMembership(ctx, &model.CoverageTopicMembership{WorkspaceID: "ws-1", FindingID: second.ID, TopicID: "topic-2", DecisionSource: model.CoverageMembershipManual, Confidence: 1, PolicyVersion: "v1"}); err != nil {
		t.Fatalf("SetCurrentMembership second: %v", err)
	}
	if err := db.Model(&model.CoverageTopicMembership{}).Where("valid_to IS NULL").Count(&current).Error; err != nil || current != 1 {
		t.Fatalf("current memberships = %d, %v", current, err)
	}
}

func TestCoverageV2AnalysisAttemptsAppendAfterFailure(t *testing.T) {
	db := setupCoverageV2TestDB(t, "attempt_history")
	repo := NewCoverageV2Repository(db)
	ctx := context.Background()
	batch, err := repo.CreateBatch(ctx, &model.CoverageBatch{WorkspaceID: "ws-1", WindowStart: time.Now().Add(-time.Hour), WindowEnd: time.Now(), AnalyzerVersion: "v2", PolicyVersion: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.StartAnalysisAttempt(ctx, batch.ID, "ws-1", "work-1", "conversation", "conversation-1", "segment-1", "hash", "v2", "v1", "worker-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkRetryable(ctx, first.ID, "worker-1", model.CoverageFailureLLMProvider, "temporary", time.Now(), 1); err != nil {
		t.Fatal(err)
	}
	second, err := repo.StartAnalysisAttempt(ctx, batch.ID, "ws-1", "work-1", "conversation", "conversation-1", "segment-1", "hash", "v2", "v1", "worker-2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if second.Attempt != 2 || second.ID == first.ID {
		t.Fatalf("second attempt = %+v, first = %+v", second, first)
	}
}

func TestCoverageV2WidgetSignalsDeduplicateSessionPrefixes(t *testing.T) {
	db := setupCoverageV2TestDB(t, "widget_signal")
	repo := NewCoverageV2Repository(db)
	now := time.Now().UTC()
	for index, query := range []string{"reset account password", "reset account password now"} {
		signal := &model.CoverageUnreviewedSignal{WorkspaceID: "ws-1", SourceKind: "widget_search", SourceID: fmt.Sprintf("event-%d", index), SessionID: "anon-1", NormalizedQuery: query, SignalKey: fmt.Sprintf("key-%d", index), MeaningfulTokens: 3 + index, Status: model.CoverageSignalUnreviewed, ObservedAt: now.Add(time.Duration(index) * time.Second)}
		if err := repo.UpsertUnreviewedSignal(context.Background(), signal); err != nil {
			t.Fatal(err)
		}
	}
	var signals []model.CoverageUnreviewedSignal
	if err := db.Find(&signals).Error; err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 || signals[0].NormalizedQuery != "reset account password now" {
		t.Fatalf("deduplicated signals = %+v", signals)
	}
}

func setupCoverageV2TestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:coverage_v2_"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE coverage_batches (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, window_start DATETIME NOT NULL, window_end DATETIME NOT NULL, analyzer_version TEXT NOT NULL, policy_version TEXT NOT NULL, status TEXT NOT NULL, cursor TEXT NOT NULL DEFAULT '', lease_owner TEXT NOT NULL DEFAULT '', lease_expires_at DATETIME, candidate_count INTEGER NOT NULL DEFAULT 0, succeeded_count INTEGER NOT NULL DEFAULT 0, retryable_count INTEGER NOT NULL DEFAULT 0, dead_letter_count INTEGER NOT NULL DEFAULT 0, failure_class TEXT NOT NULL DEFAULT '', failure_message TEXT NOT NULL DEFAULT '', correlation_id TEXT NOT NULL DEFAULT '', metadata TEXT NOT NULL DEFAULT '{}', started_at DATETIME, completed_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(workspace_id, window_start, window_end, analyzer_version, policy_version))`,
		`CREATE TABLE coverage_analysis_attempts (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, batch_id TEXT NOT NULL, logical_work_key TEXT NOT NULL, source_kind TEXT NOT NULL, source_id TEXT NOT NULL, segment_id TEXT NOT NULL DEFAULT '', content_hash TEXT NOT NULL, analyzer_version TEXT NOT NULL, policy_version TEXT NOT NULL, attempt INTEGER NOT NULL, status TEXT NOT NULL, stage TEXT NOT NULL, lease_owner TEXT NOT NULL DEFAULT '', lease_expires_at DATETIME, retry_at DATETIME, retry_budget_used INTEGER NOT NULL DEFAULT 0, failure_class TEXT NOT NULL DEFAULT '', failure_message TEXT NOT NULL DEFAULT '', ai_execution_id TEXT, correlation_id TEXT NOT NULL DEFAULT '', metadata TEXT NOT NULL DEFAULT '{}', started_at DATETIME, completed_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(workspace_id, logical_work_key, attempt))`,
		`CREATE TABLE coverage_findings (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, logical_work_key TEXT NOT NULL, analysis_attempt_id TEXT NOT NULL, source_kind TEXT NOT NULL, source_id TEXT NOT NULL, conversation_id TEXT, customer_id TEXT, customer_need TEXT NOT NULL, ai_answer TEXT NOT NULL DEFAULT '', ai_failure TEXT NOT NULL DEFAULT '', human_answer TEXT NOT NULL DEFAULT '', fix_type TEXT NOT NULL, fix_target TEXT NOT NULL, rationale TEXT NOT NULL, suggested_change TEXT NOT NULL, confidence REAL NOT NULL, is_current INTEGER NOT NULL DEFAULT 1, embedding_status TEXT NOT NULL DEFAULT 'pending', assignment_status TEXT NOT NULL DEFAULT 'pending', embedding TEXT, embedding_provider TEXT NOT NULL DEFAULT '', embedding_model TEXT NOT NULL DEFAULT '', embedding_version TEXT NOT NULL DEFAULT '', embedding_dimensions INTEGER NOT NULL DEFAULT 0, metadata TEXT NOT NULL DEFAULT '{}', superseded_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE coverage_topic_memberships (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, finding_id TEXT NOT NULL, topic_id TEXT NOT NULL, decision_source TEXT NOT NULL, confidence REAL NOT NULL, policy_version TEXT NOT NULL, assignment_attempt_id TEXT, actor_id TEXT, valid_from DATETIME NOT NULL, valid_to DATETIME, metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE coverage_unreviewed_signals (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, source_kind TEXT NOT NULL, source_id TEXT NOT NULL, session_id TEXT NOT NULL DEFAULT '', normalized_query TEXT NOT NULL, signal_key TEXT NOT NULL, meaningful_tokens INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, confidence REAL NOT NULL DEFAULT 0, finding_id TEXT, topic_id TEXT, reviewed_by TEXT, reviewed_at DATETIME, metadata TEXT NOT NULL DEFAULT '{}', observed_at DATETIME NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(workspace_id, signal_key))`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func coverageFindingFixture(id, attemptID string) *model.CoverageFinding {
	return &model.CoverageFinding{ID: id, WorkspaceID: "ws-1", LogicalWorkKey: "work-1", AnalysisAttemptID: attemptID, SourceKind: "conversation", SourceID: "conversation-1", CustomerNeed: "reset password", AIFailure: "could not answer", FixType: "update_article", FixTarget: "Password help", Rationale: "missing steps", SuggestedChange: "Add reset steps", Confidence: .9}
}
