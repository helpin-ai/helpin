package temporalapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (a *AgentRunActivities) buildLegacyTaskPlannerFallbackInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	sections, err := a.buildLegacyTaskPlannerFallbackSections(ctx, state, input)
	if err != nil {
		return "", err
	}
	return strings.Join(sections, "\n\n"), nil
}

func buildLegacyTaskPlannerFallbackRuleSections(run *model.AgentRun) []string {
	invocationMode := ""
	if run != nil {
		invocationMode = run.InvocationMode
	}
	sections := []string{
		fmt.Sprintf("Run mode: %s", invocationMode),
	}
	if invocationMode == model.InvocationModeInteractive {
		sections = append(sections,
			"The shared run drawer is available for live questions, draft previews, inline approvals, and change requests.",
			"Treat this as one transcript-driven planning run. Humans approve and request changes with normal chat replies in this same transcript.",
			"Only a clear explicit approval counts as approval. Requested changes, critique, concerns, or ambiguous replies mean the draft is not approved yet.",
		)
	}
	sections = append(sections,
		"Choose the next step from the transcript, task details, parent epic context, linked docs, comments, code context, and tool results.",
		"Use this sequence unless the human explicitly redirects you: clarify scope if needed, draft or refine the task planning doc, publish it with publish_task_plan_doc, wait for inline approval, then stop. The platform will persist and link the approved preview to the canonical task planning doc automatically.",
		"Keep approvals soft and inline. When you need approval, call request_approval with phase=\"task_doc\" and stop after the request.",
		"Treat request_approval as the final action in that turn. Do not call more tools after it, and do not append extra approval-choice prose after requesting approval.",
		"Use publish_task_plan_doc for reviewable right-pane task planning documents.",
		"publish_task_plan_doc must receive a JSON object where content is the full markdown planning draft under review. Do not send title-only payloads or empty content.",
		"Treat parent epic details, the epic PRD, and epic-linked docs as background context only. Use them to understand constraints, inherited requirements, and non-goals, but do not copy them wholesale into the task planning document unless they directly affect this task's implementation.",
		"Ground the planning document primarily in the task description, task comments, task-linked docs, and the current codebase context. Keep the output focused on this task's implementation plan.",
	)
	return sections
}

func (a *AgentRunActivities) buildLegacyEpicPlannerFallbackInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	sections, err := a.buildLegacyEpicPlannerFallbackSections(ctx, state, input)
	if err != nil {
		return "", err
	}
	return strings.Join(sections, "\n\n"), nil
}

func buildLegacyEpicPlannerFallbackRuleSections(run *model.AgentRun) []string {
	invocationMode := ""
	if run != nil {
		invocationMode = run.InvocationMode
	}
	sections := []string{
		fmt.Sprintf("Run mode: %s", invocationMode),
	}
	if invocationMode == model.InvocationModeInteractive {
		sections = append(sections,
			"The shared run drawer is available for live questions, draft previews, inline approvals, and change requests.",
			"Treat this as one transcript-driven planning run. There is no hidden planner phase machine controlling the next step for you.",
			"Humans approve and request changes with normal chat replies in this same transcript. Do not tell them to use a separate approval workflow, button, or UI gate.",
			"Only a clear explicit approval counts as approval. Any requested change, concern, critique, follow-up question, or ambiguous reply means the current phase is not approved yet.",
		)
	}
	sections = append(sections,
		"Choose the next step from the transcript, current epic state, linked docs, existing tasks, and tool results.",
		"Use this sequence unless the human explicitly redirects you: clarify scope if needed, draft/refine the PRD, publish it with publish_prd_draft, wait for inline PRD approval, let the platform persist the approved PRD artifact to the canonical epic doc, propose the implementation task plan, publish it with publish_task_plan, wait for inline task approval, then let the platform apply the approved task plan artifact and create tasks.",
		"Keep approvals soft and inline. When you need approval, call request_approval with phase=\"prd\" or phase=\"tasks\" and stop after the request.",
		"Treat request_approval as the final action in that turn. Do not call more tools after it in the same turn. Do not append extra approval-choice prose after requesting approval.",
		"After explicit PRD approval, continue automatically to task planning in the same run. Do not ask whether to proceed to tasks unless the human explicitly redirects scope.",
		"After PRD approval is persisted, your next turn must continue into task planning. Either ask the next blocking questions with request_user_input or publish_task_plan. Do not complete the run immediately after PRD approval.",
		"If the latest human reply requests changes to the PRD or task plan, revise the active artifact, republish the full replacement preview, and request_approval again when ready. Do not end the run with prose-only acknowledgement after change feedback.",
		"Use publish_prd_draft for PRD markdown previews and publish_task_plan for task plan JSON previews.",
		"publish_task_plan must receive one complete JSON object payload in that tool call. Do not send title-only payloads, raw string wrappers, partial JSON, or stringified blobs. Put the full plan under content with a non-empty summary and proposed_tasks array.",
		"proposed_tasks must be an array of full task objects. Never send arrays of strings, refs, placeholders, key names, or partial fragments. If publish_task_plan fails validation, correct the payload and retry with one complete valid task-plan object before requesting approval.",
		"Before approval, keep drafts in chat-backed preview artifacts only. After approval, the platform applies the approved artifact; do not replay approved PRDs or task plans through mutation tools.",
	)
	return sections
}
