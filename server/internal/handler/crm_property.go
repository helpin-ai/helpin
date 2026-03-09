package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMPropertyHandler handles CRM property definition and group endpoints.
type CRMPropertyHandler struct {
	propertyService *service.CRMPropertyService
}

// NewCRMPropertyHandler creates a new CRMPropertyHandler.
func NewCRMPropertyHandler(propertyService *service.CRMPropertyService) *CRMPropertyHandler {
	return &CRMPropertyHandler{propertyService: propertyService}
}

// ── Property Definitions ──

// ListDefinitions handles GET /api/crm/properties.
func (h *CRMPropertyHandler) ListDefinitions(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	objectType := r.URL.Query().Get("object_type")

	defs, err := h.propertyService.ListDefinitions(r.Context(), workspaceID, objectType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if defs == nil {
		defs = []model.CRMPropertyDefinition{}
	}
	writeJSON(w, http.StatusOK, defs)
}

// CreateDefinition handles POST /api/crm/properties.
func (h *CRMPropertyHandler) CreateDefinition(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMPropertyDefinitionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	def, err := h.propertyService.CreateDefinition(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, def)
}

// UpdateDefinition handles PUT /api/crm/properties/{id}.
func (h *CRMPropertyHandler) UpdateDefinition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMPropertyDefinitionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	def, err := h.propertyService.UpdateDefinition(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, def)
}

// DeleteDefinition handles DELETE /api/crm/properties/{id}.
func (h *CRMPropertyHandler) DeleteDefinition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.propertyService.DeleteDefinition(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "property definition deleted"})
}

// ── Property Groups ──

// ListGroups handles GET /api/crm/property-groups.
func (h *CRMPropertyHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	objectType := r.URL.Query().Get("object_type")

	groups, err := h.propertyService.ListGroups(r.Context(), workspaceID, objectType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if groups == nil {
		groups = []model.CRMPropertyGroup{}
	}
	writeJSON(w, http.StatusOK, groups)
}

// CreateGroup handles POST /api/crm/property-groups.
func (h *CRMPropertyHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMPropertyGroupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	group, err := h.propertyService.CreateGroup(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

// UpdateGroup handles PUT /api/crm/property-groups/{id}.
func (h *CRMPropertyHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMPropertyGroupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	group, err := h.propertyService.UpdateGroup(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, group)
}

// DeleteGroup handles DELETE /api/crm/property-groups/{id}.
func (h *CRMPropertyHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.propertyService.DeleteGroup(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "property group deleted"})
}
