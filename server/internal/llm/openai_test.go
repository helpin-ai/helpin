package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestOpenAIProviderBuildChatCompletionBodyOmitsTemperatureForGPT5(t *testing.T) {
	models := []string{"gpt-5.6-luna", "gpt-5.6-terra"}
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"route"},
		"properties": map[string]any{
			"route": map[string]any{"type": "string"},
		},
	}

	for _, modelName := range models {
		t.Run(modelName, func(t *testing.T) {
			provider := NewOpenAIProvider("test-key", "https://api.openai.com/v1", "")
			body := provider.buildChatCompletionBody(
				modelName,
				[]map[string]any{{"role": "user", "content": "hello"}},
				123,
				0.1,
				true,
				schema,
				true,
				nil,
			)

			if _, ok := body["max_completion_tokens"]; !ok {
				t.Fatalf("expected max_completion_tokens in request body, got %#v", body)
			}
			if _, ok := body["max_tokens"]; ok {
				t.Fatalf("did not expect max_tokens for GPT-5 request body, got %#v", body)
			}
			if _, ok := body["temperature"]; ok {
				t.Fatalf("did not expect temperature for GPT-5 request body, got %#v", body)
			}
			responseFormat, ok := body["response_format"].(map[string]any)
			if !ok || responseFormat["type"] != "json_schema" {
				t.Fatalf("expected structured output response_format in request body, got %#v", body)
			}
			structuredFormat, ok := responseFormat["json_schema"].(map[string]any)
			if !ok {
				t.Fatalf("expected json_schema configuration, got %#v", responseFormat)
			}
			if structuredFormat["name"] != "helpin_structured_response" || structuredFormat["strict"] != true {
				t.Fatalf("expected named strict json schema, got %#v", structuredFormat)
			}
			if !reflect.DeepEqual(structuredFormat["schema"], schema) {
				t.Fatalf("expected request schema to be preserved, got %#v", structuredFormat["schema"])
			}
		})
	}
}

func TestOpenAIProviderBuildChatCompletionBodyKeepsGenericJSONModeWithoutStrictSchema(t *testing.T) {
	provider := NewOpenAIProvider("test-key", "https://api.openai.com/v1", "")
	body := provider.buildChatCompletionBody(
		"gpt-5.6-luna",
		[]map[string]any{{"role": "user", "content": "hello"}},
		123,
		0.1,
		true,
		map[string]any{"type": "object"},
		false,
		nil,
	)

	responseFormat, ok := body["response_format"].(map[string]string)
	if !ok || responseFormat["type"] != "json_object" {
		t.Fatalf("expected generic JSON mode response_format, got %#v", body)
	}
}

func TestOpenAIProviderBuildChatCompletionBodyUsesMaxTokensForNonGPT5(t *testing.T) {
	provider := NewOpenAIProvider("test-key", "https://api.openai.com/v1", "")
	body := provider.buildChatCompletionBody(
		"gpt-4o-mini",
		[]map[string]any{{"role": "user", "content": "hello"}},
		123,
		0.1,
		false,
		nil,
		false,
		nil,
	)

	if _, ok := body["max_tokens"]; !ok {
		t.Fatalf("expected max_tokens in request body, got %#v", body)
	}
	if _, ok := body["max_completion_tokens"]; ok {
		t.Fatalf("did not expect max_completion_tokens for non-GPT-5 request body, got %#v", body)
	}
	if temperature, ok := body["temperature"].(float64); !ok || temperature != 0.1 {
		t.Fatalf("expected temperature 0.1 for non-GPT-5 request body, got %#v", body)
	}
	if _, ok := body["response_format"]; ok {
		t.Fatalf("did not expect response_format when JSON mode is off, got %#v", body)
	}
}

func TestOpenAIProviderBuildChatCompletionBodyIncludesProviderOptions(t *testing.T) {
	provider := NewOpenAIProvider("test-key", "https://openrouter.ai/api/v1", "")
	body := provider.buildChatCompletionBody(
		"openai/gpt-5.5",
		[]map[string]any{{"role": "user", "content": "hello"}},
		123,
		0.1,
		false,
		nil,
		false,
		[]byte(`{"order":["openai"],"allow_fallbacks":false}`),
	)

	raw, ok := body["provider"].(json.RawMessage)
	if !ok {
		t.Fatalf("expected provider options raw json, got %#v", body["provider"])
	}
	if string(raw) != `{"order":["openai"],"allow_fallbacks":false}` {
		t.Fatalf("unexpected provider options: %s", string(raw))
	}
}

func TestBuildOpenAIMessageUsesContentParts(t *testing.T) {
	message := buildOpenAIMessage(Message{
		Role: "user",
		ContentParts: []ContentPart{
			{Type: "text", Text: "Inspect this screenshot"},
			{Type: "image_url", ImageURL: &ImageURLPart{URL: "https://assets.example.com/example.png", Detail: "auto"}},
		},
	})

	content, ok := message["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected content parts array, got %#v", message["content"])
	}
	if len(content) != 2 {
		t.Fatalf("expected 2 content parts, got %#v", content)
	}
	if content[0]["type"] != "text" {
		t.Fatalf("expected first part text, got %#v", content[0])
	}
	if content[1]["type"] != "image_url" {
		t.Fatalf("expected second part image_url, got %#v", content[1])
	}
}

func TestOpenAIProviderChatCompletionNilReceiver(t *testing.T) {
	var provider *OpenAIProvider

	_, err := provider.ChatCompletion(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected nil provider error")
	}
}

func TestTokenUsageFromOpenAIIncludesCacheAndReasoningBreakdown(t *testing.T) {
	usage := tokenUsageFromOpenAI(openAIUsage{
		PromptTokens:     1200,
		CompletionTokens: 300,
		PromptTokensDetails: openAIPromptTokenDetails{
			CachedTokens:     800,
			CacheWriteTokens: 200,
		},
		CompletionTokensDetails: openAICompletionTokenDetails{ReasoningTokens: 100},
	})

	if usage.InputTokens != 1200 || usage.CachedInputTokens != 800 || usage.CacheWriteTokens != 200 {
		t.Fatalf("input usage = %#v", usage)
	}
	if usage.OutputTokens != 200 || usage.ReasoningTokens != 100 {
		t.Fatalf("output usage = %#v", usage)
	}
}

func TestOpenAIProviderCreateEmbeddingsNilReceiver(t *testing.T) {
	var provider *OpenAIProvider

	_, err := provider.CreateEmbeddings(context.Background(), EmbeddingRequest{
		Inputs: []string{"hello"},
	})
	if err == nil {
		t.Fatal("expected nil provider error")
	}
}

func TestOpenAIProviderCreateEmbeddingsAcceptsResponseLargerThanLegacyLimit(t *testing.T) {
	responseBody := `{"data":[],"padding":"` + strings.Repeat("x", (4<<20)+1024) + `"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responseBody))
	}))
	t.Cleanup(server.Close)

	provider := NewOpenAIProvider("test-key", server.URL, "")
	response, err := provider.CreateEmbeddings(context.Background(), EmbeddingRequest{Inputs: []string{"hello"}})
	if err != nil {
		t.Fatalf("CreateEmbeddings returned error for valid large response: %v", err)
	}
	if response == nil || len(response.Vectors) != 0 {
		t.Fatalf("unexpected embeddings response: %#v", response)
	}
}

func TestOpenAIProviderCreateEmbeddingsRejectsEmptySuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	provider := NewOpenAIProvider("test-key", server.URL, "")
	_, err := provider.CreateEmbeddings(context.Background(), EmbeddingRequest{Inputs: []string{"hello"}})
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Fatalf("error = %v, want explicit empty response error", err)
	}
}

func TestNewSupportRouterReturnsNilEmbeddingProviderWithoutOpenAIKey(t *testing.T) {
	router, embedder := NewSupportRouter("", "", "", "", "")

	if router != nil {
		t.Fatalf("expected nil router when no providers are configured, got %#v", router)
	}
	if embedder != nil {
		t.Fatalf("expected nil embedding provider when OpenAI is not configured, got %#v", embedder)
	}
}
