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
	AnalyzeDraft(context.Context, string, model.PMTriageDraftRequest) (*model.PMTriageView, error)
	Analyze(context.Context, string, string, string) (*model.PMTriageView, error)
}

// PMTriageHandler exposes reviewed PM decision suggestions.
type PMTriageHandler struct {
	service  pmTriageAnalyzer
	reviewer pmTriageReviewer
}

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

type pmTriageReviewer interface {
	Review(context.Context, string, string, string, model.PMTriageReviewRequest) (*model.PMTriageReviewResult, error)
}

// SetReviewer attaches the explicitly reviewed mutation flow.
func (h *PMTriageHandler) SetReviewer(reviewer pmTriageReviewer) { h.reviewer = reviewer }

// ReviewTask applies or dismisses a suggestion from a task assessment.
func (h *PMTriageHandler) ReviewTask(w http.ResponseWriter, r *http.Request) { h.review(w, r, "task") }

// ReviewConversation applies or dismisses a support-to-task match.
func (h *PMTriageHandler) ReviewConversation(w http.ResponseWriter, r *http.Request) {
	h.review(w, r, "support_conversation")
}

func (h *PMTriageHandler) review(w http.ResponseWriter, r *http.Request, kind string) {
	workspaceID, id := getWorkspaceID(r), chi.URLParam(r, "id")
	if _, err := uuid.Parse(workspaceID); err != nil {
		writeError(w, http.StatusBadRequest, "valid workspace_id is required")
		return
	}
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "valid source ID is required")
		return
	}
	var req model.PMTriageReviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid review request")
		return
	}
	if _, err := uuid.Parse(req.AssessmentID); err != nil {
		writeError(w, http.StatusBadRequest, "valid assessment ID is required")
		return
	}
	if (req.Action != "task_type" && req.Action != "team" && req.Action != "match") || len(req.Value) == 0 || len(req.Value) > 100 {
		writeError(w, http.StatusBadRequest, "invalid suggestion")
		return
	}
	if h.reviewer == nil {
		writeError(w, http.StatusServiceUnavailable, "Suggestion review is unavailable.")
		return
	}
	result, err := h.reviewer.Review(r.Context(), workspaceID, kind, id, req)
	if err != nil {
		var forbidden *model.ErrForbidden
		if errors.As(err, &forbidden) {
			writeError(w, http.StatusForbidden, "You do not have access to review this work.")
			return
		}
		if errors.Is(err, service.ErrPMTriageStale) || errors.Is(err, service.ErrPMTriageSourceUnavailable) {
			writeError(w, http.StatusConflict, "This work has changed. Refresh suggestions before applying them.")
			return
		}
		slog.ErrorContext(r.Context(), "review PM triage", "error", err, "workspace_id", workspaceID, "source_id", id)
		writeError(w, http.StatusBadRequest, "This suggestion could not be applied. Check the task and try again.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// AnalyzeDraft checks unsaved work against accessible candidate tasks.
func (h *PMTriageHandler) AnalyzeDraft(w http.ResponseWriter, r *http.Request) {
	var req model.PMTriageDraftRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid task draft")
		return
	}
	if _, err := uuid.Parse(req.DraftID); err != nil {
		writeError(w, http.StatusBadRequest, "valid draft_id is required")
		return
	}
	if len(req.Name) == 0 || len(req.Name) > 1000 || len(req.Description) > 64000 {
		writeError(w, http.StatusBadRequest, "task draft is empty or too large")
		return
	}
	workspaceID := getWorkspaceID(r)
	if _, err := uuid.Parse(workspaceID); err != nil {
		writeError(w, http.StatusBadRequest, "valid workspace_id is required")
		return
	}
	result, err := h.service.AnalyzeDraft(r.Context(), workspaceID, req)
	if err != nil {
		var forbidden *model.ErrForbidden
		if errors.As(err, &forbidden) {
			writeError(w, http.StatusForbidden, "You do not have access to triage this work.")
			return
		}
		slog.ErrorContext(r.Context(), "triage task draft", "error", err, "workspace_id", workspaceID)
		writeError(w, http.StatusServiceUnavailable, "Task matching is unavailable. You can continue without suggestions.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
