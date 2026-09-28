package repository

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnqueueLatestPortalCustomerMessageOnlyOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:portal_ai_dispatch_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT, channel TEXT, portal_visible BOOLEAN, status TEXT)`,
		`CREATE TABLE support_messages (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, sender_type TEXT, message_type TEXT, is_internal BOOLEAN, created_at DATETIME)`,
		`CREATE TABLE ai_message_processing (source_message_id TEXT PRIMARY KEY)`,
		`CREATE TABLE support_portal_ai_dispatches (source_message_id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, mode_at_enqueue TEXT)`,
		`INSERT INTO support_conversations VALUES ('conv', 'ws', 'portal', true, 'open')`,
		`INSERT INTO support_messages VALUES ('first', 'ws', 'conv', 'customer', 'reply', false, '2026-09-28 10:00:00')`,
		`INSERT INTO support_messages VALUES ('latest', 'ws', 'conv', 'customer', 'reply', false, '2026-09-28 10:01:00')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if err := EnqueueLatestPortalCustomerMessage(ctx, db, "ws", "conv", "ai_first"); err != nil {
		t.Fatal(err)
	}
	var source string
	if err := db.Table("support_portal_ai_dispatches").Select("source_message_id").Scan(&source).Error; err != nil {
		t.Fatal(err)
	}
	if source != "latest" {
		t.Fatalf("queued %q, want latest", source)
	}
	if err := EnqueueLatestPortalCustomerMessage(ctx, db, "ws", "conv", "ai_first"); !errors.Is(err, ErrPortalAINoPendingCustomerMessage) {
		t.Fatalf("duplicate enqueue = %v", err)
	}
}
