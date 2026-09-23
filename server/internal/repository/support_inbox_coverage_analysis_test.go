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

func setupSupportInboxCoverageAnalysisTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:support_inbox_coverage_analysis_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_conversations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		mailbox_id TEXT,
		display_id INTEGER NOT NULL DEFAULT 0,
		subject TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'open',
		flow_state TEXT,
		priority TEXT NOT NULL DEFAULT 'medium',
		channel TEXT NOT NULL DEFAULT 'widget',
		customer_name TEXT,
		customer_email TEXT,
		customer_phone TEXT,
		opened_by_user_id TEXT,
		assigned_user_id TEXT,
		assigned_agent_id TEXT,
		linked_task_id TEXT,
		source TEXT NOT NULL DEFAULT 'internal',
		anonymous_id TEXT,
		crm_contact_id TEXT,
		resolved_at DATETIME,
		closed_at DATETIME,
		team_last_seen_at DATETIME,
		contact_last_seen_at DATETIME,
		email_unsubscribed BOOLEAN NOT NULL DEFAULT 0,
		ai_state TEXT,
		ai_resolved_at DATETIME,
		ai_escalated_at DATETIME,
		ai_resolution_type TEXT,
		ai_turn_count INTEGER NOT NULL DEFAULT 0,
		customer_requested_human_at DATETIME,
		ai_active_run_id TEXT,
		human_takeover BOOLEAN DEFAULT 0,
            ai_control_version BIGINT NOT NULL DEFAULT 0, ai_resumed_at DATETIME, ai_paused_at DATETIME, ai_paused_by_user_id TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create support_conversations: %v", err)
	}
	if err := db.Exec(sampleDataItemsTestSchema).Error; err != nil {
		t.Fatalf("create sample_data_items: %v", err)
	}
	return db
}

func TestSupportConversationRepository_CoverageCandidatesExcludeSampleData(t *testing.T) {
	db := setupSupportInboxCoverageAnalysisTestDB(t)
	repo := NewSupportConversationRepository(db)
	now := time.Now().UTC()
	insertCoverageCandidateConversation(t, db, "real", "ws-1", "open", now, nil, nil)
	insertCoverageCandidateConversation(t, db, "sample", "ws-1", "open", now, nil, nil)
	if err := db.Exec(`INSERT INTO sample_data_items (id, workspace_id, entity_type, entity_id) VALUES ('item', 'ws-1', 'support_conversation', 'sample')`).Error; err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListCoverageAnalysisCandidates(context.Background(), "ws-1", now.Add(-time.Hour), now.Add(time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "real" {
		t.Fatalf("candidates = %+v, want only the real conversation", got)
	}
}

func insertCoverageCandidateConversation(t *testing.T, db *gorm.DB, id, workspaceID, status string, updatedAt time.Time, resolvedAt *time.Time, aiState *string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO support_conversations
		(id, workspace_id, display_id, subject, status, priority, channel, source, updated_at, created_at, resolved_at, ai_state)
		VALUES (?, ?, ?, ?, ?, 'medium', 'widget', 'widget', ?, ?, ?, ?)`,
		id, workspaceID, 1, id, status, updatedAt, updatedAt.Add(-time.Hour), resolvedAt, aiState,
	).Error; err != nil {
		t.Fatalf("insert conversation %s: %v", id, err)
	}
}

func TestSupportConversationRepository_ListCoverageAnalysisCandidates(t *testing.T) {
	db := setupSupportInboxCoverageAnalysisTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	windowStart := time.Date(2026, 4, 28, 4, 30, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 4, 29, 4, 30, 0, 0, time.UTC)
	aiState := "pending"
	resolvedInside := windowStart.Add(2 * time.Hour)
	resolvedOutside := windowStart.Add(-time.Minute)

	insertCoverageCandidateConversation(t, db, "updated-inside-human-only", "ws-1", model.SupportConversationStatusOpen, windowStart.Add(time.Hour), nil, nil)
	insertCoverageCandidateConversation(t, db, "resolved-inside-updated-before", "ws-1", model.SupportConversationStatusResolved, windowStart.Add(-time.Hour), &resolvedInside, nil)
	insertCoverageCandidateConversation(t, db, "spam-inside", "ws-1", model.SupportConversationStatusSpam, windowStart.Add(30*time.Minute), nil, nil)
	insertCoverageCandidateConversation(t, db, "updated-outside", "ws-1", model.SupportConversationStatusOpen, windowStart.Add(-time.Minute), nil, &aiState)
	insertCoverageCandidateConversation(t, db, "resolved-outside", "ws-1", model.SupportConversationStatusResolved, windowStart.Add(-2*time.Hour), &resolvedOutside, nil)
	insertCoverageCandidateConversation(t, db, "other-workspace", "ws-2", model.SupportConversationStatusOpen, windowStart.Add(15*time.Minute), nil, nil)
	insertCoverageCandidateConversation(t, db, "updated-inside-second", "ws-1", model.SupportConversationStatusOpen, windowStart.Add(90*time.Minute), nil, &aiState)

	conversations, err := repo.ListCoverageAnalysisCandidates(ctx, "ws-1", windowStart, windowEnd, 100)
	if err != nil {
		t.Fatalf("ListCoverageAnalysisCandidates: %v", err)
	}

	got := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		got = append(got, conversation.ID)
	}
	want := []string{"resolved-inside-updated-before", "updated-inside-human-only", "updated-inside-second"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestSupportConversationRepository_ListCoverageAnalysisCandidatesDefaultLimit(t *testing.T) {
	db := setupSupportInboxCoverageAnalysisTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	windowStart := time.Date(2026, 4, 28, 4, 30, 0, 0, time.UTC)
	windowEnd := windowStart.Add(24 * time.Hour)

	for i := 0; i < 3; i++ {
		insertCoverageCandidateConversation(t, db, fmt.Sprintf("conversation-%d", i), "ws-1", model.SupportConversationStatusOpen, windowStart.Add(time.Duration(i)*time.Minute), nil, nil)
	}

	conversations, err := repo.ListCoverageAnalysisCandidates(ctx, "ws-1", windowStart, windowEnd, 0)
	if err != nil {
		t.Fatalf("ListCoverageAnalysisCandidates default limit: %v", err)
	}
	if len(conversations) != 3 {
		t.Fatalf("expected all three conversations under default limit, got %d", len(conversations))
	}
}

func TestSupportConversationRepository_CoverageCandidateCursorPaginationHasNoGaps(t *testing.T) {
	db := setupSupportInboxCoverageAnalysisTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	windowStart := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(24 * time.Hour)
	for i := 0; i < 7; i++ {
		insertCoverageCandidateConversation(t, db, fmt.Sprintf("conversation-%02d", i), "ws-1", model.SupportConversationStatusOpen, windowStart.Add(time.Duration(i/2)*time.Minute), nil, nil)
	}
	var cursor *CoverageAnalysisCandidateCursor
	var got []string
	for {
		page, next, err := repo.ListCoverageAnalysisCandidatesPage(ctx, "ws-1", windowStart, windowEnd, cursor, 3)
		if err != nil {
			t.Fatal(err)
		}
		for _, conversation := range page {
			got = append(got, conversation.ID)
		}
		if next == nil {
			break
		}
		cursor = next
	}
	if len(got) != 7 {
		t.Fatalf("candidate count = %d, want 7 (%v)", len(got), got)
	}
	seen := map[string]bool{}
	for _, id := range got {
		if seen[id] {
			t.Fatalf("duplicate candidate %s", id)
		}
		seen[id] = true
	}
}

func TestSupportConversationRepository_CoverageCandidateLimitIsExplicit(t *testing.T) {
	repo := NewSupportConversationRepository(setupSupportInboxCoverageAnalysisTestDB(t))
	_, _, err := repo.ListCoverageAnalysisCandidatesPage(context.Background(), "ws-1", time.Now().Add(-time.Hour), time.Now(), nil, CoverageAnalysisCandidatePageMax+1)
	if err == nil {
		t.Fatal("expected oversized candidate page to be rejected")
	}
}
