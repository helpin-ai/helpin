package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// OrchestrationHandler handles epic orchestration endpoints.
type OrchestrationHandler struct {
	orchService *service.OrchestrationService
}

// NewOrchestrationHandler creates a new OrchestrationHandler.
func NewOrchestrationHandler(orchService *service.OrchestrationService) *OrchestrationHandler {
	return &OrchestrationHandler{orchService: orchService}
}

// Orchestrate handles POST /api/pm/epics/{id}/orchestrate.
func (h *OrchestrationHandler) Orchestrate(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	epicID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.OrchestrateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	proposal, err := h.orchService.Orchestrate(r.Context(), workspaceID, epicID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

// ConfirmOrchestration handles POST /api/pm/epics/{id}/orchestrate/confirm.
func (h *OrchestrationHandler) ConfirmOrchestration(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	epicID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.ConfirmOrchestrationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	stories, err := h.orchService.ConfirmOrchestration(r.Context(), workspaceID, epicID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, stories)
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
