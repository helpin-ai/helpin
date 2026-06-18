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
			embedding TEXT, embedding_provider TEXT NOT NULL DEFAULT '',
			embedding_model TEXT NOT NULL DEFAULT '', embedding_version TEXT NOT NULL DEFAULT '',
			embedding_dimensions INTEGER NOT NULL DEFAULT 0, embedding_text_hash TEXT NOT NULL DEFAULT '',
			embedding_updated_at DATETIME, nearest_content_score REAL NOT NULL DEFAULT 0,
			nearest_content_document_id TEXT, nearest_content_title TEXT NOT NULL DEFAULT '',
			nearest_content_checked_at DATETIME, impact_score REAL NOT NULL DEFAULT 0,
			created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE UNIQUE INDEX idx_support_coverage_gaps_workspace_topic_open
			ON support_coverage_gaps(workspace_id, topic_id)
			WHERE status = 'open' AND topic_id IS NOT NULL`,
		`CREATE TABLE support_gap_evidence (
			id TEXT PRIMARY KEY, gap_id TEXT NOT NULL, workspace_id TEXT NOT NULL,
			evidence_type TEXT NOT NULL, conversation_id TEXT, message_id TEXT,
			widget_session_id TEXT, document_id TEXT, article_public_id TEXT,
			source_signal TEXT NOT NULL DEFAULT '', source_key TEXT NOT NULL DEFAULT '',
			excerpt TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME
		)`,
		`CREATE TABLE support_messages (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, conversation_id TEXT,
			sender_type TEXT NOT NULL, message_type TEXT NOT NULL DEFAULT 'reply',
			content TEXT NOT NULL DEFAULT '', is_internal BOOLEAN NOT NULL DEFAULT 0,
			deleted_at DATETIME, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE support_conversations (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL,
			customer_email TEXT NOT NULL DEFAULT '', crm_contact_id TEXT, anonymous_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
		`CREATE TABLE support_coverage_recommendations (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, gap_id TEXT NOT NULL,
			analysis_id TEXT, recommendation_type TEXT NOT NULL,
			target_type TEXT NOT NULL DEFAULT '', target_id TEXT,
			target_title TEXT NOT NULL DEFAULT '', target_url TEXT NOT NULL DEFAULT '',
			priority TEXT NOT NULL DEFAULT 'secondary', status TEXT NOT NULL DEFAULT 'open',
			rationale TEXT NOT NULL DEFAULT '', suggested_change TEXT NOT NULL DEFAULT '',
			implementation_notes TEXT NOT NULL DEFAULT '', suggestion_id TEXT,
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
		`CREATE TABLE support_coverage_analysis_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			window_start DATETIME NOT NULL,
			window_end DATETIME NOT NULL,
			cursor_started_at DATETIME NOT NULL,
			cursor_ended_at DATETIME NOT NULL,
			analyzer_version TEXT NOT NULL DEFAULT 'v1',
			status TEXT NOT NULL DEFAULT 'running',
			conversation_cnt INTEGER NOT NULL DEFAULT 0,
			gap_count INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			metadata TEXT NOT NULL DEFAULT '{}',
			started_at DATETIME NOT NULL,
			completed_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_coverage_cluster_rebuild_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'running',
			gaps_scanned INTEGER NOT NULL DEFAULT 0,
			clusters_found INTEGER NOT NULL DEFAULT 0,
			auto_merged INTEGER NOT NULL DEFAULT 0,
			suggestions_created INTEGER NOT NULL DEFAULT 0,
			skipped INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME NOT NULL,
			completed_at DATETIME,
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_coverage_gap_merge_suggestions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT,
			source_gap_id TEXT NOT NULL,
			target_gap_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			similarity_score REAL NOT NULL DEFAULT 0,
			reason TEXT NOT NULL DEFAULT '',
			combined_evidence_count INTEGER NOT NULL DEFAULT 0,
			reviewed_by TEXT,
			reviewed_at DATETIME,
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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

func TestSupportCoverageImpactTier(t *testing.T) {
	tests := []struct {
		evidence30d int
		want        string
	}{
		{0, "low"},
		{2, "low"},
		{3, "medium"},
		{9, "medium"},
		{10, "high"},
		{100, "high"},
	}
	for _, tt := range tests {
		if got := ImpactTier(tt.evidence30d); got != tt.want {
			t.Fatalf("ImpactTier(%d)=%q, want %q", tt.evidence30d, got, tt.want)
		}
	}
}

func TestSupportCoverage_AddDocumentToGapClosesWithResultDocument(t *testing.T) {
	_, coverageSvc, db := setupCoverageTestEnv(t)
	ctx := context.Background()
	now := time.Now()
	repo := repository.NewSupportCoverageRepository(db)

	_, _, err := repo.UpsertGapByDedupeKey(ctx, &model.SupportCoverageGap{
		ID:          "gap-editor",
		WorkspaceID: "ws-1",
		DedupeKey:   "editor-handoff",
		FirstSeenAt: now,
		LastSeenAt:  now,
	})
	if err != nil {
		t.Fatalf("seed gap: %v", err)
	}
	if err := repo.CreateEvidence(ctx, &model.SupportGapEvidence{
		GapID:        "gap-editor",
		WorkspaceID:  "ws-1",
		EvidenceType: model.SupportEventDocsIssueFeedback,
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	if err := coverageSvc.AddDocumentToGap(ctx, "ws-1", "gap-editor", "doc-editor"); err != nil {
		t.Fatalf("AddDocumentToGap: %v", err)
	}

	var gap model.SupportCoverageGap
	if err := db.Where("id = ?", "gap-editor").First(&gap).Error; err != nil {
		t.Fatalf("load gap: %v", err)
	}
	if gap.Status != model.SupportCoverageGapStatusDone {
		t.Fatalf("status=%q, want done", gap.Status)
	}
	if gap.ResultDocumentID == nil || *gap.ResultDocumentID != "doc-editor" {
		t.Fatalf("result_document_id=%v, want doc-editor", gap.ResultDocumentID)
	}
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

	gaps, total, err := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap (deduped), got %d", len(gaps))
	}
	if gaps[0].EvidenceCount != 3 {
		t.Errorf("expected evidence_count=3, got %d", gaps[0].EvidenceCount)
	}
}

func TestSupportCoverage_SpikeTriggerStartsEnrichmentOnFifthEvidence(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	runner := &fakeCoverageWorkflowRunner{}
	coverageSvc.SetCoverageWorkflowRunner(runner)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := eventSvc.RecordEvent(ctx, SupportEventInput{
			WorkspaceID:  "ws-1",
			EventType:    model.SupportEventHumanReplyAfterAI,
			IssueSummary: "How do I reset my password?",
			SourceSignal: model.SupportCoverageSourceHumanReply,
		}); err != nil {
			t.Fatalf("RecordEvent %d: %v", i, err)
		}
	}

	if len(runner.topicIDs) != 1 {
		t.Fatalf("workflow starts=%d, want 1", len(runner.topicIDs))
	}
	if runner.topicIDs[0] == "" {
		t.Fatal("expected topic id")
	}
}

func TestSupportCoverage_SpikeTriggerWaitsUntilFifthEvidence(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	runner := &fakeCoverageWorkflowRunner{}
	coverageSvc.SetCoverageWorkflowRunner(runner)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		if err := eventSvc.RecordEvent(ctx, SupportEventInput{
			WorkspaceID:  "ws-1",
			EventType:    model.SupportEventHumanReplyAfterAI,
			IssueSummary: "How do I reset my password?",
			SourceSignal: model.SupportCoverageSourceHumanReply,
		}); err != nil {
			t.Fatalf("RecordEvent %d: %v", i, err)
		}
	}

	if len(runner.topicIDs) != 0 {
		t.Fatalf("workflow starts=%d, want 0", len(runner.topicIDs))
	}
}

func TestSupportCoverage_SpikeTriggerRespectsTopicCooldown(t *testing.T) {
	eventSvc, coverageSvc, db := setupCoverageTestEnv(t)
	runner := &fakeCoverageWorkflowRunner{}
	coverageSvc.SetCoverageWorkflowRunner(runner)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		if err := eventSvc.RecordEvent(ctx, SupportEventInput{
			WorkspaceID:  "ws-1",
			EventType:    model.SupportEventHumanReplyAfterAI,
			IssueSummary: "How do I reset my password?",
			SourceSignal: model.SupportCoverageSourceHumanReply,
		}); err != nil {
			t.Fatalf("RecordEvent %d: %v", i, err)
		}
	}
	var topic model.SupportCoverageTopic
	if err := db.First(&topic).Error; err != nil {
		t.Fatalf("load topic: %v", err)
	}
	cooldown := time.Now().Add(time.Hour)
	if err := db.Model(&model.SupportCoverageTopic{}).Where("id = ?", topic.ID).Update("cooldown_until", cooldown).Error; err != nil {
		t.Fatalf("set cooldown: %v", err)
	}
	if err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventHumanReplyAfterAI,
		IssueSummary: "How do I reset my password?",
		SourceSignal: model.SupportCoverageSourceHumanReply,
	}); err != nil {
		t.Fatalf("RecordEvent fifth: %v", err)
	}

	if len(runner.topicIDs) != 0 {
		t.Fatalf("workflow starts=%d, want 0 during cooldown", len(runner.topicIDs))
	}
}

func TestSupportCoverage_RegenerateGapEnqueuesUniqueWorkflow(t *testing.T) {
	eventSvc, coverageSvc, _ := setupCoverageTestEnv(t)
	runner := &fakeCoverageWorkflowRunner{}
	coverageSvc.SetCoverageWorkflowRunner(runner)
	ctx := context.Background()

	if err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventHumanReplyAfterAI,
		IssueSummary: "How do I reset my password?",
		SourceSignal: model.SupportCoverageSourceHumanReply,
	}); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	gaps, _, err := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
	if err != nil {
		t.Fatalf("ListGaps: %v", err)
	}

	if err := coverageSvc.RegenerateGap(ctx, "ws-1", gaps[0].ID); err != nil {
		t.Fatalf("RegenerateGap: %v", err)
	}

	if len(runner.topicIDs) != 1 {
		t.Fatalf("workflow starts=%d, want 1", len(runner.topicIDs))
	}
	if !runner.unique[0] {
		t.Fatal("manual regenerate should use a unique workflow id")
	}
}

func TestSupportCoverage_RegenerateGapRejectsLegacyGapWithoutTopic(t *testing.T) {
	_, coverageSvc, db := setupCoverageTestEnv(t)
	coverageSvc.SetCoverageWorkflowRunner(&fakeCoverageWorkflowRunner{})
	ctx := context.Background()
	now := time.Now()
	legacy := model.SupportCoverageGap{
		ID:            "legacy-gap",
		WorkspaceID:   "ws-1",
		DedupeKey:     "legacy",
		Status:        model.SupportCoverageGapStatusOpen,
		EvidenceCount: 1,
		Metadata:      []byte("{}"),
		FirstSeenAt:   now,
		LastSeenAt:    now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("seed legacy gap: %v", err)
	}

	if err := coverageSvc.RegenerateGap(ctx, "ws-1", "legacy-gap"); err == nil {
		t.Fatal("expected error for legacy gap without topic")
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

	gaps, total, err := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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
	gaps, total, err := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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
	gaps, total, err = coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

func TestSupportCoverage_HumanReplyAfterAI_ReplayedEventDoesNotDoubleCount(t *testing.T) {
	eventSvc, coverageSvc, db := setupCoverageTestEnv(t)
	ctx := context.Background()
	convID := "conv-hr-retry"

	if err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:    "ws-1",
		EventType:      model.SupportEventAIHandoffTriggered,
		ConversationID: &convID,
		IssueKey:       "login_issue",
		IssueSummary:   "Cannot log in with SSO",
		FailureMode:    model.SupportCoverageFailureNoRetrieval,
		SourceSignal:   model.SupportCoverageSourceAIHandoff,
	}); err != nil {
		t.Fatalf("RecordEvent (handoff): %v", err)
	}

	gaps, total, err := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
	if err != nil {
		t.Fatalf("ListGaps after handoff: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 gap after handoff, got %d", total)
	}
	gapID := gaps[0].ID
	messageID := "msg-human-retry"
	event := &model.SupportEvent{
		ID:             "event-human-retry",
		WorkspaceID:    "ws-1",
		EventType:      model.SupportEventHumanReplyAfterAI,
		ConversationID: &convID,
		MessageID:      &messageID,
		IssueSummary:   "Agent resolved: SSO cert was expired",
		SourceSignal:   model.SupportCoverageSourceHumanReply,
		OccurredAt:     time.Now(),
	}

	if err := coverageSvc.ProcessSupportEvent(ctx, event); err != nil {
		t.Fatalf("first ProcessSupportEvent: %v", err)
	}
	if err := coverageSvc.ProcessSupportEvent(ctx, event); err != nil {
		t.Fatalf("replayed ProcessSupportEvent: %v", err)
	}

	var evidenceCount int64
	if err := db.Model(&model.SupportGapEvidence{}).Where("gap_id = ?", gapID).Count(&evidenceCount).Error; err != nil {
		t.Fatalf("count evidence: %v", err)
	}
	if evidenceCount != 2 {
		t.Fatalf("evidence rows=%d, want 2", evidenceCount)
	}
	var gap model.SupportCoverageGap
	if err := db.First(&gap, "id = ?", gapID).Error; err != nil {
		t.Fatalf("load gap: %v", err)
	}
	if gap.EvidenceCount != 2 {
		t.Fatalf("evidence_count=%d, want 2", gap.EvidenceCount)
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

	gaps, total, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
	if total != 1 {
		t.Fatalf("expected 1 new gap for orphan human reply, got %d", total)
	}
	if gaps[0].V1GapType != model.SupportCoverageV1GapNeedsReview {
		t.Errorf("expected needs_review, got %q", gaps[0].V1GapType)
	}
}

func TestSupportCoverage_HumanReplyAfterAI_CompletedAnalysisRunSkipsNewV1Gap(t *testing.T) {
	eventSvc, coverageSvc, db := setupCoverageTestEnv(t)
	ctx := context.Background()
	convID := "conv-analyzed-human-reply"
	now := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO support_coverage_analysis_runs (
		id, workspace_id, window_start, window_end, cursor_started_at, cursor_ended_at,
		analyzer_version, status, started_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-completed", "ws-1", now.Add(-24*time.Hour), now, now.Add(-24*time.Hour), now,
		"v3", model.SupportCoverageAnalysisRunStatusCompleted, now.Add(-time.Hour), now).Error; err != nil {
		t.Fatalf("seed completed run: %v", err)
	}

	err := eventSvc.RecordEvent(ctx, SupportEventInput{
		WorkspaceID:    "ws-1",
		EventType:      model.SupportEventHumanReplyAfterAI,
		ConversationID: &convID,
		IssueSummary:   "Agent helped with billing question",
		SourceSignal:   model.SupportCoverageSourceHumanReply,
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	_, total, err := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
	if err != nil {
		t.Fatalf("ListGaps: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected no v1 gap after completed analysis run, got %d", total)
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

	gaps, total, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

	_, total, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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
	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

	_, total, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
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

	gaps, _, _ := coverageSvc.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap from async recorder, got %d", len(gaps))
	}
}

type fakeCoverageWorkflowRunner struct {
	topicIDs []string
	unique   []bool
}

func (f *fakeCoverageWorkflowRunner) StartEnrichment(ctx context.Context, topicID string, unique bool) error {
	f.topicIDs = append(f.topicIDs, topicID)
	f.unique = append(f.unique, unique)
	return nil
}

func (f *fakeCoverageWorkflowRunner) StartDailyBatch(ctx context.Context) error {
	return nil
}
