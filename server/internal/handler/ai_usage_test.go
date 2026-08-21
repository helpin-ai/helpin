package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
)

func TestAIUsagePricingHandlerReturnsCanonicalPublicCatalog(t *testing.T) {
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	NewAIUsageHandler(catalog).Pricing(recorder, httptest.NewRequest(http.MethodGet, "/api/ai-pricing", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, expected := range []string{`"pricing_version":"2026-08-19"`, `"key":"small"`, `"allowance_microusd":99000000`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %s: %s", expected, body)
		}
	}
	if strings.Contains(body, "ceiling_rates") || strings.Contains(body, "source_urls") {
		t.Fatalf("response exposes internal catalog fields: %s", body)
	}
}
