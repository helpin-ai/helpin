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
func (h *SupportInboxHandler) SaveTranslationPreference(w http.ResponseWriter, r *http.Request) {
	var req model.SupportTranslationPreference
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid translation preference.")
		return
	}
	if err := h.supportService.SaveTranslationPreference(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), req); err != nil {
		writeError(w, http.StatusBadRequest, "Could not save translation preference.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}
func (h *SupportInboxHandler) SaveTranslationConversation(w http.ResponseWriter, r *http.Request) {
	var req model.SupportTranslationConversation
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid translation settings.")
		return
	}
	if err := h.supportService.SaveTranslationConversation(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), req); err != nil {
		writeError(w, http.StatusBadRequest, "Could not save translation settings.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
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
