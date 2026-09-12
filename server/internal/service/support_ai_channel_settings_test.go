package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportAIChannelSettingsDefaultsPatchAndValidation(t *testing.T) {
	for _, raw := range []string{"", `{}`, `{"ai_enabled":true}`} {
		if got := parseSettings(raw).AIReplyChannels; got != "chat" {
			t.Fatalf("default=%q", got)
		}
	}
	for _, choice := range []string{"chat", "email", "both", "invalid"} {
		settings := model.DefaultSupportInboxSettings()
		patched := mergeSettingsUpdate(settings, model.UpdateInstallationSettingsRequest{AIReplyChannels: &choice})
		if patched.AIReplyChannels != choice {
			t.Fatal("choice lost during patch")
		}
		err := (&SupportInboxService{}).validateSettings(context.Background(), "ws", patched)
		if (err != nil) != (choice == "invalid") {
			t.Fatalf("%s validation=%v", choice, err)
		}
	}
}

func TestInboundEmailRespectsAIChannelChoice(t *testing.T) {
	for _, selection := range []string{"chat", "email", "both"} {
		for _, automatic := range []bool{false, true} {
			t.Run(selection+map[bool]string{true: " automatic", false: " customer"}[automatic], func(t *testing.T) {
				ctx := context.Background()
				settings := model.DefaultSupportInboxSettings()
				settings.AIEnabled = true
				settings.AIAgentID = strPtr("agent")
				settings.AIResponseMode = "ai_first"
				settings.AIReplyChannels = selection
				env := setupEmailFallbackInboundTestEnv(t, settings)
				recorder := &emailAIRequestRecorder{}
				env.service.supportInboxService.supportAIService = &SupportAIService{js: recorder}
				conv := &model.SupportConversation{ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: "11111111-1111-1111-1111-111111111111", Status: "open", Channel: "email", Source: "email", CustomerEmail: strPtr("customer@example.com")}
				if err := env.convRepo.Create(ctx, conv); err != nil {
					t.Fatal(err)
				}
				payload := model.PostmarkInboundPayload{MessageID: "source", OriginalRecipient: "conv-" + conv.ID + "@replies.helpin.ai", To: "conv-" + conv.ID + "@replies.helpin.ai", From: "customer@example.com", FromFull: model.PostmarkAddress{Email: "customer@example.com"}, StrippedTextReply: "Please help me connect my account"}
				if automatic {
					payload.Headers = []model.PostmarkHeader{{Name: "Auto-Submitted", Value: "auto-replied"}}
				}
				for range 2 {
					if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); err != nil {
						t.Fatal(err)
					}
				}
				want := 0
				if selection != "chat" && !automatic {
					want = 2
				}
				if len(recorder.events) != want {
					t.Fatalf("dispatches=%d, want %d", len(recorder.events), want)
				}
			})
		}
	}
}

func TestNewEmailConversationDispatchRetriesWhenEmailAIEnabled(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIAgentID = strPtr("agent")
	settings.AIResponseMode = "ai_first"
	settings.AIReplyChannels = "email"
	env := setupEmailFallbackInboundTestEnv(t, settings)
	recorder := &emailAIRequestRecorder{failNext: true}
	env.service.supportInboxService.supportAIService = &SupportAIService{js: recorder}
	route := &model.SupportEmailRoute{ID: "a1111111-1111-1111-1111-111111111111", WorkspaceID: "11111111-1111-1111-1111-111111111111", RouteKey: "route-shared123", InboundAddress: "inbox@acme.on.helpin.email", ProviderType: "forwarding", Active: true, CreatedByID: "22222222-2222-2222-2222-222222222222"}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatal(err)
	}
	payload := model.PostmarkInboundPayload{MessageID: "new-email", OriginalRecipient: route.InboundAddress, To: route.InboundAddress, FromFull: model.PostmarkAddress{Email: "customer@example.com", Name: "Customer"}, Subject: "Account help", StrippedTextReply: "Please help connect my account"}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); !errors.Is(err, ErrInboundEmailAIDispatchRetry) {
		t.Fatalf("first dispatch=%v", err)
	}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); err != nil {
		t.Fatal(err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("recovered dispatches=%d", len(recorder.events))
	}
	var count int64
	env.convRepo.DB().Model(&model.SupportConversation{}).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate conversations=%d", count)
	}
}

func TestEmailOnlyWidgetAutomationLeavesHumanOwnership(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIAgentID = strPtr("agent")
	settings.AIReplyChannels = "email"
	env := setupEmailFallbackInboundTestEnv(t, settings)
	recorder := &emailAIRequestRecorder{}
	env.service.supportInboxService.supportAIService = &SupportAIService{js: recorder}
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "11111111-1111-1111-1111-111111111111", Channel: "widget", Status: "open", AssignedAgentID: strPtr("agent"), AIState: strPtr("pending"), FlowState: strPtr(model.SupportConversationFlowStateAIHandling)}
	if err := env.convRepo.Create(context.Background(), conv); err != nil {
		t.Fatal(err)
	}
	env.service.supportInboxService.runWidgetPostMessageAutomation(context.Background(), conv.WorkspaceID, conv.ID, "message", "Please help")
	if len(recorder.events) != 0 {
		t.Fatal("chat dispatched AI with email-only selection")
	}
	saved, err := env.convRepo.GetByID(context.Background(), conv.WorkspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if saved.AssignedAgentID != nil || derefString(saved.FlowState) == model.SupportConversationFlowStateAIHandling {
		t.Fatal("chat retained AI ownership")
	}
	inst, err := env.installRepo.GetByWorkspace(context.Background(), conv.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	config, err := env.service.supportInboxService.buildWidgetConfigResponse(context.Background(), inst)
	if err != nil {
		t.Fatal(err)
	}
	if config.Features.AIEnabled || config.Features.AIFirst {
		t.Fatal("widget still advertises AI")
	}
}

func TestSupportFollowUpUsesSelectedChannels(t *testing.T) {
	for _, selection := range []string{"chat", "email", "both"} {
		for _, channel := range []string{"widget", "email", "email_continuation"} {
			t.Run(selection+" "+channel, func(t *testing.T) {
				svc, db, run, e, d := setupFollowUpTest(t)
				var inst model.SupportWidgetInstallation
				db.First(&inst, "id = ?", "inst")
				settings := parseSettings(inst.Settings)
				settings.AIReplyChannels = selection
				raw, err := json.Marshal(settings)
				if err != nil {
					t.Fatal(err)
				}
				mustExec(t, db, `UPDATE support_widget_installations SET settings=?`, string(raw))
				effectiveChannel := channel
				if channel == "email_continuation" {
					mustExec(t, db, `UPDATE support_conversations SET channel='widget'`)
					mustExec(t, db, `UPDATE support_messages SET via_channel='email' WHERE id=?`, e.SourceMessageID)
					effectiveChannel = "email"
				} else {
					mustExec(t, db, `UPDATE support_conversations SET channel=?`, channel)
				}
				status, err := svc.Complete(context.Background(), run, e.ID, d)
				if err != nil {
					t.Fatal(err)
				}
				want := "suppressed"
				if model.SupportAIChannelEnabled(settings, effectiveChannel) {
					want = "waiting"
				}
				if status != want {
					t.Fatalf("status=%q, want %q", status, want)
				}
			})
		}
	}
}

func TestInboundEmailMachineSignalsSuppressAI(t *testing.T) {
	tests := []struct {
		name, header, value, from string
		blocked                   bool
	}{
		{"human", "", "", "person@example.com", false},
		{"explicit human", "Auto-Submitted", "no; owner-email=person@example.com", "person@example.com", false},
		{"autoresponder", "Auto-Submitted", "auto-replied", "person@example.com", true},
		{"delivery receipt", "Content-Type", "multipart/report; report-type=delivery-status", "person@example.com", true},
		{"read receipt", "Content-Type", "message/disposition-notification", "person@example.com", true},
		{"bounce", "Return-Path", "<>", "person@example.com", true},
		{"mailer daemon", "", "", "MAILER-DAEMON@example.com", true},
		{"list", "List-ID", "announcements.example.com", "person@example.com", true},
		{"bulk", "Precedence", "bulk", "person@example.com", true},
		{"spam", "X-Spam-Status", "Yes, score=7", "person@example.com", true},
		{"spam score", "X-Spam-Score", "7", "person@example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := model.PostmarkInboundPayload{From: tt.from, Headers: []model.PostmarkHeader{{Name: tt.header, Value: tt.value}}}
			raw := inboundEmailAIMetadata("null", payload, "Can you help me log in?")
			var metadata struct {
				AIRequest bool `json:"email_ai_request"`
			}
			if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
				t.Fatal(err)
			}
			if metadata.AIRequest == tt.blocked {
				t.Fatalf("request=%v, blocked=%v", metadata.AIRequest, tt.blocked)
			}
			msg := &model.SupportMessage{SenderType: "customer", Content: "Can you help me log in?", Metadata: raw, ViaChannel: strPtr("email")}
			if got := model.SupportAIReplyAllowed(model.SupportInboxSettings{AIReplyChannels: "both"}, nil, msg); got == tt.blocked {
				t.Fatalf("delayed work allowed=%v blocked=%v", got, tt.blocked)
			}
		})
	}
}
