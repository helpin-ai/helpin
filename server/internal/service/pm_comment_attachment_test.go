package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPMCommentService_CreateFailsAndRollsBackWhenAttachmentReassignFails(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	workspaceID := "ws-comment-attachments"
	userID := "user-comment-attachments"
	ctx := context.Background()

	seedUser(t, db, userID, "comment-attachments@test.com", "Comment Attachments", "hash")
	seedWorkspace(t, db, workspaceID, "Comment Attachment Workspace", "comment-attachment-ws", userID)
	seedWorkspaceMember(t, db, "member-comment-attachments", workspaceID, userID, "comment-attachments@test.com", "Comment Attachments", model.RoleAdmin)
	seedEditorUploadAttachment(t, db, "attachment-present", workspaceID, workspaceID, userID)

	commentService := NewPMCommentService(
		repository.NewPMCommentRepository(db),
		repository.NewPMTaskRepository(db),
		repository.NewPMAttachmentRepository(db),
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		repository.NewWorkspaceRepository(db),
		nil,
	)

	_, err := commentService.Create(ctx, model.CreateCommentRequest{
		EntityType:    "doc",
		EntityID:      "doc-1",
		Body:          "<p>comment</p>",
		AttachmentIDs: []string{"attachment-present", "attachment-missing"},
	}, userID, workspaceID)
	if err == nil {
		t.Fatal("expected create to fail when any attachment cannot be reassigned")
	}

	var commentCount int64
	if err := db.Model(&model.PMComment{}).Where("entity_type = ? AND entity_id = ?", "doc", "doc-1").Count(&commentCount).Error; err != nil {
		t.Fatalf("count comments: %v", err)
	}
	if commentCount != 0 {
		t.Fatalf("comments count = %d, want 0", commentCount)
	}

	attachment, err := repository.NewPMAttachmentRepository(db).GetByID(ctx, "attachment-present")
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

func TestPMCommentService_UpdateFailsWhenAttachmentReassignFails(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	workspaceID := "ws-comment-update-attachments"
	userID := "user-comment-update-attachments"
	ctx := context.Background()
	now := time.Now().UTC()

	seedUser(t, db, userID, "comment-update-attachments@test.com", "Comment Update Attachments", "hash")
	seedWorkspace(t, db, workspaceID, "Comment Update Attachment Workspace", "comment-update-attachment-ws", userID)
	seedWorkspaceMember(t, db, "member-comment-update-attachments", workspaceID, userID, "comment-update-attachments@test.com", "Comment Update Attachments", model.RoleAdmin)
	mustExec(t, db, `INSERT INTO pm_comments (id, workspace_id, entity_type, entity_id, author_id, body, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"comment-update-1", workspaceID, "doc", "doc-1", userID, "<p>before</p>", now, now)
	seedEditorUploadAttachment(t, db, "attachment-present", workspaceID, workspaceID, userID)

	commentService := NewPMCommentService(
		repository.NewPMCommentRepository(db),
		repository.NewPMTaskRepository(db),
		repository.NewPMAttachmentRepository(db),
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		repository.NewWorkspaceRepository(db),
		nil,
	)

	_, err := commentService.Update(ctx, "comment-update-1", model.UpdateCommentRequest{
		Body:          "<p>after</p>",
		AttachmentIDs: []string{"attachment-present", "attachment-missing"},
	}, userID, false, workspaceID)
	if err == nil {
		t.Fatal("expected update to fail when any attachment cannot be reassigned")
	}

	var body string
	if err := db.Raw(`SELECT body FROM pm_comments WHERE id = ?`, "comment-update-1").Scan(&body).Error; err != nil {
		t.Fatalf("load comment body: %v", err)
	}
	if body != "<p>before</p>" {
		t.Fatalf("body = %q, want original body", body)
	}

	attachment, err := repository.NewPMAttachmentRepository(db).GetByID(ctx, "attachment-present")
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
