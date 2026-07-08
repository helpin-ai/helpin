package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakeWorkspaceContextFetcher struct {
	pages map[string]string
}

func (f fakeWorkspaceContextFetcher) FetchText(ctx context.Context, rawURL string) (string, error) {
	return f.pages[rawURL], nil
}

type fakeWorkspaceContextLLM struct {
	lastRequest llm.ChatRequest
}

func (f *fakeWorkspaceContextLLM) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.lastRequest = req
	return &llm.ChatResponse{
		Content: "# Acme\n\nAcme helps support and product teams understand customers.\n",
	}, nil
}

func TestWorkspaceServiceGenerateCompanyProductDescriptionUsesDirectWebsiteFetch(t *testing.T) {
	llmProvider := &fakeWorkspaceContextLLM{}
	svc := NewWorkspaceService(nil, nil, nil).
		SetContextGeneratorDependencies(llmProvider, fakeWorkspaceContextFetcher{
			pages: map[string]string{
				"https://acme.com":          "Acme is a customer intelligence platform.",
				"https://acme.com/features": "Features include support answers, product analytics, and docs automation.",
				"https://acme.com/about":    "Built for support, product, founders, and growth teams.",
			},
		})

	resp, err := svc.GenerateCompanyProductDescription(context.Background(), model.GenerateWorkspaceContextDescriptionRequest{
		WorkspaceName: "Acme",
		WebsiteURL:    "acme.com",
	})
	if err != nil {
		t.Fatalf("GenerateCompanyProductDescription() error = %v", err)
	}
	if !strings.Contains(resp.Description, "Acme helps support") {
		t.Fatalf("description = %q, want generated markdown", resp.Description)
	}
	if len(llmProvider.lastRequest.Messages) != 1 || !strings.Contains(llmProvider.lastRequest.Messages[0].Content, "support answers") {
		t.Fatalf("LLM prompt did not include fetched website text: %#v", llmProvider.lastRequest.Messages)
	}
	if strings.Contains(llmProvider.lastRequest.Messages[0].Content, "RAG") {
		t.Fatalf("LLM prompt should be direct website context, got %q", llmProvider.lastRequest.Messages[0].Content)
	}
}
