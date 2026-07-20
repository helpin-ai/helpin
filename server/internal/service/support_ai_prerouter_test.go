package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Unit tests: hardened confirmation detection
// ---------------------------------------------------------------------------

func TestIsConfirmationMessageHardened(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		// Genuine confirmations.
		{name: "thanks", content: "Thanks!", want: true},
		{name: "that helped", content: "Oh that helped a lot", want: true},
		{name: "perfect", content: "perfect", want: true},
		{name: "that worked", content: "that worked!", want: true},
		{name: "all good", content: "all good", want: true},
		{name: "makes sense", content: "ok makes sense", want: true},
		{name: "great thanks", content: "Great, thanks", want: true},

		// Regression: negative feedback must never resolve the conversation.
		// The old substring matcher classified these as confirmations.
		{name: "not helpful", content: "not helpful at all", want: false},
		{name: "thanks but broken", content: "thanks but it still doesn't work", want: false},
		{name: "didnt help", content: "that didn't help", want: false},
		{name: "still broken", content: "still broken", want: false},
		{name: "great but question", content: "great, but how do I export?", want: false},
		{name: "no thats wrong", content: "no that's wrong", want: false},
		{name: "greatly disappointed", content: "greatly disappointed", want: false},

		// Guards.
		{name: "question", content: "How do I reset?", want: false},
		{name: "empty", content: "", want: false},
		{name: "long message with thanks", content: "thanks for the reply but I also wanted to ask about how the export works for multiple workspaces at once", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConfirmationMessage(tt.content); got != tt.want {
				t.Errorf("isConfirmationMessage(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestCountSubstantiveAITurns(t *testing.T) {
	agentID := "agent-1"
	mkAI := func(kind string) model.SupportMessage {
		meta, _ := json.Marshal(AIMessageMetadata{AIAutoReply: true, AIAgentID: agentID, AIReplyKind: kind, AIConfidence: 0.8})
		return model.SupportMessage{SenderType: "ai", SenderAgentID: &agentID, Content: "x", Metadata: string(meta)}
	}
	history := []model.SupportMessage{
		{SenderType: "customer", Content: "Hi"},
		mkAI(supportReplyKindGreeting),
		{SenderType: "customer", Content: "how do I export?"},
		mkAI(supportReplyKindAnswer),
	}
	if got := countSubstantiveAITurns(history, agentID); got != 1 {
		t.Fatalf("countSubstantiveAITurns = %d, want 1 (greeting turn must not count)", got)
	}
	if got := countAgentAITurns(history, agentID); got != 2 {
		t.Fatalf("countAgentAITurns = %d, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// End-to-end: HandleIncomingMessage through the pre-router (sqlite)
// ---------------------------------------------------------------------------

// recordingLLMStub records every chat call. Every customer message must invoke
// the LLM pre-router; short-circuited routes skip only the generation call.
type recordingLLMStub struct {
	mu       sync.Mutex
	calls    []llm.ChatRequest
	response string
}

func (s *recordingLLMStub) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, req)
	if s.response == "" {
		return nil, fmt.Errorf("stub has no response configured")
	}
	return &llm.ChatResponse{Content: s.response}, nil
}

func (s *recordingLLMStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

type preRouterTestEnv struct {
	db      *gorm.DB
	svc     *SupportAIService
	stub    *recordingLLMStub
	msgRepo *repository.SupportMessageRepository
	convo   *model.SupportConversation
	wsID    string
	agentID string
}

func setupPreRouterEnv(t *testing.T, responseMode string) *preRouterTestEnv {
	t.Helper()
	db := newTestDB(t)
	ctx := context.Background()

	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT 'Agent',
			provider TEXT,
			model TEXT,
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS ai_message_processing (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			source_message_id TEXT NOT NULL UNIQUE,
			conversation_id TEXT NOT NULL,
			reply_message_id TEXT,
			status TEXT NOT NULL DEFAULT 'processing',
			attempts INTEGER NOT NULL DEFAULT 1,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	wsID := "ws-prerouter"
	userID := "user-prerouter"
	agentID := "agent-prerouter"
	seedUser(t, db, userID, "owner@prerouter.test", "Owner", "hash")
	seedWorkspace(t, db, wsID, "PreRouter WS", "prerouter-ws", userID)
	if err := db.Exec(`INSERT INTO agents (id, workspace_id, name) VALUES (?, ?, 'Echo')`, agentID, wsID).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	settings := fmt.Sprintf(`{"ai_enabled": true, "ai_agent_id": %q, "ai_response_mode": %q, "ai_confidence_threshold": 0.7, "welcome_message": "Hi there! How can we help you today?"}`, agentID, responseMode)
	if err := db.Exec(`INSERT INTO support_widget_installations (id, workspace_id, widget_key, secret_key, settings, active) VALUES ('inst-1', ?, 'wk', 'sk', ?, 1)`, wsID, settings).Error; err != nil {
		t.Fatalf("seed installation: %v", err)
	}

	convRepo := repository.NewSupportConversationRepository(db)
	msgRepo := repository.NewSupportMessageRepository(db)
	convo := &model.SupportConversation{
		WorkspaceID: wsID,
		Subject:     "Widget conversation",
		Status:      "open",
		Channel:     "widget",
	}
	if err := convRepo.Create(ctx, convo); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	stub := &recordingLLMStub{}
	svc := NewSupportAIService(
		stub,
		nil, "",
		repository.NewDocsChunkRepository(db),
		repository.NewAgentKnowledgeSourceRepository(db),
		repository.NewSupportContentChunkRepository(db),
		repository.NewAgentContentSourceRepository(db),
		repository.NewAIMessageProcessingRepository(db),
		convRepo,
		msgRepo,
		nil,
		repository.NewAgentRepository(db),
		repository.NewAgentHandoffRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		nil, // wsPublisher is nil-safe
		nil, nil,
		db,
		"planner-test-model", "anthropic",
	)
	return &preRouterTestEnv{db: db, svc: svc, stub: stub, msgRepo: msgRepo, convo: convo, wsID: wsID, agentID: agentID}
}

func (e *preRouterTestEnv) sendCustomerMessage(t *testing.T, content string) *model.SupportMessage {
	t.Helper()
	msg := &model.SupportMessage{
		WorkspaceID:    e.wsID,
		ConversationID: e.convo.ID,
		SenderType:     "customer",
		MessageType:    "reply",
		Content:        content,
	}
	if err := e.msgRepo.Create(context.Background(), msg); err != nil {
		t.Fatalf("create customer message: %v", err)
	}
	return msg
}

func (e *preRouterTestEnv) seedAITurn(t *testing.T, kind, content string) {
	t.Helper()
	meta, _ := json.Marshal(AIMessageMetadata{AIAutoReply: true, AIAgentID: e.agentID, AIReplyKind: kind, AIConfidence: 0.8, AIModel: "test"})
	msg := &model.SupportMessage{
		WorkspaceID:    e.wsID,
		ConversationID: e.convo.ID,
		SenderType:     "ai",
		SenderAgentID:  &e.agentID,
		MessageType:    "reply",
		Content:        content,
		Metadata:       string(meta),
	}
	if err := e.msgRepo.Create(context.Background(), msg); err != nil {
		t.Fatalf("seed AI turn: %v", err)
	}
}

func (e *preRouterTestEnv) reloadConversation(t *testing.T) *model.SupportConversation {
	t.Helper()
	var conv model.SupportConversation
	if err := e.db.Where("id = ?", e.convo.ID).First(&conv).Error; err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	return &conv
}

func (e *preRouterTestEnv) listMessages(t *testing.T) []model.SupportMessage {
	t.Helper()
	var msgs []model.SupportMessage
	if err := e.db.Where("conversation_id = ?", e.convo.ID).Order("created_at ASC, id ASC").Find(&msgs).Error; err != nil {
		t.Fatalf("list messages: %v", err)
	}
	return msgs
}

func TestHandleIncomingMessageGreetingShortCircuit(t *testing.T) {
	env := setupPreRouterEnv(t, "ai_first")
	env.stub.response = `{"route":"conversational","reply":"Hello! How can I help?","intent":"unknown","subject":"","language":"en","risk":"general","required_evidence":[],"context_action":"new_issue","issue_key":"greeting","issue_summary":"Customer greeted support.","progress_signal":"new_issue","standalone_query":"","search_queries":[],"reason":"greeting"}`
	msg := env.sendCustomerMessage(t, "Hi")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}

	if n := env.stub.callCount(); n != 1 {
		t.Fatalf("greeting must invoke only the LLM pre-router, got %d calls", n)
	}
	msgs := env.listMessages(t)
	var aiMsg *model.SupportMessage
	for i := range msgs {
		if msgs[i].SenderType == "ai" {
			aiMsg = &msgs[i]
		}
	}
	if aiMsg == nil {
		t.Fatalf("expected an AI greeting reply, messages: %d", len(msgs))
	}
	if aiMsg.Content != "Hello! How can I help?" {
		t.Errorf("greeting content = %q", aiMsg.Content)
	}
	var meta AIMessageMetadata
	if err := json.Unmarshal([]byte(aiMsg.Metadata), &meta); err != nil {
		t.Fatalf("parse AI metadata: %v", err)
	}
	if meta.AIReplyKind != supportReplyKindGreeting {
		t.Errorf("reply kind = %q, want %q", meta.AIReplyKind, supportReplyKindGreeting)
	}
	if meta.AIModel != "planner-test-model" {
		t.Errorf("model = %q, want planner-test-model", meta.AIModel)
	}
	if meta.AITokensUsed != 0 {
		t.Errorf("tokens used = %d, want 0", meta.AITokensUsed)
	}

	conv := env.reloadConversation(t)
	if conv.AIState == nil || *conv.AIState != "pending" {
		t.Errorf("ai_state = %v, want pending", conv.AIState)
	}
	if conv.FlowState == nil || *conv.FlowState != model.SupportConversationFlowStateAIHandling {
		t.Errorf("flow_state = %v, want ai_handling", conv.FlowState)
	}

	var processing model.AIMessageProcessing
	if err := env.db.Where("source_message_id = ?", msg.ID).First(&processing).Error; err != nil {
		t.Fatalf("load processing row: %v", err)
	}
	if processing.Status != "completed" {
		t.Errorf("processing status = %q, want completed", processing.Status)
	}
}

func TestHandleIncomingMessageSubstantiveReachesPlanner(t *testing.T) {
	env := setupPreRouterEnv(t, "internal_note")
	env.stub.response = `{"route":"handoff","reply":"I’ll connect you with the team.","intent":"unknown","subject":"subscription","language":"en","risk":"general","required_evidence":[],"context_action":"new_issue","issue_key":"cancel_subscription","issue_summary":"Customer wants to cancel.","progress_signal":"new_issue","standalone_query":"","search_queries":[],"reason":"account_specific_action"}`
	msg := env.sendCustomerMessage(t, "Hi. How to cancel the subscription?")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}

	if n := env.stub.callCount(); n == 0 {
		t.Fatalf("substantive message must reach the planner (0 LLM calls recorded)")
	}
	msgs := env.listMessages(t)
	foundNote := false
	for _, m := range msgs {
		if m.SenderType == "ai" && m.IsInternal {
			foundNote = true
		}
	}
	if !foundNote {
		t.Errorf("expected an internal handoff note from the planner path")
	}
}

func TestHandleIncomingMessageGreetingInNoteModeSkipsSilently(t *testing.T) {
	env := setupPreRouterEnv(t, "internal_note")
	env.stub.response = `{"route":"conversational","reply":"Hello! How can I help?","intent":"unknown","subject":"","language":"en","risk":"general","required_evidence":[],"context_action":"new_issue","issue_key":"greeting","issue_summary":"Customer greeted support.","progress_signal":"new_issue","standalone_query":"","search_queries":[],"reason":"greeting"}`
	msg := env.sendCustomerMessage(t, "Hello")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}
	if n := env.stub.callCount(); n != 1 {
		t.Fatalf("greeting must invoke only the LLM pre-router, got %d calls", n)
	}
	foundNote := false
	for _, m := range env.listMessages(t) {
		if m.SenderType == "ai" && m.IsInternal {
			foundNote = true
		}
	}
	if !foundNote {
		t.Fatal("internal-note mode should create an LLM-authored greeting suggestion")
	}
}

func TestHandleIncomingMessageThanksAfterAnswerResolves(t *testing.T) {
	env := setupPreRouterEnv(t, "ai_first")
	env.stub.response = `{"route":"confirmation","reply":"You’re welcome!","intent":"unknown","subject":"export","language":"en","risk":"general","required_evidence":[],"context_action":"confirm_previous","issue_key":"export","issue_summary":"Customer confirmed the export answer.","progress_signal":"same_issue_new_info","standalone_query":"","search_queries":[],"reason":"confirmed"}`
	env.seedAITurn(t, supportReplyKindAnswer, "You can export from Settings → Data export.")
	msg := env.sendCustomerMessage(t, "Thanks!")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}
	if n := env.stub.callCount(); n != 1 {
		t.Fatalf("confirmation must invoke only the LLM pre-router, got %d calls", n)
	}
	conv := env.reloadConversation(t)
	if conv.AIState == nil || *conv.AIState != "resolved" {
		t.Fatalf("ai_state = %v, want resolved", conv.AIState)
	}
	if conv.AIResolutionType == nil || *conv.AIResolutionType != "confirmed" {
		t.Errorf("ai_resolution_type = %v, want confirmed", conv.AIResolutionType)
	}
	if conv.FlowState == nil || *conv.FlowState != model.SupportConversationFlowStateResolvedByAI {
		t.Errorf("flow_state = %v, want resolved_by_ai", conv.FlowState)
	}
}

func TestHandleIncomingMessageConfirmationInNoteModeDoesNotResolve(t *testing.T) {
	env := setupPreRouterEnv(t, "internal_note")
	env.stub.response = `{"route":"confirmation","reply":"You’re welcome!","intent":"unknown","subject":"export","language":"en","risk":"general","required_evidence":[],"context_action":"confirm_previous","issue_key":"export","issue_summary":"Customer confirmed the export answer.","progress_signal":"same_issue_new_info","standalone_query":"","search_queries":[],"reason":"confirmed"}`
	env.seedAITurn(t, supportReplyKindAnswer, "You can export from Settings → Data export.")
	msg := env.sendCustomerMessage(t, "Thanks!")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}
	conv := env.reloadConversation(t)
	if conv.AIState != nil && *conv.AIState == "resolved" {
		t.Fatal("internal-note shadow mode must not resolve the live conversation")
	}
	foundNote := false
	for _, message := range env.listMessages(t) {
		if message.SenderType == "ai" && message.IsInternal && strings.Contains(message.Content, "You’re welcome!") {
			foundNote = true
		}
	}
	if !foundNote {
		t.Fatal("internal-note mode should preserve the confirmation as a suggestion")
	}
}

func TestHandleIncomingMessageNegativeFeedbackDoesNotResolve(t *testing.T) {
	env := setupPreRouterEnv(t, "internal_note")
	env.stub.response = `{"route":"handoff","reply":"I’ll connect you with the team.","intent":"unknown","subject":"","language":"en","risk":"general","required_evidence":[],"context_action":"continue","issue_key":"unhelpful_answer","issue_summary":"Customer says the previous answer was not helpful.","progress_signal":"same_issue_repeat","standalone_query":"","search_queries":[],"reason":"needs_human_help"}`
	env.seedAITurn(t, supportReplyKindAnswer, "Try clearing your cache.")
	msg := env.sendCustomerMessage(t, "not helpful at all")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}
	conv := env.reloadConversation(t)
	if conv.AIState != nil && *conv.AIState == "resolved" {
		t.Fatalf("negative feedback must never resolve the conversation (old substring-matcher bug)")
	}
}

func TestHandleIncomingMessageThanksAfterGreetingOnlyDoesNotResolve(t *testing.T) {
	env := setupPreRouterEnv(t, "internal_note")
	env.stub.response = `{"route":"confirmation","reply":"You’re welcome!","intent":"unknown","subject":"","language":"en","risk":"general","required_evidence":[],"context_action":"confirm_previous","issue_key":"greeting","issue_summary":"Customer thanked support after a greeting.","progress_signal":"same_issue_new_info","standalone_query":"","search_queries":[],"reason":"confirmed"}`
	env.seedAITurn(t, supportReplyKindGreeting, "Hello! How can I help?")
	msg := env.sendCustomerMessage(t, "thanks")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}
	conv := env.reloadConversation(t)
	if conv.AIState != nil && *conv.AIState == "resolved" {
		t.Fatalf("thanks after a greeting-only turn must not count as a confirmed resolution")
	}
}
