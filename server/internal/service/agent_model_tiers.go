package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

var selectableAgentTierRoutes = map[aiusage.Tier]AICompletionRoute{
	aiusage.TierSmall: {
		Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731", ServiceTier: defaultAICompletionServiceTier,
	},
	aiusage.TierMedium: {
		Provider: "openrouter", Model: "google/gemini-3.7-flash", ServiceTier: defaultAICompletionServiceTier,
	},
	aiusage.TierLarge: {
		Provider: "openrouter", Model: "openai/gpt-5.6-terra", ServiceTier: defaultAICompletionServiceTier,
	},
	aiusage.TierFlagship: {
		Provider: "openrouter", Model: "anthropic/claude-sonnet-5", ServiceTier: defaultAICompletionServiceTier,
	},
}

// AgentModelTierSnapshot is the internal execution identity resolved from a public model size.
type AgentModelTierSnapshot struct {
	ModelTier   string
	Provider    string
	Model       string
	ServiceTier string
	RuntimeKind string
}

// AgentModelTierResolver owns the public-tier to internal-route policy.
type AgentModelTierResolver struct {
	catalog     *aiusage.Catalog
	hasProvider func(string) bool
}

func NewAgentModelTierResolver(catalog *aiusage.Catalog, hasProvider func(string) bool) *AgentModelTierResolver {
	return &AgentModelTierResolver{catalog: catalog, hasProvider: hasProvider}
}

// ValidateSelectable verifies every public tier before the process starts serving traffic.
func (r *AgentModelTierResolver) ValidateSelectable() []error {
	ordered := []aiusage.Tier{aiusage.TierSmall, aiusage.TierMedium, aiusage.TierLarge, aiusage.TierFlagship}
	var issues []error
	for _, tier := range ordered {
		route := selectableAgentTierRoutes[tier]
		if r == nil || r.catalog == nil {
			issues = append(issues, fmt.Errorf("agent model tier %q has no pricing catalog", tier))
			continue
		}
		if r.hasProvider != nil && !r.hasProvider(route.Provider) {
			issues = append(issues, fmt.Errorf("agent model tier %q provider %q is not configured", tier, route.Provider))
			continue
		}
		resolved, err := r.catalog.Resolve(route.Provider, route.Model, route.Model, route.ServiceTier)
		if err != nil {
			issues = append(issues, fmt.Errorf("agent model tier %q: %w", tier, err))
			continue
		}
		if resolved.Tier != tier {
			issues = append(issues, fmt.Errorf("agent model tier %q resolves as %q", tier, resolved.Tier))
		}
	}
	return issues
}

// ResolveCustom validates a selectable tier and returns its immutable execution snapshot.
func (r *AgentModelTierResolver) ResolveCustom(tier aiusage.Tier, allowedTargets, allowedTools []string, preserveRuntime string) (AgentModelTierSnapshot, error) {
	route, ok := selectableAgentTierRoutes[tier]
	if !ok || r == nil || r.catalog == nil {
		return AgentModelTierSnapshot{}, fmt.Errorf("model size temporarily unavailable")
	}
	if r.hasProvider != nil && !r.hasProvider(route.Provider) {
		return AgentModelTierSnapshot{}, fmt.Errorf("model size temporarily unavailable")
	}
	resolved, err := r.catalog.Resolve(route.Provider, route.Model, route.Model, route.ServiceTier)
	if err != nil || resolved.Tier != tier {
		return AgentModelTierSnapshot{}, fmt.Errorf("model size temporarily unavailable")
	}
	runtimeKind := strings.TrimSpace(preserveRuntime)
	if runtimeKind == "" {
		runtimeKind = runtimeForCustomAgentCapabilities(allowedTargets, allowedTools)
	}
	return AgentModelTierSnapshot{
		ModelTier: string(tier), Provider: route.Provider, Model: route.Model,
		ServiceTier: route.ServiceTier, RuntimeKind: runtimeKind,
	}, nil
}

// Derive returns the public tier for an existing exact execution route.
func (r *AgentModelTierResolver) Derive(provider, model, serviceTier string) (aiusage.Tier, error) {
	if r == nil || r.catalog == nil {
		return "", fmt.Errorf("model size temporarily unavailable")
	}
	resolved, err := r.catalog.ResolveDefault(provider, model, serviceTier)
	if err != nil {
		return "", err
	}
	return resolved.Tier, nil
}

func runtimeForCustomAgentCapabilities(targets, tools []string) string {
	for _, target := range targets {
		switch strings.ToLower(strings.TrimSpace(target)) {
		case "repository", "pull_request", "git_repository", "github_pull_request":
			return "codex"
		}
	}
	for _, tool := range tools {
		switch strings.ToLower(strings.TrimSpace(tool)) {
		case "shell", "bash", "apply_patch", "git", "read_file", "write_file", "edit_file":
			return "codex"
		}
	}
	return "native_sdk"
}

var (
	defaultAgentTierResolverOnce sync.Once
	defaultAgentTierResolver     *AgentModelTierResolver
)

func loadDefaultAgentModelTierResolver() *AgentModelTierResolver {
	defaultAgentTierResolverOnce.Do(func() {
		catalog, err := aiusage.LoadCatalog()
		if err == nil {
			defaultAgentTierResolver = NewAgentModelTierResolver(catalog, nil)
		}
	})
	return defaultAgentTierResolver
}

func deriveAgentModelTier(provider, modelName *string, executionConfig model.JSONBlob) string {
	resolver := loadDefaultAgentModelTierResolver()
	if resolver == nil || provider == nil || modelName == nil {
		return ""
	}
	serviceTier := defaultAICompletionServiceTier
	var config model.AgentExecutionConfig
	if len(executionConfig) > 0 && json.Unmarshal(executionConfig, &config) == nil && config.ServiceTier != nil {
		if value := strings.TrimSpace(*config.ServiceTier); value != "" {
			serviceTier = value
		}
	}
	tier, err := resolver.Derive(*provider, *modelName, serviceTier)
	if err != nil {
		return ""
	}
	return string(tier)
}
