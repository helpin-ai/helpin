package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
)

type dockMediaCompletionFunc func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error)

func (f dockMediaCompletionFunc) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return f(ctx, req)
}

func TestDockChatMediaCompletionAllowsLongAnalysis(t *testing.T) {
	provider := dockMediaCompletionFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) < 3*time.Minute+50*time.Second || time.Until(deadline) > 4*time.Minute {
			t.Fatalf("media provider deadline = %v, want approximately four minutes", deadline)
		}
		action, ok := aipolicy.DefaultRegistry().Lookup(aipolicy.ActionAskMediaEnrichment)
		if !ok || action.Timeout != 4*time.Minute {
			t.Fatalf("media policy timeout = %v, want four minutes", action.Timeout)
		}
		return &llm.ChatResponse{Content: "The screenshot shows an upload error."}, nil
	})
	s := (&DockChatService{}).SetMediaAnalyzer(&PMAttachmentService{}, provider)
	result, err := s.analyzeDockChatMedia(context.Background(), "ws", "user", "What went wrong?", []dockChatMediaAttachment{
		{ID: "image", FileType: "image/png", URL: "https://example.com/screenshot.png"},
	})
	if err != nil || result != "The screenshot shows an upload error." {
		t.Fatalf("media analysis = %q, %v", result, err)
	}
}

func TestDockChatMediaCompletionPreservesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := dockMediaCompletionFunc(func(callCtx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		cancel()
		return nil, callCtx.Err()
	})
	s := (&DockChatService{}).SetMediaAnalyzer(&PMAttachmentService{}, provider)
	_, err := s.analyzeDockChatMedia(ctx, "ws", "user", "Describe this", []dockChatMediaAttachment{
		{ID: "image", FileType: "image/png", URL: "https://example.com/screenshot.png"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("media analysis error = %v, want caller cancellation", err)
	}
}

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
		t.Fatalf("media providers = %v, want explicit Flex endpoints", options.Only)
	}
	if store.reconcile.Entry.CanonicalModel != "gemini-3.8-flash" || store.reconcile.Entry.ModelTier != string(aiusage.TierMedium) {
		t.Fatalf("media ledger = %+v", store.reconcile.Entry)
	}
	if len(audit.started) != 1 || len(audit.finished) != 1 || audit.started[0].ActionKey != aipolicy.ActionAskMediaEnrichment {
		t.Fatalf("media audit start/finish = %d/%d", len(audit.started), len(audit.finished))
	}
}
