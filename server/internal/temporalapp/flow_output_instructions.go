package temporalapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func (a *AgentRunActivities) buildFlowOutputInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	switch strings.TrimSpace(input.FlowOutputKind) {
	case "pm.task_completion_followups":
		return a.buildTaskCompletionInstructions(state, input), nil
	case "crm.deal_review_actions":
		return a.buildCRMDealReviewInstructions(ctx, state, input)
	default:
		return runInputAdditionalContext(state.run.Input), nil
	}
}

func (a *AgentRunActivities) buildTaskCompletionInstructions(state *resolvedRunState, input planningRunInput) string {
	var sections []string
	sections = append(sections, "Review this completed task and return JSON only with the shape {\"summary\":\"...\",\"followups\":[{\"title\":\"...\",\"description\":\"...\",\"task_type\":\"chore\",\"priority\":\"medium\"}]}.")
	sections = append(sections, "Only propose internal PM/docs/support follow-up work. Do not publish customer-facing docs or website changes directly.")
	if state.task != nil {
		sections = append(sections, fmt.Sprintf("Task: %s", state.task.Name))
		if state.task.Description != nil {
			if description := tiptap.RichTextToMarkdown(*state.task.Description); description != "" {
				sections = append(sections, "Task description:\n"+truncatePlanningText(description, 8000))
			}
		}
		if state.task.EpicID != nil && *state.task.EpicID != "" {
			sections = append(sections, fmt.Sprintf("Epic ID: %s", *state.task.EpicID))
		}
	}
	if strings.TrimSpace(input.AdditionalContext) != "" {
		sections = append(sections, "Operator notes:\n"+input.AdditionalContext)
	}
	return strings.Join(sections, "\n\n")
}

func (a *AgentRunActivities) buildCRMDealReviewInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	deal, err := a.crmDealRepo.GetByID(ctx, state.run.TargetID)
	if err != nil {
		return "", err
	}
	if deal == nil {
		return "", fmt.Errorf("deal not found")
	}
	dealID := deal.ID
	signals, _, err := a.crmSignalRepo.ListSignals(ctx, state.run.WorkspaceID, model.CRMBuyerSignalListFilters{
		DealID: &dealID,
	}, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		return "", err
	}
	var sections []string
	sections = append(sections, "Review this CRM deal and return JSON only with the shape {\"summary\":\"...\",\"recommended_stage_id\":\"optional-stage-id\",\"note\":\"optional internal note\"}.")
	sections = append(sections, "Do not propose outbound messaging, contact creation, or sequence enrollment in this run.")
	sections = append(sections, fmt.Sprintf("Deal: %s", deal.Name))
	if deal.Stage != nil {
		sections = append(sections, fmt.Sprintf("Current stage: %s (%s)", deal.Stage.Name, deal.Stage.ID))
	}
	pipeline := deal.Pipeline
	if pipeline != nil && len(pipeline.Stages) == 0 {
		loadedPipeline, err := a.crmDealRepo.GetPipeline(ctx, pipeline.ID)
		if err != nil {
			return "", err
		}
		if loadedPipeline != nil {
			pipeline = loadedPipeline
		}
	}
	if pipeline != nil && len(pipeline.Stages) > 0 {
		lines := make([]string, 0, len(pipeline.Stages))
		for _, stage := range pipeline.Stages {
			lines = append(lines, fmt.Sprintf("- %s (%s)", stage.Name, stage.ID))
		}
		sections = append(sections, "Available stages:\n"+strings.Join(lines, "\n"))
	}
	if len(signals) > 0 {
		lines := make([]string, 0, len(signals))
		for _, signal := range signals {
			lines = append(lines, fmt.Sprintf("- %s: %s (confidence %.2f)", signal.SignalType, signal.Summary, signal.Confidence))
		}
		sections = append(sections, "Recent buyer signals:\n"+strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(input.AdditionalContext) != "" {
		sections = append(sections, "Operator notes:\n"+input.AdditionalContext)
	}
	return strings.Join(sections, "\n\n"), nil
}
