package repository

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPortalAuthAssociateVerifiedEmailScopedAndVisible(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT)`,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT, customer_email TEXT, crm_contact_id TEXT, portal_visible BOOLEAN, status TEXT, channel TEXT, source TEXT, deleted_at DATETIME, subject TEXT, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT UNIQUE, portal_identity_id TEXT, reference TEXT UNIQUE, created_at DATETIME)`,
		`INSERT INTO crm_contacts VALUES ('contact-a','workspace-a','alice@example.com')`,
		`INSERT INTO support_conversations (id, workspace_id, customer_email, crm_contact_id, portal_visible, status, channel, source) VALUES
   ('email-match','workspace-a','ALICE@example.com',NULL,1,'open','email','email'),
   ('contact-match','workspace-a',NULL,'contact-a',1,'open','widget','widget'),
   ('conflicting','workspace-a','bob@example.com','contact-a',1,'open','widget','widget'),
   ('hidden','workspace-a','alice@example.com',NULL,0,'open','email','email'),
   ('internal','workspace-a','alice@example.com',NULL,1,'open','internal','internal'),
   ('other-workspace','workspace-b','alice@example.com',NULL,1,'open','email','email')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewPortalAuthRepository(db)
	if err := repo.AssociateVerifiedEmail(db, "workspace-a", "alice@example.com", "identity-a"); err != nil {
		t.Fatal(err)
	}
	if err := repo.AssociateVerifiedEmail(db, "workspace-a", "alice@example.com", "identity-b"); err != nil {
		t.Fatal(err)
	}
	var ids []string
	if err := db.Table("support_portal_request_references").Where("portal_identity_id = ?", "identity-a").Order("conversation_id").Pluck("conversation_id", &ids).Error; err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "contact-match" || ids[1] != "email-match" {
		t.Fatalf("associated: %v", ids)
	}
	requests, err := repo.ListRequests(context.Background(), "workspace-a", "identity-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 0 {
		t.Fatalf("existing ownership changed: %v", requests)
	}
	if err := db.Table("support_conversations").Where("id = ?", "email-match").Update("portal_visible", false).Error; err != nil {
		t.Fatal(err)
	}
	requests, err = repo.ListRequests(context.Background(), "workspace-a", "identity-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 {
		t.Fatalf("hidden request exposed: %v", requests)
	}
}
