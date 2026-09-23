package service

import (
	"context"
	"errors"
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
	requests    []llm.ChatRequest
	failures    map[string]error
}

func (f *fakeWorkspaceContextLLM) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.lastRequest = req
	f.requests = append(f.requests, req)
	if err := f.failures[req.Provider]; err != nil {
		return nil, err
	}
	return &llm.ChatResponse{
		Content: "Product: **Acme** helps support and product teams understand customers.\nCustomers:\n- Support teams\nKey capabilities:\n1. Support answers\n",
	}, nil
}

func TestWorkspaceServiceGenerateCompanyProductDescriptionReturnsOpenRouterFailureWithoutChangingModels(t *testing.T) {
	llmProvider := &fakeWorkspaceContextLLM{failures: map[string]error{
		"openrouter": errors.New("openrouter unavailable"),
	}}
	svc := NewWorkspaceService(nil, nil, nil).
		SetContextGeneratorDependencies(llmProvider, fakeWorkspaceContextFetcher{pages: map[string]string{
			"https://acme.com": "Acme is a customer intelligence platform.",
		}})

	_, err := svc.GenerateCompanyProductDescription(context.Background(), "", model.GenerateWorkspaceContextDescriptionRequest{
		WorkspaceName: "Acme",
		WebsiteURL:    "https://acme.com",
	})
	if err == nil || !strings.Contains(err.Error(), "openrouter unavailable") {
		t.Fatalf("GenerateCompanyProductDescription() error = %v, want OpenRouter failure", err)
	}
	if len(llmProvider.requests) != 1 {
		t.Fatalf("LLM requests = %d, want one OpenRouter attempt", len(llmProvider.requests))
	}
	if got := llmProvider.requests[0]; got.Provider != "openrouter" || got.Model != "z-ai/glm-5.3-flash:exacto" {
		t.Fatalf("route = %q/%q, want openrouter/z-ai/glm-5.3-flash:exacto", got.Provider, got.Model)
	}
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

	resp, err := svc.GenerateCompanyProductDescription(context.Background(), "", model.GenerateWorkspaceContextDescriptionRequest{
		WorkspaceName: "Acme",
		WebsiteURL:    "acme.com",
	})
	if err != nil {
		t.Fatalf("GenerateCompanyProductDescription() error = %v", err)
	}
	if strings.Contains(resp.Description, "#") || strings.Contains(resp.Description, "**") {
		t.Fatalf("description = %q, want plain text without markdown syntax", resp.Description)
	}
	if strings.Contains(resp.Description, "Product:") || strings.Contains(resp.Description, "Customers:") {
		t.Fatalf("description = %q, want product label removed and customers renamed", resp.Description)
	}
	if !strings.Contains(resp.Description, "- Acme helps support") || !strings.Contains(resp.Description, "Audience:") || !strings.Contains(resp.Description, "- Support answers") {
		t.Fatalf("description = %q, want clean dash-pointer structure", resp.Description)
	}
	if !strings.Contains(resp.Description, "Acme helps support") {
		t.Fatalf("description = %q, want generated plain text", resp.Description)
	}
	if len(llmProvider.lastRequest.Messages) != 1 || !strings.Contains(llmProvider.lastRequest.Messages[0].Content, "support answers") {
		t.Fatalf("LLM prompt did not include fetched website text: %#v", llmProvider.lastRequest.Messages)
	}
	if llmProvider.lastRequest.Provider != "openrouter" || llmProvider.lastRequest.Model != "z-ai/glm-5.3-flash:exacto" {
		t.Fatalf("LLM route = %q/%q, want openrouter/z-ai/glm-5.3-flash:exacto", llmProvider.lastRequest.Provider, llmProvider.lastRequest.Model)
	}
	promptText := strings.ToLower(llmProvider.lastRequest.SystemPrompt + "\n" + llmProvider.lastRequest.Messages[0].Content)
	if strings.Contains(promptText, "markdown") {
		t.Fatalf("LLM prompt should request plain text, got system=%q user=%q", llmProvider.lastRequest.SystemPrompt, llmProvider.lastRequest.Messages[0].Content)
	}
	if strings.Contains(llmProvider.lastRequest.Messages[0].Content, "Product:") ||
		strings.Contains(llmProvider.lastRequest.Messages[0].Content, "Customers:") ||
		strings.Contains(llmProvider.lastRequest.Messages[0].Content, "Constraints:") ||
		!strings.Contains(llmProvider.lastRequest.Messages[0].Content, "Audience:") ||
		!strings.Contains(llmProvider.lastRequest.Messages[0].Content, "Competitors:") {
		t.Fatalf("LLM prompt should omit Product/Customers/Constraints and use Audience/Competitors, got %q", llmProvider.lastRequest.Messages[0].Content)
	}
	if strings.Contains(llmProvider.lastRequest.Messages[0].Content, "RAG") {
		t.Fatalf("LLM prompt should be direct website context, got %q", llmProvider.lastRequest.Messages[0].Content)
	}
}

func TestWorkspaceServiceGenerateCompanyProductDescriptionProvidesAIUsageContext(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	llmProvider := NewMeteredLLMProvider(&fakeWorkspaceContextLLM{}, NewTokenPricedAIUsageMeter(consumer))
	svc := NewWorkspaceService(nil, nil, nil).
		SetContextGeneratorDependencies(llmProvider, fakeWorkspaceContextFetcher{
			pages: map[string]string{
				"https://acme.com": "Acme is a customer intelligence platform.",
			},
		})

	resp, err := svc.GenerateCompanyProductDescription(context.Background(), "", model.GenerateWorkspaceContextDescriptionRequest{
		WorkspaceName: "Acme",
		WebsiteURL:    "https://acme.com",
	})
	if err != nil {
		t.Fatalf("GenerateCompanyProductDescription() error = %v", err)
	}
	if resp.CompanyProductContext == "" {
		t.Fatal("CompanyProductContext = empty, want generated context")
	}
	if !consumer.preflight.Promotional {
		t.Fatalf("setup context generation should not consume credits, got preflight=%#v consume=%#v", consumer.preflight, consumer.input)
	}
}

func (p *fakeWorkspaceContextLLM) ResolvePricingIdentity(req llm.ChatRequest) (llm.ChatPricingIdentity, error) {
	return testMeterIdentity(req), nil
}
