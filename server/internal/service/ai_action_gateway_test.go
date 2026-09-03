package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestGovernedLLMProviderRejectsMissingCoverageActionBeforeProvider(t *testing.T) {
	base := &gatewayFakeProvider{}
	provider := NewGovernedLLMProvider(base, nil, aipolicy.DefaultRegistry(), &gatewayFakeAudit{})
	ctx := WithAIUsageMeteringExempt(WithAIUsageMetering(context.Background(), AIUsageMeteringContext{
		WorkspaceID: "ws-1", FeatureKey: BillingFeatureCoverageGapAnalysis,
		IdempotencyKey: "coverage-1",
	}))
	_, err := provider.ChatCompletion(ctx, llm.ChatRequest{Provider: "openai", Model: "gpt-5.5", MaxTokens: 100})
	if !errors.Is(err, aipolicy.ErrActionRequired) {
		t.Fatalf("ChatCompletion() error = %v, want ErrActionRequired", err)
	}
	if base.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", base.calls)
	}
}

func TestGovernedLLMProviderRejectsDisallowedRouteBeforeProvider(t *testing.T) {
	base := &gatewayFakeProvider{}
	audit := &gatewayFakeAudit{}
	provider := NewGovernedLLMProvider(base, nil, aipolicy.DefaultRegistry(), audit)
	ctx := WithAIUsageMeteringExempt(WithAIUsageMetering(context.Background(), AIUsageMeteringContext{
		WorkspaceID: "ws-1", FeatureKey: BillingFeatureCoverageGapAnalysis,
		ActionKey: aipolicy.ActionSupportCoverageAnalyze, IdempotencyKey: "coverage-2",
	}))
	_, err := provider.ChatCompletion(ctx, llm.ChatRequest{
		Provider: "unknown", Model: "unapproved", MaxTokens: 100,
	})
	if !errors.Is(err, aipolicy.ErrRouteNotAllowed) {
		t.Fatalf("ChatCompletion() error = %v, want ErrRouteNotAllowed", err)
	}
	if base.calls != 0 || len(audit.started) != 0 {
		t.Fatalf("provider/audit calls = %d/%d, want 0/0", base.calls, len(audit.started))
	}
}

func TestGovernedLLMProviderAuditsSuccessfulCoverageCall(t *testing.T) {
	base := &gatewayFakeProvider{response: &llm.ChatResponse{
		Content: "{}", Provider: "openai", Model: "gpt-5.5",
		TokensUsed: llm.TokenUsage{InputTokens: 12, OutputTokens: 4},
	}}
	audit := &gatewayFakeAudit{}
	provider := NewGovernedLLMProvider(base, nil, aipolicy.DefaultRegistry(), audit)
	ctx := WithAIUsageMeteringExempt(WithAIUsageMetering(context.Background(), AIUsageMeteringContext{
		WorkspaceID: "ws-1", FeatureKey: BillingFeatureCoverageGapAnalysis,
		ActionKey: aipolicy.ActionSupportCoverageAnalyze, IdempotencyKey: "coverage-3", Attempt: 2,
	}))
	if _, err := provider.ChatCompletion(ctx, llm.ChatRequest{
		Provider: "openai", Model: "gpt-5.5", MaxTokens: 100,
	}); err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if base.calls != 1 || len(audit.started) != 1 || len(audit.finished) != 1 {
		t.Fatalf("provider/start/finish = %d/%d/%d, want 1/1/1",
			base.calls, len(audit.started), len(audit.finished))
	}
	started := audit.started[0]
	if started.ActionKey != aipolicy.ActionSupportCoverageAnalyze || started.Category != "Support AI" ||
		started.Origin != "coverage" || started.Attempt != 2 {
		t.Fatalf("audit start = %+v", started)
	}
	if audit.finished[0].Status != model.AIActionExecutionSucceeded ||
		audit.finished[0].InputTokens != 12 || audit.finished[0].OutputTokens != 4 {
		t.Fatalf("audit finish = %+v", audit.finished[0])
	}
}

func TestGovernedEmbeddingProviderRequiresEmbeddingActionAndAudits(t *testing.T) {
	base := &gatewayFakeEmbeddingProvider{response: &llm.EmbeddingResponse{
		Vectors: [][]float32{{0.1, 0.2}},
	}}
	audit := &gatewayFakeAudit{}
	provider := NewGovernedEmbeddingProvider(base, nil, aipolicy.DefaultRegistry(), audit)
	ctx := WithAIUsageMeteringExempt(WithAIUsageMetering(context.Background(), AIUsageMeteringContext{
		WorkspaceID: "ws-1", FeatureKey: BillingFeatureCoverageGapAnalysis,
		ActionKey: aipolicy.ActionSupportCoverageEmbed, IdempotencyKey: "embed-1",
	}))
	if _, err := provider.CreateEmbeddings(ctx, llm.EmbeddingRequest{
		Provider: "openai", Model: "text-embedding-3-small", Inputs: []string{"customer need"},
	}); err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}
	if base.calls != 1 || len(audit.started) != 1 || len(audit.finished) != 1 {
		t.Fatalf("provider/start/finish = %d/%d/%d, want 1/1/1",
			base.calls, len(audit.started), len(audit.finished))
	}
	if audit.started[0].Modality != string(aipolicy.ModalityEmbedding) {
		t.Fatalf("audit modality = %q", audit.started[0].Modality)
	}
}

func TestWithAIActionMeteringUsesRegistryFeatureAndStableIdentity(t *testing.T) {
	ctx := withAIActionMetering(context.Background(), "workspace-1", aipolicy.ActionDocsEmbed, "docs_sync", "document-1", map[string]interface{}{"surface": "docs"})
	metering, ok := AIUsageMeteringFromContext(ctx)
	if !ok {
		t.Fatal("expected valid metering context")
	}
	if metering.ActionKey != aipolicy.ActionDocsEmbed || metering.FeatureKey != "semantic_embedding" {
		t.Fatalf("unexpected action context: %#v", metering)
	}
	if metering.IdempotencyKey == "" {
		t.Fatal("expected stable idempotency key")
	}
}

func TestGovernedRerankerRequiresPolicyContextAndAudits(t *testing.T) {
	base := &gatewayFakeReranker{}
	audit := &gatewayFakeAudit{}
	reranker := NewGovernedSupportKnowledgeReranker(base, aipolicy.DefaultRegistry(), audit)
	if _, err := reranker.Rerank(context.Background(), "query", []SupportRerankCandidate{{ID: "1", Text: "text"}}); !errors.Is(err, ErrAIUsageMeteringRequired) {
		t.Fatalf("Rerank() error = %v, want ErrAIUsageMeteringRequired", err)
	}
	ctx := withAIActionMetering(context.Background(), "workspace-1", aipolicy.ActionPlatformRerank, "support_rerank", "query", map[string]interface{}{"surface": "support_search"})
	if _, err := reranker.Rerank(ctx, "query", []SupportRerankCandidate{{ID: "1", Text: "text"}}); err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}
	if base.calls != 1 || len(audit.started) != 1 || len(audit.finished) != 1 {
		t.Fatalf("provider/start/finish = %d/%d/%d, want 1/1/1", base.calls, len(audit.started), len(audit.finished))
	}
}

type gatewayFakeProvider struct {
	calls    int
	response *llm.ChatResponse
	err      error
}

type gatewayFakeEmbeddingProvider struct {
	calls    int
	response *llm.EmbeddingResponse
	err      error
}

type gatewayFakeReranker struct{ calls int }

func (r *gatewayFakeReranker) Rerank(_ context.Context, _ string, _ []SupportRerankCandidate) ([]SupportRerankScore, error) {
	r.calls++
	return []SupportRerankScore{{Index: 0, Score: 1}}, nil
}

func (r *gatewayFakeReranker) Name() string { return "test-reranker" }

func (p *gatewayFakeEmbeddingProvider) CreateEmbeddings(_ context.Context, _ llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	p.calls++
	return p.response, p.err
}

func (p *gatewayFakeProvider) ChatCompletion(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	p.calls++
	return p.response, p.err
}

type gatewayFakeAudit struct {
	started  []model.AIActionExecution
	finished []aipolicy.ExecutionResult
}

func (a *gatewayFakeAudit) Start(_ context.Context, execution *model.AIActionExecution) (*model.AIActionExecution, error) {
	a.started = append(a.started, *execution)
	if execution.ID == "" {
		execution.ID = "audit-1"
	}
	return execution, nil
}

func (a *gatewayFakeAudit) Finish(_ context.Context, _ string, result aipolicy.ExecutionResult) error {
	a.finished = append(a.finished, result)
	return nil
}

var _ llm.Provider = (*gatewayFakeProvider)(nil)
var _ llm.EmbeddingProvider = (*gatewayFakeEmbeddingProvider)(nil)
var _ aipolicy.ExecutionAudit = (*gatewayFakeAudit)(nil)
