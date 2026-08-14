package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"gorm.io/gorm"
)

func createSupportAttachmentTestTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE support_attachments (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		conversation_id TEXT,
		message_id TEXT,
		file_name TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		content_type TEXT NOT NULL,
		storage_key TEXT NOT NULL DEFAULT '',
		public_url TEXT NOT NULL DEFAULT '',
		is_uploaded BOOLEAN NOT NULL DEFAULT 0,
		uploaded_by_type TEXT NOT NULL,
		uploaded_by_id TEXT,
		session_id TEXT,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create support_attachments: %v", err)
	}
}

func TestSupportAttachmentCreateStagesWidgetUploadWithoutConversation(t *testing.T) {
	db := newTestDB(t)
	createSupportAttachmentTestTable(t, db)
	repo := repository.NewSupportAttachmentRepository(db)
	s3Client := storage.NewS3Client("test-key", "test-secret", "test-bucket", "us-east-1", "http://storage.test", "")
	svc := NewSupportAttachmentService(repo, s3Client)
	sessionID := "widget-session-1"

	response, err := svc.Create(context.Background(), model.CreateSupportAttachmentRequest{
		FileName: "receipt.pdf", FileSize: 512, ContentType: "application/pdf",
	}, "workspace-1", "", "customer", nil, &sessionID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if response.Attachment.ConversationID != nil {
		t.Fatalf("conversation_id = %v, want nil while upload is staged", response.Attachment.ConversationID)
	}
	if !strings.Contains(response.Attachment.StorageKey, "/support/sessions/widget-session-1/") {
		t.Fatalf("storage_key = %q, want session staging path", response.Attachment.StorageKey)
	}
	var conversationCount int64
	if err := db.Model(&model.SupportConversation{}).Count(&conversationCount).Error; err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if conversationCount != 0 {
		t.Fatalf("conversation count = %d, want 0", conversationCount)
	}
}

func TestSupportAttachmentConfirmUploadRequiresOwningWidgetSession(t *testing.T) {
	db := newTestDB(t)
	createSupportAttachmentTestTable(t, db)
	repo := repository.NewSupportAttachmentRepository(db)
	svc := NewSupportAttachmentService(repo, nil)
	ownerSessionID := "owner-session"
	attachment := &model.SupportAttachment{
		WorkspaceID: "workspace-1", FileName: "private.pdf", FileSize: 256, ContentType: "application/pdf",
		StorageKey: "private.pdf", UploadedByType: "customer", SessionID: &ownerSessionID,
	}
	if err := repo.Create(context.Background(), attachment); err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	otherSessionID := "other-session"
	if err := svc.ConfirmUpload(context.Background(), attachment.ID, "customer", nil, &otherSessionID); err == nil {
		t.Fatal("ConfirmUpload succeeded for another widget session")
	}
	stored, err := repo.GetByID(context.Background(), attachment.ID)
	if err != nil {
		t.Fatalf("get attachment: %v", err)
	}
	if stored.IsUploaded {
		t.Fatal("attachment was confirmed by another widget session")
	}
	if err := svc.ConfirmUpload(context.Background(), attachment.ID, "customer", nil, &ownerSessionID); err != nil {
		t.Fatalf("ConfirmUpload as owner: %v", err)
	}
}
