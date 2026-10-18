package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestEmailFallbackUsesVerifiedSenderWithoutTryingReceivingAddress(t *testing.T) {
	for _, tc := range []struct {
		name          string
		inboxRoute    bool
		pendingSender bool
	}{
		{name: "conversation reply address"},
		{name: "workspace inbox reply address", inboxRoute: true},
		{name: "custom sender still awaiting DNS", inboxRoute: true, pendingSender: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			settings := model.DefaultSupportInboxSettings()
			settings.EmailFallbackEnabled = true
			settings.EmailFallbackFromName = "Acme Support"
			env := setupEmailFallbackInboundTestEnv(t, settings)
			conv := &model.SupportConversation{
				ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: "11111111-1111-1111-1111-111111111111",
				Status: "open", CustomerEmail: strPtr("customer@example.com"),
			}
			if err := env.convRepo.Create(ctx, conv); err != nil {
				t.Fatal(err)
			}
			wantReplyTo := `"Acme Support" <conv-` + conv.ID + `@replies.helpin.ai>`
			if tc.inboxRoute {
				route := &model.SupportEmailRoute{
					ID: "55555555-5555-5555-5555-555555555555", WorkspaceID: conv.WorkspaceID,
					RouteKey: "route-test", InboundAddress: "inbox@acme.on.helpin.email", ProviderType: "forwarding", Active: true,
				}
				if err := env.routeRepo.Create(ctx, route); err != nil {
					t.Fatal(err)
				}
				wantReplyTo = `"Acme Support" <inbox@acme.on.helpin.email>`
			}
			if tc.pendingSender {
				sender := verifiedSender(conv.WorkspaceID, nil, "support@acme.example", "workspace", "22222222-2222-2222-2222-222222222222")
				sender.DKIMVerified = false
				sender.DomainStatus = supportEmailSenderStatusPendingDNS
				sender.VerificationStatus = supportEmailSenderStatusPendingDNS
				if err := env.senderRepo.Create(ctx, sender); err != nil {
					t.Fatal(err)
				}
			}
			message := &model.SupportMessage{
				WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "user",
				SenderDisplayName: strPtr("Alex"), MessageType: "reply", Content: "We can help.",
			}
			if err := env.messageRepo.Create(ctx, message); err != nil {
				t.Fatal(err)
			}
			var attempts []capturedPostmarkRequest
			env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var sent capturedPostmarkRequest
				if err := json.NewDecoder(req.Body).Decode(&sent); err != nil {
					t.Fatal(err)
				}
				attempts = append(attempts, sent)
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ErrorCode":0,"MessageID":"verified-first-attempt"}`))}, nil
			})})
			if err := env.service.fireEmail(ctx, conv.ID, []string{message.ID}); err != nil {
				t.Fatal(err)
			}
			if len(attempts) != 1 {
				t.Fatalf("expected one send using the verified sender, got %d", len(attempts))
			}
			if attempts[0].From != "Alex - Acme Support <noreply@example.com>" {
				t.Fatalf("attempted unverified receiving address as sender: %q", attempts[0].From)
			}
			if attempts[0].ReplyTo != wantReplyTo {
				t.Fatalf("reply routing changed: got %q, want %q", attempts[0].ReplyTo, wantReplyTo)
			}
			logs, err := env.emailLogRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(logs) != 1 || logs[0].FromEmail != "noreply@example.com" || logs[0].FromSource != "verified_fallback_sender" || logs[0].FromFallbackReason != "" {
				t.Fatalf("unexpected sender log: %+v", logs)
			}
		})
	}
}
