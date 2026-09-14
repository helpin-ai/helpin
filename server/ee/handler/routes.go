package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"net/http"
)

type Routes struct {
	billing *BillingHandler
	pricing *AIUsageHandler
	authz   *authorization.AuthzService
}

func NewRoutes(billing *BillingHandler, pricing *AIUsageHandler, authz *authorization.AuthzService) *Routes {
	return &Routes{billing: billing, pricing: pricing, authz: authz}
}
func (h *Routes) RequireActiveWorkspace(next http.Handler) http.Handler {
	return h.billing.RequireUnlockedWorkspace(next)
}
func (h *Routes) RegisterPublic(r chi.Router) {
	r.Get("/ai-pricing", h.pricing.Pricing)
	r.Post("/webhooks/stripe", h.billing.StripeWebhook)
}

func (h *Routes) RegisterAuthenticated(r chi.Router) {

	r.With(h.billing.RequireOrgBillingOwner).Get("/organizations/{id}/billing", h.billing.GetOrganizationBilling)
	r.With(h.billing.RequireOrgBillingOwner).Get("/organizations/{id}/billing/cards", h.billing.ListCards)
	r.With(h.billing.RequireOrgBillingOwner).Put("/organizations/{id}/billing/cards/{cardId}", h.billing.UpdateCard)
	r.With(h.billing.RequireOrgBillingOwner).Delete("/organizations/{id}/billing/cards/{cardId}", h.billing.DeleteCard)
	r.With(h.billing.RequireOrgBillingOwner).Get("/organizations/{id}/billing/invoices", h.billing.ListInvoices)

}
func (h *Routes) RegisterWorkspace(r chi.Router) {

	r.With(authorization.RequirePermission(h.authz, authorization.PermSettingsRead)).Get("/billing", h.billing.Get)
	r.With(authorization.RequirePermission(h.authz, authorization.PermSettingsManage), h.billing.RequireWorkspaceBillingOwner).Post("/billing/checkout", h.billing.Checkout)
	r.With(authorization.RequirePermission(h.authz, authorization.PermSettingsManage), h.billing.RequireWorkspaceBillingOwner).Post("/billing/confirm-checkout", h.billing.ConfirmCheckout)
	r.With(authorization.RequirePermission(h.authz, authorization.PermSettingsManage), h.billing.RequireWorkspaceBillingOwner).Post("/billing/preview-plan-change", h.billing.PreviewPlanChange)
	r.With(authorization.RequirePermission(h.authz, authorization.PermSettingsManage), h.billing.RequireWorkspaceBillingOwner).Post("/billing/change-plan", h.billing.ChangePlan)
	r.With(authorization.RequirePermission(h.authz, authorization.PermSettingsManage), h.billing.RequireWorkspaceBillingOwner).Post("/billing/resume-subscription", h.billing.ResumeSubscription)
	r.With(authorization.RequirePermission(h.authz, authorization.PermSettingsManage), h.billing.RequireWorkspaceBillingOwner).Post("/billing/portal", h.billing.Portal)
	r.With(authorization.RequirePermission(h.authz, authorization.PermSettingsManage), h.billing.RequireWorkspaceBillingOwner).Put("/billing/extra-usage", h.billing.SetOnDemand)
	r.With(authorization.RequirePermission(h.authz, authorization.PermSettingsManage), h.billing.RequireWorkspaceBillingOwner).Post("/billing/test-scenario", h.billing.ApplyTestScenario)
	// Usage is read-only and visible to any settings reader (matches the
	// billing summary). Payment-method changes remain billing-owner-only.
	r.With(authorization.RequirePermission(h.authz, authorization.PermSettingsRead)).Get("/billing/usage", h.billing.GetUsage)
	r.With(h.billing.RequireWorkspaceBillingOwner).Put("/billing/payment-method", h.billing.LinkPaymentMethod)

}
