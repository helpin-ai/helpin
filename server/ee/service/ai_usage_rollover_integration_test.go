//go:build ee

package service

import (
	"context"
	"testing"
	"time"

	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBillingUsageWindowAdvancesDespiteOutstandingReservation(t *testing.T) {
	db := newBillingTestDB(t)
	if err := db.AutoMigrate(&model.AIUsagePeriod{}, &model.AIUsageReservation{}, &model.AIUsageLedgerEntry{}, &model.AIUsageSettlement{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX one_open_period ON billing_ai_usage_periods(workspace_id) WHERE status = 'open'").Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	boundary := time.Date(2026, 9, 1, 9, 59, 46, 116239000, time.UTC)
	repo := eerepository.NewBillingRepository(db)
	if err := repo.UpsertWorkspaceBilling(ctx, &model.WorkspaceBilling{
		ID: "billing", WorkspaceID: "ws", Plan: model.BillingPlanFounder, Status: model.BillingStatusActive,
		BillingInterval: "monthly", CurrentPeriodStart: boundary, CurrentPeriodEnd: boundary.AddDate(0, 1, 0),
	}); err != nil {
		t.Fatal(err)
	}
	period := model.AIUsagePeriod{ID: "old", WorkspaceID: "ws", PeriodStart: boundary.AddDate(0, -1, 0), PeriodEnd: boundary.Add(-35404 * time.Microsecond),
		AllowanceMicrousd: 150_000_000, EnforcementMode: model.AIUsageEnforcementSoft, Status: model.AIUsagePeriodOpen, PricingVersion: "2026-08-13"}
	if err := db.Create(&period).Error; err != nil {
		t.Fatal(err)
	}
	usage := eerepository.NewAIUsageRepository(db)
	hold, err := usage.Reserve(ctx, eerepository.AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "unfinished-run", ReservedMicrousd: 18_888_550})
	if err != nil {
		t.Fatal(err)
	}
	billing := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	worker := NewAIUsagePeriodWorker(usage, billing.NextAIUsagePeriodSchedule)
	// The live billing/AI anniversary anchors differ by milliseconds. Catch up
	// through that short period, just as successive background sweeps would.
	for i := 0; i < 3; i++ {
		if _, err := worker.CloseDuePeriods(ctx, now, 100); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := billing.GetWorkspaceBilling(ctx, "ws")
	if err != nil {
		t.Fatal(err)
	}
	if !summary.AIUsagePeriodStart.Equal(boundary) || !summary.AIUsagePeriodEnd.Equal(boundary.AddDate(0, 1, 0)) {
		t.Fatalf("chart still receives stale window: %s — %s", summary.AIUsagePeriodStart, summary.AIUsagePeriodEnd)
	}
	// Usage was still recorded on September 16 while the old window was stuck.
	if err := db.Create(&model.AIUsageLedgerEntry{ID: "september-usage", WorkspaceID: "ws", PeriodID: period.ID,
		EntryKind: "usage", FeatureKey: "ask_chat", FinalChargedMicrousd: 250_000, CreatedAt: now, IdempotencyKey: "september-usage"}).Error; err != nil {
		t.Fatal(err)
	}
	chart, err := billing.GetWorkspaceUsage(ctx, "ws", "2026-09", "daily",
		summary.AIUsagePeriodStart.Format(time.RFC3339Nano), summary.AIUsagePeriodEnd.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if len(chart.Series) != 1 || chart.Series[0].Date != "2026-09-16" || chart.Series[0].Features["ask_chat"] != 250_000 {
		t.Fatalf("September 16 usage is missing from the chart: %+v", chart.Series)
	}
	// Completing the old run must neither charge the new allowance nor reopen
	// a historical period when the settlement sweep runs again.
	if _, err := usage.Reconcile(ctx, eerepository.AIUsageReconcileRequest{ReservationID: hold.ID, ChargedMicrousd: 100_000, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "finished-run"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.CloseDuePeriods(ctx, now, 100); err != nil {
		t.Fatal(err)
	}
	var open []model.AIUsagePeriod
	if err := db.Where("workspace_id = ? AND status = ?", "ws", model.AIUsagePeriodOpen).Find(&open).Error; err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || !open[0].PeriodStart.Equal(boundary) || open[0].UsedMicrousd != 0 {
		t.Fatalf("incorrect current allowance: %+v", open)
	}
	due, err := usage.ListDuePeriods(ctx, now, 100)
	if err != nil || len(due) != 0 {
		t.Fatalf("completed periods keep retrying: %+v %v", due, err)
	}
}
