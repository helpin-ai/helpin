package llm

import (
	"context"
	"encoding/json"
	"testing"
)

func TestOpenAIProviderBuildChatCompletionBodyOmitsTemperatureForGPT5(t *testing.T) {
	models := []string{"gpt-5.6-luna", "gpt-5.6-terra"}

	for _, modelName := range models {
		t.Run(modelName, func(t *testing.T) {
			provider := NewOpenAIProvider("test-key", "https://api.openai.com/v1", "")
			body := provider.buildChatCompletionBody(
				modelName,
				[]map[string]any{{"role": "user", "content": "hello"}},
				123,
				0.1,
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
			if _, ok := body["response_format"]; !ok {
				t.Fatalf("expected JSON mode response_format in request body, got %#v", body)
			}
		})
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

func TestOpenAIProviderCreateEmbeddingsNilReceiver(t *testing.T) {
	var provider *OpenAIProvider

	_, err := provider.CreateEmbeddings(context.Background(), EmbeddingRequest{
		Inputs: []string{"hello"},
	})
	if err == nil {
		t.Fatal("expected nil provider error")
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
