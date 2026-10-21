package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CancelConversationFollowUp cancels scheduled AI work without taking ownership.
func (h *SupportInboxHandler) CancelConversationFollowUp(w http.ResponseWriter, r *http.Request) {
	var req model.CancelSupportFollowUpRequest
	if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "Invalid follow-up request")
		return
	}
	conversation, err := h.supportService.CancelConversationFollowUp(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req, middleware.GetUserID(r.Context()))
	if errors.Is(err, repository.ErrSupportFollowUpChanged) {
		writeError(w, http.StatusConflict, "This follow-up has changed. Refresh the conversation and try again.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "Unable to cancel the follow-up for this conversation")
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

// PreviewConversationFollowUps returns a read-only backlog preview for support administrators.
func (h *SupportInboxHandler) PreviewConversationFollowUps(w http.ResponseWriter, r *http.Request) {
	preview, err := h.supportService.PreviewConversationFollowUps(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Unable to preview support follow-ups")
		return
	}
	writeJSON(w, http.StatusOK, preview)
}
