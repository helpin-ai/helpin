package llm

import (
	"context"
	"fmt"
	"strings"
)

// Router dispatches chat and embedding requests to provider-specific implementations.
type Router struct {
	defaultChatProvider      string
	defaultEmbeddingProvider string
	chatProviders            map[string]Provider
	embeddingProviders       map[string]EmbeddingProvider
}

// NewRouter creates a new LLM router.
func NewRouter(
	defaultChatProvider string,
	chatProviders map[string]Provider,
	defaultEmbeddingProvider string,
	embeddingProviders map[string]EmbeddingProvider,
) *Router {
	return &Router{
		defaultChatProvider:      normalizeProviderName(defaultChatProvider),
		defaultEmbeddingProvider: normalizeProviderName(defaultEmbeddingProvider),
		chatProviders:            chatProviders,
		embeddingProviders:       embeddingProviders,
	}
}

// ChatCompletion routes a chat completion request to the requested provider.
func (r *Router) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if r == nil {
		return nil, fmt.Errorf("llm router is nil")
	}
	providerName := normalizeProviderName(req.Provider)
	if providerName == "" {
		providerName = r.defaultChatProvider
	}
	provider, ok := r.chatProviders[providerName]
	if !ok || provider == nil {
		return nil, fmt.Errorf("chat provider %q is not configured", providerName)
	}
	req.Provider = providerName
	return provider.ChatCompletion(ctx, req)
}

// CreateEmbeddings routes an embedding request to the requested provider.
func (r *Router) CreateEmbeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if r == nil {
		return nil, fmt.Errorf("llm router is nil")
	}
	providerName := normalizeProviderName(req.Provider)
	if providerName == "" {
		providerName = r.defaultEmbeddingProvider
	}
	provider, ok := r.embeddingProviders[providerName]
	if !ok || provider == nil {
		return nil, fmt.Errorf("embedding provider %q is not configured", providerName)
	}
	req.Provider = providerName
	return provider.CreateEmbeddings(ctx, req)
}

func normalizeProviderName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "":
		return ""
	case "anthropic", "claude", "claude_direct":
		return "anthropic"
	case "openai", "openai_direct":
		return "openai"
	case "openrouter", "openrouter-responses":
		return "openrouter"
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}
