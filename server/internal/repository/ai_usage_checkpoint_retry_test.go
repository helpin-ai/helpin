package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPausedCheckpointRetryPreservesTerminalUsage(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementStrict, 1000000)
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
	paused := AIUsageCheckpointRequest{ReservationID: reservation.ID, ChargedMicrousd: 100, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "run:turn:1", EntryKind: "usage", InputTokensTotal: 100}, RunID: "run", RunOutputSummary: model.JSONBlob(`{"ai_usage_checkpoint":{"turn":1,"input_tokens":100}}`)}
	if _, err := repo.Checkpoint(ctx, paused); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Checkpoint(ctx, paused); err != nil {
		t.Fatalf("current retry: %v", err)
	}
	conflicting := paused
	conflicting.Entry.InputTokensTotal++
	if _, err := repo.Checkpoint(ctx, conflicting); !errors.Is(err, ErrAIUsageWatermarkChanged) {
		t.Fatalf("same key with different usage: %v", err)
	}
	terminal := AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 100, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "run:turn:2", EntryKind: "usage", InputTokensTotal: 100}, AllowLateUsage: true, RunID: "run", RunOutputSummary: model.JSONBlob(`{"ai_usage_checkpoint":{"turn":2,"input_tokens":200},"agent_runtime_usage_consumed":true}`)}
	if _, err := repo.Reconcile(ctx, terminal); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Checkpoint(ctx, paused); !errors.Is(err, ErrAIUsageWatermarkChanged) {
		t.Fatalf("stale retry: %v", err)
	}
	var summary string
	if err := repo.db.Raw(`SELECT output_summary FROM agent_runs WHERE id = 'run'`).Scan(&summary).Error; err != nil {
		t.Fatal(err)
	}
	if summary != string(terminal.RunOutputSummary) {
		t.Fatalf("usage regressed: %s", summary)
	}
}
