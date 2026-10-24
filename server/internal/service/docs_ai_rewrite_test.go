package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type docsRewriteCompleter struct {
	request AICompletionRequest
	err     error
	calls   int
}

func (p *docsRewriteCompleter) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	panic("must use governed completion")
}
func (p *docsRewriteCompleter) Complete(_ context.Context, req AICompletionRequest) (*llm.ChatResponse, error) {
	p.request = req
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return &llm.ChatResponse{Content: `{"content":"<p>A <strong>shorter</strong> update.</p>"}`}, nil
}
func TestDocsRewriteActionsUseGovernedCompletion(t *testing.T) {
	for _, action := range []string{"improve", "fix_grammar", "shorten", "expand", "simplify", "professional", "friendly", "casual", "custom"} {
		t.Run(action, func(t *testing.T) {
			p := &docsRewriteCompleter{}
			result, err := NewDocsAIRewriteService(p).Rewrite(context.Background(), "workspace", model.DocsAIRewriteRequest{Content: "<p>Original update.</p>", Operation: action, Instructions: "Use plain language"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(result.Content, "<strong>") {
				t.Fatal("formatting lost")
			}
			if p.request.FeatureKey != BillingFeatureDocsSelectionRewrite || p.request.WorkspaceID != "workspace" || !p.request.RequireComplete {
				t.Fatalf("missing governed completion: %+v", p.request)
			}
			if action == "custom" && !strings.Contains(p.request.Chat.Messages[0].Content, "Use plain language") {
				t.Fatal("custom instruction missing")
			}
		})
	}
}
func TestDocsRewriteRejectsInvalidInputBeforeAI(t *testing.T) {
	for _, req := range []model.DocsAIRewriteRequest{
		{Content: "text", Operation: "translate"}, {Content: " ", Operation: "shorten"}, {Content: "text", Operation: "custom"},
		{Content: strings.Repeat("x", 24001), Operation: "shorten"}, {Content: "text", Operation: "custom", Instructions: strings.Repeat("x", 2001)},
	} {
		p := &docsRewriteCompleter{}
		_, err := NewDocsAIRewriteService(p).Rewrite(context.Background(), "workspace", req)
		if !errors.Is(err, ErrDocsRewriteInvalidInput) || p.calls != 0 {
			t.Fatalf("invalid input called AI: %v", err)
		}
	}
}
func TestDocsRewritePreservesUsageErrors(t *testing.T) {
	denied := errors.New("usage denied")
	p := &docsRewriteCompleter{err: denied}
	_, err := NewDocsAIRewriteService(p).Rewrite(context.Background(), "workspace", model.DocsAIRewriteRequest{Content: "text", Operation: "shorten"})
	if !errors.Is(err, denied) {
		t.Fatalf("lost usage error: %v", err)
	}
}

func TestDocsRewriteTracksEachRetryAsANewGeneration(t *testing.T) {
	p := &docsRewriteCompleter{}
	svc := NewDocsAIRewriteService(p)
	req := model.DocsAIRewriteRequest{Content: "original", Operation: "shorten"}
	if _, err := svc.Rewrite(context.Background(), "workspace", req); err != nil {
		t.Fatal(err)
	}
	first := p.request.IdempotencyKey
	if _, err := svc.Rewrite(context.Background(), "workspace", req); err != nil {
		t.Fatal(err)
	}
	if first == p.request.IdempotencyKey {
		t.Fatal("retry reused the previous generation's usage key")
	}
	feature, ok := AIUsageFeature(BillingFeatureDocsSelectionRewrite)
	if !ok || feature.FeatureKey != BillingFeatureDocsSelectionRewrite {
		t.Fatal("rewrite missing from usage feature registry")
	}
}
