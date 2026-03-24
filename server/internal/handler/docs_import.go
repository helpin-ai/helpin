package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ImportListJobs handles GET /api/docs/import/jobs.
func (h *DocsHandler) ImportListJobs(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	jobs, err := h.importService.ListJobs(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

// ImportReconvert handles POST /api/docs/import/{jobId}/reconvert.
func (h *DocsHandler) ImportReconvert(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	result, err := h.importService.Reconvert(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ImportPreviewHelpscout handles POST /api/docs/import/helpscout/preview.
func (h *DocsHandler) ImportPreviewHelpscout(w http.ResponseWriter, r *http.Request) {
	var req model.DocsImportPreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.importService.Preview(r.Context(), req.APIKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ImportStartHelpscout handles POST /api/docs/import/helpscout/start.
func (h *DocsHandler) ImportStartHelpscout(w http.ResponseWriter, r *http.Request) {
	var req model.DocsImportStartRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	userID := middleware.GetUserID(r.Context())
	// Get workspace_id from query param (standard pattern in this codebase).
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	jobID, err := h.importService.Start(r.Context(), req, workspaceID, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

// ImportGetStatus handles GET /api/docs/import/{jobId}/status.
func (h *DocsHandler) ImportGetStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	job, err := h.importService.GetStatus(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if job == nil {
		writeError(w, http.StatusNotFound, "import job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// ImportRetry handles POST /api/docs/import/{jobId}/retry.
func (h *DocsHandler) ImportRetry(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.importService.Retry(r.Context(), jobID, req.APIKey); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "retry started"})
}

// ImportGetRedirectMap handles GET /api/docs/import/{jobId}/redirect-map.
func (h *DocsHandler) ImportGetRedirectMap(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	data, err := h.importService.GetRedirectMap(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="redirect-map.json"`)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
