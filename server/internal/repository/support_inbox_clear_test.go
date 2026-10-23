package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupSupportInboxClearTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:support_inbox_clear_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE support_inbox_clears (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			scope_key TEXT NOT NULL,
			mailbox_id TEXT,
			conversation_id TEXT NOT NULL,
			local_date TEXT NOT NULL,
			cleared_at DATETIME NOT NULL,
			created_at DATETIME
		)`,
		`CREATE UNIQUE INDEX idx_support_inbox_clears_once ON support_inbox_clears(workspace_id, user_id, scope_key, conversation_id)`,
		`CREATE TABLE support_conversations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			mailbox_id TEXT,
			status TEXT NOT NULL DEFAULT 'open'
		)`,
		`CREATE TABLE support_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			sender_user_id TEXT,
			message_type TEXT NOT NULL DEFAULT 'reply',
			system_event_type TEXT,
			created_at DATETIME NOT NULL
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func insertSupportInboxClearConversation(t *testing.T, db *gorm.DB, id, workspaceID, mailboxID, status string) {
	t.Helper()
	var mailbox any
	if mailboxID != "" {
		mailbox = mailboxID
	}
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, mailbox_id, status) VALUES (?, ?, ?, ?)`,
		id, workspaceID, mailbox, status).Error; err != nil {
		t.Fatalf("insert conversation %s: %v", id, err)
	}
}

func insertSupportInboxClearMessage(t *testing.T, db *gorm.DB, id, workspaceID, conversationID, userID, messageType, eventType string, at time.Time) {
	t.Helper()
	var event any
	if eventType != "" {
		event = eventType
	}
	if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_user_id, message_type, system_event_type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, workspaceID, conversationID, userID, messageType, event, at.UTC()).Error; err != nil {
		t.Fatalf("insert message %s: %v", id, err)
	}
}

func TestSupportInboxClearRepository_CreateIfAbsentRecordsOnce(t *testing.T) {
	repo := NewSupportInboxClearRepository(setupSupportInboxClearTestDB(t))
	ctx := context.Background()
	clearedAt := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	first := &model.SupportInboxClear{ID: "clear-1", WorkspaceID: "ws-1", UserID: "user-1", ScopeKey: "shared", ConversationID: "conv-1", LocalDate: "2026-10-23", ClearedAt: clearedAt}
	inserted, err := repo.CreateIfAbsent(ctx, first)
	if err != nil || !inserted {
		t.Fatalf("first insert = %v, %v; want inserted", inserted, err)
	}
	repeat := &model.SupportInboxClear{ID: "clear-2", WorkspaceID: "ws-1", UserID: "user-1", ScopeKey: "shared", ConversationID: "conv-1", LocalDate: "2026-10-23", ClearedAt: clearedAt.Add(time.Minute)}
	inserted, err = repo.CreateIfAbsent(ctx, repeat)
	if err != nil || inserted {
		t.Fatalf("repeat insert = %v, %v; want skipped", inserted, err)
	}
	found, err := repo.FindClear(ctx, "ws-1", "user-1", "shared", "conv-1")
	if err != nil || found == nil || found.ID != "clear-1" {
		t.Fatalf("FindClear = %+v, %v; want the first clear", found, err)
	}
	missing, err := repo.FindClear(ctx, "ws-1", "user-1", "mine", "conv-1")
	if err != nil || missing != nil {
		t.Fatalf("FindClear other scope = %+v, %v; want nil", missing, err)
	}
}

func TestSupportInboxClearRepository_ClearHistory(t *testing.T) {
	repo := NewSupportInboxClearRepository(setupSupportInboxClearTestDB(t))
	ctx := context.Background()
	base := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	for _, entry := range []model.SupportInboxClear{
		{ID: "c1", WorkspaceID: "ws-1", UserID: "user-1", ScopeKey: "shared", ConversationID: "conv-1", LocalDate: "2026-10-21", ClearedAt: base.Add(-48 * time.Hour)},
		{ID: "c2", WorkspaceID: "ws-1", UserID: "user-1", ScopeKey: "shared", ConversationID: "conv-2", LocalDate: "2026-10-23", ClearedAt: base},
		{ID: "c3", WorkspaceID: "ws-1", UserID: "user-1", ScopeKey: "mine", ConversationID: "conv-3", LocalDate: "2026-10-22", ClearedAt: base.Add(-24 * time.Hour)},
		{ID: "c4", WorkspaceID: "ws-1", UserID: "user-1", ScopeKey: "shared", ConversationID: "conv-4", LocalDate: "2026-10-23", ClearedAt: base.Add(time.Hour)},
		{ID: "c5", WorkspaceID: "ws-1", UserID: "user-2", ScopeKey: "shared", ConversationID: "conv-5", LocalDate: "2026-10-23", ClearedAt: base.Add(30 * time.Minute)},
	} {
		entry := entry
		if _, err := repo.CreateIfAbsent(ctx, &entry); err != nil {
			t.Fatal(err)
		}
	}

	count, err := repo.CountClearsOnDate(ctx, "ws-1", "user-1", "shared", "2026-10-23")
	if err != nil || count != 2 {
		t.Fatalf("CountClearsOnDate = %d, %v; want 2", count, err)
	}
	dates, err := repo.ListClearDates(ctx, "ws-1", "user-1", 10)
	if err != nil || fmt.Sprint(dates) != "[2026-10-23 2026-10-22 2026-10-21]" {
		t.Fatalf("ListClearDates = %v, %v", dates, err)
	}
	previous, err := repo.PreviousClearAt(ctx, "ws-1", "user-1", "shared", base.Add(time.Hour), "conv-4")
	if err != nil || previous == nil || !previous.Equal(base) {
		t.Fatalf("PreviousClearAt = %v, %v; want %v", previous, err, base)
	}
	none, err := repo.PreviousClearAt(ctx, "ws-1", "user-1", "mine", base.Add(-24*time.Hour), "conv-3")
	if err != nil || none != nil {
		t.Fatalf("PreviousClearAt first clear = %v, %v; want nil", none, err)
	}
}

func TestSupportInboxClearRepository_CountResolvedBy(t *testing.T) {
	db := setupSupportInboxClearTestDB(t)
	repo := NewSupportInboxClearRepository(db)
	ctx := context.Background()
	since := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	insertSupportInboxClearConversation(t, db, "conv-billing-1", "ws-1", "mb-billing", "resolved")
	insertSupportInboxClearConversation(t, db, "conv-billing-2", "ws-1", "mb-billing", "resolved")
	insertSupportInboxClearConversation(t, db, "conv-sales", "ws-1", "mb-sales", "resolved")
	insertSupportInboxClearConversation(t, db, "conv-shared", "ws-1", "", "resolved")
	insertSupportInboxClearMessage(t, db, "m9", "ws-1", "conv-shared", "user-1", "system", "resolved", since.Add(6*time.Hour))
	insertSupportInboxClearMessage(t, db, "m1", "ws-1", "conv-billing-1", "user-1", "system", "resolved", since.Add(time.Hour))
	insertSupportInboxClearMessage(t, db, "m2", "ws-1", "conv-billing-1", "user-1", "system", "resolved", since.Add(2*time.Hour)) // resolved again: counted once
	insertSupportInboxClearMessage(t, db, "m3", "ws-1", "conv-billing-2", "user-1", "system", "resolved", since.Add(3*time.Hour))
	insertSupportInboxClearMessage(t, db, "m4", "ws-1", "conv-sales", "user-1", "system", "resolved", since.Add(4*time.Hour))
	insertSupportInboxClearMessage(t, db, "m5", "ws-1", "conv-billing-2", "user-1", "system", "reopened", since.Add(5*time.Hour))
	insertSupportInboxClearMessage(t, db, "m6", "ws-1", "conv-billing-2", "user-1", "reply", "", since.Add(5*time.Hour))
	insertSupportInboxClearMessage(t, db, "m7", "ws-1", "conv-sales", "user-2", "system", "resolved", since.Add(5*time.Hour))
	insertSupportInboxClearMessage(t, db, "m8", "ws-1", "conv-sales", "user-1", "system", "resolved", since.Add(-time.Hour))

	all, err := repo.CountResolvedBy(ctx, "ws-1", "user-1", since, model.SupportInboxClearCount{})
	if err != nil || all != 4 {
		t.Fatalf("CountResolvedBy all = %d, %v; want 4", all, err)
	}
	shared, err := repo.CountResolvedBy(ctx, "ws-1", "user-1", since, model.SupportInboxClearCount{SharedOnly: true})
	if err != nil || shared != 1 {
		t.Fatalf("CountResolvedBy shared = %d, %v; want 1", shared, err)
	}
	mailbox := "mb-billing"
	billing, err := repo.CountResolvedBy(ctx, "ws-1", "user-1", since, model.SupportInboxClearCount{MailboxID: &mailbox})
	if err != nil || billing != 2 {
		t.Fatalf("CountResolvedBy billing = %d, %v; want 2", billing, err)
	}
	later, err := repo.CountResolvedBy(ctx, "ws-1", "user-1", since.Add(150*time.Minute), model.SupportInboxClearCount{MailboxID: &mailbox})
	if err != nil || later != 1 {
		t.Fatalf("CountResolvedBy billing later = %d, %v; want 1", later, err)
	}
}

func TestSupportInboxClearRepository_FindConversationStatus(t *testing.T) {
	db := setupSupportInboxClearTestDB(t)
	repo := NewSupportInboxClearRepository(db)
	insertSupportInboxClearConversation(t, db, "conv-1", "ws-1", "", "resolved")
	status, found, err := repo.FindConversationStatus(context.Background(), "ws-1", "conv-1")
	if err != nil || !found || status != "resolved" {
		t.Fatalf("FindConversationStatus = %q, %v, %v", status, found, err)
	}
	_, found, err = repo.FindConversationStatus(context.Background(), "ws-2", "conv-1")
	if err != nil || found {
		t.Fatalf("FindConversationStatus other workspace found = %v, %v; want not found", found, err)
	}
}
