package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PMEpicHandler handles PM epic HTTP endpoints.
type PMEpicHandler struct {
	epicService *service.PMEpicService
}

// NewPMEpicHandler creates a new PMEpicHandler.
func NewPMEpicHandler(epicService *service.PMEpicService) *PMEpicHandler {
	return &PMEpicHandler{epicService: epicService}
}

// List handles GET /api/pm/epics.
func (h *PMEpicHandler) List(w http.ResponseWriter, r *http.Request) {
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
	filters := model.PMEpicListFilters{
		TeamID:   queryStringPtr(r, "team_id"),
		StateID:  queryStringPtr(r, "state_id"),
		LabelID:  queryStringPtr(r, "label_id"),
		Archived: archived,
	}

	epics, err := h.epicService.List(r.Context(), workspaceID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if epics == nil {
		epics = []model.EpicWithStats{}
	}
	writeJSON(w, http.StatusOK, epics)
}

// Create handles POST /api/pm/epics.
func (h *PMEpicHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.CreateEpicRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	epic, err := h.epicService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, epic)
}

// Get handles GET /api/pm/epics/{id}.
func (h *PMEpicHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	epic, err := h.epicService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, epic)
}

// Update handles PUT /api/pm/epics/{id}.
func (h *PMEpicHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.UpdateEpicRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	epic, err := h.epicService.Update(r.Context(), id, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, epic)
}

// Delete handles DELETE /api/pm/epics/{id}.
func (h *PMEpicHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.epicService.Delete(r.Context(), id, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "epic archived"})
}

// ListStories handles GET /api/pm/epics/{id}/stories.
func (h *PMEpicHandler) ListStories(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stories, err := h.epicService.ListStories(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stories == nil {
		stories = []model.PMTask{}
	}
	writeJSON(w, http.StatusOK, stories)
}

// UpdateHealth handles PUT /api/pm/epics/{id}/health.
func (h *PMEpicHandler) UpdateHealth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.UpdateEpicHealthRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.epicService.UpdateHealth(r.Context(), id, req, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "epic health updated"})
}
