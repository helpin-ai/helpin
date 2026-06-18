package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeCoverageEnrichmentLLM struct {
	resp  string
	err   error
	calls int
}

func (f *fakeCoverageEnrichmentLLM) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &llm.ChatResponse{Content: f.resp}, nil
}

func TestSupportCoverageEnrichment_LoadKBContextRanksTopFiveByTokenOverlap(t *testing.T) {
	db := setupCoverageEnrichmentTestDB(t)
	seedCoverageDoc(t, db, "doc-1", "ws-1", "Reset your password")
	seedCoverageDoc(t, db, "doc-2", "ws-1", "Change your billing email")
	seedCoverageDoc(t, db, "doc-3", "ws-1", "Password reset link expired")
	seedCoverageDoc(t, db, "doc-4", "ws-1", "Invite a teammate")
	seedCoverageDoc(t, db, "doc-5", "ws-1", "Password policy requirements")
	seedCoverageDoc(t, db, "doc-6", "ws-1", "Reset two factor authentication")
	seedCoverageDoc(t, db, "doc-7", "ws-1", "Configure SSO")
	seedCoverageDoc(t, db, "doc-other", "ws-2", "Reset password")
	svc := NewSupportCoverageEnrichmentService(db, nil)

	ctx, err := svc.LoadKBContext(context.Background(), "ws-1", "How do I reset my password link?")
	if err != nil {
		t.Fatalf("LoadKBContext: %v", err)
	}

	if len(ctx.Titles) != 7 {
		t.Fatalf("titles=%d, want 7", len(ctx.Titles))
	}
	if len(ctx.Candidates) != 5 {
		t.Fatalf("candidates=%d, want 5", len(ctx.Candidates))
	}
	if ctx.Candidates[0].DocumentID != "doc-3" {
		t.Fatalf("top candidate=%q, want doc-3", ctx.Candidates[0].DocumentID)
	}
	for _, candidate := range ctx.Candidates {
		if candidate.DocumentID == "doc-other" {
			t.Fatal("candidate from another workspace leaked into context")
		}
	}
}

func TestSupportCoverageEnrichment_EnrichTopicWritesVersionedSuggestion(t *testing.T) {
	db := setupCoverageEnrichmentTestDB(t)
	now := time.Date(2026, 4, 27, 15, 0, 0, 0, time.UTC)
	seedCoverageTopicGapEvidence(t, db, "topic-1", "gap-1", "ws-1", "How do I reset my password?")
	seedCoverageSuggestion(t, db, "old-suggestion", "gap-1", "ws-1", true)
	fake := &fakeCoverageEnrichmentLLM{resp: `{
		"canonical_title": "Write article: Reset your password",
		"gap_subtype": "missing_article",
		"route": "create_article",
		"target_document_id": "",
		"draft_content": {"type":"doc","content":[{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"Reset your password"}]}]},
		"confidence": 0.82
	}`}
	svc := NewSupportCoverageEnrichmentService(db, fake)
	svc.now = func() time.Time { return now }

	if err := svc.EnrichTopic(context.Background(), "topic-1"); err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}

	var active []model.SupportGapSuggestion
	if err := db.Where("gap_id = ? AND is_active", "gap-1").Find(&active).Error; err != nil {
		t.Fatalf("load active suggestions: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active suggestions=%d, want 1", len(active))
	}
	if active[0].ID == "old-suggestion" {
		t.Fatal("old suggestion should have been superseded")
	}
	if active[0].SuggestionType != model.SupportCoverageSuggestionCreateArticle {
		t.Fatalf("suggestion_type=%q, want create_article", active[0].SuggestionType)
	}

	var old model.SupportGapSuggestion
	if err := db.Where("id = ?", "old-suggestion").First(&old).Error; err != nil {
		t.Fatalf("load old suggestion: %v", err)
	}
	if old.IsActive || old.SupersededAt == nil {
		t.Fatal("old suggestion was not superseded")
	}

	var topic model.SupportCoverageTopic
	if err := db.Where("id = ?", "topic-1").First(&topic).Error; err != nil {
		t.Fatalf("load topic: %v", err)
	}
	if topic.CanonicalTitle == nil || *topic.CanonicalTitle != "Write article: Reset your password" {
		t.Fatalf("canonical_title=%v, want enriched title", topic.CanonicalTitle)
	}
	if topic.LastEnrichedAt == nil || topic.CooldownUntil == nil {
		t.Fatal("expected enrichment timestamps")
	}
}

func TestSupportCoverageEnrichment_RespectsCooldown(t *testing.T) {
	db := setupCoverageEnrichmentTestDB(t)
	now := time.Date(2026, 4, 27, 15, 0, 0, 0, time.UTC)
	seedCoverageTopicGapEvidence(t, db, "topic-1", "gap-1", "ws-1", "How do I reset my password?")
	cooldown := now.Add(30 * time.Minute)
	if err := db.Model(&model.SupportCoverageTopic{}).Where("id = ?", "topic-1").Update("cooldown_until", cooldown).Error; err != nil {
		t.Fatalf("set cooldown: %v", err)
	}
	fake := &fakeCoverageEnrichmentLLM{resp: `{}`}
	svc := NewSupportCoverageEnrichmentService(db, fake)
	svc.now = func() time.Time { return now }

	if err := svc.EnrichTopic(context.Background(), "topic-1"); err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("llm calls=%d, want 0 during cooldown", fake.calls)
	}
}

func setupCoverageEnrichmentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:coverage_enrichment_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	stmts := []string{
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL
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
			created_at DATETIME,
			updated_at DATETIME
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
			first_seen_at DATETIME,
			last_seen_at DATETIME,
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
			created_at DATETIME,
			updated_at DATETIME
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
			created_at DATETIME
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
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func seedCoverageDoc(t *testing.T, db *gorm.DB, id, workspaceID, title string) {
	t.Helper()
	if err := db.Exec(
		"INSERT INTO docs_documents (id, workspace_id, title, status) VALUES (?, ?, ?, ?)",
		id, workspaceID, title, model.DocStatusPublished,
	).Error; err != nil {
		t.Fatalf("seed doc: %v", err)
	}
}

func seedCoverageTopicGapEvidence(t *testing.T, db *gorm.DB, topicID, gapID, workspaceID, summary string) {
	t.Helper()
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	topic := model.SupportCoverageTopic{
		ID:          topicID,
		WorkspaceID: workspaceID,
		IssueKey:    topicID,
		Title:       summary,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("seed topic: %v", err)
	}
	gap := model.SupportCoverageGap{
		ID:            gapID,
		WorkspaceID:   workspaceID,
		TopicID:       &topicID,
		DedupeKey:     "dedupe-" + gapID,
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryUnknown,
		V1GapType:     model.SupportCoverageV1GapNeedsReview,
		Title:         summary,
		Status:        model.SupportCoverageGapStatusOpen,
		EvidenceCount: 1,
		Metadata:      []byte("{}"),
		FirstSeenAt:   now,
		LastSeenAt:    now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(&gap).Error; err != nil {
		t.Fatalf("seed gap: %v", err)
	}
	evidence := model.SupportGapEvidence{
		ID:           "evidence-" + gapID,
		GapID:        gapID,
		WorkspaceID:  workspaceID,
		EvidenceType: model.SupportEventWidgetSearchPerformed,
		Excerpt:      summary,
		Metadata:     []byte("{}"),
		CreatedAt:    now,
	}
	if err := db.Create(&evidence).Error; err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
}

func seedCoverageSuggestion(t *testing.T, db *gorm.DB, id, gapID, workspaceID string, active bool) {
	t.Helper()
	suggestion := model.SupportGapSuggestion{
		ID:             id,
		GapID:          gapID,
		WorkspaceID:    workspaceID,
		SuggestionType: model.SupportCoverageSuggestionCreateArticle,
		Status:         model.SupportCoverageSuggestionStatusDraft,
		Title:          "Old suggestion",
		Content:        []byte(`{"type":"doc","content":[]}`),
		IsActive:       active,
		Metadata:       []byte("{}"),
		CreatedAt:      time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC),
	}
	if err := db.Create(&suggestion).Error; err != nil {
		t.Fatalf("seed suggestion: %v", err)
	}
}
