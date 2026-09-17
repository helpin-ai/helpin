package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func greetingMessages() (*model.SupportConversation, *model.SupportMessage) {
	return &model.SupportConversation{ID: "conv", WorkspaceID: "ws", Channel: "widget", Status: "open", LastPublicMessageID: strPtr("source")},
		&model.SupportMessage{ID: "source", WorkspaceID: "ws", ConversationID: "conv", SenderType: "customer", MessageType: "reply", Content: "Hello!"}
}

func TestSupportCommonMessageRouting(t *testing.T) {
	for _, tc := range []struct{ text, route string }{
		{"Hi", supportGreetingOperation}, {"  HELLO!!! ", supportGreetingOperation},
		{"good morning", supportGreetingOperation}, {"bonjour", supportGreetingOperation},
		{"مرحبا", supportGreetingOperation}, {"你好", supportGreetingOperation},
		{"Hi, my payment failed", ""}, {"hello\nI need a refund", ""},
		{"hello human please", ""}, {"hi https://example.com", ""},
		{"thanks", ""}, {"yes", ""}, {"done", ""}, {"cancel", ""},
		{"any update?", ""}, {"still not working", ""}, {"", ""},
	} {
		t.Run(tc.text, func(t *testing.T) {
			conv, source := greetingMessages()
			source.Content = tc.text
			if got := classifySupportCommonMessage(conv, source, []model.SupportMessage{*source}); got != tc.route {
				t.Fatalf("route = %q, want %q", got, tc.route)
			}
		})
	}
}

func TestSupportCommonMessageKeepsConversationContext(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*model.SupportConversation, *model.SupportMessage, *[]model.SupportMessage)
	}{
		{"earlier customer issue", func(_ *model.SupportConversation, _ *model.SupportMessage, h *[]model.SupportMessage) {
			*h = append(*h, model.SupportMessage{ID: "earlier", MessageType: "reply", SenderType: "customer", Content: "Payment failed"})
		}},
		{"earlier AI reply", func(c *model.SupportConversation, _ *model.SupportMessage, _ *[]model.SupportMessage) {
			c.AITurnCount = 1
		}},
		{"existing run", func(c *model.SupportConversation, _ *model.SupportMessage, _ *[]model.SupportMessage) {
			c.AIActiveRunID = strPtr("run")
		}},
		{"resumed conversation", func(c *model.SupportConversation, _ *model.SupportMessage, _ *[]model.SupportMessage) {
			c.AIControlVersion = 2
		}},
		{"new message", func(c *model.SupportConversation, _ *model.SupportMessage, _ *[]model.SupportMessage) {
			c.LastPublicMessageID = strPtr("new")
		}},
		{"attachment", func(_ *model.SupportConversation, m *model.SupportMessage, _ *[]model.SupportMessage) {
			m.Attachments = []model.SupportAttachmentPayload{{FileName: "error.png"}}
		}},
		{"human owner", func(c *model.SupportConversation, _ *model.SupportMessage, _ *[]model.SupportMessage) {
			c.AssignedUserID = strPtr("human")
		}},
		{"linked work", func(c *model.SupportConversation, _ *model.SupportMessage, _ *[]model.SupportMessage) {
			c.LinkedTaskID = strPtr("task")
		}},
		{"email", func(c *model.SupportConversation, _ *model.SupportMessage, _ *[]model.SupportMessage) {
			c.Channel = "email"
		}},
		{"missing history", func(_ *model.SupportConversation, _ *model.SupportMessage, h *[]model.SupportMessage) { *h = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conv, source := greetingMessages()
			history := []model.SupportMessage{*source}
			tc.change(conv, source, &history)
			if got := classifySupportCommonMessage(conv, source, history); got != "" {
				t.Fatalf("unsafe shortcut = %q", got)
			}
		})
	}
}

type greetingProvider struct {
	response *llm.ChatResponse
	err      error
	during   func()
	requests []llm.ChatRequest
	deadline time.Duration
}

func (p *greetingProvider) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.requests = append(p.requests, req)
	deadline, _ := ctx.Deadline()
	p.deadline = time.Until(deadline)
	if p.during != nil {
		p.during()
	}
	if p.err != nil {
		return nil, p.err
	}
	if p.response != nil {
		return p.response, nil
	}
	return &llm.ChatResponse{Content: `{"content":"Hello! How can I help?"}`, Provider: req.Provider, Model: req.Model, FinishReason: "stop", TokensUsed: llm.TokenUsage{InputTokens: 40, OutputTokens: 12}}, nil
}

func setupSupportGreetingTest(t *testing.T) (*SupportChatService, *gorm.DB, *model.SupportConversation, *model.SupportMessage, *model.AIMessageProcessing, model.SupportInboxSettings, *greetingProvider) {
	t.Helper()
	db := newTestDB(t)
	mustExec(t, db, `CREATE TABLE ai_message_processing (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT, conversation_id TEXT, source_message_id TEXT UNIQUE, reply_message_id TEXT, status TEXT, attempts INTEGER, tokens_used INTEGER, created_at DATETIME, updated_at DATETIME)`)
	mustExec(t, db, `CREATE TABLE support_attachments (id TEXT PRIMARY KEY, workspace_id TEXT, message_id TEXT)`)
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIAgentID = strPtr("agent")
	settings.AIResponseMode = "ai_first"
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO support_widget_installations(id,workspace_id,widget_key,secret_key,settings,active) VALUES ('inst','ws','key','secret',?,true)`, string(raw))
	conv, source := greetingMessages()
	convRepo := repository.NewSupportConversationRepository(db)
	msgRepo := repository.NewSupportMessageRepository(db)
	if err := convRepo.Create(context.Background(), conv); err != nil {
		t.Fatal(err)
	}
	if err := msgRepo.Create(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	processingRepo := repository.NewAIMessageProcessingRepository(db)
	processing, ok := processingRepo.BeginAttempt(context.Background(), "ws", "source", "conv")
	if !ok {
		t.Fatal("cannot claim greeting")
	}
	provider := &greetingProvider{}
	svc := &SupportChatService{conversationRepo: convRepo, messageRepo: msgRepo, processingRepo: processingRepo, supportAIService: &SupportAIService{llmProvider: provider}}
	return svc, db, conv, source, processing, settings, provider
}

func TestSupportGreetingUsesLunaWithoutStartingAgent(t *testing.T) {
	svc, db, conv, source, processing, settings, provider := setupSupportGreetingTest(t)
	handled, err := svc.replyToInitialGreeting(context.Background(), conv, source, &model.Agent{ID: "agent"}, processing, settings)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("calls = %d", len(provider.requests))
	}
	req := provider.requests[0]
	if req.Model != "openai/gpt-5.6-luna" || req.Provider != "openrouter" || req.Reasoning.Effort != "low" || req.MaxTokens != 256 {
		t.Fatalf("unexpected route: %+v", req)
	}
	if len(req.Messages) != 1 || req.Messages[0].Content != source.Content {
		t.Fatalf("greeting sent extra context: %+v", req.Messages)
	}
	if provider.deadline <= 0 || provider.deadline > 5*time.Second {
		t.Fatalf("deadline=%v", provider.deadline)
	}
	var reply model.SupportMessage
	if err := db.Where("sender_type = 'ai'").First(&reply).Error; err != nil {
		t.Fatal(err)
	}
	var metadata AIMessageMetadata
	if err := json.Unmarshal([]byte(reply.Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if reply.Content != "Hello! How can I help?" || metadata.AIPreRoute != supportGreetingOperation || metadata.AIReplyKind != supportReplyKindGreeting {
		t.Fatalf("reply=%+v metadata=%+v", reply, metadata)
	}
	var row model.AIMessageProcessing
	if err := db.First(&row, "id = ?", processing.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != "completed" || derefString(row.ReplyMessageID) != reply.ID {
		t.Fatalf("unsettled turn: %+v", row)
	}
	if _, again := svc.processingRepo.BeginAttempt(context.Background(), "ws", "source", "conv"); again {
		t.Fatal("duplicate delivery reclaimed settled greeting")
	}
	current := readControlConversation(t, db)
	if current.AITurnCount != 1 || current.AIActiveRunID != nil {
		t.Fatalf("conversation=%+v", current)
	}
}

func TestSupportGreetingSuppressesChangedConversation(t *testing.T) {
	for _, tc := range []struct{ name, sql string }{
		{"new customer message", `UPDATE support_conversations SET last_public_message_id='new-message'`},
		{"human takeover", `UPDATE support_conversations SET human_takeover=true`},
		{"human assignment", `UPDATE support_conversations SET assigned_user_id='human'`},
		{"pause and return", `UPDATE support_conversations SET ai_control_version=2`},
		{"agent started", `UPDATE support_conversations SET ai_active_run_id='run'`},
		{"source edited", `UPDATE support_messages SET content='Hello, I need a refund' WHERE id='source'`},
		{"source deleted", `UPDATE support_messages SET deleted_at=CURRENT_TIMESTAMP WHERE id='source'`},
		{"media attached", `INSERT INTO support_attachments(id,workspace_id,message_id) VALUES ('file','ws','source')`},
		{"settings disabled", `UPDATE support_widget_installations SET settings='{}'`},
		{"installation disabled", `UPDATE support_widget_installations SET active=false`},
		{"changed to private mode", `UPDATE support_widget_installations SET settings='{"ai_enabled":true,"ai_agent_id":"agent","ai_response_mode":"internal_note"}'`},
		{"earlier public reply", `INSERT INTO support_messages(id,workspace_id,conversation_id,sender_type,message_type,content,is_internal) VALUES ('earlier','ws','conv','customer','reply','Where is my order?',false)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, conv, source, processing, settings, provider := setupSupportGreetingTest(t)
			provider.during = func() { mustExec(t, db, tc.sql) }
			handled, err := svc.replyToInitialGreeting(context.Background(), conv, source, &model.Agent{ID: "agent"}, processing, settings)
			if err != nil || !handled {
				t.Fatalf("handled=%v err=%v", handled, err)
			}
			var count int64
			if err := db.Model(&model.SupportMessage{}).Where("sender_type = 'ai'").Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal("stale greeting was published")
			}
		})
	}
}

func TestSupportGreetingFallsBackOnProviderFailure(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *llm.ChatResponse
		err      error
	}{
		{"unavailable", nil, errors.New("provider unavailable")},
		{"deadline", nil, context.DeadlineExceeded},
		{"empty", &llm.ChatResponse{Content: `{"content":" "}`}, nil},
		{"invalid JSON", &llm.ChatResponse{Content: `hello`}, nil},
		{"truncated", &llm.ChatResponse{Content: `{"content":"Hi!"}`, FinishReason: "length"}, nil},
		{"long", &llm.ChatResponse{Content: `{"content":"` + strings.Repeat("a", 401) + `"}`}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, conv, source, processing, settings, provider := setupSupportGreetingTest(t)
			provider.response, provider.err = tc.response, tc.err
			handled, err := svc.replyToInitialGreeting(context.Background(), conv, source, &model.Agent{ID: "agent"}, processing, settings)
			if err != nil || handled {
				t.Fatalf("handled=%v err=%v, expected agent fallback", handled, err)
			}
			var count int64
			if err := db.Model(&model.SupportMessage{}).Where("sender_type = 'ai'").Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal("invalid greeting published")
			}
		})
	}
}

func TestSupportGreetingHandleVisitorMessageSkipsRuntimeAndJev(t *testing.T) {
	svc, db, conv, source, _, _, provider := setupSupportGreetingTest(t)
	mustExec(t, db, `DELETE FROM ai_message_processing`)
	mustExec(t, db, `CREATE TABLE agents(id TEXT PRIMARY KEY, workspace_id TEXT, is_system BOOLEAN, name TEXT, runtime_kind TEXT, skills BLOB)`)
	mustExec(t, db, `INSERT INTO agents(id,workspace_id,is_system,name,runtime_kind,skills) VALUES ('agent','ws',true,'Echo','native_sdk','[]')`)
	svc.agentService = &AgentService{agentRepo: repository.NewAgentRepository(db)}
	svc.supportAIService.installationRepo = repository.NewSupportInboxInstallationRepository(db)
	// No runtime client is configured; a successful turn must bypass it.
	jev := &lifecycleJev{err: errors.New("greeting must not call Jev")}
	svc.jev = &SupportJevService{provider: jev}
	for range 2 {
		if err := svc.HandleVisitorMessage(context.Background(), conv.WorkspaceID, conv.ID, source); err != nil {
			t.Fatal(err)
		}
	}
	if len(provider.requests) != 1 || jev.calls != 0 {
		t.Fatalf("Luna calls=%d Jev calls=%d", len(provider.requests), jev.calls)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("sender_type='ai'").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("AI replies=%d", count)
	}
}

func TestSupportGreetingRespectsInternalNoteMode(t *testing.T) {
	svc, db, conv, source, processing, settings, _ := setupSupportGreetingTest(t)
	settings.AIResponseMode = "internal_note"
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE support_widget_installations SET settings=?`, string(raw))
	handled, err := svc.replyToInitialGreeting(context.Background(), conv, source, &model.Agent{ID: "agent"}, processing, settings)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	var reply model.SupportMessage
	if err := db.Where("sender_type='ai'").First(&reply).Error; err != nil {
		t.Fatal(err)
	}
	if !reply.IsInternal || reply.WidgetVisible() {
		t.Fatal("private greeting became public")
	}
}
