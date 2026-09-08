package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestBillingHistoryIncludesLateAskChatAfterAutomaticRenewal(t *testing.T) {
	db := newBillingTestDB(t)
	if err := db.AutoMigrate(&model.AIUsagePeriod{}, &model.AIUsageReservation{}, &model.AIUsageLedgerEntry{}, &model.AIUsageSettlement{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	boundary := now.Add(-time.Hour)
	repo := repository.NewBillingRepository(db)
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{ID: "billing", WorkspaceID: "ws", Plan: model.BillingPlanFounder, Status: model.BillingStatusActive, BillingInterval: "monthly", CurrentPeriodStart: boundary, CurrentPeriodEnd: boundary.AddDate(0, 1, 0)}); err != nil {
		t.Fatal(err)
	}
	previous := model.AIUsagePeriod{ID: "old", WorkspaceID: "ws", PeriodStart: boundary.AddDate(0, -1, 0), PeriodEnd: boundary, AllowanceMicrousd: 1000, UsedMicrousd: 250, EnforcementMode: model.AIUsageEnforcementSoft, Status: "open", PricingVersion: "test"}
	if err := db.Create(&previous).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AIUsageLedgerEntry{ID: "entry", WorkspaceID: "ws", PeriodID: "old", EntryKind: "usage", FeatureKey: BillingFeatureAskChat, FinalChargedMicrousd: 250, CreatedAt: now, IdempotencyKey: "run:usage"}).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	history, err := svc.GetWorkspaceUsage(context.Background(), "ws", "", "daily", previous.PeriodStart.Format(time.RFC3339Nano), previous.PeriodEnd.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Features) != 1 || history.Features[0].FeatureKey != BillingFeatureAskChat || history.Features[0].ChargedMicrousd != 250 || history.Features[0].Pct != 25 {
		t.Fatalf("history=%+v", history)
	}
	if history.AIUsageUsedMicrousd != 250 || history.AIUsageAllowanceMicrousd != 1000 || len(history.Periods) != 2 {
		t.Fatalf("historical summary=%+v", history)
	}
	current, err := svc.GetWorkspaceBilling(context.Background(), "ws")
	if err != nil {
		t.Fatal(err)
	}
	if !current.AIUsagePeriodEnd.After(now) || current.AIUsageUsedMicrousd != 0 {
		t.Fatalf("renewed summary=%+v", current)
	}
}
