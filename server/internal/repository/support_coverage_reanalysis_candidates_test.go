package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCoverageReanalysisIncludesTheEntireUnresolvedFailureBacklog(t *testing.T) {
	db := setupSupportInboxCoverageAnalysisTestDB(t)
	if err := db.Exec(`CREATE TABLE coverage_analysis_attempts (
		id TEXT PRIMARY KEY, workspace_id TEXT, source_kind TEXT, source_id TEXT,
		segment_id TEXT, content_hash TEXT, status TEXT, attempt INTEGER, created_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	old := now.Add(-60 * 24 * time.Hour)
	// More than the health endpoint's 25-row limit and outside the 30-day window.
	for i := 0; i < 31; i++ {
		id := fmt.Sprintf("failed-%02d", i)
		status := []string{model.CoverageAttemptDeadLetter, model.CoverageAttemptPausedConfiguration, model.CoverageAttemptRetryable}[i%3]
		insertCoverageCandidateConversation(t, db, id, "ws-1", model.SupportConversationStatusResolved, old, &old, nil)
		if err := db.Exec(`INSERT INTO coverage_analysis_attempts VALUES (?, 'ws-1', 'conversation', ?, 'old-segment', 'old-hash', ?, 1, ?)`, id, id, status, old).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ id, ws, status string }{
		{"recovered", "ws-1", model.SupportConversationStatusResolved},
		{"sample", "ws-1", model.SupportConversationStatusResolved},
		{"spam", "ws-1", model.SupportConversationStatusSpam},
		{"other-workspace", "ws-2", model.SupportConversationStatusResolved},
	} {
		insertCoverageCandidateConversation(t, db, row.id, row.ws, row.status, old, &old, nil)
		if err := db.Exec(`INSERT INTO coverage_analysis_attempts VALUES (?, ?, 'conversation', ?, 'old-segment', 'old-hash', 'dead_letter', 1, ?)`, row.id, row.ws, row.id, old).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO coverage_analysis_attempts VALUES ('recovered-new', 'ws-1', 'conversation', 'recovered', 'new-segment', 'new-hash', 'succeeded', 1, ?)`, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO sample_data_items (id, workspace_id, entity_type, entity_id) VALUES ('sample-item', 'ws-1', 'support_conversation', 'sample')`).Error; err != nil {
		t.Fatal(err)
	}
	insertCoverageCandidateConversation(t, db, "recent", "ws-1", model.SupportConversationStatusResolved, now.Add(-time.Hour), nil, nil)
	repo := NewSupportConversationRepository(db)
	var ids []string
	var cursor *CoverageAnalysisCandidateCursor
	for {
		conversations, next, err := repo.ListCoverageReanalysisCandidatesPage(context.Background(), "ws-1", now.Add(-30*24*time.Hour), now, 7*24*time.Hour, cursor, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, conversation := range conversations {
			ids = append(ids, conversation.ID)
		}
		if next == nil {
			break
		}
		cursor = next
	}
	if len(ids) != 32 {
		t.Fatalf("reanalysis selected %d conversations (%v), want 31 failed conversations plus the recent conversation", len(ids), ids)
	}
	for i := 0; i < 31; i++ {
		if ids[i] != fmt.Sprintf("failed-%02d", i) {
			t.Fatalf("unexpected candidate %q", ids[i])
		}
	}
	if ids[31] != "recent" {
		t.Fatalf("last candidate = %q, want recent", ids[31])
	}
}
