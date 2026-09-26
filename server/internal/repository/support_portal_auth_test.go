package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPortalCreateRequestProjectsFirstCustomerMessage(t *testing.T) {
	db := setupSupportConversationMessageTestDB(t)
	for _, statement := range []string{
		`ALTER TABLE support_conversations ADD COLUMN portal_visible BOOLEAN NOT NULL DEFAULT 0`,
		`ALTER TABLE support_conversations ADD COLUMN handoff_state TEXT`,
		`ALTER TABLE support_conversations ADD COLUMN primary_recipient_state TEXT`,
		`ALTER TABLE support_conversations ADD COLUMN suggested_primary_recipient_email TEXT`,
		`ALTER TABLE support_conversations ADD COLUMN suggested_primary_recipient_name TEXT`,
		`ALTER TABLE support_conversations ADD COLUMN email_cc TEXT`,
		`ALTER TABLE support_conversations ADD COLUMN email_thread_participants TEXT`,
		`ALTER TABLE support_conversations ADD COLUMN crm_company_id TEXT`,
		`ALTER TABLE support_conversations ADD COLUMN portal_visibility_changed_at DATETIME`,
		`ALTER TABLE support_conversations ADD COLUMN deleted_at DATETIME`,
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, reference TEXT, created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	workspaceID := uuid.NewString()
	identity := &model.SupportPortalIdentity{ID: uuid.NewString(), WorkspaceID: workspaceID, Email: "customer@example.com"}
	flowState := model.SupportConversationFlowStateWaitingForHuman
	conversation, request, err := NewPortalAuthRepository(db).CreateRequest(context.Background(), workspaceID, identity, "Help", "Please help", "REQ-123", nil, nil, nil, &flowState, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if request.Reference != "REQ-123" || request.LastActivityAt == nil {
		t.Fatalf("unexpected portal request: %+v", request)
	}
	var stored model.SupportConversation
	if err := db.First(&stored, "id = ?", conversation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ListLastMessagePreview == nil || *stored.ListLastMessagePreview != "Please help" || stored.ListLastMessageAt == nil || stored.LastPublicSenderType == nil || *stored.LastPublicSenderType != "customer" || stored.LastCustomerMessageID == nil || stored.UnansweredCustomerMessageCount != 1 || !stored.CustomerAwaitingResponse || !stored.NeedsHumanReply || stored.SupportStateVersion != 1 {
		t.Fatalf("first customer message not projected into inbox: %+v", stored)
	}
	var count int64
	if err := db.Model(&model.SupportPortalRequestReference{}).Where("conversation_id = ? AND portal_identity_id = ?", conversation.ID, identity.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("portal request reference count=%d err=%v", count, err)
	}
}

func TestPortalAuthTokensAreScopedSingleUseAndRevocable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE support_portal_magic_links (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT, token_hash TEXT, expires_at DATETIME, used_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_portal_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, identity_id TEXT, token_hash TEXT, expires_at DATETIME, revoked_at DATETIME, created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewPortalAuthRepository(db)
	ctx := context.Background()
	now := time.Now()
	link := &model.PortalMagicLink{ID: "link", WorkspaceID: "workspace-a", Email: "customer@example.com", TokenHash: "link-hash", ExpiresAt: now.Add(time.Minute)}
	if err := repo.CreateLink(ctx, link); err != nil {
		t.Fatal(err)
	}
	create := func(tx *gorm.DB, link *model.PortalMagicLink) (*model.PortalSession, error) {
		return &model.PortalSession{ID: "session", WorkspaceID: link.WorkspaceID, IdentityID: "identity", TokenHash: "session-hash", ExpiresAt: now.Add(time.Hour)}, nil
	}
	if _, err := repo.ConsumeLink(ctx, "workspace-b", "link-hash", now, create); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-workspace exchange: %v", err)
	}
	if _, err := repo.ConsumeLink(ctx, "workspace-a", "link-hash", now.Add(time.Hour), create); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expired exchange: %v", err)
	}
	if _, err := repo.ConsumeLink(ctx, "workspace-a", "link-hash", now, create); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ConsumeLink(ctx, "workspace-a", "link-hash", now, create); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("reused exchange: %v", err)
	}
	if _, err := repo.FindSession(ctx, "workspace-b", "session-hash", now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-workspace session: %v", err)
	}
	if _, err := repo.FindSession(ctx, "workspace-a", "session-hash", now); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeSession(ctx, "workspace-a", "session-hash", now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindSession(ctx, "workspace-a", "session-hash", now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("revoked session: %v", err)
	}
}
