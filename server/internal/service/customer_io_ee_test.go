//go:build ee

package service

import (
	"context"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
	"log/slog"
	"testing"
)

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
		billingInsights: NewCustomerIOBillingReader(eerepository.NewBillingRepository(db), nil),
		logger:          slog.Default(),
	}
	summary := svc.organizationSummary(context.Background(), orgID)
	if summary.PaidWorkspaceCount != 2 {
		t.Fatalf("PaidWorkspaceCount = %d, want 2 paid active workspaces", summary.PaidWorkspaceCount)
	}
}
