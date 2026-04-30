package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupSupportMessageTestDB creates an in-memory SQLite DB with the
// support_messages schema needed for the message-actions repository tests.
// It mirrors the production schema closely enough that GORM's reflection
// over model.SupportMessage works against it (column names + nullability).
func setupSupportMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:support_message_actions_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_messages (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		conversation_id TEXT,
		sender_type TEXT NOT NULL,
		message_type TEXT NOT NULL DEFAULT 'reply',
		system_event_type TEXT,
		sender_user_id TEXT,
		sender_agent_id TEXT,
		sender_display_name TEXT,
		sender_avatar_url TEXT,
		content TEXT NOT NULL,
		is_internal BOOLEAN NOT NULL DEFAULT 0,
		metadata TEXT NOT NULL DEFAULT '{}',
		via_channel TEXT,
		email_notified_at DATETIME,
		email_read_at DATETIME,
		cancellable_until DATETIME,
		deleted_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create support_messages: %v", err)
	}
	return db
}

// insertMessage is a small helper for the tests below — it inserts a row
// directly via SQL so we can vary the columns the repo cares about
// (sender_type, sender_user_id, message_type, is_internal) without
// dragging in every other field on the model.
func insertMessage(t *testing.T, db *gorm.DB, m model.SupportMessage) {
	t.Helper()
	if m.MessageType == "" {
		m.MessageType = "reply"
	}
	if m.Metadata == "" {
		m.Metadata = "{}"
	}
	if err := db.Exec(`INSERT INTO support_messages
		(id, workspace_id, conversation_id, sender_type, message_type, sender_user_id,
		 content, is_internal, metadata, cancellable_until)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.WorkspaceID, m.ConversationID, m.SenderType, m.MessageType,
		m.SenderUserID, m.Content, m.IsInternal, m.Metadata, m.CancellableUntil,
	).Error; err != nil {
		t.Fatalf("insert message %s: %v", m.ID, err)
	}
}

func TestSupportMessageRepository_GetMessageForActor(t *testing.T) {
	db := setupSupportMessageTestDB(t)
	repo := NewSupportMessageRepository(db)
	ctx := context.Background()

	tests := []struct {
		name    string
		seed    model.SupportMessage
		queryAs string // user ID we look up as
		wantHit bool
	}{
		{
			name: "owned reply is returned",
			seed: model.SupportMessage{
				ID: "m1", WorkspaceID: "w", ConversationID: "c",
				SenderType: "user", SenderUserID: strPtr("u1"),
				Content: "hi", MessageType: "reply",
			},
			queryAs: "u1",
			wantHit: true,
		},
		{
			name: "not owned (different user)",
			seed: model.SupportMessage{
				ID: "m2", WorkspaceID: "w", ConversationID: "c",
				SenderType: "user", SenderUserID: strPtr("u1"),
				Content: "hi", MessageType: "reply",
			},
			queryAs: "u2",
			wantHit: false,
		},
		{
			name: "AI message is never owned by a human user",
			seed: model.SupportMessage{
				ID: "m3", WorkspaceID: "w", ConversationID: "c",
				SenderType: "ai", SenderUserID: nil,
				Content: "hi", MessageType: "reply",
			},
			queryAs: "u1",
			wantHit: false,
		},
		{
			name: "internal note excluded",
			seed: model.SupportMessage{
				ID: "m4", WorkspaceID: "w", ConversationID: "c",
				SenderType: "user", SenderUserID: strPtr("u1"),
				Content: "note", MessageType: "reply", IsInternal: true,
			},
			queryAs: "u1",
			wantHit: false,
		},
		{
			name: "system message excluded",
			seed: model.SupportMessage{
				ID: "m5", WorkspaceID: "w", ConversationID: "c",
				SenderType: "user", SenderUserID: strPtr("u1"),
				Content: "sys", MessageType: "system",
			},
			queryAs: "u1",
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insertMessage(t, db, tt.seed)
			got, err := repo.GetMessageForActor(ctx, tt.seed.ID, tt.queryAs)
			if err != nil {
				t.Fatalf("GetMessageForActor: %v", err)
			}
			if tt.wantHit && got == nil {
				t.Fatalf("expected hit, got nil")
			}
			if !tt.wantHit && got != nil {
				t.Fatalf("expected miss, got %+v", got)
			}
		})
	}
}

func TestSupportMessageRepository_SoftDeleteMessage(t *testing.T) {
	db := setupSupportMessageTestDB(t)
	repo := NewSupportMessageRepository(db)
	ctx := context.Background()

	insertMessage(t, db, model.SupportMessage{
		ID: "m1", WorkspaceID: "w", ConversationID: "c",
		SenderType: "user", SenderUserID: strPtr("u1"),
		Content: "hi", MessageType: "reply",
	})

	// owned soft-delete writes deleted_at
	if err := repo.SoftDeleteMessage(ctx, "m1", "u1"); err != nil {
		t.Fatalf("SoftDeleteMessage owned: %v", err)
	}
	var deletedAt sql.NullTime
	if err := db.Raw(`SELECT deleted_at FROM support_messages WHERE id = ?`, "m1").Scan(&deletedAt).Error; err != nil {
		t.Fatalf("read deleted_at m1: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatalf("expected deleted_at to be set on m1")
	}

	// once soft-deleted, the actor lookup misses (filtered by deleted_at)
	got, err := repo.GetMessageForActor(ctx, "m1", "u1")
	if err != nil {
		t.Fatalf("GetMessageForActor after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("expected soft-deleted row to be filtered, got %+v", got)
	}

	// not-owned soft-delete is a no-op (no row matches predicate)
	insertMessage(t, db, model.SupportMessage{
		ID: "m2", WorkspaceID: "w", ConversationID: "c",
		SenderType: "user", SenderUserID: strPtr("u1"),
		Content: "hi", MessageType: "reply",
	})
	if err := repo.SoftDeleteMessage(ctx, "m2", "stranger"); err != nil {
		t.Fatalf("SoftDeleteMessage not-owned: %v", err)
	}
	deletedAt = sql.NullTime{}
	if err := db.Raw(`SELECT deleted_at FROM support_messages WHERE id = ?`, "m2").Scan(&deletedAt).Error; err != nil {
		t.Fatalf("read deleted_at m2: %v", err)
	}
	if deletedAt.Valid {
		t.Fatalf("expected non-owned soft-delete to be a no-op (deleted_at = %v)", deletedAt.Time)
	}
}

func TestSupportMessageRepository_SetCancellableUntil(t *testing.T) {
	db := setupSupportMessageTestDB(t)
	repo := NewSupportMessageRepository(db)
	ctx := context.Background()

	insertMessage(t, db, model.SupportMessage{
		ID: "m1", WorkspaceID: "w", ConversationID: "c",
		SenderType: "user", SenderUserID: strPtr("u1"),
		Content: "hi", MessageType: "reply",
	})

	want := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Second)
	if err := repo.SetCancellableUntil(ctx, "m1", want); err != nil {
		t.Fatalf("SetCancellableUntil: %v", err)
	}
	var got *time.Time
	if err := db.Raw(`SELECT cancellable_until FROM support_messages WHERE id = ?`, "m1").Scan(&got).Error; err != nil {
		t.Fatalf("read cancellable_until: %v", err)
	}
	if got == nil || !got.UTC().Equal(want) {
		t.Fatalf("cancellable_until: got %v want %v", got, want)
	}
}
