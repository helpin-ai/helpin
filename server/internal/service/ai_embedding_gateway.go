package service

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type governedEmbeddingProvider struct {
	base     llm.EmbeddingProvider
	meter    *AIUsageMeter
	registry *aipolicy.Registry
	audit    aipolicy.ExecutionAudit
}

// NewGovernedEmbeddingProvider wraps embeddings with policy, billing, and audit enforcement.
func NewGovernedEmbeddingProvider(base llm.EmbeddingProvider, meter *AIUsageMeter, registry *aipolicy.Registry, audit aipolicy.ExecutionAudit) llm.EmbeddingProvider {
	if base == nil {
		return nil
	}
	return &governedEmbeddingProvider{base: base, meter: meter, registry: registry, audit: audit}
}

func (p *governedEmbeddingProvider) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	metering, ok := AIUsageMeteringFromContext(ctx)
	if !ok {
		return nil, ErrAIUsageMeteringRequired
	}
	action, err := aipolicy.ResolveExecution(p.registry, aipolicy.ExecutionContext{
		WorkspaceID: metering.WorkspaceID, ActionKey: metering.ActionKey,
		FeatureKey: metering.FeatureKey, IdempotencyKey: metering.IdempotencyKey,
		Attempt: metering.Attempt, Metadata: metering.Metadata,
	}, aipolicy.Route{Provider: req.Provider, Model: req.Model})
	if err != nil {
		return nil, err
	}
	if action.Modality != aipolicy.ModalityEmbedding {
		return nil, fmt.Errorf("%w: action %s is not an embedding action", aipolicy.ErrInvalidExecutionContext, action.Key)
	}
	if req.Provider == "" {
		req.Provider = action.DefaultProvider
	}
	if req.Model == "" {
		req.Model = action.DefaultModel
	}
	attempt := metering.Attempt
	if attempt <= 0 {
		attempt = 1
	}
	var execution *model.AIActionExecution
	if p.audit != nil {
		execution, err = p.audit.Start(ctx, &model.AIActionExecution{
			WorkspaceID: metering.WorkspaceID, ActionKey: action.Key,
			PolicyVersion: action.PolicyVersion, FeatureKey: action.FeatureKey,
			Category: string(action.Category), Origin: action.Origin,
			Modality: string(action.Modality), Provider: req.Provider, Model: req.Model,
			IdempotencyKey: metering.IdempotencyKey, Attempt: attempt,
			Status: model.AIActionExecutionRunning, Metadata: mustJSONMetadata(metering.Metadata),
			StartedAt: time.Now().UTC(),
		})
		if err != nil {
			return nil, fmt.Errorf("start embedding audit: %w", err)
		}
	}
	estimatedInputTokens := estimateEmbeddingInputTokens(req.Inputs)
	if p.meter != nil {
		if err := p.meter.PreflightUsage(ctx, AIUsageMeterInput{
			WorkspaceID: metering.WorkspaceID, FeatureKey: action.FeatureKey,
			IdempotencyKey: metering.IdempotencyKey, InputTokens: estimatedInputTokens,
			Metadata: metering.Metadata,
		}); err != nil {
			p.finishAudit(ctx, execution, estimatedInputTokens, err)
			return nil, err
		}
	}
	response, callErr := p.base.CreateEmbeddings(ctx, req)
	if callErr == nil && response != nil && p.meter != nil {
		_, callErr = p.meter.Consume(ctx, AIUsageMeterInput{
			WorkspaceID: metering.WorkspaceID, FeatureKey: action.FeatureKey,
			IdempotencyKey: metering.IdempotencyKey, InputTokens: estimatedInputTokens,
			Metadata: metering.Metadata,
		})
	}
	p.finishAudit(ctx, execution, estimatedInputTokens, callErr)
	return response, callErr
}

func (p *governedEmbeddingProvider) finishAudit(ctx context.Context, execution *model.AIActionExecution, inputTokens int, callErr error) {
	if p.audit == nil || execution == nil {
		return
	}
	result := aipolicy.ExecutionResult{
		Status: model.AIActionExecutionSucceeded, InputTokens: inputTokens,
		CompletedAt: time.Now().UTC(),
	}
	if callErr != nil {
		result.Status = model.AIActionExecutionFailed
		result.FailureClass = aiActionFailureClass(callErr)
		result.FailureMessage = sanitizeAIActionFailure(callErr)
	}
	_ = p.audit.Finish(ctx, execution.ID, result)
}

func estimateEmbeddingInputTokens(inputs []string) int {
	characters := 0
	for _, input := range inputs {
		characters += len(input)
	}
	return (characters + 3) / 4
}

func withAIActionMetering(ctx context.Context, workspaceID, actionKey, operation, identity string, metadata map[string]interface{}) context.Context {
	action, ok := aipolicy.DefaultRegistry().Lookup(actionKey)
	if !ok {
		// Preserve the unknown action so the governed provider returns the
		// registry error before any external request is made.
		action.FeatureKey = "unknown_ai_action"
	}
	return WithAIUsageMetering(ctx, AIUsageMeteringContext{
		WorkspaceID: workspaceID,
		ActionKey:   actionKey,
		FeatureKey:  action.FeatureKey,
		IdempotencyKey: aiUsageIdempotencyKey(
			workspaceID,
			operation,
			aiUsageStableHash(identity),
		),
		Metadata: metadata,
	})
}

var _ llm.EmbeddingProvider = (*governedEmbeddingProvider)(nil)
