package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// WorkspaceHandler handles workspace HTTP requests.
type WorkspaceHandler struct {
	workspaceService *service.WorkspaceService
	authz            *authorization.AuthzService
}

// NewWorkspaceHandler creates a new WorkspaceHandler.
func NewWorkspaceHandler(workspaceService *service.WorkspaceService, authz *authorization.AuthzService) *WorkspaceHandler {
	return &WorkspaceHandler{workspaceService: workspaceService, authz: authz}
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

// GenerateCompanyProductDescription drafts workspace context from a website.
func (h *WorkspaceHandler) GenerateCompanyProductDescription(w http.ResponseWriter, r *http.Request) {
	var req model.GenerateWorkspaceContextDescriptionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := middleware.GetUserID(r.Context())
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	if req.WorkspaceID != "" && !h.canEditWorkspaceContext(r, req.WorkspaceID, userID) {
		writeError(w, http.StatusForbidden, "You don't have permission to update this workspace.")
		return
	}

	resp, err := h.workspaceService.GenerateCompanyProductDescription(r.Context(), userID, req)
	if err != nil {
		writeWorkspaceContextError(w, r, err, req.WorkspaceID, userID)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// workspaceContextErrorMessages are the only texts returned for generation
// failures; internal detail is logged, never sent.
var workspaceContextErrorMessages = map[string]string{
	service.WorkspaceContextErrAIUnavailable:     "AI isn't connected yet. Connect an AI provider to generate this, or write it yourself.",
	service.WorkspaceContextErrWebsiteUnreadable: "We couldn't read your website. It may need JavaScript or block automated visitors. Write a short description instead.",
	service.WorkspaceContextErrTimeout:           "This is taking too long. Write a short description instead.",
	service.WorkspaceContextErrGenerationFailed:  "We couldn't generate a description. Write a short description instead.",
}

// canEditWorkspaceContext matches PUT /workspaces/{id}: the caller must be an
// active member with workspace.update.
func (h *WorkspaceHandler) canEditWorkspaceContext(r *http.Request, workspaceID, userID string) bool {
	if h.authz == nil {
		return false
	}
	actor, err := h.authz.ResolveActor(r.Context(), workspaceID, userID)
	if err != nil {
		return false
	}
	return h.authz.Can(actor, authorization.PermWorkspaceUpdate)
}

func writeWorkspaceContextError(w http.ResponseWriter, r *http.Request, err error, workspaceID, userID string) {
	var validation *service.WorkspaceContextValidationError
	if errors.As(err, &validation) {
		writeError(w, http.StatusBadRequest, validation.Message)
		return
	}
	code := service.WorkspaceContextErrGenerationFailed
	var contextErr *service.WorkspaceContextError
	if errors.As(err, &contextErr) {
		if _, known := workspaceContextErrorMessages[contextErr.Code]; known {
			code = contextErr.Code
		}
	}
	slog.WarnContext(r.Context(), "company/product context generation failed",
		"workspace_id", workspaceID, "user_id", userID, "code", code, "error", err)
	writeErrorCode(w, http.StatusUnprocessableEntity, workspaceContextErrorMessages[code], code)
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

// UpdateSupportTaskPreferences handles PATCH /api/workspaces/{id}/me/support-task-preferences.
func (h *WorkspaceHandler) UpdateSupportTaskPreferences(w http.ResponseWriter, r *http.Request) {
	actor := authorization.GetActor(r.Context())
	if actor == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.UpdateSupportTaskPreferencesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.workspaceService.UpdateSupportTaskPreferences(r.Context(), actor.WorkspaceID, actor.WorkspaceMemberID, req); err != nil {
		slog.ErrorContext(r.Context(), "update support task preferences", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update preferences")
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "preferences updated"})
}

// RemoveMember handles DELETE /api/workspaces/{id}/members/{memberId}.
func (h *WorkspaceHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	memberID := chi.URLParam(r, "memberId")
	userID := middleware.GetUserID(r.Context())

	if err := h.workspaceService.RemoveMember(r.Context(), workspaceID, userID, memberID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "member removed"})
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

// ListMemberPresence handles GET /api/workspaces/{id}/members/presence.
func (h *WorkspaceHandler) ListMemberPresence(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	statuses, err := h.workspaceService.ListMemberPresence(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if statuses == nil {
		statuses = []model.WorkspaceMemberPresenceStatus{}
	}
	writeJSON(w, http.StatusOK, statuses)
}

// ListAssignableMembers handles GET /api/workspaces/{id}/assignable-members.
func (h *WorkspaceHandler) ListAssignableMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	members, err := h.workspaceService.ListAssignableMembers(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if module := model.ModuleID(r.URL.Query().Get("module")); module != "" {
		if !model.IsValidWorkspaceModule(module) {
			writeError(w, http.StatusBadRequest, "unknown module")
			return
		}
		members, err = h.authz.FilterMembersByModuleAccess(r.Context(), id, members, module)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve member module access")
			return
		}
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

	modules, err := h.authz.AccessibleModules(r.Context(), actor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve module access")
		return
	}
	moduleStrings := make([]string, len(modules))
	for i, module := range modules {
		moduleStrings[i] = string(module)
	}
	claims := middleware.ClaimsFrom(r.Context())
	mfaSatisfied := claims != nil && claims.MFASatisfied
	securityPolicy, err := h.workspaceService.GetWorkspaceMFAPolicy(r.Context(), actor.WorkspaceID, actor.UserID, mfaSatisfied)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve workspace security policy")
		return
	}

	// Fetch full member record for preference fields.
	member, memberErr := h.workspaceService.GetMyMembership(r.Context(), actor.WorkspaceID, actor.UserID)

	membership := map[string]interface{}{
		"id":      actor.WorkspaceMemberID,
		"user_id": actor.UserID,
		"role":    actor.Role,
		"status":  actor.Status,
	}
	if memberErr == nil && member != nil {
		membership["support_default_team_id"] = member.SupportDefaultTeamID
		membership["support_task_dialog_dismissed"] = member.SupportTaskDialogDismissed
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"workspace_id":     actor.WorkspaceID,
		"membership":       membership,
		"permissions":      permStrings,
		"team_memberships": teamMemberships,
		"modules":          moduleStrings,
		"security_policy":  securityPolicy,
	})
}
