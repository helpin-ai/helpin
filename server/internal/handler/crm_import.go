package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMImportHandler handles CRM import HTTP endpoints.
type CRMImportHandler struct {
	importService *service.CRMImportService
}

// NewCRMImportHandler creates a new CRMImportHandler.
func NewCRMImportHandler(importService *service.CRMImportService) *CRMImportHandler {
	return &CRMImportHandler{importService: importService}
}

// Create handles POST /api/crm/imports.
func (h *CRMImportHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMImportRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	job, err := h.importService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

// Get handles GET /api/crm/imports/{id}.
func (h *CRMImportHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, err := h.importService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// List handles GET /api/crm/imports.
func (h *CRMImportHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	pagination := queryPagination(r)
	jobs, total, err := h.importService.List(r.Context(), workspaceID, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if jobs == nil {
		jobs = []model.CRMImportJob{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  jobs,
		"total": total,
		"page":  pagination.Page,
	})
}

// Process handles POST /api/crm/imports/{id}/process.
func (h *CRMImportHandler) Process(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.ProcessCRMImportRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	job, err := h.importService.Process(r.Context(), id, req)
	if err != nil {
		var entitlementErr *service.EntitlementError
		if errors.As(err, &entitlementErr) {
			writeError(w, http.StatusPaymentRequired, entitlementErr.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}
