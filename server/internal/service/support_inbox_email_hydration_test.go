package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakeSupportEmailLogReader struct {
	logs []model.SupportEmailLog
	err  error
}

func (f fakeSupportEmailLogReader) ListByConversation(context.Context, string, string) ([]model.SupportEmailLog, error) {
	return f.logs, f.err
}

func TestHydrateEmailBodiesHandlesNilRepo(t *testing.T) {
	messages := []model.SupportMessage{
		{ID: "msg-1", Content: "Fallback reply"},
	}

	hydrateEmailBodies(context.Background(), nil, "workspace-1", "conversation-1", messages)

	if messages[0].HTMLBody != "" || messages[0].EmailDeliveryStatus != "" {
		t.Fatalf("expected nil repo to leave message untouched, got %#v", messages[0])
	}
}

func TestHydrateEmailBodiesAddsInboundBodiesAndOutboundStatus(t *testing.T) {
	viaEmail := "email"
	messages := []model.SupportMessage{
		{ID: "inbound-1", ViaChannel: &viaEmail, Content: "Email reply"},
		{ID: "outbound-1", Content: "Agent fallback"},
		{ID: "outbound-2", Content: "Agent fallback failed"},
		{ID: "unmatched", Content: "Widget-only message"},
	}
	reader := fakeSupportEmailLogReader{
		logs: []model.SupportEmailLog{
			{
				Direction:    "inbound",
				MessageIDs:   model.DocsStringArray{"inbound-1"},
				ReplyTo:      "Taylor Visitor <taylor.visitor@example.com>",
				HTMLBody:     "<p>Email reply</p>",
				StrippedText: "Email reply",
			},
			{
				Direction:  "outbound",
				MessageIDs: model.DocsStringArray{"outbound-1"},
				Status:     "delivered",
			},
			{
				Direction:    "outbound",
				MessageIDs:   model.DocsStringArray{"outbound-2"},
				Status:       "bounced",
				ErrorMessage: "Mailbox unavailable",
			},
		},
	}

	hydrateEmailBodies(context.Background(), reader, "workspace-1", "conversation-1", messages)

	if messages[0].HTMLBody != "<p>Email reply</p>" || messages[0].StrippedText != "Email reply" {
		t.Fatalf("expected inbound email body hydration, got %#v", messages[0])
	}
	if messages[0].EmailReplyTo != "Taylor Visitor <taylor.visitor@example.com>" {
		t.Fatalf("expected inbound reply-to hydration, got %#v", messages[0])
	}
	if messages[1].EmailDeliveryStatus != "delivered" || messages[1].EmailDeliveryError != "" {
		t.Fatalf("expected delivered outbound status, got %#v", messages[1])
	}
	if messages[2].EmailDeliveryStatus != "bounced" || messages[2].EmailDeliveryError != "Mailbox unavailable" {
		t.Fatalf("expected bounced outbound status and error, got %#v", messages[2])
	}
	if messages[3].HTMLBody != "" || messages[3].EmailDeliveryStatus != "" {
		t.Fatalf("expected unmatched widget message to remain unhydrated, got %#v", messages[3])
	}
}
