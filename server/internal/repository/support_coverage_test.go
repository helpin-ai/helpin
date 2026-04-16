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

func setupSupportCoverageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:support_coverage_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	tables := []string{
		`CREATE TABLE support_events (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			conversation_id TEXT,
			message_id TEXT,
			widget_session_id TEXT,
			anonymous_id TEXT,
			document_id TEXT,
			article_id TEXT,
			article_public_id TEXT,
			actor_type TEXT NOT NULL DEFAULT '',
			channel TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			issue_key TEXT NOT NULL DEFAULT '',
			issue_summary TEXT NOT NULL DEFAULT '',
			failure_mode TEXT NOT NULL DEFAULT '',
			source_signal TEXT NOT NULL DEFAULT '',
			can_answer TEXT,
			can_resolve TEXT,
			metadata TEXT NOT NULL DEFAULT '{}',
			occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_coverage_topics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			issue_key TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			gap_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(workspace_id, issue_key)
		)`,
		`CREATE TABLE support_coverage_gaps (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			topic_id TEXT,
			dedupe_key TEXT NOT NULL,
			gap_category TEXT NOT NULL DEFAULT 'unknown',
			v1_gap_type TEXT NOT NULL DEFAULT 'needs_review',
			title TEXT NOT NULL DEFAULT '',
			issue_key TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'open',
			confidence REAL NOT NULL DEFAULT 0,
			evidence_count INTEGER NOT NULL DEFAULT 0,
			failure_mode TEXT NOT NULL DEFAULT '',
			source_signal TEXT NOT NULL DEFAULT '',
			can_answer TEXT,
			can_resolve TEXT,
			metadata TEXT NOT NULL DEFAULT '{}',
			first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			status_changed_by TEXT,
			status_changed_at DATETIME,
			issue_resolved BOOLEAN,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_gap_evidence (
			id TEXT PRIMARY KEY,
			gap_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			evidence_type TEXT NOT NULL,
			conversation_id TEXT,
			message_id TEXT,
			widget_session_id TEXT,
			document_id TEXT,
			article_public_id TEXT,
			source_signal TEXT NOT NULL DEFAULT '',
			excerpt TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_gap_suggestions (
			id TEXT PRIMARY KEY,
			gap_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			suggestion_type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			title TEXT NOT NULL DEFAULT '',
			content TEXT,
			evidence_summary TEXT NOT NULL DEFAULT '',
			target_space_id TEXT,
			target_collection_id TEXT,
			target_document_id TEXT,
			result_document_id TEXT,
			result_article_id TEXT,
			applied_at DATETIME,
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_coverage_gap_articles (
			id TEXT PRIMARY KEY,
			gap_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(gap_id, document_id)
		)`,
		`CREATE TABLE support_coverage_snapshots (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			snapshot_at DATETIME NOT NULL,
			metrics TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_coverage_digest_deliveries (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			week_start DATETIME NOT NULL,
			recipient_user_id TEXT NOT NULL,
			sent_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(workspace_id, week_start, recipient_user_id)
		)`,
	}
	for _, stmt := range tables {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	return db
}

func TestSupportCoverageRepository_UpsertTopic(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()

	topic1, err := repo.UpsertTopicByIssueKey(ctx, "ws-1", "billing_refund", "Billing Refund")
	if err != nil {
		t.Fatalf("UpsertTopicByIssueKey: %v", err)
	}
	if topic1.ID == "" {
		t.Fatal("expected topic ID")
	}

	// Same key returns same topic.
	topic2, err := repo.UpsertTopicByIssueKey(ctx, "ws-1", "billing_refund", "Billing Refund Updated")
	if err != nil {
		t.Fatalf("UpsertTopicByIssueKey second: %v", err)
	}
	if topic2.ID != topic1.ID {
		t.Errorf("expected same topic ID, got %q vs %q", topic1.ID, topic2.ID)
	}

	// Different workspace creates new topic.
	topic3, err := repo.UpsertTopicByIssueKey(ctx, "ws-2", "billing_refund", "Billing Refund")
	if err != nil {
		t.Fatalf("UpsertTopicByIssueKey ws-2: %v", err)
	}
	if topic3.ID == topic1.ID {
		t.Error("different workspace should create different topic")
	}
}

func TestSupportCoverageRepository_UpsertGap(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	gap := &model.SupportCoverageGap{
		ID:          "gap-1",
		WorkspaceID: "ws-1",
		DedupeKey:   "missing_article:billing_refund",
		GapCategory: model.SupportCoverageGapCategoryKnowledge,
		V1GapType:   model.SupportCoverageV1GapMissingArticle,
		Title:       "Billing Refund",
		IssueKey:    "billing_refund",
		Status:      model.SupportCoverageGapStatusOpen,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}

	created, isNew, err := repo.UpsertGapByDedupeKey(ctx, gap)
	if err != nil {
		t.Fatalf("UpsertGapByDedupeKey: %v", err)
	}
	if !isNew {
		t.Error("expected new gap")
	}
	if created.EvidenceCount != 1 {
		t.Errorf("expected evidence_count=1, got %d", created.EvidenceCount)
	}

	// Second upsert increments evidence count.
	gap2 := &model.SupportCoverageGap{
		WorkspaceID: "ws-1",
		DedupeKey:   "missing_article:billing_refund",
		LastSeenAt:  now.Add(time.Hour),
	}
	updated, isNew2, err := repo.UpsertGapByDedupeKey(ctx, gap2)
	if err != nil {
		t.Fatalf("UpsertGapByDedupeKey second: %v", err)
	}
	if isNew2 {
		t.Error("expected existing gap")
	}
	if updated.EvidenceCount != 2 {
		t.Errorf("expected evidence_count=2, got %d", updated.EvidenceCount)
	}
}

func TestSupportCoverageRepository_CreateEvidence(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()

	evidence := &model.SupportGapEvidence{
		ID:           "ev-1",
		GapID:        "gap-1",
		WorkspaceID:  "ws-1",
		EvidenceType: "ai_handoff",
		SourceSignal: model.SupportCoverageSourceAIHandoff,
		Excerpt:      "How do I get a refund?",
		CreatedAt:    time.Now(),
	}
	if err := repo.CreateEvidence(ctx, evidence); err != nil {
		t.Fatalf("CreateEvidence: %v", err)
	}
}

func TestSupportCoverageRepository_CreateSuggestion(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()

	suggestion := &model.SupportGapSuggestion{
		ID:             "sug-1",
		GapID:          "gap-1",
		WorkspaceID:    "ws-1",
		SuggestionType: model.SupportCoverageSuggestionCreateArticle,
		Title:          "Refund Policy",
	}
	created, err := repo.CreateSuggestion(ctx, suggestion)
	if err != nil {
		t.Fatalf("CreateSuggestion: %v", err)
	}
	if created.Status != model.SupportCoverageSuggestionStatusDraft {
		t.Errorf("expected draft status, got %q", created.Status)
	}
}

func TestSupportCoverageRepository_UpdateGapStatus(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()

	repo.UpsertGapByDedupeKey(ctx, &model.SupportCoverageGap{
		ID: "gap-1", WorkspaceID: "ws-1", DedupeKey: "test",
		FirstSeenAt: time.Now(), LastSeenAt: time.Now(),
	})

	if err := repo.UpdateGapStatus(ctx, "ws-1", "gap-1", model.SupportCoverageGapStatusIgnored, "user-1", nil); err != nil {
		t.Fatalf("UpdateGapStatus: %v", err)
	}

	// Wrong workspace returns error.
	if err := repo.UpdateGapStatus(ctx, "ws-other", "gap-1", model.SupportCoverageGapStatusFixed, "user-1", nil); err == nil {
		t.Error("expected error for wrong workspace")
	}
}

func TestSupportCoverageRepository_MergeGaps(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	repo.UpsertGapByDedupeKey(ctx, &model.SupportCoverageGap{
		ID: "gap-src", WorkspaceID: "ws-1", DedupeKey: "src",
		FirstSeenAt: now, LastSeenAt: now,
	})
	repo.UpsertGapByDedupeKey(ctx, &model.SupportCoverageGap{
		ID: "gap-tgt", WorkspaceID: "ws-1", DedupeKey: "tgt",
		FirstSeenAt: now, LastSeenAt: now,
	})
	repo.CreateEvidence(ctx, &model.SupportGapEvidence{
		ID: "ev-1", GapID: "gap-src", WorkspaceID: "ws-1",
		EvidenceType: "test", CreatedAt: now,
	})

	if err := repo.MergeGaps(ctx, "ws-1", "gap-src", "gap-tgt"); err != nil {
		t.Fatalf("MergeGaps: %v", err)
	}

	// Source should be merged.
	var src model.SupportCoverageGap
	db.Where("id = ?", "gap-src").First(&src)
	if src.Status != model.SupportCoverageGapStatusMerged {
		t.Errorf("expected merged status, got %q", src.Status)
	}

	// Evidence should be moved to target.
	var evidence []model.SupportGapEvidence
	db.Where("gap_id = ?", "gap-tgt").Find(&evidence)
	if len(evidence) != 1 {
		t.Errorf("expected 1 evidence on target, got %d", len(evidence))
	}
}

func TestSupportCoverageRepository_DigestDelivery(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	weekStart := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)

	delivery := &model.SupportCoverageDigestDelivery{
		ID:              "del-1",
		WorkspaceID:     "ws-1",
		WeekStart:       weekStart,
		RecipientUserID: "user-1",
		SentAt:          time.Now(),
	}
	if err := repo.CreateDigestDelivery(ctx, delivery); err != nil {
		t.Fatalf("CreateDigestDelivery: %v", err)
	}

	found, err := repo.GetDigestDelivery(ctx, "ws-1", weekStart, "user-1")
	if err != nil {
		t.Fatalf("GetDigestDelivery: %v", err)
	}
	if found == nil {
		t.Fatal("expected delivery record")
	}

	// Duplicate insert should not error (ON CONFLICT DO NOTHING).
	delivery2 := &model.SupportCoverageDigestDelivery{
		ID:              "del-2",
		WorkspaceID:     "ws-1",
		WeekStart:       weekStart,
		RecipientUserID: "user-1",
		SentAt:          time.Now(),
	}
	if err := repo.CreateDigestDelivery(ctx, delivery2); err != nil {
		t.Fatalf("duplicate CreateDigestDelivery should not error: %v", err)
	}

	// Different user should not be found.
	notFound, _ := repo.GetDigestDelivery(ctx, "ws-1", weekStart, "user-2")
	if notFound != nil {
		t.Error("expected nil for different user")
	}
}
