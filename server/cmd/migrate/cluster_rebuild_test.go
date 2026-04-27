package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunClusterRebuildGORM_MergesOpenDuplicateClustersPerWorkspace(t *testing.T) {
	db := setupClusterRebuildTestDB(t)
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	seedClusterRebuildGap(t, db, "gap-a1", "ws-1", "How do I reset my password?", 1, now.Add(-3*time.Hour))
	seedClusterRebuildGap(t, db, "gap-a2", "ws-1", "how to reset password", 1, now.Add(-2*time.Hour))
	seedClusterRebuildGap(t, db, "gap-b1", "ws-1", "cancel subscription", 1, now.Add(-1*time.Hour))
	seedClusterRebuildGap(t, db, "gap-c1", "ws-2", "How do I reset my password?", 1, now.Add(-3*time.Hour))

	if err := runClusterRebuildGORM(context.Background(), db, now); err != nil {
		t.Fatalf("runClusterRebuildGORM: %v", err)
	}

	var ws1Open int64
	db.Model(&model.SupportCoverageGap{}).
		Where("workspace_id = ? AND status = ?", "ws-1", model.SupportCoverageGapStatusOpen).
		Count(&ws1Open)
	if ws1Open != 2 {
		t.Fatalf("ws-1 open gaps=%d, want 2", ws1Open)
	}

	var duplicate model.SupportCoverageGap
	if err := db.Where("id = ?", "gap-a2").First(&duplicate).Error; err != nil {
		t.Fatalf("load duplicate: %v", err)
	}
	if duplicate.Status != model.SupportCoverageGapStatusRejected {
		t.Fatalf("duplicate status=%q, want rejected", duplicate.Status)
	}
	if duplicate.RejectionReason == nil || *duplicate.RejectionReason == "" {
		t.Fatal("expected rejection reason")
	}
	if duplicate.ClosedAt == nil {
		t.Fatal("expected closed_at")
	}

	var primary model.SupportCoverageGap
	if err := db.Where("id = ?", "gap-a1").First(&primary).Error; err != nil {
		t.Fatalf("load primary: %v", err)
	}
	if primary.EvidenceCount != 2 {
		t.Fatalf("primary evidence_count=%d, want 2", primary.EvidenceCount)
	}
	if primary.TopicID == nil || duplicate.TopicID == nil || *primary.TopicID != *duplicate.TopicID {
		t.Fatal("expected duplicate and primary to share rebuild topic")
	}

	var movedEvidence int64
	db.Model(&model.SupportGapEvidence{}).Where("gap_id = ?", "gap-a1").Count(&movedEvidence)
	if movedEvidence != 2 {
		t.Fatalf("moved evidence rows=%d, want 2", movedEvidence)
	}

	var ws2Open int64
	db.Model(&model.SupportCoverageGap{}).
		Where("workspace_id = ? AND status = ?", "ws-2", model.SupportCoverageGapStatusOpen).
		Count(&ws2Open)
	if ws2Open != 1 {
		t.Fatalf("ws-2 open gaps=%d, want 1", ws2Open)
	}
}

func setupClusterRebuildTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:coverage_rebuild_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	stmts := []string{
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
		`CREATE UNIQUE INDEX idx_support_coverage_topics_workspace_cluster_key
			ON support_coverage_topics(workspace_id, cluster_key)
			WHERE cluster_key IS NOT NULL`,
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
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE UNIQUE INDEX idx_support_coverage_gaps_workspace_topic_open
			ON support_coverage_gaps(workspace_id, topic_id)
			WHERE status = 'open' AND topic_id IS NOT NULL`,
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
		`CREATE TABLE support_coverage_gap_articles (
			id TEXT PRIMARY KEY,
			gap_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			created_at DATETIME
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func seedClusterRebuildGap(t *testing.T, db *gorm.DB, id, workspaceID, summary string, evidenceCount int, createdAt time.Time) {
	t.Helper()
	gap := model.SupportCoverageGap{
		ID:            id,
		WorkspaceID:   workspaceID,
		DedupeKey:     "old-" + id,
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryUnknown,
		V1GapType:     model.SupportCoverageV1GapNeedsReview,
		Title:         summary,
		Status:        model.SupportCoverageGapStatusOpen,
		EvidenceCount: evidenceCount,
		Metadata:      []byte("{}"),
		FirstSeenAt:   createdAt,
		LastSeenAt:    createdAt,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}
	if err := db.Create(&gap).Error; err != nil {
		t.Fatalf("seed gap: %v", err)
	}
	evidence := model.SupportGapEvidence{
		ID:           "ev-" + id,
		GapID:        id,
		WorkspaceID:  workspaceID,
		EvidenceType: model.SupportEventWidgetSearchPerformed,
		Excerpt:      summary,
		Metadata:     []byte("{}"),
		CreatedAt:    createdAt,
	}
	if err := db.Create(&evidence).Error; err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
}
