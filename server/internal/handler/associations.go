package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// AssociationsHandler exposes grouped associations and task relationship endpoints.
type AssociationsHandler struct {
	associationsService *service.AssociationsService
	authz               *authorization.AuthzService
}

// NewAssociationsHandler creates a new AssociationsHandler.
func NewAssociationsHandler(associationsService *service.AssociationsService, authz *authorization.AuthzService) *AssociationsHandler {
	return &AssociationsHandler{associationsService: associationsService, authz: authz}
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
	actor := authorization.GetActor(r.Context())
	if actor == nil {
		writeError(w, http.StatusInternalServerError, "authorization context missing")
		return
	}
	canAccess := func(permission authorization.Permission, module model.ModuleID) (bool, error) {
		if !h.authz.Can(actor, permission) {
			return false, nil
		}
		return h.authz.CanAccessModule(r.Context(), actor, module)
	}
	canPM, err := canAccess(authorization.PermPMRead, model.ModulePM)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify project access")
		return
	}
	canCRM, err := canAccess(authorization.PermCRMRead, model.ModuleCRM)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify CRM access")
		return
	}
	canSupport, err := canAccess(authorization.PermSupportRead, model.ModuleSupport)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify support access")
		return
	}
	canDocs, err := canAccess(authorization.PermDocsRead, model.ModuleDocs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify docs access")
		return
	}
	filterGroupedAssociations(response, canPM, canCRM, canSupport, canDocs)
	writeJSON(w, http.StatusOK, response)
}

func filterGroupedAssociations(response *model.GroupedAssociationsResponse, canPM, canCRM, canSupport, canDocs bool) {
	if response == nil {
		return
	}
	if !canPM {
		response.TaskRelationships = model.TaskRelationshipGroups{
			BlockedBy: []model.TaskRelationshipSummary{}, Blocking: []model.TaskRelationshipSummary{},
			RelatesTo: []model.TaskRelationshipSummary{}, RelatedBy: []model.TaskRelationshipSummary{},
			Duplicates: []model.TaskRelationshipSummary{}, DuplicatedBy: []model.TaskRelationshipSummary{},
		}
		response.Tasks = []model.AssociationObjectSummary{}
	}
	if !canCRM {
		response.CRMRecords = []model.AssociationObjectSummary{}
	}
	if !canSupport {
		response.SupportConversations = []model.AssociationObjectSummary{}
	}
	if !canDocs {
		response.Docs = []model.AssociationObjectSummary{}
	}
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
