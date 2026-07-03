package service

import (
	"context"
	"testing"

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
