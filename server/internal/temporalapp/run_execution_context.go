package temporalapp

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/agentskills"
	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

type runtimeExecutionContextInput struct {
	workDir                   string
	runtimeKind               string
	planningInput             planningRunInput
	legacyInitialInstructions string
	phaseGuidance             string
	repairInstruction         nativeRepairInstruction
	allowedTools              map[string]bool
	activeSkillSelection      agentskills.NativeActiveSelection
	config                    *workerpkg.WorkflowConfig
	artifactContext           *workerpkg.ArtifactContext
	providerContinuation      *workerpkg.ProviderContinuation
	history                   []workerpkg.ExecutionMessage
}

func (a *AgentRunActivities) buildRuntimeExecutionContext(ctx context.Context, state *resolvedRunState, input runtimeExecutionContextInput) *workerpkg.ExecutionContext {
	execCtx := &workerpkg.ExecutionContext{
		Context:                    ctx,
		WorkDir:                    input.workDir,
		WorkspaceID:                state.run.WorkspaceID,
		AgentID:                    state.run.AgentID,
		RunID:                      state.run.ID,
		TargetType:                 state.run.TargetType,
		TargetID:                   state.run.TargetID,
		Agent:                      state.agent,
		Task:                       state.task,
		Epic:                       state.epic,
		EpicTasks:                  state.epicTasks,
		Conversation:               state.conversation,
		GitIntegration:             state.integration,
		GitAccessToken:             state.accessToken,
		Repo:                       repoFullName(state),
		BaseBranch:                 derefString(state.run.BaseBranch),
		WorkingBranch:              derefString(state.run.WorkingBranch),
		BranchSyncStatus:           strings.TrimSpace(state.branchSync.Status),
		BranchSyncConflictFiles:    slices.Clone(state.branchSync.ConflictFiles),
		InitialInstructions:        input.legacyInitialInstructions,
		PhaseGuidance:              input.phaseGuidance,
		RepairGuidance:             input.repairInstruction.Instructions,
		RepairGuidanceSource:       input.repairInstruction.Source,
		RepairGuidanceClass:        input.repairInstruction.Class,
		PlanningStage:              input.planningInput.Stage,
		PlanningMethodology:        input.planningInput.PlanningMethodology,
		PlanningSpecDocumentID:     input.planningInput.SpecDocumentID,
		PlanningSpecVersionID:      input.planningInput.SpecVersionID,
		RunFacts:                   buildDurableRunFacts(state, input.planningInput),
		Config:                     input.config,
		ResolvedProfile:            state.resolved,
		RuntimeSkillRefs:           state.runtimeSkillRefs,
		ActiveRuntimeSkillRefs:     input.activeSkillSelection.Refs,
		ActiveSkillInstructions:    input.activeSkillSelection.Instructions,
		SkillPolicy:                state.skillPolicy,
		NativeSelectivePathEnabled: state.nativeSelectivePathEnabled,
		AllowedTools:               input.allowedTools,
		Services:                   a.serviceBridge(),
		ArtifactContext:            input.artifactContext,
		ProviderContinuation:       input.providerContinuation,
		ConversationHistory:        input.history,
		Heartbeat: func(stage string) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			now := time.Now()
			recordActivityHeartbeatSafe(ctx, stage)
			state.run.ExecutionStage = &stage
			state.run.LastHeartbeatAt = &now
			if err := a.runRepo.UpdateStage(ctx, state.run.WorkspaceID, state.run.ID, stage, &now); err != nil {
				return err
			}
			a.runRepo.Notify(ctx, state.run)
			return nil
		},
		OnExecutionEvent: func(event workerpkg.ExecutionEvent) {
			a.publishRunStreamEvent(state.run, event)
		},
		OnGitPush: func(branch, sha string) error {
			return a.recordPushAndEnsureDeliveryPR(ctx, state, branch, sha)
		},
		OnPROpen: func(metadata workerpkg.PRMetadata, title string) error {
			return a.recordPR(ctx, state, metadata, title)
		},
	}
	if state.task != nil {
		execCtx.TaskID = state.task.ID
	}
	if state.conversation != nil {
		execCtx.ConversationID = state.conversation.ID
	}

	heartbeatStage := "codex_running"
	var heartbeatStageMu sync.RWMutex
	setHeartbeatStage := func(stage string) {
		heartbeatStageMu.Lock()
		defer heartbeatStageMu.Unlock()
		if strings.TrimSpace(stage) == "" {
			heartbeatStage = "codex_running"
			return
		}
		heartbeatStage = strings.TrimSpace(stage)
	}
	execCtx.HeartbeatStageProvider = func() string {
		heartbeatStageMu.RLock()
		defer heartbeatStageMu.RUnlock()
		return heartbeatStage
	}
	if input.runtimeKind == "codex" && state.run.InvocationMode == model.InvocationModeInteractive {
		execCtx.HandleInteractivePause = func(result *workerpkg.ExecutionResult) (*workerpkg.LiveExecutionResumeSignal, error) {
			return a.handleLiveCodexInteractivePause(ctx, state, execCtx, result, setHeartbeatStage)
		}
	}

	return execCtx
}
