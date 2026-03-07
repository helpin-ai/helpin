package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// OrchestrationHandler handles epic orchestration endpoints.
type OrchestrationHandler struct {
	orchService *service.OrchestrationService
}

// NewOrchestrationHandler creates a new OrchestrationHandler.
func NewOrchestrationHandler(orchService *service.OrchestrationService) *OrchestrationHandler {
	return &OrchestrationHandler{orchService: orchService}
}

// AssignOrchestrator handles POST /api/pm/epics/{id}/assign-orchestrator.
func (h *OrchestrationHandler) AssignOrchestrator(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	epicID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.AssignAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.orchService.AssignOrchestrator(r.Context(), workspaceID, epicID, req.AgentID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"assigned": true})
}
