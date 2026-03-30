package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type stubSupportLinkPreviewer struct{}

func (stubSupportLinkPreviewer) EnrichMessage(_ context.Context, msg *model.SupportMessage) {
	if msg == nil || !strings.Contains(msg.Content, "https://example.com") {
		return
	}
	metadata, err := mergeSupportLinkPreviewMetadata(msg.Metadata, []model.SupportLinkPreview{{
		URL:   "https://example.com",
		Title: "Example",
		Host:  "example.com",
	}})
	if err != nil {
		return
	}
	msg.Metadata = metadata
}

func TestCreateConversationMessageStoresLinkPreviewMetadata(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-support-preview"
	userID := "user-support-preview"
	seedUser(t, db, userID, "agent@example.com", "Agent Example", "hash")
	seedWorkspace(t, db, workspaceID, "Preview WS", "preview-ws", userID)

	convRepo := repository.NewSupportConversationRepository(db)
	msgRepo := repository.NewSupportMessageRepository(db)
	conv := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Need a preview",
		Status:      "open",
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		msgRepo,
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsHelpcenterRepository(db),
	)
	svc.SetLinkPreviewService(stubSupportLinkPreviewer{})

	message, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "See https://example.com", MessageType: "reply"},
		"user",
		&userID,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}
	if !strings.Contains(message.Metadata, `"link_previews"`) {
		t.Fatalf("expected link preview metadata on created message, got %q", message.Metadata)
	}

	stored, err := msgRepo.GetByID(ctx, message.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored == nil || !strings.Contains(stored.Metadata, `"link_previews"`) {
		t.Fatalf("expected stored message metadata to include link preview, got %#v", stored)
	}
}

func TestWidgetCreateMessageStoresLinkPreviewMetadata(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-widget-preview"
	seedWorkspace(t, db, workspaceID, "Widget Preview WS", "widget-preview", "user-123")

	convRepo := repository.NewSupportConversationRepository(db)
	msgRepo := repository.NewSupportMessageRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	installationRepo := repository.NewSupportInboxInstallationRepository(db)

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		msgRepo,
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
	svc.SetLinkPreviewService(stubSupportLinkPreviewer{})

	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		WidgetKey:   "widget-key",
		SecretKey:   "widget-secret",
		Settings:    "{}",
		Active:      true,
	}); err != nil {
		t.Fatalf("create installation: %v", err)
	}

	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "widget-preview-session",
		AnonymousID:  "anon-preview",
		IsAnonymous:  true,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	message, err := svc.WidgetCreateMessage(ctx, session.SessionToken, "Customer shared https://example.com", nil)
	if err != nil {
		t.Fatalf("WidgetCreateMessage: %v", err)
	}
	if !strings.Contains(message.Metadata, `"link_previews"`) {
		t.Fatalf("expected widget message metadata to include link preview, got %q", message.Metadata)
	}
}
