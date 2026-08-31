package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
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

func serializeSupportLinkPreviewTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	// The preview write runs in a goroutine. A shared-cache in-memory SQLite
	// database returns SQLITE_LOCKED instead of waiting when the assertion read
	// overlaps that write, so keep this integration test on one connection.
	sqlDB.SetMaxOpenConns(1)
}

func TestCreateConversationMessageStoresLinkPreviewMetadata(t *testing.T) {
	db := newTestDB(t)
	serializeSupportLinkPreviewTestDB(t, db)
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
		repository.NewDocsCollectionRepository(db, false),
		repository.NewDocsHelpcenterRepository(db, false),
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
	// Link enrichment runs after creation so the reply endpoint is not blocked
	// by external page fetches. The persisted metadata arrives asynchronously.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		stored, getErr := msgRepo.GetByID(ctx, message.ID)
		if getErr != nil {
			t.Fatalf("GetByID: %v", getErr)
		}
		if stored != nil && strings.Contains(stored.Metadata, `"link_previews"`) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected stored message metadata to include link preview")
}

func TestWidgetCreateMessageStoresLinkPreviewMetadata(t *testing.T) {
	db := newTestDB(t)
	serializeSupportLinkPreviewTestDB(t, db)
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
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		stored, getErr := msgRepo.GetByID(ctx, message.ID)
		if getErr != nil {
			t.Fatalf("GetByID: %v", getErr)
		}
		if stored != nil && strings.Contains(stored.Metadata, `"link_previews"`) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected widget message metadata to include link preview")
}
