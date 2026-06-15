package temporalapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func (a *AgentRunActivities) buildTaskPlannerContextSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, error) {
	if state.task == nil {
		return nil, fmt.Errorf("task planner requires a task target")
	}

	var sections []string
	taskPlanSections, err := a.buildTaskPlanDocumentContextSections(ctx, input.PlanDocumentID)
	if err != nil {
		return nil, err
	}
	sections = append(sections, taskPlanSections...)

	sections = appendOperatorNotesSection(sections, input.AdditionalContext)

	sections = appendTaskPlannerTaskSummarySections(sections, state.task)

	if state.epic != nil {
		sections = appendTaskPlannerParentEpicSummarySections(sections, state.epic)
	}

	specSections, err := a.buildTaskPlannerSpecContextSections(ctx, input)
	if err != nil {
		return nil, err
	}
	sections = append(sections, specSections...)

	supportingSections, err := a.buildTaskPlannerSupportingContextSections(ctx, state, input)
	if err != nil {
		return nil, err
	}
	sections = append(sections, supportingSections...)

	sections, err = a.appendPlannerRepositoryContextSection(ctx, sections, state, "Current implementation context from the live repository:", []string{
		state.task.Name,
		tiptap.RichTextToMarkdown(derefString(state.task.Description)),
		input.AdditionalContext,
	})
	if err != nil {
		return nil, err
	}

	return sections, nil
}

type taskPlannerAssemblyState struct {
	contextSections []string
	phaseName       string
}

func (a *AgentRunActivities) buildTaskPlannerAssemblyState(ctx context.Context, state *resolvedRunState, input planningRunInput) (taskPlannerAssemblyState, error) {
	contextSections, err := a.buildTaskPlannerContextSections(ctx, state, input)
	if err != nil {
		return taskPlannerAssemblyState{}, err
	}
	phaseName := strings.TrimSpace(contractSkillPlanningStage(state, input.Stage))
	if phaseName == "" {
		phaseName = model.PlanningStageTaskPlanDoc
	}
	return taskPlannerAssemblyState{
		contextSections: contextSections,
		phaseName:       phaseName,
	}, nil
}

func (a *AgentRunActivities) buildLegacyTaskPlannerFallbackSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, error) {
	if state.task == nil {
		return nil, fmt.Errorf("task planner requires a task target")
	}
	assemblyState, err := a.buildTaskPlannerAssemblyState(ctx, state, input)
	if err != nil {
		return nil, err
	}
	sections := buildLegacyTaskPlannerFallbackRuleSections(state.run)
	sections = append(sections, assemblyState.contextSections...)
	return sections, nil
}

type epicPlannerAssemblyState struct {
	contextSections []string
	hasSpecContent  bool
	hasTasks        bool
	taskCount       int
}

func (a *AgentRunActivities) buildEpicPlannerAssemblyState(ctx context.Context, state *resolvedRunState, input planningRunInput) (epicPlannerAssemblyState, error) {
	contextSections, hasSpecContent, err := a.buildEpicPlannerContextSections(ctx, state, input)
	if err != nil {
		return epicPlannerAssemblyState{}, err
	}
	taskCount := len(state.epicTasks)
	return epicPlannerAssemblyState{
		contextSections: contextSections,
		hasSpecContent:  hasSpecContent,
		hasTasks:        taskCount > 0,
		taskCount:       taskCount,
	}, nil
}

func (a *AgentRunActivities) buildLegacyEpicPlannerFallbackSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, error) {
	assemblyState, err := a.buildEpicPlannerAssemblyState(ctx, state, input)
	if err != nil {
		return nil, err
	}
	sections := buildLegacyEpicPlannerFallbackRuleSections(state.run)
	sections = append(sections, assemblyState.contextSections...)
	sections = append(sections, formatInteractivePlanningFacts(input, assemblyState.hasSpecContent, assemblyState.taskCount))
	sections = append(sections, epicPlannerDerivedStateFacts(input, assemblyState.hasSpecContent, assemblyState.hasTasks))
	return sections, nil
}

func (a *AgentRunActivities) buildEpicPlannerContextSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, bool, error) {
	sections, hasSpecContent, err := a.buildEpicPlannerSpecContextSections(ctx, input)
	if err != nil {
		return nil, false, err
	}
	sections = appendEpicTeamSections(sections, state.epic)
	sections = appendOperatorNotesSection(sections, input.AdditionalContext)

	supportingSections, err := a.buildEpicPlannerSupportingContextSections(ctx, state, input)
	if err != nil {
		return nil, false, err
	}
	sections = append(sections, supportingSections...)

	return sections, hasSpecContent, nil
}

func appendOperatorNotesSection(sections []string, additionalContext string) []string {
	additionalContext = strings.TrimSpace(additionalContext)
	if additionalContext == "" {
		return sections
	}
	return append(sections, "Operator notes:\n"+additionalContext)
}

func appendEpicTeamSections(sections []string, epic *model.PMEpic) []string {
	if epic != nil && epic.TeamID != nil && strings.TrimSpace(*epic.TeamID) != "" {
		return append(sections, fmt.Sprintf("Epic team ID: %s", strings.TrimSpace(*epic.TeamID)))
	}
	return append(sections, "This epic does not currently have a team. Before creating tasks, call list_workspace_teams and ask the human to choose the correct team inline in chat.")
}

func appendEpicLinkedTicketsSection(sections []string, linkedTickets string) []string {
	linkedTickets = strings.TrimSpace(linkedTickets)
	if linkedTickets == "" {
		return sections
	}
	return append(sections, "Support and customer context already linked to this epic:\n"+linkedTickets)
}

func appendTitledPlanningContextSection(sections []string, title, body string) []string {
	body = strings.TrimSpace(body)
	if body == "" {
		return sections
	}
	return append(sections, title+"\n"+body)
}

func appendTaskPlannerTaskSummarySections(sections []string, task *model.PMTask) []string {
	if task == nil {
		return sections
	}
	sections = append(sections, fmt.Sprintf("Task: %s", task.Name))
	if task.Description != nil {
		if description := tiptap.RichTextToMarkdown(*task.Description); description != "" {
			sections = append(sections, "Task description:\n"+truncatePlanningText(description, 8000))
		}
	}
	if task.TeamID != nil && strings.TrimSpace(*task.TeamID) != "" {
		sections = append(sections, fmt.Sprintf("Task team ID: %s", strings.TrimSpace(*task.TeamID)))
	}
	return sections
}

func appendTaskPlannerParentEpicSummarySections(sections []string, epic *model.PMEpic) []string {
	if epic == nil {
		return sections
	}
	sections = append(sections, fmt.Sprintf("Parent epic: %s", epic.Name))
	if epic.Description != nil {
		if description := tiptap.RichTextToMarkdown(*epic.Description); description != "" {
			sections = append(sections, "Parent epic description:\n"+truncatePlanningText(description, 8000))
		}
	}
	return sections
}

func (a *AgentRunActivities) buildTaskPlannerSpecContextSections(ctx context.Context, input planningRunInput) ([]string, error) {
	if input.SpecVersionID != "" {
		return a.buildApprovedSpecVersionSections(ctx, input.SpecVersionID, "Approved epic PRD version ID", "Approved epic PRD snapshot", 16000)
	}
	if input.SpecDocumentID != "" {
		specSections, _, err := a.buildSpecDocumentDraftSections(ctx, input.SpecDocumentID, "Parent epic PRD document ID", "Current epic PRD draft", 12000)
		if err != nil {
			return nil, err
		}
		return specSections, nil
	}
	return nil, nil
}

func (a *AgentRunActivities) buildTaskPlannerParentEpicLinkedDocSections(ctx context.Context, workspaceID, epicID, excludeDocumentID string) ([]string, error) {
	epicLinkedDocs, err := a.renderLinkedDocsContext(ctx, workspaceID, epicID, excludeDocumentID)
	if err != nil {
		return nil, err
	}
	return appendTitledPlanningContextSection(nil, "Other docs linked to the parent epic:", epicLinkedDocs), nil
}

func (a *AgentRunActivities) buildTaskPlannerCommentSections(ctx context.Context, taskID string) ([]string, error) {
	commentsContext, err := a.renderTaskCommentsContext(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return appendTitledPlanningContextSection(nil, "Task comments:", commentsContext), nil
}

func (a *AgentRunActivities) buildTaskPlannerSupportingContextSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, error) {
	if state == nil || state.run == nil || state.task == nil {
		return nil, nil
	}

	var sections []string

	taskLinkedDocSections, err := a.buildTaskLinkedDocsSections(ctx, state.run.WorkspaceID, state.task.ID, input.PlanDocumentID, "Other docs linked directly to this task:")
	if err != nil {
		return nil, err
	}
	sections = append(sections, taskLinkedDocSections...)

	if state.epic != nil {
		parentEpicLinkedDocSections, err := a.buildTaskPlannerParentEpicLinkedDocSections(ctx, state.run.WorkspaceID, state.epic.ID, input.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		sections = append(sections, parentEpicLinkedDocSections...)
	}

	taskCommentSections, err := a.buildTaskPlannerCommentSections(ctx, state.task.ID)
	if err != nil {
		return nil, err
	}
	sections = append(sections, taskCommentSections...)

	return sections, nil
}

func (a *AgentRunActivities) buildEpicPlannerSupportingContextSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, error) {
	if state == nil || state.run == nil || state.epic == nil {
		return nil, nil
	}

	var sections []string

	linkedDocs, err := a.renderLinkedDocsContext(ctx, state.run.WorkspaceID, state.epic.ID, input.SpecDocumentID)
	if err != nil {
		return nil, err
	}
	sections = appendTitledPlanningContextSection(sections, "Other docs linked to this epic:", linkedDocs)

	linkedTickets, err := a.renderLinkedTicketsContext(ctx, state)
	if err != nil {
		return nil, err
	}
	sections = appendEpicLinkedTicketsSection(sections, linkedTickets)

	sections, err = a.appendPlannerRepositoryContextSection(ctx, sections, state, "Current implementation context from the planning repository:", []string{
		state.epic.Name,
		tiptap.RichTextToMarkdown(derefString(state.epic.Description)),
		linkedDocs,
		linkedTickets,
		input.AdditionalContext,
	})
	if err != nil {
		return nil, err
	}

	sections = appendExistingEpicTasksSection(sections, state.epicTasks)
	return sections, nil
}

func (a *AgentRunActivities) buildEpicPlannerSpecContextSections(ctx context.Context, input planningRunInput) ([]string, bool, error) {
	var sections []string
	var hasSpecContent bool
	if input.SpecDocumentID != "" {
		specSections, draftFound, err := a.buildSpecDocumentDraftSections(ctx, input.SpecDocumentID, "Existing canonical spec document ID", "Current spec draft", 12000)
		if err != nil {
			return nil, false, err
		}
		sections = append(sections, specSections...)

		if input.SpecVersionID == "" && draftFound {
			hasSpecContent = true
			sections = append(sections[:1], append([]string{"IMPORTANT: A PRD draft already exists in the spec document but was never formally approved. Resume from the current draft instead of starting over. Present the draft, revise it if needed, and request PRD approval before any task planning."}, sections[1:]...)...)
		} else if input.SpecVersionID == "" {
			hasSpecContent = false
		}
	}
	if input.SpecVersionID != "" {
		sections = append(sections, fmt.Sprintf("Approved spec version ID: %s", input.SpecVersionID))
		specSections, err := a.buildApprovedSpecVersionSections(ctx, input.SpecVersionID, "", "", 0)
		if err != nil {
			return nil, false, err
		}
		sections = append(sections, specSections...)
		sections = append(sections, "IMPORTANT: A previously approved spec already exists. The PRD is LOCKED. Do not redraft, rewrite, or re-approve it. Use it as the read-only source of truth for task planning. If the human asks to revise the PRD, explain the spec is approved and suggest creating a follow-up epic instead, unless they insist.")
		sections = append(sections, "If the current facts show the PRD is already approved, treat persistence as complete and continue from that state. Do not replay the PRD through mutation tools.")
	}
	return sections, hasSpecContent, nil
}

func appendExistingEpicTasksSection(sections []string, tasks []model.PMTask) []string {
	if len(tasks) == 0 {
		return sections
	}
	lines := make([]string, 0, len(tasks))
	for _, task := range tasks {
		lines = append(lines, fmt.Sprintf("- %s [%s]", task.Name, task.ID))
	}
	sections = append(sections, fmt.Sprintf("IMPORTANT: %d tasks already exist under this epic. Do NOT recreate them. Only create new tasks if the human explicitly requests additions.", len(tasks)))
	sections = append(sections, "Tasks already linked to this epic:\n"+strings.Join(lines, "\n"))
	return sections
}

func (a *AgentRunActivities) buildTaskPlanDocumentContextSections(ctx context.Context, planDocumentID string) ([]string, error) {
	planDocumentID = strings.TrimSpace(planDocumentID)
	if planDocumentID == "" {
		return []string{"No canonical task planning doc exists yet. Keep the draft in chat-backed preview artifacts until approval; the platform will create, persist, and link the approved artifact."}, nil
	}
	sections := []string{fmt.Sprintf("Canonical task planning document ID: %s", planDocumentID)}
	if a.docsContentRepo == nil {
		return sections, nil
	}
	content, err := a.docsContentRepo.GetByDocumentID(ctx, planDocumentID)
	if err != nil {
		return nil, err
	}
	if content != nil && strings.TrimSpace(content.ContentText) != "" {
		sections = append(sections, "Current task planning draft already in Docs:\n"+truncatePlanningText(content.ContentText, 12000))
		sections = append(sections, "Resume from the existing planning doc draft instead of starting over unless the human explicitly wants a reset.")
	}
	return sections, nil
}

func (a *AgentRunActivities) buildTaskLinkedDocsSections(ctx context.Context, workspaceID, taskID, excludeDocumentID, header string) ([]string, error) {
	taskLinkedDocs, err := a.renderObjectLinkedDocsContext(ctx, workspaceID, model.LinkedObjectTask, taskID, excludeDocumentID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(taskLinkedDocs) == "" {
		return nil, nil
	}
	return []string{header + "\n" + taskLinkedDocs}, nil
}

func (a *AgentRunActivities) buildApprovedSpecVersionSections(ctx context.Context, specVersionID, idLabel, snapshotLabel string, maxChars int) ([]string, error) {
	if strings.TrimSpace(specVersionID) == "" || a.docsVersionRepo == nil {
		return nil, nil
	}
	version, err := a.docsVersionRepo.GetByID(ctx, specVersionID)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, nil
	}
	var sections []string
	if strings.TrimSpace(idLabel) != "" {
		sections = append(sections, fmt.Sprintf("%s: %s", idLabel, version.ID))
	}
	if strings.TrimSpace(snapshotLabel) != "" && strings.TrimSpace(version.ContentText) != "" {
		sections = append(sections, snapshotLabel+":\n"+truncatePlanningText(version.ContentText, maxChars))
	}
	return sections, nil
}

func (a *AgentRunActivities) buildSpecDocumentDraftSections(ctx context.Context, specDocumentID, documentIDLabel, draftLabel string, maxChars int) ([]string, bool, error) {
	if strings.TrimSpace(specDocumentID) == "" {
		return nil, false, nil
	}
	sections := []string{fmt.Sprintf("%s: %s", documentIDLabel, specDocumentID)}
	if a.docsContentRepo == nil {
		return sections, false, nil
	}
	content, err := a.docsContentRepo.GetByDocumentID(ctx, specDocumentID)
	if err != nil {
		return nil, false, err
	}
	if content == nil || strings.TrimSpace(content.ContentText) == "" {
		return sections, false, nil
	}
	sections = append(sections, draftLabel+":\n"+truncatePlanningText(content.ContentText, maxChars))
	return sections, true, nil
}

func (a *AgentRunActivities) appendPlannerRepositoryContextSection(ctx context.Context, sections []string, state *resolvedRunState, header string, seedParts []string) ([]string, error) {
	repoContext, err := a.buildDraftSpecCodeContext(ctx, state, strings.Join(seedParts, "\n\n"))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(repoContext) == "" {
		return sections, nil
	}
	return append(sections, header+"\n"+repoContext), nil
}
