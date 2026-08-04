package handler

// Public help center AI endpoints: grounded answers over published articles
// and thumbs feedback. Generated content is never indexable and never cached
// at the HTTP layer (the service has its own content-fingerprinted cache).

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PublicAnswerQuestion handles POST /api/hc/{subdomain}/answer (and the
// locale-prefixed variant).
func (h *DocsHandler) PublicAnswerQuestion(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	if h.aiSearchSvc == nil || !cfg.AIAnswersEnabled {
		writeError(w, http.StatusNotFound, "AI answers are not enabled")
		return
	}
	locale, ok := h.resolveRequestedPublicLocale(w, r, cfg)
	if !ok {
		return
	}
	var req model.HelpcenterAnswerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	outcome, err := h.aiSearchSvc.Answer(r.Context(), cfg.WorkspaceID, locale, publicAnswerFallbackLocale(cfg, locale), req.Query, strings.TrimSpace(req.SpaceSlug))
	if err != nil {
		if strings.Contains(err.Error(), "too short") || strings.Contains(err.Error(), "too long") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "could not generate an answer")
		return
	}
	if outcome.BudgetBlocked {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "the daily AI answer limit for this help center was reached")
		return
	}

	response := outcome.Response
	sourceSignal := model.SupportCoverageSourceSelfService
	if response.Status != model.HelpcenterAnswerStatusAnswered && service.IsMeaningfulCoverageSearchQuery(req.Query) {
		sourceSignal = "no_results"
	}
	if !response.Cached {
		h.recordSupportEvent(service.SupportEventInput{
			WorkspaceID:  cfg.WorkspaceID,
			EventType:    model.SupportEventWidgetSearchPerformed,
			ActorType:    model.SupportEventActorCustomer,
			Channel:      "helpcenter",
			SourceSignal: sourceSignal,
			IssueSummary: req.Query,
			Metadata: map[string]any{
				"type":   "ai_answer",
				"query":  req.Query,
				"status": response.Status,
			},
		})
	}

	w.Header().Set("X-Robots-Tag", "noindex")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, response)
}

// PublicAnswerFeedback handles
// POST /api/hc/{subdomain}/answer/{answerID}/feedback.
func (h *DocsHandler) PublicAnswerFeedback(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	if h.aiSearchSvc == nil {
		writeError(w, http.StatusNotFound, "AI answers are not enabled")
		return
	}
	var req model.HelpcenterAnswerFeedbackRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.aiSearchSvc.RecordAnswerFeedback(r.Context(), cfg.WorkspaceID, chi.URLParam(r, "answerID"), req.IsHelpful); err != nil {
		writeError(w, http.StatusNotFound, "answer not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// publicAnswerFallbackLocale returns the locale used to resolve article
// references when the requested locale has no published translation.
func publicAnswerFallbackLocale(cfg *model.DocsHelpcenterConfig, locale string) string {
	if cfg.FallbackToDefaultLocale && cfg.DefaultLocale != locale {
		return cfg.DefaultLocale
	}
	return ""
}
