package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCoverageTestEnv(t *testing.T) (*SupportEventService, *SupportCoverageService, *gorm.DB) {
	t.Helper()
	dbName := fmt.Sprintf("file:svc_coverage_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	tables := []string{
		`CREATE TABLE support_events (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, event_type TEXT NOT NULL,
			conversation_id TEXT, message_id TEXT, widget_session_id TEXT, anonymous_id TEXT,
			document_id TEXT, article_id TEXT, article_public_id TEXT,
			actor_type TEXT NOT NULL DEFAULT '', channel TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '', issue_key TEXT NOT NULL DEFAULT '',
			issue_summary TEXT NOT NULL DEFAULT '', failure_mode TEXT NOT NULL DEFAULT '',
			source_signal TEXT NOT NULL DEFAULT '', can_answer TEXT, can_resolve TEXT,
			metadata TEXT NOT NULL DEFAULT '{}',
			occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_coverage_topics (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, issue_key TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '', gap_count INTEGER NOT NULL DEFAULT 0,
			cluster_key TEXT, canonical_title TEXT, last_enriched_at DATETIME,
			cooldown_until DATETIME, created_at DATETIME, updated_at DATETIME,
			UNIQUE(workspace_id, issue_key)
		)`,
		`CREATE UNIQUE INDEX idx_support_coverage_topics_workspace_cluster_key
			ON support_coverage_topics(workspace_id, cluster_key)
			WHERE cluster_key IS NOT NULL`,
		`CREATE TABLE support_coverage_gaps (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, topic_id TEXT,
			dedupe_key TEXT NOT NULL, gap_kind TEXT NOT NULL DEFAULT 'content',
			gap_category TEXT NOT NULL DEFAULT 'unknown',
			v1_gap_type TEXT NOT NULL DEFAULT 'needs_review', title TEXT NOT NULL DEFAULT '',
			issue_key TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'open',
			confidence REAL NOT NULL DEFAULT 0, evidence_count INTEGER NOT NULL DEFAULT 0,
			failure_mode TEXT NOT NULL DEFAULT '', source_signal TEXT NOT NULL DEFAULT '',
			can_answer TEXT, can_resolve TEXT, metadata TEXT NOT NULL DEFAULT '{}',
			first_seen_at DATETIME, last_seen_at DATETIME,
			status_changed_by TEXT, status_changed_at DATETIME, issue_resolved BOOLEAN,
			closed_at DATETIME, closed_evidence_count INTEGER, result_document_id TEXT,
			rejection_reason TEXT,
			created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE UNIQUE INDEX idx_support_coverage_gaps_workspace_topic_open
			ON support_coverage_gaps(workspace_id, topic_id)
			WHERE status = 'open' AND topic_id IS NOT NULL`,
		`CREATE TABLE support_gap_evidence (
			id TEXT PRIMARY KEY, gap_id TEXT NOT NULL, workspace_id TEXT NOT NULL,
			evidence_type TEXT NOT NULL, conversation_id TEXT, message_id TEXT,
			widget_session_id TEXT, document_id TEXT, article_public_id TEXT,
			source_signal TEXT NOT NULL DEFAULT '', excerpt TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME
		)`,
		`CREATE TABLE support_gap_suggestions (
			id TEXT PRIMARY KEY, gap_id TEXT NOT NULL, workspace_id TEXT NOT NULL,
			suggestion_type TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'draft',
			title TEXT NOT NULL DEFAULT '', content TEXT, evidence_summary TEXT NOT NULL DEFAULT '',
			target_space_id TEXT, target_collection_id TEXT, target_document_id TEXT,
			result_document_id TEXT, result_article_id TEXT, applied_at DATETIME,
			is_active BOOLEAN NOT NULL DEFAULT 1, superseded_at DATETIME,
			metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE support_coverage_gap_articles (
			id TEXT PRIMARY KEY, gap_id TEXT NOT NULL, document_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL, created_at DATETIME, UNIQUE(gap_id, document_id)
		)`,
		`CREATE TABLE support_coverage_snapshots (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, snapshot_at DATETIME NOT NULL,
			metrics TEXT NOT NULL DEFAULT '{}', created_at DATETIME
		)`,
		`CREATE TABLE support_coverage_digest_deliveries (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, week_start DATETIME NOT NULL,
			recipient_user_id TEXT NOT NULL, sent_at DATETIME NOT NULL, created_at DATETIME,
			UNIQUE(workspace_id, week_start, recipient_user_id)
		)`,
	}
	for _, stmt := range tables {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	eventRepo := repository.NewSupportEventRepository(db)
	coverageRepo := repository.NewSupportCoverageRepository(db)
	coverageSvc := NewSupportCoverageService(coverageRepo)
	eventSvc := NewSupportEventService(eventRepo, coverageSvc)
	return eventSvc, coverageSvc, db
}

func TestSupportCoverage_AIHandoff_NoRetrieval_CreatesGap(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()

	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventAIHandoffTriggered,
		IssueKey:     "billing_refund",
		IssueSummary: "How do I get a refund?",
		FailureMode:  model.SupportCoverageFailureNoRetrieval,
		SourceSignal: model.SupportCoverageSourceAIHandoff,
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	gaps, total, err := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if err != nil {
		t.Fatalf("ListGaps: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 gap, got %d", total)
	}
	if gaps[0].V1GapType != model.SupportCoverageV1GapMissingArticle {
		t.Errorf("expected missing_article, got %q", gaps[0].V1GapType)
	}
	if gaps[0].EvidenceCount != 1 {
		t.Errorf("expected evidence_count=1, got %d", gaps[0].EvidenceCount)
	}
}

func TestSupportCoverage_AIHandoff_WeakRetrieval_CreatesWeakArticleGap(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()
	docID := "doc-123"

	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID: "ws-1",
		EventType:   model.SupportEventAIHandoffTriggered,
		IssueKey:    "sso_setup",
		FailureMode: model.SupportCoverageFailureWeakRetrieval,
		DocumentID:  &docID,
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].V1GapType != model.SupportCoverageV1GapWeakArticle {
		t.Errorf("expected weak_article, got %q", gaps[0].V1GapType)
	}
}

func TestSupportCoverage_ArticleFeedback_CreatesWeakGap(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()
	docID := "doc-456"

	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventArticleFeedback,
		IssueKey:     "password_reset",
		DocumentID:   &docID,
		SourceSignal: "not_helpful",
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].V1GapType != model.SupportCoverageV1GapWeakArticle {
		t.Errorf("expected weak_article, got %q", gaps[0].V1GapType)
	}
}

func TestSupportCoverage_WidgetSearch_NoResults_CreatesGap(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()

	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventWidgetSearchPerformed,
		IssueKey:     "cancel_subscription",
		IssueSummary: "cancel subscription",
		SourceSignal: "no_results",
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].V1GapType != model.SupportCoverageV1GapMissingArticle {
		t.Errorf("expected missing_article, got %q", gaps[0].V1GapType)
	}
}

func TestSupportCoverage_WidgetSearch_NoResults_NoIssueKey_NeedsReview(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()

	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventWidgetSearchPerformed,
		IssueSummary: "something obscure",
		SourceSignal: "no_results",
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].V1GapType != model.SupportCoverageV1GapNeedsReview {
		t.Errorf("expected needs_review, got %q", gaps[0].V1GapType)
	}
}

func TestSupportCoverage_DocsIssueFeedback_CreatesNeedsReview(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()

	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventDocsIssueFeedback,
		IssueKey:     "import_failed",
		SourceSignal: model.SupportCoverageSourceAgentFeedback,
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].V1GapType != model.SupportCoverageV1GapNeedsReview {
		t.Errorf("expected needs_review, got %q", gaps[0].V1GapType)
	}
}

func TestSupportCoverage_DuplicateEvents_IncrementEvidence(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		eventSvc.RecordEvent(ctx, SupportEventInput{
			WorkspaceID: "ws-1",
			EventType:   model.SupportEventAIHandoffTriggered,
			IssueKey:    "billing_refund",
			FailureMode: model.SupportCoverageFailureNoRetrieval,
		})
	}

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap (deduped), got %d", len(gaps))
	}
	if gaps[0].EvidenceCount != 3 {
		t.Errorf("expected evidence_count=3, got %d", gaps[0].EvidenceCount)
	}
}

func TestSupportCoverage_NormalizedVariantsShareCluster(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()

	inputs := []string{
		"How do I reset my password?",
		"how to reset password",
	}
	for _, summary := range inputs {
		if err := eventSvc.RecordEvent(ctx, SupportEventInput{
			WorkspaceID:  "ws-1",
			EventType:    model.SupportEventWidgetSearchPerformed,
			IssueSummary: summary,
			SourceSignal: "no_results",
		}); err != nil {
			t.Fatalf("RecordEvent(%q): %v", summary, err)
		}
	}

	gaps, total, err := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if err != nil {
		t.Fatalf("ListGaps: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected normalized variants to share one gap, got %d", total)
	}
	if gaps[0].EvidenceCount != 2 {
		t.Errorf("evidence_count=%d, want 2", gaps[0].EvidenceCount)
	}
}

func TestSupportCoverage_NoIssueKey_NoRetrieval_NeedsReview(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()

	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventAIHandoffTriggered,
		IssueSummary: "something random",
		FailureMode:  model.SupportCoverageFailureNoRetrieval,
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].V1GapType != model.SupportCoverageV1GapNeedsReview {
		t.Errorf("expected needs_review without issue key, got %q", gaps[0].V1GapType)
	}
}

func TestSupportCoverage_HumanReplyAfterAI_AttachesToExistingGap(t *testing.T) {
	eventSvc, coverageSvc, db := setupCoverageTestEnv(t)
	ctx := context.Background()
	convID := "conv-hr-1"

	// Step 1: AI handoff creates the initial gap with evidence for this conversation.
	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:    "ws-1",
		EventType:      model.SupportEventAIHandoffTriggered,
		ConversationID: &convID,
		IssueKey:       "login_issue",
		IssueSummary:   "Cannot log in with SSO",
		FailureMode:    model.SupportCoverageFailureNoRetrieval,
		SourceSignal:   model.SupportCoverageSourceAIHandoff,
	})
	if err != nil {
		t.Fatalf("RecordEvent (handoff): %v", err)
	}

	// Verify: 1 gap, 1 evidence row.
	gaps, total, err := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if err != nil {
		t.Fatalf("ListGaps after handoff: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 gap after handoff, got %d", total)
	}
	gapID := gaps[0].ID

	// Step 2: Human reply on the same conversation should attach evidence,
	// NOT create a second gap.
	msgID := "msg-hr-1"
	err = eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:    "ws-1",
		EventType:      model.SupportEventHumanReplyAfterAI,
		ConversationID: &convID,
		MessageID:      &msgID,
		IssueKey:       "login_issue",
		IssueSummary:   "Agent resolved: SSO cert was expired",
		SourceSignal:   model.SupportCoverageSourceHumanReply,
	})
	if err != nil {
		t.Fatalf("RecordEvent (human reply): %v", err)
	}

	// Verify: still 1 gap (not 2).
	gaps, total, err = coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if err != nil {
		t.Fatalf("ListGaps after human reply: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 gap after human reply (attached to existing), got %d", total)
	}
	if gaps[0].ID != gapID {
		t.Errorf("gap ID changed: was %q, now %q", gapID, gaps[0].ID)
	}

	// Verify: 2 evidence rows on that gap.
	var evidenceCount int64
	db.Table("support_gap_evidence").Where("gap_id = ?", gapID).Count(&evidenceCount)
	if evidenceCount != 2 {
		t.Errorf("expected 2 evidence rows on gap %q, got %d", gapID, evidenceCount)
	}

	// Verify the second evidence row is the human reply type.
	var humanEvidence model.SupportGapEvidence
	db.Table("support_gap_evidence").
		Where("gap_id = ? AND evidence_type = ?", gapID, model.SupportEventHumanReplyAfterAI).
		First(&humanEvidence)
	if humanEvidence.ID == "" {
		t.Error("expected human_reply_after_ai evidence row, not found")
	}
	if humanEvidence.ConversationID == nil || *humanEvidence.ConversationID != convID {
		t.Errorf("human reply evidence conversation_id mismatch")
	}
}

func TestSupportCoverage_HumanReplyAfterAI_NoExistingGap_CreatesNew(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()
	convID := "conv-hr-orphan"

	// Human reply with no prior gap for this conversation — should create a new gap.
	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:    "ws-1",
		EventType:      model.SupportEventHumanReplyAfterAI,
		ConversationID: &convID,
		IssueKey:       "orphan_topic",
		IssueSummary:   "Agent helped with billing question",
		SourceSignal:   model.SupportCoverageSourceHumanReply,
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	gaps, total, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if total != 1 {
		t.Fatalf("expected 1 new gap for orphan human reply, got %d", total)
	}
	if gaps[0].V1GapType != model.SupportCoverageV1GapNeedsReview {
		t.Errorf("expected needs_review, got %q", gaps[0].V1GapType)
	}
}

func TestSupportCoverage_ConversationResolvedByHuman_CreatesGap(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()
	convID := "conv-resolved-1"

	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:    "ws-1",
		EventType:      model.SupportEventConversationResolved,
		ConversationID: &convID,
		IssueSummary:   "I cannot reset my password — the link in the email is broken",
		SourceSignal:   model.SupportCoverageSourceConversationResolvedByHuman,
		ActorType:      model.SupportEventActorAgent,
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	gaps, total, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if total != 1 {
		t.Fatalf("expected 1 gap for human-resolved conversation, got %d", total)
	}
	gap := gaps[0]
	if gap.V1GapType != model.SupportCoverageV1GapNeedsReview {
		t.Errorf("expected needs_review, got %q", gap.V1GapType)
	}
	if gap.SourceSignal != model.SupportCoverageSourceConversationResolvedByHuman {
		t.Errorf("expected source_signal %q, got %q",
			model.SupportCoverageSourceConversationResolvedByHuman, gap.SourceSignal)
	}
}

func TestSupportCoverage_ConversationResolved_WithoutSignal_NoGap(t *testing.T) {
	// Bare conversation_resolved events (AI-resolved or human-only-no-AI)
	// must not produce a gap. Only the explicit
	// conversation_resolved_by_human source signal qualifies.
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()
	convID := "conv-resolved-nosignal"

	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:    "ws-1",
		EventType:      model.SupportEventConversationResolved,
		ConversationID: &convID,
		ActorType:      model.SupportEventActorAgent,
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	_, total, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if total != 0 {
		t.Fatalf("expected no gap for untagged conversation_resolved event, got %d", total)
	}
}

func TestSupportCoverage_ConversationResolvedByHuman_AttachesToExistingGap(t *testing.T) {
	eventSvc, coverageSvc, db := setupCoverageTestEnv(t)
	ctx := context.Background()
	convID := "conv-resolved-merge"

	// Seed: AI handoff created a gap earlier in the conversation lifetime.
	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:    "ws-1",
		EventType:      model.SupportEventAIHandoffTriggered,
		ConversationID: &convID,
		IssueKey:       "password_reset",
		IssueSummary:   "Password reset link broken",
		FailureMode:    model.SupportCoverageFailureNoRetrieval,
		SourceSignal:   model.SupportCoverageSourceAIHandoff,
	})
	if err != nil {
		t.Fatalf("RecordEvent (handoff): %v", err)
	}
	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if len(gaps) != 1 {
		t.Fatalf("setup: expected 1 gap after handoff, got %d", len(gaps))
	}
	gapID := gaps[0].ID

	// Human resolves the same conversation — should attach evidence, not create a second gap.
	err = eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:    "ws-1",
		EventType:      model.SupportEventConversationResolved,
		ConversationID: &convID,
		IssueSummary:   "Password reset link broken",
		SourceSignal:   model.SupportCoverageSourceConversationResolvedByHuman,
		ActorType:      model.SupportEventActorAgent,
	})
	if err != nil {
		t.Fatalf("RecordEvent (resolved): %v", err)
	}

	_, total, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if total != 1 {
		t.Fatalf("expected 1 gap after human resolution (attached to existing), got %d", total)
	}

	var evidenceCount int64
	db.Table("support_gap_evidence").Where("gap_id = ?", gapID).Count(&evidenceCount)
	if evidenceCount != 2 {
		t.Errorf("expected 2 evidence rows on gap %q, got %d", gapID, evidenceCount)
	}
}

func TestSupportCoverage_AsyncRecorder_NilSafe(t *testing.T) {
	// Nil recorder should not panic.
	var recorder *SupportEventAsyncRecorder
	recorder.RecordEventBestEffort(SupportEventInput{
		WorkspaceID: "ws-1",
		EventType:   model.SupportEventAIHandoffTriggered,
	})
}

func TestSupportCoverage_AsyncRecorder_Queues(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	ctx := context.Background()

	recorder := NewSupportEventAsyncRecorder(eventSvc, 10)

	recorder.RecordEventBestEffort(SupportEventInput{
		WorkspaceID: "ws-1",
		EventType:   model.SupportEventAIHandoffTriggered,
		IssueKey:    "async_test",
		FailureMode: model.SupportCoverageFailureNoRetrieval,
	})

	// Wait for async worker to drain and stop.
	recorder.Close()

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap from async recorder, got %d", len(gaps))
	}
}
