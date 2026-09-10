package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (h *CRMDealHandler) pipelineForWorkspace(w http.ResponseWriter, r *http.Request, id string) (*model.CRMPipeline, bool) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return nil, false
	}
	pipeline, err := h.dealService.GetPipeline(r.Context(), id)
	if err != nil {
		writeCRMPipelineError(w, r, err)
		return nil, false
	}
	if pipeline.WorkspaceID != workspaceID {
		writeError(w, http.StatusNotFound, "pipeline not found")
		return nil, false
	}
	return pipeline, true
}

func writeCRMPipelineError(w http.ResponseWriter, r *http.Request, err error) {
	var validation *model.CRMPipelineValidationError
	switch {
	case errors.Is(err, model.ErrCRMPipelineNotFound):
		writeError(w, http.StatusNotFound, model.ErrCRMPipelineNotFound.Error())
	case errors.Is(err, model.ErrCRMPipelineConflict):
		writeError(w, http.StatusConflict, model.ErrCRMPipelineConflict.Error())
	case errors.As(err, &validation):
		writeError(w, http.StatusBadRequest, validation.Message)
	default:
		slog.ErrorContext(r.Context(), "CRM pipeline request failed", "error", err, "workspace_id", getWorkspaceID(r))
		writeError(w, http.StatusInternalServerError, "unable to save or load the pipeline; try again")
	}
}
