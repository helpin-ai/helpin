package service

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
)

type scriptedAICompletionProvider struct {
	requests  []llm.ChatRequest
	responses map[string]*llm.ChatResponse
	errors    map[string]error
}

func (p *scriptedAICompletionProvider) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.requests = append(p.requests, req)
	if err := p.errors[req.Model]; err != nil {
		return nil, err
	}
	if response := p.responses[req.Model]; response != nil {
		copy := *response
		return &copy, nil
	}
	return &llm.ChatResponse{
		Content: "ok", TokensUsed: llm.TokenUsage{InputTokensTotal: 10, CompletionTokensTotal: 5, OutputTokens: 5},
	}, nil
}

func validAICompletionRequest() AICompletionRequest {
	return AICompletionRequest{
		WorkspaceID: "ws-1", FeatureKey: BillingFeatureCRMSummary,
		IdempotencyKey: "crm-summary:1", Chat: llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: "summarize"}}, MaxTokens: 100},
	}
}

type cancelRewriteProvider struct{ cancel context.CancelFunc }

func (p *cancelRewriteProvider) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	p.cancel()
	return nil, &llm.ProviderError{Err: context.DeadlineExceeded}
}

type cancelAwareRewriteAudit struct{ gatewayFakeAudit }

func (a *cancelAwareRewriteAudit) Finish(ctx context.Context, id string, result aipolicy.ExecutionResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.gatewayFakeAudit.Finish(ctx, id, result)
}
