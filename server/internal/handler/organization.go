package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// OrganizationHandler handles organization HTTP requests.
type OrganizationHandler struct {
	orgService *service.OrganizationService
}

// NewOrganizationHandler creates a new OrganizationHandler.
func NewOrganizationHandler(orgService *service.OrganizationService) *OrganizationHandler {
	return &OrganizationHandler{orgService: orgService}
}

// List handles GET /api/organizations.
func (h *OrganizationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	orgs, err := h.orgService.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if orgs == nil {
		orgs = []model.OrganizationWithRole{}
	}
	writeJSON(w, http.StatusOK, orgs)
}

// Create handles POST /api/organizations.
func (h *OrganizationHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.CreateOrganizationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	org, err := h.orgService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, org)
}

// Get handles GET /api/organizations/{id}.
func (h *OrganizationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	org, err := h.orgService.GetByID(r.Context(), id, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, org)
}

// Update handles PUT /api/organizations/{id}.
func (h *OrganizationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	var req model.UpdateOrganizationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	org, err := h.orgService.Update(r.Context(), id, userID, req)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, org)
}

// Delete handles DELETE /api/organizations/{id}.
func (h *OrganizationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	if err := h.orgService.Delete(r.Context(), id, userID); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "organization deleted"})
}

// ListMembers handles GET /api/organizations/{id}/members.
func (h *OrganizationHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	members, err := h.orgService.ListMembers(r.Context(), id, userID)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if members == nil {
		members = []model.MemberWithUser{}
	}
	writeJSON(w, http.StatusOK, members)
}

// AddMember handles POST /api/organizations/{id}/members.
func (h *OrganizationHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	var req model.AddOrgMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	member, err := h.orgService.AddMember(r.Context(), id, userID, req)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, member)
}

// UpdateMember handles PUT /api/organizations/{id}/members/{userId}.
func (h *OrganizationHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	targetUserID := chi.URLParam(r, "userId")
	userID := middleware.GetUserID(r.Context())

	var req model.UpdateOrgMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.orgService.UpdateMember(r.Context(), id, userID, targetUserID, req); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "member updated"})
}

// RemoveMember handles DELETE /api/organizations/{id}/members/{userId}.
func (h *OrganizationHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	targetUserID := chi.URLParam(r, "userId")
	userID := middleware.GetUserID(r.Context())

	if err := h.orgService.RemoveMember(r.Context(), id, userID, targetUserID); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "member removed"})
}
