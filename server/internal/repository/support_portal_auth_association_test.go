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
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT, customer_email TEXT, crm_contact_id TEXT, portal_visible BOOLEAN, portal_visibility_changed_at DATETIME, primary_recipient_state TEXT, status TEXT, channel TEXT, source TEXT, deleted_at DATETIME, subject TEXT, created_at DATETIME, updated_at DATETIME, last_public_message_at DATETIME, resolved_at DATETIME)`,
		`CREATE TABLE support_widget_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, customer_email TEXT, identity_trust TEXT, identity_verified_at DATETIME)`,
		`CREATE TABLE support_portal_identities (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT)`,
		`INSERT INTO support_portal_identities VALUES ('identity-a','workspace-a','alice@example.com'), ('identity-b','workspace-a','alice@example.com')`,
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT UNIQUE, portal_identity_id TEXT, reference TEXT UNIQUE, created_at DATETIME)`,
		`INSERT INTO crm_contacts VALUES ('contact-a','workspace-a','alice@example.com')`,
		`INSERT INTO support_conversations (id, workspace_id, customer_email, crm_contact_id, portal_visible, status, channel, source) VALUES
   ('email-match','workspace-a','ALICE@example.com',NULL,1,'open','email','email'),
   ('contact-match','workspace-a','alice@example.com','contact-a',1,'open','widget','widget'),
   ('conflicting','workspace-a','bob@example.com','contact-a',1,'open','widget','widget'),
   ('unverified','workspace-a','alice@example.com',NULL,1,'open','widget','widget'),
   ('mismatched','workspace-a','alice@example.com',NULL,1,'open','widget','widget'),
   ('ambiguous','workspace-a','alice@example.com',NULL,1,'open','widget','widget'),
   ('hidden','workspace-a','alice@example.com',NULL,0,'open','email','email'),
   ('internal','workspace-a','alice@example.com',NULL,1,'open','internal','internal'),
   ('other-workspace','workspace-b','alice@example.com',NULL,1,'open','email','email')`,
		`INSERT INTO support_widget_sessions VALUES ('verified','workspace-a','contact-match','alice@example.com','verified','2026-01-01'), ('unsigned','workspace-a','unverified','alice@example.com','untrusted',NULL), ('different','workspace-a','mismatched','bob@example.com','verified','2026-01-01'), ('first','workspace-a','ambiguous','alice@example.com','verified','2026-01-01'), ('second','workspace-a','ambiguous','bob@example.com','verified','2026-01-01')`,
		`UPDATE support_conversations SET portal_visibility_changed_at = '2026-01-01' WHERE id = 'hidden'`,
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
	requests, err := repo.ListRequests(context.Background(), "workspace-a", "identity-b", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 0 {
		t.Fatalf("existing ownership changed: %v", requests)
	}
	if err := db.Table("support_conversations").Where("id = ?", "email-match").Updates(map[string]any{"portal_visible": false, "portal_visibility_changed_at": "2026-01-01"}).Error; err != nil {
		t.Fatal(err)
	}
	requests, err = repo.ListRequests(context.Background(), "workspace-a", "identity-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 {
		t.Fatalf("hidden request exposed: %v", requests)
	}
	if err := db.Exec(`UPDATE support_conversations SET status = 'waiting_on_customer', created_at = '2026-01-01 12:00:00', updated_at = '2026-01-03 12:00:00', last_public_message_at = '2026-01-02 12:00:00' WHERE id = 'contact-match'`).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		filter string
		want   int
	}{
		{"active", 0}, {"waiting_on_customer", 1}, {"resolved", 0}, {"", 1},
	} {
		items, err := repo.ListRequests(context.Background(), "workspace-a", "identity-a", tc.filter)
		if err != nil || len(items) != tc.want {
			t.Fatalf("filter %q: items=%v err=%v", tc.filter, items, err)
		}
		if tc.want == 1 && (items[0].Status != "waiting_on_customer" || items[0].LastActivityAt == nil || items[0].LastActivityAt.IsZero()) {
			t.Fatalf("unsafe or missing projection: %+v", items[0])
		}
		if tc.want == 1 && items[0].LastActivityAt.Format("2006-01-02") != "2026-01-02" {
			t.Fatalf("last activity should use public message, not internal update: %+v", items[0])
		}
	}
	if err := db.Exec(`UPDATE support_conversations SET last_public_message_at = NULL WHERE id = 'contact-match'`).Error; err != nil {
		t.Fatal(err)
	}
	items, err := repo.ListRequests(context.Background(), "workspace-a", "identity-a", "")
	if err != nil || len(items) != 1 || items[0].LastActivityAt == nil || items[0].LastActivityAt.Format("2006-01-02") != "2026-01-01" {
		t.Fatalf("last activity should fall back to creation, not internal update: %v %v", items, err)
	}
	if err := db.Exec(`UPDATE support_conversations SET status = 'resolved' WHERE id = 'contact-match'`).Error; err != nil {
		t.Fatal(err)
	}
	items, err = repo.ListRequests(context.Background(), "workspace-a", "identity-a", "resolved")
	if err != nil || len(items) != 1 || items[0].Status != "resolved" {
		t.Fatalf("status refresh: %v %v", items, err)
	}
	if err := db.Exec(`UPDATE support_conversations SET status = 'waiting' WHERE id = 'contact-match'`).Error; err != nil {
		t.Fatal(err)
	}
	items, err = repo.ListRequests(context.Background(), "workspace-a", "identity-a", "waiting_on_customer")
	if err != nil || len(items) != 1 || items[0].Status != "waiting_on_customer" {
		t.Fatalf("legacy status projection: %v %v", items, err)
	}
	// A reference already assigned before contradictory widget verification must
	// no longer authorize either the request list or direct request reads.
	if err := db.Exec(`INSERT INTO support_widget_sessions VALUES ('late-conflict','workspace-a','contact-match','bob@example.com','verified','2026-01-02')`).Error; err != nil {
		t.Fatal(err)
	}
	items, err = repo.ListRequests(context.Background(), "workspace-a", "identity-a", "")
	if err != nil || len(items) != 0 {
		t.Fatalf("conflicting widget reference listed: %v %v", items, err)
	}
	var ref string
	if err := db.Table("support_portal_request_references").Where("conversation_id = ?", "contact-match").Pluck("reference", &ref).Error; err != nil {
		t.Fatal(err)
	}
	conversation, err := repo.FindRequest(context.Background(), "workspace-a", "identity-a", ref)
	if err != nil || conversation != nil {
		t.Fatalf("conflicting widget reference readable: %v %v", conversation, err)
	}
	projectionRef, err := NewSupportPortalRepository(db).FindReferenceForIdentity(context.Background(), "workspace-a", ref, "identity-a")
	if err != nil || projectionRef != nil {
		t.Fatalf("conflicting widget reference projected: %v %v", projectionRef, err)
	}
	if err := db.Exec(`DELETE FROM support_widget_sessions WHERE id = 'late-conflict'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE support_conversations SET customer_email = 'bob@example.com' WHERE id = 'contact-match'`).Error; err != nil {
		t.Fatal(err)
	}
	conversation, err = repo.FindRequest(context.Background(), "workspace-a", "identity-a", ref)
	if err != nil || conversation != nil {
		t.Fatalf("changed customer email readable: %v %v", conversation, err)
	}
}

func TestPortalAuthReconcileRequestsAfterSessionEstablished(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE support_portal_identities (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT)`,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT, customer_email TEXT, portal_visible BOOLEAN, portal_visibility_changed_at DATETIME, primary_recipient_state TEXT, status TEXT, channel TEXT, source TEXT, deleted_at DATETIME, subject TEXT, created_at DATETIME, last_public_message_at DATETIME, resolved_at DATETIME)`,
		`CREATE TABLE support_widget_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, customer_email TEXT, identity_trust TEXT, identity_verified_at DATETIME)`,
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT UNIQUE, portal_identity_id TEXT, reference TEXT UNIQUE, created_at DATETIME)`,
		`INSERT INTO support_portal_identities VALUES ('alice','ws','alice@example.com')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewPortalAuthRepository(db)
	ctx := context.Background()
	if err := repo.ReconcileRequests(ctx, "ws", "missing"); err == nil {
		t.Fatal("unknown identity must not reconcile")
	}
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, customer_email, portal_visible, status, channel, source) VALUES ('email','ws','alice@example.com',0,'open','email','email'), ('widget','ws','alice@example.com',0,'open','widget','widget')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.ReconcileRequests(ctx, "ws", "alice"); err != nil {
		t.Fatal(err)
	}
	items, err := repo.ListRequests(ctx, "ws", "alice", "")
	if err != nil || len(items) != 1 {
		t.Fatalf("new email request not discovered: %v %v", items, err)
	}
	if err := db.Exec(`INSERT INTO support_widget_sessions VALUES ('verified','ws','widget','alice@example.com','verified','2026-01-01')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.ReconcileRequests(ctx, "ws", "alice"); err != nil {
		t.Fatal(err)
	}
	items, err = repo.ListRequests(ctx, "ws", "alice", "")
	if err != nil || len(items) != 2 {
		t.Fatalf("new verified widget request not discovered: %v %v", items, err)
	}
}
