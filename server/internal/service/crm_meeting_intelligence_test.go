package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type meetingIntelligenceLLMResult struct {
	response *llm.ChatResponse
	err      error
}

type scriptedMeetingIntelligenceLLM struct {
	results  []meetingIntelligenceLLMResult
	requests []llm.ChatRequest
	metering []AIUsageMeteringContext
}

func (p *scriptedMeetingIntelligenceLLM) ChatCompletion(ctx context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
	p.requests = append(p.requests, request)
	if metering, ok := AIUsageMeteringFromContext(ctx); ok {
		p.metering = append(p.metering, metering)
	}
	index := len(p.requests) - 1
	if index >= len(p.results) {
		return nil, errors.New("unexpected meeting intelligence call")
	}
	return p.results[index].response, p.results[index].err
}

func TestMeetingIntelligenceFallsBackAfterReasoningOnlyResponse(t *testing.T) {
	provider := &scriptedMeetingIntelligenceLLM{results: []meetingIntelligenceLLMResult{
		{response: &llm.ChatResponse{
			FinishReason: "length",
			TokensUsed: llm.TokenUsage{
				CompletionTokensTotal: 7696,
				ReasoningTokens:       7696,
			},
		}},
		{response: &llm.ChatResponse{Content: validMeetingIntelligenceJSON()}},
	}}
	processor := &CRMMeetingProcessingService{llmProvider: provider}
	output, err := processor.generateIntelligence(
		context.Background(),
		&model.CRMMeeting{ID: "meeting-1", WorkspaceID: "workspace-1", Title: "Weekly sync"},
		&model.CRMMeetingTranscript{Checksum: "checksum-1", PlainText: "Azhar: We will ship Friday.", SourceProvider: "recall"},
	)
	if err != nil {
		t.Fatalf("generate intelligence: %v", err)
	}
	if output == nil || output.SummaryMarkdown != "The team agreed to ship." {
		t.Fatalf("output = %#v", output)
	}
	if len(output.ParticipantsContext) != 1 || len(output.Rapport) != 1 {
		t.Fatalf("linear sections = %#v", output)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("requests = %d, want primary and fallback", len(provider.requests))
	}
	primary, fallback := provider.requests[0], provider.requests[1]
	if primary.Provider != meetingIntelligenceLLMProvider || primary.Model != meetingIntelligenceLLMModel {
		t.Fatalf("primary route = %s/%s", primary.Provider, primary.Model)
	}
	if fallback.Provider != meetingIntelligenceFallbackProvider || fallback.Model != meetingIntelligenceFallbackModel {
		t.Fatalf("fallback route = %s/%s", fallback.Provider, fallback.Model)
	}
	for index, request := range provider.requests {
		if request.MaxTokens != meetingIntelligenceMaxTokens {
			t.Fatalf("request %d max tokens = %d", index, request.MaxTokens)
		}
		if !request.JSONMode || !request.JSONSchemaStrict || len(request.JSONSchema) == 0 {
			t.Fatalf("request %d missing strict structured output: %#v", index, request)
		}
		if request.Reasoning == nil || request.Reasoning.Effort != "low" {
			t.Fatalf("request %d reasoning = %#v", index, request.Reasoning)
		}
		if string(request.ProviderOptions) != `{"require_parameters":true}` {
			t.Fatalf("request %d provider options = %s", index, request.ProviderOptions)
		}
	}
	if len(provider.metering) != 2 ||
		!strings.HasSuffix(provider.metering[0].IdempotencyKey, ":v3:primary") ||
		!strings.HasSuffix(provider.metering[1].IdempotencyKey, ":v3:fallback") {
		t.Fatalf("metering = %#v", provider.metering)
	}
}

func TestMeetingIntelligenceReturnsStableErrorAfterInvalidFallback(t *testing.T) {
	provider := &scriptedMeetingIntelligenceLLM{results: []meetingIntelligenceLLMResult{
		{response: &llm.ChatResponse{Content: "{"}},
		{response: &llm.ChatResponse{Content: "{}"}},
	}}
	processor := &CRMMeetingProcessingService{llmProvider: provider}
	_, err := processor.generateIntelligence(
		context.Background(),
		&model.CRMMeeting{ID: "meeting-2", WorkspaceID: "workspace-1", Title: "Weekly sync"},
		&model.CRMMeetingTranscript{Checksum: "checksum-2", PlainText: "Maya: No decisions today.", SourceProvider: "recall"},
	)
	if err == nil {
		t.Fatal("expected invalid structured output error")
	}
	if !strings.Contains(err.Error(), "generation failed after trying a backup model") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Fatalf("low-level JSON error leaked: %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("requests = %d, want primary and fallback", len(provider.requests))
	}
}

func validMeetingIntelligenceJSON() string {
	return `{
		"summary_markdown":"The team agreed to ship.",
		"participants_context":["Azhar led the weekly release sync"],
		"key_points":["Release is ready"],
		"decisions":["Ship Friday"],
		"objections":[],
		"risks":[],
		"open_questions":[],
		"next_steps":["Prepare release"],
		"rapport":["The team was aligned on the release"],
		"action_items":[],
		"follow_up_draft":{"subject":"Release","body":"We will ship Friday."}
	}`
}
