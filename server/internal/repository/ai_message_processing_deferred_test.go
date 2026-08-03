package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProcessingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:ai_processing_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE ai_message_processing (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		source_message_id TEXT NOT NULL UNIQUE,
		conversation_id TEXT NOT NULL,
		reply_message_id TEXT,
		status TEXT NOT NULL DEFAULT 'processing',
		attempts INTEGER NOT NULL DEFAULT 1,
		tokens_used INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestAIMessageProcessingDeferredLifecycle(t *testing.T) {
	db := setupProcessingTestDB(t)
	repo := NewAIMessageProcessingRepository(db)
	ctx := context.Background()

	first, ok := repo.BeginAttempt(ctx, "ws-1", "msg-1", "conv-1")
	if !ok || first == nil {
		t.Fatal("begin first attempt")
	}
	second, ok := repo.BeginAttempt(ctx, "ws-1", "msg-2", "conv-1")
	if !ok || second == nil {
		t.Fatal("begin second attempt")
	}

	// Park the second message (arrived mid-turn), keep the first processing.
	if err := repo.MarkDeferred(ctx, second.ID); err != nil {
		t.Fatalf("mark deferred: %v", err)
	}

	inFlight, err := repo.LatestProcessingForConversation(ctx, "ws-1", "conv-1")
	if err != nil || inFlight == nil || inFlight.ID != first.ID {
		t.Fatalf("latest processing = %+v (err %v), want first row", inFlight, err)
	}

	deferred, err := repo.ListDeferredForConversation(ctx, "ws-1", "conv-1")
	if err != nil || len(deferred) != 1 || deferred[0].ID != second.ID {
		t.Fatalf("deferred = %+v (err %v), want second row", deferred, err)
	}

	// Nudge marker: attempts increment.
	if err := repo.IncrementAttempts(ctx, first.ID); err != nil {
		t.Fatalf("increment attempts: %v", err)
	}
	inFlight, _ = repo.LatestProcessingForConversation(ctx, "ws-1", "conv-1")
	if inFlight.Attempts != 2 {
		t.Errorf("attempts = %d, want 2", inFlight.Attempts)
	}

	// Drain: reclaim the deferred row as the new in-flight turn.
	if err := repo.MarkCompleted(ctx, first.ID, nil, 0); err != nil {
		t.Fatalf("complete first: %v", err)
	}
	if err := repo.MarkProcessing(ctx, second.ID); err != nil {
		t.Fatalf("reclaim second: %v", err)
	}
	inFlight, _ = repo.LatestProcessingForConversation(ctx, "ws-1", "conv-1")
	if inFlight == nil || inFlight.ID != second.ID {
		t.Fatalf("latest processing after drain = %+v, want second row", inFlight)
	}

	// Sweep listing honors the cutoff.
	stale, err := repo.ListDeferredOlderThan(ctx, time.Now().Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("list deferred older than: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("expected no deferred rows after drain, got %d", len(stale))
	}
}
