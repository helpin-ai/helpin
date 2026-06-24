package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	stripe "github.com/stripe/stripe-go/v86"
)

func TestBillingLockAllowsBillingRoutes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/ws_123/billing", nil)

	if !billingLockAllowedRequest(req) {
		t.Fatal("expected billing routes to remain accessible when a workspace is locked")
	}
}

func TestBillingLockDoesNotAllowRegularWorkspaceRoutes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/ws_123/docs", nil)

	if billingLockAllowedRequest(req) {
		t.Fatal("expected regular workspace routes to be lockable")
	}
}

func TestParseStripeInvoiceEventUsesParentSubscriptionDetails(t *testing.T) {
	event := stripe.Event{
		ID:   "evt_invoice_parent",
		Type: "invoice.payment_failed",
		Data: &stripe.EventData{Raw: []byte(`{
			"id": "in_parent",
			"customer": "cus_123",
			"parent": {
				"subscription_details": {
					"subscription": "sub_parent"
				}
			}
		}`)},
	}

	parsed, err := parseStripeInvoiceEvent(event)
	if err != nil {
		t.Fatalf("parse event: %v", err)
	}
	if parsed.EventID != "evt_invoice_parent" || parsed.EventType != "invoice.payment_failed" {
		t.Fatalf("unexpected event identity: %#v", parsed)
	}
	if parsed.CustomerID != "cus_123" || parsed.SubscriptionID != "sub_parent" || parsed.InvoiceID != "in_parent" {
		t.Fatalf("unexpected parsed invoice event: %#v", parsed)
	}
}

func TestParseStripeInvoiceEventFallsBackToLegacySubscriptionField(t *testing.T) {
	event := stripe.Event{
		ID:   "evt_invoice_legacy",
		Type: "invoice.payment_succeeded",
		Data: &stripe.EventData{Raw: []byte(`{
			"id": "in_legacy",
			"customer": "cus_456",
			"subscription": "sub_legacy"
		}`)},
	}

	parsed, err := parseStripeInvoiceEvent(event)
	if err != nil {
		t.Fatalf("parse event: %v", err)
	}
	if parsed.EventID != "evt_invoice_legacy" || parsed.EventType != "invoice.payment_succeeded" {
		t.Fatalf("unexpected event identity: %#v", parsed)
	}
	if parsed.CustomerID != "cus_456" || parsed.SubscriptionID != "sub_legacy" || parsed.InvoiceID != "in_legacy" {
		t.Fatalf("unexpected parsed invoice event: %#v", parsed)
	}
}
