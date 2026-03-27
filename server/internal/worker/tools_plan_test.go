package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

func TestUpdatePlanToolReturnsUpdatedPayload(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolUpdatePlan: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolUpdatePlan, json.RawMessage(`{
		"note":"Waiting on repo findings",
		"plan":[
			{"step":"Review current PRD draft","status":"completed"},
			{"step":"Inspect relevant modules","status":"in_progress"},
			{"step":"Draft revised PRD","status":"pending"}
		]
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"status":"updated"`, `"note":"Waiting on repo findings"`, `"status":"in_progress"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected update_plan payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestUpdatePlanToolRejectsMultipleInProgressSteps(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolUpdatePlan: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolUpdatePlan, json.RawMessage(`{
		"plan":[
			{"step":"Step 1","status":"in_progress"},
			{"step":"Step 2","status":"in_progress"}
		]
	}`))
	if err == nil || !strings.Contains(err.Error(), "at most one in_progress") {
		t.Fatalf("expected in_progress validation error, got %v", err)
	}
}

func TestExtractLatestRunPlanReturnsNewestValidPlan(t *testing.T) {
	plan := ExtractLatestRunPlan([]appmodel.ToolInvocation{
		{
			ToolName: ToolUpdatePlan,
			Input:    json.RawMessage(`{"plan":[{"step":"Old","status":"completed"}]}`),
		},
		{
			ToolName: "read_file",
			Input:    json.RawMessage(`{"path":"a.go"}`),
		},
		{
			ToolName: ToolUpdatePlan,
			Input:    json.RawMessage(`{"note":"Current","plan":[{"step":"New","status":"in_progress"}]}`),
		},
	})
	if plan == nil {
		t.Fatal("expected latest run plan")
	}
	if plan.Note != "Current" || len(plan.Plan) != 1 || plan.Plan[0].Step != "New" {
		t.Fatalf("unexpected latest run plan %#v", plan)
	}
}

func TestFormatRunPlanArtifactContentForContext(t *testing.T) {
	text := FormatRunPlanArtifactContentForContext(&RunPlanArtifact{
		Note: "Keep the current scope tight.",
		Plan: []RunPlanStep{
			{Step: "Review current PRD draft", Status: PlanStepCompleted},
			{Step: "Inspect relevant modules", Status: PlanStepInProgress},
			{Step: "Draft revised PRD", Status: PlanStepPending},
		},
	})
	for _, snippet := range []string{"Note: Keep the current scope tight.", "- [x] Review current PRD draft", "- [>] Inspect relevant modules", "- [ ] Draft revised PRD"} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("expected formatted run plan content to contain %q, got %s", snippet, text)
		}
	}
}
