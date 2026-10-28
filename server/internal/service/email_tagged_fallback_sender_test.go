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

func TestSupportConversationFallbackAddress(t *testing.T) {
	const conversationID = "33333333-3333-3333-3333-333333333333"
	tests := []struct {
		name           string
		base           string
		conversationID string
		want           string
	}{
		{name: "tags the shared sender", base: "support@helpin.email", conversationID: conversationID, want: "support+conv-" + conversationID + "@helpin.email"},
		{name: "keeps an address that is already tagged", base: "support+other@helpin.email", conversationID: conversationID, want: "support+other@helpin.email"},
		{name: "keeps the sender without a conversation", base: "support@helpin.email", conversationID: "", want: "support@helpin.email"},
		{name: "rejects a malformed conversation id", base: "support@helpin.email", conversationID: "not-a-uuid", want: "support@helpin.email"},
		{name: "leaves an empty sender empty", base: "", conversationID: conversationID, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportConversationFallbackAddress(tt.base, tt.conversationID); got != tt.want {
				t.Fatalf("supportConversationFallbackAddress(%q, %q) = %q, want %q", tt.base, tt.conversationID, got, tt.want)
			}
		})
	}
}

func TestMailboxHashFromRecipientReadsTaggedFallbackSender(t *testing.T) {
	const conversationID = "33333333-3333-3333-3333-333333333333"
	tests := []struct {
		recipient string
		want      string
	}{
		{recipient: "support+conv-" + conversationID + "@helpin.email", want: "conv-" + conversationID},
		{recipient: `"Acme Support" <support+conv-` + conversationID + `@helpin.email>`, want: "conv-" + conversationID},
		{recipient: "conv-" + conversationID + "@replies.helpin.email", want: "conv-" + conversationID},
		{recipient: "support+newsletter@helpin.email", want: ""},
		{recipient: "support@helpin.email", want: ""},
	}
	for _, tt := range tests {
		if got := mailboxHashFromRecipient(tt.recipient); got != tt.want {
			t.Fatalf("mailboxHashFromRecipient(%q) = %q, want %q", tt.recipient, got, tt.want)
		}
	}
}

// A customer who writes to the From address of a fallback email (instead of
// using Reply-To) must still reach the conversation.
func TestEmailFallbackInboundToTaggedFallbackSenderReachesConversation(t *testing.T) {
	for _, tc := range []struct {
		name         string
		providerHash bool
	}{
		{name: "provider reports the hash", providerHash: true},
		{name: "hash read from the recipient"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
			customerEmail := "customer@example.com"
			conv := &model.SupportConversation{
				ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: "11111111-1111-1111-1111-111111111111",
				Subject: "Pricing", Status: "resolved", CustomerEmail: &customerEmail,
			}
			if err := env.convRepo.Create(ctx, conv); err != nil {
				t.Fatal(err)
			}
			tagged := "noreply+conv-" + conv.ID + "@example.com"
			payload := model.PostmarkInboundPayload{
				FromFull:          model.PostmarkAddress{Email: customerEmail, Name: "Taylor"},
				To:                `"Acme Support" <` + tagged + `>`,
				ToFull:            []model.PostmarkAddress{{Email: tagged, Name: "Acme Support"}},
				OriginalRecipient: tagged,
				Subject:           "Re: Pricing",
				MessageID:         "pm-tagged-" + strings.ReplaceAll(tc.name, " ", "-"),
				StrippedTextReply: "Following up on the quote.",
				TextBody:          "Following up on the quote.",
			}
			if tc.providerHash {
				payload.MailboxHash = "conv-" + conv.ID
			}
			if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
				t.Fatalf("process inbound email: %v", err)
			}
			messages, err := env.messageRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(messages) != 1 || messages[0].SenderType != "customer" || messages[0].IsInternal {
				t.Fatalf("expected one public customer reply, got %+v", messages)
			}
			var stored model.SupportConversation
			if err := env.convRepo.DB().Where("id = ?", conv.ID).First(&stored).Error; err != nil {
				t.Fatal(err)
			}
			for _, cc := range stored.EmailCC {
				if strings.EqualFold(cc, tagged) {
					t.Fatalf("own fallback sender was added to CC: %v", stored.EmailCC)
				}
			}
		})
	}
}

func TestEmailFallbackRetriesPlainSenderWhenTaggedSenderRejected(t *testing.T) {
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
		if len(attempts) == 1 {
			return &http.Response{StatusCode: http.StatusUnprocessableEntity, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(
				`{"ErrorCode":300,"Message":"The 'From' address you supplied is not a Sender Signature on your account."}`))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ErrorCode":0,"MessageID":"plain-sender"}`))}, nil
	})})
	if err := env.service.fireEmail(ctx, conv.ID, []string{message.ID}); err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected tagged attempt then plain retry, got %d attempts", len(attempts))
	}
	if attempts[0].From != "Alex - Acme Support <noreply+conv-"+conv.ID+"@example.com>" || attempts[1].From != "Alex - Acme Support <noreply@example.com>" {
		t.Fatalf("unexpected sender order: %q then %q", attempts[0].From, attempts[1].From)
	}
	if attempts[0].ReplyTo != attempts[1].ReplyTo {
		t.Fatalf("reply-to changed across retry: %q then %q", attempts[0].ReplyTo, attempts[1].ReplyTo)
	}
	logs, err := env.emailLogRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].FromEmail != "noreply@example.com" || logs[0].FromFallbackReason != "postmark_sender_signature_rejected" {
		t.Fatalf("unexpected sender log: %+v", logs)
	}
}
