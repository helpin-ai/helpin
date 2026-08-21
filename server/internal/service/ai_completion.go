package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// AICompleter is the mandatory product boundary for direct LLM completions.
type AICompleter interface {
	Complete(context.Context, AICompletionRequest) (*llm.ChatResponse, error)
}

// AICompletionRequest carries product, pricing, routing, and prompt identity explicitly.
type AICompletionRequest struct {
	WorkspaceID        string
	FeatureKey         string
	OperationKey       string
	IdempotencyKey     string
	Metadata           map[string]interface{}
	PreferredRoute     *AICompletionRoute
	RequireComplete    bool
	RetryInvalidOutput bool
	ValidateResponse   func(*llm.ChatResponse) error
	Chat               llm.ChatRequest
}

// AICompletionService routes, meters, executes, and reconciles direct completions.
type AICompletionService struct {
	provider llm.Provider
	usage    *AIUsageService
	routes   AICompletionRouteRegistry
}

// NewAICompletionService creates the single direct-completion execution boundary.
func NewAICompletionService(
	provider llm.Provider,
	usage *AIUsageService,
	routes AICompletionRouteRegistry,
) *AICompletionService {
	return &AICompletionService{provider: provider, usage: usage, routes: routes}
}

// ChatCompletion exists only so AICompletionService can replace legacy llm.Provider
// constructor dependencies during migration. Product code must call Complete.
func (s *AICompletionService) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, fmt.Errorf("direct ChatCompletion is disabled; use AICompleter.Complete")
}

// Complete executes one feature-owned completion using only catalogued routes.
func (s *AICompletionService) Complete(ctx context.Context, input AICompletionRequest) (*llm.ChatResponse, error) {
	if s == nil || s.provider == nil || s.usage == nil {
		return nil, fmt.Errorf("AI completion service is not configured")
	}
	if strings.TrimSpace(input.FeatureKey) == "" || strings.TrimSpace(input.IdempotencyKey) == "" {
		return nil, fmt.Errorf("AI completion feature and idempotency key are required")
	}
	feature, known := AIUsageFeature(input.FeatureKey)
	if strings.TrimSpace(input.WorkspaceID) == "" && (!known || feature.Chargeable) {
		return nil, fmt.Errorf("AI completion workspace is required")
	}
	if strings.TrimSpace(input.Chat.Provider) != "" || strings.TrimSpace(input.Chat.Model) != "" {
		return nil, fmt.Errorf("AI completion technical route must be supplied through routing policy")
	}
	policy, ok := s.routes.Policy(input.FeatureKey, input.OperationKey)
	if !ok {
		return nil, fmt.Errorf("%w: no route policy for feature %q operation %q", model.ErrPricingConfigurationMissing, input.FeatureKey, input.OperationKey)
	}
	if input.Chat.MaxTokens <= 0 {
		input.Chat.MaxTokens = policy.MaximumOutputTokens
	}
	if input.Chat.MaxTokens > policy.MaximumOutputTokens {
		return nil, fmt.Errorf(
			"AI completion output ceiling %d exceeds feature maximum %d",
			input.Chat.MaxTokens, policy.MaximumOutputTokens,
		)
	}

	routes := completionCandidateRoutes(policy, input.PreferredRoute)
	var attemptErrors []error
	for index, route := range routes {
		response, retry, err := s.completeAttempt(ctx, input, policy, route, index)
		if err == nil {
			return response, nil
		}
		attemptErrors = append(attemptErrors, err)
		if !retry || index == len(routes)-1 {
			break
		}
	}
	return nil, fmt.Errorf("AI completion failed: %w", errors.Join(attemptErrors...))
}

func completeAI(ctx context.Context, provider llm.Provider, input AICompletionRequest) (*llm.ChatResponse, error) {
	if completer, ok := provider.(AICompleter); ok {
		return completer.Complete(ctx, input)
	}
	if provider == nil {
		return nil, fmt.Errorf("AI completion client is not configured")
	}
	// Legacy provider support keeps focused service tests isolated. Production
	// dependency injection supplies AICompletionService and always takes the
	// typed path above.
	policy, ok := DefaultAICompletionRouteRegistry().Policy(input.FeatureKey, input.OperationKey)
	if !ok {
		return nil, fmt.Errorf("no AI completion route for feature %q", input.FeatureKey)
	}
	routes := completionCandidateRoutes(policy, input.PreferredRoute)
	if len(routes) == 0 {
		return nil, fmt.Errorf("no AI completion route for feature %q", input.FeatureKey)
	}
	var attemptErrors []error
	for index, route := range routes {
		chat := input.Chat
		chat.Provider = route.Provider
		chat.Model = route.Model
		legacyCtx := WithAIUsageMetering(ctx, AIUsageMeteringContext{
			WorkspaceID: input.WorkspaceID, FeatureKey: input.FeatureKey, OperationKey: input.OperationKey,
			IdempotencyKey: fmt.Sprintf("%s:route:%d", input.IdempotencyKey, index), Metadata: input.Metadata,
		})
		response, err := provider.ChatCompletion(legacyCtx, chat)
		if err == nil && response == nil {
			err = fmt.Errorf("provider returned no response")
		}
		if err == nil && input.RequireComplete && isIncompleteFinishReason(response.FinishReason) {
			err = fmt.Errorf("model output was incomplete (finish_reason=%s)", response.FinishReason)
		}
		if err == nil && input.ValidateResponse != nil {
			err = input.ValidateResponse(response)
		}
		if err == nil {
			return response, nil
		}
		attemptErrors = append(attemptErrors, err)
		retry := llm.IsRetryableProviderError(err) || input.RetryInvalidOutput
		if !retry || index == len(routes)-1 {
			break
		}
	}
	return nil, fmt.Errorf("AI completion failed: %w", errors.Join(attemptErrors...))
}

func (s *AICompletionService) completeAttempt(
	ctx context.Context,
	input AICompletionRequest,
	policy AICompletionRoutePolicy,
	route AICompletionRoute,
	index int,
) (*llm.ChatResponse, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	attemptKey := fmt.Sprintf("%s:route:%d:%s", input.IdempotencyKey, index, aiUsageStableHash(aiCompletionRouteKey(route)))
	feature, known := AIUsageFeature(input.FeatureKey)
	promotional := known && !feature.Chargeable
	preflight, err := s.usage.Preflight(ctx, PreflightRequest{Metering: MeteringRequest{
		WorkspaceID: input.WorkspaceID, TaskNature: taskNatureForFeature(input.FeatureKey),
		FeatureKey: input.FeatureKey, OperationKey: input.OperationKey,
		Provider: route.Provider, Model: route.Model, Route: route.Model, ServiceTier: route.ServiceTier,
		FundingMode: aiusage.FundingHelpinHosted, InputTokensEstimate: estimateChatInputTokens(input.Chat),
		MaximumOutputTokens: int64(input.Chat.MaxTokens), ExecutionID: metadataString(input.Metadata, "execution_id"),
		IdempotencyKey: attemptKey, Promotional: promotional,
	}})
	if err != nil {
		retry := input.PreferredRoute != nil && policy.PreferRequestRoute && errors.Is(err, model.ErrModelUnavailableUnderPricing)
		return nil, retry, err
	}

	chat := input.Chat
	chat.Provider = route.Provider
	chat.Model = route.Model
	response, providerErr := s.provider.ChatCompletion(ctx, chat)
	if providerErr != nil || response == nil {
		if preflight.ReservationID != "" {
			_ = s.usage.Fail(ctx, preflight.ReservationID)
		}
		if providerErr == nil {
			providerErr = &llm.ProviderError{
				Provider: route.Provider, Operation: "chat_completion", Kind: llm.ProviderErrorUnavailable,
				Message: "provider returned no response",
			}
		}
		return nil, llm.IsRetryableProviderError(providerErr), providerErr
	}

	_, reconcileErr := s.usage.Reconcile(ctx, CompletionUsage{
		Context: *preflight,
		Telemetry: aiusage.TokenTelemetry{
			InputTokensTotal:            int64(response.TokensUsed.InputTokensTotal),
			CacheReadTokens:             int64(response.TokensUsed.CacheReadTokens),
			CacheWriteTokens:            int64(response.TokensUsed.CacheWriteTokens),
			CompletionTokensTotal:       int64(response.TokensUsed.CompletionTokensTotal),
			OutputTokens:                int64(response.TokensUsed.OutputTokens),
			ReasoningTokens:             int64(response.TokensUsed.ReasoningTokens),
			CompletionIncludesReasoning: response.TokensUsed.CompletionIncludesReasoning,
		},
		MeasurementStatus: measurementStatus(response.TokensUsed),
	})
	if reconcileErr != nil {
		slog.ErrorContext(ctx, "AI completion usage reconciliation failed",
			"workspace_id", input.WorkspaceID, "feature_key", input.FeatureKey, "error", reconcileErr)
	}

	if input.RequireComplete && isIncompleteFinishReason(response.FinishReason) {
		return nil, true, fmt.Errorf("model output was incomplete (finish_reason=%s)", response.FinishReason)
	}
	if input.ValidateResponse != nil {
		if err := input.ValidateResponse(response); err != nil {
			return nil, input.RetryInvalidOutput, err
		}
	}
	return response, false, nil
}

func completionCandidateRoutes(policy AICompletionRoutePolicy, preferred *AICompletionRoute) []AICompletionRoute {
	routes := make([]AICompletionRoute, 0, 2+len(policy.Fallbacks))
	if preferred != nil && policy.PreferRequestRoute {
		routes = append(routes, *preferred)
	}
	routes = append(routes, policy.Primary)
	routes = append(routes, policy.Fallbacks...)
	seen := make(map[string]struct{}, len(routes))
	unique := routes[:0]
	for _, route := range routes {
		key := aiCompletionRouteKey(route)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, route)
	}
	return unique
}

func isIncompleteFinishReason(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "length", "max_tokens", "max_output_tokens":
		return true
	default:
		return false
	}
}
