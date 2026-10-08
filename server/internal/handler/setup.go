package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type SetupHandler struct {
	service *service.SetupService
	authz   *authorization.AuthzService
}

func NewSetupHandler(setupService *service.SetupService, authz *authorization.AuthzService) *SetupHandler {
	return &SetupHandler{service: setupService, authz: authz}
}

func (h *SetupHandler) Get(w http.ResponseWriter, r *http.Request) {
	access, err := h.setupAccess(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	view, err := h.service.Get(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), access)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *SetupHandler) UpdateGoals(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateSetupGoalsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	access, err := h.setupAccess(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	view, err := h.service.UpdateGoals(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), req.Goals, access)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *SetupHandler) setupAccess(r *http.Request) (service.SetupAccess, error) {
	actor := authorization.GetActor(r.Context())
	access := service.SetupAccess{Permissions: map[string]bool{}, Modules: map[string]bool{}}
	for _, permission := range h.authz.PermissionsForActor(actor) {
		access.Permissions[string(permission)] = true
	}
	modules, err := h.authz.AccessibleModules(r.Context(), actor)
	if err != nil {
		return service.SetupAccess{}, err
	}
	for _, module := range modules {
		access.Modules[string(module)] = true
	}
	return access, nil
}

func (h *SetupHandler) UpdatePreference(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateSetupPreferenceRequest
	if err := decodeJSON(r, &req); err != nil || req.SidebarDismissed == nil {
		writeError(w, http.StatusBadRequest, "sidebar_dismissed is required")
		return
	}
	preference, err := h.service.UpdatePreference(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), *req.SidebarDismissed)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preference)
}

func (h *SetupHandler) StartRecommendation(w http.ResponseWriter, r *http.Request) {
	if err := h.service.StartRecommendation(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), chi.URLParam(r, "taskKey")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SupportSetup exposes the same read-only evidence as the public MCP operation.
func (h *SetupHandler) SupportSetup(w http.ResponseWriter, r *http.Request) {
	access, err := h.setupAccess(r)
	if err != nil {
		slog.ErrorContext(r.Context(), "inspect support setup failed", "workspace_id", chi.URLParam(r, "id"), "error", err)
		writeError(w, http.StatusInternalServerError, "Unable to inspect support setup")
		return
	}
	guide, err := h.service.SupportSetup(r.Context(), chi.URLParam(r, "id"), access)
	if err != nil {
		slog.ErrorContext(r.Context(), "inspect support setup failed", "workspace_id", chi.URLParam(r, "id"), "error", err)
		writeError(w, http.StatusInternalServerError, "Unable to inspect support setup")
		return
	}
	writeJSON(w, http.StatusOK, guide)
}
