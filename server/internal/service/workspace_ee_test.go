//go:build ee

package service

import (
	"context"
	"errors"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
	"testing"
)

func TestWorkspaceService_DeleteCancelsActiveSubscriptionImmediately(t *testing.T) {
	db, svc := newWorkspaceTestHarness(t)
	createDeleteStubTables(t, db)
	ctx := context.Background()
	gateway := &fakeBillingGateway{}
	billingRepo := eerepository.NewBillingRepository(db)
	billingSvc := NewBillingService(billingRepo, gateway, nil)
	svc.SetBillingService(billingSvc)

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Paid Delete", Slug: "paid-delete", WorkspaceKey: "PDL"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := billingRepo.UpsertWorkspaceBilling(ctx, &model.WorkspaceBilling{
		WorkspaceID:          created.ID,
		Plan:                 model.BillingPlanStarter,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_paid"),
		StripeSubscriptionID: billingStringPtr("sub_paid"),
		BillingInterval:      "monthly",
		IncludedCredits:      5000,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(gateway.immediateCancels) != 1 || gateway.immediateCancels[0].SubscriptionID != "sub_paid" {
		t.Fatalf("unexpected immediate cancels: %#v", gateway.immediateCancels)
	}
	var count int64
	if err := db.Model(&model.WorkspaceBilling{}).Where("workspace_id = ?", created.ID).Count(&count).Error; err != nil {
		t.Fatalf("count workspace billing: %v", err)
	}
	if count != 0 {
		t.Fatalf("workspace billing rows after delete = %d, want 0", count)
	}
}

func TestWorkspaceService_DeleteStopsWhenSubscriptionCancellationFails(t *testing.T) {
	db, svc := newWorkspaceTestHarness(t)
	createDeleteStubTables(t, db)
	ctx := context.Background()
	gateway := &fakeBillingGateway{cancelErr: errors.New("stripe unavailable")}
	billingRepo := eerepository.NewBillingRepository(db)
	billingSvc := NewBillingService(billingRepo, gateway, nil)
	svc.SetBillingService(billingSvc)

	created, err := svc.Create(ctx, model.CreateWorkspaceRequest{Name: "Paid Delete Fail", Slug: "paid-delete-fail", WorkspaceKey: "PDF"}, "owner-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := billingRepo.UpsertWorkspaceBilling(ctx, &model.WorkspaceBilling{
		WorkspaceID:          created.ID,
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_paid"),
		StripeSubscriptionID: billingStringPtr("sub_paid"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	err = svc.Delete(ctx, created.ID)
	if err == nil {
		t.Fatal("expected delete to fail when subscription cancellation fails")
	}
	if _, getErr := svc.GetByID(ctx, created.ID); getErr != nil {
		t.Fatalf("workspace should remain after failed billing cancellation: %v", getErr)
	}
}
