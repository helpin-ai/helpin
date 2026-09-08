package repository

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIUsageRolloverTransfersHoldsAndPreservesLedger(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementSoft, 150_000_000)
	testAIUsageRolloverTransfersHolds(t, repo)
}

func testAIUsageRolloverTransfersHolds(t *testing.T, repo *AIUsageRepository) {
	t.Helper()
	ctx := context.Background()
	reservation, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run", ReservedMicrousd: 3_000_000})
	if err != nil {
		t.Fatal(err)
	}
	input := AIUsageCheckpointRequest{ReservationID: reservation.ID, ChargedMicrousd: 2_000_843, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "turn:1", EntryKind: "usage", FeatureKey: "ask_chat", InputTokensTotal: 48_000_000, CacheReadTokens: 40_000_000, OutputTokens: 100_000}}
	if _, err := repo.Checkpoint(ctx, input); err != nil {
		t.Fatal(err)
	}
	boundary := time.Now().UTC().Add(-time.Minute)
	if err := repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Update("period_end", boundary).Error; err != nil {
		t.Fatal(err)
	}
	schedule := AIUsagePeriodSchedule{WorkspaceID: "ws", Start: boundary, End: boundary.AddDate(0, 1, 0), AllowanceMicrousd: 150_000_000, EnforcementMode: model.AIUsageEnforcementSoft, PricingVersion: "test"}
	for i := 0; i < 2; i++ {
		if err := repo.RolloverPeriod(ctx, "period", schedule); err != nil {
			t.Fatal(err)
		}
	}
	var current model.AIUsagePeriod
	if err := repo.db.Where("workspace_id = ? AND status = ?", "ws", "open").First(&current).Error; err != nil {
		t.Fatal(err)
	}
	if current.UsedMicrousd != 0 || current.ReservedMicrousd != 999_157 {
		t.Fatalf("current=%+v", current)
	}
	// Replayed usage remains in the old period even after the hold has moved.
	if _, err := repo.Checkpoint(ctx, input); err != nil {
		t.Fatal(err)
	}
	terminal := AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 100, AllowLateUsage: true, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "terminal", EntryKind: "usage", FeatureKey: "ask_chat"}}
	for i := 0; i < 2; i++ {
		if _, err := repo.Reconcile(ctx, terminal); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.db.First(&current, "id = ?", current.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.UsedMicrousd != 100 || current.ReservedMicrousd != 0 {
		t.Fatalf("settled current=%+v", current)
	}
	var previous model.AIUsagePeriod
	if err := repo.db.First(&previous, "id = ?", "period").Error; err != nil {
		t.Fatal(err)
	}
	if previous.Status != "closed" || previous.UsedMicrousd != 2_000_843 || previous.ReservedMicrousd != 0 {
		t.Fatalf("previous=%+v", previous)
	}
	rows, err := NewBillingRepository(repo.db).UsageByDayFeature(ctx, "ws", previous.PeriodStart, previous.PeriodEnd)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ChargedMicrousd != 2_000_843 {
		t.Fatalf("historical usage outside date window missing: %+v", rows)
	}
}

func TestAIUsageRolloverFailureRollsBackCloseAndHolds(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementSoft, 1000)
	reservation, err := repo.Reserve(context.Background(), AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run", ReservedMicrousd: 100})
	if err != nil {
		t.Fatal(err)
	}
	boundary := time.Now().UTC().Add(-time.Minute)
	if err := repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Update("period_end", boundary).Error; err != nil {
		t.Fatal(err)
	}
	err = repo.RolloverPeriod(context.Background(), "period", AIUsagePeriodSchedule{WorkspaceID: "ws", Start: boundary, End: boundary})
	if err == nil {
		t.Fatal("expected invalid schedule failure")
	}
	var period model.AIUsagePeriod
	if err := repo.db.First(&period, "id = ?", reservation.PeriodID).Error; err != nil {
		t.Fatal(err)
	}
	if period.Status != "open" || period.ReservedMicrousd != 100 {
		t.Fatalf("partial rollover committed: %+v", period)
	}
}

func TestAIUsageExpiredPeriodRejectsNewReservation(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementSoft, 1000)
	if err := repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Update("period_end", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Reserve(context.Background(), AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "new", ReservedMicrousd: 10}); err != ErrAIUsagePeriodExpired {
		t.Fatalf("error=%v", err)
	}
}

func TestAIUsagePausedHoldResumesInRenewedPeriod(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1000)
	ctx := context.Background()
	reservation, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run", ReservedMicrousd: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ResizeReservation(ctx, reservation.ID, 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	boundary := time.Now().Add(-time.Minute)
	if err := repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Update("period_end", boundary).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.RolloverPeriod(ctx, "period", AIUsagePeriodSchedule{WorkspaceID: "ws", Start: boundary, End: boundary.AddDate(0, 1, 0), AllowanceMicrousd: 50, EnforcementMode: model.AIUsageEnforcementStrict, PricingVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ResizeReservation(ctx, reservation.ID, 100, time.Now()); err != model.ErrAIUsageExhausted {
		t.Fatalf("resume bypassed new allowance: %v", err)
	}
	if err := repo.ResizeReservation(ctx, reservation.ID, 40, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestAIUsageLateSettlementDoesNotChangeClosedInvoice(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementExtra, 100)
	if err := repo.db.Exec("CREATE TABLE workspace_billing (workspace_id text, stripe_customer_id text, stripe_subscription_id text)").Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	reservation, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run", ReservedMicrousd: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Reconcile(ctx, AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 150, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "terminal"}}); err != nil {
		t.Fatal(err)
	}
	boundary := time.Now().Add(-time.Minute)
	if err := repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Update("period_end", boundary).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.RolloverPeriod(ctx, "period", AIUsagePeriodSchedule{WorkspaceID: "ws", Start: boundary, End: boundary.AddDate(0, 1, 0), AllowanceMicrousd: 100, EnforcementMode: model.AIUsageEnforcementExtra, PricingVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	current, err := repo.Reconcile(ctx, AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 15, AllowLateUsage: true, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "late"}})
	if err != nil {
		t.Fatal(err)
	}
	if current.ID == "period" || current.UsedMicrousd != 15 {
		t.Fatalf("late usage not in current period: %+v", current)
	}
	var settlement model.AIUsageSettlement
	if err := repo.db.Where("period_id = ?", "period").First(&settlement).Error; err != nil {
		t.Fatal(err)
	}
	if settlement.ExactOverageMicrousd != 50 {
		t.Fatalf("settlement mutated: %+v", settlement)
	}
}
