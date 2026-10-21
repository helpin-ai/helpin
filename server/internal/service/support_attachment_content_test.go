package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

func TestSupportAttachmentContent(t *testing.T) {
	for _, scenario := range []string{"allowed", "inbox member", "other workspace", "other conversation", "no actor", "private inbox", "unlinked", "not uploaded", "deleted message"} {
		t.Run(scenario, func(t *testing.T) {
			f := newSupportTriageTestFixture(t, nil, nil)
			createSupportAttachmentTestTable(t, f.db)
			mailbox := f.createMailbox(t, "Private", "private", false)
			conversation := f.createConversation(t, "Image", "customer@example.com", &mailbox.ID)
			message := f.createCustomerReply(t, conversation.ID, "Image attached")
			attachment := &model.SupportAttachment{
				ID: "image", WorkspaceID: f.workspaceID, ConversationID: &conversation.ID,
				MessageID: &message.ID, FileName: "image.png", FileSize: 5, ContentType: "image/png",
				StorageKey: "private/image.png", IsUploaded: true, UploadedByType: "user",
			}
			actor := &authorization.Actor{WorkspaceID: f.workspaceID, WorkspaceMemberID: "wm-support-owner", UserID: f.actorID, Role: model.RoleAdmin}
			workspaceID, conversationID := f.workspaceID, conversation.ID
			switch scenario {
			case "other workspace":
				attachment.WorkspaceID = "another-workspace"
			case "other conversation":
				other := f.createConversation(t, "Other", "other@example.com", nil)
				conversationID = other.ID
			case "private inbox":
				actor.Role = model.RoleMember
			case "inbox member":
				actor.Role = model.RoleMember
				if err := f.mailboxRepo.AddMembers(f.ctx, mailbox.ID, []string{actor.WorkspaceMemberID}); err != nil {
					t.Fatal(err)
				}
			case "unlinked":
				attachment.MessageID = nil
			case "not uploaded":
				attachment.IsUploaded = false
			case "deleted message":
				if err := f.db.Delete(message).Error; err != nil {
					t.Fatal(err)
				}
			}
			ctx := authorization.WithActor(context.Background(), actor)
			if scenario == "no actor" {
				ctx = context.Background()
			}
			storageReads := 0
			objectStore := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				storageReads++
				if r.URL.Path != "/bucket/private/image.png" {
					t.Errorf("unexpected object path %q", r.URL.Path)
				}
				w.Header().Set("Content-Length", "5")
				if _, err := io.WriteString(w, "image"); err != nil {
					t.Error(err)
				}
			}))
			defer objectStore.Close()
			repo := repository.NewSupportAttachmentRepository(f.db)
			if err := repo.Create(f.ctx, attachment); err != nil {
				t.Fatal(err)
			}
			f.supportSvc.attachmentService = NewSupportAttachmentService(repo, storage.NewS3Client("key", "secret", "bucket", "us-east-1", objectStore.URL, ""))
			content, err := f.supportSvc.OpenAttachmentContent(ctx, workspaceID, conversationID, attachment.ID)
			if scenario != "allowed" && scenario != "inbox member" {
				if !errors.Is(err, ErrSupportAttachmentNotFound) {
					t.Fatalf("expected inaccessible attachment, got %v", err)
				}
				if storageReads != 0 {
					t.Fatal("unauthorized request reached storage")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer content.Body.Close()
			body, err := io.ReadAll(content.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "image" || content.Size != 5 || content.ContentType != "image/png" || content.FileName != "image.png" {
				t.Fatalf("unexpected content: %#v, %q", content, body)
			}
		})
	}
}
