package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMDealAutomationHandler handles CRM deal automation endpoints.
type CRMDealAutomationHandler struct {
	automationService *service.DealAutomationService
}

// NewCRMDealAutomationHandler creates a new CRMDealAutomationHandler.
func NewCRMDealAutomationHandler(automationService *service.DealAutomationService) *CRMDealAutomationHandler {
	return &CRMDealAutomationHandler{automationService: automationService}
}

// GetAutonomySettings handles GET /crm/autonomy-settings.
func (h *CRMDealAutomationHandler) GetAutonomySettings(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	settings := h.automationService.GetAutonomySettings(r.Context(), workspaceID)
	writeJSON(w, http.StatusOK, settings)
}

// UpdateAutonomySettings handles PUT /crm/autonomy-settings.
func (h *CRMDealAutomationHandler) UpdateAutonomySettings(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	var settings model.CRMAutonomySettings
	if err := decodeJSON(r, &settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.automationService.UpdateAutonomySettings(r.Context(), workspaceID, settings); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}
