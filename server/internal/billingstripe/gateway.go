package billingstripe

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
	portalsession "github.com/stripe/stripe-go/v86/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v86/checkout/session"
	"github.com/stripe/stripe-go/v86/customer"
	"github.com/stripe/stripe-go/v86/invoice"
	"github.com/stripe/stripe-go/v86/invoiceitem"
	"github.com/stripe/stripe-go/v86/paymentmethod"
	"github.com/stripe/stripe-go/v86/setupintent"
	"github.com/stripe/stripe-go/v86/subscription"
	"github.com/stripe/stripe-go/v86/subscriptionschedule"

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
	if result == "success" {
		values.Set("checkout_session_id", "{CHECKOUT_SESSION_ID}")
	}
	parsed.RawQuery = values.Encode()
	if result == "success" {
		parsed.RawQuery = strings.ReplaceAll(parsed.RawQuery, "%7BCHECKOUT_SESSION_ID%7D", "{CHECKOUT_SESSION_ID}")
	}
	return parsed.String()
}

func (g *Gateway) RetrieveCheckoutSession(ctx context.Context, sessionID string) (*service.BillingCheckoutSession, error) {
	_ = ctx
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("checkout session ID is required")
	}
	session, err := checkoutsession.Get(sessionID, &stripe.CheckoutSessionParams{
		Params: stripe.Params{
			Expand: []*string{
				stripe.String("subscription"),
				stripe.String("customer"),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("retrieve stripe checkout session: %w", err)
	}
	out := &service.BillingCheckoutSession{
		ID:            session.ID,
		WorkspaceID:   strings.TrimSpace(session.ClientReferenceID),
		Plan:          session.Metadata["plan"],
		Interval:      session.Metadata["interval"],
		Status:        string(session.Status),
		PaymentStatus: string(session.PaymentStatus),
	}
	if out.WorkspaceID == "" {
		out.WorkspaceID = session.Metadata["workspace_id"]
	}
	if session.Customer != nil {
		out.StripeCustomerID = session.Customer.ID
	}
	if session.Subscription != nil {
		out.StripeSubscriptionID = session.Subscription.ID
		out.SubscriptionStatus = string(session.Subscription.Status)
		out.CancelAtPeriodEnd = session.Subscription.CancelAtPeriodEnd
		if session.Subscription.Items != nil && len(session.Subscription.Items.Data) > 0 && session.Subscription.Items.Data[0] != nil && session.Subscription.Items.Data[0].Price != nil {
			item := session.Subscription.Items.Data[0]
			out.CurrentPeriodStart = timeFromStripeUnix(item.CurrentPeriodStart)
			out.CurrentPeriodEnd = timeFromStripeUnix(item.CurrentPeriodEnd)
			out.StripePriceID = item.Price.ID
		}
	}
	return out, nil
}

func timeFromStripeUnix(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(value, 0).UTC()
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

func (g *Gateway) PreviewSubscriptionPriceChange(ctx context.Context, input service.BillingSubscriptionChangeInput) (*service.BillingStripeInvoicePreview, error) {
	_ = ctx
	if strings.TrimSpace(input.SubscriptionID) == "" {
		return nil, fmt.Errorf("stripe subscription ID is required")
	}
	if strings.TrimSpace(input.PriceID) == "" {
		return nil, fmt.Errorf("stripe price ID is required")
	}
	sub, err := subscription.Get(input.SubscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("retrieve stripe subscription: %w", err)
	}
	itemID := firstSubscriptionItemID(sub)
	if itemID == "" {
		return nil, fmt.Errorf("stripe subscription has no subscription item")
	}
	prorationDate := input.ProrationDate
	if prorationDate <= 0 {
		prorationDate = time.Now().UTC().Unix()
	}
	params := &stripe.InvoiceCreatePreviewParams{
		Customer:     stripe.String(sub.Customer.ID),
		Subscription: stripe.String(input.SubscriptionID),
		SubscriptionDetails: &stripe.InvoiceCreatePreviewSubscriptionDetailsParams{
			Items: []*stripe.InvoiceCreatePreviewSubscriptionDetailsItemParams{{
				ID:       stripe.String(itemID),
				Price:    stripe.String(input.PriceID),
				Quantity: stripe.Int64(1),
			}},
			ProrationBehavior: stripe.String("always_invoice"),
			ProrationDate:     stripe.Int64(prorationDate),
		},
	}
	preview, err := invoice.CreatePreview(params)
	if err != nil {
		return nil, fmt.Errorf("preview stripe subscription price change: %w", err)
	}
	out := &service.BillingStripeInvoicePreview{
		AmountDueCents:     preview.AmountDue,
		SubtotalCents:      preview.Subtotal,
		TotalCents:         preview.Total,
		Currency:           string(preview.Currency),
		NextPaymentAttempt: preview.NextPaymentAttempt,
	}
	if preview.PeriodStart > 0 {
		out.PeriodStart = preview.PeriodStart
	}
	if preview.PeriodEnd > 0 {
		out.PeriodEnd = preview.PeriodEnd
	}
	if preview.Lines != nil {
		for _, line := range preview.Lines.Data {
			if line == nil {
				continue
			}
			out.Lines = append(out.Lines, service.BillingStripeInvoicePreviewLine{
				Description: line.Description,
				AmountCents: line.Amount,
				Proration:   invoiceLineIsProration(line),
			})
		}
	}
	return out, nil
}

func (g *Gateway) UpdateSubscriptionPrice(ctx context.Context, input service.BillingSubscriptionChangeInput) error {
	_ = ctx
	if strings.TrimSpace(input.SubscriptionID) == "" {
		return fmt.Errorf("stripe subscription ID is required")
	}
	if strings.TrimSpace(input.PriceID) == "" {
		return fmt.Errorf("stripe price ID is required")
	}
	sub, err := subscription.Get(input.SubscriptionID, nil)
	if err != nil {
		return fmt.Errorf("retrieve stripe subscription: %w", err)
	}
	if err := releaseAttachedSubscriptionSchedule(sub); err != nil {
		return err
	}
	if sub.CancelAtPeriodEnd {
		if err := clearSubscriptionCancellation(input.SubscriptionID, input.WorkspaceID); err != nil {
			return err
		}
	}
	itemID := firstSubscriptionItemID(sub)
	if itemID == "" {
		return fmt.Errorf("stripe subscription has no subscription item")
	}
	params := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{{
			ID:    stripe.String(itemID),
			Price: stripe.String(input.PriceID),
		}},
		Metadata: map[string]string{
			"workspace_id":  input.WorkspaceID,
			"plan":          input.Plan,
			"interval":      input.Interval,
			"pending_plan":  "",
			"pending_until": "",
		},
		PaymentBehavior:   stripe.String("pending_if_incomplete"),
		ProrationBehavior: stripe.String("always_invoice"),
	}
	if input.ProrationDate > 0 {
		params.ProrationDate = stripe.Int64(input.ProrationDate)
	}
	if _, err := subscription.Update(input.SubscriptionID, params); err != nil {
		return fmt.Errorf("update stripe subscription price: %w", err)
	}
	return nil
}

func (g *Gateway) ScheduleSubscriptionPriceChange(ctx context.Context, input service.BillingSubscriptionChangeInput) error {
	_ = ctx
	if strings.TrimSpace(input.SubscriptionID) == "" {
		return fmt.Errorf("stripe subscription ID is required")
	}
	if strings.TrimSpace(input.PriceID) == "" {
		return fmt.Errorf("stripe price ID is required")
	}
	sub, err := subscription.Get(input.SubscriptionID, nil)
	if err != nil {
		return fmt.Errorf("retrieve stripe subscription: %w", err)
	}
	currentPriceID := strings.TrimSpace(input.CurrentPriceID)
	if currentPriceID == "" {
		currentPriceID = firstSubscriptionItemPriceID(sub)
	}
	if currentPriceID == "" {
		return fmt.Errorf("current stripe price ID is required")
	}
	scheduleID := ""
	if sub.Schedule != nil {
		scheduleID = sub.Schedule.ID
	}
	if scheduleID == "" {
		schedule, err := subscriptionschedule.New(&stripe.SubscriptionScheduleParams{
			FromSubscription: stripe.String(input.SubscriptionID),
		})
		if err != nil {
			return fmt.Errorf("create stripe subscription schedule: %w", err)
		}
		scheduleID = schedule.ID
	}

	currentStart := input.CurrentPeriodStart.Unix()
	currentEnd := input.CurrentPeriodEnd.Unix()
	if currentStart <= 0 || currentEnd <= 0 {
		return fmt.Errorf("current period start and end are required")
	}
	params := &stripe.SubscriptionScheduleParams{
		EndBehavior:       stripe.String("release"),
		ProrationBehavior: stripe.String("none"),
		Metadata: map[string]string{
			"workspace_id":     input.WorkspaceID,
			"pending_plan":     input.Plan,
			"pending_interval": input.Interval,
		},
		Phases: []*stripe.SubscriptionSchedulePhaseParams{
			{
				StartDate: stripe.Int64(currentStart),
				EndDate:   stripe.Int64(currentEnd),
				Items: []*stripe.SubscriptionSchedulePhaseItemParams{{
					Price:    stripe.String(currentPriceID),
					Quantity: stripe.Int64(1),
				}},
				Metadata: map[string]string{
					"workspace_id": input.WorkspaceID,
					"plan":         input.CurrentPlan,
					"interval":     input.CurrentInterval,
				},
				ProrationBehavior: stripe.String("none"),
			},
			{
				StartDate: stripe.Int64(currentEnd),
				Items: []*stripe.SubscriptionSchedulePhaseItemParams{{
					Price:    stripe.String(input.PriceID),
					Quantity: stripe.Int64(1),
				}},
				Metadata: map[string]string{
					"workspace_id": input.WorkspaceID,
					"plan":         input.Plan,
					"interval":     input.Interval,
				},
				ProrationBehavior: stripe.String("none"),
			},
		},
	}
	if _, err := subscriptionschedule.Update(scheduleID, params); err != nil {
		return fmt.Errorf("schedule stripe subscription price change: %w", err)
	}
	return nil
}

func (g *Gateway) CancelSubscriptionAtPeriodEnd(ctx context.Context, input service.BillingSubscriptionCancelInput) error {
	_ = ctx
	if strings.TrimSpace(input.SubscriptionID) == "" {
		return fmt.Errorf("stripe subscription ID is required")
	}
	sub, err := subscription.Get(input.SubscriptionID, nil)
	if err != nil {
		return fmt.Errorf("retrieve stripe subscription: %w", err)
	}
	if err := releaseAttachedSubscriptionSchedule(sub); err != nil {
		return err
	}
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
		Metadata: map[string]string{
			"workspace_id":  input.WorkspaceID,
			"pending_plan":  "free",
			"pending_until": "period_end",
		},
	}
	if _, err := subscription.Update(input.SubscriptionID, params); err != nil {
		return fmt.Errorf("schedule stripe subscription cancellation: %w", err)
	}
	return nil
}

func (g *Gateway) CancelSubscriptionImmediately(ctx context.Context, input service.BillingSubscriptionCancelInput) error {
	_ = ctx
	if strings.TrimSpace(input.SubscriptionID) == "" {
		return fmt.Errorf("stripe subscription ID is required")
	}
	sub, err := subscription.Get(input.SubscriptionID, nil)
	if err != nil {
		return fmt.Errorf("retrieve stripe subscription: %w", err)
	}
	if err := releaseAttachedSubscriptionSchedule(sub); err != nil {
		return err
	}
	params := &stripe.SubscriptionCancelParams{
		InvoiceNow: stripe.Bool(false),
		Prorate:    stripe.Bool(false),
	}
	if _, err := subscription.Cancel(input.SubscriptionID, params); err != nil {
		return fmt.Errorf("cancel stripe subscription: %w", err)
	}
	return nil
}

func (g *Gateway) ResumeSubscription(ctx context.Context, input service.BillingSubscriptionCancelInput) error {
	_ = ctx
	if strings.TrimSpace(input.SubscriptionID) == "" {
		return fmt.Errorf("stripe subscription ID is required")
	}
	sub, err := subscription.Get(input.SubscriptionID, nil)
	if err != nil {
		return fmt.Errorf("retrieve stripe subscription: %w", err)
	}
	if err := releaseAttachedSubscriptionSchedule(sub); err != nil {
		return err
	}
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(false),
		Metadata: map[string]string{
			"workspace_id":  input.WorkspaceID,
			"pending_plan":  "",
			"pending_until": "",
			"resume_source": "helpin_billing",
			"resume_action": "cancel_at_period_end_false",
		},
	}
	if _, err := subscription.Update(input.SubscriptionID, params); err != nil {
		return fmt.Errorf("resume stripe subscription: %w", err)
	}
	return nil
}

func clearSubscriptionCancellation(subscriptionID, workspaceID string) error {
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(false),
		Metadata: map[string]string{
			"workspace_id":  workspaceID,
			"pending_plan":  "",
			"pending_until": "",
		},
	}
	if _, err := subscription.Update(subscriptionID, params); err != nil {
		return fmt.Errorf("clear stripe subscription cancellation: %w", err)
	}
	return nil
}

func releaseAttachedSubscriptionSchedule(sub *stripe.Subscription) error {
	if sub == nil || sub.Schedule == nil || strings.TrimSpace(sub.Schedule.ID) == "" {
		return nil
	}
	if _, err := subscriptionschedule.Release(sub.Schedule.ID, &stripe.SubscriptionScheduleReleaseParams{
		PreserveCancelDate: stripe.Bool(false),
	}); err != nil {
		return fmt.Errorf("release stripe subscription schedule: %w", err)
	}
	return nil
}

func firstSubscriptionItemID(sub *stripe.Subscription) string {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 || sub.Items.Data[0] == nil {
		return ""
	}
	return sub.Items.Data[0].ID
}

func firstSubscriptionItemPriceID(sub *stripe.Subscription) string {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 || sub.Items.Data[0] == nil || sub.Items.Data[0].Price == nil {
		return ""
	}
	return sub.Items.Data[0].Price.ID
}

func invoiceLineIsProration(line *stripe.InvoiceLineItem) bool {
	if line == nil || line.Parent == nil {
		return false
	}
	if line.Parent.InvoiceItemDetails != nil && line.Parent.InvoiceItemDetails.Proration {
		return true
	}
	if line.Parent.SubscriptionItemDetails != nil && line.Parent.SubscriptionItemDetails.Proration {
		return true
	}
	return false
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
