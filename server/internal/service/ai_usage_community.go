package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// AIExecutionUsageStore records usage without depending on billing tables.
type AIExecutionUsageStore interface {
	RecordExecutionUsage(context.Context, model.AIExecutionUsage, model.JSONBlob) error
}

// CommunityAIUsage implements the complete lifecycle with no financial effects.
type CommunityAIUsage struct{ store AIExecutionUsageStore }

// NewCommunityAIUsage creates a usage recorder that needs no model price catalog.
func NewCommunityAIUsage(store AIExecutionUsageStore) *CommunityAIUsage {
	return &CommunityAIUsage{store: store}
}

var _ AIUsageLifecycle = (*CommunityAIUsage)(nil)

func (s *CommunityAIUsage) ChargeForTokens(metering MeteringContext, _ aiusage.NormalizedTokens) (int64, error) {
	if metering.PolicyMode != "community" {
		return 0, fmt.Errorf("community usage requires its accepted policy")
	}
	return 0, nil
}

// ResolveMeteringContext pins execution identity, including custom unpriced models.
func (s *CommunityAIUsage) ResolveMeteringContext(input MeteringRequest) (MeteringContext, error) {
	if strings.TrimSpace(input.Provider) == "" || strings.TrimSpace(input.Model) == "" || strings.TrimSpace(input.IdempotencyKey) == "" {
		return MeteringContext{}, fmt.Errorf("provider, model, and usage identity are required")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" && !input.Promotional {
		return MeteringContext{}, fmt.Errorf("usage workspace is required")
	}
	route := input.Route
	if route == "" {
		route = input.Model
	}
	return MeteringContext{
		PolicyMode: "community", WorkspaceID: input.WorkspaceID, TaskNature: input.TaskNature,
		FeatureKey: input.FeatureKey, OperationKey: input.OperationKey, IdempotencyKey: input.IdempotencyKey,
		FundingMode: input.FundingMode, Promotional: input.Promotional,
		Route: aiusage.ResolvedRoute{Provider: input.Provider, CanonicalModel: input.Model, Route: route, ServiceTier: input.ServiceTier},
	}, nil
}

// Preflight validates execution identity without reserving credits.
func (s *CommunityAIUsage) Preflight(ctx context.Context, input PreflightRequest) (*MeteringContext, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("usage recorder is required")
	}
	resolved, err := s.ResolveMeteringContext(input.Metering)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

// Reconcile records terminal telemetry and the run watermark without charging.
func (s *CommunityAIUsage) Reconcile(ctx context.Context, input CompletionUsage) (*UsageResult, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("usage recorder is required")
	}
	if input.Context.PolicyMode != "community" {
		return nil, fmt.Errorf("community usage requires its accepted policy")
	}
	tokens, err := aiusage.NormalizeTokens(input.Telemetry)
	if err != nil {
		return nil, err
	}
	for _, tool := range input.PaidTools {
		if tool.Key == "" || tool.Count < 0 {
			return nil, fmt.Errorf("invalid paid-tool usage")
		}
	}
	tools, err := json.Marshal(input.PaidTools)
	if err != nil {
		return nil, err
	}
	entry := model.AIExecutionUsage{
		WorkspaceID: input.Context.WorkspaceID, IdempotencyKey: input.Context.IdempotencyKey,
		RunID: input.RunID, FeatureKey: input.Context.FeatureKey, Provider: input.Context.Route.Provider,
		Model: input.Context.Route.Route, InputTokens: tokens.InputTokensTotal,
		OutputTokens: tokens.OutputTokens, ReasoningTokens: tokens.ReasoningTokens,
		CacheReadTokens: tokens.CacheReadTokens, CacheWriteTokens: tokens.CacheWriteTokens,
		MeasurementStatus: input.MeasurementStatus, PaidTools: tools,
	}
	if err := s.store.RecordExecutionUsage(ctx, entry, input.RunOutputSummary); err != nil {
		return nil, err
	}
	return &UsageResult{}, nil
}

// Checkpoint records an interactive usage delta with the same idempotency contract.
func (s *CommunityAIUsage) Checkpoint(ctx context.Context, input CompletionUsage) (*UsageResult, error) {
	return s.Reconcile(ctx, input)
}

// Heartbeat honors cancellation; community has no financial hold to renew.
func (s *CommunityAIUsage) Heartbeat(ctx context.Context, _ MeteringContext) error { return ctx.Err() }

// SuspendReservation honors cancellation without allocating or releasing credits.
func (s *CommunityAIUsage) SuspendReservation(ctx context.Context, _ MeteringContext) error {
	return ctx.Err()
}

// Fail needs no financial cleanup; execution failure remains in the core run/audit.
func (s *CommunityAIUsage) Fail(ctx context.Context, _ string) error { return ctx.Err() }

// Release needs no financial cleanup and remains safe to retry.
func (s *CommunityAIUsage) Release(ctx context.Context, _, _ string) error { return ctx.Err() }
