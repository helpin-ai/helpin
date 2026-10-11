package repository

import (
	"context"
	"errors"
	"github.com/helpin-ai/helpin/server/internal/model"
	"strings"
	"testing"
	"time"
)

func TestSupportExplicitEmailStatusPreservesIntentAndOtherMetadata(t *testing.T) {
	db := setupSupportMessageTestDB(t)
	repo := NewSupportMessageRepository(db)
	ctx := context.Background()
	for _, tc := range []struct{ id, metadata string }{{"email", `{"delivery_mode":"email_only","client_message_id":"client","email_cc":["copy@example.com"]}`}, {"legacy", `{"client_message_id":"legacy"}`}} {
		insertMessage(t, db, model.SupportMessage{ID: tc.id, WorkspaceID: "ws", ConversationID: "conv", SenderType: "user", Content: "reply", Metadata: tc.metadata, CreatedAt: time.Now()})
	}
	if err := repo.UpdateExplicitEmailStatus(ctx, []string{"email", "legacy"}, "blocked", "Recipient changed"); err != nil {
		t.Fatal(err)
	}
	email, err := repo.GetByID(ctx, "email")
	if err != nil {
		t.Fatal(err)
	}
	if !email.ExplicitEmailDelivery() || !strings.Contains(email.Metadata, `"client_message_id":"client"`) || !strings.Contains(email.Metadata, `"email_delivery_status":"blocked"`) || !strings.Contains(email.Metadata, "copy@example.com") {
		t.Fatalf("metadata=%s", email.Metadata)
	}
	legacy, err := repo.GetByID(ctx, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(legacy.Metadata, "email_delivery_status") {
		t.Fatalf("legacy changed=%s", legacy.Metadata)
	}
}

func TestMergeSharedFallbackRecordsBlockedEmailStatus(t *testing.T) {
	db := setupSupportMergeTestDB(t)
	repo := NewSupportMessageRepository(db)
	ctx := context.Background()
	merges := NewSupportMergeRepository(db)
	if _, err := merges.Merge(ctx, "ws", "b", []string{"a"}, "actor", "op", false); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, &model.SupportMessage{ID: "shared", WorkspaceID: "ws", ConversationID: "b", SenderType: "agent", MessageType: "reply", Content: "reply", Metadata: "{}"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetCancellableUntil(ctx, "shared", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := merges.EnsureNoPendingSends(ctx, "ws", []string{"b"}); !errors.Is(err, ErrSupportMergeSending) {
		t.Fatalf("queued shared reply did not block unmerge: %v", err)
	}
	if err := db.Model(&model.SupportConversation{}).Where("id = ?", "b").Update("contact_last_seen_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if err := merges.EnsureNoPendingSends(ctx, "ws", []string{"b"}); !errors.Is(err, ErrSupportMergeSending) {
		t.Fatalf("reading the chat bypassed the shared email queue: %v", err)
	}
	if err := db.Model(&model.SupportConversation{}).Where("id = ?", "b").Update("customer_email", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err := merges.EnsureNoPendingSends(ctx, "ws", []string{"b"}); !errors.Is(err, ErrSupportMergeSending) {
		t.Fatalf("anonymous primary bypassed the shared email queue: %v", err)
	}
	if err := repo.UpdateExplicitEmailStatus(ctx, []string{"shared"}, "blocked", "The recipients changed."); err != nil {
		t.Fatal(err)
	}
	message, err := repo.GetByID(ctx, "shared")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Metadata, `"email_delivery_status":"blocked"`) || !strings.Contains(message.Metadata, "sam@example.test") {
		t.Fatalf("shared fallback lost blocked status or audience: %s", message.Metadata)
	}
	if err := merges.EnsureNoPendingSends(ctx, "ws", []string{"b"}); err != nil {
		t.Fatalf("blocked email still prevents unmerge: %v", err)
	}
}
