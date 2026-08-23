package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMSummaryHandler handles CRM summary endpoints.
type CRMSummaryHandler struct {
	summaryService *service.CRMSummaryService
}

// NewCRMSummaryHandler creates a new CRMSummaryHandler.
func NewCRMSummaryHandler(summaryService *service.CRMSummaryService) *CRMSummaryHandler {
	return &CRMSummaryHandler{summaryService: summaryService}
}

// GetContactSummary handles GET /api/crm/contacts/{id}/summary.
func (h *CRMSummaryHandler) GetContactSummary(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	contactID := chi.URLParam(r, "id")

	summary, err := h.summaryService.GetContactSummary(r.Context(), workspaceID, contactID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// GetDealSummary handles GET /api/crm/deals/{id}/summary.
func (h *CRMSummaryHandler) GetDealSummary(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	dealID := chi.URLParam(r, "id")

	summary, err := h.summaryService.GetDealSummary(r.Context(), workspaceID, dealID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// RefreshContactSummary handles POST /api/crm/contacts/{id}/summary/refresh.
func (h *CRMSummaryHandler) RefreshContactSummary(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	contactID := chi.URLParam(r, "id")

	summary, err := h.summaryService.RefreshContactSummaryNow(r.Context(), workspaceID, contactID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// RefreshDealSummary handles POST /api/crm/deals/{id}/summary/refresh.
func (h *CRMSummaryHandler) RefreshDealSummary(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	dealID := chi.URLParam(r, "id")

	summary, err := h.summaryService.RefreshDealSummaryNow(r.Context(), workspaceID, dealID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
