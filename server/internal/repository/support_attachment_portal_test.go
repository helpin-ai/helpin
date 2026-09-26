package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPortalAttachmentScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:portal_attachments?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE support_attachments (id TEXT PRIMARY KEY, workspace_id TEXT, session_id TEXT, conversation_id TEXT, message_id TEXT, uploaded_by_type TEXT, is_uploaded BOOLEAN, file_name TEXT, content_type TEXT, file_size INTEGER, storage_key TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	workspace, session, conversation := uuid.NewString(), uuid.NewString(), uuid.NewString()
	attachment := &model.SupportAttachment{ID: uuid.NewString(), WorkspaceID: workspace, SessionID: &session, ConversationID: &conversation, UploadedByType: "customer", IsUploaded: true, FileName: "a.txt", ContentType: "text/plain", FileSize: 1, StorageKey: "test"}
	if err := db.Exec(`INSERT INTO support_attachments (id, workspace_id, session_id, conversation_id, uploaded_by_type, is_uploaded) VALUES (?, ?, ?, ?, 'customer', 1)`, attachment.ID, workspace, session, conversation).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewSupportAttachmentRepository(db)
	ids := []string{attachment.ID}
	for _, scope := range []struct{ workspace, session, conversation string }{
		{uuid.NewString(), session, conversation},
		{workspace, uuid.NewString(), conversation},
		{workspace, session, uuid.NewString()},
		{workspace, session, ""},
	} {
		if err := repo.ValidatePortalAttachments(ctx, ids, scope.workspace, scope.session, scope.conversation); err == nil {
			t.Fatalf("accepted foreign scope: %+v", scope)
		}
	}
	if err := repo.ValidatePortalAttachments(ctx, ids, workspace, session, conversation); err != nil {
		t.Fatal(err)
	}
	if err := repo.LinkPortalAttachments(ctx, ids, workspace, session, uuid.NewString(), conversation, uuid.NewString()); err == nil {
		t.Fatal("linked across requests")
	}
	if err := repo.LinkPortalAttachments(ctx, ids, workspace, session, conversation, conversation, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := repo.ValidatePortalAttachments(ctx, ids, workspace, session, conversation); err == nil {
		t.Fatal("reused attachment")
	}
}
