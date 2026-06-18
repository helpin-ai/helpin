package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
			cluster_key TEXT,
			canonical_title TEXT,
			last_enriched_at DATETIME,
			cooldown_until DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(workspace_id, issue_key)
		)`,
		`CREATE TABLE support_coverage_gaps (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			topic_id TEXT,
			dedupe_key TEXT NOT NULL,
			gap_kind TEXT NOT NULL DEFAULT 'content',
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
			closed_at DATETIME,
			closed_evidence_count INTEGER,
			result_document_id TEXT,
			rejection_reason TEXT,
			embedding TEXT,
			embedding_provider TEXT NOT NULL DEFAULT '',
			embedding_model TEXT NOT NULL DEFAULT '',
			embedding_version TEXT NOT NULL DEFAULT '',
			embedding_dimensions INTEGER NOT NULL DEFAULT 0,
			embedding_text_hash TEXT NOT NULL DEFAULT '',
			embedding_updated_at DATETIME,
			nearest_content_score REAL NOT NULL DEFAULT 0,
			nearest_content_document_id TEXT,
			nearest_content_title TEXT NOT NULL DEFAULT '',
			nearest_content_checked_at DATETIME,
			impact_score REAL NOT NULL DEFAULT 0,
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
			source_key TEXT NOT NULL DEFAULT '',
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
			is_active BOOLEAN NOT NULL DEFAULT 1,
			superseded_at DATETIME,
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_coverage_recommendations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			gap_id TEXT NOT NULL,
			analysis_id TEXT,
			recommendation_type TEXT NOT NULL,
			target_type TEXT NOT NULL DEFAULT '',
			target_id TEXT,
			target_title TEXT NOT NULL DEFAULT '',
			target_url TEXT NOT NULL DEFAULT '',
			priority TEXT NOT NULL DEFAULT 'secondary',
			status TEXT NOT NULL DEFAULT 'open',
			rationale TEXT NOT NULL DEFAULT '',
			suggested_change TEXT NOT NULL DEFAULT '',
			implementation_notes TEXT NOT NULL DEFAULT '',
			suggestion_id TEXT,
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
		`CREATE TABLE support_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			conversation_id TEXT,
			sender_type TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_conversations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			customer_email TEXT NOT NULL DEFAULT '',
			crm_contact_id TEXT,
			anonymous_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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

func TestSupportCoverageRepository_UpsertOpenGapByDedupeKeyNoBump(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	gap := &model.SupportCoverageGap{
		ID:          "gap-no-bump",
		WorkspaceID: "ws-1",
		DedupeKey:   "semantic:refunds",
		Title:       "Refund policy missing",
		Status:      model.SupportCoverageGapStatusOpen,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}

	created, isNew, err := repo.UpsertOpenGapByDedupeKeyNoBump(ctx, gap)
	if err != nil {
		t.Fatalf("UpsertOpenGapByDedupeKeyNoBump: %v", err)
	}
	if !isNew {
		t.Fatal("expected new gap")
	}
	if created.EvidenceCount != 0 {
		t.Fatalf("new gap evidence_count=%d, want 0", created.EvidenceCount)
	}

	again, isNew, err := repo.UpsertOpenGapByDedupeKeyNoBump(ctx, &model.SupportCoverageGap{
		WorkspaceID: "ws-1",
		DedupeKey:   "semantic:refunds",
		LastSeenAt:  now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("UpsertOpenGapByDedupeKeyNoBump again: %v", err)
	}
	if isNew {
		t.Fatal("expected existing gap")
	}
	if again.ID != created.ID {
		t.Fatalf("existing gap ID=%q, want %q", again.ID, created.ID)
	}
	if again.EvidenceCount != 0 {
		t.Fatalf("existing gap evidence_count=%d, want 0", again.EvidenceCount)
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

func TestSupportCoverageRepository_CreateEvidenceIfAbsentBySourceKey(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	inserted, err := repo.CreateEvidenceIfAbsent(ctx, &model.SupportGapEvidence{
		ID:           "ev-1",
		GapID:        "gap-1",
		WorkspaceID:  "ws-1",
		SourceKey:    "coverage_analysis:analysis-1",
		EvidenceType: model.SupportCoverageGapSourceDailyConversationAnalysis,
		SourceSignal: model.SupportCoverageGapSourceDailyConversationAnalysis,
		CreatedAt:    now,
	})
	if err != nil {
		t.Fatalf("CreateEvidenceIfAbsent first: %v", err)
	}
	if !inserted {
		t.Fatal("expected first evidence insert")
	}

	inserted, err = repo.CreateEvidenceIfAbsent(ctx, &model.SupportGapEvidence{
		ID:           "ev-duplicate",
		GapID:        "gap-1",
		WorkspaceID:  "ws-1",
		SourceKey:    "coverage_analysis:analysis-1",
		EvidenceType: model.SupportCoverageGapSourceDailyConversationAnalysis,
		SourceSignal: model.SupportCoverageGapSourceDailyConversationAnalysis,
		CreatedAt:    now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateEvidenceIfAbsent duplicate: %v", err)
	}
	if inserted {
		t.Fatal("expected duplicate source_key to be ignored")
	}

	var count int64
	if err := db.Model(&model.SupportGapEvidence{}).Where("workspace_id = ? AND source_key = ?", "ws-1", "coverage_analysis:analysis-1").Count(&count).Error; err != nil {
		t.Fatalf("count evidence: %v", err)
	}
	if count != 1 {
		t.Fatalf("evidence count=%d, want 1", count)
	}
}

func TestSupportCoverageRepository_UpdateGapEmbedding(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	if _, _, err := repo.UpsertOpenGapByDedupeKeyNoBump(ctx, &model.SupportCoverageGap{
		ID:          "gap-embedding",
		WorkspaceID: "ws-1",
		DedupeKey:   "semantic:embedding",
		Title:       "Embedding test",
		Status:      model.SupportCoverageGapStatusOpen,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}); err != nil {
		t.Fatalf("seed gap: %v", err)
	}

	if err := repo.UpdateGapEmbedding(ctx, "gap-embedding", "[0.1,0.2]", "openai", "text-embedding-3-small", "coverage-gap-canonical-v1", 1536, "hash-1", now); err != nil {
		t.Fatalf("UpdateGapEmbedding: %v", err)
	}

	var gap model.SupportCoverageGap
	if err := db.First(&gap, "id = ?", "gap-embedding").Error; err != nil {
		t.Fatalf("load gap: %v", err)
	}
	if gap.Embedding != "[0.1,0.2]" || gap.EmbeddingModel != "text-embedding-3-small" || gap.EmbeddingTextHash != "hash-1" {
		t.Fatalf("embedding fields not persisted: %+v", gap)
	}
}

func TestSupportCoverageRepository_FindNearestOpenGapsByEmbeddingSQLiteReturnsEmpty(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()

	items, err := repo.FindNearestOpenGapsByEmbedding(ctx, "ws-1", "[0.1,0.2]", "openai", "text-embedding-3-small", "coverage-gap-canonical-v1", 1536, 10)
	if err != nil {
		t.Fatalf("FindNearestOpenGapsByEmbedding: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items=%d, want 0 on sqlite", len(items))
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

func TestSupportCoverageRepository_ListGapsRanksByRecentEvidence(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	older := now.Add(-31 * 24 * time.Hour)
	for _, seed := range []struct {
		id         string
		dedupeKey  string
		title      string
		recentRows int
		oldRows    int
	}{
		{id: "gap-low", dedupeKey: "low", title: "Low evidence", recentRows: 1},
		{id: "gap-high", dedupeKey: "high", title: "High evidence", recentRows: 3, oldRows: 2},
	} {
		if _, _, err := repo.UpsertGapByDedupeKey(ctx, &model.SupportCoverageGap{
			ID:          seed.id,
			WorkspaceID: "ws-1",
			DedupeKey:   seed.dedupeKey,
			Title:       seed.title,
			FirstSeenAt: now,
			LastSeenAt:  now,
		}); err != nil {
			t.Fatalf("seed gap %s: %v", seed.id, err)
		}
		for i := 0; i < seed.recentRows; i++ {
			if err := repo.CreateEvidence(ctx, &model.SupportGapEvidence{
				GapID:        seed.id,
				WorkspaceID:  "ws-1",
				EvidenceType: model.SupportEventDocsIssueFeedback,
				CreatedAt:    now.Add(time.Duration(i) * time.Minute),
			}); err != nil {
				t.Fatalf("seed recent evidence: %v", err)
			}
		}
		for i := 0; i < seed.oldRows; i++ {
			if err := repo.CreateEvidence(ctx, &model.SupportGapEvidence{
				GapID:        seed.id,
				WorkspaceID:  "ws-1",
				EvidenceType: model.SupportEventDocsIssueFeedback,
				CreatedAt:    older.Add(time.Duration(i) * time.Minute),
			}); err != nil {
				t.Fatalf("seed old evidence: %v", err)
			}
		}
	}

	items, total, err := repo.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if err != nil {
		t.Fatalf("ListGaps: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("got total=%d len=%d, want 2", total, len(items))
	}
	if items[0].ID != "gap-high" || items[0].Evidence30d != 3 {
		t.Fatalf("first item id=%s evidence_30d=%d, want gap-high/3", items[0].ID, items[0].Evidence30d)
	}
	if items[1].ID != "gap-low" || items[1].Evidence30d != 1 {
		t.Fatalf("second item id=%s evidence_30d=%d, want gap-low/1", items[1].ID, items[1].Evidence30d)
	}
}

func TestSupportCoverageRepositoryListGapsRanksByExplainableImpact(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	for _, gap := range []model.SupportCoverageGap{
		{ID: "gap-noisy", WorkspaceID: "ws-1", DedupeKey: "noisy", Title: "One account repeats the same issue", Status: model.SupportCoverageGapStatusOpen, Confidence: 0.9, Metadata: json.RawMessage(`{}`), FirstSeenAt: now.Add(-2 * time.Hour), LastSeenAt: now.Add(-time.Hour)},
		{ID: "gap-broad", WorkspaceID: "ws-1", DedupeKey: "broad", Title: "Several customers need missing setup docs", Status: model.SupportCoverageGapStatusOpen, Confidence: 0.9, Metadata: json.RawMessage(`{}`), FirstSeenAt: now.Add(-2 * time.Hour), LastSeenAt: now.Add(-90 * time.Minute)},
	} {
		if err := db.Create(&gap).Error; err != nil {
			t.Fatalf("seed gap %s: %v", gap.ID, err)
		}
	}
	for i := 0; i < 10; i++ {
		convID := fmt.Sprintf("conv-noisy-%d", i)
		if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, customer_email) VALUES (?, ?, ?)`, convID, "ws-1", "noisy@example.com").Error; err != nil {
			t.Fatalf("seed noisy conversation: %v", err)
		}
		if err := repo.CreateEvidence(ctx, &model.SupportGapEvidence{GapID: "gap-noisy", WorkspaceID: "ws-1", ConversationID: &convID, EvidenceType: model.SupportEventDocsIssueFeedback, CreatedAt: now.Add(time.Duration(i) * time.Minute)}); err != nil {
			t.Fatalf("seed noisy evidence: %v", err)
		}
	}
	for i := 0; i < 4; i++ {
		convID := fmt.Sprintf("conv-broad-%d", i)
		email := fmt.Sprintf("customer-%d@example.com", i)
		if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, customer_email) VALUES (?, ?, ?)`, convID, "ws-1", email).Error; err != nil {
			t.Fatalf("seed broad conversation: %v", err)
		}
		if err := repo.CreateEvidence(ctx, &model.SupportGapEvidence{GapID: "gap-broad", WorkspaceID: "ws-1", ConversationID: &convID, EvidenceType: model.SupportEventDocsIssueFeedback, CreatedAt: now.Add(time.Duration(i) * time.Minute)}); err != nil {
			t.Fatalf("seed broad evidence: %v", err)
		}
	}

	items, total, err := repo.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if err != nil {
		t.Fatalf("ListGaps: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("got total=%d len=%d, want 2", total, len(items))
	}
	if items[0].ID != "gap-broad" {
		t.Fatalf("first item id=%s, want gap-broad; scores: %+v", items[0].ID, items)
	}
	if items[0].DistinctCustomers30d != 4 || items[0].Evidence30d != 4 || items[0].EvidenceAll != 4 {
		t.Fatalf("impact components not populated: %+v", items[0])
	}
	if items[0].ImpactScore <= items[1].ImpactScore {
		t.Fatalf("impact scores not ranked: first=%f second=%f", items[0].ImpactScore, items[1].ImpactScore)
	}
	if items[0].ImpactExplanation == "" || !strings.Contains(items[0].ImpactExplanation, "4 customers") || !strings.Contains(items[0].ImpactExplanation, "no nearby content") {
		t.Fatalf("impact explanation missing components: %q", items[0].ImpactExplanation)
	}
}

func TestSupportCoverageRepository_ListGapsHidesRawLowConfidenceEventGapsByDefault(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	seeds := []model.SupportCoverageGap{
		{
			ID:          "raw-low",
			WorkspaceID: "ws-1",
			DedupeKey:   "raw-low",
			Title:       "Raw low confidence event gap",
			V1GapType:   model.SupportCoverageV1GapNeedsReview,
			Confidence:  0.4,
			Metadata:    []byte(`{"source":"event_detection"}`),
			FirstSeenAt: now,
			LastSeenAt:  now,
		},
		{
			ID:          "raw-high",
			WorkspaceID: "ws-1",
			DedupeKey:   "raw-high",
			Title:       "Raw high confidence event gap",
			V1GapType:   model.SupportCoverageV1GapNeedsReview,
			Confidence:  0.8,
			Metadata:    []byte(`{"source":"event_detection"}`),
			FirstSeenAt: now,
			LastSeenAt:  now,
		},
		{
			ID:          "daily-low",
			WorkspaceID: "ws-1",
			DedupeKey:   "daily-low",
			Title:       "Daily analyzer gap",
			V1GapType:   model.SupportCoverageV1GapNeedsReview,
			Confidence:  0.4,
			Metadata:    []byte(`{"source":"daily_conversation_analysis"}`),
			FirstSeenAt: now,
			LastSeenAt:  now,
		},
	}
	for i := range seeds {
		if err := db.Create(&seeds[i]).Error; err != nil {
			t.Fatalf("seed gap %s: %v", seeds[i].ID, err)
		}
	}

	items, total, err := repo.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{})
	if err != nil {
		t.Fatalf("ListGaps: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("got total=%d len=%d, want 2 visible gaps", total, len(items))
	}
	for _, item := range items {
		if item.ID == "raw-low" {
			t.Fatal("raw low-confidence event gap should be hidden by default")
		}
	}

	items, total, err = repo.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
	if err != nil {
		t.Fatalf("ListGaps show raw: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("show raw got total=%d len=%d, want 3", total, len(items))
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

	if err := repo.UpdateGapStatus(ctx, "ws-1", "gap-1", model.SupportCoverageGapStatusRejected, "user-1", nil); err != nil {
		t.Fatalf("UpdateGapStatus: %v", err)
	}
	var gap model.SupportCoverageGap
	if err := db.Where("id = ?", "gap-1").First(&gap).Error; err != nil {
		t.Fatalf("load gap: %v", err)
	}
	if gap.ClosedAt == nil || gap.ClosedEvidenceCount == nil || *gap.ClosedEvidenceCount != 1 {
		t.Fatalf("expected closed metadata, got closed_at=%v closed_evidence_count=%v", gap.ClosedAt, gap.ClosedEvidenceCount)
	}

	// Wrong workspace returns error.
	if err := repo.UpdateGapStatus(ctx, "ws-other", "gap-1", model.SupportCoverageGapStatusDone, "user-1", nil); err == nil {
		t.Error("expected error for wrong workspace")
	}
}

func TestSupportCoverageRepository_MarkGapDoneSnapshotsEvidenceAndRejectsClosedGap(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	_, _, err := repo.UpsertGapByDedupeKey(ctx, &model.SupportCoverageGap{
		ID:          "gap-done",
		WorkspaceID: "ws-1",
		DedupeKey:   "done-test",
		FirstSeenAt: now,
		LastSeenAt:  now,
	})
	if err != nil {
		t.Fatalf("seed gap: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := repo.CreateEvidence(ctx, &model.SupportGapEvidence{
			GapID:        "gap-done",
			WorkspaceID:  "ws-1",
			EvidenceType: model.SupportEventHumanReplyAfterAI,
			Excerpt:      "evidence",
			CreatedAt:    now.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("seed evidence: %v", err)
		}
	}

	evidence30d, err := repo.CountEvidence30d(ctx, "gap-done")
	if err != nil {
		t.Fatalf("CountEvidence30d: %v", err)
	}
	if evidence30d != 2 {
		t.Fatalf("evidence30d=%d, want 2", evidence30d)
	}
	if err := repo.MarkGapDone(ctx, "ws-1", "gap-done", "doc-1", evidence30d); err != nil {
		t.Fatalf("MarkGapDone: %v", err)
	}

	var gap model.SupportCoverageGap
	if err := db.Where("id = ?", "gap-done").First(&gap).Error; err != nil {
		t.Fatalf("load gap: %v", err)
	}
	if gap.Status != model.SupportCoverageGapStatusDone {
		t.Fatalf("status=%q, want done", gap.Status)
	}
	if gap.ResultDocumentID == nil || *gap.ResultDocumentID != "doc-1" {
		t.Fatalf("result_document_id=%v, want doc-1", gap.ResultDocumentID)
	}
	if gap.ClosedEvidenceCount == nil || *gap.ClosedEvidenceCount != 2 {
		t.Fatalf("closed_evidence_count=%v, want 2", gap.ClosedEvidenceCount)
	}
	if err := repo.MarkGapDone(ctx, "ws-1", "gap-done", "doc-2", evidence30d); err == nil {
		t.Fatal("expected second MarkGapDone on closed gap to fail")
	}
}

func TestSupportCoverageRepository_MarkGapRejectedSnapshotsReasonAndRejectsClosedGap(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	_, _, err := repo.UpsertGapByDedupeKey(ctx, &model.SupportCoverageGap{
		ID:          "gap-reject",
		WorkspaceID: "ws-1",
		DedupeKey:   "reject-test",
		FirstSeenAt: now,
		LastSeenAt:  now,
	})
	if err != nil {
		t.Fatalf("seed gap: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := repo.CreateEvidence(ctx, &model.SupportGapEvidence{
			GapID:        "gap-reject",
			WorkspaceID:  "ws-1",
			EvidenceType: model.SupportEventDocsIssueFeedback,
			Excerpt:      "evidence",
			CreatedAt:    now.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("seed evidence: %v", err)
		}
	}

	reason := "not a docs problem"
	evidence30d, err := repo.CountEvidence30d(ctx, "gap-reject")
	if err != nil {
		t.Fatalf("CountEvidence30d: %v", err)
	}
	if err := repo.MarkGapRejected(ctx, "ws-1", "gap-reject", "user-1", &reason, evidence30d); err != nil {
		t.Fatalf("MarkGapRejected: %v", err)
	}

	var gap model.SupportCoverageGap
	if err := db.Where("id = ?", "gap-reject").First(&gap).Error; err != nil {
		t.Fatalf("load gap: %v", err)
	}
	if gap.Status != model.SupportCoverageGapStatusRejected {
		t.Fatalf("status=%q, want rejected", gap.Status)
	}
	if gap.RejectionReason == nil || *gap.RejectionReason != reason {
		t.Fatalf("rejection_reason=%v, want %q", gap.RejectionReason, reason)
	}
	if gap.StatusChangedBy == nil || *gap.StatusChangedBy != "user-1" {
		t.Fatalf("status_changed_by=%v, want user-1", gap.StatusChangedBy)
	}
	if gap.ClosedEvidenceCount == nil || *gap.ClosedEvidenceCount != 3 {
		t.Fatalf("closed_evidence_count=%v, want 3", gap.ClosedEvidenceCount)
	}
	if err := repo.MarkGapRejected(ctx, "ws-1", "gap-reject", "user-1", nil, evidence30d); !errors.Is(err, ErrGapAlreadyClosed) {
		t.Fatalf("second MarkGapRejected error=%v, want ErrGapAlreadyClosed", err)
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

func TestSupportCoverageRepository_GetGapDetailExposesAnalysisExplanationAndRecommendations(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	gap, _, err := repo.UpsertGapByDedupeKey(ctx, &model.SupportCoverageGap{
		ID:          "gap-analysis",
		WorkspaceID: "ws-1",
		DedupeKey:   "analysis-gap",
		Title:       "Refund exceptions",
		Status:      model.SupportCoverageGapStatusOpen,
		FirstSeenAt: now,
		LastSeenAt:  now,
	})
	if err != nil {
		t.Fatalf("seed gap: %v", err)
	}
	metadata, _ := json.Marshal(map[string]any{
		"customer_need":    "Customer needed refund exception criteria.",
		"ai_failure":       "AI only found generic refund docs.",
		"human_resolution": "Agent explained the exception and refunded from Stripe.",
		"decision_reason":  "The human answer should be reusable by AI.",
	})
	if err := repo.CreateEvidence(ctx, &model.SupportGapEvidence{
		ID:           "evidence-analysis",
		GapID:        gap.ID,
		WorkspaceID:  "ws-1",
		EvidenceType: "daily_conversation_analysis",
		SourceSignal: "daily_conversation_analysis",
		Excerpt:      "Customer needed refund exception criteria.",
		Metadata:     metadata,
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
	targetID := "doc-1"
	if err := db.Create(&model.SupportCoverageRecommendation{
		ID:                 "rec-primary",
		WorkspaceID:        "ws-1",
		GapID:              gap.ID,
		RecommendationType: model.SupportCoverageFixUpdateArticle,
		TargetType:         "docs",
		TargetID:           &targetID,
		TargetTitle:        "Refunds",
		Priority:           model.SupportCoverageRecommendationPriorityPrimary,
		Status:             model.SupportCoverageRecommendationStatusOpen,
		Rationale:          "Existing docs are close.",
		SuggestedChange:    "Add exception criteria.",
		Metadata:           json.RawMessage(`{}`),
		CreatedAt:          now,
		UpdatedAt:          now,
	}).Error; err != nil {
		t.Fatalf("seed recommendation: %v", err)
	}

	detail, err := repo.GetGapDetail(ctx, "ws-1", gap.ID)
	if err != nil {
		t.Fatalf("GetGapDetail: %v", err)
	}
	if detail.AnalysisExplanation == nil {
		t.Fatal("expected analysis explanation")
	}
	if detail.AnalysisExplanation.CustomerNeed != "Customer needed refund exception criteria." {
		t.Fatalf("unexpected explanation: %+v", detail.AnalysisExplanation)
	}
	if len(detail.Recommendations) != 1 {
		t.Fatalf("expected one recommendation, got %+v", detail.Recommendations)
	}
	if detail.Recommendations[0].RecommendationType != model.SupportCoverageFixUpdateArticle {
		t.Fatalf("unexpected recommendation: %+v", detail.Recommendations[0])
	}
}

func TestSupportCoverageRepository_GetGapDetailJoinsSenderRole(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	ctx := context.Background()
	now := time.Now()

	gap, _, err := repo.UpsertGapByDedupeKey(ctx, &model.SupportCoverageGap{
		ID:          "gap-roles",
		WorkspaceID: "ws-1",
		DedupeKey:   "roles-gap",
		Title:       "Sender role join",
		Status:      model.SupportCoverageGapStatusOpen,
		FirstSeenAt: now,
		LastSeenAt:  now,
	})
	if err != nil {
		t.Fatalf("seed gap: %v", err)
	}

	type seedMsg struct {
		ID         string
		SenderType string
	}
	messages := []seedMsg{
		{ID: "msg-customer", SenderType: "customer"},
		{ID: "msg-agent", SenderType: "agent"},
		{ID: "msg-ai", SenderType: "ai"},
	}
	for _, m := range messages {
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content) VALUES (?, ?, ?, ?, ?)`,
			m.ID, "ws-1", "conv-1", m.SenderType, "hello",
		).Error; err != nil {
			t.Fatalf("seed message %s: %v", m.ID, err)
		}
		convID := "conv-1"
		messageID := m.ID
		if err := repo.CreateEvidence(ctx, &model.SupportGapEvidence{
			ID:             "ev-" + m.ID,
			GapID:          gap.ID,
			WorkspaceID:    "ws-1",
			EvidenceType:   "ai_handoff",
			ConversationID: &convID,
			MessageID:      &messageID,
			Excerpt:        "from " + m.SenderType,
			CreatedAt:      now,
		}); err != nil {
			t.Fatalf("seed evidence: %v", err)
		}
	}

	// Evidence with no message_id should produce empty sender_role.
	if err := repo.CreateEvidence(ctx, &model.SupportGapEvidence{
		ID:           "ev-no-msg",
		GapID:        gap.ID,
		WorkspaceID:  "ws-1",
		EvidenceType: "article_feedback",
		Excerpt:      "no linked message",
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("seed evidence no-msg: %v", err)
	}

	detail, err := repo.GetGapDetail(ctx, "ws-1", gap.ID)
	if err != nil {
		t.Fatalf("GetGapDetail: %v", err)
	}
	if len(detail.Evidence) != 4 {
		t.Fatalf("expected 4 evidence rows, got %d", len(detail.Evidence))
	}

	roleByEvidence := map[string]string{}
	for _, ev := range detail.Evidence {
		roleByEvidence[ev.ID] = ev.SenderRole
	}
	want := map[string]string{
		"ev-msg-customer": "customer",
		"ev-msg-agent":    "agent",
		"ev-msg-ai":       "ai",
		"ev-no-msg":       "",
	}
	for id, expected := range want {
		if got := roleByEvidence[id]; got != expected {
			t.Errorf("evidence %s: sender_role = %q, want %q", id, got, expected)
		}
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
