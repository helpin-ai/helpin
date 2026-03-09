package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMActivityHandler handles CRM activity HTTP endpoints.
type CRMActivityHandler struct {
	activityService *service.CRMActivityService
}

// NewCRMActivityHandler creates a new CRMActivityHandler.
func NewCRMActivityHandler(activityService *service.CRMActivityService) *CRMActivityHandler {
	return &CRMActivityHandler{activityService: activityService}
}

// List handles GET /api/crm/activities.
func (h *CRMActivityHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMActivityListFilters{
		ActivityType: queryStringPtr(r, "activity_type"),
		ContactID:    queryStringPtr(r, "contact_id"),
		CompanyID:    queryStringPtr(r, "company_id"),
		DealID:       queryStringPtr(r, "deal_id"),
	}
	pagination := queryPagination(r)

	activities, total, err := h.activityService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if activities == nil {
		activities = []model.CRMActivity{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  activities,
		"total": total,
		"page":  pagination.Page,
	})
}

// Create handles POST /api/crm/activities.
func (h *CRMActivityHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMActivityRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	activity, err := h.activityService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, activity)
}

// Get handles GET /api/crm/activities/{id}.
func (h *CRMActivityHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	activity, err := h.activityService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, activity)
}

// Update handles PUT /api/crm/activities/{id}.
func (h *CRMActivityHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMActivityRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	activity, err := h.activityService.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, activity)
}

// Delete handles DELETE /api/crm/activities/{id}.
func (h *CRMActivityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.activityService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "activity deleted"})
}

// ListByContact handles GET /api/crm/contacts/{id}/activities.
func (h *CRMActivityHandler) ListByContact(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	contactID := chi.URLParam(r, "id")
	filters := model.CRMActivityListFilters{ContactID: &contactID}
	pagination := queryPagination(r)

	activities, total, err := h.activityService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if activities == nil {
		activities = []model.CRMActivity{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  activities,
		"total": total,
		"page":  pagination.Page,
	})
}

// ListByCompany handles GET /api/crm/companies/{id}/activities.
func (h *CRMActivityHandler) ListByCompany(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	companyID := chi.URLParam(r, "id")
	filters := model.CRMActivityListFilters{CompanyID: &companyID}
	pagination := queryPagination(r)

	activities, total, err := h.activityService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if activities == nil {
		activities = []model.CRMActivity{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  activities,
		"total": total,
		"page":  pagination.Page,
	})
}

// ListByDeal handles GET /api/crm/deals/{id}/activities.
func (h *CRMActivityHandler) ListByDeal(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	dealID := chi.URLParam(r, "id")
	filters := model.CRMActivityListFilters{DealID: &dealID}
	pagination := queryPagination(r)

	activities, total, err := h.activityService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if activities == nil {
		activities = []model.CRMActivity{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  activities,
		"total": total,
		"page":  pagination.Page,
	})
}
