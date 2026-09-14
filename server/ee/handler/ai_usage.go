package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/ee/pricing"
)

// AIUsageHandler exposes customer-safe, canonical AI pricing.
type AIUsageHandler struct{ catalog *pricing.Catalog }

// NewAIUsageHandler creates the public pricing handler.
func NewAIUsageHandler(catalog *pricing.Catalog) *AIUsageHandler {
	return &AIUsageHandler{catalog: catalog}
}

// Pricing returns the catalog snapshot used by backend billing.
func (h *AIUsageHandler) Pricing(w http.ResponseWriter, _ *http.Request) {
	if h == nil || h.catalog == nil {
		writeError(w, http.StatusServiceUnavailable, "AI pricing is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, h.catalog.PublicSnapshot())
}
