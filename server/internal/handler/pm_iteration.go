package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// PMIterationHandler handles PM iteration HTTP endpoints.
type PMIterationHandler struct {
	iterationService *service.PMIterationService
}

// NewPMIterationHandler creates a new PMIterationHandler.
func NewPMIterationHandler(iterationService *service.PMIterationService) *PMIterationHandler {
	return &PMIterationHandler{iterationService: iterationService}
}

// List handles GET /api/pm/iterations.
func (h *PMIterationHandler) List(w http.ResponseWriter, r *http.Request) {
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
	filters := model.PMIterationListFilters{
		TeamID:   queryStringPtr(r, "team_id"),
		Status:   queryStringPtr(r, "status"),
		Archived: archived,
	}
	iterations, err := h.iterationService.List(r.Context(), workspaceID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if iterations == nil {
		iterations = []model.IterationWithStats{}
	}
	writeJSON(w, http.StatusOK, iterations)
}

// Create handles POST /api/pm/iterations.
func (h *PMIterationHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.CreateIterationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	iteration, err := h.iterationService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, iteration)
}

// Get handles GET /api/pm/iterations/{id}.
func (h *PMIterationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	iteration, err := h.iterationService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, iteration)
}

// Update handles PUT /api/pm/iterations/{id}.
func (h *PMIterationHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.UpdateIterationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	iteration, err := h.iterationService.Update(r.Context(), id, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, iteration)
}

// Delete handles DELETE /api/pm/iterations/{id}.
func (h *PMIterationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.iterationService.Delete(r.Context(), id, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "iteration deleted"})
}

// ListStories handles GET /api/pm/iterations/{id}/stories.
func (h *PMIterationHandler) ListStories(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stories, err := h.iterationService.ListStories(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stories == nil {
		stories = []model.PMStory{}
	}
	writeJSON(w, http.StatusOK, stories)
}
