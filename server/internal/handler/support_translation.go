package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (h *SupportInboxHandler) TranslationOptions(w http.ResponseWriter, r *http.Request) {
	result, err := h.supportService.TranslationOptions(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Translation settings are unavailable for this conversation.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *SupportInboxHandler) TranslateMessage(w http.ResponseWriter, r *http.Request) {
	var req model.SupportTranslateRequest
	if err := decodeJSON(r, &req); err != nil || req.MessageID == "" {
		writeError(w, http.StatusBadRequest, "Invalid translation request.")
		return
	}
	result, err := h.supportService.TranslateSupport(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Translation unavailable. Check your language settings and try again.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
