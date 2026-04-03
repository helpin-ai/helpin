package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// WorkspaceHandler handles workspace HTTP requests.
type WorkspaceHandler struct {
	workspaceService *service.WorkspaceService
}

// NewWorkspaceHandler creates a new WorkspaceHandler.
func NewWorkspaceHandler(workspaceService *service.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{workspaceService: workspaceService}
}

// List handles GET /api/workspaces.
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	organizationID := r.URL.Query().Get("organization_id")
	workspaces, err := h.workspaceService.List(r.Context(), userID, organizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if workspaces == nil {
		workspaces = []model.WorkspaceWithRole{}
	}
	writeJSON(w, http.StatusOK, workspaces)
}

// Create handles POST /api/workspaces.
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.CreateWorkspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ws, err := h.workspaceService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, ws)
}

// GetBySlug handles GET /api/workspaces/by-slug/{slug}.
func (h *WorkspaceHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ws, err := h.workspaceService.GetBySlug(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

// Update handles PUT /api/workspaces/{id}.
func (h *WorkspaceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req model.UpdateWorkspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	actorID := middleware.GetUserID(r.Context())
	ws, err := h.workspaceService.Update(r.Context(), id, req, actorID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

// UpdateMember handles PUT /api/workspaces/{id}/members/{memberId}.
func (h *WorkspaceHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	memberID := chi.URLParam(r, "memberId")
	userID := middleware.GetUserID(r.Context())

	var req model.UpdateWorkspaceMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.workspaceService.UpdateMember(r.Context(), workspaceID, userID, memberID, req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "member updated"})
}

// UploadLogo handles POST /api/workspaces/{id}/logo.
func (h *WorkspaceHandler) UploadLogo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 2MB)")
		return
	}

	file, header, err := r.FormFile("logo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing logo file")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/webp" && contentType != "image/svg+xml" {
		writeError(w, http.StatusBadRequest, "only PNG, JPEG, WebP, and SVG images are allowed")
		return
	}

	ws, err := h.workspaceService.UploadLogo(r.Context(), id, file, header.Size, contentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

// DeleteLogo handles DELETE /api/workspaces/{id}/logo.
func (h *WorkspaceHandler) DeleteLogo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ws, err := h.workspaceService.DeleteLogo(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

// Delete handles DELETE /api/workspaces/{id}.
func (h *WorkspaceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.workspaceService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "workspace deleted"})
}

// GetKeyHistory handles GET /api/workspaces/{id}/key-history.
func (h *WorkspaceHandler) GetKeyHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	history, err := h.workspaceService.GetKeyHistory(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if history == nil {
		history = []model.WorkspaceKeyHistory{}
	}
	writeJSON(w, http.StatusOK, history)
}

// GetMyRole handles GET /api/workspaces/{id}/my-role.
func (h *WorkspaceHandler) GetMyRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	role, err := h.workspaceService.GetMyRole(r.Context(), id, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"role": role})
}

// GetMyMembership handles GET /api/workspaces/{id}/my-membership.
func (h *WorkspaceHandler) GetMyMembership(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	membership, err := h.workspaceService.GetMyMembership(r.Context(), id, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, membership)
}

// ListMembers handles GET /api/workspaces/{id}/members.
func (h *WorkspaceHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	members, err := h.workspaceService.ListMembers(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if members == nil {
		members = []model.MemberWithUser{}
	}
	writeJSON(w, http.StatusOK, members)
}

// ListAssignableMembers handles GET /api/workspaces/{id}/assignable-members.
func (h *WorkspaceHandler) ListAssignableMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	members, err := h.workspaceService.ListAssignableMembers(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if members == nil {
		members = []model.AssignableMember{}
	}
	writeJSON(w, http.StatusOK, members)
}

// GetMe handles GET /api/workspaces/{id}/me.
// Returns the actor's membership, effective permissions, and team memberships.
func (h *WorkspaceHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	actor := authorization.GetActor(r.Context())
	if actor == nil {
		writeError(w, http.StatusInternalServerError, "authorization context missing")
		return
	}

	// Build permission strings from actor's role
	perms := authorization.NewRBACEngine().PermissionsForRole(actor.Role)
	permStrings := make([]string, len(perms))
	for i, p := range perms {
		permStrings[i] = string(p)
	}

	teamMemberships := make([]map[string]string, len(actor.TeamMemberships))
	for i, tm := range actor.TeamMemberships {
		teamMemberships[i] = map[string]string{
			"team_id": tm.TeamID,
			"role":    tm.Role,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"workspace_id": actor.WorkspaceID,
		"membership": map[string]string{
			"id":     actor.WorkspaceMemberID,
			"role":   actor.Role,
			"status": actor.Status,
		},
		"permissions":      permStrings,
		"team_memberships": teamMemberships,
	})
}
