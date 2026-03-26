package llm

import "testing"

func TestOpenAIProviderBuildChatCompletionBodyUsesMaxCompletionTokensForGPT5(t *testing.T) {
	provider := NewOpenAIProvider("test-key", "https://api.openai.com/v1", "")
	body := provider.buildChatCompletionBody(
		"gpt-5.4-mini",
		[]map[string]any{{"role": "user", "content": "hello"}},
		123,
		0.1,
		true,
	)

	if _, ok := body["max_completion_tokens"]; !ok {
		t.Fatalf("expected max_completion_tokens in request body, got %#v", body)
	}
	if _, ok := body["max_tokens"]; ok {
		t.Fatalf("did not expect max_tokens for GPT-5 request body, got %#v", body)
	}
	if _, ok := body["response_format"]; !ok {
		t.Fatalf("expected JSON mode response_format in request body, got %#v", body)
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
	)

	if _, ok := body["max_tokens"]; !ok {
		t.Fatalf("expected max_tokens in request body, got %#v", body)
	}
	if _, ok := body["max_completion_tokens"]; ok {
		t.Fatalf("did not expect max_completion_tokens for non-GPT-5 request body, got %#v", body)
	}
	if _, ok := body["response_format"]; ok {
		t.Fatalf("did not expect response_format when JSON mode is off, got %#v", body)
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
