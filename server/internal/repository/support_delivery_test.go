package repository

import (
	"context"
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
