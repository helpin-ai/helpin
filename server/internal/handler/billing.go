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
	billingService      *service.BillingService
	stripeWebhookSecret string
	appBaseURL          string
}

func NewBillingHandler(billingService *service.BillingService, webhookSecret, appBaseURL string) *BillingHandler {
	return &BillingHandler{
		billingService:      billingService,
		stripeWebhookSecret: strings.TrimSpace(webhookSecret),
		appBaseURL:          strings.TrimRight(strings.TrimSpace(appBaseURL), "/"),
	}
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

type billingCheckoutRequest struct {
	Plan      string `json:"plan"`
	Interval  string `json:"interval"`
	ReturnURL string `json:"return_url"`
}

func (h *BillingHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
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

func (h *BillingHandler) Portal(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
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
		CurrentPeriodEnd:     now.AddDate(0, 1, 0),
	})
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
	})
	return err
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
