//go:build ee

package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportGreetingMetersLunaAndAuditsOneCompletion(t *testing.T) {
	svc, _, conv, source, processing, settings, provider := setupSupportGreetingTest(t)
	store := &fakeAIUsageStore{}
	audit := &gatewayFakeAudit{}
	svc.supportAIService.llmProvider = newTestAICompletionService(t, provider, store).SetGovernance(aipolicy.DefaultRegistry(), audit)
	handled, err := svc.replyToInitialGreeting(context.Background(), conv, source, &model.Agent{ID: "agent"}, processing, settings)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if store.reserveCalls != 1 || store.reconcile.Entry.ModelTier != "small" || store.reconcile.Entry.FeatureKey != BillingFeatureSupportAIReply || store.reconcile.Entry.CanonicalModel != "gpt-5.6-luna" {
		t.Fatalf("unexpected usage: reserves=%d entry=%+v", store.reserveCalls, store.reconcile.Entry)
	}
	if len(audit.started) != 1 || len(audit.finished) != 1 || audit.started[0].Model != "openai/gpt-5.6-luna" {
		t.Fatalf("audit start/finish=%+v/%+v", audit.started, audit.finished)
	}
}
