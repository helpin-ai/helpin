package repository

import (
	"fmt"
	"testing"
	"time"

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
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX idx_support_coverage_gaps_workspace_dedupe
			ON support_coverage_gaps (workspace_id, dedupe_key)
			WHERE status != 'merged'`,
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

func TestSupportCoverageRepository_SetupCompiles(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	repo := NewSupportCoverageRepository(db)
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}
