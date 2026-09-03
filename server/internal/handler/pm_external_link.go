package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PMExternalLinkHandler handles PM external link HTTP endpoints.
type PMExternalLinkHandler struct {
	service *service.PMExternalLinkService
}

// NewPMExternalLinkHandler creates a new PMExternalLinkHandler.
func NewPMExternalLinkHandler(service *service.PMExternalLinkService) *PMExternalLinkHandler {
	return &PMExternalLinkHandler{service: service}
}

// List handles GET /api/pm/tasks/{id}/links
func (h *PMExternalLinkHandler) List(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	links, err := h.service.List(r.Context(), taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if links == nil {
		links = []model.PMExternalLink{}
	}
	writeJSON(w, http.StatusOK, links)
}

// Create handles POST /api/pm/tasks/{id}/links
func (h *PMExternalLinkHandler) Create(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	workspaceID := middleware.GetWorkspaceID(r.Context())
	var req model.CreateExternalLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	link, err := h.service.Create(r.Context(), taskID, req, userID, workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

// Update handles PUT /api/pm/links/{id}
func (h *PMExternalLinkHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := middleware.GetWorkspaceID(r.Context())
	actorID := middleware.GetUserID(r.Context())
	var req model.UpdateExternalLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	link, err := h.service.Update(r.Context(), id, req, workspaceID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, link)
}

// ListByEntity handles GET /api/pm/entity-links/{entity_type}/{entity_id}
func (h *PMExternalLinkHandler) ListByEntity(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entity_type")
	entityID := chi.URLParam(r, "entity_id")
	links, err := h.service.ListByEntity(r.Context(), entityType, entityID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if links == nil {
		links = []model.PMExternalLink{}
	}
	writeJSON(w, http.StatusOK, links)
}

// CreateForEntity handles POST /api/pm/entity-links/{entity_type}/{entity_id}
func (h *PMExternalLinkHandler) CreateForEntity(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entity_type")
	entityID := chi.URLParam(r, "entity_id")
	userID := middleware.GetUserID(r.Context())
	workspaceID := middleware.GetWorkspaceID(r.Context())
	var req model.CreateExternalLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	link, err := h.service.CreateForEntity(r.Context(), entityType, entityID, req, userID, workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

// Delete handles DELETE /api/pm/links/{id}
func (h *PMExternalLinkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := middleware.GetWorkspaceID(r.Context())
	actorID := middleware.GetUserID(r.Context())
	if err := h.service.Delete(r.Context(), id, workspaceID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "external link deleted"})
}
