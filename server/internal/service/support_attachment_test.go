package service

import (
	"context"
	"net/url"
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
		content_type TEXT NOT NULL, content_id TEXT NOT NULL DEFAULT '', processing_status TEXT NOT NULL DEFAULT '', processing_error TEXT NOT NULL DEFAULT '',
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
	upload, err := url.Parse(response.UploadURL)
	if err != nil {
		t.Fatal(err)
	}
	download, err := url.Parse(response.PublicURL)
	if err != nil {
		t.Fatal(err)
	}
	if upload.Query().Get("x-amz-acl") != "" {
		t.Fatal("customer upload granted public access")
	}
	if download.Query().Get("X-Amz-Signature") == "" {
		t.Fatal("download URL must be signed")
	}
	if response.Attachment.PublicURL != "" {
		t.Fatal("must not persist expiring or public attachment URLs")
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

func TestSupportAttachmentCreateSizeAndTypeValidation(t *testing.T) {
	for _, tt := range []struct {
		name            string
		size            int64
		mime, wantError string
	}{
		{"exactly100MiB video", 100 * 1024 * 1024, "video/mp4", ""},
		{"over100MiB", 100*1024*1024 + 1, "video/mp4", "file exceeds maximum size of 100MB"},
		{"empty", 0, "video/mp4", "file_size must be positive"},
		{"negative", -1, "video/mp4", "file_size must be positive"},
		{"missing content type", 512, "", "content_type is required"},
		{"unsupported type", 512, "application/x-executable", "file type application/x-executable is not allowed"},
		{"quicktime video", 512, "video/quicktime", ""},
		{"webm video", 512, "video/webm", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			createSupportAttachmentTestTable(t, db)
			svc := NewSupportAttachmentService(repository.NewSupportAttachmentRepository(db), storage.NewS3Client("key", "secret", "bucket", "us-east-1", "http://storage.test", ""))
			sessionID := "owner-session"
			response, err := svc.Create(context.Background(), model.CreateSupportAttachmentRequest{FileName: "recording", FileSize: tt.size, ContentType: tt.mime}, "workspace", "", "customer", nil, &sessionID)
			if tt.wantError != "" {
				if err == nil || err.Error() != tt.wantError {
					t.Fatalf("error = %v, want %q", err, tt.wantError)
				}
				var count int64
				if err := db.Model(&model.SupportAttachment{}).Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatalf("invalid upload created %d records", count)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if response.UploadURL == "" || response.Attachment.FileSize != tt.size || response.Attachment.ContentType != tt.mime {
				t.Fatalf("unexpected response: %+v", response)
			}
		})
	}
}

func TestDeleteUnsentWidgetOwnershipAndSentProtection(t *testing.T) {
	for _, tt := range []struct {
		name, session   string
		sent, wantError bool
	}{
		{"owner can clean up", "owner", false, false},
		{"other session denied", "other", false, true},
		{"sent file protected", "owner", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			createSupportAttachmentTestTable(t, db)
			repo := repository.NewSupportAttachmentRepository(db)
			svc := NewSupportAttachmentService(repo, nil)
			owner := "owner"
			attachment := &model.SupportAttachment{WorkspaceID: "workspace", FileName: "test.png", FileSize: 1, ContentType: "image/png", UploadedByType: "customer", SessionID: &owner}
			if tt.sent {
				message := "message"
				attachment.MessageID = &message
			}
			if err := repo.Create(context.Background(), attachment); err != nil {
				t.Fatal(err)
			}
			err := svc.DeleteUnsentWidget(context.Background(), attachment.ID, tt.session)
			if (err != nil) != tt.wantError {
				t.Fatalf("error=%v wantError=%v", err, tt.wantError)
			}
			remaining, err := repo.GetByID(context.Background(), attachment.ID)
			if err != nil {
				t.Fatal(err)
			}
			if (remaining != nil) != tt.wantError {
				t.Fatal("attachment deletion violated ownership or sent protection")
			}
		})
	}
}
