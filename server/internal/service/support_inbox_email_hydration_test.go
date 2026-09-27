package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/email/inboundhtml"
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
				FromEmail:    "website@acme.com",
				ToEmail:      "support@acme.on.helpin.email",
				CCEmails:     model.DocsStringArray{"teammate1@company.com", "teammate2@company.com"},
				BCCEmails:    model.DocsStringArray{"hidden@company.com"},
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
	if messages[0].EmailFrom != "website@acme.com" {
		t.Fatalf("expected inbound from hydration, got %#v", messages[0])
	}
	if messages[0].EmailReplyTo != "Taylor Visitor <taylor.visitor@example.com>" {
		t.Fatalf("expected inbound reply-to hydration, got %#v", messages[0])
	}
	if messages[0].EmailTo != "support@acme.on.helpin.email" {
		t.Fatalf("expected inbound to hydration, got %#v", messages[0])
	}
	if len(messages[0].EmailCC) != 2 || messages[0].EmailCC[0] != "teammate1@company.com" || messages[0].EmailCC[1] != "teammate2@company.com" {
		t.Fatalf("expected inbound cc hydration, got %#v", messages[0].EmailCC)
	}
	if len(messages[0].EmailBCC) != 1 || messages[0].EmailBCC[0] != "hidden@company.com" {
		t.Fatalf("expected inbound bcc hydration, got %#v", messages[0].EmailBCC)
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

func TestHydrateEmailBodiesUsesCurrentProjectionIncludingExplicitFalse(t *testing.T) {
	viaEmail := "email"
	messages := []model.SupportMessage{
		{ID: "quoted", ViaChannel: &viaEmail},
		{ID: "not-quoted", ViaChannel: &viaEmail},
	}
	reader := fakeSupportEmailLogReader{logs: []model.SupportEmailLog{
		{
			Direction:                 "inbound",
			MessageIDs:                model.DocsStringArray{"quoted"},
			EmailVisibleText:          "Fresh reply",
			EmailQuotedText:           "Old reply",
			EmailHasQuotedContent:     true,
			EmailProjectionConfidence: inboundhtml.ProjectionConfidenceHigh,
			EmailProjectionVersion:    inboundhtml.CurrentProjectionVersion,
		},
		{
			Direction:                 "inbound",
			MessageIDs:                model.DocsStringArray{"not-quoted"},
			EmailVisibleText:          "Only reply",
			EmailHasQuotedContent:     false,
			EmailProjectionConfidence: inboundhtml.ProjectionConfidenceNone,
			EmailProjectionVersion:    inboundhtml.CurrentProjectionVersion,
		},
	}}

	hydrateEmailBodies(context.Background(), reader, "workspace-1", "conversation-1", messages)

	if messages[0].EmailVisibleText != "Fresh reply" || messages[0].EmailQuotedText != "Old reply" {
		t.Fatalf("current quoted projection not hydrated: %#v", messages[0])
	}
	if messages[0].EmailHasQuotedContent == nil || !*messages[0].EmailHasQuotedContent {
		t.Fatalf("quoted projection flag = %#v, want true", messages[0].EmailHasQuotedContent)
	}
	if messages[1].EmailHasQuotedContent == nil || *messages[1].EmailHasQuotedContent {
		t.Fatalf("no-quote projection flag = %#v, want explicit false", messages[1].EmailHasQuotedContent)
	}
}

func TestHydrateEmailBodiesProjectsLegacyHTMLInMemory(t *testing.T) {
	viaEmail := "email"
	messages := []model.SupportMessage{{ID: "legacy", ViaChannel: &viaEmail}}
	reader := fakeSupportEmailLogReader{logs: []model.SupportEmailLog{{
		Direction:    "inbound",
		MessageIDs:   model.DocsStringArray{"legacy"},
		HTMLBody:     `<p>Fresh reply</p><div class="gmail_quote"><p>Old reply</p></div>`,
		StrippedText: "Fresh reply",
	}}}

	hydrateEmailBodies(context.Background(), reader, "workspace-1", "conversation-1", messages)

	if messages[0].EmailVisibleText != "Fresh reply" || messages[0].EmailQuotedText != "Old reply" {
		t.Fatalf("legacy projection = %#v", messages[0])
	}
	if messages[0].EmailHasQuotedContent == nil || !*messages[0].EmailHasQuotedContent {
		t.Fatalf("legacy quote flag = %#v, want true", messages[0].EmailHasQuotedContent)
	}
	if messages[0].EmailProjectionVersion != inboundhtml.CurrentProjectionVersion {
		t.Fatalf("legacy projection version = %d, want current", messages[0].EmailProjectionVersion)
	}
	if messages[0].HTMLBody == "" {
		t.Fatal("legacy rich HTML was discarded")
	}
}

func TestHydrateOutgoingEmailKeepsAuthoredReply(t *testing.T) {
	for _, sender := range []string{"ai", "agent"} {
		t.Run(sender, func(t *testing.T) {
			messages := []model.SupportMessage{{ID: "sent", SenderType: sender, ViaChannel: strPtr("email"), Content: "Let me connect you with a team member."}}
			logs := []model.SupportEmailLog{{Direction: "outbound", MessageIDs: model.DocsStringArray{"sent"}, Status: "delivered", FromEmail: "support@example.com", ToEmail: "customer@example.com", HTMLBody: "<p>Please type your reply above this line</p><p>Reply</p><footer>Powered by Helpin AI</footer>", StrippedText: "Please type your reply above this line\nReply\nPowered by Helpin AI"}}
			hydrateEmailBodiesFromLogs(messages, logs)
			if messages[0].EmailVisibleText != "" || messages[0].HTMLBody != "" || messages[0].StrippedText != "" {
				t.Fatalf("transport wrapper leaked into thread: %+v", messages[0])
			}
			if messages[0].Content != "Let me connect you with a team member." || messages[0].EmailDeliveryStatus != "delivered" || messages[0].EmailFrom != "support@example.com" || messages[0].EmailTo != "customer@example.com" {
				t.Fatalf("authored reply or delivery details lost: %+v", messages[0])
			}
			if logs[0].HTMLBody == "" || logs[0].StrippedText == "" {
				t.Fatal("original email must remain available in email details")
			}
		})
	}
}
