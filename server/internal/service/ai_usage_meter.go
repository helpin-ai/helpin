package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	BillingFeatureAIRouting               = "ai_routing"
	BillingFeatureCoverageGapAnalysis     = "coverage_gap_analysis"
	BillingFeatureCRMSignalDetection      = "crm_signal_detection"
	BillingFeatureSupportReplyRewrite     = "support_reply_rewrite"
	BillingFeaturePMCommentRewrite        = "pm_comment_rewrite"
	BillingFeatureCRMEmailRewrite         = "crm_email_rewrite"
	BillingFeatureDealAutomationInference = "deal_automation_inference"
	BillingFeatureMeetingIntelligence     = "meeting_intelligence"
	BillingFeatureCRMSummary              = "crm_summary"
	BillingFeatureTaskStandingBrief       = "task_standing_brief"
	BillingFeatureSupportTaskDraft        = "support_task_draft"
	BillingFeatureDocsArticleTranslation  = "docs_article_translation"
	BillingFeatureDocsArticleGeneration   = "docs_article_generation"
	BillingFeatureDocsImportConversion    = "docs_import_conversion"
	BillingFeatureHelpcenterAnswer        = "helpcenter_answer_generation"
	BillingFeatureBuiltInLightAgentRun    = "built_in_light_agent_run"
	BillingFeatureAskChat                 = "ask_chat"
	BillingFeatureScribeRun               = "scribe_run"
	BillingFeatureMiraRun                 = "mira_run"
	BillingFeatureQuillRun                = "quill_run"
	BillingFeatureCustomAgentRun          = "custom_agent_run"
	BillingFeatureAtlasRun                = "atlas_run"
	BillingFeatureForgeRun                = "forge_run"
	BillingFeatureLensRun                 = "lens_run"
	BillingFeatureCustomCodingReviewRun   = "custom_coding_review_run"
	BillingFeatureCustomAgentDraft        = "custom_agent_draft"
	BillingFeatureAutomationSetup         = "automation_setup"
	BillingFeatureFlowSetup               = "flow_setup"
	BillingFeatureAgentPromptImprovement  = "agent_prompt_improvement"
	BillingFeatureDataImportSetup         = "data_import_setup"
	BillingFeatureCompanyProductContext   = "company_product_context_generation"
	BillingFeatureDockChatTitle           = "dock_chat_title_generation"
)

// AIUsageCalculation is the normalized token usage input for internal AI usage
// metering. Customer-facing surfaces call the result "AI usage"; the billing
// ledger still stores credits internally.
type AIUsageCalculation struct {
	FeatureKey        string
	InputTokens       int
	OutputTokens      int
	ReasoningTokens   int
	CachedInputTokens int
}

func aiUsageIdempotencyKey(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	return strings.Join(clean, ":")
}

func aiUsageStableHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func aiUsagePayloadIdempotencyKey(payload []byte, parts ...string) string {
	return aiUsageIdempotencyKey(append(parts, aiUsageStableHash(string(payload)))...)
}

// AIUsageFeatureDefinition describes one metered AI feature.
type AIUsageFeatureDefinition struct {
	FeatureKey string
	Label      string
	Category   string
	FloorUnits int
	Chargeable bool
}

type aiUsageCreditConsumer interface {
	PreflightCredits(ctx context.Context, input BillingCreditPreflight) error
	ConsumeCredits(ctx context.Context, input BillingCreditConsumption) (*BillingSummary, error)
}

// AIUsageMeter is the central adapter from AI token usage to the billing ledger.
type AIUsageMeter struct {
	consumer aiUsageCreditConsumer
	usage    *AIUsageService
}

// AIUsageMeterInput describes one completed AI action to charge.
type AIUsageMeterInput struct {
	WorkspaceID       string
	FeatureKey        string
	IdempotencyKey    string
	InputTokens       int
	OutputTokens      int
	ReasoningTokens   int
	CachedInputTokens int
	CacheWriteTokens  int
	Metadata          map[string]interface{}
	AllowOverage      bool
}

type aiUsageMeteringContextKey struct{}
type aiUsageMeteringExemptContextKey struct{}

var ErrAIUsageMeteringRequired = errors.New("AI usage metering context is required")

// AIUsageMeteringContext marks one LLM call for usage metering.
type AIUsageMeteringContext struct {
	WorkspaceID    string
	ActionKey      string
	FeatureKey     string
	OperationKey   string
	IdempotencyKey string
	Attempt        int
	Metadata       map[string]interface{}
}

// MeteredLLMProvider wraps an LLM provider and charges AI usage for calls that
// opt in through WithAIUsageMetering.
type MeteredLLMProvider struct {
	base     llm.Provider
	meter    *AIUsageMeter
	registry *aipolicy.Registry
	audit    aipolicy.ExecutionAudit
}

var aiUsageFeatures = aiUsageFeaturesFromRegistry(aipolicy.DefaultRegistry())

func aiUsageFeaturesFromRegistry(registry *aipolicy.Registry) map[string]AIUsageFeatureDefinition {
	features := make(map[string]AIUsageFeatureDefinition)
	for _, action := range registry.Actions() {
		_, exists := features[action.FeatureKey]
		// The feature-level action owns the customer-facing label/category.
		// Specialized sub-actions only supply execution policy and audit detail.
		if exists && !strings.HasPrefix(action.Key, "feature.") {
			continue
		}
		features[action.FeatureKey] = AIUsageFeatureDefinition{
			FeatureKey: action.FeatureKey,
			Label:      action.Label,
			Category:   string(action.Category),
			FloorUnits: action.FloorUnits,
			Chargeable: action.Chargeable,
		}
	}
	return features
}

func NewAIUsageMeter(consumer aiUsageCreditConsumer) *AIUsageMeter {
	return &AIUsageMeter{consumer: consumer}
}

// NewTokenPricedAIUsageMeter creates the active micro-USD metering adapter.
func NewTokenPricedAIUsageMeter(usage *AIUsageService) *AIUsageMeter {
	return &AIUsageMeter{usage: usage}
}

func NewMeteredLLMProvider(base llm.Provider, meter *AIUsageMeter) llm.Provider {
	if base == nil || meter == nil {
		return base
	}
	return &MeteredLLMProvider{base: base, meter: meter}
}

// NewGovernedLLMProvider wraps chat execution with policy validation, usage metering, and audit.
func NewGovernedLLMProvider(base llm.Provider, meter *AIUsageMeter, registry *aipolicy.Registry, audit aipolicy.ExecutionAudit) llm.Provider {
	if base == nil {
		return nil
	}
	return &MeteredLLMProvider{base: base, meter: meter, registry: registry, audit: audit}
}

func WithAIUsageMetering(ctx context.Context, input AIUsageMeteringContext) context.Context {
	return context.WithValue(ctx, aiUsageMeteringContextKey{}, input)
}

func WithAIUsageMeteringExempt(ctx context.Context) context.Context {
	return context.WithValue(ctx, aiUsageMeteringExemptContextKey{}, true)
}

func AIUsageMeteringExemptFromContext(ctx context.Context) bool {
	exempt, _ := ctx.Value(aiUsageMeteringExemptContextKey{}).(bool)
	return exempt
}

func AIUsageMeteringFromContext(ctx context.Context) (AIUsageMeteringContext, bool) {
	input, ok := ctx.Value(aiUsageMeteringContextKey{}).(AIUsageMeteringContext)
	if !ok {
		return AIUsageMeteringContext{}, false
	}
	if input.FeatureKey == "" || input.IdempotencyKey == "" {
		return input, false
	}
	feature, featureKnown := AIUsageFeature(input.FeatureKey)
	if input.WorkspaceID == "" && (!featureKnown || feature.Chargeable) {
		return input, false
	}
	return input, true
}

func (p *MeteredLLMProvider) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	metering, ok := AIUsageMeteringFromContext(ctx)
	if !ok && !AIUsageMeteringExemptFromContext(ctx) {
		return nil, ErrAIUsageMeteringRequired
	}
	var auditExecution *model.AIActionExecution
	if p.registry != nil {
		if !ok {
			return nil, aipolicy.ErrInvalidExecutionContext
		}
		action, err := aipolicy.ResolveExecution(p.registry, aipolicy.ExecutionContext{
			WorkspaceID: metering.WorkspaceID, ActionKey: metering.ActionKey,
			FeatureKey: metering.FeatureKey, IdempotencyKey: metering.IdempotencyKey,
			Attempt: metering.Attempt, Metadata: metering.Metadata,
		}, aipolicy.Route{Provider: req.Provider, Model: req.Model})
		if err != nil {
			return nil, err
		}
		if req.Provider == "" {
			req.Provider = action.DefaultProvider
		}
		if req.Model == "" {
			req.Model = action.DefaultModel
		}
		if p.audit != nil {
			attempt := metering.Attempt
			if attempt <= 0 {
				attempt = 1
			}
			auditExecution, err = p.audit.Start(ctx, &model.AIActionExecution{
				WorkspaceID: metering.WorkspaceID, ActionKey: action.Key,
				PolicyVersion: action.PolicyVersion, FeatureKey: action.FeatureKey,
				Category: string(action.Category), Origin: action.Origin,
				Modality: string(action.Modality), Provider: req.Provider, Model: req.Model,
				IdempotencyKey: metering.IdempotencyKey, Attempt: attempt,
				Status: model.AIActionExecutionRunning, Metadata: mustJSONMetadata(metering.Metadata),
				StartedAt: time.Now().UTC(),
			})
			if err != nil {
				return nil, fmt.Errorf("start AI action audit: %w", err)
			}
		}
	}
	if p.meter == nil {
		resp, err := p.base.ChatCompletion(ctx, req)
		p.finishAIActionAudit(ctx, auditExecution, resp, err)
		return resp, err
	}
	if ok && p.meter.usage != nil {
		resp, err := p.chatCompletionTokenPriced(ctx, req, metering)
		p.finishAIActionAudit(ctx, auditExecution, resp, err)
		return resp, err
	}
	if ok {
		if err := p.meter.Preflight(ctx, AIUsageMeterInput{
			WorkspaceID:    metering.WorkspaceID,
			FeatureKey:     metering.FeatureKey,
			IdempotencyKey: metering.IdempotencyKey,
			Metadata:       metering.Metadata,
		}); err != nil {
			return nil, err
		}
	}

	resp, err := p.base.ChatCompletion(ctx, req)
	if err != nil || resp == nil {
		p.finishAIActionAudit(ctx, auditExecution, resp, err)
		return resp, err
	}
	if !ok {
		return resp, nil
	}
	if _, err := p.meter.Consume(ctx, AIUsageMeterInput{
		WorkspaceID:       metering.WorkspaceID,
		FeatureKey:        metering.FeatureKey,
		IdempotencyKey:    metering.IdempotencyKey,
		InputTokens:       resp.TokensUsed.InputTokens,
		OutputTokens:      resp.TokensUsed.OutputTokens,
		ReasoningTokens:   resp.TokensUsed.ReasoningTokens,
		CachedInputTokens: resp.TokensUsed.CachedInputTokens,
		CacheWriteTokens:  resp.TokensUsed.CacheWriteTokens,
		Metadata:          metering.Metadata,
	}); err != nil {
		p.finishAIActionAudit(ctx, auditExecution, resp, err)
		return nil, err
	}
	p.finishAIActionAudit(ctx, auditExecution, resp, nil)
	return resp, nil
}

func (p *MeteredLLMProvider) finishAIActionAudit(ctx context.Context, execution *model.AIActionExecution, resp *llm.ChatResponse, callErr error) {
	if p == nil || p.audit == nil || execution == nil {
		return
	}
	result := aipolicy.ExecutionResult{Status: model.AIActionExecutionSucceeded, CompletedAt: time.Now().UTC()}
	if resp != nil {
		result.InputTokens = resp.TokensUsed.InputTokens
		result.OutputTokens = resp.TokensUsed.OutputTokens
		result.ReasoningTokens = resp.TokensUsed.ReasoningTokens
		result.CachedInputTokens = resp.TokensUsed.CachedInputTokens
	}
	if callErr != nil {
		result.Status = model.AIActionExecutionFailed
		result.FailureClass = aiActionFailureClass(callErr)
		result.FailureMessage = sanitizeAIActionFailure(callErr)
	}
	if err := p.audit.Finish(ctx, execution.ID, result); err != nil {
		slog.ErrorContext(ctx, "finish AI action audit", "execution_id", execution.ID, "error", err)
	}
}

func mustJSONMetadata(metadata map[string]interface{}) json.RawMessage {
	if len(metadata) == 0 {
		return []byte("{}")
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

func aiActionFailureClass(err error) string {
	if errors.Is(err, model.ErrPricingConfigurationMissing) {
		return "configuration"
	}
	var providerErr *llm.ProviderError
	if errors.As(err, &providerErr) {
		return "llm_provider"
	}
	return "llm_preflight"
}

func sanitizeAIActionFailure(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func (p *MeteredLLMProvider) chatCompletionTokenPriced(ctx context.Context, req llm.ChatRequest, input AIUsageMeteringContext) (*llm.ChatResponse, error) {
	resolver, ok := p.base.(llm.PricingIdentityResolver)
	if !ok {
		return nil, fmt.Errorf("AI usage pricing identity is unavailable")
	}
	identity, err := resolver.ResolvePricingIdentity(req)
	if err != nil {
		return nil, err
	}
	feature, known := AIUsageFeature(input.FeatureKey)
	promotional := known && !feature.Chargeable
	maximumOutput := int64(req.MaxTokens)
	if maximumOutput <= 0 {
		maximumOutput = 4096
	}
	preflight, err := p.meter.usage.Preflight(ctx, PreflightRequest{Metering: MeteringRequest{
		WorkspaceID: input.WorkspaceID, TaskNature: taskNatureForFeature(input.FeatureKey), FeatureKey: input.FeatureKey, OperationKey: input.OperationKey,
		Provider: identity.Provider, Model: identity.Model, Route: identity.Route, ServiceTier: identity.ServiceTier,
		FundingMode: aiusage.FundingHelpinHosted, InputTokensEstimate: estimateChatInputTokens(req),
		MaximumOutputTokens: maximumOutput, ExecutionID: metadataString(input.Metadata, "execution_id"),
		IdempotencyKey: input.IdempotencyKey, Promotional: promotional,
	}})
	if err != nil {
		return nil, err
	}
	response, providerErr := p.base.ChatCompletion(ctx, req)
	if providerErr != nil || response == nil {
		if preflight.ReservationID != "" {
			_ = p.meter.usage.Fail(ctx, preflight.ReservationID)
		}
		return response, providerErr
	}
	_, reconcileErr := p.meter.usage.Reconcile(ctx, CompletionUsage{
		Context: *preflight,
		Telemetry: aiusage.TokenTelemetry{
			InputTokensTotal: int64(response.TokensUsed.InputTokensTotal), CacheReadTokens: int64(response.TokensUsed.CacheReadTokens),
			CacheWriteTokens: int64(response.TokensUsed.CacheWriteTokens), CompletionTokensTotal: int64(response.TokensUsed.CompletionTokensTotal),
			OutputTokens: int64(response.TokensUsed.OutputTokens), ReasoningTokens: int64(response.TokensUsed.ReasoningTokens),
			CompletionIncludesReasoning: response.TokensUsed.CompletionIncludesReasoning,
		},
		MeasurementStatus: measurementStatus(response.TokensUsed),
	})
	if reconcileErr != nil {
		slog.Error("AI usage reconciliation failed after successful provider response", "workspace_id", input.WorkspaceID, "feature_key", input.FeatureKey, "error", reconcileErr)
	}
	return response, nil
}

func estimateChatInputTokens(req llm.ChatRequest) int64 {
	characters := len(req.SystemPrompt)
	for _, message := range req.Messages {
		characters += len(message.Content)
		for _, part := range message.ContentParts {
			characters += len(part.Text)
		}
	}
	return int64((characters + 3) / 4)
}

func metadataString(metadata map[string]interface{}, key string) string {
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
}

func measurementStatus(usage llm.TokenUsage) string {
	if usage.InputTokensTotal == 0 && usage.CompletionTokensTotal == 0 && usage.OutputTokens == 0 {
		return "estimated"
	}
	return "actual"
}

func taskNatureForFeature(featureKey string) string {
	switch featureKey {
	case BillingFeaturePlanningRun, BillingFeatureAtlasRun, BillingFeatureScribeRun:
		return "planning"
	case BillingFeatureCodingRun, BillingFeatureForgeRun:
		return "coding"
	case BillingFeatureLensRun, BillingFeatureCustomCodingReviewRun:
		return "review"
	default:
		return "support"
	}
}

func (m *AIUsageMeter) Preflight(ctx context.Context, input AIUsageMeterInput) error {
	if m != nil && m.usage != nil {
		// Token-priced calls reserve with their resolved provider route immediately
		// before execution. Agent Runtime launches use PreflightAgentRunAIUsage,
		// which persists the resulting reservation on the durable run.
		return nil
	}
	if m == nil || m.consumer == nil {
		return fmt.Errorf("ai usage meter billing consumer is required")
	}
	credits := BillingCreditsForFeature(input.FeatureKey)
	if credits == 0 {
		return nil
	}
	return m.consumer.PreflightCredits(ctx, BillingCreditPreflight{
		WorkspaceID: input.WorkspaceID,
		FeatureKey:  input.FeatureKey,
		Credits:     credits,
	})
}

func (m *AIUsageMeter) PreflightUsage(ctx context.Context, input AIUsageMeterInput) error {
	if m != nil && m.usage != nil {
		return nil
	}
	if m == nil || m.consumer == nil {
		return fmt.Errorf("ai usage meter billing consumer is required")
	}
	units := CalculateAIUsageUnits(AIUsageCalculation{
		FeatureKey:        input.FeatureKey,
		InputTokens:       input.InputTokens,
		OutputTokens:      input.OutputTokens,
		ReasoningTokens:   input.ReasoningTokens,
		CachedInputTokens: input.CachedInputTokens,
	})
	if units == 0 {
		return nil
	}
	return m.consumer.PreflightCredits(ctx, BillingCreditPreflight{
		WorkspaceID: input.WorkspaceID,
		FeatureKey:  input.FeatureKey,
		Credits:     units,
	})
}

func (m *AIUsageMeter) Consume(ctx context.Context, input AIUsageMeterInput) (*BillingSummary, error) {
	if m != nil && m.usage != nil {
		return nil, nil
	}
	if m == nil || m.consumer == nil {
		return nil, fmt.Errorf("ai usage meter billing consumer is required")
	}
	units := CalculateAIUsageUnits(AIUsageCalculation{
		FeatureKey:        input.FeatureKey,
		InputTokens:       input.InputTokens,
		OutputTokens:      input.OutputTokens,
		ReasoningTokens:   input.ReasoningTokens,
		CachedInputTokens: input.CachedInputTokens,
	})
	if units == 0 {
		return nil, nil
	}
	metadata := map[string]any{}
	for key, value := range input.Metadata {
		metadata[key] = value
	}
	if feature, ok := AIUsageFeature(input.FeatureKey); ok {
		metadata["usage_label"] = feature.Label
		metadata["usage_category"] = feature.Category
		metadata["usage_floor"] = feature.FloorUnits
	}
	metadata["input_tokens"] = input.InputTokens
	metadata["output_tokens"] = input.OutputTokens
	metadata["reasoning_tokens"] = input.ReasoningTokens
	metadata["cached_input_tokens"] = input.CachedInputTokens
	metadata["cache_write_tokens"] = input.CacheWriteTokens
	metadata["weighted_token_formula"] = "input + output*6 + reasoning*6 - cached_input*0.90"

	return m.consumer.ConsumeCredits(ctx, BillingCreditConsumption{
		WorkspaceID:    input.WorkspaceID,
		FeatureKey:     input.FeatureKey,
		Credits:        units,
		IdempotencyKey: input.IdempotencyKey,
		Metadata:       metadata,
		AllowOverage:   input.AllowOverage,
	})
}

// AIUsageFeature returns the metering definition for a feature key.
func AIUsageFeature(featureKey string) (AIUsageFeatureDefinition, bool) {
	feature, ok := aiUsageFeatures[featureKey]
	return feature, ok
}

// AIUsageFeatures returns all known metering definitions in stable order.
func AIUsageFeatures() []AIUsageFeatureDefinition {
	features := make([]AIUsageFeatureDefinition, 0, len(aiUsageFeatures))
	for _, feature := range aiUsageFeatures {
		features = append(features, feature)
	}
	sort.Slice(features, func(i, j int) bool {
		return features[i].FeatureKey < features[j].FeatureKey
	})
	return features
}

// CalculateAIUsageUnits converts model token usage into internal usage units.
func CalculateAIUsageUnits(input AIUsageCalculation) int {
	feature, ok := AIUsageFeature(input.FeatureKey)
	if ok && !feature.Chargeable {
		return 0
	}

	weightedTokens := float64(max(input.InputTokens, 0)) +
		float64(max(input.OutputTokens, 0))*6 +
		float64(max(input.ReasoningTokens, 0))*6 -
		float64(max(input.CachedInputTokens, 0))*0.90
	if weightedTokens < 0 {
		weightedTokens = 0
	}

	tokenUnits := int(math.Ceil(weightedTokens / 1000))
	floor := 0
	if ok {
		floor = feature.FloorUnits
	}
	if tokenUnits < floor {
		return floor
	}
	return tokenUnits
}

func AgentRunAIUsageFeature(agent *model.Agent) string {
	if agent == nil {
		return BillingFeatureBuiltInLightAgentRun
	}
	preset := strings.TrimSpace(agent.PresetKey)
	if !agent.IsSystem {
		switch preset {
		case model.AgentPresetCodeBuilder, model.AgentPresetReviewAgent:
			return BillingFeatureCustomCodingReviewRun
		default:
			return BillingFeatureCustomAgentRun
		}
	}
	switch preset {
	case model.AgentPresetAskAgent:
		return BillingFeatureAskChat
	case model.AgentPresetEpicPlanner:
		return BillingFeatureAtlasRun
	case model.AgentPresetTaskPlanner:
		return BillingFeatureScribeRun
	case model.AgentPresetMarketer:
		return BillingFeatureMiraRun
	case model.AgentPresetDocumentationAgent:
		return BillingFeatureQuillRun
	case model.AgentPresetCodeBuilder:
		return BillingFeatureForgeRun
	case model.AgentPresetReviewAgent:
		return BillingFeatureLensRun
	default:
		return BillingFeatureBuiltInLightAgentRun
	}
}

func RecordAgentRunAIUsage(ctx context.Context, meter *AIUsageMeter, run *model.AgentRun, agent *model.Agent) error {
	if meter == nil || run == nil {
		return nil
	}
	inputTokens := run.InputTokens
	outputTokens := run.OutputTokens
	if inputTokens == 0 && outputTokens == 0 && run.TokensUsed > 0 {
		inputTokens = run.TokensUsed
	}
	_, err := meter.Consume(ctx, AIUsageMeterInput{
		WorkspaceID:       run.WorkspaceID,
		FeatureKey:        AgentRunAIUsageFeature(agent),
		IdempotencyKey:    aiUsageIdempotencyKey(run.WorkspaceID, "agent_run", run.ID),
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		CachedInputTokens: run.CachedInputTokens,
		Metadata: map[string]interface{}{
			"run_id":     run.ID,
			"agent_id":   run.AgentID,
			"preset_key": strings.TrimSpace(agentPresetKey(agent)),
			"is_system":  agent != nil && agent.IsSystem,
		},
	})
	return err
}

func PreflightAgentRunAIUsage(ctx context.Context, meter *AIUsageMeter, run *model.AgentRun, agent *model.Agent) error {
	if meter == nil || run == nil {
		return nil
	}
	if meter.usage != nil {
		provider, modelID, _ := agentPricingIdentity(agent)
		featureKey := AgentRunAIUsageFeature(agent)
		metering, err := meter.usage.Preflight(ctx, PreflightRequest{Metering: MeteringRequest{
			WorkspaceID: run.WorkspaceID, TaskNature: taskNatureForFeature(featureKey), FeatureKey: featureKey,
			Provider: provider, Model: modelID, Route: "", ServiceTier: "standard",
			FundingMode: aiusage.FundingHelpinHosted, InputTokensEstimate: int64((len(run.Input) + 3) / 4),
			MaximumOutputTokens: 128000, ExecutionID: run.ID,
			IdempotencyKey: aiUsageIdempotencyKey(run.WorkspaceID, "agent_run", run.ID),
		}})
		if err != nil {
			return err
		}
		return storeAgentRunMeteringContext(run, *metering)
	}
	return meter.Preflight(ctx, AIUsageMeterInput{
		WorkspaceID:    run.WorkspaceID,
		FeatureKey:     AgentRunAIUsageFeature(agent),
		IdempotencyKey: aiUsageIdempotencyKey(run.WorkspaceID, "agent_run", run.ID, "preflight"),
		Metadata: map[string]interface{}{
			"run_id":     run.ID,
			"agent_id":   run.AgentID,
			"preset_key": strings.TrimSpace(agentPresetKey(agent)),
			"is_system":  agent != nil && agent.IsSystem,
		},
	})
}

func agentPricingIdentity(agent *model.Agent) (provider, modelID, route string) {
	if agent != nil {
		provider = strings.ToLower(strings.TrimSpace(derefString(agent.Provider)))
		modelID = strings.TrimSpace(derefString(agent.Model))
	}
	if provider == "openrouter-responses" {
		provider = "openrouter"
	}
	if provider == "" || modelID == "" {
		feature := AgentRunAIUsageFeature(agent)
		if taskNatureForFeature(feature) == "planning" || taskNatureForFeature(feature) == "coding" || taskNatureForFeature(feature) == "review" {
			provider, modelID = "openrouter", "openai/gpt-5.6-terra"
		} else {
			provider, modelID = "openrouter", "openai/gpt-5.6-luna"
		}
	}
	route = modelID
	if provider == "openai" && strings.Contains(modelID, "/") {
		route = modelID[strings.LastIndex(modelID, "/")+1:]
	}
	return provider, modelID, route
}

const agentRunMeteringSummaryKey = "ai_usage_metering"

const agentRunUsageCheckpointSummaryKey = "ai_usage_checkpoint"

type agentRunUsageCheckpoint struct {
	Turn                  int `json:"turn"`
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
}

func storeAgentRunMeteringContext(run *model.AgentRun, metering MeteringContext) error {
	if run == nil {
		return nil
	}
	summary := map[string]any{}
	if len(run.OutputSummary) > 0 && string(run.OutputSummary) != "null" {
		_ = json.Unmarshal(run.OutputSummary, &summary)
	}
	summary[agentRunMeteringSummaryKey] = metering
	raw, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("store agent run AI usage context: %w", err)
	}
	run.OutputSummary = raw
	return nil
}

func agentRunMeteringContext(run *model.AgentRun) (MeteringContext, bool) {
	if run == nil || len(run.OutputSummary) == 0 {
		return MeteringContext{}, false
	}
	var summary map[string]json.RawMessage
	if err := json.Unmarshal(run.OutputSummary, &summary); err != nil {
		return MeteringContext{}, false
	}
	var metering MeteringContext
	if err := json.Unmarshal(summary[agentRunMeteringSummaryKey], &metering); err != nil || metering.PricingVersion == "" {
		return MeteringContext{}, false
	}
	return metering, true
}

func (m *AIUsageMeter) reconcileAgentRun(ctx context.Context, run *model.AgentRun, usage agentRuntimeUsagePayload) error {
	if m == nil || m.usage == nil {
		return nil
	}
	metering, ok := agentRunMeteringContext(run)
	if !ok {
		return model.ErrPricingConfigurationMissing
	}
	checkpoint := agentRunUsageCheckpointFromSummary(run.OutputSummary)
	delta := agentRunUsageDelta(usage, checkpoint)
	if agentRunUsageIsZero(delta) {
		return m.usage.Release(ctx, metering.ReservationID, "interactive_run_complete")
	}
	if checkpoint.Turn > 0 {
		metering.IdempotencyKey = aiUsageIdempotencyKey(metering.IdempotencyKey, "turn", fmt.Sprint(checkpoint.Turn+1))
	}
	status := "actual"
	if agentRunUsageIsZero(usage) {
		status = "estimated"
	}
	_, err := m.usage.Reconcile(ctx, CompletionUsage{
		Context: metering,
		Telemetry: aiusage.TokenTelemetry{
			InputTokensTotal: int64(delta.InputTokens), CacheReadTokens: int64(delta.CachedInputTokens),
			CompletionTokensTotal: int64(delta.OutputTokens + delta.ReasoningOutputTokens),
			OutputTokens:          int64(delta.OutputTokens), ReasoningTokens: int64(delta.ReasoningOutputTokens),
		},
		MeasurementStatus: status,
	})
	return err
}

func (m *AIUsageMeter) checkpointAgentRun(ctx context.Context, run *model.AgentRun, usage agentRuntimeUsagePayload) error {
	if m == nil || m.usage == nil || run == nil {
		return nil
	}
	metering, ok := agentRunMeteringContext(run)
	if !ok {
		return model.ErrPricingConfigurationMissing
	}
	checkpoint := agentRunUsageCheckpointFromSummary(run.OutputSummary)
	delta := agentRunUsageDelta(usage, checkpoint)
	if agentRunUsageIsZero(delta) {
		return nil
	}
	turn := checkpoint.Turn + 1
	metering.IdempotencyKey = aiUsageIdempotencyKey(metering.IdempotencyKey, "turn", fmt.Sprint(turn))
	previousSummary := append(json.RawMessage(nil), run.OutputSummary...)
	nextCheckpoint := agentRunUsageCheckpoint{
		Turn: turn, InputTokens: usage.InputTokens, CachedInputTokens: usage.CachedInputTokens,
		OutputTokens: usage.OutputTokens, ReasoningOutputTokens: usage.ReasoningOutputTokens,
	}
	if err := storeAgentRunUsageCheckpoint(run, nextCheckpoint); err != nil {
		return err
	}
	if _, err := m.usage.Checkpoint(ctx, CompletionUsage{
		Context: metering,
		Telemetry: aiusage.TokenTelemetry{
			InputTokensTotal: int64(delta.InputTokens), CacheReadTokens: int64(delta.CachedInputTokens),
			CompletionTokensTotal: int64(delta.OutputTokens + delta.ReasoningOutputTokens),
			OutputTokens:          int64(delta.OutputTokens), ReasoningTokens: int64(delta.ReasoningOutputTokens),
		},
		MeasurementStatus: "actual",
		RunID:             run.ID,
		RunOutputSummary:  model.JSONBlob(run.OutputSummary),
	}); err != nil {
		run.OutputSummary = previousSummary
		return err
	}
	return nil
}

func agentRunUsageCheckpointFromSummary(summary json.RawMessage) agentRunUsageCheckpoint {
	if len(summary) == 0 {
		return agentRunUsageCheckpoint{}
	}
	var body map[string]json.RawMessage
	if json.Unmarshal(summary, &body) != nil {
		return agentRunUsageCheckpoint{}
	}
	var checkpoint agentRunUsageCheckpoint
	_ = json.Unmarshal(body[agentRunUsageCheckpointSummaryKey], &checkpoint)
	return checkpoint
}

func storeAgentRunUsageCheckpoint(run *model.AgentRun, checkpoint agentRunUsageCheckpoint) error {
	body := map[string]any{}
	if len(run.OutputSummary) > 0 && string(run.OutputSummary) != "null" {
		_ = json.Unmarshal(run.OutputSummary, &body)
	}
	body[agentRunUsageCheckpointSummaryKey] = checkpoint
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("store agent run AI usage checkpoint: %w", err)
	}
	run.OutputSummary = raw
	return nil
}

func agentRunUsageDelta(usage agentRuntimeUsagePayload, checkpoint agentRunUsageCheckpoint) agentRuntimeUsagePayload {
	return agentRuntimeUsagePayload{
		InputTokens:           max(usage.InputTokens-checkpoint.InputTokens, 0),
		CachedInputTokens:     max(usage.CachedInputTokens-checkpoint.CachedInputTokens, 0),
		OutputTokens:          max(usage.OutputTokens-checkpoint.OutputTokens, 0),
		ReasoningOutputTokens: max(usage.ReasoningOutputTokens-checkpoint.ReasoningOutputTokens, 0),
	}
}

func agentRunUsageIsZero(usage agentRuntimeUsagePayload) bool {
	return usage.InputTokens == 0 && usage.CachedInputTokens == 0 &&
		usage.OutputTokens == 0 && usage.ReasoningOutputTokens == 0
}

func agentRunUsageExceedsBudget(run *model.AgentRun, usage agentRuntimeUsagePayload) bool {
	metering, ok := agentRunMeteringContext(run)
	if !ok || metering.EnforcementMode != model.AIUsageEnforcementStrict || metering.MaxBillableMicrousd <= 0 {
		return false
	}
	normalized, err := aiusage.NormalizeTokens(aiusage.TokenTelemetry{
		InputTokensTotal: int64(usage.InputTokens), CacheReadTokens: int64(usage.CachedInputTokens),
		CompletionTokensTotal: int64(usage.OutputTokens + usage.ReasoningOutputTokens),
		OutputTokens:          int64(usage.OutputTokens), ReasoningTokens: int64(usage.ReasoningOutputTokens),
	})
	if err != nil {
		return false
	}
	charge, err := aiusage.CalculateCharge(aiusage.ChargeInput{FundingMode: metering.FundingMode, Tokens: normalized, Rates: metering.Route.Rates})
	return err == nil && charge.FinalMicrousd >= metering.MaxBillableMicrousd
}

func agentPresetKey(agent *model.Agent) string {
	if agent == nil {
		return ""
	}
	return agent.PresetKey
}
