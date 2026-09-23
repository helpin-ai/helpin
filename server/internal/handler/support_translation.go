package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
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

func (h *SupportInboxHandler) SetLiveTranslate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled          *bool  `json:"enabled"`
		CustomerLanguage string `json:"customer_language"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "Invalid translation settings.")
		return
	}
	result, err := h.supportService.SetLiveTranslate(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), *req.Enabled, req.CustomerLanguage)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Could not update Live Translate.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *SupportInboxHandler) CachedLiveTranslations(w http.ResponseWriter, r *http.Request) {
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	result, err := h.supportService.CachedLiveTranslations(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), ids)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Translation unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *SupportInboxHandler) QueueSupportSend(w http.ResponseWriter, r *http.Request) {
	var req model.CreateMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid reply.")
		return
	}
	result, err := h.supportService.QueueSupportSend(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), req)
	if err != nil {
		if errors.Is(err, service.ErrSupportTranslation) {
			writeError(w, http.StatusUnprocessableEntity, "Translation unavailable. Your draft has been preserved.")
			return
		}
		writeError(w, http.StatusBadRequest, "Could not queue reply. Your draft has been preserved.")
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}
func (h *SupportInboxHandler) PendingSupportSends(w http.ResponseWriter, r *http.Request) {
	result, err := h.supportService.PendingSupportSends(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Could not load pending replies.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *SupportInboxHandler) RetrySupportSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid action.")
		return
	}
	err := h.supportService.RetrySupportSend(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), chi.URLParam(r, "send_id"), req.Action)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Could not update reply.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
