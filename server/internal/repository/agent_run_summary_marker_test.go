package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestRuntimeSummaryMarkerPreservesSettledUsage(t *testing.T) {
	for _, test := range []struct{ name, key, value string }{
		{"finalizer", "agent_runtime_finalizer_agent_idle", "true"},
		{"replay", "agent_runtime_v2_replay_through", "20"},
		{"transcript", "agent_runtime_transcript_reconciled_runtime_updated_at", `"2026-09-07T12:00:00Z"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := openAgentVersionColumnCompatDB(t)
			repo := NewAgentRunRepository(db)
			ctx := context.Background()
			run := projectionTestRun(t) // The newer worker already committed turn 2.
			if err := repo.Create(ctx, run); err != nil {
				t.Fatal(err)
			}
			before, err := parseRunUsageWatermark(run.OutputSummary)
			if err != nil {
				t.Fatal(err)
			}
			stale := *run
			stale.OutputSummary = json.RawMessage(`{"ai_usage_checkpoint":{"turn":1,"input_tokens":100}}`)
			if err := repo.UpdateRuntimeSummaryMarker(ctx, run.WorkspaceID, run.ID, test.key, json.RawMessage(test.value)); err != nil {
				t.Fatal(err)
			}
			var row struct {
				OutputSummary model.JSONBlob
				Status        string
			}
			if err := db.Model(&model.AgentRun{}).Where("id = ?", run.ID).Take(&row).Error; err != nil {
				t.Fatal(err)
			}
			after, err := parseRunUsageWatermark(json.RawMessage(row.OutputSummary))
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(row.OutputSummary, &body); err != nil {
				t.Fatal(err)
			}
			if after != before || string(body[test.key]) != test.value || row.Status != model.AgentRunStatusCancelled {
				t.Fatalf("marker changed settled usage or lifecycle: %s", row.OutputSummary)
			}
			if err := repo.UpdateRuntimeProjection(ctx, &stale); !errors.Is(err, ErrAIUsageWatermarkChanged) {
				t.Fatalf("marker bypassed final save guard: %v", err)
			}
		})
	}
}

func TestRuntimeSummaryMarkerPreservesOtherMarkers(t *testing.T) {
	db := openAgentVersionColumnCompatDB(t)
	repo := NewAgentRunRepository(db)
	ctx := context.Background()
	run := projectionTestRun(t)
	if err := repo.Create(ctx, run); err != nil {
		t.Fatal(err)
	}
	for _, marker := range []struct{ key, value string }{
		{"agent_runtime_first", "true"},
		{"agent_runtime_v2_replay_through", "20"},
		{"agent_runtime_second", `"done"`},
		{"agent_runtime_v2_replay_through", "10"},
	} {
		if err := repo.UpdateRuntimeSummaryMarker(ctx, run.WorkspaceID, run.ID, marker.key, json.RawMessage(marker.value)); err != nil {
			t.Fatal(err)
		}
	}
	var summary string
	if err := db.Raw("SELECT output_summary FROM agent_runs WHERE id = ?", run.ID).Scan(&summary).Error; err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(summary), &body); err != nil {
		t.Fatal(err)
	}
	if string(body["agent_runtime_first"]) != "true" || string(body["agent_runtime_second"]) != `"done"` || string(body["agent_runtime_v2_replay_through"]) != "20" {
		t.Fatalf("markers regressed: %s", summary)
	}
}

func TestRuntimeSummaryMarkerRejectsInvalidWrites(t *testing.T) {
	for _, test := range []struct{ name, workspace, key, value string }{
		{"wrong workspace", "other", "marker", "true"},
		{"invalid JSON", "ws", "marker", "{"},
		{"empty key", "ws", "", "true"},
		{"invalid cursor", "ws", "agent_runtime_v2_replay_through", `"bad"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := openAgentVersionColumnCompatDB(t)
			repo := NewAgentRunRepository(db)
			run := projectionTestRun(t)
			ctx := context.Background()
			if err := repo.Create(ctx, run); err != nil {
				t.Fatal(err)
			}
			if err := repo.UpdateRuntimeSummaryMarker(ctx, test.workspace, run.ID, test.key, json.RawMessage(test.value)); err == nil {
				t.Fatal("invalid write accepted")
			}
			var summary string
			if err := db.Raw("SELECT output_summary FROM agent_runs WHERE id = ?", run.ID).Scan(&summary).Error; err != nil {
				t.Fatal(err)
			}
			if summary != string(run.OutputSummary) {
				t.Fatal("invalid write changed summary")
			}
		})
	}
}
