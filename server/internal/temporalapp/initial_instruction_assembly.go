package temporalapp

import (
	"context"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

const defaultInitialRunUserPrompt = "Start this agent run. Follow the configured system prompt and use the available tools to complete the requested work."

func splitContractPhaseGuidance(_ string, state *resolvedRunState, initialInstructions string) (string, string) {
	initialInstructions = strings.TrimSpace(initialInstructions)
	if initialInstructions == "" {
		return "", ""
	}
	if state != nil && state.executionContractActive {
		return "", initialInstructions
	}
	return initialInstructions, ""
}

func (a *AgentRunActivities) ensureRunConversation(ctx context.Context, state *resolvedRunState, initialInstructions string, planningInput planningRunInput) ([]workerpkg.ExecutionMessage, *workerpkg.ArtifactContext, *workerpkg.ProviderContinuation, repairInstruction, error) {
	artifactContext, err := a.loadRunArtifactContext(ctx, state)
	if err != nil {
		return nil, nil, nil, repairInstruction{}, err
	}
	providerContinuation, err := a.loadProviderContinuation(ctx, state)
	if err != nil {
		return nil, nil, nil, repairInstruction{}, err
	}
	if a.runMessageRepo == nil {
		return nil, artifactContext, providerContinuation, repairInstruction{}, nil
	}

	messages, err := a.runMessageRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return nil, nil, nil, repairInstruction{}, err
	}
	var artifacts []model.AgentRunArtifact
	if state.executionContractActive && a.artifactRepo != nil {
		artifacts, err = a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
		if err != nil {
			return nil, nil, nil, repairInstruction{}, err
		}
	}
	repairHint := resolveLatestRepairInstruction(state, messages, artifacts)
	replayMessages := replayMessagesForExecution(state, messages)
	if !hasExecutionHistoryMessages(replayMessages) {
		prompt, err := a.buildInitialRunUserPrompt(ctx, state, artifactContext, planningInput, initialInstructions)
		if err != nil {
			return nil, nil, nil, repairInstruction{}, err
		}
		created, err := a.createRunMessage(ctx, state.run, "user", "prompt", prompt, nil, nil, nil, nil)
		if err != nil {
			return nil, nil, nil, repairInstruction{}, err
		}
		messages = append(messages, *created)
		replayMessages = append(replayMessages, *created)
	}

	transcriptSummary, err := a.ensureTranscriptSummaryCheckpoint(ctx, state, replayMessages)
	if err != nil {
		return nil, nil, nil, repairInstruction{}, err
	}
	history := workerpkg.BuildExecutionHistory(replayMessages, transcriptSummary)
	slog.InfoContext(ctx, "agent run prepared execution history",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"contract_active", state.executionContractActive,
		"repair_guidance_present", strings.TrimSpace(repairHint.Instructions) != "",
		"repair_guidance_class", strings.TrimSpace(repairHint.Class),
		"filtered_policy_retry_messages", len(messages)-len(replayMessages),
		"history_messages", len(history),
		"transcript_summary_present", transcriptSummary != nil && strings.TrimSpace(transcriptSummary.Summary) != "",
	)
	return history, artifactContext, providerContinuation, repairHint, nil
}

func (a *AgentRunActivities) buildInitialRunUserPrompt(ctx context.Context, state *resolvedRunState, artifactContext *workerpkg.ArtifactContext, planningInput planningRunInput, initialInstructions string) (string, error) {
	var checklist []model.PMChecklistItem
	if state.task != nil {
		items, err := a.checklistRepo.List(ctx, state.task.ID)
		if err != nil {
			return "", err
		}
		checklist = items
	}

	var ticketMessages []model.SupportMessage
	if state.conversation != nil {
		messages, err := a.messageRepo.ListByConversation(ctx, state.run.WorkspaceID, state.conversation.ID, true)
		if err != nil {
			return "", err
		}
		ticketMessages = messages
	}

	prompt := workerpkg.BuildUserPrompt(
		state.agent,
		state.task,
		state.epic,
		state.epicTasks,
		state.conversation,
		ticketMessages,
		checklist,
		artifactContext,
		planningInput.Stage,
		initialInstructions,
	)
	if strings.TrimSpace(prompt) == "" {
		return defaultInitialRunUserPrompt, nil
	}
	return prompt, nil
}

func (a *AgentRunActivities) buildInitialInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if shouldUseContractPhaseGuidance(state) {
		return a.buildContractPhaseGuidance(ctx, state, input)
	}
	tools := effectiveToolSet(state.resolved, input.AllowedTools)
	if strings.TrimSpace(input.FlowOutputKind) != "" {
		return a.buildFlowOutputInstructions(ctx, state, input)
	}
	if state.task != nil {
		if tools[workerpkg.ToolPublishTaskPlanDoc] {
			return a.buildLegacyTaskPlannerFallbackInstructions(ctx, state, input)
		}
		return a.buildTaskExecutionInstructions(ctx, state, input)
	}
	if state.run.TargetType != "epic" || state.epic == nil {
		return runInputAdditionalContext(state.run.Input), nil
	}
	if !tools[workerpkg.ToolPublishPRDDraft] || !tools[workerpkg.ToolPublishTaskPlan] {
		return runInputAdditionalContext(state.run.Input), nil
	}
	return a.buildLegacyEpicPlannerFallbackInstructions(ctx, state, input)
}

func shouldUseContractPhaseGuidance(state *resolvedRunState) bool {
	if state == nil || !state.executionContractActive {
		return false
	}
	return true
}

func (a *AgentRunActivities) buildContractPhaseGuidance(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if state == nil || state.run == nil {
		return "", nil
	}
	if state.task != nil {
		return a.buildTaskPlannerPhaseGuidance(ctx, state, input)
	}
	if state.run.TargetType == "epic" && state.epic != nil {
		return a.buildEpicPlannerPhaseGuidance(ctx, state, input)
	}
	return runInputAdditionalContext(state.run.Input), nil
}
