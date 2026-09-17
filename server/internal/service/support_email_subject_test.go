package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func subjectDeliveryRequest(t *testing.T, mode, subject string) model.CreateMessageRequest {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"content": "Subject-specific reply", "delivery_mode": mode, "email_subject": subject})
	if err != nil {
		t.Fatal(err)
	}
	var req model.CreateMessageRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		t.Fatal(err)
	}
	return req
}

func TestSupportEmailSubjectValidationPrecedesMessage(t *testing.T) {
	for _, tc := range []struct {
		name, mode, subject, sender, messageType string
		internal                                 bool
	}{
		{name: "empty", mode: "email_only", subject: "  ", sender: "user"},
		{name: "long", mode: "email_only", subject: strings.Repeat("a", 501), sender: "user"},
		{name: "CRLF", mode: "email_only", subject: "Subject\r\nBcc: other@example.com", sender: "user"},
		{name: "newline at edge", mode: "email_only", subject: "Subject\n", sender: "user"},
		{name: "control", mode: "email_only", subject: "Subject\x00", sender: "user"},
		{name: "chat", mode: "chat_only", subject: "Subject", sender: "user"},
		{name: "legacy", subject: "Subject", sender: "user"},
		{name: "customer", mode: "email_only", subject: "Subject", sender: "customer"},
		{name: "agent", mode: "email_only", subject: "Subject", sender: "agent"},
		{name: "internal", mode: "email_only", subject: "Subject", sender: "user", internal: true},
		{name: "system", mode: "email_only", subject: "Subject", sender: "user", messageType: "system"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, conv := deliveryModeFixture(t)
			req := subjectDeliveryRequest(t, tc.mode, tc.subject)
			req.IsInternal, req.MessageType = tc.internal, tc.messageType
			ctx := context.Background()
			if _, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, req, tc.sender, nil, nil, nil); err == nil {
				t.Fatal("invalid email subject request accepted")
			}
			rows, err := env.messageRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID, true)
			if err != nil || len(rows) != 0 {
				t.Fatalf("saved failed message=%+v err=%v", rows, err)
			}
		})
	}
}

func TestSupportEmailSubjectSnapshotsQueuedDelivery(t *testing.T) {
	for _, tc := range []struct {
		name, mode, subject, want string
		custom                    bool
	}{
		{name: "default", mode: "email_only", want: "Delivery"},
		{name: "custom email", mode: "email_only", subject: "  Invoice clarification  ", want: "Invoice clarification", custom: true},
		{name: "custom chat and email", mode: "chat_and_email", subject: "Follow up", want: "Follow up", custom: true},
		{name: "unicode limit", mode: "email_only", subject: strings.Repeat("界", 500), want: strings.Repeat("界", 500), custom: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, conv := deliveryModeFixture(t)
			ctx := context.Background()
			now := time.Now().UTC()
			env.service.now = func() time.Time { return now }
			req := explicitDeliveryRequest(t, tc.mode)
			if tc.custom {
				req = subjectDeliveryRequest(t, tc.mode, tc.subject)
			}
			msg, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, req, "user", nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			var meta map[string]any
			if err := json.Unmarshal([]byte(msg.Metadata), &meta); err != nil {
				t.Fatal(err)
			}
			if meta["email_subject"] != tc.want {
				t.Fatalf("subject snapshot=%v want=%q", meta["email_subject"], tc.want)
			}
			unchanged, err := env.convRepo.GetByID(ctx, conv.WorkspaceID, conv.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			if unchanged.Subject != "Delivery" {
				t.Fatalf("conversation renamed to %q", unchanged.Subject)
			}
			conv.Subject = "Renamed after queueing"
			if err := env.convRepo.UpdateSubject(ctx, conv.ID, conv.Subject); err != nil {
				t.Fatal(err)
			}
			sent := captureExplicitDeliveryEmails(t, env)
			now = now.Add(time.Minute)
			if err := env.service.fireEmail(ctx, conv.ID, []string{msg.ID}); err != nil {
				t.Fatal(err)
			}
			if len(*sent) != 1 {
				t.Fatalf("sends=%d", len(*sent))
			}
			if (*sent)[0].Subject != tc.want {
				t.Fatalf("delivered subject=%q want=%q", (*sent)[0].Subject, tc.want)
			}
		})
	}
}

func TestSupportDifferentEmailSubjectsRetainThreading(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	env.service.now = func() time.Time { return now }
	var ids []string
	for _, subject := range []string{"First subject", "Second subject"} {
		msg, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, subjectDeliveryRequest(t, "email_only", subject), "user", nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, msg.ID)
		now = now.Add(time.Second)
	}
	sent := captureExplicitDeliveryEmails(t, env)
	now = now.Add(time.Minute)
	if err := env.service.fireEmail(ctx, conv.ID, ids); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 2 {
		t.Fatalf("sends=%d want separate subjects", len(*sent))
	}
	if (*sent)[0].Subject != "First subject" || (*sent)[1].Subject != "Second subject" {
		t.Fatalf("subjects=%q, %q", (*sent)[0].Subject, (*sent)[1].Subject)
	}
	var firstID, inReplyTo, references string
	for _, header := range (*sent)[0].Headers {
		if header.Name == "Message-ID" {
			firstID = header.Value
		}
	}
	for _, header := range (*sent)[1].Headers {
		if header.Name == "In-Reply-To" {
			inReplyTo = header.Value
		}
		if header.Name == "References" {
			references = header.Value
		}
	}
	if firstID == "" || inReplyTo != firstID || !strings.Contains(references, firstID) {
		t.Fatalf("thread headers: first=%q reply=%q references=%q", firstID, inReplyTo, references)
	}
	if (*sent)[0].ReplyTo == "" || (*sent)[0].ReplyTo != (*sent)[1].ReplyTo {
		t.Fatalf("reply route changed between subjects")
	}
}
