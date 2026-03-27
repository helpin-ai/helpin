package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportInboxServiceWidgetSessionLifecycle(t *testing.T) {
	db := newTestDB(t)

	const (
		workspaceID = "ws-widget-lifecycle"
		widgetKey   = "wk_widget_lifecycle"
	)

	seedWorkspace(t, db, workspaceID, "Widget Lifecycle WS", "widget-lifecycle", "user-123")

	ctx := context.Background()
	installationRepo := repository.NewSupportInboxInstallationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)

	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		WidgetKey:   widgetKey,
		SecretKey:   "sk_widget_lifecycle",
		Settings:    "{}",
		Active:      true,
	}); err != nil {
		t.Fatalf("create installation: %v", err)
	}

	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		installationRepo,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	name := "Jane Widget"
	email := "jane@example.com"
	pageURL := "https://example.com/pricing"
	timezone := "America/New_York"
	locale := "en-US"

	identified, err := svc.CreateWidgetSession(ctx, widgetKey, "anon-identified", &name, &email, nil, &pageURL, &timezone, &locale)
	if err != nil {
		t.Fatalf("CreateWidgetSession identified: %v", err)
	}
	if identified.WorkspaceID != workspaceID {
		t.Fatalf("workspace_id = %q, want %q", identified.WorkspaceID, workspaceID)
	}
	if identified.SessionToken == "" || len(identified.SessionToken) != 64 {
		t.Fatalf("session_token = %q, want 64-char token", identified.SessionToken)
	}
	if identified.IsAnonymous {
		t.Fatal("expected identified session to be non-anonymous")
	}
	if identified.CustomerEmail == nil || *identified.CustomerEmail != email {
		t.Fatalf("customer_email = %v, want %q", identified.CustomerEmail, email)
	}
	if identified.LastPageURL == nil || *identified.LastPageURL != pageURL {
		t.Fatalf("last_page_url = %v, want %q", identified.LastPageURL, pageURL)
	}
	if remaining := time.Until(identified.ExpiresAt); remaining < (29*24*time.Hour) || remaining > (31*24*time.Hour) {
		t.Fatalf("expires_at remaining = %v, want about 30 days", remaining)
	}

	fetched, err := svc.GetWidgetSession(ctx, identified.SessionToken)
	if err != nil {
		t.Fatalf("GetWidgetSession: %v", err)
	}
	if fetched.ID != identified.ID {
		t.Fatalf("session id = %q, want %q", fetched.ID, identified.ID)
	}

	anonymous, err := svc.CreateWidgetSession(ctx, widgetKey, "anon-anonymous", nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateWidgetSession anonymous: %v", err)
	}
	if !anonymous.IsAnonymous {
		t.Fatal("expected anonymous session to be marked anonymous")
	}
	if anonymous.CustomerEmail != nil {
		t.Fatalf("customer_email = %v, want nil", anonymous.CustomerEmail)
	}

	if err := svc.RevokeWidgetSession(ctx, identified.SessionToken); err != nil {
		t.Fatalf("RevokeWidgetSession: %v", err)
	}

	if _, err := svc.GetWidgetSession(ctx, identified.SessionToken); err == nil || !strings.Contains(err.Error(), "session revoked") {
		t.Fatalf("GetWidgetSession after revoke error = %v, want revoked", err)
	}
}

func TestSupportInboxServiceCreateWidgetSessionRejectsInvalidKey(t *testing.T) {
	db := newTestDB(t)

	ctx := context.Background()
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	installationRepo := repository.NewSupportInboxInstallationRepository(db)

	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		installationRepo,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if _, err := svc.CreateWidgetSession(ctx, "wk_missing", "anon-1", nil, nil, nil, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "invalid widget key") {
		t.Fatalf("CreateWidgetSession error = %v, want invalid widget key", err)
	}
}

func TestSupportInboxServiceGetWidgetSessionRejectsExpired(t *testing.T) {
	db := newTestDB(t)

	const workspaceID = "ws-widget-expired"
	seedWorkspace(t, db, workspaceID, "Widget Expired WS", "widget-expired", "user-123")

	ctx := context.Background()
	sessionRepo := repository.NewSupportInboxSessionRepository(db)

	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		nil,
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	expired := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "expired-session-token",
		AnonymousID:  "anon-expired",
		IsAnonymous:  true,
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, expired); err != nil {
		t.Fatalf("create expired session: %v", err)
	}

	if _, err := svc.GetWidgetSession(ctx, expired.SessionToken); err == nil || !strings.Contains(err.Error(), "session expired") {
		t.Fatalf("GetWidgetSession error = %v, want expired", err)
	}
}

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
