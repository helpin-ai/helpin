package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildRolloutSummaryAggregatesByProviderModelAndPreset(t *testing.T) {
	openai := "openai"
	openaiModel := "gpt-5.4"
	anthropic := "anthropic"
	anthropicModel := "claude-4"

	debug1, _ := json.Marshal(nativeTurnDebugPayload{
		ContinuationMode:      "responses_previous_response",
		RepairGuidancePresent: true,
		RepairGuidanceClass:   "publish_task_plan_object_shape",
	})
	debug2, _ := json.Marshal(nativeTurnDebugPayload{
		ContinuationMode: "responses_previous_response",
	})
	debug3, _ := json.Marshal(nativeTurnDebugPayload{
		ContinuationMode: "full_prompt",
	})
	applied1, _ := json.Marshal(appliedPreviewPayload{
		ApprovedArtifactID: "approved-prd-1",
		Action:             "persist_prd",
		AppliedAt:          time.Now().UTC(),
	})
	applied2, _ := json.Marshal(appliedPreviewPayload{
		ApprovedArtifactID: "approved-task-1",
		Action:             "create_tasks",
		AppliedAt:          time.Now().UTC(),
	})

	summary := buildRolloutSummary(rolloutFilters{}, []rolloutRunRow{
		{
			RunID:     "run-1",
			Status:    model.AgentRunStatusCompleted,
			Provider:  &openai,
			Model:     &openaiModel,
			PresetKey: model.AgentPresetEpicPlanner,
		},
		{
			RunID:     "run-2",
			Status:    model.AgentRunStatusFailed,
			Provider:  &openai,
			Model:     &openaiModel,
			PresetKey: model.AgentPresetEpicPlanner,
		},
		{
			RunID:     "run-3",
			Status:    model.AgentRunStatusCompleted,
			Provider:  &anthropic,
			Model:     &anthropicModel,
			PresetKey: model.AgentPresetTaskPlanner,
		},
	}, []rolloutArtifactRow{
		{RunID: "run-1", ArtifactType: model.AgentRunArtifactTypeNativeTurnDebug, InlineContent: strPtr(string(debug1))},
		{RunID: "run-1", ArtifactType: model.AgentRunArtifactTypeNativeTurnDebug, InlineContent: strPtr(string(debug2))},
		{RunID: "run-1", ArtifactType: model.AgentRunArtifactTypeApprovedPreviewApplied, InlineContent: strPtr(string(applied1))},
		{RunID: "run-1", ArtifactType: model.AgentRunArtifactTypeApprovedPreviewApplied, InlineContent: strPtr(string(applied2))},
		{RunID: "run-3", ArtifactType: model.AgentRunArtifactTypeNativeTurnDebug, InlineContent: strPtr(string(debug3))},
	})

	if len(summary.Groups) != 2 {
		t.Fatalf("expected 2 provider groups, got %#v", summary.Groups)
	}

	first := summary.Groups[0]
	if first.Provider != "openai" || first.Model != "gpt-5.4" || first.PresetKey != model.AgentPresetEpicPlanner {
		t.Fatalf("unexpected first summary group %#v", first)
	}
	if first.RunCount != 2 || first.CompletedRuns != 1 || first.FailedRuns != 1 {
		t.Fatalf("unexpected openai run counts %#v", first)
	}
	if first.RunsWithNativeDebugArtifacts != 1 || first.RunsWithRepairGuidance != 1 {
		t.Fatalf("unexpected openai debug counts %#v", first)
	}
	if first.AverageNativeDebugTurns != 1 {
		t.Fatalf("expected average 1.0 native debug turns, got %#v", first)
	}
	if got := first.ContinuationModes["responses_previous_response"]; got != 2 {
		t.Fatalf("expected 2 responses continuation turns, got %#v", first.ContinuationModes)
	}
	if got := first.AppliedActions["persist_prd"]; got != 1 {
		t.Fatalf("expected persist_prd action count, got %#v", first.AppliedActions)
	}
	if got := first.AppliedActions["create_tasks"]; got != 1 {
		t.Fatalf("expected create_tasks action count, got %#v", first.AppliedActions)
	}

	second := summary.Groups[1]
	if second.Provider != "anthropic" || second.Model != "claude-4" || second.PresetKey != model.AgentPresetTaskPlanner {
		t.Fatalf("unexpected second summary group %#v", second)
	}
	if second.RunCount != 1 || second.CompletedRuns != 1 || second.FailedRuns != 0 {
		t.Fatalf("unexpected anthropic run counts %#v", second)
	}
	if got := second.ContinuationModes["full_prompt"]; got != 1 {
		t.Fatalf("expected full_prompt continuation count, got %#v", second.ContinuationModes)
	}
}

func TestBuildRolloutSummaryDefaultsUnknownProviderAndIgnoresMalformedArtifacts(t *testing.T) {
	summary := buildRolloutSummary(rolloutFilters{}, []rolloutRunRow{
		{
			RunID:     "run-1",
			Status:    model.AgentRunStatusPaused,
			PresetKey: model.AgentPresetTaskPlanner,
		},
	}, []rolloutArtifactRow{
		{RunID: "run-1", ArtifactType: model.AgentRunArtifactTypeNativeTurnDebug, InlineContent: strPtr(`not-json`)},
		{RunID: "run-1", ArtifactType: model.AgentRunArtifactTypeApprovedPreviewApplied, InlineContent: strPtr(`not-json`)},
	})

	if len(summary.Groups) != 1 {
		t.Fatalf("expected one summary group, got %#v", summary.Groups)
	}
	group := summary.Groups[0]
	if group.Provider != "unknown" {
		t.Fatalf("expected unknown provider, got %#v", group)
	}
	if group.PausedRuns != 1 || group.RunCount != 1 {
		t.Fatalf("unexpected paused run counts %#v", group)
	}
	if group.RunsWithNativeDebugArtifacts != 0 || group.RunsWithRepairGuidance != 0 {
		t.Fatalf("expected malformed artifacts to be ignored, got %#v", group)
	}
}

func strPtr(value string) *string {
	return &value
}
