package temporalapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
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
	sections := []string{
		fmt.Sprintf("Current planning phase: %s", phaseName),
		"Phase objective: refine a task-scoped implementation planning document, publish it with publish_task_plan_doc, and stop at inline approval.",
		"Treat this as a transcript-driven task planning run. Continue from the latest human reply, active draft, linked task context, and repository evidence rather than restarting the plan from scratch.",
		"Next-step rule: clarify scope only when blocked, otherwise update the active task planning draft, publish the full replacement preview, and request inline approval when the document is ready.",
		"Approval rule: use request_approval with phase=\"task_doc\" only after publish_task_plan_doc in the same turn. Treat request_approval as the final action in that turn.",
		"Approval binding rule: set preview_panel_key=\"task_plan_doc\" when requesting approval for the task planning document.",
		"Contract reminder: publish_task_plan_doc must receive one JSON object whose content field contains the full markdown draft under review.",
		"Focus rule: keep the planning document grounded in the task description, task comments, task-linked docs, parent-epic constraints that matter to this task, and the current codebase context.",
	}
	if run != nil && run.InvocationMode == model.InvocationModeInteractive {
		sections = append(sections, "Interactive approval semantics: explicit approval advances the run; change requests, critique, concerns, and ambiguous replies mean the draft is still unapproved and must be revised in the same transcript.")
	}
	sections = append(sections, codexMCPPlannerToolGuidance(run)...)
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
	sections := []string{
		fmt.Sprintf("Planning selector tag (not an instruction): %s", phaseName),
		"Phase objective: move the epic to the next durable planning checkpoint using the current transcript, approved artifacts, linked context, and repository evidence.",
		"Transcript rule: continue from the latest human reply and current planning state rather than restarting the PRD or task plan from scratch.",
		"Approval rule: use request_approval with phase=\"prd\" or phase=\"tasks\" only after publishing the same-turn preview artifact that is being reviewed. Treat request_approval as the final action in that turn.",
		"Approval binding rule: use preview_panel_key=\"prd_draft\" for PRD approval and preview_panel_key=\"task_plan\" for task-plan approval.",
		"PRD contract reminder: use publish_prd_draft for markdown previews that the human will review inline.",
		"Task-plan contract reminder: publish_task_plan must receive one complete JSON object with non-empty summary and proposed_tasks fields before task-plan approval is requested.",
		"Revision rule: if the latest human reply asks for changes to the active PRD or task plan, revise the active artifact, republish the full replacement preview, and request approval again when ready.",
	}
	if run != nil && run.InvocationMode == model.InvocationModeInteractive {
		sections = append(sections, "Interactive approval semantics: only explicit approval advances the phase. Change requests, critique, concerns, and ambiguous replies keep the current phase active.")
	}
	sections = append(sections, codexMCPPlannerToolGuidance(run)...)
	sections = append(sections, epicPlannerDerivedStateFacts(input, hasSpecContent, hasTasks))
	return sections
}

func codexMCPPlannerToolGuidance(run *model.AgentRun) []string {
	if run == nil || strings.TrimSpace(run.RuntimeKind) != "codex" {
		return nil
	}
	return []string{
		"Codex MCP tool naming: Helpin artifact and interaction tools are exposed through the Helpin MCP server, not as local filesystem artifacts.",
		"Use the Helpin MCP tool namespace for update_plan, publish_prd_draft, publish_task_plan, publish_task_plan_doc, request_user_input, and request_approval. Depending on the Codex surface, this may appear as functions such as mcp__helpin__publish_task_plan or as a mcp__helpin__ namespace with those function names.",
		"Do not create local task-plan or PRD files as substitutes for publishing artifacts through the Helpin MCP tools.",
	}
}
