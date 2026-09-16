//go:build ee

package repository

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIUsageReserveCountsActiveReservations(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	ctx := context.Background()
	if _, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run:1", ReservedMicrousd: 600_000}); err != nil {
		t.Fatalf("first Reserve() error = %v", err)
	}
	_, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run:2", ReservedMicrousd: 500_000})
	if !errors.Is(err, model.ErrAIUsageExhausted) {
		t.Fatalf("second Reserve() error = %v, want exhausted", err)
	}
}

func TestAIUsageReserveRejectsInvalidRequest(t *testing.T) {
	for _, test := range []struct {
		name  string
		input AIUsageReservationRequest
	}{
		{name: "missing workspace", input: AIUsageReservationRequest{IdempotencyKey: "run"}},
		{name: "missing idempotency key", input: AIUsageReservationRequest{WorkspaceID: "ws"}},
		{name: "negative hold", input: AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run", ReservedMicrousd: -1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 0)
			if _, err := repo.Reserve(context.Background(), test.input); err == nil {
				t.Fatal("invalid reservation accepted")
			}
			var count int64
			if err := repo.db.Model(&model.AIUsageReservation{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("invalid request persisted a reservation")
			}
		})
	}
}

func TestAIUsageReserveSoftBudgetNeverBlocks(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementSoft, 1_000_000)
	if _, err := repo.Reserve(context.Background(), AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run:1", ReservedMicrousd: 2_000_000}); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
}

func TestAIUsageReserveExtraUsageNeverBlocks(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementExtra, 1_000_000)
	if _, err := repo.Reserve(context.Background(), AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run:1", ReservedMicrousd: 2_000_000}); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
}

func TestAIUsageReserveDuplicateReturnsOriginal(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	input := AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run:1", ReservedMicrousd: 600_000}
	first, err := repo.Reserve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.Reserve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate IDs = %q and %q", first.ID, second.ID)
	}
	if second.EnforcementMode != model.AIUsageEnforcementStrict {
		t.Fatalf("duplicate enforcement mode = %q, want strict", second.EnforcementMode)
	}
	var period model.AIUsagePeriod
	repo.db.First(&period, "id = ?", "period")
	if period.ReservedMicrousd != 600_000 {
		t.Fatalf("reserved = %d, want 600000", period.ReservedMicrousd)
	}
}

func TestAIUsageConcurrentReservationsCannotOverspend(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for _, key := range []string{"run:1", "run:2"} {
		wait.Add(1)
		go func(key string) {
			defer wait.Done()
			<-start
			_, err := repo.Reserve(context.Background(), AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: key, ReservedMicrousd: 600_000})
			errs <- err
		}(key)
	}
	close(start)
	wait.Wait()
	close(errs)
	var success, exhausted int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, model.ErrAIUsageExhausted):
			exhausted++
		default:
			t.Fatalf("unexpected Reserve() error = %v", err)
		}
	}
	if success != 1 || exhausted != 1 {
		t.Fatalf("success/exhausted = %d/%d, want 1/1", success, exhausted)
	}
}

func TestAIUsageResizeRejectsAllowanceOverrun(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	reservation, err := repo.Reserve(context.Background(), AIUsageReservationRequest{
		WorkspaceID: "ws", IdempotencyKey: "run:1", ReservedMicrousd: 600_000,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = repo.ResizeReservation(context.Background(), reservation.ID, 1_100_000, time.Now().UTC())
	if !errors.Is(err, model.ErrAIUsageExhausted) {
		t.Fatalf("ResizeReservation() error = %v, want exhausted", err)
	}
}

func TestAIUsageReconcileIsIdempotentAndUpdatesPeriod(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	reservation, err := repo.Reserve(context.Background(), AIUsageReservationRequest{
		WorkspaceID: "ws", IdempotencyKey: "reserve:1", ReservedMicrousd: 600_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	input := AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 400_000,
		Entry: model.AIUsageLedgerEntry{IdempotencyKey: "usage:1", EntryKind: "usage"}}
	for i := 0; i < 2; i++ {
		if _, err := repo.Reconcile(context.Background(), input); err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
	}
	var period model.AIUsagePeriod
	if err := repo.db.First(&period, "id = ?", "period").Error; err != nil {
		t.Fatal(err)
	}
	if period.UsedMicrousd != 400_000 || period.ReservedMicrousd != 0 {
		t.Fatalf("period used/reserved = %d/%d", period.UsedMicrousd, period.ReservedMicrousd)
	}
	var count int64
	repo.db.Model(&model.AIUsageLedgerEntry{}).Count(&count)
	if count != 1 {
		t.Fatalf("ledger count = %d, want 1", count)
	}
}

func TestAIUsageCheckpointIsIdempotentAndKeepsReservationActive(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	if err := repo.db.Exec(`CREATE TABLE agent_runs (id text PRIMARY KEY, workspace_id text, output_summary blob, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec(`INSERT INTO agent_runs (id, workspace_id, output_summary) VALUES (?, ?, ?)`, "run-1", "ws", `{}`).Error; err != nil {
		t.Fatal(err)
	}
	reservation, err := repo.Reserve(context.Background(), AIUsageReservationRequest{
		WorkspaceID: "ws", IdempotencyKey: "reserve:1", ReservedMicrousd: 600_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	input := AIUsageCheckpointRequest{ReservationID: reservation.ID, ChargedMicrousd: 100_000,
		RunID: "run-1", RunOutputSummary: model.JSONBlob(`{"ai_usage_checkpoint":{"turn":1}}`),
		Entry: model.AIUsageLedgerEntry{IdempotencyKey: "usage:turn:1", EntryKind: "usage"}}
	for i := 0; i < 2; i++ {
		if _, err := repo.Checkpoint(context.Background(), input); err != nil {
			t.Fatalf("Checkpoint() error = %v", err)
		}
	}
	var period model.AIUsagePeriod
	if err := repo.db.First(&period, "id = ?", "period").Error; err != nil {
		t.Fatal(err)
	}
	if period.UsedMicrousd != 100_000 || period.ReservedMicrousd != 500_000 {
		t.Fatalf("period used/reserved = %d/%d", period.UsedMicrousd, period.ReservedMicrousd)
	}
	var stored model.AIUsageReservation
	if err := repo.db.First(&stored, "id = ?", reservation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.AIUsageReservationActive || stored.ReservedMicrousd != 500_000 || stored.ConsumedMicrousd != 100_000 {
		t.Fatalf("reservation after checkpoint = %#v", stored)
	}
	var count int64
	repo.db.Model(&model.AIUsageLedgerEntry{}).Count(&count)
	if count != 1 {
		t.Fatalf("ledger count = %d, want 1", count)
	}
	var summary string
	if err := repo.db.Raw(`SELECT output_summary FROM agent_runs WHERE id = ?`, "run-1").Scan(&summary).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, `"turn":1`) {
		t.Fatalf("run output summary = %q", summary)
	}
	if _, err := repo.Reconcile(context.Background(), AIUsageReconcileRequest{
		ReservationID: reservation.ID, ChargedMicrousd: 50_000,
		Entry: model.AIUsageLedgerEntry{IdempotencyKey: "usage:turn:2", EntryKind: "usage"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.db.First(&stored, "id = ?", reservation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.AIUsageReservationReconciled || stored.ConsumedMicrousd != 150_000 {
		t.Fatalf("reservation after terminal reconciliation = %#v", stored)
	}
}

func TestAIUsageReconcileRollsBackWhenLedgerInsertFails(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	reservation, err := repo.Reserve(context.Background(), AIUsageReservationRequest{
		WorkspaceID: "ws", IdempotencyKey: "reserve:1", ReservedMicrousd: 600_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	const callback = "test:fail_ai_usage_ledger"
	err = repo.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == (model.AIUsageLedgerEntry{}).TableName() {
			tx.AddError(errors.New("forced ledger failure"))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.db.Callback().Create().Remove(callback) })

	_, err = repo.Reconcile(context.Background(), AIUsageReconcileRequest{
		ReservationID: reservation.ID, ChargedMicrousd: 400_000,
		Entry: model.AIUsageLedgerEntry{IdempotencyKey: "usage:1", EntryKind: "usage"},
	})
	if err == nil {
		t.Fatal("Reconcile() error = nil, want forced failure")
	}
	var period model.AIUsagePeriod
	repo.db.First(&period, "id = ?", "period")
	if period.UsedMicrousd != 0 || period.ReservedMicrousd != 600_000 {
		t.Fatalf("period used/reserved = %d/%d, want 0/600000", period.UsedMicrousd, period.ReservedMicrousd)
	}
	var stored model.AIUsageReservation
	repo.db.First(&stored, "id = ?", reservation.ID)
	if stored.Status != model.AIUsageReservationActive {
		t.Fatalf("reservation status = %q, want active", stored.Status)
	}
}

func TestAIUsageReleaseIsIdempotent(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	reservation, err := repo.Reserve(context.Background(), AIUsageReservationRequest{
		WorkspaceID: "ws", IdempotencyKey: "reserve:1", ReservedMicrousd: 600_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := repo.Release(context.Background(), reservation.ID, "provider_failure"); err != nil {
			t.Fatal(err)
		}
	}
	var period model.AIUsagePeriod
	repo.db.First(&period, "id = ?", "period")
	if period.ReservedMicrousd != 0 {
		t.Fatalf("reserved = %d, want 0", period.ReservedMicrousd)
	}
}

func TestAIUsageClosePeriodCreatesExactSettlement(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementExtra, 1_000_000)
	repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Updates(map[string]any{
		"used_microusd": 1_337_001, "overage_microusd": 337_001, "period_end": time.Now().UTC().Add(-time.Minute),
	})
	settlement, err := repo.ClosePeriod(context.Background(), "period", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if settlement == nil || settlement.ExactOverageMicrousd != 337_001 {
		t.Fatalf("settlement = %#v", settlement)
	}
	if settlement.RoundedInvoiceCents != 34 || settlement.RoundingAdjustmentMicrousd != 2_999 {
		t.Fatalf("rounded cents/adjustment = %d/%d", settlement.RoundedInvoiceCents, settlement.RoundingAdjustmentMicrousd)
	}
}

func TestAIUsageCloseFounderPeriodNeverCreatesSettlement(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementSoft, 150_000_000)
	repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Updates(map[string]any{
		"used_microusd": 200_000_000, "period_end": time.Now().UTC().Add(-time.Minute),
	})
	settlement, err := repo.ClosePeriod(context.Background(), "period", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if settlement != nil {
		t.Fatalf("Founder settlement = %#v, want nil", settlement)
	}
	var count int64
	repo.db.Model(&model.AIUsageSettlement{}).Count(&count)
	if count != 0 {
		t.Fatalf("settlement count = %d, want 0", count)
	}
}

func TestAIUsageOpenAndListPeriodWork(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	now := time.Now().UTC()
	repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Update("period_end", now.Add(-time.Minute))

	due, err := repo.ListDuePeriods(context.Background(), now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].ID != "period" {
		t.Fatalf("due periods = %#v", due)
	}

	next, err := repo.OpenNextPeriod(context.Background(), AIUsagePeriodSchedule{
		WorkspaceID: "other", PricingVersion: "2026-08-13", EnforcementMode: model.AIUsageEnforcementSoft,
		Start: now, End: now.AddDate(0, 1, 0), AllowanceMicrousd: 150_000_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if next.UsedMicrousd != 0 || next.AllowanceMicrousd != 150_000_000 {
		t.Fatalf("next period = %#v", next)
	}
}

func TestAIUsageListStaleReservationsDoesNotReleaseThem(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	old := time.Now().UTC().Add(-2 * time.Hour)
	reservation, err := repo.Reserve(context.Background(), AIUsageReservationRequest{
		WorkspaceID: "ws", IdempotencyKey: "stale:1", ReservedMicrousd: 100_000, HeartbeatAt: old,
	})
	if err != nil {
		t.Fatal(err)
	}
	stale, err := repo.ListStaleReservations(context.Background(), time.Now().UTC().Add(-time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 || stale[0].ID != reservation.ID {
		t.Fatalf("stale reservations = %#v", stale)
	}
	var stored model.AIUsageReservation
	repo.db.First(&stored, "id = ?", reservation.ID)
	if stored.Status != model.AIUsageReservationActive {
		t.Fatalf("status = %q, want active", stored.Status)
	}
}

func setupAIUsageRepository(t *testing.T, mode string, allowance int64) *AIUsageRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:aiusage-"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.AIUsagePeriod{}, &model.AIUsageReservation{}, &model.AIUsageLedgerEntry{}, &model.AIUsageSettlement{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&model.AIUsagePeriod{ID: "period", WorkspaceID: "ws", PeriodStart: now, PeriodEnd: now.Add(time.Hour), AllowanceMicrousd: allowance, EnforcementMode: mode, Status: model.AIUsagePeriodOpen, PricingVersion: "2026-08-13"}).Error; err != nil {
		t.Fatal(err)
	}
	return NewAIUsageRepository(db)
}
