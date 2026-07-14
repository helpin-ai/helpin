package handler

import (
	"log/slog"
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// TLSAskHandler answers Caddy on-demand TLS "ask" checks. Caddy calls
// GET ...?domain=<fqdn> during the TLS handshake for hostnames it has no
// certificate for; 200 approves issuance, any other status denies it. The
// endpoint is unauthenticated and hammered by scanners, so denials return
// early and log at debug level at most.
type TLSAskHandler struct {
	service *service.TLSAskService
}

// NewTLSAskHandler builds the handler.
func NewTLSAskHandler(svc *service.TLSAskService) *TLSAskHandler {
	return &TLSAskHandler{service: svc}
}

// Verify approves certificate issuance for registered help center custom
// domains and first-party hosts, and denies everything else with 404.
func (h *TLSAskHandler) Verify(w http.ResponseWriter, r *http.Request) {
	domain, err := service.NormalizeTLSAskDomain(r.URL.Query().Get("domain"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	allowed, err := h.service.Allowed(r.Context(), domain)
	if err != nil {
		slog.ErrorContext(r.Context(), "tls ask lookup failed", "error", err, "domain", domain)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !allowed {
		slog.DebugContext(r.Context(), "tls ask denied", "domain", domain)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}
