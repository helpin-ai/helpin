package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// AssociationsHandler exposes grouped associations and task relationship endpoints.
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

// ListTaskAssociations handles GET /api/pm/tasks/{id}/associations.
func (h *AssociationsHandler) ListTaskAssociations(w http.ResponseWriter, r *http.Request) {
	h.listByObject(w, r, "task")
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

// CreateTaskRelationship handles POST /api/pm/tasks/{id}/relationships.
func (h *AssociationsHandler) CreateTaskRelationship(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	taskID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateTaskRelationshipRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	link, err := h.associationsService.CreateTaskRelationship(r.Context(), workspaceID, taskID, actorID, model.CreateTaskRelationshipRequest{
		RelationshipType: req.RelationshipType,
		OtherTaskID:      req.OtherTaskID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

// DeleteTaskRelationship handles DELETE /api/pm/task-relationships/{id}.
func (h *AssociationsHandler) DeleteTaskRelationship(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	relationshipID := chi.URLParam(r, "id")
	if err := h.associationsService.DeleteTaskRelationship(r.Context(), workspaceID, relationshipID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "relationship deleted"})
}
