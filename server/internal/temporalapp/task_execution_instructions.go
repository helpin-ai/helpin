package temporalapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (a *AgentRunActivities) buildTaskExecutionInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if state.task == nil {
		return runInputAdditionalContext(state.run.Input), nil
	}

	sections, err := a.buildTaskRunContextSections(ctx, state, input)
	if err != nil {
		return "", err
	}
	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildTaskRunContextSections(ctx context.Context, state *resolvedRunState, input planningRunInput) ([]string, error) {
	var sections []string
	additionalContext := strings.TrimSpace(input.AdditionalContext)
	if additionalContext == "" && state != nil && state.run != nil {
		additionalContext = runInputAdditionalContext(state.run.Input)
	}
	sections = appendOperatorNotesSection(sections, additionalContext)
	sections = appendTaskRunRepositoryBranchSections(sections, state.run)

	taskDocumentSections, err := a.buildTaskRunDocumentContextSections(ctx, state.run.WorkspaceID, state.task)
	if err != nil {
		return nil, err
	}
	sections = append(sections, taskDocumentSections...)

	return sections, nil
}

func appendTaskRunRepositoryBranchSections(sections []string, run *model.AgentRun) []string {
	if run == nil {
		return sections
	}
	baseBranch := strings.TrimSpace(derefString(run.BaseBranch))
	workingBranch := strings.TrimSpace(derefString(run.WorkingBranch))
	if baseBranch == "" && workingBranch == "" {
		return sections
	}
	switch {
	case baseBranch != "" && workingBranch != "":
		return append(sections, fmt.Sprintf("Repository branches: base `%s`, working `%s`.", baseBranch, workingBranch))
	case workingBranch != "":
		return append(sections, fmt.Sprintf("Repository working branch: `%s`.", workingBranch))
	default:
		return append(sections, fmt.Sprintf("Repository base branch: `%s`.", baseBranch))
	}
}

func (a *AgentRunActivities) buildTaskRunDocumentContextSections(ctx context.Context, workspaceID string, task *model.PMTask) ([]string, error) {
	if task == nil {
		return nil, nil
	}
	planDocumentID := strings.TrimSpace(derefString(task.PlanDocumentID))
	planDocumentSections, err := a.buildTaskRunPlanDocumentContextSections(ctx, planDocumentID)
	if err != nil {
		return nil, err
	}
	taskLinkedDocSections, err := a.buildTaskLinkedDocsSections(ctx, workspaceID, task.ID, planDocumentID, "Other docs linked directly to this task:")
	if err != nil {
		return nil, err
	}
	return append(planDocumentSections, taskLinkedDocSections...), nil
}

func (a *AgentRunActivities) buildTaskRunPlanDocumentContextSections(ctx context.Context, planDocumentID string) ([]string, error) {
	planDocumentID = strings.TrimSpace(planDocumentID)
	if planDocumentID == "" || a.docsContentRepo == nil {
		return nil, nil
	}
	title := ""
	if a.docsDocRepo != nil {
		doc, err := a.docsDocRepo.GetByID(ctx, planDocumentID)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			title = strings.TrimSpace(doc.Title)
		}
	}
	content, err := a.docsContentRepo.GetByDocumentID(ctx, planDocumentID)
	if err != nil {
		return nil, err
	}
	if markdown := docsContentMarkdown(content); markdown != "" {
		header := fmt.Sprintf("Canonical task planning document ID: %s", planDocumentID)
		if title != "" {
			header = fmt.Sprintf("Canonical task planning document: %s [%s]", title, planDocumentID)
		}
		return []string{header + "\n" + truncatePlanningText(markdown, 12000)}, nil
	}
	return nil, nil
}
