package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type pmTriageAnalyzer interface {
	Analyze(context.Context, string, string, string) (*model.PMTriageView, error)
}

// PMTriageHandler exposes reviewed PM decision suggestions.
type PMTriageHandler struct{ service pmTriageAnalyzer }

// NewPMTriageHandler constructs the task and support triage endpoints.
func NewPMTriageHandler(triage pmTriageAnalyzer) *PMTriageHandler {
	return &PMTriageHandler{service: triage}
}

// AnalyzeTask handles POST /api/pm/tasks/{id}/triage.
func (h *PMTriageHandler) AnalyzeTask(w http.ResponseWriter, r *http.Request) {
	h.analyze(w, r, "task")
}

// AnalyzeConversation handles POST /api/support/inbox/conversations/{id}/task-triage.
func (h *PMTriageHandler) AnalyzeConversation(w http.ResponseWriter, r *http.Request) {
	h.analyze(w, r, "support_conversation")
}

func (h *PMTriageHandler) analyze(w http.ResponseWriter, r *http.Request, kind string) {
	workspaceID, id := getWorkspaceID(r), chi.URLParam(r, "id")
	if _, err := uuid.Parse(workspaceID); err != nil {
		writeError(w, http.StatusBadRequest, "valid workspace_id is required")
		return
	}
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "valid source ID is required")
		return
	}
	result, err := h.service.Analyze(r.Context(), workspaceID, kind, id)
	if err != nil {
		var forbidden *model.ErrForbidden
		if errors.As(err, &forbidden) {
			writeError(w, http.StatusForbidden, "You do not have access to triage this work.")
			return
		}
		if errors.Is(err, service.ErrPMTriageSourceUnavailable) {
			writeError(w, http.StatusNotFound, "This source is no longer available.")
			return
		}
		slog.ErrorContext(r.Context(), "evaluate PM triage", "error", err, "workspace_id", workspaceID, "source_kind", kind, "source_id", id)
		writeError(w, http.StatusServiceUnavailable, "Task suggestions are temporarily unavailable. You can continue without them.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
