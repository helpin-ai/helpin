package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// PMWorkflowHandler handles PM workflow HTTP endpoints.
type PMWorkflowHandler struct {
	workflowService *service.PMWorkflowService
}

// NewPMWorkflowHandler creates a new PMWorkflowHandler.
func NewPMWorkflowHandler(workflowService *service.PMWorkflowService) *PMWorkflowHandler {
	return &PMWorkflowHandler{workflowService: workflowService}
}

// List handles GET /api/pm/workflows.
func (h *PMWorkflowHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	workflows, err := h.workflowService.ListByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if workflows == nil {
		workflows = []model.WorkflowWithStates{}
	}
	writeJSON(w, http.StatusOK, workflows)
}

// ListEpicStates handles GET /api/pm/workflows/epic-states.
func (h *PMWorkflowHandler) ListEpicStates(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	states, err := h.workflowService.ListEpicStates(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if states == nil {
		states = []model.PMEpicWorkflowState{}
	}
	writeJSON(w, http.StatusOK, states)
}

// Create handles POST /api/pm/workflows.
func (h *PMWorkflowHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateWorkflowRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	workflow, err := h.workflowService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, workflow)
}

// Get handles GET /api/pm/workflows/{id}.
func (h *PMWorkflowHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workflow, err := h.workflowService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflow)
}

// Update handles PUT /api/pm/workflows/{id}.
func (h *PMWorkflowHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateWorkflowRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	workflow, err := h.workflowService.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflow)
}

// Delete handles DELETE /api/pm/workflows/{id}.
func (h *PMWorkflowHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.workflowService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "workflow deleted"})
}

// CreateState handles POST /api/pm/workflows/{id}/states.
func (h *PMWorkflowHandler) CreateState(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "id")
	var req model.CreateWorkflowStateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	state, err := h.workflowService.CreateState(r.Context(), workflowID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, state)
}

// UpdateState handles PUT /api/pm/workflows/{id}/states/{stateId}.
func (h *PMWorkflowHandler) UpdateState(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "id")
	stateID := chi.URLParam(r, "stateId")

	var req model.UpdateWorkflowStateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	state, err := h.workflowService.UpdateState(r.Context(), workflowID, stateID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// DeleteState handles DELETE /api/pm/workflows/{id}/states/{stateId}.
func (h *PMWorkflowHandler) DeleteState(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "id")
	stateID := chi.URLParam(r, "stateId")
	if err := h.workflowService.DeleteState(r.Context(), workflowID, stateID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "state deleted"})
}

// ReorderStates handles PUT /api/pm/workflows/{id}/states/reorder.
func (h *PMWorkflowHandler) ReorderStates(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "id")
	var req model.ReorderStatesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.workflowService.ReorderStates(r.Context(), workflowID, req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "states reordered"})
}
