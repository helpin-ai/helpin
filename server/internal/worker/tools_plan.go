package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const (
	ToolUpdatePlan = "update_plan"

	PlanStepPending    = "pending"
	PlanStepInProgress = "in_progress"
	PlanStepCompleted  = "completed"

	maxUpdatePlanSteps = 12
)

type RunPlanStep struct {
	Step   string `json:"step"`
	Status string `json:"status"`
}

type RunPlanArtifact struct {
	Note string        `json:"note,omitempty"`
	Plan []RunPlanStep `json:"plan"`
}

func toolUpdatePlan(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var plan RunPlanArtifact
	if err := json.Unmarshal(input, &plan); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if err := validateRunPlanArtifact(&plan); err != nil {
		return "", err
	}

	return toCompactJSONString(map[string]any{
		"status": "updated",
		"plan":   plan.Plan,
		"note":   plan.Note,
	}), nil
}

func validateRunPlanArtifact(plan *RunPlanArtifact) error {
	if plan == nil {
		return fmt.Errorf("plan is required")
	}
	plan.Note = strings.TrimSpace(plan.Note)
	if len(plan.Plan) == 0 {
		return fmt.Errorf("plan is required")
	}
	if len(plan.Plan) > maxUpdatePlanSteps {
		return fmt.Errorf("plan cannot exceed %d steps", maxUpdatePlanSteps)
	}

	inProgressCount := 0
	for i := range plan.Plan {
		plan.Plan[i].Step = strings.TrimSpace(plan.Plan[i].Step)
		plan.Plan[i].Status = strings.TrimSpace(plan.Plan[i].Status)
		if plan.Plan[i].Step == "" {
			return fmt.Errorf("plan step %d is missing step text", i+1)
		}
		switch plan.Plan[i].Status {
		case PlanStepPending, PlanStepInProgress, PlanStepCompleted:
		default:
			return fmt.Errorf("plan step %d has invalid status %q", i+1, plan.Plan[i].Status)
		}
		if plan.Plan[i].Status == PlanStepInProgress {
			inProgressCount++
		}
	}
	if inProgressCount > 1 {
		return fmt.Errorf("plan can include at most one in_progress step")
	}
	return nil
}

func ValidateRunPlanArtifactForContext(plan *RunPlanArtifact) error {
	return validateRunPlanArtifact(plan)
}

func ExtractLatestRunPlan(toolInvocations []appmodel.ToolInvocation) *RunPlanArtifact {
	for i := len(toolInvocations) - 1; i >= 0; i-- {
		invocation := toolInvocations[i]
		if CanonicalToolName(invocation.ToolName) != ToolUpdatePlan {
			continue
		}
		var plan RunPlanArtifact
		if err := json.Unmarshal(invocation.Input, &plan); err != nil {
			continue
		}
		if err := validateRunPlanArtifact(&plan); err != nil {
			continue
		}
		return &plan
	}
	return nil
}

func formatRunPlanArtifactContent(plan *RunPlanArtifact) string {
	if plan == nil || len(plan.Plan) == 0 {
		return ""
	}
	lines := make([]string, 0, len(plan.Plan)+2)
	if plan.Note != "" {
		lines = append(lines, "Note: "+plan.Note)
	}
	lines = append(lines, "Plan:")
	for _, step := range plan.Plan {
		prefix := "[ ]"
		switch step.Status {
		case PlanStepCompleted:
			prefix = "[x]"
		case PlanStepInProgress:
			prefix = "[>]"
		}
		lines = append(lines, fmt.Sprintf("- %s %s", prefix, step.Step))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func FormatRunPlanArtifactContentForContext(plan *RunPlanArtifact) string {
	return formatRunPlanArtifactContent(plan)
}
