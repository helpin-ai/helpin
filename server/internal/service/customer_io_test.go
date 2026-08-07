package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
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

func TestCustomerIOWorkspaceEventAttributesIncludeSlug(t *testing.T) {
	got := customerIOWorkspaceEventAttributes(
		map[string]any{"source": "backend"},
		&model.Workspace{ID: "workspace-1", Slug: "acme-team"},
	)

	if got["workspace_id"] != "workspace-1" {
		t.Fatalf("workspace_id = %#v, want workspace-1", got["workspace_id"])
	}
	if got["workspace_slug"] != "acme-team" {
		t.Fatalf("workspace_slug = %#v, want acme-team", got["workspace_slug"])
	}
	if got["source"] != "backend" {
		t.Fatalf("existing attributes changed: %#v", got)
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
	if attrs["highest_plan"] != "growth" || attrs["estimated_monthly_due_cents"] != float64(10000) {
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
