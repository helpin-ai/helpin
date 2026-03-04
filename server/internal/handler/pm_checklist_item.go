package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// PMChecklistItemHandler handles PM checklist item HTTP endpoints.
type PMChecklistItemHandler struct {
	service *service.PMChecklistItemService
}

// NewPMChecklistItemHandler creates a new PMChecklistItemHandler.
func NewPMChecklistItemHandler(service *service.PMChecklistItemService) *PMChecklistItemHandler {
	return &PMChecklistItemHandler{service: service}
}

// List handles GET /api/pm/stories/{id}/checklist
func (h *PMChecklistItemHandler) List(w http.ResponseWriter, r *http.Request) {
	storyID := chi.URLParam(r, "id")
	items, err := h.service.List(r.Context(), storyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []model.PMChecklistItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

// Create handles POST /api/pm/stories/{id}/checklist
func (h *PMChecklistItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	storyID := chi.URLParam(r, "id")
	workspaceID := middleware.GetWorkspaceID(r.Context())
	actorID := middleware.GetUserID(r.Context())
	var req model.CreateChecklistItemRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.service.Create(r.Context(), storyID, req, workspaceID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Update handles PUT /api/pm/checklist-items/{id}
func (h *PMChecklistItemHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := middleware.GetWorkspaceID(r.Context())
	actorID := middleware.GetUserID(r.Context())
	var req model.UpdateChecklistItemRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.service.Update(r.Context(), id, req, workspaceID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// Delete handles DELETE /api/pm/checklist-items/{id}
func (h *PMChecklistItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := middleware.GetWorkspaceID(r.Context())
	actorID := middleware.GetUserID(r.Context())
	if err := h.service.Delete(r.Context(), id, workspaceID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "checklist item deleted"})
}
