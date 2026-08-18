package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSupportMessagePaginationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:support_message_pagination_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_messages (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		conversation_id TEXT NOT NULL,
		is_internal BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create support_messages: %v", err)
	}
	return db
}

func TestSupportMessageRepositoryListConversationPageBeforeReturnsNewestChronologically(t *testing.T) {
	db := setupSupportMessagePaginationTestDB(t)
	for i := 1; i <= 25; i++ {
		createdAt := time.Date(2026, 8, 18, 10, i, 0, 0, time.UTC)
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, created_at) VALUES (?, ?, ?, ?)`,
			fmt.Sprintf("msg-%02d", i), "ws-1", "conv-1", createdAt,
		).Error; err != nil {
			t.Fatalf("insert message %d: %v", i, err)
		}
	}

	repo := NewSupportMessageRepository(db)
	messages, hasMore, err := repo.ListConversationPageBefore(
		context.Background(), "ws-1", "conv-1", true, 20, nil, "",
	)
	if err != nil {
		t.Fatalf("list newest page: %v", err)
	}
	if len(messages) != 20 {
		t.Fatalf("message count = %d, want 20", len(messages))
	}
	if !hasMore {
		t.Fatal("hasMore = false, want true")
	}
	if messages[0].ID != "msg-06" || messages[19].ID != "msg-25" {
		t.Fatalf("page bounds = %s..%s, want msg-06..msg-25", messages[0].ID, messages[19].ID)
	}
}

func TestSupportMessageRepositoryListConversationPageBeforeUsesStableCursor(t *testing.T) {
	db := setupSupportMessagePaginationTestDB(t)
	sharedTime := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	for _, id := range []string{"msg-a", "msg-b", "msg-c"} {
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, created_at) VALUES (?, ?, ?, ?)`,
			id, "ws-1", "conv-1", sharedTime,
		).Error; err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}

	repo := NewSupportMessageRepository(db)
	first, hasMore, err := repo.ListConversationPageBefore(
		context.Background(), "ws-1", "conv-1", true, 2, nil, "",
	)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if !hasMore || len(first) != 2 || first[0].ID != "msg-b" || first[1].ID != "msg-c" {
		t.Fatalf("first page = %#v, hasMore = %v", first, hasMore)
	}

	older, olderHasMore, err := repo.ListConversationPageBefore(
		context.Background(), "ws-1", "conv-1", true, 2, &first[0].CreatedAt, first[0].ID,
	)
	if err != nil {
		t.Fatalf("list older page: %v", err)
	}
	if olderHasMore || len(older) != 1 || older[0].ID != "msg-a" {
		t.Fatalf("older page = %#v, hasMore = %v", older, olderHasMore)
	}
}
