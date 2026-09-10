package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/nats-io/nats.go"
)

type emailAIRequestRecorder struct {
	nats.JetStreamContext
	events   []AIRequestEvent
	failNext bool
}

func (r *emailAIRequestRecorder) Publish(subject string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error) {
	var event AIRequestEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	if r.failNext {
		r.failNext = false
		return nil, errors.New("temporary NATS failure")
	}
	r.events = append(r.events, event)
	return &nats.PubAck{}, nil
}

func TestEmailCustomerReplyResumesAI(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name, status, content string
		change                func(*model.SupportConversation, *model.SupportInboxSettings)
		want                  int
	}{
		{"confirmed resolution", "resolved", "Actually I still need help", nil, 1},
		{"assumed resolution", "resolved", "No, it still fails", func(c *model.SupportConversation, _ *model.SupportInboxSettings) {
			c.AIResolutionType = strPtr("assumed")
		}, 1},
		{"waiting followup", "waiting_on_customer", "Still broken", nil, 1},
		{"takeover", "resolved", "Help", func(c *model.SupportConversation, _ *model.SupportInboxSettings) { c.HumanTakeover = boolPtr(true) }, 0},
		{"human request", "resolved", "Help", func(c *model.SupportConversation, _ *model.SupportInboxSettings) { c.CustomerRequestedHumanAt = &now }, 0},
		{"escalated", "resolved", "Help", func(c *model.SupportConversation, _ *model.SupportInboxSettings) { c.AIState = strPtr("escalated") }, 0},
		{"assigned human", "resolved", "Help", func(c *model.SupportConversation, _ *model.SupportInboxSettings) {
			c.AssignedUserID = strPtr("22222222-2222-2222-2222-222222222222")
		}, 0},
		{"human flow", "resolved", "Help", func(c *model.SupportConversation, _ *model.SupportInboxSettings) {
			c.FlowState = strPtr(model.SupportConversationFlowStateAssignedToHuman)
		}, 0},
		{"disabled", "resolved", "Help", func(_ *model.SupportConversation, s *model.SupportInboxSettings) { s.AIEnabled = false }, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := model.DefaultSupportInboxSettings()
			settings.AIEnabled = true
			settings.AIAgentID = strPtr("ai-agent")
			settings.AIResponseMode = "ai_first"
			conv := &model.SupportConversation{ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: "11111111-1111-1111-1111-111111111111", Status: tt.status, CustomerEmail: strPtr("customer@example.com"), AssignedAgentID: strPtr("ai-agent"), AIState: strPtr("pending"), FlowState: strPtr(model.SupportConversationFlowStateResolvedByAI), AIResolvedAt: &now, AIResolutionType: strPtr("confirmed"), Source: "email", MailboxID: strPtr("mailbox-kept")}
			if tt.change != nil {
				tt.change(conv, &settings)
			}
			env := setupEmailFallbackInboundTestEnv(t, settings)
			recorder := &emailAIRequestRecorder{}
			env.service.supportInboxService.supportAIService = &SupportAIService{js: recorder}
			if err := env.convRepo.Create(context.Background(), conv); err != nil {
				t.Fatal(err)
			}
			payload := model.PostmarkInboundPayload{MessageID: "pm-resume", OriginalRecipient: "conv-" + conv.ID + "@replies.helpin.ai", To: "conv-" + conv.ID + "@replies.helpin.ai", From: "customer@example.com", FromFull: model.PostmarkAddress{Email: "customer@example.com"}, Subject: "Re: Help", StrippedTextReply: tt.content}
			for i := 0; i < 2; i++ {
				if err := env.service.ProcessInboundEmail(context.Background(), payload, `{}`); err != nil {
					t.Fatal(err)
				}
			}
			if len(recorder.events) != tt.want*2 {
				t.Fatalf("AI publish attempts = %d, want %d", len(recorder.events), tt.want*2)
			}
			saved, err := env.convRepo.GetByID(context.Background(), conv.WorkspaceID, conv.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			if saved.Status != "open" || derefString(saved.MailboxID) != "mailbox-kept" || saved.AIResolvedAt != nil || saved.AIResolutionType != nil {
				t.Fatalf("unexpected reopened state: %+v", saved)
			}
			if tt.want == 1 {
				if derefString(saved.FlowState) != model.SupportConversationFlowStateAIHandling {
					t.Fatalf("flow state = %s", derefString(saved.FlowState))
				}
				if recorder.events[0] != recorder.events[1] {
					t.Fatal("retry changed the AI source message")
				}
				event := recorder.events[0]
				msg, err := env.messageRepo.GetByID(context.Background(), event.MessageID)
				if err != nil {
					t.Fatal(err)
				}
				if event.Content != tt.content || msg.SenderType != "customer" || event.ConversationID != conv.ID {
					t.Fatalf("unexpected request: %+v", event)
				}
			}
		})
	}
}

func TestFollowUpEmailUsesActualWorkspaceDisplayName(t *testing.T) {
	for _, tt := range []struct{ name, metadata, want string }{
		{"first followup", `{"ai_reply_kind":"inactivity_follow_up"}`, "Acme Support"},
		{"second followup", `{"ai_reply_kind":"inactivity_follow_up","follow_up_number":2}`, "Acme Support"},
		{"ordinary reply", `{}`, "Helpin AI - Configured Brand"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			settings := model.DefaultSupportInboxSettings()
			settings.EmailFallbackEnabled = true
			settings.EmailFallbackFromName = "Configured Brand"
			env := setupEmailFallbackInboundTestEnv(t, settings)
			conv := &model.SupportConversation{ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: "11111111-1111-1111-1111-111111111111", Status: "open", CustomerEmail: strPtr("customer@example.com")}
			if err := env.convRepo.Create(context.Background(), conv); err != nil {
				t.Fatal(err)
			}
			msg := &model.SupportMessage{WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "ai", SenderDisplayName: strPtr("Helpin AI"), MessageType: "reply", Content: "Do you still need help?", Metadata: tt.metadata}
			if err := env.messageRepo.Create(context.Background(), msg); err != nil {
				t.Fatal(err)
			}
			var captured capturedPostmarkRequest
			attempts := 0
			env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if err := json.NewDecoder(req.Body).Decode(&captured); err != nil {
					t.Fatal(err)
				}
				attempts++
				if attempts == 1 {
					if captured.From != tt.want+" <inbox@acme.on.helpin.email>" {
						t.Fatalf("branded From = %q", captured.From)
					}
					return &http.Response{StatusCode: 422, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ErrorCode":300,"Message":"The From address is not a Sender Signature on your account."}`))}, nil
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ErrorCode":0,"MessageID":"sent"}`))}, nil
			})})
			if err := env.service.fireEmailWithOptions(context.Background(), conv.ID, []string{msg.ID}, emailFallbackFireOptions{}); err != nil {
				t.Fatal(err)
			}
			if captured.From != tt.want+" <noreply@example.com>" {
				t.Fatalf("From = %q", captured.From)
			}
			if !strings.Contains(captured.ReplyTo, "conv-"+conv.ID+"@replies.helpin.ai") {
				t.Fatalf("Reply-To = %q", captured.ReplyTo)
			}
		})
	}
}

func TestWidgetPostMessageAutomationRespectsHumanOwnership(t *testing.T) {
	now := time.Now()
	for _, tt := range []struct {
		name   string
		change func(*model.SupportConversation)
		want   int
	}{
		{"AI owned", nil, 1},
		{"human requested", func(c *model.SupportConversation) { c.CustomerRequestedHumanAt = &now }, 0},
		{"escalated", func(c *model.SupportConversation) { c.AIState = strPtr("escalated") }, 0},
		{"assigned human", func(c *model.SupportConversation) { c.AssignedUserID = strPtr("22222222-2222-2222-2222-222222222222") }, 0},
		{"human takeover", func(c *model.SupportConversation) { c.HumanTakeover = boolPtr(true) }, 0},
		{"human flow", func(c *model.SupportConversation) {
			c.FlowState = strPtr(model.SupportConversationFlowStateAssignedToHuman)
		}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			settings := model.DefaultSupportInboxSettings()
			settings.AIEnabled = true
			settings.AIAgentID = strPtr("ai-agent")
			settings.AIResponseMode = "ai_first"
			env := setupEmailFallbackInboundTestEnv(t, settings)
			conv := &model.SupportConversation{ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: "11111111-1111-1111-1111-111111111111", Status: "open", FlowState: strPtr(model.SupportConversationFlowStateAIHandling), AssignedAgentID: strPtr("ai-agent")}
			if tt.change != nil {
				tt.change(conv)
			}
			if err := env.convRepo.Create(context.Background(), conv); err != nil {
				t.Fatal(err)
			}
			recorder := &emailAIRequestRecorder{}
			env.service.supportInboxService.supportAIService = &SupportAIService{js: recorder}
			env.service.supportInboxService.runWidgetPostMessageAutomation(context.Background(), conv.WorkspaceID, conv.ID, "source-message", "Still need help")
			if len(recorder.events) != tt.want {
				t.Fatalf("published = %d, want %d", len(recorder.events), tt.want)
			}
		})
	}
}

func TestEmailCustomerReplyRetriesFailedAIPublish(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIAgentID = strPtr("ai-agent")
	settings.AIResponseMode = "ai_first"
	env := setupEmailFallbackInboundTestEnv(t, settings)
	conv := &model.SupportConversation{ID: "33333333-3333-3333-3333-333333333333", WorkspaceID: "11111111-1111-1111-1111-111111111111", Status: "resolved", CustomerEmail: strPtr("customer@example.com"), AssignedAgentID: strPtr("ai-agent"), FlowState: strPtr(model.SupportConversationFlowStateResolvedByAI)}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	recorder := &emailAIRequestRecorder{failNext: true}
	env.service.supportInboxService.supportAIService = &SupportAIService{js: recorder}
	payload := model.PostmarkInboundPayload{MessageID: "retry-source", OriginalRecipient: "conv-" + conv.ID + "@replies.helpin.ai", To: "conv-" + conv.ID + "@replies.helpin.ai", From: "customer@example.com", FromFull: model.PostmarkAddress{Email: "customer@example.com"}, StrippedTextReply: "Still broken"}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); !errors.Is(err, ErrInboundEmailAIDispatchRetry) {
		t.Fatalf("expected retryable publish error, got %v", err)
	}
	logRow, err := env.emailLogRepo.GetByPostmarkMessageID(ctx, payload.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	if logRow == nil || len(logRow.MessageIDs) != 1 {
		t.Fatal("customer message was not durably saved")
	}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{}`); err != nil {
		t.Fatal(err)
	}
	if len(recorder.events) != 1 || recorder.events[0].MessageID != logRow.MessageIDs[0] || recorder.events[0].Content != "Still broken" {
		t.Fatalf("retry did not publish saved source: %+v", recorder.events)
	}
	messages, err := env.messageRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	customerCount := 0
	for _, msg := range messages {
		if msg.SenderType == "customer" {
			customerCount++
		}
	}
	if customerCount != 1 {
		t.Fatalf("customer messages = %d, want 1", customerCount)
	}
}
