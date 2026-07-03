package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestResolveConfiguredHandoffMailboxDoesNotFallbackToDefaultMailbox(t *testing.T) {
	defaultMailboxID := "mailbox-1"
	settings := model.DefaultSupportInboxSettings()
	settings.DefaultMailboxID = &defaultMailboxID
	settings.AIHandoffMailboxID = nil

	mailboxID := (&SupportAIService{}).resolveConfiguredHandoffMailbox(context.Background(), "workspace-1", settings)
	if mailboxID != nil {
		t.Fatalf("expected no configured handoff mailbox, got %q", *mailboxID)
	}
}
