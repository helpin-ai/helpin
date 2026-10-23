package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ListConversationSharedItems handles GET
// /api/support/inbox/conversations/{id}/shared-items: the help-center
// articles and files the team sent to the customer in the conversation.
func (h *SupportInboxHandler) ListConversationSharedItems(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	conversationID := chi.URLParam(r, "id")

	items, err := h.supportService.ListConversationSharedItems(r.Context(), workspaceID, conversationID)
	if err != nil {
		if err.Error() == "conversation not found" {
			writeError(w, http.StatusNotFound, "conversation not found")
			return
		}
		slog.ErrorContext(r.Context(), "list conversation shared items", "error", err, "workspace_id", workspaceID, "conversation_id", conversationID)
		writeError(w, http.StatusInternalServerError, "could not load shared items")
		return
	}
	writeJSON(w, http.StatusOK, items)
}
