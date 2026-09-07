package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIUsageLateSettlementPreservesOtherReservations(t *testing.T) {
	for _, released := range []bool{false, true} {
		t.Run(map[bool]string{false: "reconciled", true: "released"}[released], func(t *testing.T) {
			repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
			ctx := context.Background()
			reservation, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "reserve", ReservedMicrousd: 400000})
			if err != nil {
				t.Fatal(err)
			}
			if released {
				if err := repo.Release(ctx, reservation.ID, "cancelled"); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := repo.Reconcile(ctx, AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 100000, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "first", EntryKind: "usage"}}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "unrelated", ReservedMicrousd: 300000}); err != nil {
				t.Fatal(err)
			}
			input := AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 50000, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "late", EntryKind: "usage"}, AllowLateUsage: true}
			for i := 0; i < 2; i++ {
				if _, err := repo.Reconcile(ctx, input); err != nil {
					t.Fatal(err)
				}
			}
			var period model.AIUsagePeriod
			if err := repo.db.First(&period, "id = ?", "period").Error; err != nil {
				t.Fatal(err)
			}
			want := int64(150000)
			if released {
				want = 50000
			}
			if period.UsedMicrousd != want || period.ReservedMicrousd != 300000 {
				t.Fatalf("used=%d reserved=%d", period.UsedMicrousd, period.ReservedMicrousd)
			}
			input.Entry.IdempotencyKey = "not-allowed"
			input.AllowLateUsage = false
			if _, err := repo.Reconcile(ctx, input); err == nil {
				t.Fatal("ordinary reconciliation accepted a closed reservation")
			}
		})
	}
}

func TestAIUsageOldDuplicateAfterNewerSettlement(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	ctx := context.Background()
	if err := repo.db.Exec(`CREATE TABLE agent_runs (id text PRIMARY KEY, workspace_id text, output_summary blob, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec(`INSERT INTO agent_runs (id,workspace_id,output_summary) VALUES ('run','ws','{}')`).Error; err != nil {
		t.Fatal(err)
	}
	reservation, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "reserve", ReservedMicrousd: 400000})
	if err != nil {
		t.Fatal(err)
	}
	first := AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 100, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "terminal", EntryKind: "usage", InputTokensTotal: 100}, AllowLateUsage: true, RunID: "run", RunOutputSummary: model.JSONBlob(`{"ai_usage_checkpoint":{"turn":1,"input_tokens":100}}`)}
	if _, err := repo.Reconcile(ctx, first); err != nil {
		t.Fatal(err)
	}
	// An exact retry remains successful while it is the current watermark.
	if _, err := repo.Reconcile(ctx, first); err != nil {
		t.Fatalf("current duplicate: %v", err)
	}
	second := first
	second.Entry.IdempotencyKey = "terminal:turn:2"
	second.RunOutputSummary = model.JSONBlob(`{"ai_usage_checkpoint":{"turn":2,"input_tokens":200}}`)
	if _, err := repo.Reconcile(ctx, second); err != nil {
		t.Fatal(err)
	}
	// A second projector loaded the run before turn 1, then stalled until turn 2
	// committed. Matching the old ledger delta must not permit a stale run save.
	if _, err := repo.Reconcile(ctx, first); !errors.Is(err, ErrAIUsageWatermarkChanged) {
		t.Fatalf("stale duplicate accepted: %v", err)
	}
	var summary string
	if err := repo.db.Raw(`SELECT output_summary FROM agent_runs WHERE id = 'run'`).Scan(&summary).Error; err != nil {
		t.Fatal(err)
	}
	if summary != string(second.RunOutputSummary) {
		t.Fatalf("watermark overwritten: %s", summary)
	}
	var period model.AIUsagePeriod
	if err := repo.db.First(&period, "id = ?", reservation.PeriodID).Error; err != nil {
		t.Fatal(err)
	}
	if period.UsedMicrousd != 200 {
		t.Fatalf("duplicate changed charges: %d", period.UsedMicrousd)
	}
}

func TestAIUsageTerminalWatermarkCommitsWithLedger(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1_000_000)
	ctx := context.Background()
	if err := repo.db.Exec(`CREATE TABLE agent_runs (id text PRIMARY KEY, workspace_id text, output_summary blob, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	reservation, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "reserve", ReservedMicrousd: 400000})
	if err != nil {
		t.Fatal(err)
	}
	input := AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 100, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "terminal", EntryKind: "usage", InputTokensTotal: 100}, AllowLateUsage: true, RunID: "run", RunOutputSummary: model.JSONBlob(`{"ai_usage_checkpoint":{"input_tokens":100}}`)}
	if _, err := repo.Reconcile(ctx, input); err == nil {
		t.Fatal("missing run should roll back ledger")
	}
	var count int64
	if err := repo.db.Model(&model.AIUsageLedgerEntry{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("watermark failure committed charge")
	}
	if err := repo.db.Exec(`INSERT INTO agent_runs (id,workspace_id,output_summary) VALUES (?,?,?)`, "run", "ws", `{}`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Reconcile(ctx, input); err != nil {
		t.Fatal(err)
	}
	var summary string
	if err := repo.db.Raw(`SELECT output_summary FROM agent_runs WHERE id = ?`, "run").Scan(&summary).Error; err != nil {
		t.Fatal(err)
	}
	if summary != string(input.RunOutputSummary) {
		t.Fatal("watermark not committed")
	}
	input.Entry.InputTokensTotal = 200
	if _, err := repo.Reconcile(ctx, input); err == nil {
		t.Fatal("concurrent different cumulative checkpoint silently accepted")
	}
}
