package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestInboundEmailAbsenceClassification(t *testing.T) {
	tests := []struct {
		name, header, subject, body string
		extra                       []model.PostmarkHeader
		want                        bool
	}{
		{name: "screenshot", header: "auto-generated", subject: "Automatic reply: Your post was partially published", body: "Thanks for reaching out! I am out of the office beginning on Wednesday. I'll respond to all emails upon my return.", want: true},
		{name: "standard absence", header: "auto-replied", body: "I am currently out of the office.", want: true},
		{name: "parameters", header: "auto-replied; owner-email=person@example.com", body: "I'm on annual leave.", want: true},
		{name: "automated alert", header: "auto-generated", body: "The production database is down."},
		{name: "automatic acknowledgement", header: "auto-replied", body: "We received your request and will respond shortly."},
		{name: "manual absence", body: "I am out of the office. Please refund my order."},
		{name: "explicit human", header: "no", body: "I am currently out of the office."},
		{name: "question", header: "auto-replied", body: "I am out of the office. Can you reset my password?"},
		{name: "imperative", header: "auto-replied", body: "I am out of the office. Please cancel my account."},
		{name: "quoted absence", header: "auto-replied", body: "Forwarded message: I am currently out of the office."},
		{name: "unknown language", header: "auto-replied", body: "Je suis absent du bureau."},
		{name: "delivery report", header: "auto-replied", body: "I am out of the office.", extra: []model.PostmarkHeader{{Name: "Content-Type", Value: "multipart/report; report-type=delivery-status"}}},
		{name: "list", header: "auto-replied", body: "I am out of the office.", extra: []model.PostmarkHeader{{Name: "List-Id", Value: "list.example.com"}}},
		{name: "spam", header: "auto-replied", body: "I am out of the office.", extra: []model.PostmarkHeader{{Name: "X-Spam-Status", Value: "Yes, score=7"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := model.PostmarkInboundPayload{Subject: tt.subject, FromFull: model.PostmarkAddress{Email: "customer@example.com"}, Headers: append(tt.extra, model.PostmarkHeader{Name: "Auto-Submitted", Value: tt.header})}
			metadata := inboundEmailAIMetadata("{}", payload, tt.body)
			got := inboundEmailHasAbsenceNotice(metadata)
			if got != tt.want {
				t.Fatalf("notice=%v, want %v; metadata=%s", got, tt.want, metadata)
			}
		})
	}
}

func noticeTestSettings() model.SupportInboxSettings {
	s := model.DefaultSupportInboxSettings()
	s.AIEnabled = true
	s.AIAgentID = strPtr("agent")
	s.AIReplyChannels = "both"
	s.AIResponseMode = "ai_first"
	return s
}

func noticeTestPayload(recipient string) model.PostmarkInboundPayload {
	return model.PostmarkInboundPayload{MessageID: "absence-1", OriginalRecipient: recipient, To: recipient, FromFull: model.PostmarkAddress{Email: "customer@example.com", Name: "Customer"}, Subject: "Automatic reply: Help", TextBody: "I am out of the office until September 21. I'll respond upon my return.", Headers: []model.PostmarkHeader{{Name: "Auto-Submitted", Value: "auto-generated"}}}
}

func TestEmailAbsencePreservesExistingConversation(t *testing.T) {
	for _, status := range []string{"open", "waiting_on_customer", "resolved"} {
		for _, channels := range []string{"chat", "both"} {
			t.Run(status+"/"+channels, func(t *testing.T) {
				ctx := context.Background()
				settings := noticeTestSettings()
				settings.AIReplyChannels = channels
				env := setupEmailFallbackInboundTestEnv(t, settings)
				recorder := &emailAIRequestRecorder{}
				env.service.supportInboxService.supportAIService = &SupportAIService{js: recorder}
				events := &captureSupportEventRecorder{}
				env.service.supportInboxService.SetSupportEventRecorder(events)
				now := time.Now().UTC().Add(-time.Hour)
				c := &model.SupportConversation{ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: "11111111-1111-1111-1111-111111111111", Status: status, Channel: "email", CustomerEmail: strPtr("customer@example.com"), AssignedAgentID: strPtr("agent"), AIState: strPtr("pending"), AIActiveRunID: strPtr("run"), FlowState: strPtr("ai_handling"), UpdatedAt: now}
				if status == "resolved" {
					c.FlowState = strPtr("resolved_by_ai")
					c.AIState = strPtr("resolved")
					c.ResolvedAt = &now
					c.ClosedAt = &now
				}
				if err := env.convRepo.Create(ctx, c); err != nil {
					t.Fatal(err)
				}
				payload := noticeTestPayload("conv-" + c.ID + "@replies.helpin.ai")
				for range 2 {
					if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); err != nil {
						t.Fatal(err)
					}
				}
				saved, err := env.convRepo.GetByID(ctx, c.WorkspaceID, c.ID, "", model.RoleOwner)
				if err != nil {
					t.Fatal(err)
				}
				if saved.Status != status || derefString(saved.FlowState) != derefString(c.FlowState) || derefString(saved.AssignedAgentID) != "agent" || derefString(saved.AIActiveRunID) != "run" || derefString(saved.AIState) != derefString(c.AIState) || (saved.HumanTakeover != nil && *saved.HumanTakeover) {
					t.Fatalf("notice changed ownership/status: %+v", saved)
				}
				if !saved.UpdatedAt.Equal(c.UpdatedAt) {
					t.Fatal("notice bumped inbox activity")
				}
				if len(recorder.events) != 0 || len(events.events) != 0 {
					t.Fatal("notice triggered AI or customer-reply events")
				}
				messages, err := env.messageRepo.ListByConversation(ctx, c.WorkspaceID, c.ID, true)
				if err != nil {
					t.Fatal(err)
				}
				if len(messages) != 1 || messages[0].MessageType != model.SupportMessageTypeEmailNotice || messages[0].Content != payload.TextBody {
					t.Fatalf("notice not retained exactly once: %+v", messages)
				}
			})
		}
	}
}

func TestEmailAbsenceNewConversationClosesWithoutResolutionCredit(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, noticeTestSettings())
	recorder := &emailAIRequestRecorder{}
	env.service.supportInboxService.supportAIService = &SupportAIService{js: recorder}
	events := &captureSupportEventRecorder{}
	env.service.supportInboxService.SetSupportEventRecorder(events)
	route := &model.SupportEmailRoute{ID: "a1111111-1111-1111-1111-111111111111", WorkspaceID: "11111111-1111-1111-1111-111111111111", RouteKey: "route-shared123", InboundAddress: "inbox@acme.on.helpin.email", ProviderType: "forwarding", Active: true, CreatedByID: "22222222-2222-2222-2222-222222222222"}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatal(err)
	}
	payload := noticeTestPayload(route.InboundAddress)
	for range 2 {
		if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); err != nil {
			t.Fatal(err)
		}
	}
	log, err := env.emailLogRepo.GetByPostmarkMessageID(ctx, payload.MessageID)
	if err != nil || log == nil {
		t.Fatalf("log: %v", err)
	}
	c, err := env.convRepo.GetByID(ctx, route.WorkspaceID, log.ConversationID, "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != "resolved" || c.ClosedAt == nil || c.ResolvedAt != nil || c.AIResolvedAt != nil || c.FlowState != nil || c.AssignedUserID != nil || c.AssignedAgentID != nil {
		t.Fatalf("wrong system closure: %+v", c)
	}
	messages, err := env.messageRepo.ListByConversation(ctx, c.WorkspaceID, c.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages=%d, want original and audit", len(messages))
	}
	var foundNotice, foundAudit bool
	for _, m := range messages {
		if m.MessageType == model.SupportMessageTypeEmailNotice {
			foundNotice = true
		}
		if derefString(m.SystemEventType) == model.SystemEventClosed {
			var metadata map[string]string
			if err := json.Unmarshal([]byte(m.Metadata), &metadata); err != nil {
				t.Fatal(err)
			}
			foundAudit = metadata["closure_actor"] == "system" && metadata["closure_reason"] == "out_of_office"
		}
	}
	if !foundNotice || !foundAudit || len(events.events) != 0 || len(recorder.events) != 0 {
		t.Fatal("missing notice/audit or unexpected automation")
	}
	// A subsequent real customer reply reopens this thread and can reach the AI.
	payload.MessageID = "human-2"
	payload.Headers = nil
	payload.OriginalRecipient = "conv-" + c.ID + "@replies.helpin.ai"
	payload.To = payload.OriginalRecipient
	payload.TextBody = "I am back. Can you help reconnect my account?"
	if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); err != nil {
		t.Fatal(err)
	}
	c, err = env.convRepo.GetByID(ctx, c.WorkspaceID, c.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != "open" || len(recorder.events) != 1 {
		t.Fatalf("real reply not resumed: status=%s dispatches=%d", c.Status, len(recorder.events))
	}
}

func TestEmailAbsenceDoesNotAttachToUnrelatedActiveIssue(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, noticeTestSettings())
	c := &model.SupportConversation{ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: "11111111-1111-1111-1111-111111111111", Subject: "Unrelated billing issue", Status: "open", Channel: "email", CustomerEmail: strPtr("customer@example.com")}
	if err := env.convRepo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	route := &model.SupportEmailRoute{ID: "a1111111-1111-1111-1111-111111111111", WorkspaceID: c.WorkspaceID, RouteKey: "route-shared123", InboundAddress: "inbox@acme.on.helpin.email", ProviderType: "forwarding", Active: true, CreatedByID: "22222222-2222-2222-2222-222222222222"}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatal(err)
	}
	payload := noticeTestPayload(route.InboundAddress)
	if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); err != nil {
		t.Fatal(err)
	}
	log, err := env.emailLogRepo.GetByPostmarkMessageID(ctx, payload.MessageID)
	if err != nil || log == nil {
		t.Fatalf("log: %v", err)
	}
	if log.ConversationID == c.ID {
		t.Fatal("notice attached to unrelated issue by sender alone")
	}
	saved, err := env.convRepo.GetByID(ctx, c.WorkspaceID, c.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "open" {
		t.Fatal("notice closed unrelated issue")
	}
}

func TestEmailAbsenceUncertainMessagesStayActionable(t *testing.T) {
	for _, tt := range []struct {
		name, body, from string
		headers          []model.PostmarkHeader
	}{
		{name: "alert", body: "Production service failed", from: "alerts@example.com", headers: []model.PostmarkHeader{{Name: "Auto-Submitted", Value: "auto-generated"}}},
		{name: "delivery failure", body: "Your reply could not be delivered", from: "mailer-daemon@example.com", headers: []model.PostmarkHeader{{Name: "Content-Type", Value: "multipart/report; report-type=delivery-status"}}},
		{name: "forwarding confirmation", body: "Confirm forwarding at https://mail-settings.google.com/mail/vf-test", from: "forwarding-noreply@google.com"},
		{name: "manual customer", body: "I am currently out of the office. Please help reset my password.", from: "customer@example.com"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			env := setupEmailFallbackInboundTestEnv(t, noticeTestSettings())
			route := &model.SupportEmailRoute{ID: "a1111111-1111-1111-1111-111111111111", WorkspaceID: "11111111-1111-1111-1111-111111111111", RouteKey: "route-shared123", InboundAddress: "inbox@acme.on.helpin.email", ProviderType: "forwarding", Active: true, CreatedByID: "22222222-2222-2222-2222-222222222222"}
			if err := env.routeRepo.Create(ctx, route); err != nil {
				t.Fatal(err)
			}
			payload := noticeTestPayload(route.InboundAddress)
			payload.Subject = "Incoming email"
			payload.TextBody = tt.body
			payload.FromFull.Email = tt.from
			payload.Headers = tt.headers
			if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); err != nil {
				t.Fatal(err)
			}
			log, err := env.emailLogRepo.GetByPostmarkMessageID(ctx, payload.MessageID)
			if err != nil || log == nil {
				t.Fatalf("log: %v", err)
			}
			c, err := env.convRepo.GetByID(ctx, route.WorkspaceID, log.ConversationID, "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			if c.Status != "open" {
				t.Fatalf("uncertain or actionable mail closed: %s", c.Status)
			}
		})
	}
}

func TestEmailAbsenceFromTeammateDoesNotTakeOver(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, noticeTestSettings())
	ws, owner := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	if _, err := env.service.workspaceRepo.AddMember(ctx, ws, owner, model.RoleOwner); err != nil {
		t.Fatal(err)
	}
	c := &model.SupportConversation{ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: ws, Status: "resolved", CustomerEmail: strPtr("customer@example.com"), FlowState: strPtr("resolved_by_ai"), AIState: strPtr("resolved"), AssignedAgentID: strPtr("agent")}
	if err := env.convRepo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	payload := noticeTestPayload("conv-" + c.ID + "@replies.helpin.ai")
	payload.FromFull = model.PostmarkAddress{Email: "owner@example.com", Name: "Owner"}
	payload.ToFull = []model.PostmarkAddress{{Email: "customer@example.com"}}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); err != nil {
		t.Fatal(err)
	}
	saved, err := env.convRepo.GetByID(ctx, ws, c.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "resolved" || saved.OpenedByUserID != nil || (saved.HumanTakeover != nil && *saved.HumanTakeover) || derefString(saved.AssignedAgentID) != "agent" {
		t.Fatalf("teammate notice took over: %+v", saved)
	}
	log, err := env.emailLogRepo.GetByPostmarkMessageID(ctx, payload.MessageID)
	if err != nil || log == nil {
		t.Fatalf("log: %v", err)
	}
	if log.Direction != "inbound" {
		t.Fatalf("notice recorded as an outgoing human reply: %s", log.Direction)
	}
}
