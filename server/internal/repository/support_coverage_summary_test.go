package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func coverageFilterFromJSON(t *testing.T, value string) model.SupportCoverageGapFilter {
	t.Helper()
	var filter model.SupportCoverageGapFilter
	if err := json.Unmarshal([]byte(value), &filter); err != nil {
		t.Fatal(err)
	}
	return filter
}

func TestCoverageEarlierDetectionsCanBeReviewed(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	if err := db.Exec(`CREATE TABLE support_coverage_analysis_runs (id TEXT PRIMARY KEY, workspace_id TEXT, status TEXT, completed_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	for _, seed := range []struct{ id, workspace, status, source string }{
		{"hidden", "ws-1", "open", "event_detection"},
		{"visible", "ws-1", "open", "daily_conversation_analysis"},
		{"closed", "ws-1", "rejected", "event_detection"},
		{"foreign", "ws-2", "open", "event_detection"},
	} {
		if err := db.Exec(`INSERT INTO support_coverage_gaps (id, workspace_id, dedupe_key, status, metadata, confidence) VALUES (?, ?, ?, ?, ?, 0.4)`, seed.id, seed.workspace, seed.id, seed.status, []byte(`{"source":"`+seed.source+`"}`)).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewSupportCoverageRepository(db)
	filter := coverageFilterFromJSON(t, `{"review_only":true,"status":"open"}`)
	items, total, err := repo.ListGaps(context.Background(), "ws-1", filter)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "hidden" {
		t.Fatalf("review list: total %d, items %+v", total, items)
	}
	summary, err := repo.GetSummary(context.Background(), "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	var counts map[string]any
	if err := json.Unmarshal(data, &counts); err != nil {
		t.Fatal(err)
	}
	if counts["unreviewed_detection_count"] != float64(1) {
		t.Fatalf("review count: %s", data)
	}
	if err := repo.ReclassifyGap(context.Background(), "ws-1", "hidden", "missing_article"); err != nil {
		t.Fatal(err)
	}
	_, total, err = repo.ListGaps(context.Background(), "ws-1", filter)
	if err != nil || total != 0 {
		t.Fatalf("classified detection remains in review: total %d, err %v", total, err)
	}
}

func TestCoverageSummaryMatchesVisibleGaps(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	if err := db.Exec(`CREATE TABLE support_coverage_analysis_runs (id TEXT PRIMARY KEY, workspace_id TEXT, status TEXT, completed_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, seed := range []struct {
		id, workspace, status, source, kind string
		confidence                          float64
	}{
		{"visible", "ws-1", "open", "event_detection", "missing_article", 0.4},
		{"hidden", "ws-1", "open", "event_detection", "needs_review", 0.4},
		{"daily", "ws-1", "open", "daily_conversation_analysis", "needs_review", 0.4},
		{"threshold", "ws-1", "open", "event_detection", "needs_review", 0.7},
		{"done", "ws-1", "done", "event_detection", "missing_article", 0.8},
		{"hidden-done", "ws-1", "done", "event_detection", "needs_review", 0.4},
		{"merged", "ws-1", "merged", "daily_conversation_analysis", "missing_article", 0.8},
		{"other", "ws-2", "open", "daily_conversation_analysis", "missing_article", 0.8},
	} {
		if err := db.Exec(`INSERT INTO support_coverage_gaps
			(id, workspace_id, dedupe_key, status, metadata, v1_gap_type, confidence, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, seed.id, seed.workspace, seed.id, seed.status,
			[]byte(`{"source":"`+seed.source+`"}`), seed.kind, seed.confidence, now, now).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO support_gap_evidence (id, workspace_id, gap_id, evidence_type) VALUES (?, ?, ?, 'test')`, seed.id, seed.workspace, seed.id).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewSupportCoverageRepository(db)
	summary, err := repo.GetSummary(context.Background(), "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalOpenGaps != 3 || summary.NewGapsThisWeek != 4 || summary.GapsFixedThisWeek != 1 || summary.TotalEvidenceCount != 4 {
		t.Fatalf("summary includes hidden, merged, or other-workspace records: %+v", summary)
	}
}

func TestCoverageGapListFiltersSourceConversation(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	for _, id := range []string{"match", "unrelated", "foreign-evidence"} {
		if err := db.Exec(`INSERT INTO support_coverage_gaps (id, workspace_id, dedupe_key, v1_gap_type, metadata) VALUES (?, 'ws-1', ?, 'missing_article', ?)`, id, id, []byte(`{}`)).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, seed := range []struct{ id, workspace, gap, conversation string }{
		{"e1", "ws-1", "match", "conv-1"}, {"e2", "ws-1", "unrelated", "conv-2"}, {"e3", "ws-2", "foreign-evidence", "conv-1"},
	} {
		if err := db.Exec(`INSERT INTO support_gap_evidence (id, workspace_id, gap_id, conversation_id, evidence_type) VALUES (?, ?, ?, ?, 'test')`, seed.id, seed.workspace, seed.gap, seed.conversation).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Decode through JSON so this regression fails behaviorally before the DTO field exists.
	filter := coverageFilterFromJSON(t, `{"conversation_id":"conv-1","status":"open"}`)
	items, total, err := NewSupportCoverageRepository(db).ListGaps(context.Background(), "ws-1", filter)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "match" {
		t.Fatalf("got total %d, items %+v", total, items)
	}
}

func TestCoverageSummaryReturnsDatabaseErrors(t *testing.T) {
	db := setupSupportCoverageTestDB(t)
	if err := db.Exec("DROP TABLE support_coverage_gaps").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewSupportCoverageRepository(db).GetSummary(context.Background(), "ws-1"); err == nil {
		t.Fatal("expected database error rather than misleading zero counts")
	}
}
