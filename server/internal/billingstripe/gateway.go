package billingstripe

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	stripe "github.com/stripe/stripe-go/v86"
	portalsession "github.com/stripe/stripe-go/v86/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v86/checkout/session"
	"github.com/stripe/stripe-go/v86/customer"
	"github.com/stripe/stripe-go/v86/invoice"
	"github.com/stripe/stripe-go/v86/invoiceitem"
	"github.com/stripe/stripe-go/v86/paymentmethod"
	"github.com/stripe/stripe-go/v86/setupintent"

	"github.com/helpin-ai/helpin/server/internal/service"
)

var _ service.BillingStripeGateway = (*Gateway)(nil)

type Gateway struct {
	secretKey          string
	creditBlockPriceID string
}

func New(secretKey, creditBlockPriceID string) *Gateway {
	secretKey = strings.TrimSpace(secretKey)
	if secretKey == "" {
		return nil
	}
	stripe.Key = secretKey
	return &Gateway{
		secretKey:          secretKey,
		creditBlockPriceID: strings.TrimSpace(creditBlockPriceID),
	}
}

func (g *Gateway) CreateCheckoutSession(ctx context.Context, input service.BillingCheckoutInput) (string, error) {
	_ = ctx
	if strings.TrimSpace(input.PriceID) == "" {
		return "", fmt.Errorf("stripe price ID is required")
	}
	customerID := strings.TrimSpace(input.CustomerID)
	if customerID == "" {
		params := &stripe.CustomerParams{
			Email: stripe.String(strings.TrimSpace(input.Email)),
			Metadata: map[string]string{
				"workspace_id": input.WorkspaceID,
				"user_id":      input.UserID,
			},
		}
		created, err := customer.New(params)
		if err != nil {
			return "", fmt.Errorf("create stripe customer: %w", err)
		}
		customerID = created.ID
	}

	successURL := withBillingResult(input.ReturnURL, "success")
	cancelURL := withBillingResult(input.ReturnURL, "cancelled")
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:          stripe.String(customerID),
		ClientReferenceID: stripe.String(input.WorkspaceID),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			Price:    stripe.String(input.PriceID),
			Quantity: stripe.Int64(1),
		}},
		Metadata: map[string]string{
			"workspace_id": input.WorkspaceID,
			"user_id":      input.UserID,
			"plan":         input.Plan,
			"interval":     input.Interval,
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"workspace_id": input.WorkspaceID,
				"plan":         input.Plan,
				"interval":     input.Interval,
			},
		},
	}
	session, err := checkoutsession.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe checkout session: %w", err)
	}
	return session.URL, nil
}

func withBillingResult(rawURL, result string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		separator := "?"
		if strings.Contains(rawURL, "?") {
			separator = "&"
		}
		return rawURL + separator + "billing=" + url.QueryEscape(result)
	}
	values := parsed.Query()
	values.Set("billing", result)
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func (g *Gateway) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	_ = ctx
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	}
	session, err := portalsession.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe billing portal session: %w", err)
	}
	return session.URL, nil
}

func (g *Gateway) BillCreditBlock(ctx context.Context, input service.BillingCreditBlockCharge) error {
	_ = ctx
	if strings.TrimSpace(input.CustomerID) == "" {
		return fmt.Errorf("stripe customer ID is required")
	}
	description := fmt.Sprintf("Helpin on-demand AI credits (%d x 5,000)", input.Blocks)
	params := &stripe.InvoiceItemParams{
		Customer:     stripe.String(input.CustomerID),
		Subscription: stripe.String(input.SubscriptionID),
		Description:  stripe.String(description),
		Metadata: map[string]string{
			"workspace_id": input.WorkspaceID,
			"blocks":       fmt.Sprintf("%d", input.Blocks),
		},
	}
	if g.creditBlockPriceID != "" {
		params.Pricing = &stripe.InvoiceItemPricingParams{Price: stripe.String(g.creditBlockPriceID)}
		params.Quantity = stripe.Int64(int64(input.Blocks))
	} else {
		params.Amount = stripe.Int64(int64(input.AmountCents))
		params.Currency = stripe.String(string(stripe.CurrencyUSD))
	}
	params.SetIdempotencyKey(input.IdempotencyKey)
	if _, err := invoiceitem.New(params); err != nil {
		return fmt.Errorf("create stripe credit invoice item: %w", err)
	}
	return nil
}

// EnsureCustomer finds or creates the Stripe customer for an organization.
func (g *Gateway) EnsureCustomer(ctx context.Context, orgID, email string) (string, error) {
	_ = ctx
	params := &stripe.CustomerParams{
		Email: stripe.String(strings.TrimSpace(email)),
		Metadata: map[string]string{
			"organization_id": orgID,
		},
	}
	created, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("ensure stripe customer: %w", err)
	}
	return created.ID, nil
}

// CreateSetupIntent starts a card-collection flow for a customer and returns the
// client secret.
func (g *Gateway) CreateSetupIntent(ctx context.Context, customerID string) (string, error) {
	_ = ctx
	if strings.TrimSpace(customerID) == "" {
		return "", fmt.Errorf("stripe customer ID is required")
	}
	params := &stripe.SetupIntentParams{
		Customer:           stripe.String(customerID),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Usage:              stripe.String("off_session"),
	}
	si, err := setupintent.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe setup intent: %w", err)
	}
	return si.ClientSecret, nil
}

// ListPaymentMethods returns the saved cards for a customer.
func (g *Gateway) ListPaymentMethods(ctx context.Context, customerID string) ([]service.StripePaymentMethod, error) {
	_ = ctx
	if strings.TrimSpace(customerID) == "" {
		return nil, fmt.Errorf("stripe customer ID is required")
	}
	params := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerID),
		Type:     stripe.String("card"),
	}
	var out []service.StripePaymentMethod
	iter := paymentmethod.List(params)
	for iter.Next() {
		pm := iter.PaymentMethod()
		item := service.StripePaymentMethod{ID: pm.ID}
		if pm.Card != nil {
			item.Brand = string(pm.Card.Brand)
			item.Last4 = pm.Card.Last4
			item.ExpMonth = int(pm.Card.ExpMonth)
			item.ExpYear = int(pm.Card.ExpYear)
		}
		if pm.BillingDetails != nil {
			item.Cardholder = pm.BillingDetails.Name
		}
		out = append(out, item)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("list stripe payment methods: %w", err)
	}
	return out, nil
}

// DetachPaymentMethod removes a saved card from its customer.
func (g *Gateway) DetachPaymentMethod(ctx context.Context, paymentMethodID string) error {
	_ = ctx
	if strings.TrimSpace(paymentMethodID) == "" {
		return fmt.Errorf("stripe payment method ID is required")
	}
	if _, err := paymentmethod.Detach(paymentMethodID, nil); err != nil {
		return fmt.Errorf("detach stripe payment method: %w", err)
	}
	return nil
}

// SetDefaultPaymentMethod sets the customer's default invoice payment method.
func (g *Gateway) SetDefaultPaymentMethod(ctx context.Context, customerID, paymentMethodID string) error {
	_ = ctx
	params := &stripe.CustomerParams{
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(paymentMethodID),
		},
	}
	if _, err := customer.Update(customerID, params); err != nil {
		return fmt.Errorf("set stripe default payment method: %w", err)
	}
	return nil
}

// ListInvoices returns invoices for a customer.
func (g *Gateway) ListInvoices(ctx context.Context, customerID string) ([]service.StripeInvoice, error) {
	_ = ctx
	if strings.TrimSpace(customerID) == "" {
		return []service.StripeInvoice{}, nil
	}
	params := &stripe.InvoiceListParams{Customer: stripe.String(customerID)}
	params.Limit = stripe.Int64(50)
	var out []service.StripeInvoice
	iter := invoice.List(params)
	for iter.Next() {
		inv := iter.Invoice()
		item := service.StripeInvoice{
			ID:         inv.ID,
			Number:     inv.Number,
			Status:     string(inv.Status),
			AmountDue:  inv.AmountDue,
			AmountPaid: inv.AmountPaid,
			Currency:   string(inv.Currency),
			Created:    inv.Created,
			HostedURL:  inv.HostedInvoiceURL,
			PDFURL:     inv.InvoicePDF,
		}
		if inv.PeriodStart > 0 {
			item.PeriodStart = inv.PeriodStart
		}
		if inv.PeriodEnd > 0 {
			item.PeriodEnd = inv.PeriodEnd
		}
		out = append(out, item)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("list stripe invoices: %w", err)
	}
	return out, nil
}
