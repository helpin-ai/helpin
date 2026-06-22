package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func seedEntitlementBilling(t *testing.T, db *gorm.DB, workspaceID, plan, status string) *EntitlementService {
	t.Helper()

	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	billingRepo := repository.NewBillingRepository(db)
	if err := billingRepo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:        workspaceID,
		Plan:               plan,
		Status:             status,
		BillingInterval:    "monthly",
		IncludedCredits:    5000,
		CurrentPeriodStart: now.Add(-24 * time.Hour),
		CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}
	return NewEntitlementService(NewBillingService(billingRepo, &fakeBillingGateway{}, func() time.Time { return now }))
}

func createEntitlementBillingTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, stmt := range []string{
		`CREATE TABLE workspace_billing (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL UNIQUE,
			plan TEXT NOT NULL DEFAULT 'growth',
			status TEXT NOT NULL DEFAULT 'trialing',
			stripe_customer_id TEXT,
			stripe_subscription_id TEXT,
			stripe_price_id TEXT,
			billing_interval TEXT NOT NULL DEFAULT 'monthly',
			included_credits INTEGER NOT NULL DEFAULT 25000,
			credits_used INTEGER NOT NULL DEFAULT 0,
			on_demand_enabled BOOLEAN NOT NULL DEFAULT 0,
			on_demand_blocks_invoiced INTEGER NOT NULL DEFAULT 0,
			current_period_start DATETIME NOT NULL,
			current_period_end DATETIME NOT NULL,
			trial_ends_at DATETIME,
			pending_plan TEXT,
			pending_billing_interval TEXT,
			pending_change_at DATETIME,
			cancel_at_period_end BOOLEAN NOT NULL DEFAULT 0,
			canceled_at DATETIME,
			billing_notice_type TEXT,
			billing_notice_message TEXT,
			billing_notice_at DATETIME,
			payment_failed_at DATETIME,
			trial_will_end_at DATETIME,
			last_stripe_event_id TEXT,
			payment_method_id TEXT,
			billing_owner_user_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE billing_credit_ledger (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			feature_key TEXT NOT NULL DEFAULT '',
			credits INTEGER NOT NULL,
			idempotency_key TEXT NOT NULL UNIQUE,
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create entitlement billing table: %v", err)
		}
	}
}

func TestEntitlementServiceRequiresGrowthForGrowthOnlyFeatures(t *testing.T) {
	db := newTestDB(t)
	createEntitlementBillingTables(t, db)
	entitlements := seedEntitlementBilling(t, db, "ws-entitlements", model.BillingPlanStarter, model.BillingStatusActive)

	err := entitlements.RequireFeature(context.Background(), "ws-entitlements", EntitlementFeatureCustomAgents)
	if err == nil || !strings.Contains(err.Error(), "Growth") {
		t.Fatalf("RequireFeature error = %v, want Growth upgrade error", err)
	}
}

func TestSettingsServiceCreateTeamRejectsStarterTeamLimit(t *testing.T) {
	db := newTestDB(t)
	createEntitlementBillingTables(t, db)
	entitlements := seedEntitlementBilling(t, db, "ws-teams", model.BillingPlanStarter, model.BillingStatusActive)

	for i := 0; i < 10; i++ {
		if err := db.Exec(`INSERT INTO workspace_teams (id, workspace_id, name, handle, team_type, default_task_type, created_at, updated_at) VALUES (?, ?, ?, ?, 'engineering', 'feature', ?, ?)`,
			fmt.Sprintf("team-%d", i), "ws-teams", fmt.Sprintf("Team %d", i), fmt.Sprintf("team-%d", i), time.Now(), time.Now()).Error; err != nil {
			t.Fatalf("seed team: %v", err)
		}
	}

	svc := NewSettingsService(repository.NewSettingsRepository(db), nil, nil, nil).SetEntitlementService(entitlements)
	_, err := svc.CreateTeam(context.Background(), model.CreateTeamRequest{
		WorkspaceID: "ws-teams",
		Name:        "Blocked team",
	}, "")
	if err == nil || !strings.Contains(err.Error(), "10 teams") {
		t.Fatalf("CreateTeam error = %v, want Starter team limit error", err)
	}
}

func TestCRMContactServiceCreateRejectsStarterContactLimit(t *testing.T) {
	db := newTestDB(t)
	createEntitlementBillingTables(t, db)
	entitlements := seedEntitlementBilling(t, db, "ws-contacts", model.BillingPlanStarter, model.BillingStatusActive)

	for i := 0; i < 5000; i++ {
		if err := db.Exec(`INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, lifecycle_stage, lead_status, custom_properties, created_at, updated_at) VALUES (?, ?, ?, ?, 'subscriber', 'new', '{}', ?, ?)`,
			fmt.Sprintf("contact-%d", i), "ws-contacts", fmt.Sprintf("CON-%d", i+1), fmt.Sprintf("Contact %d", i), time.Now(), time.Now()).Error; err != nil {
			t.Fatalf("seed contact: %v", err)
		}
	}

	svc := NewCRMContactService(repository.NewCRMContactRepository(db)).SetEntitlementService(entitlements)
	_, err := svc.Create(context.Background(), model.CreateCRMContactRequest{
		WorkspaceID: "ws-contacts",
		FirstName:   "Blocked",
	})
	if err == nil || !strings.Contains(err.Error(), "5,000 contacts") {
		t.Fatalf("Create contact error = %v, want Starter contact limit error", err)
	}
}

func TestAgentServiceCreateCustomAgentRequiresGrowth(t *testing.T) {
	db := newTestDB(t)
	createEntitlementBillingTables(t, db)
	entitlements := seedEntitlementBilling(t, db, "ws-agents", model.BillingPlanStarter, model.BillingStatusActive)

	svc := (&AgentService{}).SetEntitlementService(entitlements)
	_, err := svc.CreateAgent(context.Background(), model.CreateAgentRequest{
		WorkspaceID: "ws-agents",
		Name:        "Custom agent",
	}, "user-1")
	if err == nil || !strings.Contains(err.Error(), "Growth") {
		t.Fatalf("CreateAgent error = %v, want Growth upgrade error", err)
	}
}
