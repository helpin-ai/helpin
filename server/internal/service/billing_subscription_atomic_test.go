package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSubscriptionWebhookMutationAndReceiptAreAtomic(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER injected_failure BEFORE UPDATE ON workspace_billing BEGIN SELECT RAISE(ABORT, 'injected write failure'); END`,
		`CREATE TRIGGER injected_failure BEFORE UPDATE ON stripe_webhook_events WHEN NEW.processed = 1 BEGIN SELECT RAISE(ABORT, 'injected receipt failure'); END`,
	} {
		t.Run(trigger, func(t *testing.T) {
			db := newBillingTestDB(t)
			repo := repository.NewBillingRepository(db)
			ctx := context.Background()
			now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
			s := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
			if err := repo.UpsertWorkspaceBilling(ctx, &model.WorkspaceBilling{WorkspaceID: "workspace-1", Plan: model.BillingPlanStarter, Status: model.BillingStatusActive, BillingInterval: "monthly", CurrentPeriodStart: now, CurrentPeriodEnd: now.AddDate(0, 1, 0)}); err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(trigger).Error; err != nil {
				t.Fatal(err)
			}
			update := BillingStripeSubscriptionUpdate{EventID: "evt_atomic", EventType: "customer.subscription.updated", WorkspaceID: "workspace-1", Plan: model.BillingPlanGrowth, BillingInterval: "monthly", Status: model.BillingStatusActive, CurrentPeriodStart: now, CurrentPeriodEnd: now.AddDate(0, 1, 0)}
			if _, err := s.ApplyStripeSubscriptionUpdate(ctx, update); err == nil {
				t.Fatal("injected transaction failure was ignored")
			}
			billing, err := repo.GetByWorkspaceID(ctx, "workspace-1")
			if err != nil || billing.Plan != model.BillingPlanStarter {
				t.Fatalf("failed transaction changed subscription: %+v %v", billing, err)
			}
			var count int64
			if err := db.Model(&model.StripeWebhookEvent{}).Where("id = ?", update.EventID).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("failed transaction consumed event: %d %v", count, err)
			}
			if err := db.Exec(`DROP TRIGGER injected_failure`).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := s.ApplyStripeSubscriptionUpdate(ctx, update); err != nil {
				t.Fatal(err)
			}
			billing, err = repo.GetByWorkspaceID(ctx, "workspace-1")
			if err != nil || billing.Plan != model.BillingPlanGrowth {
				t.Fatal("retry lost subscription update")
			}
			update.Plan = model.BillingPlanStarter
			if _, err := s.ApplyStripeSubscriptionUpdate(ctx, update); err != nil {
				t.Fatal(err)
			}
			billing, err = repo.GetByWorkspaceID(ctx, "workspace-1")
			if err != nil || billing.Plan != model.BillingPlanGrowth {
				t.Fatal("processed event was applied twice")
			}
		})
	}
}

func TestSubscriptionWebhookRetriesPreviouslyUnprocessedEvent(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	s := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	if err := db.Create(&model.StripeWebhookEvent{ID: "evt_unprocessed", Type: "customer.subscription.updated"}).Error; err != nil {
		t.Fatal(err)
	}
	summary, err := s.ApplyStripeSubscriptionUpdate(ctx, BillingStripeSubscriptionUpdate{EventID: "evt_unprocessed", EventType: "customer.subscription.updated", WorkspaceID: "workspace-1", Plan: model.BillingPlanGrowth, BillingInterval: "monthly", Status: model.BillingStatusActive, CurrentPeriodStart: now, CurrentPeriodEnd: now.AddDate(0, 1, 0)})
	if err != nil || summary == nil || summary.Plan != model.BillingPlanGrowth {
		t.Fatalf("unprocessed event was skipped: %+v %v", summary, err)
	}
}
