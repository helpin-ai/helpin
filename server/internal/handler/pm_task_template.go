package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PMTaskTemplateHandler handles PM task template HTTP endpoints.
type PMTaskTemplateHandler struct {
	templateService *service.PMTaskTemplateService
}

// NewPMTaskTemplateHandler creates a new PMTaskTemplateHandler.
func NewPMTaskTemplateHandler(templateService *service.PMTaskTemplateService) *PMTaskTemplateHandler {
	return &PMTaskTemplateHandler{templateService: templateService}
}

// List handles GET /api/pm/story-templates.
func (h *PMTaskTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	includeShared, err := queryBoolDefault(r, "include_shared", true)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid include_shared query param")
		return
	}
	archived, err := queryBoolPtr(r, "archived")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid archived query param")
		return
	}
	templates, err := h.templateService.ListByWorkspace(r.Context(), workspaceID, queryStringPtr(r, "team_id"), includeShared, archived)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if templates == nil {
		templates = []model.PMTaskTemplate{}
	}
	writeJSON(w, http.StatusOK, templates)
}

// Get handles GET /api/pm/story-templates/{id}.
func (h *PMTaskTemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tmpl, err := h.templateService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tmpl)
}

// Create handles POST /api/pm/story-templates.
func (h *PMTaskTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateTaskTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	tmpl, err := h.templateService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tmpl)
}

// Update handles PUT /api/pm/story-templates/{id}.
func (h *PMTaskTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateTaskTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tmpl, err := h.templateService.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tmpl)
}

// Delete handles DELETE /api/pm/story-templates/{id}.
func (h *PMTaskTemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.templateService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "task template deleted"})
}
