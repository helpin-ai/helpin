package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProviderChatCompletionPreservesFinishReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"finish_reason":"length","message":{"content":""}}],
			"usage":{
				"prompt_tokens":13522,
				"completion_tokens":7696,
				"completion_tokens_details":{"reasoning_tokens":7696}
			}
		}`))
	}))
	t.Cleanup(server.Close)

	provider := NewOpenAIProvider("test-key", server.URL, "")
	response, err := provider.ChatCompletion(context.Background(), ChatRequest{
		Provider:  "openrouter",
		Model:     "deepseek/deepseek-v4-flash-0731",
		MaxTokens: 8192,
		Reasoning: &ReasoningConfig{Effort: "low"},
	})
	if err != nil {
		t.Fatalf("chat completion: %v", err)
	}
	if response.FinishReason != "length" {
		t.Fatalf("finish reason = %q", response.FinishReason)
	}
	if response.Content != "" || response.TokensUsed.ReasoningTokens != 7696 || response.TokensUsed.OutputTokens != 0 {
		t.Fatalf("response = %#v", response)
	}
}
