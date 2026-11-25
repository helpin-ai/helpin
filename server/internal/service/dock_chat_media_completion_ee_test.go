//go:build ee

package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"slices"
	"strings"
	"testing"
)

func TestDockChatMediaCompletionUsesGovernedGemini38WithinCurrentPrices(t *testing.T) {
	store := &fakeAIUsageStore{}
	audit := &gatewayFakeAudit{}
	provider := &scriptedAICompletionProvider{}
	completion := newTestAICompletionService(t, provider, store).
		SetGovernance(aipolicy.DefaultRegistry(), audit)
	s := (&DockChatService{}).SetMediaAnalyzer(&PMAttachmentService{}, completion)
	_, err := s.analyzeDockChatMedia(context.Background(), "ws", "user", "What went wrong?", []dockChatMediaAttachment{
		{ID: "image", FileType: "image/png", URL: "https://example.com/screenshot.png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("provider requests = %d, want one", len(provider.requests))
	}
	req := provider.requests[0]
	if req.Model != "google/gemini-3.8-flash" || req.MaxTokens != 700 {
		t.Fatalf("media route/output limit = %q/%d", req.Model, req.MaxTokens)
	}
	var options struct {
		Sort     string   `json:"sort"`
		Only     []string `json:"only"`
		MaxPrice struct {
			Prompt     float64 `json:"prompt"`
			Completion float64 `json:"completion"`
		} `json:"max_price"`
	}
	if err := json.Unmarshal(req.ProviderOptions, &options); err != nil {
		t.Fatal(err)
	}
	if options.Sort != "latency" || options.MaxPrice.Prompt != 0.375 || options.MaxPrice.Completion != 1.875 {
		t.Fatalf("media provider options = %+v", options)
	}
	if !slices.Equal(options.Only, []string{"google-ai-studio/flex", "google-vertex/global/flex"}) {
		t.Fatalf("media providers = %v, want existing Flex providers", options.Only)
	}
	if store.reconcile.Entry.CanonicalModel != "gemini-3.8-flash" || store.reconcile.Entry.ModelTier != string(aiusage.TierMedium) {
		t.Fatalf("media ledger = %+v", store.reconcile.Entry)
	}
	if len(audit.started) != 1 || len(audit.finished) != 1 || audit.started[0].ActionKey != aipolicy.ActionAskMediaEnrichment {
		t.Fatalf("media audit start/finish = %d/%d", len(audit.started), len(audit.finished))
	}
}

func TestDockChatMediaFallsBackToQwenOnRateLimit(t *testing.T) {
	store := &fakeAIUsageStore{}
	provider := &scriptedAICompletionProvider{errors: map[string]error{"google/gemini-3.8-flash": &llm.ProviderError{StatusCode: 429}}}
	audit := &gatewayFakeAudit{}
	completion := newTestAICompletionService(t, provider, store).SetGovernance(aipolicy.DefaultRegistry(), audit)
	s := (&DockChatService{}).SetMediaAnalyzer(&PMAttachmentService{}, completion)
	_, err := s.analyzeDockChatMedia(context.Background(), "ws", "user", "Describe this", []dockChatMediaAttachment{{ID: "video", FileType: "video/mp4", URL: "https://example.com/test.mp4"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("requests = %d", len(provider.requests))
	}
	req := provider.requests[1]
	if req.Model != "qwen/qwen3.8-omni-flash" {
		t.Fatalf("fallback = %s", req.Model)
	}
	var opts struct {
		Only     []string `json:"only"`
		MaxPrice struct {
			Prompt     float64 `json:"prompt"`
			Completion float64 `json:"completion"`
		} `json:"max_price"`
	}
	if err := json.Unmarshal(req.ProviderOptions, &opts); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(opts.Only, []string{"alibaba"}) || opts.MaxPrice.Prompt != 0.15 || opts.MaxPrice.Completion != 0.47 {
		t.Fatalf("fallback options = %+v", opts)
	}
	if req.Reasoning == nil || req.Reasoning.Enabled == nil || *req.Reasoning.Enabled {
		t.Fatal("fallback reasoning must be disabled")
	}
	// Each attachment is preceded by a label part, so look for the video by type.
	if !slices.ContainsFunc(req.Messages[0].ContentParts, func(part llm.ContentPart) bool { return part.Type == "video_url" }) {
		t.Fatal("fallback lost video")
	}
	if store.reconcile.Entry.CanonicalModel != "qwen3.8-omni-flash" {
		t.Fatalf("fallback charged as %s", store.reconcile.Entry.CanonicalModel)
	}
	if len(audit.finished) != 2 {
		t.Fatalf("audit attempts = %d", len(audit.finished))
	}
}

func TestTaskAttachmentAnalysisUsesGovernedMediaRoute(t *testing.T) {
	store := &fakeAIUsageStore{}
	audit := &gatewayFakeAudit{}
	provider := &scriptedAICompletionProvider{responses: map[string]*llm.ChatResponse{
		"google/gemini-3.8-flash": {Content: "Friday", FinishReason: "stop", TokensUsed: llm.TokenUsage{InputTokensTotal: 10, CompletionTokensTotal: 5, OutputTokens: 5}},
	}}
	completion := newTestAICompletionService(t, provider, store).SetGovernance(aipolicy.DefaultRegistry(), audit)
	file := mediaAttachment{ID: "scan", FileName: "scan.pdf", FileType: "application/pdf", URL: "https://storage.example/scan.pdf"}
	_, err := analyzeAttachmentMedia(context.Background(), completion, "ws", BillingFeatureAskChat, aipolicy.ActionTaskAttachmentAnalysis, "What is the due date?", []mediaAttachment{file})
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) != 1 || provider.requests[0].Model != "google/gemini-3.8-flash" || store.reconcile.Entry.CanonicalModel != "gemini-3.8-flash" {
		t.Fatalf("unexpected task analysis route or ledger: %+v", store.reconcile.Entry)
	}
	if len(audit.finished) != 1 || len(audit.started) != 1 || audit.started[0].ActionKey != aipolicy.ActionTaskAttachmentAnalysis {
		t.Fatalf("unexpected task analysis audit: %+v", audit.started)
	}
	provider.errors = map[string]error{"google/gemini-3.8-flash": &llm.ProviderError{StatusCode: 429}}
	_, err = analyzeAttachmentMedia(context.Background(), completion, "ws", BillingFeatureAskChat, aipolicy.ActionTaskAttachmentAnalysis, "What is the title?", []mediaAttachment{file})
	if err == nil || len(provider.requests) != 2 || provider.requests[1].Model != "google/gemini-3.8-flash" {
		t.Fatalf("PDF should only reach the native PDF route: calls=%d err=%v", len(provider.requests), err)
	}
	delete(provider.errors, "google/gemini-3.8-flash")
	provider.responses["google/gemini-3.8-flash"].FinishReason = "length"
	answer, err := analyzeAttachmentMedia(context.Background(), completion, "ws", BillingFeatureAskChat, aipolicy.ActionTaskAttachmentAnalysis, "List all the fields.", []mediaAttachment{file})
	if err == nil || answer != "" {
		t.Fatalf("truncated analysis: answer=%q err=%v", answer, err)
	}
	last := audit.finished[len(audit.finished)-1]
	if last.FailureClass != "llm_contract" || !strings.Contains(last.FailureMessage, `finish_reason="length"`) || last.OutputTokens != 5 {
		t.Fatalf("expected an audited incomplete answer with token usage, got %+v", last)
	}
}
