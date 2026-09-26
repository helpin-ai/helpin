//go:build ee

package service

import (
	"context"
	"testing"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCompleteWithManagedProfileIsMeteredAsHostedUsage(t *testing.T) {
	store := &fakeAIUsageStore{}
	router := &scriptedAICompletionProvider{}
	audit := &gatewayFakeAudit{}
	completions := newTestAICompletionService(t, router, store).SetGovernance(aipolicy.DefaultRegistry(), audit)
	client := &scriptedAICompletionProvider{}
	execution := &AIProfileExecution{
		Selection: &model.AIExecutionSelection{ProfileID: "standard-small", Source: "workspace",
			Route:  model.AIProfileRoute{ConnectionID: "managed", Model: sdk.RunModel{Provider: "openrouter", Model: "z-ai/glm-5.3-flash:exacto"}},
			Policy: &model.AIExecutionPolicySnapshot{Mode: "ee", FundingMode: aiusage.FundingHelpinHosted}},
		Client: client,
	}
	_, err := completions.CompleteWithProfile(context.Background(), AICompletionRequest{
		WorkspaceID: "ws-1", FeatureKey: BillingFeatureCompanyProductContext, IdempotencyKey: "context:1",
		Chat: llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: "describe"}}, MaxTokens: 100},
	}, execution)
	if err != nil {
		t.Fatalf("CompleteWithProfile() error = %v", err)
	}
	if len(router.requests) != 0 || len(client.requests) != 1 || client.requests[0].Model != "z-ai/glm-5.3-flash:exacto" {
		t.Fatalf("router=%d profile=%+v, want only the profile client", len(router.requests), client.requests)
	}
	// Setup context generation is promotional: recorded uncharged, never reserved.
	entry := store.uncharged
	if store.reserveCalls != 0 || entry.Provider != "openrouter" || entry.FundingMode != string(aiusage.FundingHelpinHosted) || entry.WorkspaceID != "ws-1" {
		t.Fatalf("reserves=%d ledger entry = %+v, want uncharged hosted usage for the workspace", store.reserveCalls, entry)
	}
	if len(audit.started) != 1 || audit.started[0].Provider != "openrouter" || audit.started[0].FeatureKey != BillingFeatureCompanyProductContext {
		t.Fatalf("audit = %+v", audit.started)
	}
}
