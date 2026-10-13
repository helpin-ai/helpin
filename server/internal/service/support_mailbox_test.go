package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestShouldUseDefaultMailboxForChannel(t *testing.T) {
	defaultMailboxID := "mailbox-1"

	t.Run("widget conversations start in shared inbox", func(t *testing.T) {
		settings := model.DefaultSupportInboxSettings()
		settings.DefaultMailboxID = &defaultMailboxID

		if shouldUseDefaultMailboxForChannel(&settings, "widget") {
			t.Fatal("expected widget conversations to ignore default mailbox")
		}
	})

	t.Run("email conversations can use default mailbox", func(t *testing.T) {
		settings := model.DefaultSupportInboxSettings()
		settings.DefaultMailboxID = &defaultMailboxID
		settings.TriageEnabled = false

		if !shouldUseDefaultMailboxForChannel(&settings, "email") {
			t.Fatal("expected email conversations to use default mailbox")
		}
	})

	t.Run("automated routing shared fallback keeps conversations shared", func(t *testing.T) {
		settings := model.DefaultSupportInboxSettings()
		settings.DefaultMailboxID = &defaultMailboxID
		settings.TriageEnabled = true
		settings.TriageEmailEnabled = true
		settings.TriageFallbackBehavior = "shared"

		if shouldUseDefaultMailboxForChannel(&settings, "email") {
			t.Fatal("expected shared fallback to ignore default mailbox")
		}
	})
}

func TestResolveAIHandoffMailboxDoesNotFallbackToDefaultMailbox(t *testing.T) {
	defaultMailboxID := "mailbox-1"
	settings := model.DefaultSupportInboxSettings()
	settings.DefaultMailboxID = &defaultMailboxID
	settings.AIHandoffMailboxID = nil

	mailboxID, mailbox, err := (&SupportInboxService{}).resolveAIHandoffMailbox(context.Background(), "workspace-1", &settings)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mailboxID != nil || mailbox != nil {
		t.Fatalf("expected shared inbox handoff, got mailboxID=%v mailbox=%v", mailboxID, mailbox)
	}
}

func TestMoveConversationPreservesReadState(t *testing.T) {
	tests := []struct {
		name       string
		lastSeenAt *time.Time
		wantUnread int
	}{
		{
			name:       "read conversation remains read",
			lastSeenAt: supportMailboxTimePtr(time.Date(2026, time.January, 1, 11, 0, 0, 0, time.UTC)),
			wantUnread: 0,
		},
		{
			name:       "unread conversation remains unread",
			lastSeenAt: nil,
			wantUnread: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newSupportTriageTestFixture(t, nil, nil)
			targetMailbox := fixture.createMailbox(t, "Billing", "billing", true)
			conversation := fixture.createConversation(t, "Billing question", "buyer@example.com", nil)
			message := fixture.createCustomerReply(t, conversation.ID, "I have a billing question.")

			messageTime := time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC)
			if err := fixture.db.Model(&model.SupportMessage{}).
				Where("id = ?", message.ID).
				UpdateColumn("created_at", messageTime).Error; err != nil {
				t.Fatalf("set message timestamp: %v", err)
			}
			if err := fixture.db.Model(&model.SupportConversation{}).
				Where("id = ?", conversation.ID).
				UpdateColumn("team_last_seen_at", tt.lastSeenAt).Error; err != nil {
				t.Fatalf("set conversation read state: %v", err)
			}

			if _, err := fixture.supportSvc.moveConversationInternal(
				fixture.ctx,
				fixture.workspaceID,
				conversation.ID,
				&targetMailbox.ID,
				fixture.actorID,
				supportConversationMoveOptions{
					EnforceMailboxAccess: false,
					UseAccessibleLoad:    false,
				},
			); err != nil {
				t.Fatalf("move conversation: %v", err)
			}

			moved, err := fixture.conversationRepo.GetByID(
				fixture.ctx,
				fixture.workspaceID,
				conversation.ID,
				"",
				model.RoleOwner,
			)
			if err != nil {
				t.Fatalf("load moved conversation: %v", err)
			}
			if tt.lastSeenAt == nil && moved.TeamLastSeenAt != nil {
				t.Fatalf("team_last_seen_at = %v, want nil", moved.TeamLastSeenAt)
			}
			if tt.lastSeenAt != nil && (moved.TeamLastSeenAt == nil || !moved.TeamLastSeenAt.Equal(*tt.lastSeenAt)) {
				t.Fatalf("team_last_seen_at = %v, want %v", moved.TeamLastSeenAt, *tt.lastSeenAt)
			}

			mailboxUnread, err := fixture.mailboxRepo.CountUnread(
				fixture.ctx,
				fixture.workspaceID,
				&targetMailbox.ID,
			)
			if err != nil {
				t.Fatalf("count destination mailbox unread: %v", err)
			}
			if mailboxUnread != tt.wantUnread {
				t.Fatalf("destination mailbox unread count = %d, want %d", mailboxUnread, tt.wantUnread)
			}
		})
	}
}

func supportMailboxTimePtr(value time.Time) *time.Time {
	return &value
}

func TestMoveConversationToSharedPausesAI(t *testing.T) {
	for _, source := range []string{"shared", "team"} {
		t.Run(source, func(t *testing.T) {
			f := newSupportTriageTestFixture(t, nil, nil)
			var mailboxID *string
			if source == "team" {
				mailboxID = &f.createMailbox(t, "Support", "support", true).ID
			}
			conv := f.createConversation(t, "Need assistance", "customer@example.com", mailboxID)
			if err := f.conversationRepo.UpdateFields(f.ctx, f.workspaceID, conv.ID, map[string]any{
				"flow_state": model.SupportConversationFlowStateAIHandling, "ai_state": "pending",
				"assigned_agent_id": "agent", "ai_active_run_id": "old-run",
				"last_public_message_id": "ai-reply", "last_public_sender_type": "ai",
			}); err != nil {
				t.Fatal(err)
			}
			before, err := f.conversationRepo.GetByID(f.ctx, f.workspaceID, conv.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			settings := model.DefaultSupportInboxSettings()
			settings.AIEnabled, settings.AIFollowUpEnabled = true, true
			settings.AIResponseMode, settings.AIAgentID = "ai_first", strPtr("agent")
			episode := &model.SupportAIFollowUp{SourceMessageID: "ai-reply", Status: "scheduled"}
			if !supportFollowUpEligible(before, episode, settings) {
				t.Fatal("fixture must have an eligible AI follow-up before the move")
			}
			closer := &failingControlRunCloser{}
			f.supportSvc.supportAIService = &SupportAIService{runCloser: closer}
			moved, err := f.supportSvc.MoveConversation(f.ctx, f.workspaceID, conv.ID, nil, f.actorID)
			if err != nil {
				t.Fatal(err)
			}
			if moved.MailboxID != nil || derefString(moved.FlowState) != model.SupportConversationFlowStateWaitingForHuman || !model.SupportAIConversationBlocked(moved) {
				t.Fatalf("move did not leave the conversation in the shared human queue: %+v", moved)
			}
			if moved.AIActiveRunID != nil || moved.AssignedAgentID != nil || moved.AIPausedAt == nil || derefString(moved.AIPausedByUserID) != f.actorID || moved.AIControlVersion != 1 {
				t.Fatal("move did not release AI ownership and record the pause")
			}
			if len(closer.calls) != 1 || closer.calls[0] != "old-run" {
				t.Fatalf("active AI run was not cancelled: %v", closer.calls)
			}
			if supportFollowUpEligible(moved, episode, settings) || currentSupportFollowUp(moved, episode) != nil {
				t.Fatal("AI follow-up remained active after the move")
			}
			if bound, err := f.conversationRepo.BindAIRun(f.ctx, conv, "late-run"); err != nil || bound {
				t.Fatalf("a stale AI launch acquired ownership after the move: %v/%v", bound, err)
			}
			var publicCount int64
			if err := f.db.Model(&model.SupportMessage{}).Where("conversation_id = ? AND is_internal = false", conv.ID).Count(&publicCount).Error; err != nil || publicCount != 0 {
				t.Fatalf("manual move must not send a customer message: %d/%v", publicCount, err)
			}
		})
	}
}

func TestAutomatedMoveToSharedDoesNotPauseAI(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	box := f.createMailbox(t, "Support", "support", true)
	conv := f.createConversation(t, "Need assistance", "customer@example.com", &box.ID)
	if err := f.conversationRepo.UpdateFields(f.ctx, f.workspaceID, conv.ID, map[string]any{
		"flow_state": model.SupportConversationFlowStateAIHandling, "ai_state": "pending",
		"assigned_agent_id": "agent", "ai_active_run_id": "old-run",
	}); err != nil {
		t.Fatal(err)
	}
	moved, err := f.supportSvc.moveConversationInternal(f.ctx, f.workspaceID, conv.ID, nil, "", supportConversationMoveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if model.SupportAIConversationBlocked(moved) || moved.AIPausedAt != nil || derefString(moved.AIActiveRunID) != "old-run" || derefString(moved.AssignedAgentID) != "agent" || moved.AIControlVersion != 0 {
		t.Fatal("automatic routing must not take control away from AI")
	}
}

func TestMoveConversationToSharedRollsBackIfPauseFails(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	box := f.createMailbox(t, "Support", "support", true)
	conv := f.createConversation(t, "Need assistance", "customer@example.com", &box.ID)
	if err := f.conversationRepo.UpdateFields(f.ctx, f.workspaceID, conv.ID, map[string]any{
		"flow_state": model.SupportConversationFlowStateAIHandling, "ai_state": "pending",
		"assigned_agent_id": "agent", "ai_active_run_id": "old-run",
	}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.db, `CREATE TRIGGER reject_pause_note BEFORE INSERT ON support_messages
		WHEN NEW.system_event_type = 'ai_paused' BEGIN SELECT RAISE(ABORT, 'pause note failed'); END`)
	if _, err := f.supportSvc.MoveConversation(f.ctx, f.workspaceID, conv.ID, nil, f.actorID); err == nil {
		t.Fatal("expected pause failure")
	}
	current, err := f.conversationRepo.GetByID(f.ctx, f.workspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if derefString(current.MailboxID) != box.ID || derefString(current.FlowState) != model.SupportConversationFlowStateAIHandling || model.SupportAIConversationBlocked(current) || current.AIControlVersion != 0 || derefString(current.AIActiveRunID) != "old-run" {
		t.Fatal("a failed pause must not commit a partial inbox move")
	}
}
