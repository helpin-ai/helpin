package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PMSprintHandler handles PM sprint HTTP endpoints.
type PMSprintHandler struct {
	sprintService *service.PMSprintService
}

// NewPMSprintHandler creates a new PMSprintHandler.
func NewPMSprintHandler(sprintService *service.PMSprintService) *PMSprintHandler {
	return &PMSprintHandler{sprintService: sprintService}
}

// List handles GET /api/pm/sprints.
func (h *PMSprintHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	archived, err := queryBoolPtr(r, "archived")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid archived query param")
		return
	}
	filters := model.PMSprintListFilters{
		TeamID:   queryStringPtr(r, "team_id"),
		Status:   queryStringPtr(r, "status"),
		Archived: archived,
	}
	sprints, err := h.sprintService.List(r.Context(), workspaceID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sprints == nil {
		sprints = []model.SprintWithStats{}
	}
	writeJSON(w, http.StatusOK, sprints)
}

// PlanningWorkspace handles GET /api/pm/sprints/planning.
func (h *PMSprintHandler) PlanningWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	includeCompleted, err := queryBoolPtr(r, "include_completed")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid include_completed query param")
		return
	}
	filters := model.PMSprintPlanningFilters{
		TeamID: queryStringPtr(r, "team_id"),
	}
	if includeCompleted != nil {
		filters.IncludeCompleted = *includeCompleted
	} else {
		filters.IncludeCompleted = true
	}
	workspace, err := h.sprintService.ListPlanningWorkspace(r.Context(), workspaceID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workspace)
}

// Create handles POST /api/pm/sprints.
func (h *PMSprintHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.CreateSprintRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	sprint, err := h.sprintService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sprint)
}

// Get handles GET /api/pm/sprints/{id}.
func (h *PMSprintHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sprint, err := h.sprintService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sprint)
}

// Update handles PUT /api/pm/sprints/{id}.
func (h *PMSprintHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.UpdateSprintRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sprint, err := h.sprintService.Update(r.Context(), id, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sprint)
}

// Delete handles DELETE /api/pm/sprints/{id}.
func (h *PMSprintHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.sprintService.Delete(r.Context(), id, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "sprint deleted"})
}

// ListStories handles GET /api/pm/sprints/{id}/stories.
func (h *PMSprintHandler) ListStories(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stories, err := h.sprintService.ListStories(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stories == nil {
		stories = []model.PMTask{}
	}
	writeJSON(w, http.StatusOK, stories)
}
