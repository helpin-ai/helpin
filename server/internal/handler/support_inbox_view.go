package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type SupportInboxViewHandler struct {
	viewService *service.SupportInboxViewService
}

func NewSupportInboxViewHandler(viewService *service.SupportInboxViewService) *SupportInboxViewHandler {
	return &SupportInboxViewHandler{viewService: viewService}
}

func isSupportInboxViewForbidden(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "forbidden:")
}

func (h *SupportInboxViewHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	views, err := h.viewService.List(r.Context(), workspaceID, middleware.GetUserID(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if views == nil {
		views = []model.SupportInboxView{}
	}
	writeJSON(w, http.StatusOK, views)
}

func (h *SupportInboxViewHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	var req model.CreateSupportInboxViewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	actor := authorization.GetActor(r.Context())
	role := ""
	if actor != nil {
		role = actor.Role
	}
	view, err := h.viewService.Create(r.Context(), workspaceID, middleware.GetUserID(r.Context()), role, req)
	if err != nil {
		if isSupportInboxViewForbidden(err) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (h *SupportInboxViewHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateSupportInboxViewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	actor := authorization.GetActor(r.Context())
	role := ""
	if actor != nil {
		role = actor.Role
	}
	view, err := h.viewService.Update(r.Context(), chi.URLParam(r, "viewId"), middleware.GetUserID(r.Context()), role, req)
	if err != nil {
		if isSupportInboxViewForbidden(err) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *SupportInboxViewHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor := authorization.GetActor(r.Context())
	role := ""
	if actor != nil {
		role = actor.Role
	}
	err := h.viewService.Delete(r.Context(), chi.URLParam(r, "viewId"), middleware.GetUserID(r.Context()), role)
	if err != nil {
		if isSupportInboxViewForbidden(err) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "view deleted"})
}
