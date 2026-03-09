package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMCompanyHandler handles CRM company HTTP endpoints.
type CRMCompanyHandler struct {
	companyService *service.CRMCompanyService
}

// NewCRMCompanyHandler creates a new CRMCompanyHandler.
func NewCRMCompanyHandler(companyService *service.CRMCompanyService) *CRMCompanyHandler {
	return &CRMCompanyHandler{companyService: companyService}
}

// List handles GET /api/crm/companies.
func (h *CRMCompanyHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMCompanyListFilters{
		Industry:      queryStringPtr(r, "industry"),
		OwnerMemberID: queryStringPtr(r, "owner_member_id"),
		Search:        queryStringPtr(r, "search"),
	}
	pagination := queryPagination(r)

	companies, total, err := h.companyService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if companies == nil {
		companies = []model.CRMCompany{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  companies,
		"total": total,
		"page":  pagination.Page,
	})
}

// Create handles POST /api/crm/companies.
func (h *CRMCompanyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMCompanyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	company, err := h.companyService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, company)
}

// Get handles GET /api/crm/companies/{id}.
func (h *CRMCompanyHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	company, err := h.companyService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, company)
}

// Update handles PUT /api/crm/companies/{id}.
func (h *CRMCompanyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMCompanyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	company, err := h.companyService.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, company)
}

// Delete handles DELETE /api/crm/companies/{id}.
func (h *CRMCompanyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.companyService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "company deleted"})
}
