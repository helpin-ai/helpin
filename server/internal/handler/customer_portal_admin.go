package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CustomerPortalAdminHandler serves support admin portal access controls.
// Routes require the support admin permission.
type CustomerPortalAdminHandler struct {
	portal *service.CustomerPortalService
}

// NewCustomerPortalAdminHandler creates the portal admin handler.
func NewCustomerPortalAdminHandler(portal *service.CustomerPortalService) *CustomerPortalAdminHandler {
	return &CustomerPortalAdminHandler{portal: portal}
}

// AccessSummary handles GET /api/support/inbox/portal/access.
func (h *CustomerPortalAdminHandler) AccessSummary(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	summary, err := h.portal.AccessSummary(r.Context(), workspaceID)
	if err != nil {
		slog.ErrorContext(r.Context(), "portal access summary failed", "workspace_id", workspaceID, "error", err)
		writeError(w, http.StatusInternalServerError, "unable to load portal access")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// ContactAccess handles GET /api/support/inbox/portal/contacts/{contactId}/access.
func (h *CustomerPortalAdminHandler) ContactAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	access, err := h.portal.ContactAccess(r.Context(), workspaceID, chi.URLParam(r, "contactId"))
	if err != nil {
		writePortalAdminError(w, r, workspaceID, err)
		return
	}
	writeJSON(w, http.StatusOK, access)
}

// UpdateContactAccess handles PUT /api/support/inbox/portal/contacts/{contactId}/access.
func (h *CustomerPortalAdminHandler) UpdateContactAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	var body struct {
		PortalAccess *string `json:"portal_access"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	access, err := h.portal.SetContactAccess(r.Context(), workspaceID, chi.URLParam(r, "contactId"), middleware.GetUserID(r.Context()), body.PortalAccess)
	if err != nil {
		writePortalAdminError(w, r, workspaceID, err)
		return
	}
	writeJSON(w, http.StatusOK, access)
}

// ResendIntakeConfirmation handles
// POST /api/support/inbox/conversations/{id}/portal-confirmation.
func (h *CustomerPortalAdminHandler) ResendIntakeConfirmation(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if err := h.portal.ResendIntakeConfirmation(r.Context(), workspaceID, chi.URLParam(r, "id"), middleware.GetUserID(r.Context())); err != nil {
		writePortalAdminError(w, r, workspaceID, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writePortalAdminError(w http.ResponseWriter, r *http.Request, workspaceID string, err error) {
	switch {
	case errors.Is(err, service.ErrPortalContactNotFound), errors.Is(err, service.ErrPortalRequestNotFound), errors.Is(err, service.ErrPortalCompanyNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrPortalAccessInvalid), errors.Is(err, service.ErrPortalRoleInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrPortalUnavailable):
		writeError(w, http.StatusConflict, "the customer portal is not enabled")
	case errors.Is(err, service.ErrPortalCustomerNotEligible), errors.Is(err, service.ErrPortalLinkNotSent):
		writeError(w, http.StatusConflict, err.Error())
	default:
		slog.ErrorContext(r.Context(), "portal admin request failed", "workspace_id", workspaceID, "action", r.Method, "error", err)
		writeError(w, http.StatusInternalServerError, "unable to update portal access")
	}
}

// CompanyAccess handles GET /api/support/inbox/portal/companies/{companyId}.
func (h *CustomerPortalAdminHandler) CompanyAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	access, err := h.portal.CompanyAccess(r.Context(), workspaceID, chi.URLParam(r, "companyId"))
	if err != nil {
		writePortalAdminError(w, r, workspaceID, err)
		return
	}
	writeJSON(w, http.StatusOK, access)
}

// UpdateCompanyAccess handles PUT /api/support/inbox/portal/companies/{companyId}.
// A null or empty default_role turns company features off.
func (h *CustomerPortalAdminHandler) UpdateCompanyAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	var body struct {
		DefaultRole *string `json:"default_role"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	access, err := h.portal.SetCompanyDefaultRole(r.Context(), workspaceID, chi.URLParam(r, "companyId"), middleware.GetUserID(r.Context()), body.DefaultRole)
	if err != nil {
		writePortalAdminError(w, r, workspaceID, err)
		return
	}
	writeJSON(w, http.StatusOK, access)
}

// AllowCompanyContacts handles
// POST /api/support/inbox/portal/companies/{companyId}/allow-contacts.
func (h *CustomerPortalAdminHandler) AllowCompanyContacts(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	access, allowed, err := h.portal.AllowCompanyContacts(r.Context(), workspaceID, chi.URLParam(r, "companyId"), middleware.GetUserID(r.Context()))
	if err != nil {
		writePortalAdminError(w, r, workspaceID, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"allowed": allowed, "company": access})
}

// UpdateContactRole handles PUT /api/support/inbox/portal/contacts/{contactId}/role.
// A null or empty portal_role follows the contact's companies.
func (h *CustomerPortalAdminHandler) UpdateContactRole(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	var body struct {
		PortalRole *string `json:"portal_role"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	access, err := h.portal.SetContactRole(r.Context(), workspaceID, chi.URLParam(r, "contactId"), middleware.GetUserID(r.Context()), body.PortalRole)
	if err != nil {
		writePortalAdminError(w, r, workspaceID, err)
		return
	}
	writeJSON(w, http.StatusOK, access)
}

// ConversationPortalContext handles
// GET /api/support/inbox/conversations/{id}/portal-context: who besides the
// requester reads it in the portal, and the articles the customer opened
// before writing in. Either part is null when it does not apply.
func (h *CustomerPortalAdminHandler) ConversationPortalContext(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	portalContext, err := h.portal.ConversationPortalContext(r.Context(), workspaceID, chi.URLParam(r, "id"))
	if err != nil {
		writePortalAdminError(w, r, workspaceID, err)
		return
	}
	writeJSON(w, http.StatusOK, portalContext)
}
