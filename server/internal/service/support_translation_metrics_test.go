package service

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/observability"
)

func TestTranslationCompletionRecordsUsageEvenWhenOutputInvalid(t *testing.T) {
	for _, tc := range []struct {
		name, finish string
		invalid      bool
	}{{"truncated", "length", false}, {"invalid", "stop", true}} {
		t.Run(tc.name, func(t *testing.T) {
			model := "deepseek/deepseek-v4-flash-0731"
			provider := &scriptedAICompletionProvider{responses: map[string]*llm.ChatResponse{model: {Content: "bad output", FinishReason: tc.finish, TokensUsed: llm.TokenUsage{InputTokensTotal: 100, OutputTokens: 10}}}}
			store := &recordingCommunityUsage{}
			metrics := observability.NewMetrics()
			svc := NewAICompletionService(provider, NewCommunityAIUsage(store), DefaultAICompletionRouteRegistry()).SetMetrics(metrics)
			req := AICompletionRequest{WorkspaceID: "ws", FeatureKey: BillingFeatureSupportTranslation, IdempotencyKey: "translation-test", RequireComplete: true, Chat: llm.ChatRequest{MaxTokens: 100}}
			if tc.invalid {
				req.ValidateResponse = func(*llm.ChatResponse) error { return errors.New("invalid") }
			}
			if _, err := svc.Complete(context.Background(), req); err == nil {
				t.Fatal("invalid output accepted")
			}
			if len(store.entries) != 1 || store.entries[0].InputTokens != 100 {
				t.Fatalf("durable usage=%+v", store.entries)
			}
			w := httptest.NewRecorder()
			metrics.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
			body := w.Body.String()
			if !strings.Contains(body, `helpin_translation_provider_attempts_total{model="deepseek/deepseek-v4-flash-0731",outcome="invalid_output",provider="openrouter",stage="translation"} 1`) || !strings.Contains(body, `helpin_translation_tokens_total{kind="input",model="deepseek/deepseek-v4-flash-0731",provider="openrouter",stage="translation"} 100`) {
				t.Fatalf("missing failed-attempt telemetry: %s", body)
			}
		})
	}
}
