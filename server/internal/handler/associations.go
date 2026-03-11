package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// AssociationsHandler exposes grouped associations and story relationship endpoints.
type AssociationsHandler struct {
	associationsService *service.AssociationsService
}

// NewAssociationsHandler creates a new AssociationsHandler.
func NewAssociationsHandler(associationsService *service.AssociationsService) *AssociationsHandler {
	return &AssociationsHandler{associationsService: associationsService}
}

// ListEpicAssociations handles GET /api/pm/epics/{id}/associations.
func (h *AssociationsHandler) ListEpicAssociations(w http.ResponseWriter, r *http.Request) {
	h.listByObject(w, r, "epic")
}

// ListStoryAssociations handles GET /api/pm/stories/{id}/associations.
func (h *AssociationsHandler) ListStoryAssociations(w http.ResponseWriter, r *http.Request) {
	h.listByObject(w, r, "story")
}

// ListConversationAssociations handles GET /api/support/inbox/conversations/{id}/associations.
func (h *AssociationsHandler) ListConversationAssociations(w http.ResponseWriter, r *http.Request) {
	h.listByObject(w, r, "support_conversation")
}

func (h *AssociationsHandler) listByObject(w http.ResponseWriter, r *http.Request, objectType string) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	objectID := chi.URLParam(r, "id")

	response, err := h.associationsService.ListGrouped(r.Context(), workspaceID, objectType, objectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// CreateStoryRelationship handles POST /api/pm/stories/{id}/relationships.
func (h *AssociationsHandler) CreateStoryRelationship(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	storyID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateStoryRelationshipRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	link, err := h.associationsService.CreateStoryRelationship(r.Context(), workspaceID, storyID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

// DeleteStoryRelationship handles DELETE /api/pm/story-relationships/{id}.
func (h *AssociationsHandler) DeleteStoryRelationship(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	relationshipID := chi.URLParam(r, "id")
	if err := h.associationsService.DeleteStoryRelationship(r.Context(), workspaceID, relationshipID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "relationship deleted"})
}
