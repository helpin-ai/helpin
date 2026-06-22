package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// canManageOrg verifies the requester may manage org-level billing, writing the
// appropriate error response and returning false when not.
func (h *BillingHandler) canManageOrg(w http.ResponseWriter, r *http.Request, orgID string) bool {
	userID := middleware.GetUserID(r.Context())
	ok, err := h.billingService.CanManageOrgBilling(r.Context(), userID, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve billing access")
		return false
	}
	if !ok {
		writeError(w, http.StatusForbidden, "not authorized to manage organization billing")
		return false
	}
	return true
}

// canManageWorkspace verifies the requester may manage the workspace's billing.
func (h *BillingHandler) canManageWorkspace(w http.ResponseWriter, r *http.Request, workspaceID string) bool {
	userID := middleware.GetUserID(r.Context())
	ok, err := h.billingService.CanManageWorkspaceBilling(r.Context(), userID, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve billing access")
		return false
	}
	if !ok {
		writeError(w, http.StatusForbidden, "not authorized to manage workspace billing")
		return false
	}
	return true
}

func (h *BillingHandler) RequireOrgBillingOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.canManageOrg(w, r, chi.URLParam(r, "orgId")) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *BillingHandler) RequireWorkspaceBillingOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.canManageWorkspace(w, r, chi.URLParam(r, "id")) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetOrganizationBilling returns the org roll-up + per-workspace cards.
func (h *BillingHandler) GetOrganizationBilling(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	if !h.canManageOrg(w, r, orgID) {
		return
	}
	userID := middleware.GetUserID(r.Context())
	summary, err := h.billingService.GetOrganizationBilling(r.Context(), userID, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// ListCards returns saved cards with linked-workspace counts.
func (h *BillingHandler) ListCards(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	if !h.canManageOrg(w, r, orgID) {
		return
	}
	cards, err := h.billingService.ListPaymentMethods(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cards)
}

// UpdateCard edits card details or sets it as the org default.
func (h *BillingHandler) UpdateCard(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	cardID := chi.URLParam(r, "cardId")
	if !h.canManageOrg(w, r, orgID) {
		return
	}
	var req model.UpdatePaymentMethodRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pm, err := h.billingService.UpdatePaymentMethod(r.Context(), orgID, cardID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pm)
}

// DeleteCard detaches the card and unlinks workspaces using it.
func (h *BillingHandler) DeleteCard(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	cardID := chi.URLParam(r, "cardId")
	if !h.canManageOrg(w, r, orgID) {
		return
	}
	if err := h.billingService.DeletePaymentMethod(r.Context(), orgID, cardID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ListInvoices returns the org's Stripe invoices.
func (h *BillingHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	if !h.canManageOrg(w, r, orgID) {
		return
	}
	invoices, err := h.billingService.ListInvoices(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, invoices)
}

// GetUsage returns daily/cumulative usage for a workspace.
func (h *BillingHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if !h.canManageWorkspace(w, r, workspaceID) {
		return
	}
	period := r.URL.Query().Get("period")
	mode := r.URL.Query().Get("mode")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	usage, err := h.billingService.GetWorkspaceUsage(r.Context(), workspaceID, period, mode, start, end)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, usage)
}

// LinkPaymentMethod links a saved card to a workspace (null clears it).
func (h *BillingHandler) LinkPaymentMethod(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if !h.canManageWorkspace(w, r, workspaceID) {
		return
	}
	var req model.LinkPaymentMethodRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.billingService.LinkWorkspacePaymentMethod(r.Context(), workspaceID, req.PaymentMethodID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	summary, err := h.billingService.GetWorkspaceBilling(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
