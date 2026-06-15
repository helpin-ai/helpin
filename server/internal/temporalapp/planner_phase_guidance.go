package temporalapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func (a *AgentRunActivities) buildTaskPlannerSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, error) {
	assemblyState, err := a.buildTaskPlannerAssemblyState(ctx, state, input)
	if err != nil {
		return nil, err
	}
	sections := buildTaskPlannerRuleSections(state.run, assemblyState.phaseName)
	sections = append(sections, assemblyState.contextSections...)
	return sections, nil
}

func (a *AgentRunActivities) buildTaskPlannerPhaseGuidance(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	sections, err := a.buildTaskPlannerSections(ctx, state, input)
	if err != nil {
		return "", err
	}
	return strings.Join(sections, "\n\n"), nil
}

func buildTaskPlannerRuleSections(run *model.AgentRun, phaseName string) []string {
	publishTaskPlanDoc := workerpkg.RuntimeToolNameForPrompt(workerpkg.ToolPublishTaskPlanDoc)
	requestApproval := workerpkg.RuntimeToolNameForPrompt(workerpkg.ToolRequestApproval)
	sections := []string{
		fmt.Sprintf("Current planning phase: %s", phaseName),
		fmt.Sprintf("Phase objective: refine a task-scoped implementation planning document, publish it with `%s`, and stop at inline approval.", publishTaskPlanDoc),
		"Treat this as a transcript-driven task planning run. Continue from the latest human reply, active draft, linked task context, and repository evidence rather than restarting the plan from scratch.",
		"Next-step rule: clarify scope only when blocked, otherwise update the active task planning draft, publish the full replacement preview, and request inline approval when the document is ready.",
		fmt.Sprintf("Approval rule: use `%s` with phase=\"task_doc\" only after `%s` in the same turn. Treat `%s` as the final action in that turn.", requestApproval, publishTaskPlanDoc, requestApproval),
		"Approval binding rule: set preview_panel_key=\"task_plan_doc\" when requesting approval for the task planning document.",
		fmt.Sprintf("Contract reminder: `%s` must receive one JSON object whose content field contains the full markdown draft under review.", publishTaskPlanDoc),
		"Focus rule: keep the planning document grounded in the task description, task comments, task-linked docs, parent-epic constraints that matter to this task, and the current codebase context.",
	}
	if run != nil && run.InvocationMode == model.InvocationModeInteractive {
		sections = append(sections, "Interactive approval semantics: explicit approval advances the run; change requests, critique, concerns, and ambiguous replies mean the draft is still unapproved and must be revised in the same transcript.")
	}
	sections = append(sections, helpinMCPPlannerToolGuidance()...)
	return sections
}

func (a *AgentRunActivities) buildEpicPlannerSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, error) {
	assemblyState, err := a.buildEpicPlannerAssemblyState(ctx, state, input)
	if err != nil {
		return nil, err
	}
	phaseName := epicPlannerPhaseName(input, assemblyState.hasSpecContent, assemblyState.hasTasks)
	sections := buildEpicPlannerRuleSections(state.run, input, phaseName, assemblyState.hasSpecContent, assemblyState.hasTasks)
	sections = append(sections, formatInteractivePlanningFacts(input, assemblyState.hasSpecContent, assemblyState.taskCount))
	sections = append(sections, assemblyState.contextSections...)
	return sections, nil
}

func epicPlannerPhaseName(input planningRunInput, hasSpecContent bool, hasTasks bool) string {
	hasApprovedSpec := input.SpecVersionID != ""
	hasSpecDoc := input.SpecDocumentID != ""
	switch {
	case hasApprovedSpec && hasTasks:
		return "task_extension"
	case hasApprovedSpec && !hasTasks:
		return model.PlanningStagePlanTasks
	case hasSpecDoc && hasSpecContent:
		return "prd_revision"
	default:
		return model.PlanningStageDraftSpec
	}
}

func epicPlannerDerivedStateFacts(input planningRunInput, hasSpecContent bool, hasTasks bool) string {
	hasApprovedSpec := input.SpecVersionID != ""
	hasSpecDoc := input.SpecDocumentID != ""
	lines := []string{
		"Derived epic planning state facts:",
		fmt.Sprintf("- approved_spec_exists=%t", hasApprovedSpec),
		fmt.Sprintf("- draft_spec_exists=%t", hasSpecDoc && hasSpecContent),
		fmt.Sprintf("- existing_tasks_exist=%t", hasTasks),
	}
	switch {
	case hasApprovedSpec && hasTasks:
		lines = append(lines, "- derived_state=approved_spec_with_existing_tasks")
	case hasApprovedSpec && !hasTasks:
		lines = append(lines, "- derived_state=approved_spec_without_tasks")
	case hasSpecDoc && hasSpecContent:
		lines = append(lines, "- derived_state=unapproved_draft_spec_exists")
	default:
		lines = append(lines, "- derived_state=no_approved_or_draft_spec")
	}
	return strings.Join(lines, "\n")
}

func formatInteractivePlanningFacts(input planningRunInput, hasDraftSpec bool, taskCount int) string {
	facts := []string{
		fmt.Sprintf("- approved_spec_exists=%t", strings.TrimSpace(input.SpecVersionID) != ""),
		fmt.Sprintf("- draft_spec_exists=%t", hasDraftSpec),
		fmt.Sprintf("- existing_task_count=%d", taskCount),
	}
	if strings.TrimSpace(input.SpecDocumentID) != "" {
		facts = append(facts, fmt.Sprintf("- spec_document_id=%s", strings.TrimSpace(input.SpecDocumentID)))
	}
	if strings.TrimSpace(input.SpecVersionID) != "" {
		facts = append(facts, fmt.Sprintf("- approved_spec_version_id=%s", strings.TrimSpace(input.SpecVersionID)))
	}
	return "Current durable planning facts:\n" + strings.Join(facts, "\n")
}

func (a *AgentRunActivities) buildEpicPlannerPhaseGuidance(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	sections, err := a.buildEpicPlannerSections(ctx, state, input)
	if err != nil {
		return "", err
	}
	return strings.Join(sections, "\n\n"), nil
}

func buildEpicPlannerRuleSections(run *model.AgentRun, input planningRunInput, phaseName string, hasSpecContent bool, hasTasks bool) []string {
	publishPRDDraft := workerpkg.RuntimeToolNameForPrompt(workerpkg.ToolPublishPRDDraft)
	publishTaskPlan := workerpkg.RuntimeToolNameForPrompt(workerpkg.ToolPublishTaskPlan)
	requestApproval := workerpkg.RuntimeToolNameForPrompt(workerpkg.ToolRequestApproval)
	sections := []string{
		fmt.Sprintf("Planning selector tag (not an instruction): %s", phaseName),
		"Phase objective: move the epic to the next durable planning checkpoint using the current transcript, approved artifacts, linked context, and repository evidence.",
		"Transcript rule: continue from the latest human reply and current planning state rather than restarting the PRD or task plan from scratch.",
		fmt.Sprintf("Approval rule: use `%s` with phase=\"prd\" or phase=\"tasks\" only after publishing the same-turn preview artifact that is being reviewed. Treat `%s` as the final action in that turn.", requestApproval, requestApproval),
		"Approval binding rule: use preview_panel_key=\"prd_draft\" for PRD approval and preview_panel_key=\"task_plan\" for task-plan approval.",
		fmt.Sprintf("PRD contract reminder: use `%s` for markdown previews that the human will review inline.", publishPRDDraft),
		fmt.Sprintf("Task-plan contract reminder: `%s` must receive one complete JSON object with non-empty summary and proposed_tasks fields before task-plan approval is requested.", publishTaskPlan),
		"Revision rule: if the latest human reply asks for changes to the active PRD or task plan, revise the active artifact, republish the full replacement preview, and request approval again when ready.",
	}
	if run != nil && run.InvocationMode == model.InvocationModeInteractive {
		sections = append(sections, "Interactive approval semantics: only explicit approval advances the phase. Change requests, critique, concerns, and ambiguous replies keep the current phase active.")
	}
	sections = append(sections, helpinMCPPlannerToolGuidance()...)
	sections = append(sections, epicPlannerDerivedStateFacts(input, hasSpecContent, hasTasks))
	return sections
}

func helpinMCPPlannerToolGuidance() []string {
	toolNames := workerpkg.RuntimeToolNamesForPrompt(
		workerpkg.ToolUpdatePlan,
		workerpkg.ToolPublishPRDDraft,
		workerpkg.ToolPublishTaskPlan,
		workerpkg.ToolPublishTaskPlanDoc,
		workerpkg.ToolRequestUserInput,
		workerpkg.ToolRequestApproval,
	)
	return []string{
		"Helpin MCP tool naming: Helpin artifact and interaction tools are exposed through the Helpin MCP server, not as local filesystem artifacts.",
		fmt.Sprintf("Use these runtime tool names for planning artifacts and interactions: %s.", backtickJoin(toolNames)),
		"Do not create local task-plan or PRD files as substitutes for publishing artifacts through the Helpin MCP tools.",
	}
}

func backtickJoin(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			quoted = append(quoted, "`"+strings.TrimSpace(value)+"`")
		}
	}
	return strings.Join(quoted, ", ")
}
