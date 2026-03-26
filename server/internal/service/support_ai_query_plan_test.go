package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type scriptedSupportPlannerLLM struct {
	responses []llm.ChatResponse
	requests  []llm.ChatRequest
}

func (f *scriptedSupportPlannerLLM) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return &llm.ChatResponse{}, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return &resp, nil
}

func TestSanitizeConversationHistoryExcludesCurrentAndSystemMessages(t *testing.T) {
	history := []model.SupportMessage{
		{ID: "m1", SenderType: "customer", MessageType: "reply", Content: "Publer or ContentStudio?"},
		{ID: "m2", SenderType: "agent", MessageType: "system", Content: "Let me connect you with a team member."},
		{ID: "m3", SenderType: "ai", MessageType: "reply", Content: "ContentStudio is stronger for agencies."},
		{ID: "m4", SenderType: "customer", MessageType: "reply", Content: "features"},
	}

	sanitized := sanitizeConversationHistory(history, "m4")
	if len(sanitized) != 2 {
		t.Fatalf("sanitizeConversationHistory() len = %d, want 2", len(sanitized))
	}
	if sanitized[0].ID != "m1" || sanitized[1].ID != "m3" {
		t.Fatalf("unexpected sanitized history: %+v", sanitized)
	}
}

func TestBuildConversationMessagesTreatsAIAsAssistant(t *testing.T) {
	messages := buildConversationMessages([]model.SupportMessage{
		{SenderType: "customer", Content: "Publer or ContentStudio?"},
		{SenderType: "ai", Content: "ContentStudio is stronger for agencies."},
		{SenderType: "user", Content: "Anything else I can help with?"},
	})

	if len(messages) != 3 {
		t.Fatalf("buildConversationMessages() len = %d, want 3", len(messages))
	}
	if messages[0].Role != "user" {
		t.Fatalf("first role = %q, want user", messages[0].Role)
	}
	if messages[1].Role != "assistant" {
		t.Fatalf("ai role = %q, want assistant", messages[1].Role)
	}
	if messages[2].Role != "assistant" {
		t.Fatalf("user role = %q, want assistant", messages[2].Role)
	}
}

func TestPlanSupportQueryResolvesFollowUpFromContext(t *testing.T) {
	provider := &scriptedSupportPlannerLLM{
		responses: []llm.ChatResponse{
			{
				Content: `{"decision":"answer","standalone_query":"Publer vs ContentStudio features","search_queries":["ContentStudio features vs Publer","Publer ContentStudio feature comparison"],"clarifying_question":"","reason":"resolved_from_context"}`,
				TokensUsed: llm.TokenUsage{
					InputTokens:  15,
					OutputTokens: 9,
				},
			},
		},
	}
	svc := &SupportAIService{
		llmProvider:            provider,
		queryExpansionModel:    "gpt-5.4-mini",
		queryExpansionProvider: "openai",
	}

	history := []model.SupportMessage{
		{SenderType: "customer", MessageType: "reply", Content: "Publer or ContentStudio?"},
		{SenderType: "ai", MessageType: "reply", Content: "ContentStudio is stronger for agencies and richer analytics."},
	}

	plan, tokensUsed, err := svc.planSupportQuery(context.Background(), history, model.SupportMessage{SenderType: "customer", Content: "features"})
	if err != nil {
		t.Fatalf("planSupportQuery() error = %v", err)
	}
	if tokensUsed != 24 {
		t.Fatalf("tokensUsed = %d, want 24", tokensUsed)
	}
	if plan.Decision != supportDecisionAnswer {
		t.Fatalf("decision = %q, want %q", plan.Decision, supportDecisionAnswer)
	}
	if plan.StandaloneQuery != "Publer vs ContentStudio features" {
		t.Fatalf("standalone_query = %q", plan.StandaloneQuery)
	}
	wantQueries := []string{
		"Publer vs ContentStudio features",
		"ContentStudio features vs Publer",
		"Publer ContentStudio feature comparison",
	}
	if len(plan.SearchQueries) != len(wantQueries) {
		t.Fatalf("search_queries len = %d, want %d: %#v", len(plan.SearchQueries), len(wantQueries), plan.SearchQueries)
	}
	for i := range wantQueries {
		if plan.SearchQueries[i] != wantQueries[i] {
			t.Fatalf("search_queries[%d] = %q, want %q", i, plan.SearchQueries[i], wantQueries[i])
		}
	}
	if len(provider.requests) != 1 {
		t.Fatalf("planner calls = %d, want 1", len(provider.requests))
	}
}

func TestPlanSupportQueryUsesClarifyInsteadOfHandoffForAmbiguousFollowUp(t *testing.T) {
	provider := &scriptedSupportPlannerLLM{
		responses: []llm.ChatResponse{
			{
				Content: `{"decision":"clarify","standalone_query":"","search_queries":[],"clarifying_question":"Do you mean Publer features or ContentStudio features?","reason":"needs_clarification"}`,
				TokensUsed: llm.TokenUsage{
					InputTokens:  12,
					OutputTokens: 10,
				},
			},
		},
	}
	svc := &SupportAIService{
		llmProvider:            provider,
		queryExpansionModel:    "gpt-5.4-mini",
		queryExpansionProvider: "openai",
	}

	history := []model.SupportMessage{
		{SenderType: "customer", MessageType: "reply", Content: "Which one is better?"},
	}

	plan, _, err := svc.planSupportQuery(context.Background(), history, model.SupportMessage{SenderType: "customer", Content: "pricing"})
	if err != nil {
		t.Fatalf("planSupportQuery() error = %v", err)
	}
	if plan.Decision != supportDecisionClarify {
		t.Fatalf("decision = %q, want %q", plan.Decision, supportDecisionClarify)
	}
	if plan.ClarifyingQuestion == "" {
		t.Fatal("expected clarifying question to be populated")
	}
	if plan.Reason != "needs_clarification" {
		t.Fatalf("reason = %q, want needs_clarification", plan.Reason)
	}
}

func TestPlanSupportQueryIncludesImageContentParts(t *testing.T) {
	provider := &scriptedSupportPlannerLLM{
		responses: []llm.ChatResponse{
			{
				Content: `{"decision":"answer","standalone_query":"screenshot issue","search_queries":["screenshot issue"],"clarifying_question":"","reason":"resolved_from_context"}`,
			},
		},
	}
	svc := &SupportAIService{
		llmProvider:            provider,
		queryExpansionModel:    "gpt-5.4-mini",
		queryExpansionProvider: "openai",
	}

	_, _, err := svc.planSupportQuery(context.Background(), nil, model.SupportMessage{
		SenderType: "customer",
		Attachments: []model.SupportAttachmentPayload{
			{FileName: "Screenshot.png", FileType: "image/png", URL: "https://assets.example.com/screenshot.png"},
		},
	})
	if err != nil {
		t.Fatalf("planSupportQuery() error = %v", err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("planner calls = %d, want 1", len(provider.requests))
	}
	if len(provider.requests[0].Messages) != 1 {
		t.Fatalf("planner messages = %#v", provider.requests[0].Messages)
	}
	if len(provider.requests[0].Messages[0].ContentParts) < 2 {
		t.Fatalf("expected planner content parts with image, got %#v", provider.requests[0].Messages[0].ContentParts)
	}
	foundImage := false
	for _, part := range provider.requests[0].Messages[0].ContentParts {
		if part.Type == "image_url" && part.ImageURL != nil && part.ImageURL.URL == "https://assets.example.com/screenshot.png" {
			foundImage = true
			break
		}
	}
	if !foundImage {
		t.Fatalf("expected image_url content part, got %#v", provider.requests[0].Messages[0].ContentParts)
	}
}

func TestNormalizeSupportQueryPlanFallsBackToAnswerWhenClarifyQuestionMissing(t *testing.T) {
	plan := normalizeSupportQueryPlan(SupportQueryPlanContract{
		Decision: supportDecisionClarify,
		Reason:   "needs_clarification",
	}, "features")

	if plan.Decision != supportDecisionAnswer {
		t.Fatalf("decision = %q, want fallback answer", plan.Decision)
	}
	if len(plan.SearchQueries) != 1 || plan.SearchQueries[0] != "features" {
		t.Fatalf("search_queries = %#v, want raw message fallback", plan.SearchQueries)
	}
}

func TestParseSupportQueryPlanStripsCodeFences(t *testing.T) {
	plan, err := parseSupportQueryPlan("```json\n{\"decision\":\"handoff\",\"standalone_query\":\"\",\"search_queries\":[],\"clarifying_question\":\"\",\"reason\":\"customer_requested_human\"}\n```")
	if err != nil {
		t.Fatalf("parseSupportQueryPlan() error = %v", err)
	}
	if plan.Decision != supportDecisionHandoff {
		t.Fatalf("decision = %q, want %q", plan.Decision, supportDecisionHandoff)
	}
	if plan.Reason != "customer_requested_human" {
		t.Fatalf("reason = %q, want customer_requested_human", plan.Reason)
	}
}
