package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"testing"
)

func TestMailboxAIRepliesDisabledPersists(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	box := f.createMailbox(t, "Admin", "admin", false)
	var req model.UpdateSupportMailboxRequest
	if err := json.Unmarshal([]byte(`{"ai_replies_disabled":true}`), &req); err != nil {
		t.Fatal(err)
	}
	updated, err := f.supportSvc.UpdateMailbox(f.ctx, f.workspaceID, box.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(updated)
	var fields map[string]any
	json.Unmarshal(data, &fields)
	if fields["ai_replies_disabled"] != true {
		t.Fatalf("AI reply preference not persisted: %s", data)
	}
}

func TestMailboxAIPolicyBlocksRepliesAndFollowUps(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	box := f.createMailbox(t, "Admin", "admin", false)
	box.AIRepliesDisabled = true
	if err := f.mailboxRepo.Update(f.ctx, box); err != nil {
		t.Fatal(err)
	}
	conv := &model.SupportConversation{WorkspaceID: f.workspaceID, MailboxID: &box.ID}
	if err := repository.LoadSupportMailboxAIPolicy(f.db, conv); err != nil {
		t.Fatal(err)
	}
	settings := model.DefaultSupportInboxSettings()
	settings.AIReplyChannels = "both"
	settings.AIEnabled = true
	settings.AIResponseMode = "ai_first"
	settings.AIFollowUpEnabled = true
	settings.AIAgentID = strPtr("agent")
	conv.Status = "open"
	conv.FlowState = strPtr("ai_handling")
	conv.AIState = strPtr("pending")
	conv.AssignedAgentID = settings.AIAgentID
	conv.LastPublicMessageID = strPtr("source")
	conv.LastPublicSenderType = strPtr("ai")
	episode := &model.SupportAIFollowUp{SourceMessageID: "source"}
	if model.SupportAIReplyAllowed(settings, conv, nil) {
		t.Fatal("disabled inbox allowed reply")
	}
	if !model.SupportAIConversationBlocked(conv) {
		t.Fatal("disabled inbox allowed continuation")
	}
	if supportFollowUpEligible(conv, episode, settings) {
		t.Fatal("disabled inbox allowed follow-up")
	}
	box.AIRepliesDisabled = false
	if err := f.mailboxRepo.Update(f.ctx, box); err != nil {
		t.Fatal(err)
	}
	if err := repository.LoadSupportMailboxAIPolicy(f.db, conv); err != nil {
		t.Fatal(err)
	}
	if !model.SupportAIReplyAllowed(settings, conv, nil) {
		t.Fatal("enabled inbox blocked reply")
	}
	if !supportFollowUpEligible(conv, episode, settings) {
		t.Fatal("enabled inbox blocked eligible follow-up")
	}
}

func TestSingleInboxSenderChangePreservesOtherAssignments(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	a := f.createMailbox(t, "A", "a", false)
	b := f.createMailbox(t, "B", "b", false)
	repo := repository.NewSupportEmailSenderRepository(f.db)
	old := verifiedSender(f.workspaceID, &a.ID, "old@example.com", "mailbox", f.actorID)
	next := verifiedSender(f.workspaceID, nil, "new@example.com", "workspace", f.actorID)
	for _, s := range []*model.SupportEmailSender{old, next} {
		if err := repo.Create(f.ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.SetDefault(f.ctx, f.workspaceID, old.ID, "mailbox", []string{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetMailboxSender(f.ctx, f.workspaceID, a.ID, next.ID); err != nil {
		t.Fatal(err)
	}
	remaining, err := repo.GetMailboxDefaultVerified(f.ctx, f.workspaceID, &b.ID)
	if err != nil || remaining == nil || remaining.ID != old.ID {
		t.Fatalf("other inbox assignment lost: %v %v", remaining, err)
	}
	chosen, err := repo.GetMailboxDefaultVerified(f.ctx, f.workspaceID, &a.ID)
	if err != nil || chosen != nil {
		t.Fatalf("workspace sender should use fallback: %v %v", chosen, err)
	}
	workspace, err := repo.GetWorkspaceDefaultVerified(f.ctx, f.workspaceID)
	if err != nil || workspace == nil || workspace.ID != next.ID {
		t.Fatal("workspace default changed")
	}

	c := f.createMailbox(t, "C", "c", false)
	third := verifiedSender(f.workspaceID, &c.ID, "third@example.com", "mailbox", f.actorID)
	if err := repo.Create(f.ctx, third); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetMailboxSender(f.ctx, f.workspaceID, a.ID, third.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a.ID, c.ID} {
		assigned, err := repo.GetMailboxDefaultVerified(f.ctx, f.workspaceID, &id)
		if err != nil || assigned == nil || assigned.ID != third.ID {
			t.Fatalf("selected sender lost inbox %s: %v %v", id, assigned, err)
		}
	}
	remaining, err = repo.GetMailboxDefaultVerified(f.ctx, f.workspaceID, &b.ID)
	if err != nil || remaining == nil || remaining.ID != old.ID {
		t.Fatal("unrelated assignment changed")
	}
}

func TestMailboxDisabledRejectsAlreadyRunningReply(t *testing.T) {
	svc, db, conv, _ := setupAIControlTest(t)
	ctx := context.Background()
	box := &model.SupportMailbox{ID: "admin", WorkspaceID: conv.WorkspaceID, Name: "Admin", Handle: "admin", CreatedByID: "owner", AIRepliesDisabled: true}
	if err := repository.NewSupportMailboxRepository(db).Create(ctx, box); err != nil {
		t.Fatal(err)
	}
	if err := svc.conversationRepo.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{"mailbox_id": box.ID}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `CREATE TABLE ai_message_processing(id TEXT PRIMARY KEY,workspace_id TEXT,conversation_id TEXT,source_message_id TEXT,reply_message_id TEXT,status TEXT,tokens_used INTEGER,created_at DATETIME,updated_at DATETIME)`)
	mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,source_message_id,status) VALUES ('turn','ws','conv','customer','processing')`)
	reply := &model.SupportMessage{ID: "late", WorkspaceID: "ws", ConversationID: "conv", SenderType: "ai", MessageType: "reply", Content: "Should never publish"}
	if created, err := repository.NewAIMessageProcessingRepository(db).CreateReply(ctx, "turn", reply, "old-run"); err != nil || created {
		t.Fatalf("disabled inbox published late reply: %v %v", created, err)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("id = ?", "late").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("late reply persisted: %d %v", count, err)
	}
}

func TestMailboxDisabledCancelsPendingFollowUp(t *testing.T) {
	svc, db, run, episode, decision := setupFollowUpTest(t)
	box := &model.SupportMailbox{ID: "admin", WorkspaceID: "ws", Name: "Admin", Handle: "admin", CreatedByID: "owner", AIRepliesDisabled: true}
	if err := repository.NewSupportMailboxRepository(db).Create(context.Background(), box); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE support_conversations SET mailbox_id = 'admin' WHERE id = 'conv'`)
	if _, err := svc.Complete(context.Background(), run, episode.ID, decision); err != nil {
		t.Fatal(err)
	}
	latest, err := svc.repo.Latest(context.Background(), "ws", "conv")
	if err != nil || latest.Status != "cancelled" {
		t.Fatalf("pending follow-up not cancelled: %+v %v", latest, err)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("id <> ?", "source").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("disabled inbox sent follow-up: %d %v", count, err)
	}
}
