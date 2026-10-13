//go:build integration

package service

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestWidgetReplyPostgresAtomicReopen(t *testing.T) {
	db := privacyServiceDB(t)
	ws := uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	svc := privacyInbox(db)
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws, Subject: "Resolved", Status: "resolved", Source: "widget"}
	if err := db.Create(conv).Error; err != nil {
		t.Fatal(err)
	}
	message := func() *model.SupportMessage {
		return &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: conv.ID, SenderType: "customer", MessageType: "reply", Content: "One more question", ViaChannel: strPtr("widget")}
	}
	privacyExec(t, db, "ALTER TABLE support_messages ADD CONSTRAINT reject_reopen_test CHECK (system_event_type IS DISTINCT FROM 'reopened')")
	if _, err := svc.createWidgetReply(context.Background(), message()); err == nil {
		t.Fatal("expected activity save failure")
	}
	privacyCheck(t, db, "SELECT status='resolved' FROM support_conversations WHERE id=?", conv.ID)
	privacyCheck(t, db, "SELECT count(*)=0 FROM support_messages WHERE conversation_id=?", conv.ID)
	privacyExec(t, db, "ALTER TABLE support_messages DROP CONSTRAINT reject_reopen_test")
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.createWidgetReply(context.Background(), message())
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	privacyCheck(t, db, "SELECT status='open' FROM support_conversations WHERE id=?", conv.ID)
	rows, err := svc.messageRepo.ListByConversation(context.Background(), ws, conv.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].SenderType != "customer" || derefString(rows[1].SystemEventType) != model.SystemEventReopened || !rows[1].IsInternal || rows[2].SenderType != "customer" {
		t.Fatalf("concurrent customer replies should reopen once: %+v", rows)
	}
}
