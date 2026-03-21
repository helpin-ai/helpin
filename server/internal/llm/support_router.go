package llm

import "strings"

const openRouterDefaultBaseURL = "https://openrouter.ai/api/v1"

// NewSupportRouter builds a chat router for support agents plus a separate embedding provider.
func NewSupportRouter(
	anthropicAPIKey string,
	openAIAPIKey string,
	openAIBaseURL string,
	openRouterAPIKey string,
	openRouterBaseURL string,
) (*Router, EmbeddingProvider) {
	anthropicProvider := NewClaudeProvider(strings.TrimSpace(anthropicAPIKey))
	openAIProvider := NewOpenAIProvider(strings.TrimSpace(openAIAPIKey), strings.TrimSpace(openAIBaseURL), "")

	openRouterBaseURL = strings.TrimSpace(openRouterBaseURL)
	if openRouterBaseURL == "" {
		openRouterBaseURL = openRouterDefaultBaseURL
	}
	openRouterProvider := NewOpenAIProvider(strings.TrimSpace(openRouterAPIKey), openRouterBaseURL, "")

	chatProviders := map[string]Provider{}
	if anthropicProvider != nil {
		chatProviders["anthropic"] = anthropicProvider
	}
	if openAIProvider != nil {
		chatProviders["openai"] = openAIProvider
	}
	if openRouterProvider != nil {
		chatProviders["openrouter"] = openRouterProvider
	}
	if len(chatProviders) == 0 {
		return nil, openAIProvider
	}

	defaultChatProvider := "anthropic"
	switch {
	case chatProviders["anthropic"] != nil:
	case chatProviders["openai"] != nil:
		defaultChatProvider = "openai"
	default:
		defaultChatProvider = "openrouter"
	}

	embeddingProviders := map[string]EmbeddingProvider{}
	defaultEmbeddingProvider := ""
	if openAIProvider != nil {
		embeddingProviders["openai"] = openAIProvider
		defaultEmbeddingProvider = "openai"
	}

	return NewRouter(defaultChatProvider, chatProviders, defaultEmbeddingProvider, embeddingProviders), openAIProvider
}
