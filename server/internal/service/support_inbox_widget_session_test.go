package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportInboxServiceSessionConversationLifecycle(t *testing.T) {
	db := newTestDB(t)

	workspaceID := "ws-widget-service"
	seedWorkspace(t, db, workspaceID, "Widget Service WS", "widget-service-ws", "user-123")

	ctx := context.Background()
	conversationRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)

	svc := NewSupportInboxService(
		conversationRepo,
		messageRepo,
		nil,
		nil,
		nil,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
	)

	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "widget-session-token",
		AnonymousID:  "anon-1",
		IsAnonymous:  true,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	ownedConversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Owned conversation",
		Status:      "open",
		AnonymousID: strPtr("anon-1"),
	}
	if err := conversationRepo.Create(ctx, ownedConversation); err != nil {
		t.Fatalf("create owned conversation: %v", err)
	}

	otherConversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Other visitor conversation",
		Status:      "open",
		AnonymousID: strPtr("anon-2"),
	}
	if err := conversationRepo.Create(ctx, otherConversation); err != nil {
		t.Fatalf("create other conversation: %v", err)
	}

	if err := svc.SetSessionConversation(ctx, session.SessionToken, ownedConversation.ID); err != nil {
		t.Fatalf("set session conversation: %v", err)
	}

	fetched, err := sessionRepo.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if fetched.ConversationID == nil || *fetched.ConversationID != ownedConversation.ID {
		t.Fatalf("expected conversation_id %q, got %v", ownedConversation.ID, fetched.ConversationID)
	}

	if err := svc.ClearSessionConversation(ctx, session.SessionToken); err != nil {
		t.Fatalf("clear session conversation: %v", err)
	}

	fetched, err = sessionRepo.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatalf("get session after clear: %v", err)
	}
	if fetched.ConversationID != nil {
		t.Fatalf("expected cleared conversation_id, got %v", *fetched.ConversationID)
	}

	if err := svc.SetSessionConversation(ctx, session.SessionToken, otherConversation.ID); err == nil {
		t.Fatal("expected selecting another visitor's conversation to fail")
	}
}
