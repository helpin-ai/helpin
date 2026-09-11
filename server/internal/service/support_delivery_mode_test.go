package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

func explicitDeliveryRequest(t *testing.T, mode string) model.CreateMessageRequest {
	t.Helper()
	var req model.CreateMessageRequest
	if err := json.Unmarshal([]byte(`{"content":"Private email body","delivery_mode":"`+mode+`"}`), &req); err != nil {
		t.Fatal(err)
	}
	return req
}

func deliveryModeFixture(t *testing.T) (*emailFallbackTestEnv, *model.SupportConversation) {
	t.Helper()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 30
	env := setupEmailFallbackTestEnv(t, settings)
	env.service.supportInboxService.SetEmailFallbackService(env.service)
	conv := &model.SupportConversation{ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: "11111111-1111-1111-1111-111111111111", Subject: "Delivery", Status: "open", CustomerEmail: strPtr("customer@example.com"), AnonymousID: strPtr("visitor")}
	if err := env.convRepo.Create(context.Background(), conv); err != nil {
		t.Fatal(err)
	}
	return env, conv
}

func TestSupportExplicitEmailOnlyQueuesWithoutChatExposure(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	if err := env.service.hub.Presence.SetVisitorOnline(ctx, conv.WorkspaceID, *conv.AnonymousID, "conn"); err != nil {
		t.Fatal(err)
	}
	svc := env.service.supportInboxService
	msg, err := svc.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, "email_only"), "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, strPtr("Owner"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg.Metadata, `"delivery_mode":"email_only"`) || msg.CancellableUntil == nil {
		t.Fatalf("message=%+v", msg)
	}
	ids, err := env.redis.LRange(ctx, env.service.msgListKey(conv.ID), 0, -1).Result()
	if err != nil || len(ids) != 1 || ids[0] != msg.ID {
		t.Fatalf("queue=%v err=%v", ids, err)
	}
	staff, err := svc.ListConversationMessages(ctx, conv.WorkspaceID, conv.ID, true)
	if err != nil || len(staff) != 1 {
		t.Fatalf("staff=%+v err=%v", staff, err)
	}
	widget, err := svc.ListWidgetConversationMessages(ctx, conv.WorkspaceID, conv.ID)
	if err != nil || len(widget) != 0 {
		t.Fatalf("widget=%+v err=%v", widget, err)
	}
	event := websocket.SupportMessageEvent(conv.WorkspaceID, msg, "owner")
	if len(event.Data) != 0 {
		t.Fatalf("widget event=%s", event.Data)
	}
	conversations, err := svc.GetVisitorConversations(ctx, conv.WorkspaceID, *conv.AnonymousID)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(conversations)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "Private email body") || conversations[0].UnreadCount != 0 {
		t.Fatalf("visitor projection=%s", body)
	}
}

func TestSupportExplicitEmailValidationPrecedesMessage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*emailFallbackTestEnv, *model.SupportConversation)
		want   string
	}{
		{"no address", func(e *emailFallbackTestEnv, c *model.SupportConversation) { c.CustomerEmail = nil }, "email address"},
		{"opted out", func(e *emailFallbackTestEnv, c *model.SupportConversation) { c.EmailUnsubscribed = true }, "opted out"},
		{"unconfirmed", func(e *emailFallbackTestEnv, c *model.SupportConversation) {
			c.PrimaryRecipientState = model.SupportPrimaryRecipientStateUnconfirmed
		}, "confirm"},
		{"unconfigured", func(e *emailFallbackTestEnv, c *model.SupportConversation) { e.service.emailClient = nil }, "configured"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, conv := deliveryModeFixture(t)
			tc.change(env, conv)
			if err := env.convRepo.Update(context.Background(), conv); err != nil {
				t.Fatal(err)
			}
			_, err := env.service.supportInboxService.CreateConversationMessage(context.Background(), conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, "email_only"), "user", nil, nil, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want=%s", err, tc.want)
			}
			rows, err := env.messageRepo.ListByConversation(context.Background(), conv.WorkspaceID, conv.ID, true)
			if err != nil || len(rows) != 0 {
				t.Fatalf("saved failed message=%+v err=%v", rows, err)
			}
		})
	}
}

func TestSupportExplicitEmailBatchDoesNotForceLegacyOrChatOnly(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	env.service.now = func() time.Time { return now }
	if err := env.service.hub.Presence.SetVisitorOnline(ctx, conv.WorkspaceID, *conv.AnonymousID, "conn"); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, tc := range []struct{ id, mode string }{{"legacy", ""}, {"chat", "chat_only"}, {"email", "email_only"}, {"both", "chat_and_email"}} {
		msg := &model.SupportMessage{ID: tc.id, WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "user", MessageType: "reply", Content: tc.id + " private body", CreatedAt: now.Add(-time.Minute)}
		if tc.mode != "" {
			msg.Metadata = `{"delivery_mode":"` + tc.mode + `"}`
		}
		if err := env.messageRepo.Create(ctx, msg); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, msg.ID)
	}
	seen := now
	conv.ContactLastSeenAt = &seen
	if err := env.convRepo.Update(ctx, conv); err != nil {
		t.Fatal(err)
	}
	var sent []capturedPostmarkRequest
	env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body capturedPostmarkRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		sent = append(sent, body)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"ErrorCode":0,"MessageID":"delivery-mode-email-%d"}`, len(sent))))}, nil
	})})
	for i := 0; i < 3; i++ {
		if err := env.service.fireEmail(ctx, conv.ID, ids); err != nil {
			t.Fatal(err)
		}
	}
	if len(sent) != 2 {
		t.Fatalf("sends=%d want separate explicit modes", len(sent))
	}
	for _, mail := range sent {
		if strings.Contains(mail.TextBody, "legacy private body") || strings.Contains(mail.TextBody, "chat private body") {
			t.Fatalf("wrong batch=%s", mail.TextBody)
		}
	}
	rows, err := env.messageRepo.GetByIDs(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if (row.EmailNotifiedAt != nil) != (row.ID == "email" || row.ID == "both") {
			t.Errorf("message %s email notified=%v", row.ID, row.EmailNotifiedAt)
		}
	}
}

func TestSupportExplicitEmailQueueFailureRollsBackReplyAndJoinedNotice(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	if err := env.redis.Set(ctx, env.service.msgListKey(conv.ID), "wrong-type", 0).Err(); err != nil {
		t.Fatal(err)
	}
	_, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, "chat_and_email"), "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, strPtr("Owner"))
	if err == nil || !strings.Contains(err.Error(), "queue") {
		t.Fatalf("error=%v", err)
	}
	rows, err := env.messageRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID, true)
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows=%+v error=%v", rows, err)
	}
}

func captureExplicitDeliveryEmails(t *testing.T, env *emailFallbackTestEnv) *[]capturedPostmarkRequest {
	t.Helper()
	sent := []capturedPostmarkRequest{}
	env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body capturedPostmarkRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		sent = append(sent, body)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"ErrorCode":0,"MessageID":"explicit-%d"}`, len(sent))))}, nil
	})})
	return &sent
}

func TestSupportExplicitEmailPreservesQueueAndUndoDeadlines(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	env.service.now = func() time.Time { return now }
	var ids []string
	for _, mode := range []string{"email_only", "chat_and_email"} {
		msg, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, mode), "user", nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, msg.ID)
		now = now.Add(10 * time.Second)
	}
	sent := captureExplicitDeliveryEmails(t, env)
	now = now.Add(11 * time.Second) // First is due; second still has nine seconds to undo.
	if err := env.service.fireEmail(ctx, conv.ID, ids); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("sent=%d", len(*sent))
	}
	queued, err := env.redis.LRange(ctx, env.service.msgListKey(conv.ID), 0, -1).Result()
	if err != nil || len(queued) != 1 || queued[0] != ids[1] {
		t.Fatalf("queued=%v error=%v", queued, err)
	}
	alreadySent, err := env.service.CancelForMessage(ctx, conv.WorkspaceID, conv.ID, ids[1])
	if err != nil || alreadySent {
		t.Fatalf("cancel sent=%v error=%v", alreadySent, err)
	}
	if count := env.redis.Exists(ctx, env.service.msgListKey(conv.ID)).Val(); count != 0 {
		t.Fatalf("queue remains=%d", count)
	}
}

func TestSupportExplicitEmailRecipientChangeBlocksDelivery(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	env.service.now = func() time.Time { return now }
	msg, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, "email_only"), "user", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	sent := captureExplicitDeliveryEmails(t, env)
	conv.CustomerEmail = strPtr("other@example.com")
	if err := env.convRepo.Update(ctx, conv); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := env.service.fireEmail(ctx, conv.ID, []string{msg.ID}); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 0 {
		t.Fatalf("sent=%v", sent)
	}
	row, err := env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !explicitEmailBlocked(*row) || !strings.Contains(explicitDeliveryMetadata(*row).Error, "recipient changed") {
		t.Fatalf("message=%+v", row)
	}
	info, err := NewSupportMessageActionsService(env.messageRepo, env.service, env.emailLogRepo, nil).Info(ctx, conv.WorkspaceID, conv.ID, "owner", msg.ID)
	if err != nil || info.Read || info.EmailDeliveryStatus != "blocked" || info.NotDeliveredReason == nil {
		t.Fatalf("info=%+v error=%v", info, err)
	}
}

func TestSupportExplicitEmailReconciliationRecoversOldReadResolvedReply(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	env.service.now = func() time.Time { return now }
	conv.Status = "resolved"
	conv.ContactLastSeenAt = &now
	if err := env.convRepo.Update(ctx, conv); err != nil {
		t.Fatal(err)
	}
	if err := env.service.hub.Presence.SetVisitorOnline(ctx, conv.WorkspaceID, *conv.AnonymousID, "conn"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ id, mode, status string }{{"explicit", "email_only", "queued"}, {"chat", "chat_only", ""}, {"legacy", "", ""}, {"blocked", "email_only", "blocked"}} {
		msg := &model.SupportMessage{ID: tc.id, WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "user", MessageType: "reply", Content: tc.id, CreatedAt: now.Add(-2 * time.Hour), Metadata: fmt.Sprintf(`{"delivery_mode":%q,"email_delivery_status":%q}`, tc.mode, tc.status)}
		if err := env.messageRepo.Create(ctx, msg); err != nil {
			t.Fatal(err)
		}
	}
	sent := captureExplicitDeliveryEmails(t, env)
	count, err := env.service.ReconcileMissedOutboundEmails(ctx, 25)
	if err != nil || count != 1 || len(*sent) != 1 {
		t.Fatalf("count=%d emails=%d error=%v", count, len(*sent), err)
	}
}

func TestSupportExplicitChatRequiresWidgetSession(t *testing.T) {
	for _, mode := range []string{"chat_only", "chat_and_email"} {
		t.Run(mode, func(t *testing.T) {
			env, conv := deliveryModeFixture(t)
			conv.AnonymousID = nil
			conv.Source = "email"
			if err := env.convRepo.Update(context.Background(), conv); err != nil {
				t.Fatal(err)
			}
			_, err := env.service.supportInboxService.CreateConversationMessage(context.Background(), conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, mode), "user", nil, nil, nil)
			if err == nil || !strings.Contains(err.Error(), "chat session") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestSupportExplicitDeliveryNullMetadata(t *testing.T) {
	msg := model.SupportMessage{Metadata: withSupportDeliveryMode("null", "email_only")}
	if !msg.ExplicitEmailDelivery() {
		t.Fatalf("metadata=%s", msg.Metadata)
	}
}

func TestSupportExplicitEmailReplyReturnsToSameConversation(t *testing.T) {
	for _, routeEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("route=%v", routeEnabled), func(t *testing.T) {
			env, conv := deliveryModeFixture(t)
			ctx := context.Background()
			now := time.Now().UTC()
			env.service.now = func() time.Time { return now }
			if routeEnabled {
				route := &model.SupportEmailRoute{ID: "55555555-5555-5555-5555-555555555555", WorkspaceID: conv.WorkspaceID, RouteKey: "route-explicit", InboundAddress: "inbox@acme.on.helpin.email", ProviderType: "forwarding", Active: true}
				if err := env.routeRepo.Create(ctx, route); err != nil {
					t.Fatal(err)
				}
			}
			msg, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, "email_only"), "user", nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			sent := captureExplicitDeliveryEmails(t, env)
			now = now.Add(time.Minute)
			if err := env.service.fireEmail(ctx, conv.ID, []string{msg.ID}); err != nil {
				t.Fatal(err)
			}
			if len(*sent) != 1 {
				t.Fatalf("sent=%d", len(*sent))
			}
			outbound := (*sent)[0]
			replyTo, err := mail.ParseAddress(outbound.ReplyTo)
			if err != nil {
				t.Fatal(err)
			}
			var messageID string
			for _, header := range outbound.Headers {
				if header.Name == "Message-ID" {
					messageID = header.Value
				}
			}
			if messageID == "" {
				t.Fatal("outbound RFC Message-ID missing")
			}
			payload := model.PostmarkInboundPayload{FromFull: model.PostmarkAddress{Email: *conv.CustomerEmail, Name: "Customer"}, To: replyTo.Address, OriginalRecipient: replyTo.Address, Subject: "Re: " + conv.Subject, MessageID: "explicit-inbound", StrippedTextReply: "Thanks for the email.", Headers: []model.PostmarkHeader{{Name: "Message-ID", Value: "<customer-explicit@example.com>"}, {Name: "In-Reply-To", Value: messageID}}}
			if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
				t.Fatal(err)
			}
			rows, err := env.service.supportInboxService.ListConversationMessages(ctx, conv.WorkspaceID, conv.ID, true)
			if err != nil || len(rows) != 2 {
				t.Fatalf("rows=%+v error=%v", rows, err)
			}
			widget, err := env.service.supportInboxService.ListWidgetConversationMessages(ctx, conv.WorkspaceID, conv.ID)
			if err != nil || len(widget) != 1 || widget[0].SenderType != "customer" || widget[0].Content != "Thanks for the email." {
				t.Fatalf("widget=%+v error=%v", widget, err)
			}
		})
	}
}

func TestSupportExplicitEmailFailurePersistsRetryStatus(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	env.service.now = func() time.Time { return now }
	msg, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, "email_only"), "user", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("sensitive provider diagnostic") })})
	if err := env.service.fireEmail(ctx, conv.ID, []string{msg.ID}); err == nil {
		t.Fatal("expected send error")
	}
	row, err := env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	meta := explicitDeliveryMetadata(*row)
	if meta.Status != "failed" || strings.Contains(meta.Error, "sensitive") || row.EmailNotifiedAt != nil {
		t.Fatalf("row=%+v", row)
	}
	sent := captureExplicitDeliveryEmails(t, env)
	if err := env.service.fireEmail(ctx, conv.ID, []string{msg.ID}); err != nil {
		t.Fatal(err)
	}
	row, err = env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 || explicitDeliveryMetadata(*row).Status != "sent" || row.EmailNotifiedAt == nil {
		t.Fatalf("row=%+v sent=%d", row, len(*sent))
	}
}

func TestSupportExplicitChatOnlyDoesNotEnqueueEmail(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	msg, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, "chat_only"), "user", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !msg.WidgetVisible() || msg.CancellableUntil != nil {
		t.Fatalf("message=%+v", msg)
	}
	if err := env.service.OnAgentReply(ctx, conv.WorkspaceID, msg, conv); err != nil {
		t.Fatal(err)
	}
	if count := env.redis.Exists(ctx, env.service.msgListKey(conv.ID)).Val(); count != 0 {
		t.Fatalf("queue=%d", count)
	}
}

func TestSupportExplicitDeliveryClearsEmptyOutboxWithoutDroppingNewReply(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	if err := env.redis.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: 1, Member: conv.ID}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := env.service.fireEmail(ctx, conv.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := env.redis.ZScore(ctx, emailFallbackOutboxKey, conv.ID).Result(); err != redis.Nil {
		t.Fatalf("orphan still queued: %v", err)
	}
	msg, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, "email_only"), "user", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.service.fireEmail(ctx, conv.ID, nil); err != nil {
		t.Fatal(err)
	}
	ids, err := env.redis.LRange(ctx, env.service.msgListKey(conv.ID), 0, -1).Result()
	if err != nil || len(ids) != 1 || ids[0] != msg.ID {
		t.Fatalf("ids=%v error=%v", ids, err)
	}
}

func TestSupportExplicitEmailBatchMaintainsProcessingClaim(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	env.service.now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		_, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, "email_only"), "user", nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	now = now.Add(time.Minute)
	sends := 0
	env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		sends++
		now = now.Add(31 * time.Second)
		score, err := env.redis.ZScore(ctx, emailFallbackOutboxKey, conv.ID).Result()
		if err != nil || int64(score) <= now.Unix() {
			t.Fatalf("processing claim expired during batch score=%v error=%v", score, err)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"ErrorCode":0,"MessageID":"claim-%d"}`, sends)))}, nil
	})})
	if err := env.service.claimAndFire(ctx, conv.ID); err != nil {
		t.Fatal(err)
	}
	if sends != 2 {
		t.Fatalf("sends=%d", sends)
	}
}

func TestSupportExplicitChatAndEmailJoinedNoticePrecedesReply(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	ctx := context.Background()
	msg, err := env.service.supportInboxService.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, explicitDeliveryRequest(t, "chat_and_email"), "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, strPtr("Owner"))
	if err != nil {
		t.Fatal(err)
	}
	widget, err := env.service.supportInboxService.ListWidgetConversationMessages(ctx, conv.WorkspaceID, conv.ID)
	if err != nil || len(widget) != 2 || widget[0].MessageType != "system" || widget[1].ID != msg.ID {
		t.Fatalf("widget=%+v error=%v", widget, err)
	}
}
