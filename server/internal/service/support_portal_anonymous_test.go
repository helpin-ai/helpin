package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPortalAnonymousIntakeCreatesOneAuditedRequest(t *testing.T) {
	db := newTestDB(t)
	for _, statement := range []string{
		`CREATE TABLE support_portal_intake_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT, token_hash TEXT UNIQUE, expires_at DATETIME, used_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_portal_identities (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT, display_name TEXT, auth_subject TEXT, created_at DATETIME, updated_at DATETIME, UNIQUE(workspace_id, email))`,
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, reference TEXT UNIQUE, created_at DATETIME)`,
		`CREATE TABLE support_portal_audit_events (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, actor_type TEXT, actor_user_id TEXT, event_type TEXT, metadata TEXT, occurred_at DATETIME, created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil { t.Fatal(err) }
	}
	workspaceID := uuid.NewString()
	if err := db.Exec(`INSERT INTO support_widget_installations (id, workspace_id, widget_key, secret_key, settings, active) VALUES (?, ?, 'key', 'secret', ?, true)`, uuid.NewString(), workspaceID, `{"portal_enabled":true,"portal_intake_enabled":true,"portal_anonymous_intake_enabled":true}`).Error; err != nil { t.Fatal(err) }
	inbox := NewSupportInboxService(repository.NewSupportConversationRepository(db), nil, repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil)
	svc := NewPortalAuthService(repository.NewPortalAuthRepository(db), inbox, nil, "")
	ctx := context.Background()
	token, err := svc.StartAnonymousIntake(ctx, workspaceID, "Customer@Example.com")
	if err != nil { t.Fatal(err) }
	if err := svc.CreateAnonymousRequest(ctx, workspaceID, token, "Help", "Please help", nil); err != nil { t.Fatal(err) }
	if err := svc.CreateAnonymousRequest(ctx, workspaceID, token, "Again", "Duplicate", nil); err == nil { t.Fatal("one-use intake token created a second request") }
	var conversation model.SupportConversation
	if err := db.Where("workspace_id = ? AND channel = 'portal'", workspaceID).First(&conversation).Error; err != nil { t.Fatal(err) }
	if !conversation.PortalVisible || conversation.CustomerEmail == nil || *conversation.CustomerEmail != "customer@example.com" { t.Fatalf("unexpected request: %+v", conversation) }
	var count int64
	if err := db.Model(&model.SupportPortalAuditEvent{}).Where("conversation_id = ? AND event_type = ?", conversation.ID, model.SupportPortalAuditRequestCreated).Count(&count).Error; err != nil || count != 1 { t.Fatalf("request audit count=%d err=%v", count, err) }
}
