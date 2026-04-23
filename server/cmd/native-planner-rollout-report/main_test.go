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
			RunID:      "run-1",
			Status:     model.AgentRunStatusCompleted,
			TargetType: "epic",
			IsSystem:   true,
			Provider:   &openai,
			Model:      &openaiModel,
			PresetKey:  model.AgentPresetEpicPlanner,
		},
		{
			RunID:      "run-2",
			Status:     model.AgentRunStatusCancelled,
			TargetType: "epic",
			IsSystem:   true,
			Provider:   &openai,
			Model:      &openaiModel,
			PresetKey:  model.AgentPresetEpicPlanner,
		},
		{
			RunID:      "run-3",
			Status:     model.AgentRunStatusCompleted,
			TargetType: "task",
			IsSystem:   true,
			Provider:   &anthropic,
			Model:      &anthropicModel,
			PresetKey:  model.AgentPresetTaskPlanner,
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
	if first.RunCount != 2 || first.CompletedRuns != 1 || first.FailedRuns != 0 || first.CancelledRuns != 1 {
		t.Fatalf("unexpected openai run counts %#v", first)
	}
	if first.SelectiveEligibleRuns != 2 || first.SelectiveEligibleMissingDebug != 1 || first.SelectiveValidationStatus != "partial_missing_native_debug" {
		t.Fatalf("unexpected openai selective validation counts %#v", first)
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
	if second.SelectiveEligibleRuns != 1 || second.SelectiveEligibleMissingDebug != 0 || second.SelectiveValidationStatus != "validated" {
		t.Fatalf("unexpected anthropic selective validation counts %#v", second)
	}
	if got := second.ContinuationModes["full_prompt"]; got != 1 {
		t.Fatalf("expected full_prompt continuation count, got %#v", second.ContinuationModes)
	}
}

func TestBuildRolloutSummaryDefaultsUnknownProviderAndIgnoresMalformedArtifacts(t *testing.T) {
	summary := buildRolloutSummary(rolloutFilters{}, []rolloutRunRow{
		{
			RunID:      "run-1",
			Status:     model.AgentRunStatusPaused,
			TargetType: "task",
			IsSystem:   true,
			PresetKey:  model.AgentPresetTaskPlanner,
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
	if group.SelectiveValidationStatus != "not_validated_no_native_debug" {
		t.Fatalf("expected missing native debug status, got %#v", group)
	}
}

func TestBuildRolloutSummaryMarksNonSystemRunsNotApplicable(t *testing.T) {
	summary := buildRolloutSummary(rolloutFilters{}, []rolloutRunRow{
		{
			RunID:      "run-1",
			Status:     model.AgentRunStatusCompleted,
			TargetType: "task",
			IsSystem:   false,
			PresetKey:  model.AgentPresetTaskPlanner,
		},
	}, nil)

	if len(summary.Groups) != 1 {
		t.Fatalf("expected one summary group, got %#v", summary.Groups)
	}
	group := summary.Groups[0]
	if group.SelectiveEligibleRuns != 0 || group.SelectiveEligibleMissingDebug != 0 {
		t.Fatalf("expected non-system run not to count as selective eligible, got %#v", group)
	}
	if group.SelectiveValidationStatus != "not_applicable" {
		t.Fatalf("expected not_applicable status, got %#v", group)
	}
}

func strPtr(value string) *string {
	return &value
}
