package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type scriptedSupportPreviewLLM struct {
	responses []llm.ChatResponse
	errs      []error
}

func (f *scriptedSupportPreviewLLM) ChatCompletion(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(f.responses) == 0 {
		return &llm.ChatResponse{}, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return &resp, nil
}

func TestPreviewSupportReplyClarifySkipsAnswerGeneration(t *testing.T) {
	svc := &SupportAIService{
		llmProvider:            &scriptedSupportPreviewLLM{responses: []llm.ChatResponse{{Content: `{"decision":"clarify","standalone_query":"","search_queries":[],"clarifying_question":"Do you mean Publer features or ContentStudio features?","reason":"needs_clarification"}`, TokensUsed: llm.TokenUsage{InputTokens: 10, OutputTokens: 8}}}},
		queryExpansionModel:    "gpt-5.4-mini",
		queryExpansionProvider: "openai",
	}

	resp, err := svc.previewSupportReply(
		context.Background(),
		"ws-1",
		&model.Agent{Name: "Support", Provider: strPtr(model.AgentModelProviderOpenAI), Model: strPtr("gpt-5-mini")},
		[]model.SupportMessage{{SenderType: "customer", MessageType: "reply", Content: "Publer or ContentStudio?"}},
		"features",
		true,
		8,
		0.7,
		"history",
	)
	if err != nil {
		t.Fatalf("previewSupportReply() error = %v", err)
	}
	if resp.FinalDecision != supportDecisionClarify {
		t.Fatalf("final_decision = %q, want %q", resp.FinalDecision, supportDecisionClarify)
	}
	if resp.Answer != nil {
		t.Fatalf("expected answer to be nil for clarify decision, got %#v", resp.Answer)
	}
	if resp.QueryPlan.ClarifyingQuestion == "" {
		t.Fatal("expected clarifying question to be populated")
	}
	if resp.TotalTokensUsed != 18 {
		t.Fatalf("total_tokens_used = %d, want 18", resp.TotalTokensUsed)
	}
	if resp.QueryPlan.SearchQueries == nil {
		t.Fatal("expected query_plan.search_queries to be an empty slice, got nil")
	}
	if len(resp.QueryPlan.SearchQueries) != 0 {
		t.Fatalf("search_queries len = %d, want 0", len(resp.QueryPlan.SearchQueries))
	}
}

func TestPreviewSupportReplyReturnsGroundedAnswerDecision(t *testing.T) {
	svc := &SupportAIService{
		llmProvider: &scriptedSupportPreviewLLM{
			responses: []llm.ChatResponse{
				{
					Content:    `{"decision":"answer","standalone_query":"Publer vs ContentStudio pricing","search_queries":["Publer vs ContentStudio pricing"],"clarifying_question":"","reason":"resolved_from_context"}`,
					TokensUsed: llm.TokenUsage{InputTokens: 12, OutputTokens: 7},
				},
				{
					Content:    `{"content":"ContentStudio offers a 7-day free trial and annual discounts.","can_answer":true,"source_doc_ids":[],"confidence":0.95}`,
					TokensUsed: llm.TokenUsage{InputTokens: 20, OutputTokens: 14},
				},
			},
		},
		queryExpansionModel:    "gpt-5.4-mini",
		queryExpansionProvider: "openai",
	}

	resp, err := svc.previewSupportReply(
		context.Background(),
		"ws-1",
		&model.Agent{Name: "Support", Provider: strPtr(model.AgentModelProviderOpenAI), Model: strPtr("gpt-5-mini")},
		[]model.SupportMessage{{SenderType: "customer", MessageType: "reply", Content: "Publer or ContentStudio?"}},
		"pricing",
		true,
		8,
		0.7,
		"history",
	)
	if err != nil {
		t.Fatalf("previewSupportReply() error = %v", err)
	}
	if resp.FinalDecision != supportDecisionAnswer {
		t.Fatalf("final_decision = %q, want %q", resp.FinalDecision, supportDecisionAnswer)
	}
	if resp.Answer == nil {
		t.Fatal("expected answer to be present")
	}
	if !resp.Answer.CanAnswer {
		t.Fatal("expected can_answer to be true")
	}
	if resp.Answer.GroundedConfidence <= 0.7 {
		t.Fatalf("grounded_confidence = %v, want > 0.7", resp.Answer.GroundedConfidence)
	}
	if resp.TotalTokensUsed != 53 {
		t.Fatalf("total_tokens_used = %d, want 53", resp.TotalTokensUsed)
	}
	if resp.Answer.SourceDocIDs == nil {
		t.Fatal("expected answer.source_doc_ids to be an empty slice, got nil")
	}
}

func TestPreviewSupportReplySurfacesPlannerFallback(t *testing.T) {
	svc := &SupportAIService{
		llmProvider:            &scriptedSupportPreviewLLM{errs: []error{errors.New("planner unavailable"), nil}, responses: []llm.ChatResponse{{Content: `{"content":"Fallback answer","can_answer":true,"source_doc_ids":[],"confidence":0.9}`, TokensUsed: llm.TokenUsage{InputTokens: 18, OutputTokens: 9}}}},
		queryExpansionModel:    "gpt-5.4-mini",
		queryExpansionProvider: "openai",
	}

	resp, err := svc.previewSupportReply(
		context.Background(),
		"ws-1",
		&model.Agent{Name: "Support", Provider: strPtr(model.AgentModelProviderOpenAI), Model: strPtr("gpt-5-mini")},
		nil,
		"pricing",
		true,
		8,
		0.7,
		"none",
	)
	if err != nil {
		t.Fatalf("previewSupportReply() error = %v", err)
	}
	if !resp.QueryPlan.FallbackUsed {
		t.Fatal("expected planner fallback to be recorded")
	}
	if resp.QueryPlan.Error == "" {
		t.Fatal("expected planner error to be populated")
	}
}
