//go:build integration

package repository

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestTranslationPostgresMigrationPrivacyAndConcurrentSend(t *testing.T) {
	db := contactPrivacyDB(t)
	migration, err := os.ReadFile("../dbmigrate/sql/202609180001_support_translation.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Apply the actual DDL twice as well as the ledger's normal second no-op run.
	for i := 0; i < 2; i++ {
		privacyExec(t, db, string(migration))
	}
	workspace := uuid.NewString()
	seedContactPrivacyWorkspace(t, db, workspace)
	actor := uuid.NewString()
	privacyExec(t, db, `INSERT INTO users(id,email,full_name,password_hash) VALUES (?,?,'Translator','test')`, actor, actor+"@example.invalid")
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspace, Subject: "Translation", Status: "open", Source: "widget"}
	if err := db.Create(conv).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewSupportTranslationRepository(db)
	messages := NewSupportMessageRepository(db)
	expires := time.Now().Add(time.Hour)
	candidate := &model.SupportTranslation{ID: uuid.NewString(), WorkspaceID: workspace, ConversationID: conv.ID, Purpose: "outgoing_reply", CreatedByUserID: &actor, SendKey: uuid.NewString(), SourceText: "Hello", SourceHash: "hash", TargetLanguage: "de", CacheKey: "cache", PipelineVersion: "v1", Attempts: 1, Status: "pending", ReviewStatus: "not_requested", ExpiresAt: &expires}
	artifact, owned, err := repo.Reserve(context.Background(), candidate)
	if err != nil || !owned {
		t.Fatalf("reserve owned=%v err=%v", owned, err)
	}
	artifact.Status = "ready"
	artifact.TranslatedText = "Hallo"
	if err := repo.Finish(context.Background(), artifact); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- messages.Create(context.Background(), &model.SupportMessage{WorkspaceID: workspace, ConversationID: conv.ID, SenderType: "user", SenderUserID: &actor, Content: "Hallo", MessageType: "reply", TranslationID: artifact.ID})
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent sends succeeded=%d", successes)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("conversation_id = ?", conv.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("reply count=%d", count)
	}
	// A reused client key with changed text cannot create a second artifact.
	changed := *candidate
	changed.ID = uuid.NewString()
	changed.CacheKey = "changed"
	changed.SourceText = "Different"
	if _, _, err := repo.Reserve(context.Background(), &changed); err == nil {
		t.Fatal("changed send intent accepted")
	}
	privacyExec(t, db, "UPDATE support_messages SET content='edited' WHERE conversation_id=?", conv.ID)
	privacyCheck(t, db, "SELECT count(*) = 0 FROM support_translations WHERE conversation_id=?", conv.ID)
	// Tenant composite foreign keys prevent an artifact referencing a foreign conversation.
	foreign := *candidate
	foreign.ID = uuid.NewString()
	foreign.WorkspaceID = uuid.NewString()
	foreign.CacheKey = "foreign"
	if err := db.Create(&foreign).Error; err == nil {
		t.Fatal("cross-tenant artifact accepted")
	}
	// Conversation anonymization removes originals and rejects in-flight settlement.
	pending := *candidate
	pending.ID = uuid.NewString()
	pending.SendKey = uuid.NewString()
	pending.CacheKey = "pending"
	pending.Status = "pending"
	if _, _, err := repo.Reserve(context.Background(), &pending); err != nil {
		t.Fatal(err)
	}
	privacyExec(t, db, "UPDATE support_conversations SET anonymized_at=now() WHERE id=?", conv.ID)
	pending.Status = "ready"
	if err := repo.Finish(context.Background(), &pending); err == nil {
		t.Fatal("in-flight artifact recreated after anonymization")
	}
	privacyCheck(t, db, "SELECT count(*) = 0 FROM support_translations WHERE conversation_id=?", conv.ID)
}

func TestTranslationPostgresConversationPolicyGuard(t *testing.T) {
	db := contactPrivacyDB(t)
	workspace := uuid.NewString()
	seedContactPrivacyWorkspace(t, db, workspace)
	actor := uuid.NewString()
	privacyExec(t, db, `INSERT INTO users(id,email,full_name,password_hash) VALUES (?,?,'Translator','test')`, actor, actor+"@example.invalid")
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspace, Subject: "Translation", Status: "open", Source: "widget"}
	if err := db.Create(conv).Error; err != nil {
		t.Fatal(err)
	}
	installation := &model.SupportWidgetInstallation{ID: uuid.NewString(), WorkspaceID: workspace, WidgetKey: uuid.NewString(), Settings: `{"translation_enabled":false}`}
	if err := db.Create(installation).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewSupportTranslationRepository(db)
	expires := time.Now().Add(time.Hour)
	candidate := &model.SupportTranslation{ID: uuid.NewString(), WorkspaceID: workspace, ConversationID: conv.ID, Purpose: "outgoing_reply", CreatedByUserID: &actor, SendKey: uuid.NewString(), SourceText: "Hello", SourceHash: "hash", SourceLanguage: "en", TargetLanguage: "de", TranslatedText: "Hallo", CacheKey: "cache", PipelineVersion: "v1", Attempts: 1, Status: "pending", ReviewStatus: "not_requested", ExpiresAt: &expires}
	artifact, owned, err := repo.Reserve(context.Background(), candidate)
	if err != nil || !owned {
		t.Fatalf("reserve=%v owned=%v", err, owned)
	}
	artifact.Status = "ready"
	if err := repo.Finish(context.Background(), artifact); err != nil {
		t.Fatal(err)
	}
	messages := NewSupportMessageRepository(db)
	send := func() error {
		return messages.Create(context.Background(), &model.SupportMessage{WorkspaceID: workspace, ConversationID: conv.ID, SenderType: "user", SenderUserID: &actor, Content: "Hallo", MessageType: "reply", TranslationID: artifact.ID})
	}

	// Workspace defaults initialize new conversations; this existing thread is
	// guarded by its own policy, including a manual destination correction.
	for _, policy := range []struct {
		on       bool
		language string
	}{{false, ""}, {true, "fr"}} {
		if err := repo.SetLiveConversation(context.Background(), workspace, conv.ID, policy.on, policy.language); err != nil {
			t.Fatal(err)
		}
		if err := send(); err == nil {
			t.Fatalf("conversation policy allowed send: %+v", policy)
		}
	}
	if err := repo.SetLiveConversation(context.Background(), workspace, conv.ID, true, "de"); err != nil {
		t.Fatal(err)
	}

	if err := send(); err != nil {
		t.Fatal(err)
	}
}
