//go:build integration

package repository

import (
	"context"
	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"os"
	"testing"
)

func TestLiveTranslatePostgresArrivalPolicyAndPrivacy(t *testing.T) {
	db := contactPrivacyDB(t)
	ws := uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	installation := &model.SupportWidgetInstallation{ID: uuid.NewString(), WorkspaceID: ws, WidgetKey: uuid.NewString(), Settings: `{"translation_enabled":false}`}
	if err := db.Create(installation).Error; err != nil {
		t.Fatal(err)
	}
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws, Subject: "Live", Status: "open", Source: "widget"}
	if err := db.Create(conv).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewSupportTranslationRepository(db)
	add := func() string {
		id := uuid.NewString()
		if err := db.Create(&model.SupportMessage{ID: id, WorkspaceID: ws, ConversationID: conv.ID, SenderType: "customer", MessageType: "reply", Content: "Please check my invoice"}).Error; err != nil {
			t.Fatal(err)
		}
		return id
	}
	off := add()
	if err := repo.SetLiveConversation(context.Background(), ws, conv.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	on := add()
	if err := repo.SetLiveConversation(context.Background(), ws, conv.ID, false, ""); err != nil {
		t.Fatal(err)
	}
	offAgain := add()
	if err := repo.SetLiveConversation(context.Background(), ws, conv.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, "SELECT NOT enabled AND revision=1 AND source_hash<>'' FROM support_live_messages WHERE message_id=?", off)
	privacyCheck(t, db, "SELECT enabled AND revision=2 FROM support_live_messages WHERE message_id=?", on)
	privacyCheck(t, db, "SELECT NOT enabled AND revision=3 FROM support_live_messages WHERE message_id=?", offAgain)
	// Reapplying DDL preserves explicit off and resolves legacy inherit conservatively.
	privacyExec(t, db, "UPDATE support_translation_conversations SET translation_mode='inherit' WHERE conversation_id=?", conv.ID)
	migration, err := os.ReadFile("../dbmigrate/sql/202609220002_support_live_translate.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		privacyExec(t, db, string(migration))
	}
	privacyCheck(t, db, "SELECT translation_mode='off' FROM support_translation_conversations WHERE conversation_id=?", conv.ID)
	// Manual translation is valid even for an arrival that was off.
	a := &model.SupportTranslation{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: conv.ID, Purpose: "manual_display", SourceMessageID: &off, SourceText: "Please check my invoice", SourceHash: "hash", TargetLanguage: "de", CacheKey: uuid.NewString(), Status: "ready", ReviewStatus: "not_requested", PipelineVersion: "test"}
	if err := db.Create(a).Error; err != nil {
		t.Fatal(err)
	}
	privacyExec(t, db, "UPDATE support_conversations SET anonymized_at=now() WHERE id=?", conv.ID)
	privacyCheck(t, db, "SELECT count(*)=0 FROM support_live_messages WHERE conversation_id=?", conv.ID)
	privacyCheck(t, db, "SELECT count(*)=0 FROM support_translations WHERE conversation_id=?", conv.ID)
}

func TestLiveTranslatePostgresSendRejectsChangedPolicyAndLease(t *testing.T) {
	db := contactPrivacyDB(t)
	ws := uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	user := uuid.NewString()
	privacyExec(t, db, "INSERT INTO users(id,email,full_name,password_hash) VALUES (?,?,'Agent','test')", user, user+"@example.invalid")
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws, Subject: "Reply", Status: "open", Source: "widget"}
	if err := db.Create(conv).Error; err != nil {
		t.Fatal(err)
	}
	job := &model.SupportPendingSend{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: conv.ID, UserID: user, Request: "{}", RequestHash: "hash", Revision: 1, Attempts: 1, Status: "sending"}
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}
	messages := NewSupportMessageRepository(db)
	makeMessage := func() *model.SupportMessage {
		return &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: conv.ID, SenderType: "user", SenderUserID: &user, MessageType: "reply", Content: "Hello", PendingGuard: job}
	}
	privacyExec(t, db, "UPDATE support_pending_sends SET attempts=2 WHERE id=?", job.ID)
	if err := messages.Create(context.Background(), makeMessage()); err == nil {
		t.Fatal("expired worker delivered")
	}
	job.Attempts = 2
	if err := NewSupportTranslationRepository(db).SetLiveConversation(context.Background(), ws, conv.ID, false, ""); err != nil {
		t.Fatal(err)
	}
	if err := messages.Create(context.Background(), makeMessage()); err == nil {
		t.Fatal("changed policy delivered")
	}
	privacyCheck(t, db, "SELECT count(*)=0 FROM support_messages WHERE conversation_id=?", conv.ID)
	job.Revision = 2
	if err := messages.Create(context.Background(), makeMessage()); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, "SELECT count(*)=1 FROM support_messages WHERE conversation_id=?", conv.ID)
	privacyExec(t, db, "UPDATE support_conversations SET anonymized_at=now() WHERE id=?", conv.ID)
	privacyCheck(t, db, "SELECT count(*)=0 FROM support_pending_sends WHERE conversation_id=?", conv.ID)
}
