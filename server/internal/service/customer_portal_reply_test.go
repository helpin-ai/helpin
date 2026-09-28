package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPortalMessageVisible(t *testing.T) {
	cases := []struct {
		name    string
		message model.SupportMessage
		want    bool
	}{
		{"public reply", model.SupportMessage{MessageType: "reply", SenderType: "user"}, true},
		{"portal reply", model.SupportMessage{MessageType: "reply", SenderType: "customer"}, true},
		{"email only", model.SupportMessage{MessageType: "reply", SenderType: "user", Metadata: `{"delivery_mode":"email_only"}`}, false},
		{"internal note", model.SupportMessage{MessageType: "reply", SenderType: "user", IsInternal: true}, false},
		{"routing event", model.SupportMessage{MessageType: "system", SenderType: "user"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := portalMessageVisible(&tc.message); got != tc.want {
				t.Fatalf("portalMessageVisible() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReopenForCustomerReplyFailurePreservesResolvedState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// A missing conversations table forces the update to fail before message insertion.
	svc := &SupportInboxService{conversationRepo: repository.NewSupportConversationRepository(db)}
	conv := &model.SupportConversation{Status: model.SupportConversationStatusResolved}
	if err := svc.reopenForCustomerReply(context.Background(), "ws", "conv", conv); err == nil {
		t.Fatal("expected reopen failure")
	}
	if conv.Status != model.SupportConversationStatusResolved {
		t.Fatalf("conversation status changed despite failed reopen: %s", conv.Status)
	}
}

func TestPortalReplyDoesNotAppendWhenReopenFails(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	conv := &model.SupportConversation{ID: "portal-reopen-conv", WorkspaceID: "ws-portal-reopen", Subject: "Resolved", Status: model.SupportConversationStatusResolved}
	if err := db.Exec("INSERT INTO support_conversations (id, workspace_id, display_id, subject, status, channel, source) VALUES (?, ?, ?, ?, ?, ?, ?)", conv.ID, conv.WorkspaceID, 1, conv.Subject, conv.Status, "widget", "widget").Error; err != nil {
		t.Fatal(err)
	}
	callback := "test:reject_portal_reopen"
	errReopen := errors.New("reopen unavailable")
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "support_conversations" {
			tx.AddError(errReopen)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })
	svc := &SupportInboxService{conversationRepo: convRepo, messageRepo: messageRepo}
	_, err := svc.CreateConversationMessage(context.WithValue(ctx, portalReplySourceKey{}, true), conv.WorkspaceID, conv.ID,
		model.CreateMessageRequest{Content: "Please reopen", MessageType: "reply"}, "customer", nil, nil, nil)
	if err == nil || !errors.Is(err, errReopen) {
		t.Fatalf("expected reopen failure, got %v", err)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("conversation_id = ?", conv.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("appended %d messages despite failed reopen", count)
	}
}

func TestPortalReplyRollsBackReopenWhenAttachmentLinkFails(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	conv := &model.SupportConversation{ID: "portal-attachment-rollback", WorkspaceID: "ws-portal-attachment", Subject: "Resolved", Status: model.SupportConversationStatusResolved}
	if err := db.Exec("INSERT INTO support_conversations (id, workspace_id, display_id, subject, status, channel, source) VALUES (?, ?, ?, ?, ?, ?, ?)", conv.ID, conv.WorkspaceID, 1, conv.Subject, conv.Status, "portal", "portal").Error; err != nil {
		t.Fatal(err)
	}
	svc := &SupportInboxService{conversationRepo: repository.NewSupportConversationRepository(db), messageRepo: repository.NewSupportMessageRepository(db)}
	ctx = context.WithValue(ctx, portalReplySourceKey{}, true)
	ctx = context.WithValue(ctx, portalReplyAuditKey{}, portalReplyAudit{IdentityID: "identity", SessionID: "session"})
	_, err := svc.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID,
		model.CreateMessageRequest{Content: "Please reopen", MessageType: "reply", AttachmentIDs: []string{"missing"}}, "customer", nil, nil, nil)
	if !errors.Is(err, ErrPortalAttachmentsInvalid) {
		t.Fatalf("expected attachment failure, got %v", err)
	}
	var stored model.SupportConversation
	if err := db.First(&stored, "id = ?", conv.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.SupportConversationStatusResolved {
		t.Fatalf("conversation reopened despite failed attachment link: %s", stored.Status)
	}
	var messages int64
	if err := db.Model(&model.SupportMessage{}).Where("conversation_id = ?", conv.ID).Count(&messages).Error; err != nil {
		t.Fatal(err)
	}
	if messages != 0 {
		t.Fatalf("created %d messages despite failed attachment link", messages)
	}
}

func TestPortalReplyReopenWritesAuditEvents(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE support_portal_audit_events (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, actor_type TEXT, actor_user_id TEXT, event_type TEXT, metadata TEXT, occurred_at DATETIME, created_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	conv := &model.SupportConversation{ID: "portal-audit-reply", WorkspaceID: "ws-portal-audit", Subject: "Resolved", Status: model.SupportConversationStatusResolved}
	if err := db.Exec("INSERT INTO support_conversations (id, workspace_id, display_id, subject, status, channel, source) VALUES (?, ?, ?, ?, ?, ?, ?)", conv.ID, conv.WorkspaceID, 1, conv.Subject, conv.Status, "portal", "portal").Error; err != nil {
		t.Fatal(err)
	}
	svc := &SupportInboxService{conversationRepo: repository.NewSupportConversationRepository(db), messageRepo: repository.NewSupportMessageRepository(db)}
	ctx := context.WithValue(context.Background(), portalReplySourceKey{}, true)
	ctx = context.WithValue(ctx, portalReplyAuditKey{}, portalReplyAudit{IdentityID: "identity", SessionID: "session"})
	if _, err := svc.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, model.CreateMessageRequest{Content: "Please reopen", MessageType: "reply"}, "customer", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	var stored model.SupportConversation
	if err := db.First(&stored, "id = ?", conv.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.SupportConversationStatusOpen {
		t.Fatalf("status = %s, want open", stored.Status)
	}
	var events []model.SupportPortalAuditEvent
	if err := db.Where("conversation_id = ?", conv.ID).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].EventType != model.SupportPortalAuditReplyCreated || events[1].EventType != model.SupportPortalAuditRequestReopened {
		t.Fatalf("unexpected audit events: %+v", events)
	}
}
