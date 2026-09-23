package llm

import (
	"context"
	"strings"
)

const openRouterDefaultBaseURL = "https://openrouter.ai/api/v1"

// OpenRouterDefaultBaseURL is the public OpenRouter OpenAI-compatible API root.
const OpenRouterDefaultBaseURL = openRouterDefaultBaseURL

// DefaultOpenRouterEmbeddingModel keeps the 1536-dimension vectors produced by
// OpenAI's text-embedding-3-small when embeddings are served through OpenRouter.
const DefaultOpenRouterEmbeddingModel = "openai/text-embedding-3-small"

// NewSupportRouter builds a chat router for support agents plus a separate embedding provider.
// Embeddings use OpenAI when its key is configured; otherwise OpenRouter's
// OpenAI-compatible embeddings endpoint serves them when its key is configured.
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

	var embeddingProvider EmbeddingProvider
	embeddingProviders := map[string]EmbeddingProvider{}
	defaultEmbeddingProvider := ""
	switch {
	case openAIProvider != nil:
		embeddingProvider = openAIProvider
		embeddingProviders["openai"] = openAIProvider
		defaultEmbeddingProvider = "openai"
	case openRouterProvider != nil:
		openRouterEmbeddings := &openRouterEmbeddingProvider{base: openRouterProvider}
		embeddingProvider = openRouterEmbeddings
		embeddingProviders["openrouter"] = openRouterEmbeddings
		defaultEmbeddingProvider = "openrouter"
	}

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
		return nil, embeddingProvider
	}

	defaultChatProvider := "anthropic"
	switch {
	case chatProviders["anthropic"] != nil:
	case chatProviders["openai"] != nil:
		defaultChatProvider = "openai"
	default:
		defaultChatProvider = "openrouter"
	}

	return NewRouter(defaultChatProvider, chatProviders, defaultEmbeddingProvider, embeddingProviders), embeddingProvider
}

// SupportEmbeddingModel reports the embedding model the support router uses
// for a configured model name, or "" when no embedding provider is configured.
func SupportEmbeddingModel(openAIAPIKey, openRouterAPIKey, configured string) string {
	switch {
	case strings.TrimSpace(openAIAPIKey) != "":
		if model := strings.TrimSpace(configured); model != "" {
			return model
		}
		return "text-embedding-3-small"
	case strings.TrimSpace(openRouterAPIKey) != "":
		return openRouterEmbeddingModel(configured)
	default:
		return ""
	}
}

// SupportEmbeddingProviderName reports which server key serves embeddings:
// "openai", "openrouter", or "" when neither key is configured.
func SupportEmbeddingProviderName(openAIAPIKey, openRouterAPIKey string) string {
	switch {
	case strings.TrimSpace(openAIAPIKey) != "":
		return "openai"
	case strings.TrimSpace(openRouterAPIKey) != "":
		return "openrouter"
	default:
		return ""
	}
}

// NewOpenRouterEmbeddingProvider returns an OpenRouter embeddings client that
// accepts OpenAI embedding model names, or nil when apiKey is empty. An empty
// baseURL uses the public OpenRouter API.
func NewOpenRouterEmbeddingProvider(apiKey, baseURL string) EmbeddingProvider {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = openRouterDefaultBaseURL
	}
	base := NewOpenAIProvider(strings.TrimSpace(apiKey), baseURL, "")
	if base == nil {
		return nil
	}
	return &openRouterEmbeddingProvider{base: base}
}

// openRouterEmbeddingProvider maps OpenAI embedding model names to OpenRouter's
// vendor-prefixed identifiers so callers can keep passing OPENAI_EMBEDDING_MODEL.
type openRouterEmbeddingProvider struct {
	base *OpenAIProvider
}

func (p *openRouterEmbeddingProvider) CreateEmbeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	req.Model = openRouterEmbeddingModel(req.Model)
	return p.base.CreateEmbeddings(ctx, req)
}

func openRouterEmbeddingModel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultOpenRouterEmbeddingModel
	}
	if !strings.Contains(name, "/") {
		return "openai/" + name
	}
	return name
}

var _ EmbeddingProvider = (*openRouterEmbeddingProvider)(nil)
