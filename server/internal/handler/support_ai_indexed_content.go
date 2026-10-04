package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// ListIndexedKnowledgeDocuments returns articles indexed within the selected source.
func (h *SupportAIHandler) ListIndexedKnowledgeDocuments(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	documents, err := h.knowledgeSourceSvc.ListIndexedDocuments(r.Context(), workspaceID, chi.URLParam(r, "id"), chi.URLParam(r, "sourceId"))
	if errors.Is(err, service.ErrKnowledgeSourceNotFound) {
		writeError(w, http.StatusNotFound, "Knowledge source is unavailable")
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "list indexed knowledge documents failed", "workspace_id", workspaceID, "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load indexed content")
		return
	}
	writeJSON(w, http.StatusOK, documents)
}
