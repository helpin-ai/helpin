package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

func TestSupportNoteEditUpdatesInPlaceAndRejectsStaleEdits(t *testing.T) {
	ctx := context.Background()
	repo := noteEditTestRepository(t)
	svc := NewSupportMessageActionsService(repo, nil, nil, nil)
	msg := createActionMessage(t, repo, model.SupportMessage{IsInternal: true, Content: "Old note"})
	req := model.UpdateSupportNoteRequest{Content: "Updated **note**", ExpectedUpdatedAt: msg.UpdatedAt}
	updated, err := svc.UpdateNote(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != msg.ID || updated.Content != req.Content || !updated.CreatedAt.Equal(msg.CreatedAt) || updated.NoteEditedAt() == nil {
		t.Fatalf("note not updated in place: %+v", updated)
	}
	if !updated.IsInternal {
		t.Fatal("note became public")
	}
	if event := websocket.SupportMessageUpdatedEvent(msg.WorkspaceID, updated, *msg.SenderUserID); len(event.Data) != 0 || event.Action != "updated" {
		t.Fatalf("unsafe event: %+v", event)
	}
	if _, err := svc.UpdateNote(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, req); !errors.Is(err, repository.ErrSupportNoteConflict) {
		t.Fatalf("stale edit: %v", err)
	}
	req.ExpectedUpdatedAt = updated.UpdatedAt
	unchanged, err := svc.UpdateNote(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, req)
	if err != nil || !unchanged.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Fatalf("unchanged save should be a no-op: %v", err)
	}
	info, err := svc.Info(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID)
	if err != nil || !info.Edited || info.EditedAt == nil {
		t.Fatalf("edit info missing: %+v %v", info, err)
	}
}

func TestSupportNoteEditGuards(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		mutate                         func(*model.SupportMessage)
		actor, workspace, conversation string
		content                        string
		want                           error
	}{
		{name: "other author", actor: "someone-else", content: "new", want: ErrSupportMessageActionForbidden},
		{name: "other workspace", workspace: "other-ws", content: "new", want: ErrSupportMessageActionNotFound},
		{name: "other conversation", conversation: "other-conv", content: "new", want: ErrSupportMessageActionNotFound},
		{name: "public reply", mutate: func(m *model.SupportMessage) { m.IsInternal = false }, content: "new", want: ErrSupportMessageActionForbidden},
		{name: "AI note", mutate: func(m *model.SupportMessage) { m.SenderType = "ai" }, content: "new", want: ErrSupportMessageActionForbidden},
		{name: "system note", mutate: func(m *model.SupportMessage) {
			m.MessageType = "system"
			event := "assigned"
			m.SystemEventType = &event
		}, content: "new", want: ErrSupportMessageActionForbidden},
		{name: "blank", content: " \n ", want: repository.ErrSupportNoteInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := noteEditTestRepository(t)
			svc := NewSupportMessageActionsService(repo, nil, nil, nil)
			msg := model.SupportMessage{IsInternal: true}
			if tc.mutate != nil {
				tc.mutate(&msg)
			}
			msg = createActionMessage(t, repo, msg)
			actor, ws, conv := *msg.SenderUserID, msg.WorkspaceID, msg.ConversationID
			if tc.actor != "" {
				actor = tc.actor
			}
			if tc.workspace != "" {
				ws = tc.workspace
			}
			if tc.conversation != "" {
				conv = tc.conversation
			}
			_, err := svc.UpdateNote(context.Background(), ws, conv, actor, msg.ID, model.UpdateSupportNoteRequest{Content: tc.content, ExpectedUpdatedAt: msg.UpdatedAt})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSupportNoteEditAttachmentsAreScopedAndAtomic(t *testing.T) {
	ctx := context.Background()
	repo := noteEditTestRepository(t)
	svc := NewSupportMessageActionsService(repo, nil, nil, nil)
	msg := createActionMessage(t, repo, model.SupportMessage{IsInternal: true})
	attachment := model.SupportAttachment{ID: "att-own", WorkspaceID: msg.WorkspaceID, ConversationID: &msg.ConversationID, MessageID: &msg.ID, IsUploaded: true, UploadedByType: "user", UploadedByID: msg.SenderUserID, FileName: "original.txt", ContentType: "text/plain"}
	if err := repo.DB().Create(&attachment).Error; err != nil {
		t.Fatal(err)
	}
	req := model.UpdateSupportNoteRequest{Content: "", ExpectedUpdatedAt: msg.UpdatedAt}
	updated, err := svc.UpdateNote(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, req)
	if err != nil {
		t.Fatalf("attachment-only note: %v", err)
	}
	ids := []string{"missing-attachment"}
	req.AttachmentIDs = &ids
	req.Content = "must not save"
	req.ExpectedUpdatedAt = updated.UpdatedAt
	if _, err = svc.UpdateNote(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, req); !errors.Is(err, repository.ErrSupportNoteInvalid) {
		t.Fatalf("invalid attachment: %v", err)
	}
	saved, _ := repo.GetByID(ctx, msg.ID)
	if saved.Content != "" {
		t.Fatal("failed attachment change modified note")
	}
	fresh := attachment
	fresh.ID = "att-new"
	fresh.MessageID = nil
	if err := repo.DB().Create(&fresh).Error; err != nil {
		t.Fatal(err)
	}
	ids = []string{fresh.ID}
	req.Content = "updated"
	updated, err = svc.UpdateNote(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	var old model.SupportAttachment
	repo.DB().First(&old, "id = ?", attachment.ID)
	if old.MessageID != nil {
		t.Fatal("removed attachment still linked")
	}
	var linked model.SupportAttachment
	repo.DB().First(&linked, "id = ?", fresh.ID)
	if linked.MessageID == nil || *linked.MessageID != msg.ID {
		t.Fatal("new attachment not linked")
	}
	// Cannot claim another actor's unlinked upload, even inside this conversation.
	other := fresh
	other.ID = "att-other"
	other.MessageID = nil
	otherID := "other-author"
	other.UploadedByID = &otherID
	if err := repo.DB().Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	ids = []string{other.ID}
	req.ExpectedUpdatedAt = updated.UpdatedAt
	if _, err = svc.UpdateNote(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, req); !errors.Is(err, repository.ErrSupportNoteInvalid) {
		t.Fatalf("foreign upload allowed: %v", err)
	}
}

func TestSupportNoteEditRejectsReadOnlyConversations(t *testing.T) {
	for _, column := range []string{"merged_into_id", "anonymized_at"} {
		t.Run(column, func(t *testing.T) {
			repo := noteEditTestRepository(t)
			svc := NewSupportMessageActionsService(repo, nil, nil, nil)
			msg := createActionMessage(t, repo, model.SupportMessage{IsInternal: true})
			var value any = "other-conversation"
			if column == "anonymized_at" {
				value = time.Now()
			}
			if err := repo.DB().Model(&model.SupportConversation{}).Where("id = ?", msg.ConversationID).Update(column, value).Error; err != nil {
				t.Fatal(err)
			}
			_, err := svc.UpdateNote(context.Background(), msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, model.UpdateSupportNoteRequest{Content: "new", ExpectedUpdatedAt: msg.UpdatedAt})
			if err == nil {
				t.Fatal("edited read-only history")
			}
		})
	}
}

func noteEditTestRepository(t *testing.T) *repository.SupportMessageRepository {
	t.Helper()
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE support_attachments (id TEXT PRIMARY KEY,workspace_id TEXT,conversation_id TEXT,message_id TEXT,file_name TEXT,file_size INTEGER,content_type TEXT,storage_key TEXT,public_url TEXT,content_id TEXT,processing_status TEXT,processing_error TEXT,is_uploaded BOOLEAN,uploaded_by_type TEXT,uploaded_by_id TEXT,session_id TEXT,created_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	return repository.NewSupportMessageRepository(db)
}

func TestSupportNoteEditCannotReviveDeletedNotes(t *testing.T) {
	repo := noteEditTestRepository(t)
	svc := NewSupportMessageActionsService(repo, nil, nil, nil)
	msg := createActionMessage(t, repo, model.SupportMessage{IsInternal: true})
	if err := repo.DB().Delete(&msg).Error; err != nil {
		t.Fatal(err)
	}
	_, err := svc.UpdateNote(context.Background(), msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, model.UpdateSupportNoteRequest{Content: "revive", ExpectedUpdatedAt: msg.UpdatedAt})
	if !errors.Is(err, ErrSupportMessageActionNotFound) {
		t.Fatalf("deleted note: %v", err)
	}
}

func TestSupportNoteEditRejectsForeignAndUnfinishedAttachments(t *testing.T) {
	for _, kind := range []string{"workspace", "conversation", "message", "unfinished", "customer"} {
		t.Run(kind, func(t *testing.T) {
			repo := noteEditTestRepository(t)
			svc := NewSupportMessageActionsService(repo, nil, nil, nil)
			msg := createActionMessage(t, repo, model.SupportMessage{IsInternal: true})
			att := model.SupportAttachment{ID: "attempted", WorkspaceID: msg.WorkspaceID, ConversationID: &msg.ConversationID, IsUploaded: true, UploadedByType: "user", UploadedByID: msg.SenderUserID, FileName: "file.txt", ContentType: "text/plain"}
			other := "other"
			switch kind {
			case "workspace":
				att.WorkspaceID = other
			case "conversation":
				att.ConversationID = &other
			case "message":
				att.MessageID = &other
			case "unfinished":
				att.IsUploaded = false
			case "customer":
				att.UploadedByType = "customer"
			}
			if err := repo.DB().Create(&att).Error; err != nil {
				t.Fatal(err)
			}
			ids := []string{att.ID}
			_, err := svc.UpdateNote(context.Background(), msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, model.UpdateSupportNoteRequest{Content: "updated", AttachmentIDs: &ids, ExpectedUpdatedAt: msg.UpdatedAt})
			if !errors.Is(err, repository.ErrSupportNoteInvalid) {
				t.Fatalf("%s attachment accepted: %v", kind, err)
			}
		})
	}
}
