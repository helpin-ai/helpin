package repository

import (
	"context"
	"errors"
	"strings"
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
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, reference TEXT, customer_last_read_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_portal_audit_events (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, actor_type TEXT, actor_user_id TEXT, event_type TEXT, metadata TEXT, occurred_at DATETIME, created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	workspaceID := uuid.NewString()
	identity := &model.SupportPortalIdentity{ID: uuid.NewString(), WorkspaceID: workspaceID, Email: "customer@example.com"}
	flowState := model.SupportConversationFlowStateWaitingForHuman
	conversation, request, err := NewCustomerPortalRepository(db).CreateRequest(context.Background(), PortalRequestDraft{
		WorkspaceID: workspaceID, Identity: identity, Email: identity.Email, Subject: "Help", Description: "Please help", FlowState: &flowState,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(request.Reference, "req_") || request.LastActivityAt == nil || !stored(t, db, conversation.ID).PortalVisible {
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
		`CREATE TABLE support_portal_magic_links (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT, token_hash TEXT, conversation_id TEXT, expires_at DATETIME, used_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_portal_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, identity_id TEXT, crm_contact_id TEXT, token_hash TEXT, expires_at DATETIME, revoked_at DATETIME, reconciled_at DATETIME, created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewCustomerPortalRepository(db)
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

func TestPortalIntakeSessionIsScopedOneUseAndRollsBack(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE support_portal_intake_sessions (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, email TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE, expires_at DATETIME NOT NULL, used_at DATETIME, created_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewCustomerPortalRepository(db)
	ctx := context.Background()
	now := time.Now()
	session := &model.PortalIntakeSession{ID: uuid.NewString(), WorkspaceID: "workspace-a", Email: "customer@example.com", TokenHash: "hash", ExpiresAt: now.Add(time.Hour)}
	if err := repo.CreateIntakeSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindIntakeSession(ctx, "workspace-b", "hash", now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-workspace intake token: %v", err)
	}
	if _, err := repo.FindIntakeSession(ctx, "workspace-a", "hash", now.Add(time.Hour)); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expired intake token: %v", err)
	}
	failure := errors.New("request creation failed")
	if err := repo.ConsumeIntakeSession(ctx, "workspace-a", "hash", now, func(_ *gorm.DB, _ *model.PortalIntakeSession) error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("failed request: %v", err)
	}
	if _, err := repo.FindIntakeSession(ctx, "workspace-a", "hash", now); err != nil {
		t.Fatalf("failed request consumed token: %v", err)
	}
	created := 0
	if err := repo.ConsumeIntakeSession(ctx, "workspace-a", "hash", now, func(_ *gorm.DB, current *model.PortalIntakeSession) error {
		if current.Email != session.Email {
			t.Fatalf("wrong email: %s", current.Email)
		}
		created++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ConsumeIntakeSession(ctx, "workspace-a", "hash", now, func(_ *gorm.DB, _ *model.PortalIntakeSession) error { created++; return nil }); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("reused intake token: %v", err)
	}
	if created != 1 {
		t.Fatalf("created %d requests with one token", created)
	}
}

func stored(t *testing.T, db *gorm.DB, conversationID string) model.SupportConversation {
	t.Helper()
	var conversation model.SupportConversation
	if err := db.First(&conversation, "id = ?", conversationID).Error; err != nil {
		t.Fatal(err)
	}
	return conversation
}

func TestPortalMarkSessionReconciledOncePerInterval(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE support_portal_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, identity_id TEXT, crm_contact_id TEXT, token_hash TEXT, expires_at DATETIME, revoked_at DATETIME, reconciled_at DATETIME, created_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO support_portal_sessions (id, workspace_id, identity_id, token_hash, expires_at) VALUES ('session', 'ws', 'identity', 'hash', ?)`, time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewCustomerPortalRepository(db)
	ctx := context.Background()
	now := time.Now()
	for _, tc := range []struct {
		name string
		at   time.Time
		want bool
	}{
		{"first read reconciles", now, true},
		{"read within interval skips", now.Add(time.Minute), false},
		{"read after interval reconciles", now.Add(6 * time.Minute), true},
	} {
		got, err := repo.MarkSessionReconciled(ctx, "session", tc.at, 5*time.Minute)
		if err != nil || got != tc.want {
			t.Fatalf("%s: got %v err %v, want %v", tc.name, got, err, tc.want)
		}
	}
}
