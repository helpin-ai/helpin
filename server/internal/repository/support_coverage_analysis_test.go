package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSupportCoverageAnalysisTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:support_coverage_analysis_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	tables := []string{
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
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(workspace_id, window_start, window_end, analyzer_version)
		)`,
		`CREATE TABLE support_coverage_conversation_analyses (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			status TEXT NOT NULL,
			has_gap BOOLEAN NOT NULL DEFAULT 0,
			gap_id TEXT,
			gap_kind TEXT NOT NULL DEFAULT '',
			gap_category TEXT NOT NULL DEFAULT '',
			primary_recommendation_type TEXT NOT NULL DEFAULT '',
			transcript_hash TEXT NOT NULL DEFAULT '',
			analyzer_version TEXT NOT NULL DEFAULT 'v1',
			canonical_title TEXT NOT NULL DEFAULT '',
			customer_need TEXT NOT NULL DEFAULT '',
			ai_failure TEXT NOT NULL DEFAULT '',
			human_resolution TEXT NOT NULL DEFAULT '',
			decision_reason TEXT NOT NULL DEFAULT '',
			confidence REAL NOT NULL DEFAULT 0,
			is_support_query BOOLEAN NOT NULL DEFAULT 1,
			conversation_type TEXT NOT NULL DEFAULT 'support_query',
			classification_reason TEXT NOT NULL DEFAULT '',
			error_message TEXT,
			raw_output TEXT NOT NULL DEFAULT '{}',
			embedding TEXT,
			embedding_provider TEXT NOT NULL DEFAULT '',
			embedding_model TEXT NOT NULL DEFAULT '',
			embedding_version TEXT NOT NULL DEFAULT '',
			embedding_dimensions INTEGER NOT NULL DEFAULT 0,
			embedding_text_hash TEXT NOT NULL DEFAULT '',
			embedding_updated_at DATETIME,
			materialization_metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(workspace_id, conversation_id, transcript_hash, analyzer_version)
		)`,
		`CREATE TABLE support_ai_retrieval_traces (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			message_id TEXT NOT NULL,
			search_queries TEXT NOT NULL DEFAULT '[]',
			results TEXT NOT NULL DEFAULT '[]',
			cited_source_ids TEXT NOT NULL DEFAULT '[]',
			ai_confidence REAL NOT NULL DEFAULT 0,
			can_answer TEXT,
			can_resolve TEXT,
			failure_mode TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(workspace_id, message_id)
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
	}
	for _, stmt := range tables {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	return db
}

func TestSupportCoverageAnalysisRepository_CreateRunIsIdempotentAndCursor(t *testing.T) {
	db := setupSupportCoverageAnalysisTestDB(t)
	repo := NewSupportCoverageAnalysisRepository(db)
	ctx := context.Background()
	windowStart := time.Date(2026, 4, 28, 4, 30, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 4, 29, 4, 30, 0, 0, time.UTC)

	run, err := repo.CreateRun(ctx, &model.SupportCoverageAnalysisRun{
		ID:              "run-1",
		WorkspaceID:     "ws-1",
		WindowStart:     windowStart,
		WindowEnd:       windowEnd,
		CursorStartedAt: windowStart.Add(-2 * time.Hour),
		CursorEndedAt:   windowEnd.Add(-10 * time.Minute),
		AnalyzerVersion: "v1",
		Status:          model.SupportCoverageAnalysisRunStatusRunning,
		StartedAt:       windowEnd,
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	again, err := repo.CreateRun(ctx, &model.SupportCoverageAnalysisRun{
		ID:              "run-2",
		WorkspaceID:     "ws-1",
		WindowStart:     windowStart,
		WindowEnd:       windowEnd,
		CursorStartedAt: windowStart,
		CursorEndedAt:   windowEnd,
		AnalyzerVersion: "v1",
		Status:          model.SupportCoverageAnalysisRunStatusRunning,
		StartedAt:       windowEnd,
	})
	if err != nil {
		t.Fatalf("CreateRun again: %v", err)
	}
	if again.ID != run.ID {
		t.Fatalf("expected duplicate run to return %q, got %q", run.ID, again.ID)
	}

	if err := repo.CompleteRun(ctx, run.ID, 12, 3); err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}
	cursor, err := repo.LastSuccessfulCursor(ctx, "ws-1", "v1")
	if err != nil {
		t.Fatalf("LastSuccessfulCursor: %v", err)
	}
	if cursor == nil || !cursor.Equal(windowEnd.Add(-10*time.Minute)) {
		t.Fatalf("expected cursor %s, got %v", windowEnd.Add(-10*time.Minute), cursor)
	}

	var count int64
	if err := db.Model(&model.SupportCoverageAnalysisRun{}).Count(&count).Error; err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one run, got %d", count)
	}
}

func TestSupportCoverageAnalysisRepository_RecordConversationAnalysisIdempotentByTranscript(t *testing.T) {
	db := setupSupportCoverageAnalysisTestDB(t)
	repo := NewSupportCoverageAnalysisRepository(db)
	ctx := context.Background()

	analysis := &model.SupportCoverageConversationAnalysis{
		ID:              "analysis-1",
		WorkspaceID:     "ws-1",
		RunID:           "run-1",
		ConversationID:  "conversation-1",
		Status:          model.SupportCoverageConversationAnalysisStatusAnalyzed,
		TranscriptHash:  "hash-1",
		AnalyzerVersion: "v1",
		RawOutput:       json.RawMessage(`{"has_gap":true}`),
	}
	if err := repo.RecordConversationAnalysis(ctx, analysis); err != nil {
		t.Fatalf("RecordConversationAnalysis: %v", err)
	}
	if err := repo.RecordConversationAnalysis(ctx, &model.SupportCoverageConversationAnalysis{
		ID:              "analysis-duplicate",
		WorkspaceID:     "ws-1",
		RunID:           "run-1",
		ConversationID:  "conversation-1",
		Status:          model.SupportCoverageConversationAnalysisStatusAnalyzed,
		TranscriptHash:  "hash-1",
		AnalyzerVersion: "v1",
	}); err != nil {
		t.Fatalf("RecordConversationAnalysis duplicate: %v", err)
	}

	analyzed, err := repo.AlreadyAnalyzedConversation(ctx, "ws-1", "conversation-1", "hash-1", "v1")
	if err != nil {
		t.Fatalf("AlreadyAnalyzedConversation: %v", err)
	}
	if !analyzed {
		t.Fatal("expected transcript to be marked analyzed")
	}
	analyzed, err = repo.AlreadyAnalyzedConversation(ctx, "ws-1", "conversation-1", "hash-2", "v1")
	if err != nil {
		t.Fatalf("AlreadyAnalyzedConversation hash-2: %v", err)
	}
	if analyzed {
		t.Fatal("different transcript hash should not be analyzed")
	}

	var count int64
	if err := db.Model(&model.SupportCoverageConversationAnalysis{}).Count(&count).Error; err != nil {
		t.Fatalf("count analyses: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one analysis row, got %d", count)
	}
}

func TestSupportCoverageAnalysisRepository_UpsertAndListRetrievalTraces(t *testing.T) {
	db := setupSupportCoverageAnalysisTestDB(t)
	repo := NewSupportCoverageAnalysisRepository(db)
	ctx := context.Background()
	canAnswer := "partial"

	if err := repo.UpsertRetrievalTrace(ctx, &model.SupportAIRetrievalTrace{
		ID:             "trace-1",
		WorkspaceID:    "ws-1",
		ConversationID: "conversation-1",
		MessageID:      "message-1",
		SearchQueries:  json.RawMessage(`["refund policy"]`),
		Results:        json.RawMessage(`[{"source_type":"docs","title":"Refunds"}]`),
		CitedSourceIDs: json.RawMessage(`["doc-1"]`),
		AIConfidence:   0.42,
		CanAnswer:      &canAnswer,
		FailureMode:    "weak_retrieval",
	}); err != nil {
		t.Fatalf("UpsertRetrievalTrace: %v", err)
	}
	if err := repo.UpsertRetrievalTrace(ctx, &model.SupportAIRetrievalTrace{
		ID:             "trace-duplicate",
		WorkspaceID:    "ws-1",
		ConversationID: "conversation-1",
		MessageID:      "message-1",
		SearchQueries:  json.RawMessage(`["updated"]`),
		AIConfidence:   0.81,
		FailureMode:    "",
	}); err != nil {
		t.Fatalf("UpsertRetrievalTrace duplicate: %v", err)
	}

	traces, err := repo.ListRetrievalTracesByConversation(ctx, "ws-1", "conversation-1")
	if err != nil {
		t.Fatalf("ListRetrievalTracesByConversation: %v", err)
	}
	if len(traces) != 1 {
		t.Fatalf("expected one trace, got %d", len(traces))
	}
	if traces[0].AIConfidence != 0.81 {
		t.Fatalf("expected updated confidence, got %.2f", traces[0].AIConfidence)
	}
	if string(traces[0].SearchQueries) != `["updated"]` {
		t.Fatalf("expected updated search queries, got %s", traces[0].SearchQueries)
	}
}

func TestSupportCoverageAnalysisRepository_ReplaceRecommendationsPreservesDecisions(t *testing.T) {
	db := setupSupportCoverageAnalysisTestDB(t)
	repo := NewSupportCoverageAnalysisRepository(db)
	ctx := context.Background()

	existing := []model.SupportCoverageRecommendation{
		{ID: "open-old", WorkspaceID: "ws-1", GapID: "gap-1", RecommendationType: model.SupportCoverageFixCreateArticle, Status: model.SupportCoverageRecommendationStatusOpen, Priority: model.SupportCoverageRecommendationPriorityPrimary, Metadata: json.RawMessage(`{}`)},
		{ID: "accepted-old", WorkspaceID: "ws-1", GapID: "gap-1", RecommendationType: model.SupportCoverageFixUpdateArticle, Status: model.SupportCoverageRecommendationStatusAccepted, Priority: model.SupportCoverageRecommendationPriorityPrimary, Metadata: json.RawMessage(`{}`)},
		{ID: "applied-old", WorkspaceID: "ws-1", GapID: "gap-1", RecommendationType: model.SupportCoverageFixAddAction, Status: model.SupportCoverageRecommendationStatusApplied, Priority: model.SupportCoverageRecommendationPrioritySecondary, Metadata: json.RawMessage(`{}`)},
		{ID: "dismissed-old", WorkspaceID: "ws-1", GapID: "gap-1", RecommendationType: model.SupportCoverageFixNoFix, Status: model.SupportCoverageRecommendationStatusDismissed, Priority: model.SupportCoverageRecommendationPrioritySecondary, Metadata: json.RawMessage(`{}`)},
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed recommendations: %v", err)
	}

	err := repo.ReplaceRecommendations(ctx, "ws-1", "gap-1", []model.SupportCoverageRecommendation{
		{
			ID:                 "open-new-1",
			RecommendationType: model.SupportCoverageFixUpdateWebsitePage,
			Priority:           model.SupportCoverageRecommendationPriorityPrimary,
			Rationale:          "Website search is the best prospect surface.",
		},
		{
			ID:                 "open-new-2",
			RecommendationType: model.SupportCoverageFixDefinePolicy,
			Priority:           model.SupportCoverageRecommendationPrioritySecondary,
			Rationale:          "The team needs a clear refund exception policy.",
		},
	})
	if err != nil {
		t.Fatalf("ReplaceRecommendations: %v", err)
	}

	var recs []model.SupportCoverageRecommendation
	if err := db.Order("id").Find(&recs, "workspace_id = ? AND gap_id = ?", "ws-1", "gap-1").Error; err != nil {
		t.Fatalf("list recommendations: %v", err)
	}
	seen := map[string]string{}
	for _, rec := range recs {
		seen[rec.ID] = rec.Status
	}
	if _, ok := seen["open-old"]; ok {
		t.Fatal("old open recommendation should be replaced")
	}
	for id, status := range map[string]string{
		"accepted-old":  model.SupportCoverageRecommendationStatusAccepted,
		"applied-old":   model.SupportCoverageRecommendationStatusApplied,
		"dismissed-old": model.SupportCoverageRecommendationStatusDismissed,
		"open-new-1":    model.SupportCoverageRecommendationStatusOpen,
		"open-new-2":    model.SupportCoverageRecommendationStatusOpen,
	} {
		if seen[id] != status {
			t.Fatalf("expected %s status %q, got %q", id, status, seen[id])
		}
	}
}
