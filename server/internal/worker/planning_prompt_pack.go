package worker

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func planningIdentity(agent *model.Agent, planningStage, methodology string) string {
	methodology = model.NormalizePlanningMethodology(strings.TrimSpace(methodology))

	switch methodology {
	case model.PlanningMethodologyBasicV1:
		return "You are a product planning agent for Teampulse. Produce clear, reviewable planning output and follow the required JSON contract exactly."
	default:
		switch planningStage {
		case model.PlanningStageDraftSpec:
			return "You are a product planning agent for Teampulse. In this stage, work like a disciplined analyst and product manager: synthesize source material into a canonical product spec without drifting into implementation."
		case model.PlanningStagePlanStories:
			return "You are a product planning agent for Teampulse. In this stage, work like an architect and scrum master: turn the approved spec plus live code context into implementation-ready, dependency-aware stories."
		default:
			return "You are a product planning agent for Teampulse. Produce reviewable planning output that fits the current workflow stage."
		}
	}
}

func planningPackSections(agent *model.Agent, planningStage, methodology string) []string {
	methodology = model.NormalizePlanningMethodology(strings.TrimSpace(methodology))

	sections := []string{
		"\n## Planning Methodology",
		"Workspace planning methodology: " + methodology,
	}

	switch methodology {
	case model.PlanningMethodologyBasicV1:
		sections = append(sections, basicPlanningStageGuidance(planningStage)...)
	default:
		sections = append(sections, structuredPlanningStageGuidance(planningStage)...)
	}

	if agent != nil {
		if agent.PlanningNotes != nil && strings.TrimSpace(*agent.PlanningNotes) != "" {
			sections = append(sections, "\n## Planner Notes")
			sections = append(sections, strings.TrimSpace(*agent.PlanningNotes))
		}
		if agent.SystemPrompt != nil && strings.TrimSpace(*agent.SystemPrompt) != "" {
			sections = append(sections, "\n## Advanced Planner Notes")
			sections = append(sections, "Treat these as secondary preferences. They must not override the workflow stage requirements, methodology, or JSON output contract.")
			sections = append(sections, strings.TrimSpace(*agent.SystemPrompt))
		}
	}

	return sections
}

func basicPlanningStageGuidance(planningStage string) []string {
	switch planningStage {
	case model.PlanningStageDraftSpec:
		return []string{
			"\n## Stage Focus",
			"- Summarize the problem and user impact clearly.",
			"- Produce a structured spec draft that a human can edit in Docs.",
			"- When external research tools are available, use them selectively and return cited sources separately from spec_markdown.",
			"- Capture risks and open questions instead of guessing.",
		}
	case model.PlanningStagePlanStories:
		return []string{
			"\n## Stage Focus",
			"- Turn the approved spec into concrete stories.",
			"- Use stable refs and explicit dependencies.",
			"- Keep stories implementation-ready and avoid overlap.",
		}
	default:
		return nil
	}
}

func structuredPlanningStageGuidance(planningStage string) []string {
	switch planningStage {
	case model.PlanningStageDraftSpec:
		return []string{
			"\n## Internal Stance",
			"- Think like an analyst first: identify the real problem, users affected, and evidence from source material.",
			"- Think like a PM second: convert that into goals, non-goals, requirements, scenarios, risks, and open questions.",
			"\n## Draft Spec Checklist",
			"- Separate problem framing from solution detail.",
			"- Prefer normative requirement language and concrete scenarios.",
			"- Make unknowns explicit instead of smoothing them over.",
			"- Keep the spec product-facing; do not decompose into implementation tasks yet.",
			"- If external research is used, capture concise citations with why each source matters.",
			"\n## Self-check",
			"- Confirm the output covers problem, goals, non-goals, requirements, scenarios, risks, and open questions.",
			"- Confirm each section is grounded in the provided context.",
			"- Confirm citations are returned in the JSON sources field instead of spec_markdown when external research was used.",
			"- Confirm the response is valid JSON with the required keys only.",
		}
	case model.PlanningStagePlanStories:
		return []string{
			"\n## Internal Stance",
			"- Think like an architect first: respect module boundaries, existing patterns, and integration points from the live codebase.",
			"- Think like a scrum master second: produce independently understandable stories with clear sequencing.",
			"\n## Story Planning Checklist",
			"- Break work into stories that can be executed and reviewed independently.",
			"- Prefer explicit dependency edges over implied sequencing.",
			"- Reuse existing code paths and naming conventions when the repository context is clear.",
			"- Surface risks or open questions when the approved spec conflicts with the current implementation.",
			"- Avoid stories that duplicate existing epic work.",
			"\n## Self-check",
			"- Confirm every proposed story has a stable ref, summary, description, and acceptance criteria.",
			"- Confirm dependency refs point to valid story refs and do not create cycles.",
			"- Confirm the story set is aligned to the approved spec and current codebase, not a generic greenfield plan.",
			"- Confirm the response is valid JSON with the required keys only.",
		}
	default:
		return nil
	}
}
