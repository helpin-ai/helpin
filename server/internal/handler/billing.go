package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type BillingHandler struct {
	billingService       *service.BillingService
	testScenarioService  *service.BillingTestScenarioService
	testScenariosEnabled bool
	stripeWebhookSecret  string
	appBaseURL           string
}

func NewBillingHandler(billingService *service.BillingService, webhookSecret, appBaseURL string, testScenarioService *service.BillingTestScenarioService, testScenariosEnabled bool) *BillingHandler {
	return &BillingHandler{
		billingService:       billingService,
		testScenarioService:  testScenarioService,
		testScenariosEnabled: testScenariosEnabled,
		stripeWebhookSecret:  strings.TrimSpace(webhookSecret),
		appBaseURL:           strings.TrimRight(strings.TrimSpace(appBaseURL), "/"),
	}
}

func (h *BillingHandler) RequireUnlockedWorkspace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.billingService == nil || billingLockAllowedRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		workspaceID := middleware.GetWorkspaceID(r.Context())
		if workspaceID == "" {
			workspaceID = chi.URLParam(r, "id")
		}
		if workspaceID == "" {
			next.ServeHTTP(w, r)
			return
		}
		summary, err := h.billingService.GetWorkspaceBilling(r.Context(), workspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve workspace billing")
			return
		}
		if summary != nil && summary.Locked {
			writeError(w, http.StatusPaymentRequired, "workspace is locked; choose a plan to reactivate it")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func billingLockAllowedRequest(r *http.Request) bool {
	path := r.URL.Path
	if strings.Contains(path, "/billing") {
		return true
	}
	if r.Method == http.MethodGet && strings.HasSuffix(path, "/settings") {
		return true
	}
	if r.Method == http.MethodGet && (strings.HasSuffix(path, "/me") || strings.HasSuffix(path, "/my-role") || strings.HasSuffix(path, "/my-membership")) {
		return true
	}
	return false
}

func (h *BillingHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	summary, err := h.billingService.GetWorkspaceBilling(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

type billingTestScenarioRequest struct {
	Scenario string `json:"scenario"`
}

func (h *BillingHandler) ApplyTestScenario(w http.ResponseWriter, r *http.Request) {
	if h == nil || !h.testScenariosEnabled || h.testScenarioService == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	workspaceID := chi.URLParam(r, "id")
	if !h.canManageWorkspace(w, r, workspaceID) {
		return
	}
	var req billingTestScenarioRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	summary, err := h.testScenarioService.Apply(r.Context(), workspaceID, req.Scenario)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

type billingCheckoutRequest struct {
	Plan      string `json:"plan"`
	Interval  string `json:"interval"`
	ReturnURL string `json:"return_url"`
}

type billingPlanChangeRequest struct {
	Plan          string `json:"plan"`
	Interval      string `json:"interval"`
	ProrationDate int64  `json:"proration_date,omitempty"`
}

type billingConfirmCheckoutRequest struct {
	SessionID string `json:"session_id"`
}

func (h *BillingHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if !h.canManageWorkspace(w, r, workspaceID) {
		return
	}
	var req billingCheckoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	url, err := h.billingService.CreateCheckoutSession(r.Context(), service.BillingCheckoutRequest{
		WorkspaceID: workspaceID,
		UserID:      middleware.GetUserID(r.Context()),
		Email:       middleware.GetUserEmail(r.Context()),
		Plan:        req.Plan,
		Interval:    req.Interval,
		ReturnURL:   h.returnURL(req.ReturnURL, workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *BillingHandler) ConfirmCheckout(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if !h.canManageWorkspace(w, r, workspaceID) {
		return
	}
	var req billingConfirmCheckoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	summary, err := h.billingService.ConfirmCheckoutSession(r.Context(), workspaceID, strings.TrimSpace(req.SessionID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *BillingHandler) PreviewPlanChange(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if !h.canManageWorkspace(w, r, workspaceID) {
		return
	}
	var req billingPlanChangeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	preview, err := h.billingService.PreviewWorkspacePlanChange(r.Context(), service.BillingPlanChangeRequest{
		WorkspaceID:   workspaceID,
		Plan:          req.Plan,
		Interval:      req.Interval,
		ProrationDate: req.ProrationDate,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *BillingHandler) ChangePlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if !h.canManageWorkspace(w, r, workspaceID) {
		return
	}
	var req billingPlanChangeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	summary, err := h.billingService.ChangeWorkspacePlan(r.Context(), service.BillingPlanChangeRequest{
		WorkspaceID:   workspaceID,
		Plan:          req.Plan,
		Interval:      req.Interval,
		ProrationDate: req.ProrationDate,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *BillingHandler) ResumeSubscription(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if !h.canManageWorkspace(w, r, workspaceID) {
		return
	}
	summary, err := h.billingService.ResumeWorkspaceSubscription(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *BillingHandler) Portal(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if !h.canManageWorkspace(w, r, workspaceID) {
		return
	}
	var req struct {
		ReturnURL string `json:"return_url"`
	}
	_ = decodeJSON(r, &req)
	url, err := h.billingService.CreatePortalSession(r.Context(), workspaceID, h.returnURL(req.ReturnURL, workspaceID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

type billingOnDemandRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *BillingHandler) SetOnDemand(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if !h.canManageWorkspace(w, r, workspaceID) {
		return
	}
	var req billingOnDemandRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	summary, err := h.billingService.SetOnDemandEnabled(r.Context(), workspaceID, req.Enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *BillingHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	if h.stripeWebhookSecret == "" {
		writeError(w, http.StatusServiceUnavailable, "stripe webhook secret is not configured")
		return
	}
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook body")
		return
	}
	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), h.stripeWebhookSecret)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid stripe signature")
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		if err := h.handleCheckoutCompleted(r, event); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		if err := h.handleSubscriptionEvent(r, event); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case "invoice.payment_failed":
		if err := h.handleInvoicePaymentFailed(r, event); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case "invoice.payment_succeeded":
		if err := h.handleInvoicePaymentSucceeded(r, event); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case "customer.subscription.trial_will_end":
		if err := h.handleTrialWillEnd(r, event); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"received": true})
}

func (h *BillingHandler) handleCheckoutCompleted(r *http.Request, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return err
	}
	workspaceID := session.Metadata["workspace_id"]
	if workspaceID == "" {
		return nil
	}
	customerID := ""
	if session.Customer != nil {
		customerID = session.Customer.ID
	}
	subscriptionID := ""
	if session.Subscription != nil {
		subscriptionID = session.Subscription.ID
	}
	now := time.Now().UTC()
	periodEnd := now.AddDate(0, 1, 0)
	if session.Metadata["interval"] == "annual" {
		periodEnd = now.AddDate(1, 0, 0)
	}
	_, err := h.billingService.ApplyStripeSubscriptionUpdate(r.Context(), service.BillingStripeSubscriptionUpdate{
		EventID:              event.ID,
		EventType:            string(event.Type),
		WorkspaceID:          workspaceID,
		Plan:                 session.Metadata["plan"],
		Status:               model.BillingStatusActive,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: subscriptionID,
		BillingInterval:      session.Metadata["interval"],
		CurrentPeriodStart:   now,
		CurrentPeriodEnd:     periodEnd,
		CancelAtPeriodEnd:    false,
	})
	return err
}

func (h *BillingHandler) handleInvoicePaymentFailed(r *http.Request, event stripe.Event) error {
	invoiceEvent, err := parseStripeInvoiceEvent(event)
	if err != nil {
		return err
	}
	_, err = h.billingService.ApplyStripeInvoicePaymentFailed(r.Context(), invoiceEvent)
	return err
}

func (h *BillingHandler) handleInvoicePaymentSucceeded(r *http.Request, event stripe.Event) error {
	invoiceEvent, err := parseStripeInvoiceEvent(event)
	if err != nil {
		return err
	}
	_, err = h.billingService.ApplyStripeInvoicePaymentSucceeded(r.Context(), invoiceEvent)
	return err
}

func (h *BillingHandler) handleSubscriptionEvent(r *http.Request, event stripe.Event) error {
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return err
	}
	workspaceID := subscription.Metadata["workspace_id"]
	if workspaceID == "" {
		return nil
	}
	plan := subscription.Metadata["plan"]
	interval := subscription.Metadata["interval"]
	priceID := ""
	currentPeriodStart := time.Now().UTC()
	currentPeriodEnd := currentPeriodStart.AddDate(0, 1, 0)
	if subscription.Items != nil && len(subscription.Items.Data) > 0 {
		item := subscription.Items.Data[0]
		if item.Price != nil {
			priceID = item.Price.ID
			if interval == "" && item.Price.Recurring != nil {
				interval = string(item.Price.Recurring.Interval)
			}
			if interval == "year" {
				interval = "annual"
			}
		}
		if item.CurrentPeriodStart > 0 {
			currentPeriodStart = time.Unix(item.CurrentPeriodStart, 0).UTC()
		}
		if item.CurrentPeriodEnd > 0 {
			currentPeriodEnd = time.Unix(item.CurrentPeriodEnd, 0).UTC()
		}
	}
	customerID := ""
	if subscription.Customer != nil {
		customerID = subscription.Customer.ID
	}
	var canceledAt *time.Time
	if subscription.CanceledAt > 0 {
		t := time.Unix(subscription.CanceledAt, 0).UTC()
		canceledAt = &t
	}
	_, err := h.billingService.ApplyStripeSubscriptionUpdate(r.Context(), service.BillingStripeSubscriptionUpdate{
		EventID:              event.ID,
		EventType:            string(event.Type),
		WorkspaceID:          workspaceID,
		Plan:                 plan,
		Status:               string(subscription.Status),
		StripeCustomerID:     customerID,
		StripeSubscriptionID: subscription.ID,
		StripePriceID:        priceID,
		BillingInterval:      interval,
		CurrentPeriodStart:   currentPeriodStart,
		CurrentPeriodEnd:     currentPeriodEnd,
		CancelAtPeriodEnd:    subscription.CancelAtPeriodEnd,
		CanceledAt:           canceledAt,
	})
	return err
}

func (h *BillingHandler) handleTrialWillEnd(r *http.Request, event stripe.Event) error {
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return err
	}
	customerID := ""
	if subscription.Customer != nil {
		customerID = subscription.Customer.ID
	}
	var trialEnd time.Time
	if subscription.TrialEnd > 0 {
		trialEnd = time.Unix(subscription.TrialEnd, 0).UTC()
	}
	_, err := h.billingService.ApplyStripeTrialWillEnd(r.Context(), service.BillingStripeTrialWillEndEvent{
		EventID:        event.ID,
		EventType:      string(event.Type),
		SubscriptionID: subscription.ID,
		CustomerID:     customerID,
		TrialEndsAt:    trialEnd,
	})
	return err
}

func parseStripeInvoiceEvent(event stripe.Event) (service.BillingStripeInvoiceEvent, error) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return service.BillingStripeInvoiceEvent{}, err
	}
	customerID := ""
	if invoice.Customer != nil {
		customerID = invoice.Customer.ID
	}
	subscriptionID := ""
	if invoice.Parent != nil && invoice.Parent.SubscriptionDetails != nil && invoice.Parent.SubscriptionDetails.Subscription != nil {
		subscriptionID = invoice.Parent.SubscriptionDetails.Subscription.ID
	}
	if subscriptionID == "" {
		var legacy struct {
			Subscription string `json:"subscription"`
		}
		if err := json.Unmarshal(event.Data.Raw, &legacy); err == nil {
			subscriptionID = legacy.Subscription
		}
	}
	return service.BillingStripeInvoiceEvent{
		EventID:        event.ID,
		EventType:      string(event.Type),
		SubscriptionID: subscriptionID,
		CustomerID:     customerID,
		InvoiceID:      invoice.ID,
	}, nil
}

func (h *BillingHandler) returnURL(requestURL, workspaceID string) string {
	if strings.TrimSpace(requestURL) != "" {
		return strings.TrimSpace(requestURL)
	}
	if h.appBaseURL == "" {
		return ""
	}
	return h.appBaseURL + "/settings/billing?workspace_id=" + workspaceID
}
