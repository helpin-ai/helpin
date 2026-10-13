//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestTeammateJoinedPostgresConcurrentReplies(t *testing.T) {
	db := contactPrivacyDB(t)
	ws, user := uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	privacyExec(t, db, "INSERT INTO users(id,email,full_name,password_hash) VALUES (?,?,'Agent','test')", user, user+"@example.invalid")
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws, Subject: "Timeline", Status: "open", Source: "widget"}
	if err := db.Create(conv).Error; err != nil {
		t.Fatal(err)
	}
	messages := NewSupportMessageRepository(db)
	makeMessage := func() *model.SupportMessage {
		return &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: conv.ID, SenderType: "user", SenderUserID: &user, MessageType: "reply", Content: "Hello",
			TeammateJoined: &model.SupportMessage{SenderType: "user", SenderUserID: &user, MessageType: "system", SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventTeammateJoined), Content: "Agent joined the conversation."},
		}
	}
	// A database error after the join insert must roll the join back as well.
	failed := makeMessage()
	failed.Metadata = "{invalid" // PostgreSQL rejects the reply's JSONB after the join insert.
	if err := messages.Create(context.Background(), failed); err == nil {
		t.Fatal("expected invalid actor rejection")
	}
	privacyCheck(t, db, "SELECT count(*)=0 FROM support_messages WHERE conversation_id=?", conv.ID)
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- messages.Create(context.Background(), makeMessage()) }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	rows, err := messages.ListByConversation(context.Background(), ws, conv.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].SystemEventType == nil || *rows[0].SystemEventType != model.SystemEventTeammateJoined || rows[1].MessageType != "reply" || rows[2].MessageType != "reply" {
		t.Fatalf("simultaneous replies must have one preceding join: %+v", rows)
	}
	if !rows[0].CreatedAt.Before(rows[1].CreatedAt) {
		t.Fatal("join must precede its reply at database precision")
	}
}
