package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// CancelConversationFollowUp cancels scheduled AI work without taking ownership.
func (h *SupportInboxHandler) CancelConversationFollowUp(w http.ResponseWriter, r *http.Request) {
	conversation, err := h.supportService.CancelConversationFollowUp(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
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
