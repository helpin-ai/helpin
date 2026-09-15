//go:build ee

package service

import (
	"context"
	"encoding/json"
	"errors"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"net/http"
	"testing"
)

func TestCustomerIOOutboxRefreshesWorkspaceBillingState(t *testing.T) {
	db := newBillingTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL, password_hash TEXT NOT NULL, full_name TEXT NOT NULL, totp_verified BOOLEAN NOT NULL DEFAULT 0, avatar_url TEXT, avatar_style TEXT, avatar_seed TEXT, avatar_background_mode TEXT, avatar_background_color TEXT, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE workspace_members (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT, email TEXT NOT NULL, display_name TEXT NOT NULL, role TEXT NOT NULL, status TEXT NOT NULL, created_at DATETIME, updated_at DATETIME)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}
	userID := "user-1"
	workspace := &model.Workspace{ID: "workspace-1", Name: "Acme", Slug: "acme", WorkspaceKey: "ACME", OwnerID: userID, Timezone: "UTC"}
	if err := db.Exec(`INSERT INTO workspaces (id,name,slug,owner_id) VALUES (?,?,?,?)`, workspace.ID, workspace.Name, workspace.Slug, userID).Error; err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := eerepository.NewBillingRepository(db).UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{ID: "billing-1", WorkspaceID: workspace.ID, Plan: model.BillingPlanGrowth, Status: model.BillingStatusPastDue, BillingInterval: "monthly", IncludedCredits: 25000}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}
	if err := db.Exec(`INSERT INTO users (id,email,password_hash,full_name) VALUES (?,?,?,?)`, userID, "owner@example.com", "hash", "Owner").Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Exec(`INSERT INTO workspace_members (id,workspace_id,user_id,email,display_name,role,status) VALUES (?,?,?,?,?,?,?)`, "member-1", workspace.ID, userID, "owner@example.com", "Owner", model.RoleOwner, model.WorkspaceMemberStatusActive).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	var got map[string]any
	httpClient := &http.Client{Transport: customerIORoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return customerIOTestResponse(http.StatusOK), nil
	})}
	identity := NewCustomerIOIdentityService(NewCustomerIOTrackClient(CustomerIOTrackConfig{SiteID: "site", APIKey: "key", Endpoint: "https://customer.test", HTTPClient: httpClient}), nil, repository.NewWorkspaceRepository(db), nil, NewCustomerIOBillingReader(eerepository.NewBillingRepository(db), repository.NewWorkspaceRepository(db)))

	found, err := identity.RefreshWorkspaceForOutbox(context.Background(), workspace.ID)
	if err != nil || !found {
		t.Fatalf("RefreshWorkspaceForOutbox = (%v,%v)", found, err)
	}
	attrs := got["attributes"].(map[string]any)
	if attrs["billing_status"] != model.BillingStatusPastDue || attrs["plan"] != model.BillingPlanGrowth {
		t.Fatalf("stale workspace attributes: %#v", attrs)
	}

	failingClient := &http.Client{Transport: customerIORoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return customerIOTestResponse(http.StatusServiceUnavailable), nil
	})}
	failingIdentity := NewCustomerIOIdentityService(NewCustomerIOTrackClient(CustomerIOTrackConfig{SiteID: "site", APIKey: "key", Endpoint: "https://customer.test", HTTPClient: failingClient}), nil, repository.NewWorkspaceRepository(db), nil, NewCustomerIOBillingReader(eerepository.NewBillingRepository(db), repository.NewWorkspaceRepository(db)))
	_, err = failingIdentity.RefreshWorkspaceForOutbox(context.Background(), workspace.ID)
	var deliveryErr *CustomerIODeliveryError
	if !errors.As(err, &deliveryErr) || deliveryErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("refresh error = %#v, want typed 503 delivery error", err)
	}
}
