package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCustomerIOTrackClientIdentifiesPersonWithTrackAPIEntityShape(t *testing.T) {
	var gotAuth string
	var gotPath string
	var got map[string]any
	httpClient := &http.Client{Transport: customerIORoundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return customerIOTestResponse(http.StatusOK), nil
	})}

	verifiedAt := time.Date(2026, 7, 9, 14, 15, 0, 0, time.UTC)
	user := &model.User{
		ID:                 "user-1",
		Email:              "owner@example.com",
		FullName:           "Owner User",
		EmailVerifiedAt:    &verifiedAt,
		DefaultWorkspaceID: billingStringPtr("workspace-1"),
		CreatedAt:          verifiedAt.Add(-time.Hour),
		UpdatedAt:          verifiedAt,
	}

	client := NewCustomerIOTrackClient(CustomerIOTrackConfig{
		SiteID:                "site-id",
		APIKey:                "api-key",
		WorkspaceObjectTypeID: "1",
		Endpoint:              "https://customer.test",
		HTTPClient:            httpClient,
	})

	if err := client.IdentifyUser(context.Background(), user); err != nil {
		t.Fatalf("IdentifyUser: %v", err)
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("site-id:api-key"))
	if gotAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", gotAuth, wantAuth)
	}
	if gotPath != "/api/v2/entity" {
		t.Fatalf("path = %q, want /api/v2/entity", gotPath)
	}
	if got["type"] != "person" || got["action"] != "identify" {
		t.Fatalf("unexpected envelope: %#v", got)
	}
	identifiers := got["identifiers"].(map[string]any)
	if identifiers["id"] != "user-1" {
		t.Fatalf("person id = %#v, want user-1", identifiers["id"])
	}
	attrs := got["attributes"].(map[string]any)
	if attrs["email"] != "owner@example.com" || attrs["full_name"] != "Owner User" {
		t.Fatalf("unexpected attrs: %#v", attrs)
	}
	if attrs["email_verified"] != true {
		t.Fatalf("email_verified = %#v, want true", attrs["email_verified"])
	}
	if attrs["default_workspace_id"] != "workspace-1" {
		t.Fatalf("default_workspace_id = %#v, want workspace-1", attrs["default_workspace_id"])
	}
}

func TestCustomerIOWorkspaceEventAttributesIncludeContract(t *testing.T) {
	got := customerIOWorkspaceEventAttributes(
		map[string]any{"source": "backend"},
		&model.Workspace{
			ID:             "workspace-1",
			Name:           "Acme Team",
			Slug:           "acme-team",
			OrganizationID: billingStringPtr("org-1"),
		},
	)

	for key, want := range map[string]any{
		"workspace_name":  "Acme Team",
		"workspace_id":    "workspace-1",
		"workspace_slug":  "acme-team",
		"organization_id": "org-1",
	} {
		if got[key] != want {
			t.Fatalf("%s = %#v, want %#v", key, got[key], want)
		}
	}
	if got["source"] != "backend" {
		t.Fatalf("existing attributes changed: %#v", got)
	}
}

func TestCustomerIOWorkspaceEventStripsReservedOverridesAtTrackBoundary(t *testing.T) {
	var got map[string]any
	httpClient := &http.Client{Transport: customerIORoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return customerIOTestResponse(http.StatusOK), nil
	})}
	client := NewCustomerIOTrackClient(CustomerIOTrackConfig{
		SiteID: "site-id", APIKey: "api-key", Endpoint: "https://customer.test", HTTPClient: httpClient,
	})

	err := client.TrackEvent(context.Background(), CustomerIOEvent{
		UserID: "user-1",
		Name:   "workspace_event",
		Attributes: map[string]any{
			"workspace_role":    "admin",
			"membership_status": model.WorkspaceMemberStatusActive,
			"recipient":         "attacker@example.com",
			"from_address":      "attacker@example.com",
			"reply_to":          "attacker@example.com",
		},
	})
	if err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}
	attrs := got["attributes"].(map[string]any)
	for _, key := range []string{"recipient", "from_address", "reply_to"} {
		if _, exists := attrs[key]; exists {
			t.Fatalf("reserved override %q reached Track v2 attributes: %#v", key, attrs)
		}
	}
	if attrs["workspace_role"] != "admin" || attrs["membership_status"] != model.WorkspaceMemberStatusActive {
		t.Fatalf("workspace membership contract changed: %#v", attrs)
	}
}

func TestCustomerIODeliveryErrorIncludesStatusAndRetryAfter(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		retryAfter string
		wantRetry  time.Duration
	}{
		{name: "bad request", status: http.StatusBadRequest},
		{name: "timeout", status: http.StatusRequestTimeout},
		{name: "rate limit delta", status: http.StatusTooManyRequests, retryAfter: "17", wantRetry: 17 * time.Second},
		{name: "server error", status: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewCustomerIOTrackClient(CustomerIOTrackConfig{
				SiteID: "site-id", APIKey: "api-key", Endpoint: "https://customer.test",
				HTTPClient: &http.Client{Transport: customerIORoundTripFunc(func(*http.Request) (*http.Response, error) {
					res := customerIOTestResponse(tt.status)
					res.Header.Set("Retry-After", tt.retryAfter)
					res.Body = io.NopCloser(strings.NewReader(`{"error":"delivery failed"}`))
					return res, nil
				})},
			})
			err := client.TrackEvent(context.Background(), CustomerIOEvent{UserID: "user-1", Name: "test"})
			var deliveryErr *CustomerIODeliveryError
			if !errors.As(err, &deliveryErr) {
				t.Fatalf("error = %T %v, want *CustomerIODeliveryError", err, err)
			}
			if deliveryErr.StatusCode != tt.status || deliveryErr.RetryAfter != tt.wantRetry {
				t.Fatalf("delivery error = %#v, want status=%d retry_after=%s", deliveryErr, tt.status, tt.wantRetry)
			}
		})
	}
}

func TestCustomerIODeliveryErrorParsesHTTPDateRetryAfter(t *testing.T) {
	retryAt := time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)
	client := NewCustomerIOTrackClient(CustomerIOTrackConfig{
		SiteID: "site-id", APIKey: "api-key", Endpoint: "https://customer.test",
		HTTPClient: &http.Client{Transport: customerIORoundTripFunc(func(*http.Request) (*http.Response, error) {
			res := customerIOTestResponse(http.StatusTooManyRequests)
			res.Header.Set("Retry-After", retryAt.Format(http.TimeFormat))
			return res, nil
		})},
	})
	err := client.TrackEvent(context.Background(), CustomerIOEvent{UserID: "user-1", Name: "test"})
	var deliveryErr *CustomerIODeliveryError
	if !errors.As(err, &deliveryErr) {
		t.Fatalf("error = %T %v, want *CustomerIODeliveryError", err, err)
	}
	if deliveryErr.RetryAfter < 28*time.Second || deliveryErr.RetryAfter > 31*time.Second {
		t.Fatalf("RetryAfter = %s, want approximately 30s", deliveryErr.RetryAfter)
	}
}

func TestCustomerIODeliveryErrorWrapsTransportErrors(t *testing.T) {
	transportErr := errors.New("network unavailable")
	client := NewCustomerIOTrackClient(CustomerIOTrackConfig{
		SiteID: "site-id", APIKey: "api-key", Endpoint: "https://customer.test",
		HTTPClient: &http.Client{Transport: customerIORoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, transportErr
		})},
	})
	err := client.TrackEvent(context.Background(), CustomerIOEvent{UserID: "user-1", Name: "test"})
	var deliveryErr *CustomerIODeliveryError
	if !errors.As(err, &deliveryErr) || !errors.Is(err, transportErr) {
		t.Fatalf("error = %T %v, want classifiable delivery and transport error", err, err)
	}
	if deliveryErr.StatusCode != 0 {
		t.Fatalf("StatusCode = %d, want 0 for transport error", deliveryErr.StatusCode)
	}
}

func TestCustomerIOULIDIsStablePerRecipientAndEmbedsOccurrenceTime(t *testing.T) {
	occurredAt := time.Date(2026, 8, 10, 12, 34, 56, 789000000, time.UTC)
	first := customerIOEventULID("018f6a31-1f4e-7d20-8d91-59a37c137a1a", "user-1", occurredAt)
	retry := customerIOEventULID("018f6a31-1f4e-7d20-8d91-59a37c137a1a", "user-1", occurredAt)
	otherRecipient := customerIOEventULID("018f6a31-1f4e-7d20-8d91-59a37c137a1a", "user-2", occurredAt)

	parsed, err := ulid.ParseStrict(first)
	if err != nil {
		t.Fatalf("parse generated ULID %q: %v", first, err)
	}
	if first != strings.ToUpper(first) {
		t.Fatalf("ULID = %q, want canonical uppercase Crockford Base32", first)
	}
	if parsed.Time() != ulid.Timestamp(occurredAt) {
		t.Fatalf("ULID timestamp = %d, want %d", parsed.Time(), ulid.Timestamp(occurredAt))
	}
	if retry != first {
		t.Fatalf("retry ULID = %q, want stable %q", retry, first)
	}
	if otherRecipient == first {
		t.Fatalf("different recipients received same ULID %q", first)
	}
}

func TestCustomerIOTrackClientTracksPersonEventWithWorkspaceContext(t *testing.T) {
	var got map[string]any
	httpClient := &http.Client{Transport: customerIORoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return customerIOTestResponse(http.StatusOK), nil
	})}

	client := NewCustomerIOTrackClient(CustomerIOTrackConfig{
		SiteID:     "site-id",
		APIKey:     "api-key",
		Region:     "us",
		Endpoint:   "https://customer.test",
		HTTPClient: httpClient,
	})

	eventAt := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	err := client.TrackEvent(context.Background(), CustomerIOEvent{
		UserID:     "user-1",
		EventID:    "event-1",
		Name:       "module_first_value",
		OccurredAt: eventAt,
		Attributes: map[string]any{
			"workspace_id":    "workspace-1",
			"organization_id": "org-1",
			"module":          "support",
			"milestone":       "first_reply_sent",
		},
	})
	if err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}

	if got["type"] != "person" || got["action"] != "event" {
		t.Fatalf("unexpected envelope: %#v", got)
	}
	identifiers := got["identifiers"].(map[string]any)
	if identifiers["id"] != "user-1" {
		t.Fatalf("person id = %#v, want user-1", identifiers["id"])
	}
	if got["name"] != "module_first_value" || got["id"] != "event-1" {
		t.Fatalf("unexpected event identity: %#v", got)
	}
	if got["timestamp"] != float64(eventAt.Unix()) {
		t.Fatalf("timestamp = %#v, want %d", got["timestamp"], eventAt.Unix())
	}
	attrs := got["attributes"].(map[string]any)
	if attrs["workspace_id"] != "workspace-1" || attrs["module"] != "support" {
		t.Fatalf("unexpected event attributes: %#v", attrs)
	}
}

func TestCustomerIOTrackClientDeletesPersonRelationship(t *testing.T) {
	var got map[string]any
	httpClient := &http.Client{Transport: customerIORoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return customerIOTestResponse(http.StatusOK), nil
	})}
	client := NewCustomerIOTrackClient(CustomerIOTrackConfig{SiteID: "site-id", APIKey: "api-key", Endpoint: "https://customer.test", HTTPClient: httpClient})
	if err := client.DeletePersonRelationship(context.Background(), "user-1", "1", "workspace-1"); err != nil {
		t.Fatalf("DeletePersonRelationship: %v", err)
	}
	if got["type"] != "person" || got["action"] != "delete_relationships" {
		t.Fatalf("unexpected envelope: %#v", got)
	}
	identifiers := got["identifiers"].(map[string]any)
	if identifiers["id"] != "user-1" {
		t.Fatalf("person id = %#v, want user-1", identifiers["id"])
	}
	relationships := got["cio_relationships"].([]any)
	relationship := relationships[0].(map[string]any)
	object := relationship["identifiers"].(map[string]any)
	if object["object_type_id"] != "1" || object["object_id"] != "workspace-1" {
		t.Fatalf("unexpected relationship identifiers: %#v", object)
	}
}

func TestCustomerIOTrackClientIdentifiesWorkspaceObjectWithRelationship(t *testing.T) {
	var got map[string]any
	httpClient := &http.Client{Transport: customerIORoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return customerIOTestResponse(http.StatusOK), nil
	})}

	now := time.Now().UTC()
	workspace := &model.Workspace{
		ID:             "workspace-1",
		Name:           "Acme",
		Slug:           "acme",
		WorkspaceKey:   "ACM",
		OwnerID:        "user-1",
		OrganizationID: billingStringPtr("org-1"),
		WebsiteURL:     billingStringPtr("https://acme.com"),
		CreatedAt:      now.Add(-time.Hour),
		UpdatedAt:      now,
	}
	trialEnds := now.AddDate(0, 0, 14)

	client := NewCustomerIOTrackClient(CustomerIOTrackConfig{
		SiteID:                "site-id",
		APIKey:                "api-key",
		WorkspaceObjectTypeID: "1",
		Endpoint:              "https://customer.test",
		HTTPClient:            httpClient,
	})

	err := client.IdentifyWorkspace(context.Background(), CustomerIOWorkspaceIdentity{
		Workspace:          workspace,
		Billing:            &BillingSummary{WorkspaceID: "workspace-1", Plan: "growth", Status: "trialing", BillingInterval: "monthly", Trialing: true, TrialEndsAt: &trialEnds, CreditsUsed: 10, CreditsRemaining: 24990, Locked: false, SeatLimit: 10, SeatUsage: 1},
		RelationshipUserID: "user-1",
		RelationshipRole:   "owner",
	})
	if err != nil {
		t.Fatalf("IdentifyWorkspace: %v", err)
	}

	if got["type"] != "object" || got["action"] != "identify" {
		t.Fatalf("unexpected envelope: %#v", got)
	}
	identifiers := got["identifiers"].(map[string]any)
	if identifiers["object_type_id"] != "1" || identifiers["object_id"] != "workspace-1" {
		t.Fatalf("unexpected object identifiers: %#v", identifiers)
	}
	attrs := got["attributes"].(map[string]any)
	if attrs["name"] != "Acme" || attrs["workspace_slug"] != "acme" || attrs["plan"] != "growth" {
		t.Fatalf("unexpected attrs: %#v", attrs)
	}
	if attrs["trialing"] != true || attrs["trial_days_left"].(float64) < 13 {
		t.Fatalf("unexpected trial attrs: %#v", attrs)
	}
	relationships := got["cio_relationships"].([]any)
	relationship := relationships[0].(map[string]any)
	relationshipIdentifiers := relationship["identifiers"].(map[string]any)
	if relationshipIdentifiers["id"] != "user-1" {
		t.Fatalf("relationship id = %#v, want user-1", relationshipIdentifiers["id"])
	}
	relationshipAttrs := relationship["relationship_attributes"].(map[string]any)
	if relationshipAttrs["workspace_role"] != "owner" {
		t.Fatalf("relationship attrs = %#v, want role owner", relationshipAttrs)
	}
}

func TestCustomerIOTrackClientIdentifiesOrganizationObjectWithRelationship(t *testing.T) {
	var got map[string]any
	httpClient := &http.Client{Transport: customerIORoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return customerIOTestResponse(http.StatusOK), nil
	})}

	now := time.Date(2026, 7, 9, 14, 15, 0, 0, time.UTC)
	org := &model.Organization{
		ID:        "org-1",
		Name:      "Acme",
		Slug:      "acme",
		OwnerID:   "user-1",
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}

	client := NewCustomerIOTrackClient(CustomerIOTrackConfig{
		SiteID:                   "site-id",
		APIKey:                   "api-key",
		OrganizationObjectTypeID: "2",
		Endpoint:                 "https://customer.test",
		HTTPClient:               httpClient,
	})

	err := client.IdentifyOrganization(context.Background(), CustomerIOOrganizationIdentity{
		Organization:       org,
		RelationshipUserID: "user-1",
		RelationshipRole:   "owner",
		MemberCount:        3,
		WorkspaceCount:     2,
		HighestPlan:        "growth",
		HasTrialWorkspace:  true,
		HasPaidWorkspace:   true,
		PaidWorkspaceCount: 1,
		MonthlyDueCents:    10000,
	})
	if err != nil {
		t.Fatalf("IdentifyOrganization: %v", err)
	}

	if got["type"] != "object" || got["action"] != "identify" {
		t.Fatalf("unexpected envelope: %#v", got)
	}
	identifiers := got["identifiers"].(map[string]any)
	if identifiers["object_type_id"] != "2" || identifiers["object_id"] != "org-1" {
		t.Fatalf("unexpected object identifiers: %#v", identifiers)
	}
	attrs := got["attributes"].(map[string]any)
	if attrs["name"] != "Acme" || attrs["organization_slug"] != "acme" {
		t.Fatalf("unexpected attrs: %#v", attrs)
	}
	if attrs["member_count"] != float64(3) || attrs["workspace_count"] != float64(2) {
		t.Fatalf("unexpected count attrs: %#v", attrs)
	}
	if attrs["highest_plan"] != "growth" || attrs["paid_workspace_count"] != float64(1) || attrs["monthly_due_cents"] != float64(10000) {
		t.Fatalf("unexpected commercial attrs: %#v", attrs)
	}
	relationships := got["cio_relationships"].([]any)
	relationship := relationships[0].(map[string]any)
	relationshipIdentifiers := relationship["identifiers"].(map[string]any)
	if relationshipIdentifiers["id"] != "user-1" {
		t.Fatalf("relationship id = %#v, want user-1", relationshipIdentifiers["id"])
	}
	relationshipAttrs := relationship["relationship_attributes"].(map[string]any)
	if relationshipAttrs["organization_role"] != "owner" {
		t.Fatalf("relationship attrs = %#v, want role owner", relationshipAttrs)
	}
}

func TestCustomerIOOrganizationSummaryCountsPaidActiveWorkspaces(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE workspace_billing (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL UNIQUE, plan TEXT NOT NULL,
		status TEXT NOT NULL, stripe_customer_id TEXT, stripe_subscription_id TEXT,
		stripe_price_id TEXT, billing_interval TEXT NOT NULL, included_credits INTEGER NOT NULL,
		credits_used INTEGER NOT NULL, on_demand_enabled BOOLEAN NOT NULL,
		on_demand_blocks_invoiced INTEGER NOT NULL, current_period_start DATETIME NOT NULL,
		current_period_end DATETIME NOT NULL, trial_ends_at DATETIME, pending_plan TEXT,
		pending_billing_interval TEXT, pending_change_at DATETIME,
		cancel_at_period_end BOOLEAN NOT NULL, canceled_at DATETIME, billing_notice_type TEXT,
		billing_notice_message TEXT, billing_notice_at DATETIME, payment_failed_at DATETIME,
		trial_will_end_at DATETIME, last_stripe_event_id TEXT, payment_method_id TEXT,
		billing_owner_user_id TEXT, created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create workspace billing table: %v", err)
	}
	orgID := "org-customer-io"
	for _, workspace := range []model.Workspace{
		{ID: "ws-paid", Name: "Paid", Slug: "paid", OwnerID: "user-1", OrganizationID: &orgID},
		{ID: "ws-founder", Name: "Founder", Slug: "founder", OwnerID: "user-1", OrganizationID: &orgID},
		{ID: "ws-trial", Name: "Trial", Slug: "trial", OwnerID: "user-1", OrganizationID: &orgID},
	} {
		if err := db.Create(&workspace).Error; err != nil {
			t.Fatalf("create workspace %s: %v", workspace.ID, err)
		}
	}
	subscriptionID := "sub_paid"
	for _, billing := range []model.WorkspaceBilling{
		{ID: "billing-paid", WorkspaceID: "ws-paid", Plan: model.BillingPlanGrowth, Status: model.BillingStatusActive, BillingInterval: "monthly", StripeSubscriptionID: &subscriptionID},
		{ID: "billing-founder", WorkspaceID: "ws-founder", Plan: model.BillingPlanFounder, Status: model.BillingStatusActive, BillingInterval: "monthly"},
		{ID: "billing-trial", WorkspaceID: "ws-trial", Plan: model.BillingPlanGrowth, Status: model.BillingStatusTrialing, BillingInterval: "monthly"},
	} {
		if err := db.Create(&billing).Error; err != nil {
			t.Fatalf("create billing %s: %v", billing.ID, err)
		}
	}

	svc := &CustomerIOIdentityService{
		billingRepo: repository.NewBillingRepository(db),
		logger:      slog.Default(),
	}
	summary := svc.organizationSummary(context.Background(), orgID)
	if summary.PaidWorkspaceCount != 2 {
		t.Fatalf("PaidWorkspaceCount = %d, want 2 paid active workspaces", summary.PaidWorkspaceCount)
	}
}

type customerIORoundTripFunc func(*http.Request) (*http.Response, error)

func (f customerIORoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func customerIOTestResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Header:     make(http.Header),
	}
}
