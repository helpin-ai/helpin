package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupHelpcenterAnswerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:hc_answer_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE helpcenter_answers (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		cache_key TEXT NOT NULL UNIQUE,
		locale TEXT NOT NULL DEFAULT 'en',
		space_slug TEXT,
		query TEXT NOT NULL,
		status TEXT NOT NULL,
		answer TEXT,
		citations BLOB NOT NULL DEFAULT '[]',
		confidence REAL,
		tokens_used INTEGER,
		helpful_count INTEGER NOT NULL DEFAULT 0,
		not_helpful_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestHelpcenterAnswerRepositoryLifecycle(t *testing.T) {
	db := setupHelpcenterAnswerTestDB(t)
	repo := NewHelpcenterAnswerRepository(db)
	ctx := context.Background()

	answer := &model.HelpcenterAnswer{
		WorkspaceID: "ws-1",
		CacheKey:    "key-1",
		Locale:      "en",
		Query:       "how much does it cost",
		Status:      model.HelpcenterAnswerStatusAnswered,
		Answer:      "The Growth plan costs $84 per month.",
		Citations:   json.RawMessage(`[{"document_id":"doc-1"}]`),
		Confidence:  0.9,
	}
	if err := repo.Upsert(ctx, answer); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if answer.ID == "" {
		t.Fatal("expected upsert to populate the answer ID")
	}

	// Second upsert with the same cache key keeps the first row.
	duplicate := &model.HelpcenterAnswer{
		WorkspaceID: "ws-1",
		CacheKey:    "key-1",
		Query:       "how much does it cost",
		Status:      model.HelpcenterAnswerStatusInsufficientEvidence,
		Citations:   json.RawMessage(`[]`),
	}
	if err := repo.Upsert(ctx, duplicate); err != nil {
		t.Fatalf("duplicate upsert: %v", err)
	}
	if duplicate.ID != answer.ID || duplicate.Status != model.HelpcenterAnswerStatusAnswered {
		t.Fatalf("duplicate upsert = %+v, want first row returned", duplicate)
	}

	cached, err := repo.GetByCacheKey(ctx, "ws-1", "key-1")
	if err != nil || cached == nil || cached.Answer != answer.Answer {
		t.Fatalf("GetByCacheKey() = %+v (err %v)", cached, err)
	}
	if miss, err := repo.GetByCacheKey(ctx, "ws-1", "other"); err != nil || miss != nil {
		t.Fatalf("cache miss = %+v (err %v), want nil", miss, err)
	}
	if crossWorkspace, err := repo.GetByCacheKey(ctx, "ws-2", "key-1"); err != nil || crossWorkspace != nil {
		t.Fatalf("cross-workspace read = %+v (err %v), want nil", crossWorkspace, err)
	}

	if err := repo.IncrementFeedback(ctx, "ws-1", answer.ID, true); err != nil {
		t.Fatalf("feedback: %v", err)
	}
	if err := repo.IncrementFeedback(ctx, "ws-1", answer.ID, false); err != nil {
		t.Fatalf("feedback: %v", err)
	}
	updated, err := repo.GetByID(ctx, "ws-1", answer.ID)
	if err != nil || updated == nil {
		t.Fatalf("GetByID() err = %v", err)
	}
	if updated.HelpfulCount != 1 || updated.NotHelpfulCount != 1 {
		t.Fatalf("feedback counts = %d/%d, want 1/1", updated.HelpfulCount, updated.NotHelpfulCount)
	}
}
