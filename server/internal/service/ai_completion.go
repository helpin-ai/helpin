package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/observability"
)

// AICompleter is the mandatory product boundary for direct LLM completions.
type AICompleter interface {
	Complete(context.Context, AICompletionRequest) (*llm.ChatResponse, error)
}

// AICompletionRequest carries product, pricing, routing, and prompt identity explicitly.
type AICompletionRequest struct {
	WorkspaceID        string
	ActionKey          string
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
	metrics  *observability.Metrics
	provider llm.Provider
	usage    AIUsageLifecycle
	routes   AICompletionRouteRegistry
	policy   *aipolicy.Registry
	audit    aipolicy.ExecutionAudit
}

func (s *AICompletionService) SetMetrics(metrics *observability.Metrics) *AICompletionService {
	s.metrics = metrics
	return s
}

// SetGovernance enables app-wide action policy and execution auditing.
func (s *AICompletionService) SetGovernance(registry *aipolicy.Registry, audit aipolicy.ExecutionAudit) *AICompletionService {
	if s == nil {
		return nil
	}
	s.policy = registry
	s.audit = audit
	return s
}

// NewAICompletionService creates the single direct-completion execution boundary.
func NewAICompletionService(
	provider llm.Provider,
	usage AIUsageLifecycle,
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
		chat, routeErr := completionChatRequest(input.Chat, route)
		if routeErr != nil {
			return nil, routeErr
		}
		legacyCtx := WithAIUsageMetering(ctx, AIUsageMeteringContext{
			WorkspaceID: input.WorkspaceID, ActionKey: input.ActionKey,
			FeatureKey: input.FeatureKey, OperationKey: input.OperationKey,
			IdempotencyKey: fmt.Sprintf("%s:route:%d", input.IdempotencyKey, index), Metadata: input.Metadata,
		})
		attemptCtx, cancelAttempt := completionAttemptContext(legacyCtx, input.FeatureKey)
		response, err := provider.ChatCompletion(attemptCtx, chat)
		cancelAttempt()
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
		retry := llm.IsRetryableProviderError(err) || input.RetryInvalidOutput || (errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil)
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
	chat, err := completionChatRequest(input.Chat, route)
	if err != nil {
		return nil, false, err
	}
	attemptKey := fmt.Sprintf("%s:route:%d:%s", input.IdempotencyKey, index, aiUsageStableHash(aiCompletionRouteKey(route)))
	var auditExecution *model.AIActionExecution
	if s.policy != nil {
		action, policyErr := aipolicy.ResolveExecution(s.policy, aipolicy.ExecutionContext{
			WorkspaceID: input.WorkspaceID, ActionKey: input.ActionKey,
			FeatureKey: input.FeatureKey, IdempotencyKey: input.IdempotencyKey,
			Attempt: index + 1, Metadata: input.Metadata,
		}, aipolicy.Route{Provider: route.Provider, Model: route.Model})
		if policyErr != nil {
			return nil, false, policyErr
		}
		if s.audit != nil {
			auditExecution, policyErr = s.audit.Start(ctx, &model.AIActionExecution{
				WorkspaceID: input.WorkspaceID, ActionKey: action.Key,
				PolicyVersion: action.PolicyVersion, FeatureKey: action.FeatureKey,
				Category: string(action.Category), Origin: action.Origin,
				Modality: string(action.Modality), Provider: route.Provider, Model: route.Model,
				IdempotencyKey: attemptKey, Attempt: index + 1,
				Status: model.AIActionExecutionRunning, Metadata: mustJSONMetadata(input.Metadata),
				StartedAt: time.Now().UTC(),
			})
			if policyErr != nil {
				return nil, false, fmt.Errorf("start AI completion audit: %w", policyErr)
			}
		}
	}
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
		s.finishCompletionAudit(ctx, auditExecution, nil, "llm_preflight", err)
		retry := input.PreferredRoute != nil && policy.PreferRequestRoute && errors.Is(err, model.ErrModelUnavailableUnderPricing)
		return nil, retry, err
	}

	attemptStart := time.Now()
	attemptCtx, cancelAttempt := completionAttemptContext(ctx, input.FeatureKey)
	response, providerErr := s.provider.ChatCompletion(attemptCtx, chat)
	cancelAttempt()
	attemptDuration := time.Since(attemptStart)
	attemptOutcome := "provider_error"
	var measured *aiusage.TokenTelemetry
	defer func() {
		if input.FeatureKey == BillingFeatureSupportTranslation {
			stage := "translation"
			if input.OperationKey == "quality_review" {
				stage = "sample_review"
			}
			s.metrics.TranslationAttempt(stage, route.Provider, route.Model, attemptOutcome, attemptDuration, measured, preflight.Route.Rates)
		}
	}()
	if response != nil && measurementStatus(response.TokensUsed) == "actual" {
		u := response.TokensUsed
		measured = &aiusage.TokenTelemetry{InputTokensTotal: int64(u.InputTokensTotal), CacheReadTokens: int64(u.CacheReadTokens), CacheWriteTokens: int64(u.CacheWriteTokens), CompletionTokensTotal: int64(u.CompletionTokensTotal), OutputTokens: int64(u.OutputTokens), ReasoningTokens: int64(u.ReasoningTokens), CompletionIncludesReasoning: u.CompletionIncludesReasoning}
	}

	// Finish metering and the audit even when an interactive request times out
	// or its caller disconnects. Cleanup itself must remain bounded.
	settlementCtx, cancelSettlement := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelSettlement()
	if providerErr != nil || response == nil {
		if preflight.ReservationID != "" {
			if err := s.usage.Fail(settlementCtx, preflight.ReservationID); err != nil {
				slog.ErrorContext(settlementCtx, "release failed AI completion reservation", "reservation_id", preflight.ReservationID, "error", err)
			}
		}
		if providerErr == nil {
			providerErr = &llm.ProviderError{
				Provider: route.Provider, Operation: "chat_completion", Kind: llm.ProviderErrorUnavailable,
				Message: "provider returned no response",
			}
		}
		s.finishCompletionAudit(settlementCtx, auditExecution, response, "llm_provider", providerErr)
		return nil, llm.IsRetryableProviderError(providerErr) || (errors.Is(providerErr, context.DeadlineExceeded) && ctx.Err() == nil), providerErr
	}

	_, reconcileErr := s.usage.Reconcile(settlementCtx, CompletionUsage{
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

	attemptOutcome = "invalid_output"
	if input.RequireComplete && isIncompleteFinishReason(response.FinishReason) {
		completionErr := fmt.Errorf("model output was incomplete (finish_reason=%s)", response.FinishReason)
		s.finishCompletionAudit(settlementCtx, auditExecution, response, "llm_contract", completionErr)
		return nil, true, completionErr
	}
	if input.ValidateResponse != nil {
		if err := input.ValidateResponse(response); err != nil {
			s.finishCompletionAudit(settlementCtx, auditExecution, response, "llm_contract", err)
			return nil, input.RetryInvalidOutput, err
		}
	}
	attemptOutcome = "success"
	if reconcileErr != nil {
		attemptOutcome = "accounting_error"
	}
	s.finishCompletionAudit(settlementCtx, auditExecution, response, "", nil)
	return response, false, nil
}

func (s *AICompletionService) finishCompletionAudit(ctx context.Context, execution *model.AIActionExecution, response *llm.ChatResponse, failureClass string, executionErr error) {
	if s == nil || s.audit == nil || execution == nil {
		return
	}
	result := aipolicy.ExecutionResult{Status: model.AIActionExecutionSucceeded, CompletedAt: time.Now().UTC()}
	if response != nil {
		result.InputTokens = response.TokensUsed.InputTokensTotal
		result.OutputTokens = response.TokensUsed.OutputTokens
		result.ReasoningTokens = response.TokensUsed.ReasoningTokens
		result.CachedInputTokens = response.TokensUsed.CacheReadTokens
	}
	if executionErr != nil {
		result.Status = model.AIActionExecutionFailed
		result.FailureClass = failureClass
		result.FailureMessage = sanitizeAIActionFailure(executionErr)
	}
	if err := s.audit.Finish(ctx, execution.ID, result); err != nil {
		slog.ErrorContext(ctx, "finish AI completion audit", "execution_id", execution.ID, "error", err)
	}
}

func completionChatRequest(input llm.ChatRequest, route AICompletionRoute) (llm.ChatRequest, error) {
	input.Provider = route.Provider
	input.Model = route.Model
	if route.Model == "deepseek/deepseek-v4.1-flash" && route.OpenRouterProvider == "coreweave/fp8" {
		disabled := false
		input.Reasoning = &llm.ReasoningConfig{Enabled: &disabled}
	}
	openRouterProvider := strings.TrimSpace(route.OpenRouterProvider)
	if openRouterProvider == "" {
		return input, nil
	}
	if normalizeCompletionRouteProvider(route.Provider) != "openrouter" {
		return llm.ChatRequest{}, fmt.Errorf("OpenRouter provider selection requires provider openrouter")
	}
	options := map[string]any{}
	if len(input.ProviderOptions) > 0 && strings.TrimSpace(string(input.ProviderOptions)) != "" {
		if err := json.Unmarshal(input.ProviderOptions, &options); err != nil {
			return llm.ChatRequest{}, fmt.Errorf("decode completion provider options: %w", err)
		}
		if options == nil {
			return llm.ChatRequest{}, fmt.Errorf("completion provider options must be a JSON object")
		}
	}
	options["order"] = []string{openRouterProvider}
	options["allow_fallbacks"] = false
	raw, err := json.Marshal(options)
	if err != nil {
		return llm.ChatRequest{}, fmt.Errorf("encode completion provider options: %w", err)
	}
	input.ProviderOptions = raw
	return input, nil
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

func completionAttemptContext(ctx context.Context, feature string) (context.Context, context.CancelFunc) {
	if feature == BillingFeatureSupportTranslation {
		return context.WithTimeout(ctx, 12*time.Second)
	}
	return context.WithCancel(ctx)
}
