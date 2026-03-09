package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMAssociationHandler handles CRM association HTTP endpoints.
type CRMAssociationHandler struct {
	assocService *service.CRMAssociationService
}

// NewCRMAssociationHandler creates a new CRMAssociationHandler.
func NewCRMAssociationHandler(assocService *service.CRMAssociationService) *CRMAssociationHandler {
	return &CRMAssociationHandler{assocService: assocService}
}

// Create handles POST /api/crm/associations.
func (h *CRMAssociationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMAssociationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	assoc, err := h.assocService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, assoc)
}

// Delete handles DELETE /api/crm/associations/{id}.
func (h *CRMAssociationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.assocService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "association deleted"})
}

// ListContactAssociations handles GET /api/crm/contacts/{id}/associations.
func (h *CRMAssociationHandler) ListContactAssociations(w http.ResponseWriter, r *http.Request) {
	h.listByObject(w, r, model.CRMObjectContact)
}

// ListCompanyAssociations handles GET /api/crm/companies/{id}/associations.
func (h *CRMAssociationHandler) ListCompanyAssociations(w http.ResponseWriter, r *http.Request) {
	h.listByObject(w, r, model.CRMObjectCompany)
}

// ListDealAssociations handles GET /api/crm/deals/{id}/associations.
func (h *CRMAssociationHandler) ListDealAssociations(w http.ResponseWriter, r *http.Request) {
	h.listByObject(w, r, model.CRMObjectDeal)
}

func (h *CRMAssociationHandler) listByObject(w http.ResponseWriter, r *http.Request, objectType string) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	objectID := chi.URLParam(r, "id")

	assocs, err := h.assocService.ListByObject(r.Context(), workspaceID, objectType, objectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if assocs == nil {
		assocs = []model.CRMAssociation{}
	}
	writeJSON(w, http.StatusOK, assocs)
}
