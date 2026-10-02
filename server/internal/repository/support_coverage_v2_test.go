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

func TestCoverageInsightPagesIncludeTotals(t *testing.T) {
	db := setupCoverageV2TestDB(t, "pagination")
	repo := NewCoverageV2Repository(db)
	for i := 0; i < 28; i++ {
		id := fmt.Sprintf("item-%02d", i)
		if err := db.Exec(`INSERT INTO coverage_topics (id, workspace_id, canonical_key, title, customer_need, status, assignment_policy, metadata) VALUES (?, 'ws-1', ?, ?, 'Need help', 'open', 'v1', ?)`, id, id, id, []byte(`{}`)).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO coverage_unreviewed_signals (id, workspace_id, signal_key, source_kind, source_id, normalized_query, meaningful_tokens, confidence, status, observed_at, metadata) VALUES (?, 'ws-1', ?, 'conversation', ?, 'Need help', 2, 0.4, 'unreviewed', ?, ?)`, id, id, id, time.Now().UTC(), []byte(`{}`)).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`UPDATE coverage_topics SET status = 'archived' WHERE id = 'item-00'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE coverage_unreviewed_signals SET status = 'dismissed' WHERE id = 'item-00'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE coverage_topics SET workspace_id = 'ws-2' WHERE id = 'item-01'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE coverage_unreviewed_signals SET workspace_id = 'ws-2' WHERE id = 'item-01'`).Error; err != nil {
		t.Fatal(err)
	}
	topics, err := repo.ListTopics(context.Background(), "ws-1", 25, 25)
	if err != nil || len(topics) != 1 {
		t.Fatalf("topic second page: %d, %v", len(topics), err)
	}
	signals, err := repo.ListUnreviewedSignals(context.Background(), "ws-1", 25, 25)
	if err != nil || len(signals) != 1 {
		t.Fatalf("signal second page: %d, %v", len(signals), err)
	}
	topicCount, err := repo.CountTopics(context.Background(), "ws-1")
	if err != nil || topicCount != 26 {
		t.Fatalf("topic total: %d, %v", topicCount, err)
	}
	signalCount, err := repo.CountUnreviewedSignals(context.Background(), "ws-1")
	if err != nil || signalCount != 26 {
		t.Fatalf("signal total: %d, %v", signalCount, err)
	}
}

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

func TestCoverageV2ClaimsAndListsOnlyRunnableWorkspaceWork(t *testing.T) {
	db := setupCoverageV2TestDB(t, "workspace_claim")
	repo := NewCoverageV2Repository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	fixtures := []model.CoverageAnalysisAttempt{
		{WorkspaceID: "ws-1", BatchID: "batch-1", LogicalWorkKey: "work-1", SourceKind: "conversation", SourceID: "conversation-1", ContentHash: "hash-1", AnalyzerVersion: "v4", PolicyVersion: "v1", Attempt: 1},
		{WorkspaceID: "ws-2", BatchID: "batch-2", LogicalWorkKey: "work-2", SourceKind: "conversation", SourceID: "conversation-2", ContentHash: "hash-2", AnalyzerVersion: "v4", PolicyVersion: "v1", Attempt: 1},
		{WorkspaceID: "ws-paused", BatchID: "batch-3", LogicalWorkKey: "work-3", SourceKind: "conversation", SourceID: "conversation-3", ContentHash: "hash-3", AnalyzerVersion: "v4", PolicyVersion: "v1", Attempt: 1, Status: model.CoverageAttemptPausedConfiguration},
	}
	for index := range fixtures {
		if err := repo.EnqueueAttempt(ctx, &fixtures[index]); err != nil {
			t.Fatal(err)
		}
	}
	workspaces, err := repo.ListQueuedWorkspaces(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 2 || workspaces[0] != "ws-1" || workspaces[1] != "ws-2" {
		t.Fatalf("runnable workspaces = %#v", workspaces)
	}
	claimed, err := repo.ClaimAttemptsForWorkspace(ctx, "ws-1", "worker-1", now, time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].WorkspaceID != "ws-1" {
		t.Fatalf("workspace-scoped claims = %#v", claimed)
	}
}

func TestCoverageV2RefreshBatchStatusReflectsRetryAndCompletion(t *testing.T) {
	db := setupCoverageV2TestDB(t, "batch_health")
	repo := NewCoverageV2Repository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	batch, err := repo.CreateBatch(ctx, &model.CoverageBatch{WorkspaceID: "ws-1", WindowStart: now.Add(-time.Hour), WindowEnd: now, AnalyzerVersion: "v4", PolicyVersion: "v1", Status: model.CoverageBatchQueued})
	if err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 2; index++ {
		attempt := &model.CoverageAnalysisAttempt{WorkspaceID: "ws-1", BatchID: batch.ID, LogicalWorkKey: fmt.Sprintf("work-%d", index), SourceKind: "conversation", SourceID: fmt.Sprintf("conversation-%d", index), ContentHash: "hash", AnalyzerVersion: "v4", PolicyVersion: "v1", Attempt: 1}
		if err := repo.EnqueueAttempt(ctx, attempt); err != nil {
			t.Fatal(err)
		}
	}
	claimed, err := repo.ClaimAttemptsForWorkspace(ctx, "ws-1", "worker", now, time.Minute, 10)
	if err != nil || len(claimed) != 2 {
		t.Fatalf("claim = %#v, %v", claimed, err)
	}
	if err := repo.CompleteAttempt(ctx, claimed[0].ID, "worker", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkRetryable(ctx, claimed[1].ID, "worker", model.CoverageFailureLLMProvider, "temporary", now.Add(time.Minute), 1); err != nil {
		t.Fatal(err)
	}
	if err := repo.RefreshBatchStatus(ctx, batch.ID); err != nil {
		t.Fatal(err)
	}
	var refreshed model.CoverageBatch
	if err := db.First(&refreshed, "id = ?", batch.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != model.CoverageBatchRunning || refreshed.SucceededCount != 1 || refreshed.RetryableCount != 1 {
		t.Fatalf("refreshed batch = %+v", refreshed)
	}
}

func TestCoverageV2RebuildApplyIsScopedIdempotentAndReversible(t *testing.T) {
	db := setupCoverageV2TestDB(t, "rebuild")
	repo := NewCoverageV2Repository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, row := range []struct{ id, workspaceID, status string }{{"gap-1", "ws-1", "open"}, {"gap-2", "ws-1", "done"}, {"gap-other", "ws-2", "open"}} {
		if err := db.Exec("INSERT INTO support_coverage_gaps (id, workspace_id, status, last_seen_at) VALUES (?, ?, ?, ?)", row.id, row.workspaceID, row.status, now).Error; err != nil {
			t.Fatal(err)
		}
	}
	conversationID := "conversation-1"
	if err := db.Exec("INSERT INTO support_gap_evidence (id, gap_id, workspace_id, evidence_type, conversation_id, excerpt, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "evidence-1", "gap-1", "ws-1", "conversation", conversationID, "customer question", now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO support_gap_evidence (id, gap_id, workspace_id, evidence_type, widget_session_id, excerpt, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "search-1", "gap-2", "ws-1", model.SupportEventWidgetSearchPerformed, "session-1", "reset account password", now).Error; err != nil {
		t.Fatal(err)
	}
	preview, err := repo.PreviewRebuild(ctx, "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if preview.LegacyGapCount != 2 || preview.EvidenceCount != 2 || len(preview.ConversationIDs) != 1 || len(preview.Searches) != 1 {
		t.Fatalf("preview = %+v", preview)
	}
	preview.Searches[0].MeaningfulTokens = 3
	applied, err := repo.ApplyRebuild(ctx, "ws-1", "v4", "v1", preview, preview.Searches)
	if err != nil {
		t.Fatal(err)
	}
	var archived, untouched int64
	db.Table("support_coverage_gaps").Where("workspace_id = ? AND status = ?", "ws-1", model.SupportCoverageGapStatusArchivedV1).Count(&archived)
	db.Table("support_coverage_gaps").Where("workspace_id = ? AND status = ?", "ws-2", model.SupportCoverageGapStatusOpen).Count(&untouched)
	if archived != 2 || untouched != 1 {
		t.Fatalf("archived/untouched = %d/%d", archived, untouched)
	}
	again, err := repo.ApplyRebuild(ctx, "ws-1", "v4", "v1", preview, preview.Searches)
	if err != nil || again.AuditID != applied.AuditID {
		t.Fatalf("idempotent apply = %+v, %v", again, err)
	}
	if err := repo.RollbackRebuild(ctx, "ws-1", applied.AuditID); err != nil {
		t.Fatal(err)
	}
	var open, done int64
	db.Table("support_coverage_gaps").Where("workspace_id = ? AND status = ?", "ws-1", model.SupportCoverageGapStatusOpen).Count(&open)
	db.Table("support_coverage_gaps").Where("workspace_id = ? AND status = ?", "ws-1", model.SupportCoverageGapStatusDone).Count(&done)
	if open != 1 || done != 1 {
		t.Fatalf("restored open/done = %d/%d", open, done)
	}
}

func TestCoverageV2RolloutStatsDetectSafetyViolations(t *testing.T) {
	db := setupCoverageV2TestDB(t, "rollout_stats")
	repo := NewCoverageV2Repository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	for attemptNumber := 1; attemptNumber <= 2; attemptNumber++ {
		if err := db.Exec("INSERT INTO coverage_analysis_attempts (id, workspace_id, batch_id, logical_work_key, source_kind, source_id, content_hash, analyzer_version, policy_version, attempt, status, stage, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", fmt.Sprintf("attempt-%d", attemptNumber), "ws-1", "batch-1", "duplicate-work", "conversation", "conversation-1", "hash", "v4", "v1", attemptNumber, model.CoverageAttemptSucceeded, model.CoverageFailureCandidateQuery, now).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("INSERT INTO coverage_findings (id, workspace_id, logical_work_key, analysis_attempt_id, source_kind, source_id, customer_need, fix_type, fix_target, rationale, suggested_change, confidence, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", "finding-foreign", "ws-2", "foreign-work", "attempt-1", "conversation", "conversation-1", "need", "update_article", "target", "why", "change", .9, now, now).Error; err != nil {
		t.Fatal(err)
	}
	stats, err := repo.CoverageRolloutStats(ctx, "ws-1", now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if stats.AttemptCount != 2 || stats.DuplicateIdempotencyViolations != 1 || stats.CrossWorkspaceInvariantViolations != 1 {
		t.Fatalf("rollout stats = %+v", stats)
	}
}

func TestCoverageV2ClaimsPendingAssignmentsOnce(t *testing.T) {
	db := setupCoverageV2TestDB(t, "assignment_claim")
	repo := NewCoverageV2Repository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	for index, status := range []string{"pending", "retryable", "assigned"} {
		finding := coverageFindingFixture(fmt.Sprintf("finding-%d", index), fmt.Sprintf("attempt-%d", index))
		finding.LogicalWorkKey = fmt.Sprintf("work-%d", index)
		finding.AssignmentStatus = status
		finding.CreatedAt = now.Add(time.Duration(index) * time.Second)
		if err := repo.ReplaceCurrentFinding(ctx, finding); err != nil {
			t.Fatal(err)
		}
	}
	claimed, err := repo.ClaimFindingsForAssignment(ctx, "ws-1", 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 2 {
		t.Fatalf("claimed assignments = %#v", claimed)
	}
	claimedAgain, err := repo.ClaimFindingsForAssignment(ctx, "ws-1", 25)
	if err != nil || len(claimedAgain) != 0 {
		t.Fatalf("duplicate claims = %#v, %v", claimedAgain, err)
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

func TestCoverageV2SignalReviewAndDismissAreWorkspaceScoped(t *testing.T) {
	db := setupCoverageV2TestDB(t, "signal_review")
	repo := NewCoverageV2Repository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := db.Exec("INSERT INTO coverage_topics (id, workspace_id, canonical_key, title, customer_need, assignment_policy) VALUES (?, ?, ?, ?, ?, ?)", "topic-1", "ws-1", "key", "Password reset", "Reset password", "v1").Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"signal-attach", "signal-dismiss"} {
		if err := repo.UpsertUnreviewedSignal(ctx, &model.CoverageUnreviewedSignal{ID: id, WorkspaceID: "ws-1", SourceKind: "widget_search", SourceID: id, SignalKey: id, NormalizedQuery: "reset account password", MeaningfulTokens: 3, Status: model.CoverageSignalUnreviewed, ObservedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.ReviewSignal(ctx, "ws-2", "signal-attach", "topic-1", "user-1"); err == nil {
		t.Fatal("expected cross-workspace review to fail")
	}
	if err := repo.ReviewSignal(ctx, "ws-1", "signal-attach", "topic-1", "user-1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.DismissSignal(ctx, "ws-1", "signal-dismiss", "user-1"); err != nil {
		t.Fatal(err)
	}
	var attached, dismissed int64
	db.Model(&model.CoverageUnreviewedSignal{}).Where("id = ? AND status = ?", "signal-attach", model.CoverageSignalAttached).Count(&attached)
	db.Model(&model.CoverageUnreviewedSignal{}).Where("id = ? AND status = ?", "signal-dismiss", model.CoverageSignalDismissed).Count(&dismissed)
	if attached != 1 || dismissed != 1 {
		t.Fatalf("attached/dismissed = %d/%d", attached, dismissed)
	}
}

func setupCoverageV2TestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:coverage_v2_"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE support_coverage_gaps (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, status TEXT NOT NULL, last_seen_at DATETIME NOT NULL, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE support_gap_evidence (id TEXT PRIMARY KEY, gap_id TEXT NOT NULL, workspace_id TEXT NOT NULL, evidence_type TEXT NOT NULL, conversation_id TEXT, message_id TEXT, widget_session_id TEXT, excerpt TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE coverage_batches (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, window_start DATETIME NOT NULL, window_end DATETIME NOT NULL, analyzer_version TEXT NOT NULL, policy_version TEXT NOT NULL, status TEXT NOT NULL, cursor TEXT NOT NULL DEFAULT '', lease_owner TEXT NOT NULL DEFAULT '', lease_expires_at DATETIME, candidate_count INTEGER NOT NULL DEFAULT 0, succeeded_count INTEGER NOT NULL DEFAULT 0, retryable_count INTEGER NOT NULL DEFAULT 0, dead_letter_count INTEGER NOT NULL DEFAULT 0, failure_class TEXT NOT NULL DEFAULT '', failure_message TEXT NOT NULL DEFAULT '', correlation_id TEXT NOT NULL DEFAULT '', metadata TEXT NOT NULL DEFAULT '{}', started_at DATETIME, completed_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(workspace_id, window_start, window_end, analyzer_version, policy_version))`,
		`CREATE TABLE coverage_analysis_attempts (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, batch_id TEXT NOT NULL, logical_work_key TEXT NOT NULL, source_kind TEXT NOT NULL, source_id TEXT NOT NULL, segment_id TEXT NOT NULL DEFAULT '', content_hash TEXT NOT NULL, analyzer_version TEXT NOT NULL, policy_version TEXT NOT NULL, attempt INTEGER NOT NULL, status TEXT NOT NULL, stage TEXT NOT NULL, lease_owner TEXT NOT NULL DEFAULT '', lease_expires_at DATETIME, retry_at DATETIME, retry_budget_used INTEGER NOT NULL DEFAULT 0, failure_class TEXT NOT NULL DEFAULT '', failure_message TEXT NOT NULL DEFAULT '', ai_execution_id TEXT, correlation_id TEXT NOT NULL DEFAULT '', metadata TEXT NOT NULL DEFAULT '{}', started_at DATETIME, completed_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(workspace_id, logical_work_key, attempt))`,
		`CREATE TABLE coverage_findings (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, logical_work_key TEXT NOT NULL, analysis_attempt_id TEXT NOT NULL, source_kind TEXT NOT NULL, source_id TEXT NOT NULL, conversation_id TEXT, customer_id TEXT, customer_need TEXT NOT NULL, ai_answer TEXT NOT NULL DEFAULT '', ai_failure TEXT NOT NULL DEFAULT '', human_answer TEXT NOT NULL DEFAULT '', fix_type TEXT NOT NULL, fix_target TEXT NOT NULL, rationale TEXT NOT NULL, suggested_change TEXT NOT NULL, confidence REAL NOT NULL, is_current INTEGER NOT NULL DEFAULT 1, embedding_status TEXT NOT NULL DEFAULT 'pending', assignment_status TEXT NOT NULL DEFAULT 'pending', embedding TEXT, embedding_provider TEXT NOT NULL DEFAULT '', embedding_model TEXT NOT NULL DEFAULT '', embedding_version TEXT NOT NULL DEFAULT '', embedding_dimensions INTEGER NOT NULL DEFAULT 0, metadata TEXT NOT NULL DEFAULT '{}', superseded_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE coverage_topics (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, canonical_key TEXT NOT NULL, title TEXT NOT NULL, customer_need TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'open', finding_count INTEGER NOT NULL DEFAULT 0, conversation_count INTEGER NOT NULL DEFAULT 0, customer_count INTEGER NOT NULL DEFAULT 0, assignment_policy TEXT NOT NULL, metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE coverage_topic_memberships (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, finding_id TEXT NOT NULL, topic_id TEXT NOT NULL, decision_source TEXT NOT NULL, confidence REAL NOT NULL, policy_version TEXT NOT NULL, assignment_attempt_id TEXT, actor_id TEXT, valid_from DATETIME NOT NULL, valid_to DATETIME, metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE coverage_unreviewed_signals (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, source_kind TEXT NOT NULL, source_id TEXT NOT NULL, session_id TEXT NOT NULL DEFAULT '', normalized_query TEXT NOT NULL, signal_key TEXT NOT NULL, meaningful_tokens INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, confidence REAL NOT NULL DEFAULT 0, finding_id TEXT, topic_id TEXT, reviewed_by TEXT, reviewed_at DATETIME, metadata TEXT NOT NULL DEFAULT '{}', observed_at DATETIME NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(workspace_id, signal_key))`,
		`CREATE TABLE coverage_rebuild_audits (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, status TEXT NOT NULL, analyzer_version TEXT NOT NULL, policy_version TEXT NOT NULL, legacy_gap_count INTEGER NOT NULL DEFAULT 0, evidence_count INTEGER NOT NULL DEFAULT 0, conversation_count INTEGER NOT NULL DEFAULT 0, search_count INTEGER NOT NULL DEFAULT 0, queued_work_count INTEGER NOT NULL DEFAULT 0, qualified_search_count INTEGER NOT NULL DEFAULT 0, legacy_status_snapshot TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, rolled_back_at DATETIME)`,
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
