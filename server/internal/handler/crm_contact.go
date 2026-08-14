package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMContactHandler handles CRM contact HTTP endpoints.
type CRMContactHandler struct {
	contactService *service.CRMContactService
}

// NewCRMContactHandler creates a new CRMContactHandler.
func NewCRMContactHandler(contactService *service.CRMContactService) *CRMContactHandler {
	return &CRMContactHandler{contactService: contactService}
}

func writeCRMContactError(w http.ResponseWriter, err error) {
	var entitlementErr *service.EntitlementError
	if errors.As(err, &entitlementErr) {
		writeError(w, http.StatusPaymentRequired, entitlementErr.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

// List handles GET /api/crm/contacts.
func (h *CRMContactHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	queryFilters, err := queryFilterGroup(r, "filters")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filters query")
		return
	}
	filters := model.CRMContactListFilters{
		LifecycleStage: queryStringPtr(r, "lifecycle_stage"),
		LeadStatus:     queryStringPtr(r, "lead_status"),
		OwnerMemberID:  queryStringPtr(r, "owner_member_id"),
		Search:         queryStringPtr(r, "search"),
		Query:          queryFilters,
	}
	pagination := queryPagination(r)

	contacts, total, err := h.contactService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		var validationErr *querybuilder.ValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, validationErr.Error())
			return
		}
		writeCRMContactError(w, err)
		return
	}
	if contacts == nil {
		contacts = []model.CRMContact{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  contacts,
		"total": total,
		"page":  pagination.Page,
	})
}

// Create handles POST /api/crm/contacts.
func (h *CRMContactHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMContactRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	contact, err := h.contactService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, contact)
}

// Seed handles POST /api/crm/contacts/seed.
func (h *CRMContactHandler) Seed(w http.ResponseWriter, r *http.Request) {
	var req model.SeedCRMContactsRequest
	if err := decodeJSON(r, &req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	result, err := h.contactService.Seed(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// Get handles GET /api/crm/contacts/{id}.
func (h *CRMContactHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	contact, err := h.contactService.GetByID(r.Context(), id)
	if err != nil {
		var entitlementErr *service.EntitlementError
		if errors.As(err, &entitlementErr) {
			writeError(w, http.StatusPaymentRequired, entitlementErr.Error())
			return
		}
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, contact)
}

// Update handles PUT /api/crm/contacts/{id}.
func (h *CRMContactHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMContactRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var actorUserID, actorMemberID string
	if actor := authorization.GetActor(r.Context()); actor != nil {
		actorUserID = actor.UserID
		actorMemberID = actor.WorkspaceMemberID
	}
	contact, err := h.contactService.UpdateWithActor(r.Context(), id, req, actorUserID, actorMemberID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, contact)
}

// Delete handles DELETE /api/crm/contacts/{id}.
func (h *CRMContactHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.contactService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "contact deleted"})
}
