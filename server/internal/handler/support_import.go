package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportImportHandler imports conversations from other help desks.
type SupportImportHandler struct {
	imports *service.SupportImportService
}

func NewSupportImportHandler(imports *service.SupportImportService) *SupportImportHandler {
	return &SupportImportHandler{imports: imports}
}

func writeSupportImportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrIntercomImportInvalid):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrIntercomImportInvalid.Error()+": "))
	case errors.Is(err, repository.ErrSupportImportJobNotFound):
		writeError(w, http.StatusNotFound, "import not found")
	default:
		slog.Error("support import", "error", err)
		writeError(w, http.StatusBadGateway, "Intercom could not be reached. Try again in a moment.")
	}
}

// PreviewIntercom handles POST /api/support/imports/intercom/preview.
func (h *SupportImportHandler) PreviewIntercom(w http.ResponseWriter, r *http.Request) {
	var req model.IntercomConversationImportRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	preview, err := h.imports.Preview(r.Context(), req)
	if err != nil {
		writeSupportImportError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

// StartIntercom handles POST /api/support/imports/intercom.
func (h *SupportImportHandler) StartIntercom(w http.ResponseWriter, r *http.Request) {
	var req model.IntercomConversationImportRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	job, err := h.imports.Start(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), req)
	if err != nil {
		writeSupportImportError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

// List handles GET /api/support/imports.
func (h *SupportImportHandler) List(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.imports.ListJobs(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeSupportImportError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

// Cancel handles POST /api/support/imports/{importId}/cancel.
func (h *SupportImportHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "importId")
	if !isUUID(id) {
		writeError(w, http.StatusNotFound, "import not found")
		return
	}
	if err := h.imports.Cancel(r.Context(), getWorkspaceID(r), id); err != nil {
		writeSupportImportError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
