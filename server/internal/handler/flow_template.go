package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// FlowTemplateHandler handles HTTP requests for flow template CRUD.
type FlowTemplateHandler struct {
	flowService *service.FlowService
}

// NewFlowTemplateHandler creates a new FlowTemplateHandler.
func NewFlowTemplateHandler(flowService *service.FlowService) *FlowTemplateHandler {
	return &FlowTemplateHandler{flowService: flowService}
}

// CreateTemplate creates a new flow template with nodes.
func (h *FlowTemplateHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateFlowTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	view, err := h.flowService.CreateTemplate(r.Context(), workspaceID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

// GetTemplate returns a flow template by ID.
func (h *FlowTemplateHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	templateID := chi.URLParam(r, "templateId")

	view, err := h.flowService.GetTemplate(r.Context(), workspaceID, templateID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// UpdateTemplate updates a flow template.
func (h *FlowTemplateHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	templateID := chi.URLParam(r, "templateId")

	var req model.UpdateFlowTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	view, err := h.flowService.UpdateTemplate(r.Context(), workspaceID, templateID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// ListTemplates lists all flow templates visible to the workspace.
func (h *FlowTemplateHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)

	items, err := h.flowService.ListDBTemplates(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// DuplicateTemplate creates an editable workspace copy of a template.
func (h *FlowTemplateHandler) DuplicateTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	templateID := chi.URLParam(r, "templateId")
	actorID := middleware.GetUserID(r.Context())

	view, err := h.flowService.DuplicateTemplate(r.Context(), workspaceID, templateID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

// DuplicateFromSlug creates an editable workspace copy from a template slug.
// Works for both DB-backed and hardcoded system templates.
func (h *FlowTemplateHandler) DuplicateFromSlug(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req struct {
		TemplateSlug string `json:"template_slug"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TemplateSlug == "" {
		writeError(w, http.StatusBadRequest, "template_slug is required")
		return
	}

	view, err := h.flowService.DuplicateFromSlug(r.Context(), workspaceID, req.TemplateSlug, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

// DeleteTemplate archives a flow template.
func (h *FlowTemplateHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	templateID := chi.URLParam(r, "templateId")

	if err := h.flowService.DeleteTemplate(r.Context(), workspaceID, templateID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

// CreateNode adds a node to a template.
func (h *FlowTemplateHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	templateID := chi.URLParam(r, "templateId")

	var req model.CreateFlowTemplateNodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	node, err := h.flowService.CreateTemplateNode(r.Context(), workspaceID, templateID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

// UpdateNode updates a node within a template.
func (h *FlowTemplateHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	templateID := chi.URLParam(r, "templateId")
	nodeID := chi.URLParam(r, "nodeId")

	var req model.UpdateFlowTemplateNodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	node, err := h.flowService.UpdateTemplateNode(r.Context(), workspaceID, templateID, nodeID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// DeleteNode removes a node from a template.
func (h *FlowTemplateHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	templateID := chi.URLParam(r, "templateId")
	nodeID := chi.URLParam(r, "nodeId")

	if err := h.flowService.DeleteTemplateNode(r.Context(), workspaceID, templateID, nodeID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
