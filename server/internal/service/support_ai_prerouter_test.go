package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Unit tests: pre-route classification
// ---------------------------------------------------------------------------

// Real customer messages observed in production (usermaven workspace) are
// marked with "prod:". The pre-router must never swallow a substantive one.
func TestPreRouteSupportMessage(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		aiTurnCount int
		want        preRouteOutcome
	}{
		// Greetings — short-circuit on first contact.
		{name: "prod: bare hi", content: "Hi", aiTurnCount: 0, want: preRouteGreet},
		{name: "prod: bare hello", content: "Hello", aiTurnCount: 0, want: preRouteGreet},
		{name: "hey with punctuation", content: "Hey!!", aiTurnCount: 0, want: preRouteGreet},
		{name: "hi with emoji", content: "Hi 👋", aiTurnCount: 0, want: preRouteGreet},
		{name: "hi there", content: "hi there", aiTurnCount: 0, want: preRouteGreet},
		{name: "good morning", content: "Good morning", aiTurnCount: 0, want: preRouteGreet},
		{name: "hello good morning", content: "Hello good morning!", aiTurnCount: 0, want: preRouteGreet},
		{name: "hey team", content: "Hey team", aiTurnCount: 0, want: preRouteGreet},
		{name: "all caps hi", content: "HI", aiTurnCount: 0, want: preRouteGreet},

		// Opening gratitude — lightweight acknowledgement.
		{name: "bare thanks", content: "Thanks!", aiTurnCount: 0, want: preRouteGratitude},
		{name: "thank you so much", content: "thank you so much", aiTurnCount: 0, want: preRouteGratitude},
		{name: "ok thanks", content: "ok thanks", aiTurnCount: 0, want: preRouteGratitude},
		{name: "ty", content: "ty", aiTurnCount: 0, want: preRouteGratitude},

		// Mid-conversation: never short-circuit (nudges, confirmations).
		{name: "hello nudge mid-conversation", content: "hello??", aiTurnCount: 2, want: preRouteProceed},
		{name: "thanks mid-conversation", content: "thanks", aiTurnCount: 1, want: preRouteProceed},

		// Substantive messages — all must proceed to the full pipeline.
		{name: "prod: guest posting", content: "DO YOU ALLOW GUEST POSTING?", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: plan picker template", content: `Help me choose a Usermaven plan. My answer: "I want to understand how people use my website or product." Based on this, the Growth plan looks like the best fit`, aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: greeting plus question", content: "hi, can we filter analytics dashboard by contact segments?", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: hi how to cancel", content: "Hi. How to cancel the subscription?", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: hi cancel plan", content: "Hi, we'd like to cancel the plan we have", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: mcp question", content: "does usermaven have an MCP?", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: gibberish with hello", content: "Hello.order.mr", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: b2c question", content: "do u deal with b2c", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: cancel plan", content: "Cancel plan", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: product feedback", content: "Suggest changes to product UX", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: wordpress bug report", content: "The WordPress plugin isn't working. I have the necessary connections set up, but it isn't recording any events.", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: google ads integration", content: "Hi there, I'm wanting to integrate google ads using the integration feature. It is saying the integration link is expired when I try it - any ideas?", aiTurnCount: 0, want: preRouteProceed},
		{name: "prod: mcp workspaces", content: "i need to connect several workspaces with claude via mcp it seems that only allows one", aiTurnCount: 0, want: preRouteProceed},
		{name: "greeting word inside sentence", content: "Please tell Waqar I said hi", aiTurnCount: 0, want: preRouteProceed},
		{name: "empty message", content: "", aiTurnCount: 0, want: preRouteProceed},
		{name: "emoji only", content: "👋", aiTurnCount: 0, want: preRouteProceed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preRouteSupportMessage(tt.content, tt.aiTurnCount)
			if got.Outcome != tt.want {
				t.Errorf("preRouteSupportMessage(%q, turns=%d) = %q, want %q", tt.content, tt.aiTurnCount, got.Outcome, tt.want)
			}
			if got.Outcome != preRouteProceed && strings.TrimSpace(got.Reply) == "" {
				t.Errorf("short-circuit outcome %q must carry a reply", got.Outcome)
			}
		})
	}
}

func TestIsPureGreetingEdgeCases(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"good morning", true},
		{"good day", true},
		{"good", false},         // no time-of-day, no core greeting
		{"morning", false},      // filler alone is not a greeting
		{"yo", true},
		{"there team", false},   // fillers without a core word
		{"hi hi hi hi hi hi hi", false}, // over token cap
	}
	for _, tt := range tests {
		if got := isPureGreeting(tt.content); got != tt.want {
			t.Errorf("isPureGreeting(%q) = %v, want %v", tt.content, got, tt.want)
		}
	}
}

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

// recordingLLMStub records every chat call. The pre-router contract is that
// short-circuited messages produce ZERO calls.
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
	msg := env.sendCustomerMessage(t, "Hi")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}

	if n := env.stub.callCount(); n != 0 {
		t.Fatalf("greeting must not invoke the LLM, got %d calls", n)
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
	if aiMsg.Content != supportPreRouterGreetingReply {
		t.Errorf("greeting content = %q, want %q", aiMsg.Content, supportPreRouterGreetingReply)
	}
	var meta AIMessageMetadata
	if err := json.Unmarshal([]byte(aiMsg.Metadata), &meta); err != nil {
		t.Fatalf("parse AI metadata: %v", err)
	}
	if meta.AIReplyKind != supportReplyKindGreeting {
		t.Errorf("reply kind = %q, want %q", meta.AIReplyKind, supportReplyKindGreeting)
	}
	if meta.AIModel != supportPreRouterModelName {
		t.Errorf("model = %q, want %q", meta.AIModel, supportPreRouterModelName)
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
	env.stub.response = `{"decision":"handoff","reason":"account_specific_action","issue_key":"cancel-subscription","issue_summary":"Customer wants to cancel"}`
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
	msg := env.sendCustomerMessage(t, "Hello")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}
	if n := env.stub.callCount(); n != 0 {
		t.Fatalf("greeting must not invoke the LLM, got %d calls", n)
	}
	for _, m := range env.listMessages(t) {
		if m.SenderType == "ai" {
			t.Fatalf("internal-note mode must not auto-reply to a greeting, got AI message %q", m.Content)
		}
	}
}

func TestHandleIncomingMessageThanksAfterAnswerResolves(t *testing.T) {
	env := setupPreRouterEnv(t, "ai_first")
	env.seedAITurn(t, supportReplyKindAnswer, "You can export from Settings → Data export.")
	msg := env.sendCustomerMessage(t, "Thanks!")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}
	if n := env.stub.callCount(); n != 0 {
		t.Fatalf("confirmation must not invoke the LLM, got %d calls", n)
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

func TestHandleIncomingMessageNegativeFeedbackDoesNotResolve(t *testing.T) {
	env := setupPreRouterEnv(t, "internal_note")
	env.stub.response = `{"decision":"handoff","reason":"needs_human_help"}`
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
	env.stub.response = `{"decision":"handoff","reason":"unclear_request"}`
	env.seedAITurn(t, supportReplyKindGreeting, supportPreRouterGreetingReply)
	msg := env.sendCustomerMessage(t, "thanks")

	if err := env.svc.HandleIncomingMessage(context.Background(), env.wsID, env.convo.ID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage: %v", err)
	}
	conv := env.reloadConversation(t)
	if conv.AIState != nil && *conv.AIState == "resolved" {
		t.Fatalf("thanks after a greeting-only turn must not count as a confirmed resolution")
	}
}

// ---------------------------------------------------------------------------
// Live-model oracle evaluation (requires ANTHROPIC_API_KEY; skipped otherwise)
//
// Replays the real production messages through a small Claude model acting as
// an independent classifier and asserts the deterministic pre-router agrees.
// Run with: go test ./internal/service/ -run TestPreRouterLiveModelOracle -v
// ---------------------------------------------------------------------------

func TestPreRouterLiveModelOracle(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		apiKey = readEnvFileKey(t, "ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set; skipping live-model oracle evaluation")
	}
	provider := llm.NewClaudeProvider(apiKey)
	if provider == nil {
		t.Fatal("nil Claude provider")
	}

	cases := []struct {
		content string
		want    string // greeting | gratitude | substantive
	}{
		{"Hi", "greeting"},
		{"Hello", "greeting"},
		{"Hey team", "greeting"},
		{"Good morning", "greeting"},
		{"Thanks!", "gratitude"},
		{"thank you so much", "gratitude"},
		{"DO YOU ALLOW GUEST POSTING?", "substantive"},
		{"hi, can we filter analytics dashboard by contact segments?", "substantive"},
		{"Hi. How to cancel the subscription?", "substantive"},
		{"does usermaven have an MCP?", "substantive"},
		{"do u deal with b2c", "substantive"},
		{"Cancel plan", "substantive"},
		{"The WordPress plugin isn't working. I have the necessary connections set up, but it isn't recording any events.", "substantive"},
		{`Help me choose a Usermaven plan. My answer: "I want to understand how people use my website or product." Based on this, the Growth plan looks like the best fit`, "substantive"},
	}

	systemPrompt := `You classify the FIRST customer message of a support conversation.
Reply with a single JSON object: {"category": "<greeting|gratitude|substantive>"}.
- "greeting": the message is ONLY a salutation with no request or content.
- "gratitude": the message is ONLY an expression of thanks with no request.
- "substantive": anything containing a question, request, problem, or any other content.`

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	disagreements := 0
	for _, tc := range cases {
		deterministic := "substantive"
		switch preRouteSupportMessage(tc.content, 0).Outcome {
		case preRouteGreet:
			deterministic = "greeting"
		case preRouteGratitude:
			deterministic = "gratitude"
		}
		if deterministic != tc.want {
			t.Errorf("deterministic router: %q classified as %s, want %s", tc.content, deterministic, tc.want)
		}

		resp, err := provider.ChatCompletion(ctx, llm.ChatRequest{
			Model:        "claude-haiku-4-5",
			SystemPrompt: systemPrompt,
			Messages:     []llm.Message{{Role: "user", Content: tc.content}},
			MaxTokens:    64,
			JSONMode:     true,
			JSONSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"category": map[string]any{"type": "string", "enum": []string{"greeting", "gratitude", "substantive"}},
				},
				"required": []string{"category"},
			},
		})
		if err != nil {
			t.Fatalf("live model call failed for %q: %v", tc.content, err)
		}
		var out struct {
			Category string `json:"category"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &out); err != nil {
			t.Fatalf("parse oracle response %q: %v", resp.Content, err)
		}
		if out.Category != deterministic {
			disagreements++
			t.Logf("DISAGREEMENT on %q: deterministic=%s oracle=%s", tc.content, deterministic, out.Category)
		}
	}
	if disagreements > 0 {
		t.Errorf("live-model oracle disagreed on %d/%d cases", disagreements, len(cases))
	}
}

// readEnvFileKey scans server/.env for a key without loading the whole file
// into the process environment.
func readEnvFileKey(t *testing.T, key string) string {
	t.Helper()
	for _, candidate := range []string{".env", filepath.Join("..", "..", ".env")} {
		f, err := os.Open(candidate)
		if err != nil {
			continue
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if v, ok := strings.CutPrefix(line, key+"="); ok {
				return strings.Trim(strings.TrimSpace(v), `"'`)
			}
		}
	}
	return ""
}
