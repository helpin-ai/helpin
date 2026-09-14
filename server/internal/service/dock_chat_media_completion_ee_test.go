//go:build ee

package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"slices"
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
		t.Fatalf("media providers = %v, want explicit Flex endpoints", options.Only)
	}
	if store.reconcile.Entry.CanonicalModel != "gemini-3.8-flash" || store.reconcile.Entry.ModelTier != string(aiusage.TierMedium) {
		t.Fatalf("media ledger = %+v", store.reconcile.Entry)
	}
	if len(audit.started) != 1 || len(audit.finished) != 1 || audit.started[0].ActionKey != aipolicy.ActionAskMediaEnrichment {
		t.Fatalf("media audit start/finish = %d/%d", len(audit.started), len(audit.finished))
	}
}
