package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ChangeConversationAIControl handles teammate-owned pause/return transitions.
func (h *SupportInboxHandler) ChangeConversationAIControl(w http.ResponseWriter, r *http.Request) {
	var req model.SupportAIControlRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	err := h.supportService.ChangeConversationAIControl(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), req)
	if errors.Is(err, repository.ErrSupportAIControlConflict) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		slog.WarnContext(r.Context(), "change support AI control", "error", err)
		switch err.Error() {
		case "a teammate is required", "conversation not found", "action must be pause, return, or run_now", "only a visible portal request can be sent to AI now", "automatic portal replies must be enabled to ask AI to handle a request", "support AI is not enabled", "support AI agent is unavailable", "this conversation is read-only because its customer was deleted", "this conversation is not eligible for AI; reopen it and check the enabled reply channels", "confirm returning to AI: the customer requested a human", "reopen the conversation before changing AI control":
			writeError(w, http.StatusBadRequest, err.Error())
		case repository.ErrPortalAINoPendingCustomerMessage.Error():
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "Could not change AI control. Refresh and try again.")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"updated": true})
}
