package handler

import (
	"net/http"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// PMAutomationHandler handles automation HTTP endpoints.
type PMAutomationHandler struct {
	automationService *service.PMAutomationService
}

// NewPMAutomationHandler creates a new PMAutomationHandler.
func NewPMAutomationHandler(automationService *service.PMAutomationService) *PMAutomationHandler {
	return &PMAutomationHandler{automationService: automationService}
}

// List handles GET /api/pm/automations.
func (h *PMAutomationHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	automations, err := h.automationService.List(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if automations == nil {
		automations = []model.PMAutomation{}
	}
	writeJSON(w, http.StatusOK, automations)
}

// Upsert handles PUT /api/pm/automations.
func (h *PMAutomationHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req model.UpsertAutomationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	automation, err := h.automationService.Upsert(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, automation)
}

// Delete handles DELETE /api/pm/automations?automation_type=...&team_id=...
func (h *PMAutomationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	req := model.DeleteAutomationRequest{
		WorkspaceID:    getWorkspaceID(r),
		AutomationType: r.URL.Query().Get("automation_type"),
		TeamID:         queryStringPtr(r, "team_id"),
	}
	if err := h.automationService.Delete(r.Context(), req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "automation deleted"})
}
