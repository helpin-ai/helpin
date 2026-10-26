//go:build integration

package service

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestWebhookTriggersPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	mustExec(t, db, `DELETE FROM webhook_events`)
	mustExec(t, db, `DELETE FROM webhook_deliveries`)
	receiver := &webhookReceiver{}
	server := httptest.NewServer(receiver.handler())
	defer server.Close()
	messages := repository.NewSupportMessageRepository(db)
	inbox := NewSupportInboxService(repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db), messages, nil, nil,
		repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil, repository.NewUserRepository(db), nil, nil, nil)
	inbox.tagRepo = repository.NewSupportTagRepository(db)
	webhooks := NewWebhookService(repository.NewWebhookRepository(db), inbox, WebhookConfig{EncryptionKey: []byte(strings.Repeat("k", 32)), AllowPrivateNetworks: true})
	webhooks.concurrency = 1 // The test database is one transaction.

	// Without an endpoint, triggers record nothing.
	quiet := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 1, Subject: "Quiet", Status: model.SupportConversationStatusOpen, Priority: "medium", Channel: "email", CreatedAt: time.Now().UTC()}
	if err := db.Create(quiet).Error; err != nil {
		t.Fatal(err)
	}
	var events int64
	db.Model(&model.WebhookEvent{}).Where("workspace_id = ?", workspaceID).Count(&events)
	if events != 0 {
		t.Fatalf("events recorded without endpoints: %d", events)
	}

	if _, err := webhooks.CreateEndpoint(ctx, workspaceID, userID, model.WebhookEndpointRequest{URL: server.URL}); err != nil {
		t.Fatal(err)
	}
	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 2, Subject: "Hooks", Status: model.SupportConversationStatusOpen, Priority: "medium", Channel: "email", CreatedAt: time.Now().UTC()}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	imported := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 3, Subject: "Old", Status: model.SupportConversationStatusResolved, Priority: "medium", Channel: "email", CreatedAt: time.Now().Add(-48 * time.Hour)}
	if err := db.Create(imported).Error; err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE support_conversations SET priority = 'urgent', assigned_user_id = ?, snoozed_until = now() + interval '1 day' WHERE id = ?`, userID, conversation.ID)
	mustExec(t, db, `UPDATE support_conversations SET status = 'resolved' WHERE id = ?`, conversation.ID)
	for _, message := range []*model.SupportMessage{
		{WorkspaceID: workspaceID, ConversationID: conversation.ID, SenderType: "customer", MessageType: "reply", Content: "Public"},
		{WorkspaceID: workspaceID, ConversationID: conversation.ID, SenderType: "user", MessageType: "note", IsInternal: true, Content: "Internal"},
		{WorkspaceID: workspaceID, ConversationID: imported.ID, SenderType: "customer", MessageType: "reply", Content: "Old", CreatedAt: time.Now().Add(-48 * time.Hour)},
	} {
		if err := messages.Create(ctx, message); err != nil {
			t.Fatal(err)
		}
	}
	var types []string
	db.Model(&model.WebhookEvent{}).Where("workspace_id = ?", workspaceID).Order("event_type").Pluck("event_type", &types)
	want := []string{"conversation.assigned", "conversation.created", "conversation.priority_changed", "conversation.snoozed", "conversation.status_changed", "message.created"}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("trigger events = %v, want %v", types, want)
	}
	var status model.WebhookEvent
	db.Where("workspace_id = ? AND event_type = 'conversation.status_changed'", workspaceID).Take(&status)
	if !strings.Contains(string(status.Data), `"from": "open"`) || !strings.Contains(string(status.Data), `"to": "resolved"`) {
		t.Fatalf("status change data: %s", status.Data)
	}
	if _, err := webhooks.ProcessEvents(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if sent, err := webhooks.ProcessDeliveries(ctx, 100); err != nil || sent != len(want) {
		t.Fatalf("deliveries sent: %d %v", sent, err)
	}
	var succeeded int64
	db.Model(&model.WebhookDelivery{}).Where("workspace_id = ? AND status = ?", workspaceID, model.WebhookDeliverySucceeded).Count(&succeeded)
	if succeeded != int64(len(want)) || receiver.count() != len(want) {
		t.Fatalf("succeeded=%d received=%d", succeeded, receiver.count())
	}
	db.Model(&model.WebhookEvent{}).Where("workspace_id = ?", workspaceID).Count(&events)
	if events != 0 {
		t.Fatalf("%d events left in the outbox", events)
	}
}
