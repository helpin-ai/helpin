package llm

import (
	"context"
	"fmt"
	"sort"
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

// ConfiguredChatProviders returns the normalized configured provider names in stable order.
func (r *Router) ConfiguredChatProviders() []string {
	if r == nil {
		return nil
	}
	providers := make([]string, 0, len(r.chatProviders))
	for provider, implementation := range r.chatProviders {
		if implementation != nil {
			providers = append(providers, normalizeProviderName(provider))
		}
	}
	sort.Strings(providers)
	return providers
}

// HasChatProvider reports whether a normalized provider is configured.
func (r *Router) HasChatProvider(provider string) bool {
	if r == nil {
		return false
	}
	implementation := r.chatProviders[normalizeProviderName(provider)]
	return implementation != nil
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
		return nil, &ProviderError{
			Provider: providerName, Operation: "chat_completion", Kind: ProviderErrorUnavailable,
			Message: "chat provider is not configured",
		}
	}
	req.Provider = providerName
	return provider.ChatCompletion(ctx, req)
}

// ResolvePricingIdentity resolves the same provider route used by ChatCompletion.
func (r *Router) ResolvePricingIdentity(req ChatRequest) (ChatPricingIdentity, error) {
	if r == nil {
		return ChatPricingIdentity{}, fmt.Errorf("llm router is nil")
	}
	providerName := normalizeProviderName(req.Provider)
	if providerName == "" {
		providerName = r.defaultChatProvider
	}
	provider, ok := r.chatProviders[providerName]
	if !ok || provider == nil {
		return ChatPricingIdentity{}, &ProviderError{
			Provider: providerName, Operation: "pricing_identity", Kind: ProviderErrorUnavailable,
			Message: "chat provider is not configured",
		}
	}
	req.Provider = providerName
	resolver, ok := provider.(PricingIdentityResolver)
	if !ok {
		return ChatPricingIdentity{}, fmt.Errorf("chat provider %q has no pricing identity", providerName)
	}
	identity, err := resolver.ResolvePricingIdentity(req)
	if err != nil {
		return ChatPricingIdentity{}, err
	}
	identity.Provider = providerName
	return identity, nil
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
