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

type blockingSupportPlannerLLM struct{}

func (f *blockingSupportPlannerLLM) ChatCompletion(ctx context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
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

func TestDefaultSupportQueryPlanGreeting(t *testing.T) {
	plan := defaultSupportQueryPlan("Hello!")
	if plan.Decision != supportDecisionAnswer {
		t.Fatalf("decision = %q, want conservative answer fallback", plan.Decision)
	}
	if plan.GreetingReply != "" || len(plan.SearchQueries) != 1 {
		t.Fatalf("plan = %+v, want no deterministic reply and one sufficiency query", plan)
	}
	mixed := defaultSupportQueryPlan("hi, how do I reset my password?")
	if mixed.Decision != supportDecisionAnswer {
		t.Fatalf("decision = %q, want answer for greeting+question", mixed.Decision)
	}
}
