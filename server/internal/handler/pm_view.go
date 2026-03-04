package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// PMViewHandler handles PM view HTTP endpoints.
type PMViewHandler struct {
	viewService *service.PMViewService
}

// NewPMViewHandler creates a new PMViewHandler.
func NewPMViewHandler(viewService *service.PMViewService) *PMViewHandler {
	return &PMViewHandler{viewService: viewService}
}

// List handles GET /api/pm/views.
func (h *PMViewHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID := middleware.GetUserID(r.Context())
	views, err := h.viewService.List(r.Context(), workspaceID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if views == nil {
		views = []model.PMView{}
	}
	writeJSON(w, http.StatusOK, views)
}

// Create handles POST /api/pm/views.
func (h *PMViewHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID := middleware.GetUserID(r.Context())

	var req model.CreateViewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	view, err := h.viewService.Create(r.Context(), workspaceID, userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

// Update handles PUT /api/pm/views/{id}.
func (h *PMViewHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	var req model.UpdateViewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	view, err := h.viewService.Update(r.Context(), id, userID, req)
	if err != nil {
		if err.Error() == "forbidden: you can only update your own views" {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// Delete handles DELETE /api/pm/views/{id}.
func (h *PMViewHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	if err := h.viewService.Delete(r.Context(), id, userID); err != nil {
		if err.Error() == "forbidden: you can only delete your own views" {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "view deleted"})
}
