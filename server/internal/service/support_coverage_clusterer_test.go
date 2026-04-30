package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeForCluster(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"lowercases and strips punct", "How DO I reset, my password?!", "password reset"},
		{"sorts tokens", "billing late charge", "billing charge late"},
		{"removes stopwords", "the quick brown fox over the lazy dog", "brown dog fox lazy over quick"},
		{"deduplicates tokens", "password password password", "password"},
		{"unicode preserved", "Café résumé", "café résumé"},
		{"strips trailing whitespace", "  hello   world  ", "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeForCluster(tt.in)
			if got != tt.want {
				t.Errorf("normalizeForCluster(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestComputeClusterKey_NoCrossSignalCollision(t *testing.T) {
	a := computeClusterKey("ws-1", "human_reply_after_ai", "", "", "How do I reset my password?")
	b := computeClusterKey("ws-1", "article_feedback", "", "", "How do I reset my password?")
	if a == b {
		t.Fatalf("different signal types must produce different cluster keys: %q == %q", a, b)
	}
}

func TestComputeClusterKey_NoCrossDocumentCollision(t *testing.T) {
	a := computeClusterKey("ws-1", "article_feedback", "doc-1", "", "Confusing")
	b := computeClusterKey("ws-1", "article_feedback", "doc-2", "", "Confusing")
	if a == b {
		t.Fatalf("article-feedback gaps on different docs must not collide: %q == %q", a, b)
	}
}

func TestComputeClusterKey_StableAcrossWordOrder(t *testing.T) {
	a := computeClusterKey("ws-1", "human_reply_after_ai", "", "", "How do I reset my password?")
	b := computeClusterKey("ws-1", "human_reply_after_ai", "", "", "how to reset password")
	if a != b {
		t.Fatalf("normalized variants of same question must collide: %q != %q", a, b)
	}
}

func TestComputeClusterKey_DifferentWorkspacesIsolated(t *testing.T) {
	a := computeClusterKey("ws-1", "human_reply_after_ai", "", "", "billing")
	b := computeClusterKey("ws-2", "human_reply_after_ai", "", "", "billing")
	if a == b {
		t.Fatalf("workspaces must be isolated: %q == %q", a, b)
	}
}

func TestSupportCoverageClusterer_UpsertTopicGapCreatesOnFirstCall(t *testing.T) {
	db := setupCoverageClustererTestDB(t)
	c := NewSupportCoverageClusterer(repository.NewSupportCoverageRepository(db))

	gap, err := c.UpsertTopicGap(context.Background(), &model.SupportEvent{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventHumanReplyAfterAI,
		IssueSummary: "How do I reset my password?",
	})
	if err != nil {
		t.Fatalf("UpsertTopicGap: %v", err)
	}
	if gap.ID == "" {
		t.Fatal("expected gap ID")
	}
	if gap.TopicID == nil || *gap.TopicID == "" {
		t.Fatal("expected topic ID")
	}
	if gap.EvidenceCount != 1 {
		t.Errorf("evidence_count=%d, want 1", gap.EvidenceCount)
	}
	if gap.GapKind != "content" {
		t.Errorf("gap_kind=%q, want content", gap.GapKind)
	}
	var metadata struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(gap.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata.Source != model.SupportCoverageGapSourceEventDetection {
		t.Fatalf("metadata source=%q, want event_detection", metadata.Source)
	}
}

func TestSupportCoverageClusterer_UpsertTopicGapSecondCallReusesOpenGap(t *testing.T) {
	db := setupCoverageClustererTestDB(t)
	c := NewSupportCoverageClusterer(repository.NewSupportCoverageRepository(db))
	ctx := context.Background()
	ev := &model.SupportEvent{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventHumanReplyAfterAI,
		IssueSummary: "How do I reset my password?",
	}

	g1, err := c.UpsertTopicGap(ctx, ev)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	g2, err := c.UpsertTopicGap(ctx, ev)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if g1.ID != g2.ID {
		t.Fatalf("expected same gap, got %s vs %s", g1.ID, g2.ID)
	}
	if g2.EvidenceCount != 2 {
		t.Errorf("evidence_count=%d, want 2", g2.EvidenceCount)
	}
}

func TestSupportCoverageClusterer_UpsertTopicGapDoneGapDoesNotBlockNewOpen(t *testing.T) {
	db := setupCoverageClustererTestDB(t)
	repo := repository.NewSupportCoverageRepository(db)
	c := NewSupportCoverageClusterer(repo)
	ctx := context.Background()
	ev := &model.SupportEvent{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventHumanReplyAfterAI,
		IssueSummary: "How do I reset my password?",
	}

	g1, err := c.UpsertTopicGap(ctx, ev)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.UpdateGapStatus(ctx, "ws-1", g1.ID, model.SupportCoverageGapStatusDone, "user-1", nil); err != nil {
		t.Fatalf("mark done: %v", err)
	}
	g2, err := c.UpsertTopicGap(ctx, ev)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if g1.ID == g2.ID {
		t.Fatal("done gap must not be reused; expected new open gap")
	}
	if g2.Status != model.SupportCoverageGapStatusOpen {
		t.Errorf("new gap status=%q, want open", g2.Status)
	}
}

func setupCoverageClustererTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:support_coverage_clusterer_%d?mode=memory&cache=shared", time.Now().UnixNano())
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
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
			first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			status_changed_by TEXT,
			status_changed_at DATETIME,
			issue_resolved BOOLEAN,
			closed_at DATETIME,
			closed_evidence_count INTEGER,
			result_document_id TEXT,
			rejection_reason TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create fixture schema: %v", err)
		}
	}
	return db
}
