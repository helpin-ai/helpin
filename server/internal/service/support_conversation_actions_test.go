package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type supportActionsFixture struct {
	db          *gorm.DB
	inbox       *SupportInboxService
	actions     *SupportConversationActionService
	workspaceID string
	userID      string
	ctx         context.Context
}

func newSupportActionsFixture(t *testing.T) *supportActionsFixture {
	t.Helper()
	db := newTestDB(t)
	f := &supportActionsFixture{db: db, workspaceID: uuid.NewString(), userID: uuid.NewString()}
	seedUser(t, db, f.userID, "agent@example.com", "Ada Agent", "hash")
	seedWorkspace(t, db, f.workspaceID, "Actions", "actions-"+f.workspaceID[:8], f.userID)
	memberID := uuid.NewString()
	seedWorkspaceMember(t, db, memberID, f.workspaceID, f.userID, "agent@example.com", "Ada Agent", model.RoleOwner)
	conversations := repository.NewSupportConversationRepository(db)
	messages := repository.NewSupportMessageRepository(db)
	f.inbox = NewSupportInboxService(conversations, repository.NewSupportMailboxRepository(db), messages, nil, nil,
		repository.NewSupportInboxInstallationRepository(db), nil, repository.NewSupportCannedResponseRepository(db), nil, nil, nil,
		repository.NewUserRepository(db), nil, nil, nil)
	f.inbox.workspaceRepo = repository.NewWorkspaceRepository(db)
	f.inbox.tagRepo = repository.NewSupportTagRepository(db)
	tags := NewSupportTagService(repository.NewSupportTagRepository(db), conversations, nil).SetMessageRepo(messages)
	f.actions = NewSupportConversationActionService(f.inbox, tags)
	f.ctx = authorization.WithActor(context.Background(), &authorization.Actor{UserID: f.userID, WorkspaceID: f.workspaceID, WorkspaceMemberID: memberID, Role: model.RoleOwner})
	return f
}

func (f *supportActionsFixture) conversation(t *testing.T, display int) string {
	t.Helper()
	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: f.workspaceID, DisplayID: display, Subject: "Help", Status: model.SupportConversationStatusOpen, Priority: "medium", Channel: "email"}
	if err := f.db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	return conversation.ID
}

func (f *supportActionsFixture) tag(t *testing.T, name string) string {
	t.Helper()
	id := uuid.NewString()
	mustExec(t, f.db, `INSERT INTO support_tags (id, workspace_id, name) VALUES (?, ?, ?)`, id, f.workspaceID, name)
	return id
}

func strRef(value string) *string { return &value }

func TestNormalizeSupportConversationActions(t *testing.T) {
	tag := uuid.NewString()
	for name, actions := range map[string]model.SupportConversationActions{
		"bad status":         {Status: strRef("archived")},
		"bad priority":       {Priority: strRef("critical")},
		"bad assignee":       {AssignUserID: strRef("someone")},
		"bad inbox":          {MailboxID: strRef("billing")},
		"bad tag":            {AddTagIDs: []string{"vip"}},
		"add and remove":     {AddTagIDs: []string{tag}, RemoveTagIDs: []string{tag}},
		"resolve and snooze": {Status: strRef("resolved"), SnoozeMinutes: intPtr(60)},
		"snooze too long":    {SnoozeMinutes: intPtr(supportMaxSnoozeMinutes + 1)},
	} {
		if _, err := NormalizeSupportConversationActions(actions); !errors.Is(err, ErrSupportActionsInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	normalized, err := NormalizeSupportConversationActions(model.SupportConversationActions{
		Status: strRef("closed"), Priority: strRef(" HIGH "), MailboxID: strRef(""), AssignUserID: strRef(" "),
		AddTagIDs: []string{tag, tag, " "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if *normalized.Status != "resolved" || *normalized.Priority != "high" || *normalized.MailboxID != "shared" ||
		*normalized.AssignUserID != "" || len(normalized.AddTagIDs) != 1 || normalized.RemoveTagIDs != nil {
		t.Fatalf("normalized: %+v", normalized)
	}
}

func TestApplyMacroRunsActionsInOrder(t *testing.T) {
	f := newSupportActionsFixture(t)
	conversationID := f.conversation(t, 1)
	billing := f.tag(t, "Billing")
	macro, err := f.inbox.CreateCannedResponse(f.ctx, f.workspaceID, model.CannedResponseRequest{
		ShortCode: "!refund-done",
		Actions: &model.SupportConversationActions{
			Priority: strRef("low"), AssignUserID: strRef(f.userID), AddTagIDs: []string{billing}, Status: strRef("resolved"),
		},
	}, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if macro.Content != "" || macro.Actions.Empty() {
		t.Fatalf("action-only macro: %+v", macro)
	}
	response, err := f.actions.ApplyMacro(f.ctx, f.workspaceID, conversationID, macro.ID, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if response.Result.Error != "" || strings.Join(response.Result.Applied, ",") != "assignee,priority,tags,status" {
		t.Fatalf("result: %+v", response.Result)
	}
	c := response.Conversation
	if c.Status != "resolved" || c.Priority != "low" || c.AssignedUserID == nil || *c.AssignedUserID != f.userID || len(c.Tags) != 1 {
		t.Fatalf("conversation after macro: status=%s priority=%s assignee=%v tags=%d", c.Status, c.Priority, c.AssignedUserID, len(c.Tags))
	}

	plain, err := f.inbox.CreateCannedResponse(f.ctx, f.workspaceID, model.CannedResponseRequest{ShortCode: "!hi", Content: "Hello"}, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.actions.ApplyMacro(f.ctx, f.workspaceID, conversationID, plain.ID, f.userID); !errors.Is(err, ErrSupportMacroWithoutActions) {
		t.Fatalf("text-only macro: %v", err)
	}
	if _, err := f.inbox.CreateCannedResponse(f.ctx, f.workspaceID, model.CannedResponseRequest{ShortCode: "!empty"}, f.userID); !errors.Is(err, ErrCannedResponseInvalid) {
		t.Fatalf("empty macro accepted: %v", err)
	}
	if _, err := f.inbox.CreateCannedResponse(f.ctx, f.workspaceID, model.CannedResponseRequest{ShortCode: "!bad", Content: "x", Actions: &model.SupportConversationActions{Priority: strRef("critical")}}, f.userID); !errors.Is(err, ErrCannedResponseInvalid) {
		t.Fatalf("invalid macro accepted: %v", err)
	}
	// Updating text without actions keeps the existing actions.
	updated, err := f.inbox.UpdateCannedResponse(f.ctx, f.workspaceID, macro.ID, model.CannedResponseRequest{ShortCode: "!refund-done", Content: "Refund sent."})
	if err != nil || updated.Actions.Empty() || updated.Content != "Refund sent." {
		t.Fatalf("update: %+v %v", updated, err)
	}
}

func TestApplyBulkReportsEachConversation(t *testing.T) {
	f := newSupportActionsFixture(t)
	first, second := f.conversation(t, 1), f.conversation(t, 2)
	merged := f.conversation(t, 3)
	mustExec(t, f.db, `UPDATE support_conversations SET merged_into_id = ? WHERE id = ?`, first, merged)
	missing := uuid.NewString()

	response, err := f.actions.ApplyBulk(f.ctx, f.workspaceID, model.SupportBulkActionRequest{
		ConversationIDs: []string{first, second, merged, missing, first},
		Actions:         model.SupportConversationActions{Priority: strRef("urgent")},
	}, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if response.Succeeded != 2 || response.Failed != 2 || len(response.Results) != 4 {
		t.Fatalf("bulk: %+v", response)
	}
	if response.Results[2].Error != "merged conversations are read-only" || !strings.Contains(response.Results[3].Error, "not found") {
		t.Fatalf("errors: %+v", response.Results)
	}
	var priority string
	f.db.Raw(`SELECT priority FROM support_conversations WHERE id = ?`, second).Scan(&priority)
	if priority != "urgent" {
		t.Fatalf("priority = %s", priority)
	}

	if _, err := f.actions.ApplyBulk(f.ctx, f.workspaceID, model.SupportBulkActionRequest{ConversationIDs: []string{first}}, f.userID); !errors.Is(err, ErrSupportActionsInvalid) {
		t.Fatalf("empty actions: %v", err)
	}
	tooMany := make([]string, model.SupportBulkActionLimit+1)
	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}
	if _, err := f.actions.ApplyBulk(f.ctx, f.workspaceID, model.SupportBulkActionRequest{ConversationIDs: tooMany, Actions: model.SupportConversationActions{Priority: strRef("low")}}, f.userID); !errors.Is(err, ErrSupportActionsInvalid) {
		t.Fatalf("limit: %v", err)
	}
}
