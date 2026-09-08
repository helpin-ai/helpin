package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestUpdateRuntimeProjectionRejectsStaleUsage(t *testing.T) {
	for _, test := range []struct {
		name       string
		checkpoint string
	}{
		{name: "older turn", checkpoint: `{"turn":1,"input_tokens":200,"output_tokens":20,"cached_input_tokens":10,"reasoning_output_tokens":5}`},
		{name: "input regressed", checkpoint: `{"turn":2,"input_tokens":100,"output_tokens":20,"cached_input_tokens":10,"reasoning_output_tokens":5}`},
		{name: "output regressed", checkpoint: `{"turn":2,"input_tokens":200,"output_tokens":10,"cached_input_tokens":10,"reasoning_output_tokens":5}`},
		{name: "cache regressed", checkpoint: `{"turn":2,"input_tokens":200,"output_tokens":20,"cached_input_tokens":0,"reasoning_output_tokens":5}`},
		{name: "reasoning regressed", checkpoint: `{"turn":2,"input_tokens":200,"output_tokens":20,"cached_input_tokens":10,"reasoning_output_tokens":0}`},
		{name: "checkpoint absent", checkpoint: `null`},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := openAgentVersionColumnCompatDB(t)
			repo := NewAgentRunRepository(db)
			ctx := context.Background()
			run := projectionTestRun(t)
			if err := repo.Create(ctx, run); err != nil {
				t.Fatal(err)
			}
			original := string(run.OutputSummary)
			run.OutputSummary = json.RawMessage(`{"ai_usage_checkpoint":` + test.checkpoint + `}`)
			run.Status = model.AgentRunStatusRunning
			if err := repo.UpdateRuntimeProjection(ctx, run); !errors.Is(err, ErrAIUsageWatermarkChanged) {
				t.Fatalf("stale save accepted: %v", err)
			}
			var saved model.AgentRun
			if err := db.First(&saved, "id = ?", run.ID).Error; err != nil {
				t.Fatal(err)
			}
			if string(saved.OutputSummary) != original || saved.Status != model.AgentRunStatusCancelled {
				t.Fatalf("stale projection changed durable run: %+v", saved)
			}
		})
	}
}

func TestUpdateRuntimeProjectionCurrentWatermark(t *testing.T) {
	db := openAgentVersionColumnCompatDB(t)
	repo := NewAgentRunRepository(db)
	ctx := context.Background()
	run := projectionTestRun(t)
	if err := repo.Create(ctx, run); err != nil {
		t.Fatal(err)
	}
	message := "projected cancellation"
	run.ErrorMessage = &message
	if err := repo.UpdateRuntimeProjection(ctx, run); err != nil {
		t.Fatal(err)
	}
	var saved model.AgentRun
	if err := db.First(&saved, "id = ?", run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.ErrorMessage == nil || *saved.ErrorMessage != "projected cancellation" {
		t.Fatal("current projection not saved")
	}
}

func projectionTestRun(t *testing.T) *model.AgentRun {
	t.Helper()
	return &model.AgentRun{ID: "run", WorkspaceID: "ws", AgentID: "agent", TargetType: "task", TargetID: "task", RuntimeKind: "native_sdk", InvocationMode: model.InvocationModeInteractive, ApprovalState: "not_required", PauseReason: "none", Status: model.AgentRunStatusCancelled, Input: json.RawMessage(`{}`), OutputSummary: json.RawMessage(`{"ai_usage_checkpoint":{"turn":2,"input_tokens":200,"output_tokens":20,"cached_input_tokens":10,"reasoning_output_tokens":5}}`)}
}

func TestUpdateRuntimeProjectionCannotEraseResumedTurnBudget(t *testing.T) {
	db := openAgentVersionColumnCompatDB(t)
	repo := NewAgentRunRepository(db)
	ctx := context.Background()
	run := projectionTestRun(t)
	if err := repo.Create(ctx, run); err != nil {
		t.Fatal(err)
	}
	value := json.RawMessage(`{"turn":2,"input_tokens":200,"output_tokens":20,"cached_input_tokens":10,"reasoning_output_tokens":5}`)
	if err := repo.UpdateRuntimeSummaryMarker(ctx, run.WorkspaceID, run.ID, "ai_usage_turn_start", value); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateRuntimeProjection(ctx, run); !errors.Is(err, ErrAIUsageWatermarkChanged) {
		t.Fatalf("stale projection erased turn budget: %v", err)
	}
	if err := repo.UpdateRuntimeSummaryMarker(ctx, run.WorkspaceID, run.ID, "ai_usage_turn_start", json.RawMessage(`{"turn":1}`)); !errors.Is(err, ErrAIUsageWatermarkChanged) {
		t.Fatalf("stale resume accepted: %v", err)
	}
}
