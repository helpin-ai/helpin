package service

import (
	"context"
	"io"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

type fakeAttachmentStore struct {
	getURL string
}

func (f *fakeAttachmentStore) HasPublicURL() bool { return true }

func (f *fakeAttachmentStore) PublicURL(key string) string {
	return "https://cdn.example.com/" + key
}

func (f *fakeAttachmentStore) GeneratePresignedPutURL(key, contentType string, size int64, publicRead bool) (string, error) {
	return "https://upload.example.com/" + key, nil
}

func (f *fakeAttachmentStore) PutObject(ctx context.Context, key, contentType string, size int64, body io.Reader, publicRead bool) error {
	return nil
}

func (f *fakeAttachmentStore) GeneratePresignedGetURL(key, filename string) (string, error) {
	f.getURL = "https://download.example.com/" + key + "?filename=" + filename
	return f.getURL, nil
}

func (f *fakeAttachmentStore) GeneratePresignedInlineGetURL(key string) (string, error) {
	f.getURL = "https://inline.example.com/" + key
	return f.getURL, nil
}

func (f *fakeAttachmentStore) GetObject(ctx context.Context, key string) ([]byte, error) {
	return []byte("fake-object-bytes"), nil
}

func (f *fakeAttachmentStore) DeleteObject(ctx context.Context, key string) error {
	return nil
}

func newAttachmentTestEnv(t *testing.T) (*PMAttachmentService, *repository.PMAttachmentRepository, *gorm.DB, string, string) {
	t.Helper()

	db := newTestDB(t)
	workspaceID := "ws-attachments"
	userID := "user-attachments"

	seedUser(t, db, userID, "attachments@test.com", "Attachment User", "hash")
	seedWorkspace(t, db, workspaceID, "Attachment Workspace", "attachment-ws", userID)
	seedWorkspaceMember(t, db, "member-attachments", workspaceID, userID, "attachments@test.com", "Attachment User", model.RoleAdmin)

	repo := repository.NewPMAttachmentRepository(db)
	svc := NewPMAttachmentService(repo, &storage.S3Client{}, nil)
	return svc, repo, db, workspaceID, userID
}

func seedEditorUploadAttachment(t *testing.T, db *gorm.DB, attachmentID, workspaceID, entityID, uploadedByID string) {
	t.Helper()
	now := time.Now().UTC()
	mustExec(
		t,
		db,
		`INSERT INTO pm_attachments (id, workspace_id, entity_type, entity_id, file_name, file_size, content_type, storage_key, is_uploaded, uploaded_by_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attachmentID,
		workspaceID,
		"editor_upload",
		entityID,
		"pasted-image.png",
		int64(128),
		"image/png",
		workspaceID+"/tmp-"+attachmentID,
		true,
		uploadedByID,
		now,
		now,
	)
}

func TestPMAttachmentService_PrepareAttachment_AllowsObjectiveAndSprintEntities(t *testing.T) {
	t.Parallel()

	svc, _, _, workspaceID, userID := newAttachmentTestEnv(t)
	ctx := context.Background()

	for _, entityType := range []string{"objective", "sprint"} {
		entityType := entityType
		t.Run(entityType, func(t *testing.T) {
			attachment, err := svc.prepareAttachment(ctx, model.CreateAttachmentRequest{
				EntityType:  entityType,
				EntityID:    entityType + "-1",
				FileName:    entityType + ".png",
				FileSize:    256,
				ContentType: "image/png",
			}, workspaceID, userID)
			if err != nil {
				t.Fatalf("prepareAttachment(%s): %v", entityType, err)
			}
			if attachment.EntityType != entityType {
				t.Fatalf("entity_type = %q, want %q", attachment.EntityType, entityType)
			}
		})
	}
}

func TestPMAttachmentService_PrepareAttachment_AllowsTaskTemplateEntity(t *testing.T) {
	t.Parallel()

	svc, _, _, workspaceID, userID := newAttachmentTestEnv(t)
	ctx := context.Background()

	attachment, err := svc.prepareAttachment(ctx, model.CreateAttachmentRequest{
		EntityType:  "task_template",
		EntityID:    "template-1",
		FileName:    "template.pdf",
		FileSize:    256,
		ContentType: "application/pdf",
	}, workspaceID, userID)
	if err != nil {
		t.Fatalf("prepareAttachment(task_template): %v", err)
	}
	if attachment.EntityType != "task_template" {
		t.Fatalf("entity_type = %q, want task_template", attachment.EntityType)
	}
}

func TestPMAttachmentService_PrepareAttachment_AllowsVideoUpTo50MB(t *testing.T) {
	t.Parallel()

	svc, _, _, workspaceID, userID := newAttachmentTestEnv(t)
	ctx := context.Background()

	attachment, err := svc.prepareAttachment(ctx, model.CreateAttachmentRequest{
		EntityType:  "task",
		EntityID:    "task-video",
		FileName:    "recording.mp4",
		FileSize:    maxFileSize,
		ContentType: "video/mp4",
	}, workspaceID, userID)
	if err != nil {
		t.Fatalf("prepareAttachment(video/mp4): %v", err)
	}
	if attachment.ContentType != "video/mp4" {
		t.Fatalf("content_type = %q, want video/mp4", attachment.ContentType)
	}

	_, err = svc.prepareAttachment(ctx, model.CreateAttachmentRequest{
		EntityType:  "task",
		EntityID:    "task-too-large",
		FileName:    "too-large.webm",
		FileSize:    maxFileSize + 1,
		ContentType: "video/webm",
	}, workspaceID, userID)
	if err == nil {
		t.Fatal("expected oversized video to be rejected")
	}
	if got := err.Error(); got != "file exceeds maximum size of 50MB" {
		t.Fatalf("oversized error = %q", got)
	}
}

func TestPMAttachmentService_ContentURLReturnsFreshDownloadURL(t *testing.T) {
	t.Parallel()

	_, repo, db, workspaceID, userID := newAttachmentTestEnv(t)
	store := &fakeAttachmentStore{}
	svc := NewPMAttachmentService(repo, store, nil)
	ctx := context.Background()

	seedEditorUploadAttachment(t, db, "attachment-content", workspaceID, workspaceID, userID)

	got, err := svc.ContentURL(ctx, "attachment-content")
	if err != nil {
		t.Fatalf("ContentURL: %v", err)
	}
	want := "https://inline.example.com/" + workspaceID + "/tmp-attachment-content"
	if got != want {
		t.Fatalf("ContentURL = %q, want %q", got, want)
	}
}

func TestPMAttachmentRepository_ReassignToEntity(t *testing.T) {
	t.Parallel()

	_, repo, db, workspaceID, userID := newAttachmentTestEnv(t)
	ctx := context.Background()

	seedEditorUploadAttachment(t, db, "attachment-1", workspaceID, workspaceID, userID)

	if err := repo.ReassignToEntity(ctx, []string{"attachment-1"}, "objective", "objective-1"); err != nil {
		t.Fatalf("ReassignToEntity: %v", err)
	}

	attachment, err := repo.GetByID(ctx, "attachment-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if attachment == nil {
		t.Fatal("expected attachment")
	}
	if attachment.EntityType != "objective" {
		t.Fatalf("entity_type = %q, want %q", attachment.EntityType, "objective")
	}
	if attachment.EntityID != "objective-1" {
		t.Fatalf("entity_id = %q, want %q", attachment.EntityID, "objective-1")
	}
}

func TestPMAttachmentRepository_ReassignToEntityErrorsWhenAnyIDIsMissing(t *testing.T) {
	t.Parallel()

	_, repo, db, workspaceID, userID := newAttachmentTestEnv(t)
	ctx := context.Background()

	seedEditorUploadAttachment(t, db, "attachment-present", workspaceID, workspaceID, userID)

	if err := repo.ReassignToEntity(ctx, []string{"attachment-present", "attachment-missing"}, "comment", "comment-1"); err == nil {
		t.Fatal("expected missing attachment to fail reassignment")
	}

	attachment, err := repo.GetByID(ctx, "attachment-present")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if attachment == nil {
		t.Fatal("expected attachment")
	}
	if attachment.EntityType != "editor_upload" {
		t.Fatalf("entity_type = %q, want editor_upload", attachment.EntityType)
	}
}

func TestPMAttachmentRepository_CloneUploadedFromEntityToEntity(t *testing.T) {
	t.Parallel()

	_, repo, db, workspaceID, userID := newAttachmentTestEnv(t)
	ctx := context.Background()
	now := time.Now().UTC()
	mustExec(
		t,
		db,
		`INSERT INTO pm_attachments (id, workspace_id, entity_type, entity_id, file_name, file_size, content_type, storage_key, is_uploaded, uploaded_by_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"template-attachment-1",
		workspaceID,
		"task_template",
		"template-1",
		"brief.pdf",
		int64(128),
		"application/pdf",
		workspaceID+"/shared-brief.pdf",
		true,
		userID,
		now,
		now,
	)

	cloned, err := repo.CloneUploadedFromEntityToEntity(ctx, "task_template", "template-1", "task", "task-1")
	if err != nil {
		t.Fatalf("CloneUploadedFromEntityToEntity: %v", err)
	}
	if len(cloned) != 1 {
		t.Fatalf("cloned len = %d, want 1", len(cloned))
	}
	if cloned[0].ID == "template-attachment-1" {
		t.Fatal("expected cloned attachment to have a new id")
	}
	if cloned[0].EntityType != "task" || cloned[0].EntityID != "task-1" {
		t.Fatalf("cloned entity = %s/%s, want task/task-1", cloned[0].EntityType, cloned[0].EntityID)
	}
	if cloned[0].StorageKey != workspaceID+"/shared-brief.pdf" {
		t.Fatalf("storage_key = %q, want shared key", cloned[0].StorageKey)
	}

	templateAttachments, err := repo.List(ctx, "task_template", "template-1")
	if err != nil {
		t.Fatalf("List template: %v", err)
	}
	if len(templateAttachments) != 1 {
		t.Fatalf("template attachments len = %d, want 1", len(templateAttachments))
	}
}

func TestPMAttachmentRepository_DeleteEditorUploadOnlyRemovesEditorUploads(t *testing.T) {
	t.Parallel()

	_, repo, db, workspaceID, userID := newAttachmentTestEnv(t)
	ctx := context.Background()

	seedEditorUploadAttachment(t, db, "attachment-editor-upload", workspaceID, workspaceID, userID)
	seedEditorUploadAttachment(t, db, "attachment-comment", workspaceID, workspaceID, userID)
	if err := repo.ReassignToEntity(ctx, []string{"attachment-comment"}, "comment", "comment-1"); err != nil {
		t.Fatalf("ReassignToEntity: %v", err)
	}

	deleted, err := repo.DeleteEditorUpload(ctx, "attachment-comment")
	if err != nil {
		t.Fatalf("DeleteEditorUpload reassigned attachment: %v", err)
	}
	if deleted {
		t.Fatal("expected reassigned attachment not to be deleted")
	}
	deleted, err = repo.DeleteEditorUpload(ctx, "attachment-editor-upload")
	if err != nil {
		t.Fatalf("DeleteEditorUpload editor upload: %v", err)
	}
	if !deleted {
		t.Fatal("expected editor upload attachment to be deleted")
	}

	reassigned, err := repo.GetByID(ctx, "attachment-comment")
	if err != nil {
		t.Fatalf("GetByID reassigned: %v", err)
	}
	if reassigned == nil {
		t.Fatal("expected reassigned attachment to be preserved")
	}
	if reassigned.EntityType != "comment" {
		t.Fatalf("entity_type = %q, want comment", reassigned.EntityType)
	}

	editorUpload, err := repo.GetByID(ctx, "attachment-editor-upload")
	if err != nil {
		t.Fatalf("GetByID editor upload: %v", err)
	}
	if editorUpload != nil {
		t.Fatal("expected editor upload attachment to be deleted")
	}
}

func TestPMAttachmentService_DeleteEditorUploadAllowsNonUploader(t *testing.T) {
	t.Parallel()

	_, repo, db, workspaceID, userID := newAttachmentTestEnv(t)
	svc := NewPMAttachmentService(repo, &fakeAttachmentStore{}, nil)
	ctx := context.Background()

	// Redaction depends on this: whoever redacts an image must be able to remove the original,
	// even when a colleague uploaded it.
	seedEditorUploadAttachment(t, db, "attachment-inline-image", workspaceID, workspaceID, userID)

	if err := svc.Delete(ctx, "attachment-inline-image", "another-editor"); err != nil {
		t.Fatalf("Delete editor upload as non-uploader: %v", err)
	}

	remaining, err := repo.GetByID(ctx, "attachment-inline-image")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if remaining != nil {
		t.Fatal("expected editor upload to be deleted")
	}
}

func TestPMAttachmentService_DeleteEntityAttachmentStillRequiresUploader(t *testing.T) {
	t.Parallel()

	_, repo, db, workspaceID, userID := newAttachmentTestEnv(t)
	svc := NewPMAttachmentService(repo, &fakeAttachmentStore{}, nil)
	ctx := context.Background()

	seedEditorUploadAttachment(t, db, "attachment-on-comment", workspaceID, workspaceID, userID)
	if err := repo.ReassignToEntity(ctx, []string{"attachment-on-comment"}, "comment", "comment-1"); err != nil {
		t.Fatalf("ReassignToEntity: %v", err)
	}

	if err := svc.Delete(ctx, "attachment-on-comment", "another-editor"); err == nil {
		t.Fatal("expected non-uploader deletion of an entity attachment to be rejected")
	}

	remaining, err := repo.GetByID(ctx, "attachment-on-comment")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if remaining == nil {
		t.Fatal("expected entity attachment to be preserved")
	}
}
