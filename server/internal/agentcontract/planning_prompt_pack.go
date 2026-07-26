package agentcontract

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
		case model.PlanningStagePlanTasks:
			return "You are a product planning agent for Teampulse. In this stage, work like an architect and scrum master: turn the approved spec plus live code context into implementation-ready, dependency-aware tasks."
		default:
			return "You are a product planning agent for Teampulse. Produce reviewable planning output that fits the current workflow stage."
		}
	}
}

func planningPackSections(agent *model.Agent, planningStage, methodology string) []string {
	methodology = model.NormalizePlanningMethodology(strings.TrimSpace(methodology))

	sections := []string{
		"\n## Planning Methodology",
		"Planning methodology: " + methodology,
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
			"- Separate assumptions from open questions so a human can resolve them before approval.",
			"- When external research tools are available, use them selectively and return cited sources separately from spec_markdown.",
			"- Capture risks and open questions instead of guessing.",
		}
	case model.PlanningStagePlanTasks:
		return []string{
			"\n## Stage Focus",
			"- Turn the approved spec into concrete tasks.",
			"- Use stable refs and explicit dependencies.",
			"- Keep tasks implementation-ready and avoid overlap.",
			"- Mark each task with slice_type: \"vertical\", \"enabler\", or \"spike\". Prefer vertical.",
			"- Combine horizontal layers (migration, model, CRUD) into a single enabler task.",
			"- Include implementation_brief with approach, files_to_modify, and test_strategy.",
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

			"\n## Spec Abstraction Level",
			"This is a PRODUCT SPECIFICATION (PRD), not a technical design document.",
			"Implementation tasks will be created from this spec later.",
			"",
			"DO include in the spec:",
			"- Problem statement and user impact",
			"- Goals and non-goals",
			"- Functional requirements (what the system does, not how)",
			"- User-facing scenarios with GIVEN/WHEN/THEN acceptance criteria",
			"- Constraints, edge cases, and boundary conditions",
			"- Risks, assumptions, and open questions",
			"",
			"DO NOT include:",
			"- Code snippets, type definitions, or interface signatures",
			"- Database schemas, SQL, or migration details",
			"- Internal service architecture or function signatures",
			"- Step-by-step implementation algorithms",
			"- Specific library/package choices or version numbers",
			"- Cache TTLs, polling intervals, or other implementation constants",
			"",
			"Example — WRONG (task-level decomposition inside the PRD):",
			`"Implement a CompileWeeklyInsights service with a GetOrCompile method that queries analytics_competitors table and caches for 7 days"`,
			"",
			"Example — RIGHT (PRD-level):",
			`"The system SHALL compile weekly visibility insights per brand, using cached results when available within the current reporting period"`,

			"\n## Draft Spec Checklist",
			"- Separate problem framing from solution detail.",
			"- Prefer normative requirement language (\"The system SHALL\") and concrete scenarios.",
			"- Make unknowns explicit instead of smoothing them over.",
			"- Distinguish assumptions from open questions so the human owner can resolve them cleanly.",
			"- Keep the spec product-facing; do not decompose into implementation tasks yet.",
			"- Use code exploration to understand WHAT exists, not to design HOW to change it.",
			"- If external research is used, capture concise citations with why each source matters.",

			"\n## Self-check",
			"- Confirm the output covers problem, goals, non-goals, requirements, scenarios, risks, assumptions, and open questions.",
			"- Confirm each section is grounded in the provided context.",
			"- Confirm the spec reads like a PRD, not a technical design doc — no code, schemas, or implementation algorithms.",
			"- Confirm citations are returned in the JSON sources field instead of spec_markdown when external research was used.",
			"- Confirm the response is valid JSON with the required keys only.",
		}
	case model.PlanningStagePlanTasks:
		return []string{
			"\n## Internal Stance",
			"- Think like an architect first: respect module boundaries, existing patterns, and integration points from the live codebase.",
			"- Think like a scrum master second: produce independently understandable tasks with clear sequencing.",
			"\n## Task Planning Checklist",
			"- Break work into tasks that can be executed and reviewed independently.",
			"- Prefer user-visible vertical slices over backend-only or frontend-only layering.",
			"- Keep task titles flat and outcome-oriented; do not use phase prefixes or sequence labels in task names.",
			"- Only use enabler tasks when a vertical slice would be unsafe or misleading, and make that clear in the task description.",
			"- Prefer explicit dependency edges over implied sequencing.",
			"- Reuse existing code paths and naming conventions when the repository context is clear.",
			"- Surface risks or open questions when the approved spec conflicts with the current implementation.",
			"- Avoid tasks that duplicate existing epic work.",

			"\n## Slicing Rules",
			"- DEFAULT to vertical slices for user-visible work. Each vertical task should deliver a complete behavior the coding agent can demo end-to-end.",
			"- COMBINE horizontal layers into a single enabler. If multiple tasks would each be a single horizontal layer (e.g., \"Create DB migration\", \"Add CRUD repository\", \"Add CRUD handler\"), combine them into ONE enabler task that all vertical tasks depend_on via dependency_refs. Example: \"Enabler: Pipeline entity foundation (migration + model + CRUD)\" with dependent vertical tasks like \"User can create a pipeline from the UI\".",
			"- Never create more than 2 enablers per epic. If you need more, fold enabler work into the first vertical task that needs it.",
			"- If several tasks all need the same new primitive, helper, schema, metric recorder, auth scope, or base route handling, do NOT pretend they are independent vertical slices. Create an enabler and make the dependent tasks reference it via dependency_refs.",
			"- No overlapping file changes across tasks. Two tasks must not modify the same file unless one depends on the other. If they would, merge them or restructure the dependency.",
			"- Mark each task with slice_type: \"vertical\" (default, strongly preferred), \"enabler\" (shared infrastructure needed by multiple verticals), or \"spike\" (timeboxed investigation).",

			"\n## Implementation Brief",
			"For each proposed task, include an implementation_brief object that gives the coding agent a concrete build plan:",
			"- approach: 1-2 sentences on HOW to build this. Reference existing patterns in the codebase when applicable (e.g., \"Follow the WorkflowState CRUD pattern\").",
			"- files_to_modify: Ordered list of file changes. Use actual codebase paths discovered during planning. Each entry has: path (relative file path), action (\"create\", \"modify\", or \"delete\"), description (one-line summary). Order: migrations → models → repository → service → handler → router → frontend types → frontend services → frontend hooks → frontend components.",
			"- test_strategy: What to test and where. Be specific: \"Table-driven tests for CRUD in automation_rule_test.go\" not \"write tests\".",
			"- vertical_layers: Which architectural layers this task touches. Valid values: \"migration\", \"model\", \"repository\", \"service\", \"handler\", \"route\", \"frontend_type\", \"frontend_service\", \"frontend_hook\", \"frontend_component\".",
			"- depends_on_files: Key files from dependency tasks that must exist before this task can start. Only include files from tasks listed in dependency_refs.",

			"\n## Vertical Coverage",
			"After proposed_tasks, include a vertical_coverage array mapping each user-facing behavior in the spec to the tasks that deliver it:",
			`  { "behavior": "User can create a pipeline", "task_refs": ["task_1"], "full_slice": true }`,
			`  { "behavior": "Admin configures OAuth", "task_refs": ["task_2", "task_3"], "full_slice": false }`,
			"Behaviors with full_slice=false are warnings — explain in the task descriptions why the split is necessary and ensure explicit dependency_refs between the tasks.",

			"\n## Self-check",
			"- Confirm every proposed task has a stable ref, summary, description, and acceptance criteria.",
			"- Confirm dependency refs point to valid task refs and do not create cycles.",
			"- Confirm the task set is aligned to the approved spec and current codebase, not a generic greenfield plan.",
			"- Confirm each task has a slice_type and implementation_brief.",
			"- Confirm no two tasks modify the same file without a dependency edge.",
			"- Confirm enabler count ≤ 2. If more, consolidate.",
			"- Confirm vertical_coverage accounts for every spec requirement.",
			"- Confirm the response is valid JSON with the required keys only.",
		}
	default:
		return nil
	}
}
