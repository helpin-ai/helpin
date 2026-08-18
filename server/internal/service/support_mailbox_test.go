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
