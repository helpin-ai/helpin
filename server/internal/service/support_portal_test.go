package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSupportPortalService(t *testing.T) (*SupportPortalService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, customer_email TEXT, subject TEXT NOT NULL, status TEXT NOT NULL, channel TEXT NOT NULL, source TEXT NOT NULL, portal_visible BOOLEAN NOT NULL, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE support_widget_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, customer_email TEXT, identity_trust TEXT, identity_verified_at DATETIME)`,
		`CREATE TABLE support_portal_identities (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, email TEXT NOT NULL, display_name TEXT, auth_subject TEXT, created_at DATETIME, updated_at DATETIME)`,
		`CREATE UNIQUE INDEX idx_support_portal_identity_email ON support_portal_identities (workspace_id, email)`,
		`CREATE UNIQUE INDEX idx_support_portal_identity_subject ON support_portal_identities (workspace_id, auth_subject) WHERE auth_subject IS NOT NULL`,
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, conversation_id TEXT NOT NULL, portal_identity_id TEXT NOT NULL, reference TEXT NOT NULL, created_at DATETIME)`,
		`CREATE UNIQUE INDEX idx_support_portal_request_conversation ON support_portal_request_references (workspace_id, conversation_id)`,
		`CREATE UNIQUE INDEX idx_support_portal_request_reference ON support_portal_request_references (workspace_id, reference)`,
		`CREATE TABLE support_portal_audit_events (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, conversation_id TEXT NOT NULL, portal_identity_id TEXT, actor_type TEXT NOT NULL, actor_user_id TEXT, event_type TEXT NOT NULL, metadata TEXT NOT NULL DEFAULT '{}', occurred_at DATETIME NOT NULL, created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return NewSupportPortalService(repository.NewSupportPortalRepository(db)), db
}

func createSupportPortalConversation(t *testing.T, db *gorm.DB, conversation *model.SupportConversation) {
	t.Helper()
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, customer_email, subject, status, channel, source, portal_visible, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conversation.ID, conversation.WorkspaceID, conversation.CustomerEmail, conversation.Subject, conversation.Status, conversation.Channel, conversation.Source, conversation.PortalVisible, conversation.CreatedAt, conversation.UpdatedAt, conversation.DeletedAt,
	).Error; err != nil {
		t.Fatal(err)
	}
}

func TestSupportPortalProjectsCanonicalConversationWithOpaqueStableReference(t *testing.T) {
	svc, db := setupSupportPortalService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	conversation := &model.SupportConversation{ID: "conversation-internal-id", WorkspaceID: "ws-1", Subject: "Cannot log in", Status: model.SupportConversationStatusOpen, Channel: "email", Source: "email", PortalVisible: true, CreatedAt: now, UpdatedAt: now}
	createSupportPortalConversation(t, db, conversation)
	identity, err := svc.FindOrCreateIdentity(ctx, "ws-1", "Customer@Example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE support_conversations SET customer_email = ? WHERE id = ?", identity.Email, conversation.ID).Error; err != nil {
		t.Fatal(err)
	}
	first, err := svc.EnsureRequestReference(ctx, "ws-1", conversation.ID, identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.EnsureRequestReference(ctx, "ws-1", conversation.ID, identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reference != second.Reference || !strings.HasPrefix(first.Reference, "req_") {
		t.Fatalf("reference was not stable and opaque: %#v %#v", first, second)
	}
	if strings.Contains(first.Reference, conversation.ID) {
		t.Fatalf("reference exposes conversation ID: %q", first.Reference)
	}

	request, err := svc.GetRequestProjection(ctx, "ws-1", first.Reference, identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if request.Reference != first.Reference || request.Subject != conversation.Subject || request.Status != "active" {
		t.Fatalf("unexpected projection: %#v", request)
	}
	if strings.Contains(string(mustPortalJSON(t, request)), conversation.ID) {
		t.Fatalf("projection leaked internal conversation ID: %#v", request)
	}
}

func TestSupportPortalReferenceRollsBackWhenAuditUnavailable(t *testing.T) {
	svc, db := setupSupportPortalService(t)
	ctx := context.Background()
	conversation := &model.SupportConversation{ID: "audit-failure-conversation", WorkspaceID: "ws-1", Subject: "Help", Status: model.SupportConversationStatusOpen, Channel: "email", Source: "email", PortalVisible: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	createSupportPortalConversation(t, db, conversation)
	identity, err := svc.FindOrCreateIdentity(ctx, "ws-1", "customer@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE support_conversations SET customer_email = ? WHERE id = ?", identity.Email, conversation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DROP TABLE support_portal_audit_events").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnsureRequestReference(ctx, "ws-1", conversation.ID, identity.ID); err == nil {
		t.Fatal("expected audit persistence failure")
	}
	var count int64
	if err := db.Model(&model.SupportPortalRequestReference{}).Where("conversation_id = ?", conversation.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("created %d references without audit event", count)
	}
}

func TestSupportPortalDeniesHiddenInternalSpamAndDeletedConversations(t *testing.T) {
	for _, tc := range []struct {
		name         string
		conversation model.SupportConversation
	}{
		{"not visible", model.SupportConversation{PortalVisible: false, Status: "open", Channel: "email", Source: "email"}},
		{"internal", model.SupportConversation{PortalVisible: true, Status: "open", Channel: "internal", Source: "internal"}},
		{"spam", model.SupportConversation{PortalVisible: true, Status: model.SupportConversationStatusSpam, Channel: "email", Source: "email"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := setupSupportPortalService(t)
			conversation := tc.conversation
			conversation.ID = "conv-" + strings.ReplaceAll(tc.name, " ", "-")
			conversation.WorkspaceID = "ws-1"
			conversation.Subject = "private"
			createSupportPortalConversation(t, db, &conversation)
			identity, err := svc.FindOrCreateIdentity(context.Background(), "ws-1", "customer@example.com", nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.EnsureRequestReference(context.Background(), "ws-1", conversation.ID, identity.ID); err != ErrPortalConversationNotVisible {
				t.Fatalf("error = %v, want visibility denial", err)
			}
		})
	}

	t.Run("deleted", func(t *testing.T) {
		svc, db := setupSupportPortalService(t)
		conversation := &model.SupportConversation{ID: "conv-deleted", WorkspaceID: "ws-1", Subject: "deleted", Status: "open", Channel: "email", Source: "email", PortalVisible: true}
		createSupportPortalConversation(t, db, conversation)
		if err := db.Exec("DELETE FROM support_conversations WHERE id = ?", conversation.ID).Error; err != nil {
			t.Fatal(err)
		}
		var deletedCount int64
		if err := db.Raw("SELECT COUNT(*) FROM support_conversations WHERE id = ?", conversation.ID).Scan(&deletedCount).Error; err != nil {
			t.Fatal(err)
		}
		if deletedCount != 0 {
			t.Fatalf("conversation delete left a soft-deleted row")
		}
		identity, err := svc.FindOrCreateIdentity(context.Background(), "ws-1", "customer@example.com", nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.EnsureRequestReference(context.Background(), "ws-1", conversation.ID, identity.ID); err != ErrPortalConversationNotVisible {
			t.Fatalf("error = %v, want visibility denial", err)
		}
	})
}

func TestSupportPortalIdentityAuthSubjectIsScopedToWorkspace(t *testing.T) {
	_, db := setupSupportPortalService(t)
	subject := "customer-subject"
	for _, workspaceID := range []string{"ws-1", "ws-2"} {
		identity := &model.SupportPortalIdentity{
			ID:          "identity-" + workspaceID,
			WorkspaceID: workspaceID,
			Email:       "customer-" + workspaceID + "@example.com",
			AuthSubject: &subject,
		}
		if err := db.Create(identity).Error; err != nil {
			t.Fatalf("create identity in %s: %v", workspaceID, err)
		}
	}
}

func TestSupportPortalReferenceAuthorizationAndAuditAttribution(t *testing.T) {
	svc, db := setupSupportPortalService(t)
	ctx := context.Background()
	conversation := &model.SupportConversation{ID: "conv-1", WorkspaceID: "ws-1", Subject: "help", Status: "open", Channel: "email", Source: "email", PortalVisible: true}
	createSupportPortalConversation(t, db, conversation)
	owner, err := svc.FindOrCreateIdentity(ctx, "ws-1", "owner@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := svc.FindOrCreateIdentity(ctx, "ws-1", "other@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := svc.EnsureRequestReference(ctx, "ws-1", conversation.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetRequestProjection(ctx, "ws-1", ref.Reference, other.ID); err != ErrPortalRequestNotFound {
		t.Fatalf("cross-identity access = %v", err)
	}
	if err := svc.RecordAuditEvent(ctx, "ws-1", conversation.ID, owner.ID, model.SupportPortalActorCustomer, nil, model.SupportPortalAuditReplyCreated, `{"message_id":"message-1"}`); err != nil {
		t.Fatal(err)
	}
	var events []model.SupportPortalAuditEvent
	if err := db.Where("conversation_id = ?", conversation.ID).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].PortalIdentityID == nil || *events[1].PortalIdentityID != owner.ID || events[1].EventType != model.SupportPortalAuditReplyCreated {
		t.Fatalf("audit attribution = %#v", events)
	}
}

func mustPortalJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
