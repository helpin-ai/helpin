package crmemail

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestBackfillRunner_CreatesAssociationsAndRefreshesThreadCache(t *testing.T) {
	db := setupCRMEmailTestDB(t)
	ctx := context.Background()

	contactRepo := repository.NewCRMContactRepository(db)
	emailRepo := repository.NewCRMEmailRepository(db)
	settingsRepo := repository.NewCRMEmailSyncSettingsRepository(db)
	resolver := NewResolver(contactRepo)
	runner := NewBackfillRunner(emailRepo, settingsRepo, resolver)

	if err := db.Exec(`
		INSERT INTO crm_email_accounts (id, workspace_id, member_id, provider, email_address, is_active)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "acct-1", "ws-1", "member-1", "gmail", "owner@example.com", true).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}

	settings := model.DefaultEmailSyncSettings()
	settings.WorkspaceID = "ws-1"
	settings.RecordCreationMode = "always"
	if err := settingsRepo.Upsert(ctx, &settings); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO crm_email_threads (id, workspace_id, email_account_id, thread_external_id, subject, last_message_at, message_count, contact_ids)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "thread-1", "ws-1", "acct-1", "gmail-thread-1", "Hello", time.Now(), 1, []byte(`[]`)).Error; err != nil {
		t.Fatalf("seed thread: %v", err)
	}

	toJSON, _ := json.Marshal([]string{"buyer@example.com"})
	ccJSON, _ := json.Marshal([]string{"other@example.com"})
	if err := db.Exec(`
		INSERT INTO crm_email_messages (
			id, workspace_id, email_account_id, thread_id, message_external_id,
			from_address, to_addresses, cc_addresses, subject, direction, sent_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "msg-1", "ws-1", "acct-1", "thread-1", "gmail-msg-1", "owner@example.com", toJSON, ccJSON, "Hello", model.CRMEmailDirectionOutbound, time.Now()).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var assocCount int64
	if err := db.Table("crm_email_message_contacts").Where("message_id = ?", "msg-1").Count(&assocCount).Error; err != nil {
		t.Fatalf("count associations: %v", err)
	}
	if assocCount != 2 {
		t.Fatalf("association count = %d, want 2", assocCount)
	}

	var message model.CRMEmailMessage
	if err := db.Table("crm_email_messages").Where("id = ?", "msg-1").First(&message).Error; err != nil {
		t.Fatalf("load message: %v", err)
	}
	if message.ContactID != nil {
		t.Fatalf("contact_id = %v, want nil for multi-contact message", *message.ContactID)
	}

	var thread model.CRMEmailThread
	if err := db.Table("crm_email_threads").Where("id = ?", "thread-1").First(&thread).Error; err != nil {
		t.Fatalf("load thread: %v", err)
	}

	var contactIDs []string
	if err := json.Unmarshal(thread.ContactIDs, &contactIDs); err != nil {
		t.Fatalf("unmarshal thread contact_ids: %v", err)
	}
	if len(contactIDs) != 2 {
		t.Fatalf("thread contact_ids = %v, want 2 contacts", contactIDs)
	}
}
