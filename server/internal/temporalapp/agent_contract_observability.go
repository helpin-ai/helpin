package temporalapp

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

type agentTurnDebugArtifact struct {
	RuntimeKind           string   `json:"runtime_kind"`
	ContractActive        bool     `json:"contract_active"`
	ContractKey           string   `json:"contract_key,omitempty"`
	PlanningStage         string   `json:"planning_stage,omitempty"`
	ContinuationMode      string   `json:"continuation_mode"`
	RuntimeSkillRefs      []string `json:"runtime_skill_refs,omitempty"`
	ActiveSkillRefs       []string `json:"active_skill_refs,omitempty"`
	RequiredInteractions  []string `json:"required_interactions,omitempty"`
	RepairGuidancePresent bool     `json:"repair_guidance_present"`
	RepairGuidanceSource  string   `json:"repair_guidance_source,omitempty"`
	RepairGuidanceClass   string   `json:"repair_guidance_class,omitempty"`
}

func (a *AgentRunActivities) persistAgentTurnDebugArtifact(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext, assistantMessage *model.AgentRunMessage) error {
	if a == nil || a.artifactRepo == nil || state == nil || state.run == nil || execCtx == nil || assistantMessage == nil {
		return nil
	}
	runtimeKind := executionRuntimeKind(state)
	if !state.executionContractActive || !execCtx.ContractActive {
		return nil
	}

	payload := agentTurnDebugArtifact{
		RuntimeKind:           runtimeKind,
		ContractActive:        execCtx.ContractActive,
		ContractKey:           strings.TrimSpace(execCtx.ContractKey),
		PlanningStage:         strings.TrimSpace(execCtx.PlanningStage),
		ContinuationMode:      providerContinuationMode(execCtx.ProviderContinuation),
		RuntimeSkillRefs:      runtimeSkillRefKeys(execCtx.RuntimeSkillRefs),
		ActiveSkillRefs:       runtimeSkillRefKeys(execCtx.ActiveRuntimeSkillRefs),
		RequiredInteractions:  sortedCompletionInteractionKinds(completionRequiredInteractionKinds(execCtx.SkillPolicy)),
		RepairGuidancePresent: strings.TrimSpace(execCtx.RepairGuidance) != "",
		RepairGuidanceSource:  strings.TrimSpace(execCtx.RepairGuidanceSource),
		RepairGuidanceClass:   strings.TrimSpace(execCtx.RepairGuidanceClass),
	}

	_, err := a.appendRunArtifactWithMetadata(
		ctx,
		state.run,
		model.AgentRunArtifactTypeAgentTurnDebug,
		"json",
		payload,
		buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo),
	)
	return err
}

func (a *AgentRunActivities) persistAgentRepairStateArtifact(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext, assistantMessage *model.AgentRunMessage) error {
	if a == nil || a.artifactRepo == nil || state == nil || state.run == nil || execCtx == nil || execCtx.LastExecutionResult == nil || assistantMessage == nil {
		return nil
	}
	if !state.executionContractActive || !execCtx.ContractActive {
		return nil
	}
	failure := latestExecutionToolFailure(execCtx.LastExecutionResult.Messages)
	if failure == nil {
		return nil
	}
	repair := classifyToolFailureRepair(state, failure)
	if strings.TrimSpace(repair.Class) == "" || strings.TrimSpace(repair.Instructions) == "" {
		return nil
	}
	payload := model.AgentRepairState{
		Source:       "tool_failure",
		RepairClass:  strings.TrimSpace(repair.Class),
		ToolName:     strings.TrimSpace(failure.ToolName),
		RepairHint:   strings.TrimSpace(repair.Instructions),
		ErrorSummary: strings.TrimSpace(failure.Output),
	}
	_, err := a.appendRunArtifactWithMetadata(
		ctx,
		state.run,
		model.AgentRunArtifactTypeAgentRepairState,
		"json",
		payload,
		buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo),
	)
	return err
}
