package service

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

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
			t.Parallel()

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
