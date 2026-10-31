package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
)

func TestCoverageAnalysisUsesFallbackWhenStrictSchemaHasNoCompatibleEndpoint(t *testing.T) {
	provider := &scriptedAICompletionProvider{
		errors: map[string]error{
			"z-ai/glm-5.3-flash:exacto": &llm.ProviderError{Provider: "openrouter", StatusCode: http.StatusNotFound,
				Message: `{"error":{"message":"No endpoints found that support the requested parameters"}}`},
		},
		responses: map[string]*llm.ChatResponse{"openai/gpt-5.6-luna": {Content: coverageTestGapJSON}},
	}
	completion := NewAICompletionService(provider, NewCommunityAIUsage(&recordingCommunityUsage{}), DefaultAICompletionRouteRegistry()).
		SetGovernance(aipolicy.DefaultRegistry(), nil)
	analyzer := NewSupportCoverageDailyAnalyzer(completion, "", "")
	result, _, err := analyzer.AnalyzeConversation(context.Background(), CoverageConversationAnalysisInput{
		WorkspaceID: "ws-1", ConversationID: "c-1", TranscriptHash: "hash",
		Messages: []CoverageConversationMessage{{ID: "m-1", SenderType: "customer", Content: "How do I update my billing details?"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || !result.HasGap || len(provider.requests) != 2 {
		t.Fatalf("result = %+v, model calls = %d; want fallback analysis", result, len(provider.requests))
	}
	for _, request := range provider.requests {
		if !request.JSONSchemaStrict || len(request.JSONSchema) == 0 {
			t.Fatal("fallback must retain the strict analysis contract")
		}
	}
}
