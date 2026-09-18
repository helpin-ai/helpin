package observability

import (
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTranslationMetricsNormalizeDisjointTokensAndMissingPrices(t *testing.T) {
	m := NewMetrics()
	u := &aiusage.TokenTelemetry{InputTokensTotal: 100, CacheReadTokens: 30, CacheWriteTokens: 10, CompletionTokensTotal: 50, ReasoningTokens: 20, CompletionIncludesReasoning: true}
	m.TranslationAttempt("translation", "openrouter", "deepseek", "invalid_output", time.Second, u, aiusage.TokenRates{InputMicrousdPerMillion: 1000000, CacheReadMicrousdPerMillion: 500000, CacheWriteMicrousdPerMillion: 1000000, OutputMicrousdPerMillion: 2000000})
	m.TranslationAttempt("jev_review", "typesafe", "jev", "error", time.Second, nil, aiusage.TokenRates{})
	m.TranslationAttempt("jev_review", "typesafe", "jev", "success", time.Second, u, aiusage.TokenRates{})
	m.TranslationEvent("message_display", "cache_hit")
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()
	for _, want := range []string{
		`helpin_translation_tokens_total{kind="input",model="deepseek",provider="openrouter",stage="translation"} 60`,
		`helpin_translation_tokens_total{kind="output",model="deepseek",provider="openrouter",stage="translation"} 30`,
		`helpin_translation_tokens_total{kind="reasoning",model="deepseek",provider="openrouter",stage="translation"} 20`,
		`helpin_translation_estimated_provider_cost_usd_total{model="deepseek",provider="openrouter",stage="translation"} 0.000185`,
		`helpin_translation_usage_measurements_total{measurement="price_missing",stage="jev_review"} 1`,
		`helpin_translation_usage_measurements_total{measurement="tokens_missing",stage="jev_review"} 1`,
		`helpin_translation_events_total{direction="message_display",outcome="cache_hit"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, forbidden := range []string{"workspace_id=", "conversation_id=", "source_text=", "target_language="} {
		if strings.Contains(body, forbidden) {
			t.Errorf("unbounded label %s", forbidden)
		}
	}
	if strings.Contains(body, `helpin_translation_estimated_provider_cost_usd_total{model="jev"`) {
		t.Fatal("unknown price reported as zero")
	}
}
