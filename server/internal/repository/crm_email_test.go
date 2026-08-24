package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupCRMEmailRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:crm_email_repo_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE crm_email_threads (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL,
			thread_external_id TEXT NOT NULL,
			subject TEXT NOT NULL,
			last_message_at DATETIME NOT NULL,
			message_count INTEGER NOT NULL DEFAULT 0,
			contact_ids BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			deal_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_email_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL,
			thread_id TEXT,
			message_external_id TEXT,
			rfc_message_id TEXT,
			in_reply_to TEXT,
			references_header TEXT,
			from_address TEXT NOT NULL,
			from_name TEXT,
			to_addresses BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			cc_addresses BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			subject TEXT,
			body_text TEXT,
			body_html TEXT,
			direction TEXT NOT NULL DEFAULT 'inbound',
			sent_at DATETIME NOT NULL,
			contact_id TEXT,
			deal_id TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE crm_email_message_contacts (
			message_id TEXT NOT NULL,
			contact_id TEXT NOT NULL,
			participant_role TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (message_id, contact_id, participant_role)
		)`,
		`CREATE TABLE crm_associations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			from_object_type TEXT NOT NULL,
			from_object_id TEXT NOT NULL,
			to_object_type TEXT NOT NULL,
			to_object_id TEXT NOT NULL
		)`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("apply schema statement %q: %v", stmt, err)
		}
	}

	return db
}

func TestCRMEmailRepository_ListMessagesByAssociatedCompanyContacts(t *testing.T) {
	db := setupCRMEmailRepositoryTestDB(t)
	repo := NewCRMEmailRepository(db)
	ctx := context.Background()
	now := time.Now()

	for _, message := range []struct {
		id        string
		contactID any
	}{
		{id: "message-primary", contactID: "contact-primary"},
		{id: "message-participant", contactID: nil},
		{id: "message-other-company", contactID: "contact-other"},
	} {
		if err := db.Exec(`
			INSERT INTO crm_email_messages (
				id, workspace_id, email_account_id, from_address, to_addresses, cc_addresses,
				subject, direction, sent_at, contact_id
			) VALUES (?, 'ws-1', 'account-1', 'owner@example.com', ?, ?, ?, 'outbound', ?, ?)
		`, message.id, []byte(`[]`), []byte(`[]`), message.id, now, message.contactID).Error; err != nil {
			t.Fatalf("seed %s: %v", message.id, err)
		}
	}

	for _, statement := range []string{
		`INSERT INTO crm_associations VALUES ('association-primary', 'ws-1', 'contact', 'contact-primary', 'company', 'company-1')`,
		`INSERT INTO crm_associations VALUES ('association-participant', 'ws-1', 'company', 'company-1', 'contact', 'contact-participant')`,
		`INSERT INTO crm_associations VALUES ('association-other', 'ws-1', 'contact', 'contact-other', 'company', 'company-2')`,
		`INSERT INTO crm_email_message_contacts VALUES ('message-participant', 'contact-participant', 'to', 'ws-1', NULL)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed company email association: %v", err)
		}
	}

	companyID := "company-1"
	messages, total, err := repo.ListMessages(ctx, "ws-1", model.CRMEmailMessageListFilters{
		CompanyID: &companyID,
	}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if total != 2 || len(messages) != 2 {
		t.Fatalf("messages = %#v total = %d, want two company messages", messages, total)
	}
	ids := map[string]bool{}
	for _, message := range messages {
		ids[message.ID] = true
	}
	if !ids["message-primary"] || !ids["message-participant"] || ids["message-other-company"] {
		t.Fatalf("company message IDs = %v", ids)
	}
}

func TestCRMEmailRepository_ListMessagesByAssociatedContact(t *testing.T) {
	db := setupCRMEmailRepositoryTestDB(t)
	repo := NewCRMEmailRepository(db)
	ctx := context.Background()

	if err := db.Exec(`
		INSERT INTO crm_email_messages (
			id, workspace_id, email_account_id, thread_id, from_address, to_addresses, cc_addresses, subject, direction, sent_at, contact_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "msg-1", "ws-1", "acct-1", "thread-1", "owner@example.com", []byte(`["buyer@example.com"]`), []byte(`["other@example.com"]`), "Hello", "outbound", time.Now(), nil).Error; err != nil {
		t.Fatalf("seed message 1: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO crm_email_messages (
			id, workspace_id, email_account_id, thread_id, from_address, to_addresses, cc_addresses, subject, direction, sent_at, contact_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "msg-2", "ws-1", "acct-1", "thread-2", "owner@example.com", []byte(`["else@example.com"]`), []byte(`[]`), "Other", "outbound", time.Now(), "contact-legacy").Error; err != nil {
		t.Fatalf("seed message 2: %v", err)
	}

	for _, stmt := range []string{
		`INSERT INTO crm_email_message_contacts (message_id, contact_id, participant_role, workspace_id) VALUES ('msg-1', 'contact-1', 'to', 'ws-1')`,
		`INSERT INTO crm_email_message_contacts (message_id, contact_id, participant_role, workspace_id) VALUES ('msg-1', 'contact-2', 'cc', 'ws-1')`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("seed message contact: %v", err)
		}
	}

	contactID := "contact-2"
	messages, total, err := repo.ListMessages(ctx, "ws-1", model.CRMEmailMessageListFilters{
		ContactID: &contactID,
	}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if total != 1 || len(messages) != 1 {
		t.Fatalf("messages = %d total = %d, want 1", len(messages), total)
	}
	if got := messages[0].ContactIDs; len(got) != 2 {
		t.Fatalf("contact_ids = %v, want two associated contacts", got)
	}

	legacyContactID := "contact-legacy"
	messages, total, err = repo.ListMessages(ctx, "ws-1", model.CRMEmailMessageListFilters{
		ContactID: &legacyContactID,
	}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("ListMessages legacy: %v", err)
	}
	if total != 1 || len(messages) != 1 || messages[0].ID != "msg-2" {
		t.Fatalf("legacy filtered messages = %#v total = %d, want msg-2", messages, total)
	}
}

func TestCRMEmailRepository_RefreshThreadContactIDsAndListThreads(t *testing.T) {
	db := setupCRMEmailRepositoryTestDB(t)
	repo := NewCRMEmailRepository(db)
	ctx := context.Background()

	if err := db.Exec(`
		INSERT INTO crm_email_threads (id, workspace_id, email_account_id, thread_external_id, subject, last_message_at, message_count, contact_ids)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "thread-1", "ws-1", "acct-1", "gmail-thread-1", "Hello", time.Now(), 1, []byte(`[]`)).Error; err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO crm_email_messages (
			id, workspace_id, email_account_id, thread_id, from_address, to_addresses, cc_addresses, subject, direction, sent_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "msg-1", "ws-1", "acct-1", "thread-1", "owner@example.com", []byte(`["buyer@example.com"]`), []byte(`[]`), "Hello", "outbound", time.Now()).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO crm_email_message_contacts (message_id, contact_id, participant_role, workspace_id)
		VALUES (?, ?, ?, ?), (?, ?, ?, ?)
	`, "msg-1", "contact-1", "to", "ws-1", "msg-1", "contact-2", "cc", "ws-1").Error; err != nil {
		t.Fatalf("seed associations: %v", err)
	}

	if err := repo.RefreshThreadContactIDs(ctx, "thread-1"); err != nil {
		t.Fatalf("RefreshThreadContactIDs: %v", err)
	}

	var contactPayload string
	if err := db.Table("crm_email_threads").Select("contact_ids").Where("id = ?", "thread-1").Scan(&contactPayload).Error; err != nil {
		t.Fatalf("load thread contact_ids: %v", err)
	}
	var contactIDs []string
	if err := json.Unmarshal([]byte(contactPayload), &contactIDs); err != nil {
		t.Fatalf("unmarshal thread contact_ids: %v", err)
	}
	if len(contactIDs) != 2 {
		t.Fatalf("thread contact_ids = %v, want 2", contactIDs)
	}

	contactID := "contact-1"
	threads, total, err := repo.ListThreads(ctx, "ws-1", model.CRMEmailThreadListFilters{
		ContactID: &contactID,
	}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	if total != 1 || len(threads) != 1 || threads[0].ID != "thread-1" {
		t.Fatalf("threads = %#v total = %d, want thread-1", threads, total)
	}
}
