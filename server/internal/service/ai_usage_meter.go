package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	BillingFeatureAIRouting               = "ai_routing"
	BillingFeatureCoverageGapAnalysis     = "coverage_gap_analysis"
	BillingFeatureCRMSignalDetection      = "crm_signal_detection"
	BillingFeatureSupportReplyRewrite     = "support_reply_rewrite"
	BillingFeatureDealAutomationInference = "deal_automation_inference"
	BillingFeatureCRMSummary              = "crm_summary"
	BillingFeatureTaskStandingBrief       = "task_standing_brief"
	BillingFeatureSupportTaskDraft        = "support_task_draft"
	BillingFeatureDocsArticleTranslation  = "docs_article_translation"
	BillingFeatureDocsArticleGeneration   = "docs_article_generation"
	BillingFeatureDocsImportConversion    = "docs_import_conversion"
	BillingFeatureBuiltInLightAgentRun    = "built_in_light_agent_run"
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
	FeatureKey     string
	IdempotencyKey string
	Metadata       map[string]interface{}
}

// MeteredLLMProvider wraps an LLM provider and charges AI usage for calls that
// opt in through WithAIUsageMetering.
type MeteredLLMProvider struct {
	base  llm.Provider
	meter *AIUsageMeter
}

var aiUsageFeatures = map[string]AIUsageFeatureDefinition{
	BillingFeatureAIRouting:               {FeatureKey: BillingFeatureAIRouting, Label: "AI triage and routing", Category: "Support AI", FloorUnits: 2, Chargeable: true},
	BillingFeatureCoverageGapAnalysis:     {FeatureKey: BillingFeatureCoverageGapAnalysis, Label: "Coverage gap analysis", Category: "Docs AI", FloorUnits: 2, Chargeable: true},
	BillingFeatureCRMSignalDetection:      {FeatureKey: BillingFeatureCRMSignalDetection, Label: "CRM signal detection", Category: "CRM AI", FloorUnits: 3, Chargeable: true},
	BillingFeatureSupportReplyRewrite:     {FeatureKey: BillingFeatureSupportReplyRewrite, Label: "Support reply rewrite", Category: "Support AI", FloorUnits: 4, Chargeable: true},
	BillingFeatureDealAutomationInference: {FeatureKey: BillingFeatureDealAutomationInference, Label: "Deal automation inference", Category: "CRM AI", FloorUnits: 5, Chargeable: true},
	BillingFeatureCRMSummary:              {FeatureKey: BillingFeatureCRMSummary, Label: "CRM summary", Category: "CRM AI", FloorUnits: 6, Chargeable: true},
	BillingFeatureTaskStandingBrief:       {FeatureKey: BillingFeatureTaskStandingBrief, Label: "Task standing brief", Category: "Project AI", FloorUnits: 6, Chargeable: true},
	BillingFeatureSupportAIReply:          {FeatureKey: BillingFeatureSupportAIReply, Label: "Support reply draft", Category: "Support AI", FloorUnits: 8, Chargeable: true},
	BillingFeatureSupportTaskDraft:        {FeatureKey: BillingFeatureSupportTaskDraft, Label: "Support task draft", Category: "Support AI", FloorUnits: 8, Chargeable: true},
	BillingFeatureDocsGeneration:          {FeatureKey: BillingFeatureDocsGeneration, Label: "Document generation", Category: "Docs AI", FloorUnits: 15, Chargeable: true},
	BillingFeatureDocsArticleTranslation:  {FeatureKey: BillingFeatureDocsArticleTranslation, Label: "Help article translation", Category: "Docs AI", FloorUnits: 15, Chargeable: true},
	BillingFeatureDocsArticleGeneration:   {FeatureKey: BillingFeatureDocsArticleGeneration, Label: "Help article generation", Category: "Docs AI", FloorUnits: 20, Chargeable: true},
	BillingFeatureDocsImportConversion:    {FeatureKey: BillingFeatureDocsImportConversion, Label: "Help article import formatting", Category: "Docs AI", FloorUnits: 15, Chargeable: true},
	BillingFeatureCRMAction:               {FeatureKey: BillingFeatureCRMAction, Label: "CRM / deal action", Category: "CRM AI", FloorUnits: 5, Chargeable: true},
	BillingFeatureBuiltInLightAgentRun:    {FeatureKey: BillingFeatureBuiltInLightAgentRun, Label: "Built-in agent run", Category: "Agents", FloorUnits: 40, Chargeable: true},
	BillingFeaturePlanningRun:             {FeatureKey: BillingFeaturePlanningRun, Label: "Planning run", Category: "Agents", FloorUnits: 80, Chargeable: true},
	BillingFeatureScribeRun:               {FeatureKey: BillingFeatureScribeRun, Label: "Scribe task planning run", Category: "Agents", FloorUnits: 50, Chargeable: true},
	BillingFeatureMiraRun:                 {FeatureKey: BillingFeatureMiraRun, Label: "Mira marketing run", Category: "Agents", FloorUnits: 50, Chargeable: true},
	BillingFeatureQuillRun:                {FeatureKey: BillingFeatureQuillRun, Label: "Quill documentation run", Category: "Agents", FloorUnits: 60, Chargeable: true},
	BillingFeatureCustomAgentRun:          {FeatureKey: BillingFeatureCustomAgentRun, Label: "Custom agent run", Category: "Agents", FloorUnits: 60, Chargeable: true},
	BillingFeatureAtlasRun:                {FeatureKey: BillingFeatureAtlasRun, Label: "Atlas epic planning run", Category: "Agents", FloorUnits: 80, Chargeable: true},
	BillingFeatureCodingRun:               {FeatureKey: BillingFeatureCodingRun, Label: "Coding / review run", Category: "Agents", FloorUnits: 100, Chargeable: true},
	BillingFeatureForgeRun:                {FeatureKey: BillingFeatureForgeRun, Label: "Forge coding run", Category: "Agents", FloorUnits: 100, Chargeable: true},
	BillingFeatureLensRun:                 {FeatureKey: BillingFeatureLensRun, Label: "Lens review run", Category: "Agents", FloorUnits: 100, Chargeable: true},
	BillingFeatureCustomCodingReviewRun:   {FeatureKey: BillingFeatureCustomCodingReviewRun, Label: "Custom coding/review run", Category: "Agents", FloorUnits: 100, Chargeable: true},
	BillingFeatureCustomAgentDraft:        {FeatureKey: BillingFeatureCustomAgentDraft, Label: "Custom agent draft", Category: "Setup", Chargeable: false},
	BillingFeatureAutomationSetup:         {FeatureKey: BillingFeatureAutomationSetup, Label: "Automation setup", Category: "Setup", Chargeable: false},
	BillingFeatureFlowSetup:               {FeatureKey: BillingFeatureFlowSetup, Label: "Flow setup", Category: "Setup", Chargeable: false},
	BillingFeatureAgentPromptImprovement:  {FeatureKey: BillingFeatureAgentPromptImprovement, Label: "Agent prompt improvement", Category: "Setup", Chargeable: false},
	BillingFeatureDataImportSetup:         {FeatureKey: BillingFeatureDataImportSetup, Label: "Data import setup", Category: "Setup", Chargeable: false},
	BillingFeatureCompanyProductContext:   {FeatureKey: BillingFeatureCompanyProductContext, Label: "Company/product context generation", Category: "Setup", Chargeable: false},
	BillingFeatureDockChatTitle:           {FeatureKey: BillingFeatureDockChatTitle, Label: "Dock chat title", Category: "Agents", Chargeable: false},
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
	if ok && p.meter.usage != nil {
		return p.chatCompletionTokenPriced(ctx, req, metering)
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
		return nil, err
	}
	return resp, nil
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
		WorkspaceID: input.WorkspaceID, TaskNature: taskNatureForFeature(input.FeatureKey), FeatureKey: input.FeatureKey,
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

func agentPresetKey(agent *model.Agent) string {
	if agent == nil {
		return ""
	}
	return agent.PresetKey
}
