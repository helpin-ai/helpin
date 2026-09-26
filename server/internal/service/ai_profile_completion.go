package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrWorkspaceAIUnavailable means the workspace has no AI profile that a
// direct completion can run on: no default, no usable connection, or a
// provider that cannot be called directly.
var ErrWorkspaceAIUnavailable = errors.New("workspace AI is unavailable")

// AIChatClientFactory builds the chat client for a resolved profile route from
// its connection credential. Tests replace it with a fake provider.
type AIChatClientFactory func(route sdk.RunModel, credential sdk.ModelCredential) (llm.Provider, error)

// AIProfileExecution is an admitted workspace AI profile selection with a chat
// client bound to the selected connection's credential. It is request-scoped
// and must never be persisted, because the client holds the credential.
type AIProfileExecution struct {
	Selection *model.AIExecutionSelection
	Client    llm.Provider
}

// SetChatClientFactory replaces the clients built by ResolveChatExecution.
func (s *AIProfileService) SetChatClientFactory(factory AIChatClientFactory) *AIProfileService {
	s.chatClients = factory
	return s
}

// ResolveChatExecution resolves the workspace default AI profile for an active
// member and binds a direct chat client to its connection. Every failure wraps
// ErrWorkspaceAIUnavailable so callers can offer a manual alternative.
func (s *AIProfileService) ResolveChatExecution(ctx context.Context, workspace, user string) (*AIProfileExecution, error) {
	if s == nil || s.connections == nil || !s.connections.Enabled() {
		return nil, fmt.Errorf("%w: AI connections are not enabled", ErrWorkspaceAIUnavailable)
	}
	selection, credential, err := s.Resolve(ctx, workspace, user, AIProfileSelectionRequest{Direct: true})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorkspaceAIUnavailable, err)
	}
	factory := s.chatClients
	if factory == nil {
		factory = defaultAIChatClient
	}
	client, err := factory(selection.Route.Model, *credential)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorkspaceAIUnavailable, err)
	}
	return &AIProfileExecution{Selection: selection, Client: client}, nil
}

// defaultAIChatClient supports the providers that expose a Chat Completions or
// Messages API to a plain API key. ChatGPT subscriptions run only through Agent
// Runtime and are reported as unsupported.
func defaultAIChatClient(route sdk.RunModel, credential sdk.ModelCredential) (llm.Provider, error) {
	var client llm.Provider
	switch route.Provider {
	case "openai":
		if p := llm.NewOpenAIProvider(credential.APIKey, "", ""); p != nil {
			client = p
		}
	case "openrouter", "openrouter_responses":
		if p := llm.NewOpenAIProvider(credential.APIKey, llm.OpenRouterDefaultBaseURL, ""); p != nil {
			client = p
		}
	case "anthropic":
		if p := llm.NewClaudeProvider(credential.APIKey); p != nil {
			client = p
		}
	case "openai_compatible":
		if route.Endpoint == nil {
			break
		}
		key := credential.APIKey
		if route.Endpoint.AuthMode == "none" {
			key = ""
		}
		if p := llm.NewOpenAICompatibleProvider(key, route.Endpoint.BaseURL); p != nil {
			client = p
		}
	}
	if client == nil {
		return nil, fmt.Errorf("provider %q cannot run direct completions", route.Provider)
	}
	return client, nil
}

// CompleteWithProfile executes one feature-owned completion on an admitted
// workspace AI profile. Usage is metered with the funding policy frozen on the
// selection (Community: unbilled; Enterprise: managed or flat BYOK), and the
// execution is audited under the feature's action like catalogued routes.
func (s *AICompletionService) CompleteWithProfile(ctx context.Context, input AICompletionRequest, execution *AIProfileExecution) (*llm.ChatResponse, error) {
	if s == nil || s.usage == nil {
		return nil, fmt.Errorf("AI completion service is not configured")
	}
	if execution == nil || execution.Selection == nil || execution.Client == nil {
		return nil, fmt.Errorf("AI profile execution is required")
	}
	if strings.TrimSpace(input.FeatureKey) == "" || strings.TrimSpace(input.IdempotencyKey) == "" {
		return nil, fmt.Errorf("AI completion feature and idempotency key are required")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return nil, fmt.Errorf("AI completion workspace is required")
	}
	if strings.TrimSpace(input.Chat.Provider) != "" || strings.TrimSpace(input.Chat.Model) != "" {
		return nil, fmt.Errorf("AI completion technical route must be supplied through the AI profile")
	}
	policy, ok := s.routes.Policy(input.FeatureKey, input.OperationKey)
	if !ok {
		return nil, fmt.Errorf("%w: no route policy for feature %q operation %q", model.ErrPricingConfigurationMissing, input.FeatureKey, input.OperationKey)
	}
	if input.Chat.MaxTokens <= 0 {
		input.Chat.MaxTokens = policy.MaximumOutputTokens
	}
	if input.Chat.MaxTokens > policy.MaximumOutputTokens {
		return nil, fmt.Errorf("AI completion output ceiling %d exceeds feature maximum %d", input.Chat.MaxTokens, policy.MaximumOutputTokens)
	}
	selected := execution.Selection.Route.Model
	chat, err := profileChatRequest(input.Chat, selected)
	if err != nil {
		return nil, err
	}
	funding, flat := aiusage.FundingHelpinHosted, (*aiusage.FlatTokenTariff)(nil)
	if p := execution.Selection.Policy; p != nil && p.FundingMode != "" {
		funding, flat = p.FundingMode, p.FlatTariff
	}
	route := AICompletionRoute{Provider: selected.Provider, Model: selected.Model, ServiceTier: defaultAICompletionServiceTier}
	response, _, err := s.runMeteredAttempt(ctx, input, meteredCompletionAttempt{
		client: execution.Client, chat: chat, route: route,
		// The profile route was admitted by the workspace AI policy; the action
		// policy still validates the feature identity and records the audit.
		policyRoute: aipolicy.Route{},
		endpoint:    selected.Endpoint,
		key:         fmt.Sprintf("%s:profile:%s", input.IdempotencyKey, aiUsageStableHash(execution.Selection.ProfileID+"|"+selected.Provider+"|"+selected.Model)),
		funding:     funding, flatTariff: flat,
	})
	if err != nil {
		return nil, fmt.Errorf("AI profile completion failed: %w", err)
	}
	return response, nil
}

// FeatureAvailable reports whether a catalogued route for the feature has a
// configured server provider. Providers that cannot report configuration are
// assumed available.
func (s *AICompletionService) FeatureAvailable(feature, operation string) bool {
	if s == nil || s.provider == nil || s.usage == nil {
		return false
	}
	policy, ok := s.routes.Policy(feature, operation)
	if !ok {
		return false
	}
	configured, reports := s.provider.(interface{ HasChatProvider(string) bool })
	if !reports {
		return true
	}
	for _, route := range completionCandidateRoutes(policy, nil) {
		if configured.HasChatProvider(route.Provider) {
			return true
		}
	}
	return false
}

// profileChatRequest applies the profile route, including OpenRouter
// quantization controls, to a feature chat request.
func profileChatRequest(input llm.ChatRequest, route sdk.RunModel) (llm.ChatRequest, error) {
	input.Provider, input.Model = route.Provider, route.Model
	if normalizeCompletionRouteProvider(route.Provider) != "openrouter" ||
		route.Controls == nil || route.Controls.OpenRouter == nil || route.Controls.OpenRouter.Provider == nil ||
		len(route.Controls.OpenRouter.Provider.Quantizations) == 0 {
		return input, nil
	}
	options := map[string]any{}
	if len(input.ProviderOptions) > 0 {
		if err := json.Unmarshal(input.ProviderOptions, &options); err != nil || options == nil {
			return llm.ChatRequest{}, fmt.Errorf("completion provider options must be a JSON object")
		}
	}
	options["quantizations"] = route.Controls.OpenRouter.Provider.Quantizations
	raw, err := json.Marshal(options)
	if err != nil {
		return llm.ChatRequest{}, fmt.Errorf("encode completion provider options: %w", err)
	}
	input.ProviderOptions = raw
	return input, nil
}
