package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMDealHandler handles CRM deal and pipeline HTTP endpoints.
type CRMDealHandler struct {
	dealService *service.CRMDealService
}

// NewCRMDealHandler creates a new CRMDealHandler.
func NewCRMDealHandler(dealService *service.CRMDealService) *CRMDealHandler {
	return &CRMDealHandler{dealService: dealService}
}

// ── Pipeline endpoints ──

// ListPipelines handles GET /api/crm/pipelines.
func (h *CRMDealHandler) ListPipelines(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	pipelines, err := h.dealService.ListPipelines(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if pipelines == nil {
		pipelines = []model.CRMPipeline{}
	}
	writeJSON(w, http.StatusOK, pipelines)
}

// CreatePipeline handles POST /api/crm/pipelines.
func (h *CRMDealHandler) CreatePipeline(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMPipelineRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	pipeline, err := h.dealService.CreatePipeline(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, pipeline)
}

// GetPipeline handles GET /api/crm/pipelines/{id}.
func (h *CRMDealHandler) GetPipeline(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pipeline, err := h.dealService.GetPipeline(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pipeline)
}

// UpdatePipeline handles PUT /api/crm/pipelines/{id}.
func (h *CRMDealHandler) UpdatePipeline(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMPipelineRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pipeline, err := h.dealService.UpdatePipeline(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pipeline)
}

// DeletePipeline handles DELETE /api/crm/pipelines/{id}.
func (h *CRMDealHandler) DeletePipeline(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.dealService.DeletePipeline(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "pipeline deleted"})
}

// ── Deal endpoints ──

// List handles GET /api/crm/deals.
func (h *CRMDealHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMDealListFilters{
		PipelineID:    queryStringPtr(r, "pipeline_id"),
		StageID:       queryStringPtr(r, "stage_id"),
		OwnerMemberID: queryStringPtr(r, "owner_member_id"),
		Search:        queryStringPtr(r, "search"),
	}
	pagination := queryPagination(r)

	deals, total, err := h.dealService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if deals == nil {
		deals = []model.CRMDeal{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  deals,
		"total": total,
		"page":  pagination.Page,
	})
}

// Create handles POST /api/crm/deals.
func (h *CRMDealHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMDealRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	deal, err := h.dealService.CreateWithActor(r.Context(), req, middleware.GetUserID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, deal)
}

// Get handles GET /api/crm/deals/{id}.
func (h *CRMDealHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	deal, err := h.dealService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, deal)
}

// Update handles PUT /api/crm/deals/{id}.
func (h *CRMDealHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMDealRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	deal, err := h.dealService.UpdateWithActor(r.Context(), id, req, middleware.GetUserID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, deal)
}

// Delete handles DELETE /api/crm/deals/{id}.
func (h *CRMDealHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.dealService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "deal deleted"})
}
