package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMCalendarHandler handles CRM calendar HTTP endpoints.
type CRMCalendarHandler struct {
	calendarService *service.CRMCalendarService
}

// NewCRMCalendarHandler creates a new CRMCalendarHandler.
func NewCRMCalendarHandler(calendarService *service.CRMCalendarService) *CRMCalendarHandler {
	return &CRMCalendarHandler{calendarService: calendarService}
}

// List handles GET /api/crm/calendar/events.
func (h *CRMCalendarHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMCalendarEventListFilters{
		EmailAccountID: queryStringPtr(r, "email_account_id"),
		DealID:         queryStringPtr(r, "deal_id"),
		Status:         queryStringPtr(r, "status"),
	}

	// Parse date filters.
	if sa := r.URL.Query().Get("start_after"); sa != "" {
		if t, err := time.Parse(time.RFC3339, sa); err == nil {
			filters.StartAfter = &t
		}
	}
	if sb := r.URL.Query().Get("start_before"); sb != "" {
		if t, err := time.Parse(time.RFC3339, sb); err == nil {
			filters.StartBefore = &t
		}
	}

	pagination := queryPagination(r)

	events, total, err := h.calendarService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []model.CRMCalendarEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  events,
		"total": total,
		"page":  pagination.Page,
	})
}

// Get handles GET /api/crm/calendar/events/{id}.
func (h *CRMCalendarHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	event, err := h.calendarService.GetByID(r.Context(), getWorkspaceID(r), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, event)
}

// Create handles POST /api/crm/calendar/events.
func (h *CRMCalendarHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMCalendarEventRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	event, err := h.calendarService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

// Update handles PUT /api/crm/calendar/events/{id}.
func (h *CRMCalendarHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMCalendarEventRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	event, err := h.calendarService.Update(r.Context(), getWorkspaceID(r), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, event)
}

// Delete handles DELETE /api/crm/calendar/events/{id}.
func (h *CRMCalendarHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.calendarService.Delete(r.Context(), getWorkspaceID(r), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "calendar event deleted"})
}

// ListByContact handles GET /api/crm/contacts/{id}/calendar.
func (h *CRMCalendarHandler) ListByContact(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	contactID := chi.URLParam(r, "id")
	_ = contactID // contact_ids filtering is done via JSONB; for now list all
	pagination := queryPagination(r)

	filters := model.CRMCalendarEventListFilters{
		ContactID: &contactID,
	}
	events, total, err := h.calendarService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []model.CRMCalendarEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  events,
		"total": total,
		"page":  pagination.Page,
	})
}

// ListByDeal handles GET /api/crm/deals/{id}/calendar.
func (h *CRMCalendarHandler) ListByDeal(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	dealID := chi.URLParam(r, "id")
	pagination := queryPagination(r)

	filters := model.CRMCalendarEventListFilters{
		DealID: &dealID,
	}
	events, total, err := h.calendarService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []model.CRMCalendarEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  events,
		"total": total,
		"page":  pagination.Page,
	})
}
