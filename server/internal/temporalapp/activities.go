package temporalapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

var branchTokenSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

const (
	productSpecsSpaceSlug = "product-specs"
	productSpecsSpaceName = "Product Specs"
)

var codexLivePausePollEvery = time.Second

type planningRunInput struct {
	Stage               string   `json:"stage,omitempty"`
	AdditionalContext   string   `json:"additional_context,omitempty"`
	PlanDocumentID      string   `json:"plan_document_id,omitempty"`
	SpecDocumentID      string   `json:"spec_document_id,omitempty"`
	SpecVersionID       string   `json:"spec_version_id,omitempty"`
	PlanningMethodology string   `json:"planning_methodology,omitempty"`
	AllowedTools        []string `json:"allowed_tools,omitempty"`
	FlowOutputKind      string   `json:"flow_output_kind,omitempty"`
}

type planningRunSummary struct {
	Stage               string                        `json:"stage"`
	SpecDocumentID      string                        `json:"spec_document_id,omitempty"`
	SpecVersionID       string                        `json:"spec_version_id,omitempty"`
	PlanningMethodology string                        `json:"planning_methodology,omitempty"`
	PlanDocumentID      string                        `json:"plan_document_id,omitempty"`
	Summary             string                        `json:"summary,omitempty"`
	Risks               []string                      `json:"risks,omitempty"`
	Assumptions         []string                      `json:"assumptions,omitempty"`
	OpenQuestions       []string                      `json:"open_questions,omitempty"`
	Clarifications      []model.SpecClarificationItem `json:"clarifications,omitempty"`
	Proposal            *model.OrchestrationProposal  `json:"proposal,omitempty"`
}

type supportRunActivitySummary struct {
	DraftReply    *supportRunActivityDraft `json:"draft_reply,omitempty"`
	SentMessageID *string                  `json:"sent_message_id,omitempty"`
}

type supportRunActivityDraft struct {
	Content           string  `json:"content"`
	IsInternal        bool    `json:"is_internal"`
	SenderDisplayName *string `json:"sender_display_name,omitempty"`
	ApprovalRequired  bool    `json:"approval_required"`
}

type InternalCommandExecutor interface {
	Execute(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error)
}

// AgentRunActivities contains the Temporal activities that execute an agent run.
type AgentRunActivities struct {
	runRepo             *repository.AgentRunRepository
	runMessageRepo      *repository.AgentRunMessageRepository
	agentRepo           *repository.AgentRepository
	artifactRepo        *repository.AgentRunArtifactRepository
	interactionRepo     *repository.AgentRunInteractionRepository
	sessionSnapshotRepo *repository.CodingSessionStateSnapshotRepository
	storyRepo           *repository.PMStoryRepository
	storyLinkRepo       *repository.PMStoryLinkRepository
	epicRepo            *repository.PMEpicRepository
	conversationRepo    *repository.SupportConversationRepository
	commentRepo         *repository.PMCommentRepository
	checklistRepo       *repository.PMChecklistItemRepository
	messageRepo         *repository.SupportMessageRepository
	gitIntRepo          *repository.GitIntegrationRepository
	gitRepo             *repository.GitRepositoryRepository
	gitLinkRepo         *repository.StoryGitLinkRepository
	deliveryRepo        *repository.StoryDeliveryTargetRepository
	settingsRepo        *repository.SettingsRepository
	docsSpaceRepo       *repository.DocsSpaceRepository
	docsDocRepo         *repository.DocsDocumentRepository
	docsContentRepo     *repository.DocsContentRepository
	docsVersionRepo     *repository.DocsVersionRepository
	docsLinkRepo        *repository.DocsLinkRepository
	docsSearchRepo      *repository.DocsSearchRepository
	crmDealRepo         *repository.CRMDealRepository
	crmContactRepo      *repository.CRMContactRepository
	crmSignalRepo       *repository.CRMSignalRepository
	crmActivityRepo     *repository.CRMActivityRepository
	commandExecutor     InternalCommandExecutor
	wsPublisher         websocket.EventPublisher
	runtimes            *workerpkg.RuntimeRegistry
	githubApp           *githubapp.Client
	runEngine           *RunEngine
}

// NewAgentRunActivities creates the activity set used by shared Temporal workers.
func NewAgentRunActivities(
	runRepo *repository.AgentRunRepository,
	runMessageRepo *repository.AgentRunMessageRepository,
	agentRepo *repository.AgentRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
	interactionRepo *repository.AgentRunInteractionRepository,
	sessionSnapshotRepo *repository.CodingSessionStateSnapshotRepository,
	storyRepo *repository.PMStoryRepository,
	storyLinkRepo *repository.PMStoryLinkRepository,
	epicRepo *repository.PMEpicRepository,
	conversationRepo *repository.SupportConversationRepository,
	commentRepo *repository.PMCommentRepository,
	checklistRepo *repository.PMChecklistItemRepository,
	messageRepo *repository.SupportMessageRepository,
	gitIntRepo *repository.GitIntegrationRepository,
	gitRepo *repository.GitRepositoryRepository,
	gitLinkRepo *repository.StoryGitLinkRepository,
	deliveryRepo *repository.StoryDeliveryTargetRepository,
	settingsRepo *repository.SettingsRepository,
	docsSpaceRepo *repository.DocsSpaceRepository,
	docsDocRepo *repository.DocsDocumentRepository,
	docsContentRepo *repository.DocsContentRepository,
	docsVersionRepo *repository.DocsVersionRepository,
	docsLinkRepo *repository.DocsLinkRepository,
	docsSearchRepo *repository.DocsSearchRepository,
	crmDealRepo *repository.CRMDealRepository,
	crmContactRepo *repository.CRMContactRepository,
	crmSignalRepo *repository.CRMSignalRepository,
	crmActivityRepo *repository.CRMActivityRepository,
	commandExecutor InternalCommandExecutor,
	wsPublisher websocket.EventPublisher,
	runtimes *workerpkg.RuntimeRegistry,
	githubApp *githubapp.Client,
	runEngine *RunEngine,
) *AgentRunActivities {
	return &AgentRunActivities{
		runRepo:             runRepo,
		runMessageRepo:      runMessageRepo,
		agentRepo:           agentRepo,
		artifactRepo:        artifactRepo,
		interactionRepo:     interactionRepo,
		sessionSnapshotRepo: sessionSnapshotRepo,
		storyRepo:           storyRepo,
		storyLinkRepo:       storyLinkRepo,
		epicRepo:            epicRepo,
		conversationRepo:    conversationRepo,
		commentRepo:         commentRepo,
		checklistRepo:       checklistRepo,
		messageRepo:         messageRepo,
		gitIntRepo:          gitIntRepo,
		gitRepo:             gitRepo,
		gitLinkRepo:         gitLinkRepo,
		deliveryRepo:        deliveryRepo,
		settingsRepo:        settingsRepo,
		docsSpaceRepo:       docsSpaceRepo,
		docsDocRepo:         docsDocRepo,
		docsContentRepo:     docsContentRepo,
		docsVersionRepo:     docsVersionRepo,
		docsLinkRepo:        docsLinkRepo,
		docsSearchRepo:      docsSearchRepo,
		crmDealRepo:         crmDealRepo,
		crmContactRepo:      crmContactRepo,
		crmSignalRepo:       crmSignalRepo,
		crmActivityRepo:     crmActivityRepo,
		commandExecutor:     commandExecutor,
		wsPublisher:         wsPublisher,
		runtimes:            runtimes,
		githubApp:           githubApp,
		runEngine:           runEngine,
	}
}

type resolvedRunState struct {
	run            *model.AgentRun
	agent          *model.Agent
	task           *model.PMStory
	epic           *model.PMEpic
	epicTasks      []model.PMStory
	conversation   *model.SupportConversation
	resolved       workerpkg.ResolvedProfile // merged class+agent overrides — use this for decisions
	deliveryTarget *model.StoryDeliveryTarget
	repository     *model.GitRepository
	integration    *model.GitIntegration
	teamDefault    *model.PMTeamRepoDefault
	accessToken    string
}

func executionRuntimeKind(state *resolvedRunState) string {
	if state == nil {
		return ""
	}
	if state.run != nil && strings.TrimSpace(state.run.RuntimeKind) != "" {
		return strings.TrimSpace(state.run.RuntimeKind)
	}
	if state.agent != nil {
		return strings.TrimSpace(state.agent.RuntimeKind)
	}
	return ""
}

func shouldPersistExecutionWorkspace(run *model.AgentRun, runtimeKind string) bool {
	if run == nil {
		return false
	}
	return strings.TrimSpace(runtimeKind) == "codex" && strings.TrimSpace(run.InvocationMode) == model.InvocationModeInteractive
}

func recordActivityHeartbeatSafe(ctx context.Context, details ...interface{}) {
	defer func() {
		if recover() != nil {
			// Some unit tests call activities directly without a Temporal activity context.
		}
	}()
	activity.RecordHeartbeat(ctx, details...)
}

// PrepareRunActivity resolves repo state, snapshots delivery metadata, and creates the working branch if needed.
func (a *AgentRunActivities) PrepareRunActivity(ctx context.Context, runID string) error {
	state, err := a.loadRunState(ctx, runID)
	if err != nil {
		return err
	}
	if err := ensureRunNotTerminal(state.run); err != nil {
		return err
	}

	now := time.Now()
	state.run.Status = model.AgentRunStatusRunning
	state.run.PauseReason = model.AgentRunPauseReasonNone
	if state.run.StartedAt == nil {
		state.run.StartedAt = &now
	}
	state.run.ExecutionStage = strPtr("preparing")
	state.run.LastHeartbeatAt = &now

	if state.task != nil {
		if err := a.prepareTaskDelivery(ctx, state); err != nil {
			_ = a.failRun(ctx, state, err.Error())
			return err
		}
	}

	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)

	recordActivityHeartbeatSafe(ctx, "prepared")
	return nil
}

// ExecuteRunActivity executes the agent loop on a shared runner workspace.
func (a *AgentRunActivities) ExecuteRunActivity(ctx context.Context, runID string) (ExecuteRunResult, error) {
	state, err := a.loadRunState(ctx, runID)
	if err != nil {
		return ExecuteRunResult{}, err
	}
	if err := ensureRunNotTerminal(state.run); err != nil {
		return ExecuteRunResult{}, err
	}
	planningInput, err := a.resolvePlanningRunInput(ctx, state)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if state.run.TargetType == "support_conversation" && state.run.ApprovalState == "approved" {
		if err := a.finalizeSupportConversationRun(ctx, state); err != nil {
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}

		completedAt := time.Now()
		state.run.Status = model.AgentRunStatusCompleted
		state.run.PauseReason = model.AgentRunPauseReasonNone
		state.run.CompletedAt = &completedAt
		state.run.ExecutionStage = strPtr("completed")
		state.run.LastHeartbeatAt = &completedAt
		if err := a.runRepo.Update(ctx, state.run); err != nil {
			return ExecuteRunResult{}, err
		}
		a.runRepo.Notify(ctx, state.run)
		if err := a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed); err != nil {
			return ExecuteRunResult{}, err
		}
		return ExecuteRunResult{}, nil
	}
	approvedPreviewAction, err := a.applyApprovedInteractivePreview(ctx, state, &planningInput)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	switch approvedPreviewAction {
	case "create_tasks":
		return ExecuteRunResult{}, nil
	case "persist_task_doc":
		return ExecuteRunResult{}, nil
	case "persist_prd":
		now := time.Now()
		state.run.Status = model.AgentRunStatusRunning
		state.run.PauseReason = model.AgentRunPauseReasonNone
		state.run.ExecutionStage = strPtr("continuing")
		state.run.LastHeartbeatAt = &now
		state.run.CompletedAt = nil
		if err := a.runRepo.Update(ctx, state.run); err != nil {
			return ExecuteRunResult{}, err
		}
		a.runRepo.Notify(ctx, state.run)
		return ExecuteRunResult{ContinueExecution: true}, nil
	}

	if err := a.preparePlanningRepository(ctx, state, planningInput); err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	initialInstructions, err := a.buildInitialInstructions(ctx, state, planningInput)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	now := time.Now()
	state.run.Status = "running"
	state.run.PauseReason = model.AgentRunPauseReasonNone
	state.run.StartedAt = &now
	state.run.ExecutionStage = strPtr("starting")
	state.run.LastHeartbeatAt = &now
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return ExecuteRunResult{}, err
	}
	if err := a.ensureRunBootstrapStatusMessage(ctx, state.run, "Preparing workspace and loading run context."); err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	slog.InfoContext(ctx, "agent run ensuring initial conversation",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"target_type", state.run.TargetType,
		"runtime_kind", state.run.RuntimeKind,
	)
	history, artifactContext, providerContinuation, err := a.ensureRunConversation(ctx, state, initialInstructions, planningInput)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	slog.InfoContext(ctx, "agent run preparing workspace",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"repo", repoFullName(state),
	)
	runtimeKind := executionRuntimeKind(state)
	persistWorkspace := shouldPersistExecutionWorkspace(state.run, runtimeKind)
	var (
		workDir       string
		reusedWorkDir bool
	)
	if persistWorkspace {
		workDir, reusedWorkDir, err = workerpkg.PrepareWorkspaceForRun(ctx, state.integration, repoFullName(state), state.accessToken, state.run.ID)
	} else {
		workDir, err = workerpkg.PrepareWorkspace(ctx, state.integration, repoFullName(state), state.accessToken)
	}
	if err != nil {
		_ = a.failRun(ctx, state, fmt.Sprintf("prepare workspace: %v", err))
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if !persistWorkspace {
		defer os.RemoveAll(workDir)
	}
	slog.InfoContext(ctx, "agent run workspace ready",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"repo", repoFullName(state),
		"reused", reusedWorkDir,
	)

	if state.repository != nil {
		if !reusedWorkDir {
			slog.InfoContext(ctx, "agent run checking out run ref",
				"workspace_id", state.run.WorkspaceID,
				"run_id", state.run.ID,
				"repo", state.repository.FullName,
			)
			if err := a.checkoutRunRef(ctx, workDir, state); err != nil {
				if persistWorkspace {
					_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
				}
				_ = a.failRun(ctx, state, err.Error())
				return ExecuteRunResult{}, nonRetryableRunError(err)
			}
			slog.InfoContext(ctx, "agent run checked out run ref",
				"workspace_id", state.run.WorkspaceID,
				"run_id", state.run.ID,
				"repo", state.repository.FullName,
			)
		} else {
			slog.InfoContext(ctx, "agent run reusing existing workspace checkout",
				"workspace_id", state.run.WorkspaceID,
				"run_id", state.run.ID,
				"repo", state.repository.FullName,
				"work_dir", workDir,
			)
		}
	}

	config := workerpkg.ParseWorkflowConfig(workDir)
	if config == nil {
		config = workerpkg.DefaultWorkflowConfig()
	}

	allowedTools := effectiveToolSet(state.resolved, planningInput.AllowedTools)

	bridge := a.serviceBridge()
	execCtx := &workerpkg.ExecutionContext{
		Context:                ctx,
		WorkDir:                workDir,
		WorkspaceID:            state.run.WorkspaceID,
		AgentID:                state.run.AgentID,
		RunID:                  state.run.ID,
		TargetType:             state.run.TargetType,
		TargetID:               state.run.TargetID,
		Agent:                  state.agent,
		Task:                   state.task,
		Epic:                   state.epic,
		EpicTasks:              state.epicTasks,
		Conversation:           state.conversation,
		GitIntegration:         state.integration,
		GitAccessToken:         state.accessToken,
		Repo:                   repoFullName(state),
		BaseBranch:             derefString(state.run.BaseBranch),
		WorkingBranch:          derefString(state.run.WorkingBranch),
		InitialInstructions:    initialInstructions,
		PlanningStage:          planningInput.Stage,
		PlanningMethodology:    planningInput.PlanningMethodology,
		PlanningSpecDocumentID: planningInput.SpecDocumentID,
		PlanningSpecVersionID:  planningInput.SpecVersionID,
		RunFacts:               buildDurableRunFacts(state, planningInput),
		Config:                 config,
		ResolvedProfile:        state.resolved,
		AllowedTools:           allowedTools,
		Services:               bridge,
		ArtifactContext:        artifactContext,
		ProviderContinuation:   providerContinuation,
		ConversationHistory:    history,
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
			return a.recordPush(ctx, state, branch, sha)
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
	if runtimeKind == "codex" && state.run.InvocationMode == model.InvocationModeInteractive {
		execCtx.HandleInteractivePause = func(result *workerpkg.ExecutionResult) (*workerpkg.LiveExecutionResumeSignal, error) {
			return a.handleLiveCodexInteractivePause(ctx, state, execCtx, result, setHeartbeatStage)
		}
	}

	adapter, err := a.runtimes.Get(runtimeKind)
	if err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	slog.InfoContext(ctx, "agent run runtime execution starting",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"runtime_kind", runtimeKind,
		"agent_id", state.run.AgentID,
		"work_dir", workDir,
	)

	err = adapter.Execute(execCtx, state.run)
	if err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		if ctx.Err() != nil {
			bgCtx := context.Background()
			_ = a.markAgentIdle(bgCtx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
			return ExecuteRunResult{}, nil
		}
		if err == workerpkg.ErrRunCancelled {
			bgCtx := context.Background()
			explicitlyCancelled, lookupErr := a.isRunExplicitlyCancelled(bgCtx, state.run.ID)
			if lookupErr != nil {
				_ = a.failRun(bgCtx, state, lookupErr.Error())
				return ExecuteRunResult{}, nonRetryableRunError(lookupErr)
			}
			if explicitlyCancelled {
				_ = a.markAgentIdle(bgCtx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
				return ExecuteRunResult{}, nil
			}
			unexpectedErr := fmt.Errorf("runtime reported cancellation without a cancelled run state")
			slog.ErrorContext(ctx, "agent run runtime cancelled unexpectedly",
				"workspace_id", state.run.WorkspaceID,
				"run_id", state.run.ID,
				"runtime_kind", runtimeKind,
			)
			_ = a.failRun(bgCtx, state, unexpectedErr.Error())
			return ExecuteRunResult{}, nonRetryableRunError(unexpectedErr)
		}
		bgCtx := context.Background()
		_ = a.failRun(bgCtx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	slog.InfoContext(ctx, "agent run runtime execution completed",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"runtime_kind", runtimeKind,
	)
	assistantMessage, err := a.persistAssistantRunMessage(ctx, state, execCtx)
	if err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if err := a.captureTranscriptPlanningArtifacts(ctx, state, execCtx, assistantMessage, planningInput); err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	approvalRequest := latestExecutionApprovalRequest(execCtx)
	humanInputRequest := latestExecutionHumanInputRequest(execCtx)
	authRequest := latestExecutionCodexAuthState(execCtx)
	if err := a.finalizePlanningRun(ctx, state, planningInput); err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if err := a.finalizeSupportConversationRun(ctx, state); err != nil {
		if persistWorkspace {
			_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	if state.task != nil && execCtx.WorkingBranch != "" {
		state.run.WorkingBranch = &execCtx.WorkingBranch
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return ExecuteRunResult{}, err
	}

	waitForApproval, waitForInput, waitForAuth := resolveExecutionWaitState(state.run, humanInputRequest, approvalRequest, authRequest)
	continueExecution := false
	if state.run.InvocationMode == model.InvocationModeInteractive && humanInputRequest != nil {
	}
	completedAt := time.Now()
	normalizeApprovalStateAfterExecution(state.run, waitForApproval)
	if waitForApproval {
		state.run.Status = model.AgentRunStatusPaused
		state.run.PauseReason = model.AgentRunPauseReasonHumanApproval
		state.run.CompletedAt = nil
		if approvalRequest != nil && strings.TrimSpace(approvalRequest.Phase) != "" {
			state.run.ExecutionStage = strPtr(strings.TrimSpace(approvalRequest.Phase))
		} else {
			state.run.ExecutionStage = strPtr("awaiting_approval")
		}
	} else if waitForInput {
		state.run.Status = model.AgentRunStatusPaused
		state.run.PauseReason = model.AgentRunPauseReasonHumanInput
		state.run.CompletedAt = nil
		state.run.ExecutionStage = strPtr("awaiting_input")
	} else if waitForAuth {
		state.run.Status = model.AgentRunStatusPaused
		state.run.PauseReason = model.AgentRunPauseReasonAuthentication
		state.run.CompletedAt = nil
		state.run.ExecutionStage = strPtr("awaiting_auth")
	} else if continueExecution {
		state.run.Status = model.AgentRunStatusRunning
		state.run.PauseReason = model.AgentRunPauseReasonNone
		state.run.CompletedAt = nil
	} else {
		state.run.Status = "completed"
		state.run.PauseReason = model.AgentRunPauseReasonNone
		state.run.CompletedAt = &completedAt
		state.run.ExecutionStage = strPtr("completed")
	}
	state.run.LastHeartbeatAt = &completedAt
	if isEmptySummary(state.run.OutputSummary) {
		state.run.OutputSummary = json.RawMessage(`{"status":"success"}`)
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return ExecuteRunResult{}, err
	}
	a.runRepo.Notify(ctx, state.run)
	if persistWorkspace && !waitForApproval && !waitForInput && !waitForAuth && !continueExecution {
		_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
	}

	if waitForApproval || waitForInput || waitForAuth {
		if err := a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed); err != nil {
			return ExecuteRunResult{}, err
		}
	} else if !continueExecution {
		if err := a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed); err != nil {
			return ExecuteRunResult{}, err
		}
	}

	return ExecuteRunResult{
		WaitForApproval:   waitForApproval,
		AwaitingInput:     waitForInput && !waitForApproval,
		AwaitingAuth:      waitForAuth && !waitForApproval && !waitForInput,
		ContinueExecution: continueExecution && !waitForApproval && !waitForInput && !waitForAuth,
	}, nil
}

func resolveExecutionWaitState(run *model.AgentRun, humanInputRequest *workerpkg.UserInputRequest, approvalRequest *model.ApprovalRequest, authRequest *model.CodexAuthState) (waitForApproval bool, waitForInput bool, waitForAuth bool) {
	if run == nil {
		return false, false, false
	}

	if approvalRequest != nil {
		return true, false, false
	}
	if humanInputRequest != nil {
		return false, true, false
	}
	if authRequest != nil {
		return false, false, true
	}

	waitForApproval = run.ApprovalState == "pending"
	if run.InvocationMode != model.InvocationModeInteractive {
		return waitForApproval, false, false
	}
	return waitForApproval, false, false
}

func normalizeApprovalStateAfterExecution(run *model.AgentRun, waitForApproval bool) {
	if run == nil {
		return
	}
	if waitForApproval {
		run.ApprovalState = "pending"
		return
	}
	if run.ApprovalState != "approved" {
		run.ApprovalState = "not_required"
	}
}

func (a *AgentRunActivities) finalizeSupportConversationRun(ctx context.Context, state *resolvedRunState) error {
	if state == nil || state.run == nil || state.run.TargetType != "support_conversation" || state.conversation == nil {
		return nil
	}
	if state.run.ApprovalState == "pending" {
		return nil
	}

	var summary supportRunActivitySummary
	if len(state.run.OutputSummary) == 0 {
		return nil
	}
	if err := json.Unmarshal(state.run.OutputSummary, &summary); err != nil {
		return fmt.Errorf("parse support run summary: %w", err)
	}
	if summary.SentMessageID != nil && strings.TrimSpace(*summary.SentMessageID) != "" {
		return nil
	}
	if summary.DraftReply == nil || strings.TrimSpace(summary.DraftReply.Content) == "" {
		return nil
	}

	messageID := state.run.ID
	existing, err := a.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("lookup support reply message: %w", err)
	}
	createdMessage := false
	if existing == nil {
		createdMessage = true
		existing = &model.SupportMessage{
			ID:                messageID,
			WorkspaceID:       state.run.WorkspaceID,
			ConversationID:    state.conversation.ID,
			SenderType:        "agent",
			SenderAgentID:     &state.run.AgentID,
			SenderDisplayName: summary.DraftReply.SenderDisplayName,
			Content:           strings.TrimSpace(summary.DraftReply.Content),
			IsInternal:        summary.DraftReply.IsInternal,
			MessageType:       "reply",
		}
		if err := a.messageRepo.Create(ctx, existing); err != nil {
			return fmt.Errorf("create support reply message: %w", err)
		}
	}

	summary.SentMessageID = &existing.ID
	payload, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal support run summary: %w", err)
	}
	state.run.OutputSummary = payload
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return fmt.Errorf("persist support run summary: %w", err)
	}

	if a.wsPublisher != nil {
		event := websocket.SupportMessageEvent(state.run.WorkspaceID, existing, "")
		if !createdMessage {
			event.Data = nil
		}
		a.wsPublisher.Publish(event)
	}
	a.pushVisitorConversationRefresh(ctx, state.run.WorkspaceID, state.conversation)
	return nil
}

func (a *AgentRunActivities) ensureRunConversation(ctx context.Context, state *resolvedRunState, initialInstructions string, planningInput planningRunInput) ([]workerpkg.ExecutionMessage, *workerpkg.ArtifactContext, *workerpkg.ProviderContinuation, error) {
	artifactContext, err := a.loadRunArtifactContext(ctx, state)
	if err != nil {
		return nil, nil, nil, err
	}
	providerContinuation, err := a.loadProviderContinuation(ctx, state)
	if err != nil {
		return nil, nil, nil, err
	}
	if a.runMessageRepo == nil {
		return nil, artifactContext, providerContinuation, nil
	}

	messages, err := a.runMessageRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	if !hasExecutionHistoryMessages(messages) {
		prompt, err := a.buildInitialRunUserPrompt(ctx, state, artifactContext, planningInput, initialInstructions)
		if err != nil {
			return nil, nil, nil, err
		}
		created, err := a.createRunMessage(ctx, state.run, "user", "prompt", prompt, nil, nil, nil, nil)
		if err != nil {
			return nil, nil, nil, err
		}
		messages = append(messages, *created)
	}

	transcriptSummary, err := a.ensureTranscriptSummaryCheckpoint(ctx, state, messages)
	if err != nil {
		return nil, nil, nil, err
	}
	history := workerpkg.BuildExecutionHistory(messages, transcriptSummary)
	return history, artifactContext, providerContinuation, nil
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

	return workerpkg.BuildUserPrompt(
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
	), nil
}

func (a *AgentRunActivities) loadProviderContinuation(ctx context.Context, state *resolvedRunState) (*workerpkg.ProviderContinuation, error) {
	if state == nil || state.run == nil || state.agent == nil || a.artifactRepo == nil {
		return nil, nil
	}
	provider := strings.TrimSpace(derefString(state.agent.Provider))
	if !workerpkg.ProviderSupportsResponseContinuation(provider) {
		return nil, nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return nil, err
	}
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeProviderResponseCheckpoint || artifact.InlineContent == nil {
			continue
		}
		var checkpoint model.ProviderResponseCheckpoint
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &checkpoint); err != nil {
			return nil, fmt.Errorf("parse provider response checkpoint: %w", err)
		}
		if strings.TrimSpace(checkpoint.ResponseID) == "" {
			continue
		}
		continuation := &workerpkg.ProviderContinuation{
			Provider:           strings.TrimSpace(checkpoint.Provider),
			ResponseID:         strings.TrimSpace(checkpoint.ResponseID),
			PreviousResponseID: strings.TrimSpace(checkpoint.PreviousResponseID),
			AfterSequenceNo:    checkpoint.AssistantMessageSeqNo,
		}
		slog.InfoContext(ctx, "loaded provider continuation checkpoint",
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"provider", provider,
			"response_id", continuation.ResponseID,
			"after_sequence_no", continuation.AfterSequenceNo,
		)
		return continuation, nil
	}
	return nil, nil
}

func (a *AgentRunActivities) loadRunArtifactContext(ctx context.Context, state *resolvedRunState) (*workerpkg.ArtifactContext, error) {
	if state == nil || state.run == nil {
		return nil, nil
	}

	artifactContext := &workerpkg.ArtifactContext{}
	if a.artifactRepo != nil {
		artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
		if err != nil {
			return nil, err
		}
		entries, err := buildArtifactContextEntries(artifacts)
		if err != nil {
			return nil, err
		}
		artifactContext.Entries = append(artifactContext.Entries, entries...)
	}
	if state.epic != nil && state.epic.SpecDocumentID != nil && strings.TrimSpace(*state.epic.SpecDocumentID) != "" && a.docsContentRepo != nil {
		content, err := a.docsContentRepo.GetByDocumentID(ctx, *state.epic.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if content != nil && strings.TrimSpace(content.ContentText) != "" {
			artifactContext.Entries = append(artifactContext.Entries, workerpkg.ArtifactContextEntry{
				Label:        "Linked epic spec document",
				Source:       "spec_document",
				Status:       "approved",
				Format:       "text",
				Content:      strings.TrimSpace(content.ContentText),
				PreserveFull: true,
			})
		}
	}
	if len(artifactContext.Entries) == 0 {
		return nil, nil
	}
	return workerpkg.TrimArtifactContext(artifactContext), nil
}

func buildArtifactContextEntries(artifacts []model.AgentRunArtifact) ([]workerpkg.ArtifactContextEntry, error) {
	if len(artifacts) == 0 {
		return nil, nil
	}
	sorted := append([]model.AgentRunArtifact(nil), artifacts...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].SequenceNo == sorted[j].SequenceNo {
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return sorted[i].SequenceNo < sorted[j].SequenceNo
	})

	applied := make(map[string]model.AppliedApprovedRunPreview)
	latestRunPreview := make(map[string]workerpkg.PublishedPreview)
	latestApproved := make(map[string]approvedPreviewContextEntry)
	var latestRunPlan *workerpkg.ArtifactContextEntry
	otherEntries := make([]workerpkg.ArtifactContextEntry, 0)

	for _, artifact := range sorted {
		if artifact.InlineContent == nil {
			continue
		}
		switch strings.TrimSpace(artifact.ArtifactType) {
		case model.AgentRunArtifactTypeApprovedPreviewApplied:
			var marker model.AppliedApprovedRunPreview
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &marker); err != nil {
				return nil, fmt.Errorf("parse approved preview applied artifact: %w", err)
			}
			if id := strings.TrimSpace(marker.ApprovedArtifactID); id != "" {
				applied[id] = marker
			}
		case workerpkg.RunPreviewArtifactType:
			var preview workerpkg.PublishedPreview
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
				return nil, fmt.Errorf("parse run preview artifact: %w", err)
			}
			latestRunPreview[strings.TrimSpace(preview.PanelKey)] = preview
		case model.AgentRunArtifactTypeApprovedPreview:
			var preview model.ApprovedRunPreview
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
				return nil, fmt.Errorf("parse approved preview artifact: %w", err)
			}
			latestApproved[strings.TrimSpace(preview.PanelKey)] = approvedPreviewContextEntry{
				ArtifactID: artifact.ID,
				Preview:    preview,
			}
		case model.AgentRunArtifactTypeRunPlan:
			entry, err := buildRunPlanArtifactContextEntry(artifact)
			if err != nil {
				return nil, err
			}
			latestRunPlan = entry
		default:
			if entry := buildOtherArtifactContextEntry(artifact); entry != nil {
				otherEntries = append(otherEntries, *entry)
			}
		}
	}

	panelKeys := make([]string, 0, len(latestRunPreview)+len(latestApproved))
	seenPanels := make(map[string]bool)
	for panelKey := range latestRunPreview {
		if panelKey == "" || seenPanels[panelKey] {
			continue
		}
		panelKeys = append(panelKeys, panelKey)
		seenPanels[panelKey] = true
	}
	for panelKey := range latestApproved {
		if panelKey == "" || seenPanels[panelKey] {
			continue
		}
		panelKeys = append(panelKeys, panelKey)
		seenPanels[panelKey] = true
	}
	sort.Strings(panelKeys)

	entries := make([]workerpkg.ArtifactContextEntry, 0, len(panelKeys)*2+len(otherEntries))
	for _, panelKey := range panelKeys {
		if preview, ok := latestRunPreview[panelKey]; ok && latestApproved[panelKey].Preview.Phase == "" {
			content, err := renderArtifactContextContent(preview.Format, preview.Content)
			if err != nil {
				return nil, err
			}
			entries = append(entries, workerpkg.ArtifactContextEntry{
				Label:        fmt.Sprintf("Current preview for %s", panelKey),
				Source:       workerpkg.RunPreviewArtifactType,
				Status:       "draft",
				Format:       preview.Format,
				Content:      content,
				PreserveFull: true,
			})
		}
		if approved, ok := latestApproved[panelKey]; ok {
			status := "approved"
			if marker, ok := applied[approved.ArtifactID]; ok && strings.TrimSpace(marker.Action) != "" {
				status = "approved_and_" + strings.TrimSpace(marker.Action)
			}
			content, err := renderArtifactContextContent(approved.Preview.Format, approved.Preview.Content)
			if err != nil {
				return nil, err
			}
			entries = append(entries, workerpkg.ArtifactContextEntry{
				Label:        fmt.Sprintf("Approved preview for %s", panelKey),
				Source:       model.AgentRunArtifactTypeApprovedPreview,
				Status:       status,
				Format:       approved.Preview.Format,
				Content:      content,
				PreserveFull: true,
			})
		}
	}
	if latestRunPlan != nil {
		entries = append(entries, *latestRunPlan)
	}
	entries = append(entries, otherEntries...)
	return entries, nil
}

type approvedPreviewContextEntry struct {
	ArtifactID string
	Preview    model.ApprovedRunPreview
}

func buildRunPlanArtifactContextEntry(artifact model.AgentRunArtifact) (*workerpkg.ArtifactContextEntry, error) {
	content := strings.TrimSpace(derefString(artifact.InlineContent))
	if content == "" {
		return nil, nil
	}
	var plan workerpkg.RunPlanArtifact
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("parse run plan artifact: %w", err)
	}
	if err := workerpkg.ValidateRunPlanArtifactForContext(&plan); err != nil {
		return nil, nil
	}
	rendered := workerpkg.FormatRunPlanArtifactContentForContext(&plan)
	if rendered == "" {
		return nil, nil
	}
	return &workerpkg.ArtifactContextEntry{
		Label:   "Current execution plan",
		Source:  model.AgentRunArtifactTypeRunPlan,
		Status:  "active",
		Format:  "text",
		Content: rendered,
	}, nil
}

func buildOtherArtifactContextEntry(artifact model.AgentRunArtifact) *workerpkg.ArtifactContextEntry {
	content := strings.TrimSpace(derefString(artifact.InlineContent))
	if content == "" {
		return nil
	}
	switch strings.TrimSpace(artifact.ArtifactType) {
	case "product_spec_draft":
		return &workerpkg.ArtifactContextEntry{
			Label:        "Structured product spec draft artifact",
			Source:       artifact.ArtifactType,
			Status:       "draft",
			Format:       artifact.Format,
			Content:      content,
			PreserveFull: true,
		}
	case "story_plan_proposal":
		return &workerpkg.ArtifactContextEntry{
			Label:        "Structured task plan proposal artifact",
			Source:       artifact.ArtifactType,
			Status:       "draft",
			Format:       artifact.Format,
			Content:      content,
			PreserveFull: true,
		}
	default:
		return nil
	}
}

func (a *AgentRunActivities) ensureTranscriptSummaryCheckpoint(ctx context.Context, state *resolvedRunState, messages []model.AgentRunMessage) (*workerpkg.TranscriptSummaryCheckpoint, error) {
	if a.artifactRepo == nil || state == nil || state.run == nil {
		return nil, nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return nil, err
	}
	latest, err := workerpkg.LatestTranscriptSummaryCheckpoint(artifacts)
	if err != nil {
		return nil, err
	}

	next := workerpkg.BuildTranscriptSummaryCheckpoint(messages)
	if next == nil {
		return latest, nil
	}
	if latest != nil && latest.CoveredThroughSequenceNo >= next.CoveredThroughSequenceNo {
		return latest, nil
	}

	if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, workerpkg.TranscriptSummaryArtifactType, "json", next, buildAssistantSequenceArtifactMetadata(lastAssistantSequenceNoUpTo(messages, next.CoveredThroughSequenceNo))); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "saved transcript summary checkpoint",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"covered_through_sequence_no", next.CoveredThroughSequenceNo,
		"source_message_count", next.SourceMessageCount,
	)
	return next, nil
}

func renderArtifactContextContent(format string, raw json.RawMessage) (string, error) {
	switch strings.TrimSpace(format) {
	case workerpkg.PreviewFormatMarkdown:
		var markdown string
		if err := json.Unmarshal(raw, &markdown); err != nil {
			return "", fmt.Errorf("parse markdown artifact content: %w", err)
		}
		return strings.TrimSpace(markdown), nil
	case workerpkg.PreviewFormatJSON:
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("parse json artifact content: %w", err)
		}
		pretty, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return "", fmt.Errorf("format json artifact content: %w", err)
		}
		return string(pretty), nil
	default:
		return strings.TrimSpace(string(raw)), nil
	}
}

func shouldIncludeRunMessageInExecutionHistory(message model.AgentRunMessage) bool {
	return strings.TrimSpace(message.MessageType) != "status"
}

func hasExecutionHistoryMessages(messages []model.AgentRunMessage) bool {
	for _, message := range messages {
		if shouldIncludeRunMessageInExecutionHistory(message) {
			return true
		}
	}
	return false
}

func (a *AgentRunActivities) ensureRunBootstrapStatusMessage(ctx context.Context, run *model.AgentRun, content string) error {
	if a.runMessageRepo == nil || run == nil || strings.TrimSpace(content) == "" {
		return nil
	}
	messages, err := a.runMessageRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	if len(messages) > 0 {
		return nil
	}
	_, err = a.createRunMessage(ctx, run, "assistant", "status", strings.TrimSpace(content), nil, nil, nil, nil)
	return err
}

func (a *AgentRunActivities) persistAssistantRunMessage(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext) (*model.AgentRunMessage, error) {
	if a.runMessageRepo == nil || execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil, nil
	}
	result := execCtx.LastExecutionResult
	snapshot, err := a.loadCodingSessionStreamSnapshot(ctx, state.run)
	if err != nil {
		return nil, err
	}
	assistantMessageInput, err := buildPersistedAssistantRunMessage(result, snapshot)
	if err != nil {
		return nil, err
	}
	if assistantMessageInput == nil {
		return nil, nil
	}

	assistantMessage, err := a.createRunMessage(
		ctx,
		state.run,
		assistantMessageInput.Role,
		assistantMessageInput.MessageType,
		assistantMessageInput.Content,
		assistantMessageInput.ContentBlocks,
		assistantMessageInput.TurnSegments,
		assistantMessageInput.ToolInvocations,
		assistantMessageInput.TokenUsage,
	)
	if err != nil {
		return nil, err
	}
	if err := a.persistProviderResponseCheckpoint(ctx, state, result, assistantMessage); err != nil {
		return nil, err
	}
	if err := a.persistHumanInteractionArtifacts(ctx, state, result, assistantMessage); err != nil {
		return nil, err
	}

	for _, toolMessage := range buildPersistedToolResultMessages(result.Messages) {
		if _, err := a.createRunMessage(ctx, state.run, toolMessage.Role, toolMessage.MessageType, toolMessage.Content, toolMessage.ContentBlocks, nil, nil, nil); err != nil {
			return nil, err
		}
	}

	return assistantMessage, nil
}

type persistedRunMessageInput struct {
	Role            string
	MessageType     string
	Content         string
	ContentBlocks   json.RawMessage
	TurnSegments    json.RawMessage
	ToolInvocations json.RawMessage
	TokenUsage      json.RawMessage
}

func buildPersistedAssistantRunMessage(result *workerpkg.ExecutionResult, snapshot *model.CodingSessionStreamSnapshot) (*persistedRunMessageInput, error) {
	if result == nil {
		return nil, nil
	}
	content := persistedMessageContent(strings.TrimSpace(result.AssistantText), result.AssistantBlocks)
	if content == "" && len(result.AssistantBlocks) == 0 {
		return nil, nil
	}

	blocks, err := marshalExecutionBlocks(result.AssistantBlocks)
	if err != nil {
		return nil, fmt.Errorf("marshal assistant blocks: %w", err)
	}
	turnSegments, err := marshalCodingSessionTurnSegments(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal turn segments: %w", err)
	}
	invocations, err := marshalToolInvocations(result.ToolInvocations)
	if err != nil {
		return nil, fmt.Errorf("marshal tool invocations: %w", err)
	}
	usagePayload, err := marshalTokenUsage(result.Usage)
	if err != nil {
		return nil, fmt.Errorf("marshal usage: %w", err)
	}

	return &persistedRunMessageInput{
		Role:            "assistant",
		MessageType:     "assistant_turn",
		Content:         content,
		ContentBlocks:   blocks,
		TurnSegments:    turnSegments,
		ToolInvocations: invocations,
		TokenUsage:      usagePayload,
	}, nil
}

func buildPersistedToolResultMessages(messages []workerpkg.ExecutionMessage) []persistedRunMessageInput {
	toolMessages := finalRoundToolMessages(messages)
	results := make([]persistedRunMessageInput, 0, len(toolMessages))
	for _, toolMessage := range toolMessages {
		results = append(results, persistedRunMessageInput{
			Role:          "tool",
			MessageType:   "tool_result",
			Content:       persistedMessageContent(strings.TrimSpace(toolMessage.Content), toolMessage.Blocks),
			ContentBlocks: mustMarshalExecutionBlocks(toolMessage.Blocks),
		})
	}
	return results
}

func buildAssistantSequenceArtifactMetadata(sequenceNo int) json.RawMessage {
	if sequenceNo <= 0 {
		return json.RawMessage(`{}`)
	}
	payload, err := json.Marshal(map[string]any{
		"assistant_message_sequence_no": sequenceNo,
	})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func mergeArtifactMetadata(parts ...json.RawMessage) json.RawMessage {
	merged := map[string]any{}
	for _, part := range parts {
		if len(part) == 0 || string(part) == "null" {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(part, &decoded); err != nil {
			continue
		}
		for key, value := range decoded {
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return json.RawMessage(`{}`)
	}
	payload, err := json.Marshal(merged)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func marshalExecutionBlocks(blocks []workerpkg.ExecutionBlock) (json.RawMessage, error) {
	if len(blocks) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(workerpkg.NormalizeExecutionBlocks(blocks))
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func marshalCodingSessionTurnSegments(snapshot *model.CodingSessionStreamSnapshot) (json.RawMessage, error) {
	if snapshot == nil || len(snapshot.LiveTurnSegments) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(snapshot.LiveTurnSegments)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func mustMarshalExecutionBlocks(blocks []workerpkg.ExecutionBlock) json.RawMessage {
	payload, err := marshalExecutionBlocks(blocks)
	if err != nil {
		return nil
	}
	return payload
}

func marshalToolInvocations(invocations []model.ToolInvocation) (json.RawMessage, error) {
	if len(invocations) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(invocations)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func marshalTokenUsage(usage workerpkg.ExecutionUsage) (json.RawMessage, error) {
	return json.Marshal(map[string]int{
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
	})
}

func persistedMessageContent(fallback string, blocks []workerpkg.ExecutionBlock) string {
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	text := strings.TrimSpace(workerpkg.ExtractPersistedContentFromExecutionBlocks(blocks))
	if text != "" {
		return text
	}
	return ""
}

func finalRoundToolMessages(messages []workerpkg.ExecutionMessage) []workerpkg.ExecutionMessage {
	lastAssistantIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			lastAssistantIndex = i
			break
		}
	}
	if lastAssistantIndex == -1 || lastAssistantIndex >= len(messages)-1 {
		return nil
	}

	results := make([]workerpkg.ExecutionMessage, 0, len(messages)-lastAssistantIndex-1)
	for _, message := range messages[lastAssistantIndex+1:] {
		if message.Role != "tool" {
			continue
		}
		results = append(results, message)
	}
	return results
}

func (a *AgentRunActivities) persistProviderResponseCheckpoint(ctx context.Context, state *resolvedRunState, result *workerpkg.ExecutionResult, assistantMessage *model.AgentRunMessage) error {
	if a.artifactRepo == nil || state == nil || state.run == nil || state.agent == nil || result == nil || assistantMessage == nil || result.ProviderContinuation == nil {
		return nil
	}
	if strings.TrimSpace(result.ProviderContinuation.ResponseID) == "" {
		return nil
	}

	provider := strings.TrimSpace(derefString(state.agent.Provider))
	if provider == "" {
		provider = strings.TrimSpace(result.ProviderContinuation.Provider)
	}

	_, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeProviderResponseCheckpoint, "json", model.ProviderResponseCheckpoint{
		Provider:              provider,
		ResponseID:            strings.TrimSpace(result.ProviderContinuation.ResponseID),
		PreviousResponseID:    strings.TrimSpace(result.ProviderContinuation.PreviousResponseID),
		AssistantMessageSeqNo: assistantMessage.SequenceNo,
		RecordedAt:            time.Now().UTC(),
	}, buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo))
	if err == nil {
		slog.InfoContext(ctx, "saved provider continuation checkpoint",
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"provider", provider,
			"response_id", strings.TrimSpace(result.ProviderContinuation.ResponseID),
			"assistant_sequence_no", assistantMessage.SequenceNo,
		)
	}
	return err
}

func (a *AgentRunActivities) persistHumanInteractionArtifacts(ctx context.Context, state *resolvedRunState, result *workerpkg.ExecutionResult, assistantMessage *model.AgentRunMessage) error {
	if (a.artifactRepo == nil && a.interactionRepo == nil) || state == nil || state.run == nil || result == nil || assistantMessage == nil {
		return nil
	}

	metadata := buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo)

	if inputRequest := latestHumanInputRequestFromResult(result); inputRequest != nil {
		inputMetadata := mergeArtifactMetadata(metadata, result.HumanInputMetadata)
		artifactPayload := humanInputArtifactFromWorker(inputRequest)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeHumanInputRequest, "json", artifactPayload, inputMetadata); err != nil {
				return err
			}
		}
		interaction, err := a.persistHumanInputInteraction(ctx, state, inputRequest, inputMetadata, assistantMessage.SequenceNo)
		if err != nil {
			return err
		}
		if interaction != nil {
			a.publishCodingSessionInteractionEvent(state.run, interaction)
		} else {
			a.publishCodingSessionEvent(state.run, "input.requested", map[string]any{
				"content": artifactPayload,
			})
		}
	}

	if approvalRequest := latestHumanApprovalRequestFromResult(result); approvalRequest != nil {
		approvalMetadata := mergeArtifactMetadata(metadata, result.HumanApprovalMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeHumanApprovalRequest, "json", approvalRequest, approvalMetadata); err != nil {
				return err
			}
		}
		interaction, err := a.persistHumanApprovalInteraction(ctx, state, approvalRequest, approvalMetadata, assistantMessage.SequenceNo)
		if err != nil {
			return err
		}
		if interaction != nil {
			a.publishCodingSessionInteractionEvent(state.run, interaction)
		} else {
			a.publishCodingSessionEvent(state.run, "approval.requested", map[string]any{
				"content": approvalRequest,
			})
		}
	}

	if authState := latestCodexAuthStateFromResult(result); authState != nil {
		authMetadata := mergeArtifactMetadata(metadata, result.CodexAuthMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeCodexAuthState, "json", authState, authMetadata); err != nil {
				return err
			}
		}
		a.publishCodingSessionEvent(state.run, "auth.updated", map[string]any{
			"content": authState,
		})
	}

	if runPlan := latestRunPlanFromResult(result); runPlan != nil {
		runPlanMetadata := mergeArtifactMetadata(metadata, result.RunPlanMetadata)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeRunPlan, "json", runPlan, runPlanMetadata); err != nil {
				return err
			}
		}
		a.publishCodingSessionEvent(state.run, "activity.updated", map[string]any{
			"content": runPlan,
		})
	}

	return nil
}

type interactionRuntimeMetadata struct {
	AssistantMessageSequenceNo int             `json:"assistant_message_sequence_no,omitempty"`
	RuntimeKind                string          `json:"runtime_kind,omitempty"`
	CodexRequestKind           string          `json:"codex_request_kind,omitempty"`
	CodexRequestID             string          `json:"codex_request_id,omitempty"`
	CodexThreadID              string          `json:"codex_thread_id,omitempty"`
	CodexTurnID                string          `json:"codex_turn_id,omitempty"`
	CodexItemID                string          `json:"codex_item_id,omitempty"`
	CodexApprovalID            *string         `json:"codex_approval_id,omitempty"`
	CodexRequestPayload        json.RawMessage `json:"codex_request_payload,omitempty"`
}

func (a *AgentRunActivities) persistHumanInputInteraction(ctx context.Context, state *resolvedRunState, inputRequest *workerpkg.UserInputRequest, metadata json.RawMessage, assistantSequenceNo int) (*model.AgentRunInteraction, error) {
	if a.interactionRepo == nil || state == nil || state.run == nil || inputRequest == nil {
		return nil, nil
	}

	runtimeMetadata := decodeInteractionRuntimeMetadata(metadata)
	interaction := &model.AgentRunInteraction{
		WorkspaceID:                state.run.WorkspaceID,
		RunID:                      state.run.ID,
		RuntimeKind:                firstNonEmptyString(strings.TrimSpace(runtimeMetadata.RuntimeKind), executionRuntimeKind(state), strings.TrimSpace(state.run.RuntimeKind)),
		InteractionKind:            model.AgentRunInteractionKindRequestUserInput,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionCodexV2,
		RequestPayload:             mustMarshalJSON(inputRequest),
		RuntimeMetadata:            defaultInteractionRuntimeMetadata(metadata),
		AssistantMessageSequenceNo: intPtrIfPositive(firstPositiveInt(runtimeMetadata.AssistantMessageSequenceNo, assistantSequenceNo)),
		Title:                      strPtr("User input required"),
	}

	if interaction.RuntimeKind == "codex" && len(runtimeMetadata.CodexRequestPayload) > 0 {
		interaction.RequestSchemaVersion = model.AgentRunInteractionSchemaVersionCodexV2
		interaction.RequestPayload = copyRawJSON(runtimeMetadata.CodexRequestPayload)
		interaction.RequestID = strPtrIfNotEmpty(runtimeMetadata.CodexRequestID)
		interaction.ThreadID = strPtrIfNotEmpty(runtimeMetadata.CodexThreadID)
		interaction.TurnID = strPtrIfNotEmpty(runtimeMetadata.CodexTurnID)
		interaction.ItemID = strPtrIfNotEmpty(runtimeMetadata.CodexItemID)
	}

	interaction.Summary = strPtrIfNotEmpty(workerpkg.UserInputSummary(inputRequest))

	if err := a.appendRunInteraction(ctx, interaction); err != nil {
		return nil, err
	}
	return interaction, nil
}

func (a *AgentRunActivities) persistHumanApprovalInteraction(ctx context.Context, state *resolvedRunState, approvalRequest *model.ApprovalRequest, metadata json.RawMessage, assistantSequenceNo int) (*model.AgentRunInteraction, error) {
	if a.interactionRepo == nil || state == nil || state.run == nil || approvalRequest == nil {
		return nil, nil
	}

	runtimeMetadata := decodeInteractionRuntimeMetadata(metadata)
	interaction := &model.AgentRunInteraction{
		WorkspaceID:                state.run.WorkspaceID,
		RunID:                      state.run.ID,
		RuntimeKind:                firstNonEmptyString(strings.TrimSpace(runtimeMetadata.RuntimeKind), executionRuntimeKind(state), strings.TrimSpace(state.run.RuntimeKind)),
		InteractionKind:            model.AgentRunInteractionKindReviewCheckpoint,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		RequestPayload:             mustMarshalJSON(approvalRequest),
		RuntimeMetadata:            defaultInteractionRuntimeMetadata(metadata),
		AssistantMessageSequenceNo: intPtrIfPositive(firstPositiveInt(runtimeMetadata.AssistantMessageSequenceNo, assistantSequenceNo)),
		Title:                      strPtrIfNotEmpty(strings.TrimSpace(approvalRequest.Title)),
		Summary:                    strPtrIfNotEmpty(strings.TrimSpace(approvalRequest.Summary)),
	}

	if interaction.RuntimeKind == "codex" && len(runtimeMetadata.CodexRequestPayload) > 0 {
		interaction.RequestSchemaVersion = model.AgentRunInteractionSchemaVersionCodexV2
		interaction.RequestPayload = copyRawJSON(runtimeMetadata.CodexRequestPayload)
		interaction.RequestID = strPtrIfNotEmpty(runtimeMetadata.CodexRequestID)
		interaction.ThreadID = strPtrIfNotEmpty(runtimeMetadata.CodexThreadID)
		interaction.TurnID = strPtrIfNotEmpty(runtimeMetadata.CodexTurnID)
		interaction.ItemID = strPtrIfNotEmpty(runtimeMetadata.CodexItemID)
		interaction.ApprovalID = runtimeMetadata.CodexApprovalID
		switch strings.TrimSpace(runtimeMetadata.CodexRequestKind) {
		case "command_execution":
			interaction.InteractionKind = model.AgentRunInteractionKindCommandExecutionApproval
		case "file_change":
			interaction.InteractionKind = model.AgentRunInteractionKindFileChangeApproval
		case "permissions":
			interaction.InteractionKind = model.AgentRunInteractionKindPermissionsApproval
		}
	}

	if err := a.appendRunInteraction(ctx, interaction); err != nil {
		return nil, err
	}
	return interaction, nil
}

func (a *AgentRunActivities) appendRunInteraction(ctx context.Context, interaction *model.AgentRunInteraction) error {
	if a.interactionRepo == nil || interaction == nil {
		return nil
	}
	if len(interaction.RequestPayload) == 0 {
		interaction.RequestPayload = json.RawMessage(`{}`)
	}
	if len(interaction.RuntimeMetadata) == 0 {
		interaction.RuntimeMetadata = json.RawMessage(`{}`)
	}
	if strings.TrimSpace(interaction.Status) == "" {
		interaction.Status = model.AgentRunInteractionStatusPending
	}
	if strings.TrimSpace(interaction.RequestSchemaVersion) == "" {
		interaction.RequestSchemaVersion = model.AgentRunInteractionSchemaVersionHelpinV1
	}
	return a.interactionRepo.Create(ctx, interaction)
}

func decodeInteractionRuntimeMetadata(raw json.RawMessage) interactionRuntimeMetadata {
	var metadata interactionRuntimeMetadata
	if len(raw) == 0 || string(raw) == "null" {
		return metadata
	}
	_ = json.Unmarshal(raw, &metadata)
	return metadata
}

func defaultInteractionRuntimeMetadata(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	return copyRawJSON(raw)
}

func mustMarshalJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func copyRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func intPtrIfPositive(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func strPtrIfNotEmpty(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func latestHumanApprovalRequestFromResult(result *workerpkg.ExecutionResult) *model.ApprovalRequest {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestHumanApprovalRequest(result.ToolInvocations)
}

func latestHumanInputRequestFromResult(result *workerpkg.ExecutionResult) *workerpkg.UserInputRequest {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestHumanInputRequest(result.ToolInvocations)
}

func latestCodexAuthStateFromResult(result *workerpkg.ExecutionResult) *model.CodexAuthState {
	if result == nil || result.CodexAuthState == nil {
		return nil
	}
	return result.CodexAuthState
}

func latestRunPlanFromResult(result *workerpkg.ExecutionResult) *workerpkg.RunPlanArtifact {
	if result == nil {
		return nil
	}
	return workerpkg.ExtractLatestRunPlan(result.ToolInvocations)
}

func humanInputArtifactFromWorker(req *workerpkg.UserInputRequest) model.HumanInputArtifact {
	return workerpkg.HumanInputArtifactFromUserInputRequest(req)
}

func (a *AgentRunActivities) createRunMessage(ctx context.Context, run *model.AgentRun, role, messageType, content string, blocks, turnSegments, toolInvocations, tokenUsage json.RawMessage) (*model.AgentRunMessage, error) {
	if a.runMessageRepo == nil {
		return nil, nil
	}
	sequenceNo, err := a.runMessageRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	message := &model.AgentRunMessage{
		WorkspaceID:     run.WorkspaceID,
		RunID:           run.ID,
		Role:            role,
		Content:         content,
		MessageType:     messageType,
		ContentBlocks:   blocks,
		TurnSegments:    turnSegments,
		ToolInvocations: toolInvocations,
		TokenUsage:      tokenUsage,
		SequenceNo:      sequenceNo,
	}
	if err := a.runMessageRepo.Create(ctx, message); err != nil {
		return nil, err
	}
	if shouldClearCodingSessionStreamSnapshot(role, messageType) {
		if err := a.clearCodingSessionStreamSnapshot(ctx, run); err != nil {
			slog.WarnContext(ctx, "clear coding session stream snapshot failed",
				"run_id", run.ID,
				"workspace_id", run.WorkspaceID,
				"error", err,
			)
		}
	}
	a.publishRunMessageEvent(run, message)
	return message, nil
}

func (a *AgentRunActivities) publishRunMessageEvent(run *model.AgentRun, message *model.AgentRunMessage) {
	if a.wsPublisher == nil || run == nil || message == nil {
		return
	}
	data, _ := json.Marshal(message)
	a.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "agent_run_message",
		EntityID:    message.ID,
		WorkspaceID: run.WorkspaceID,
		ParentType:  "agent_run",
		ParentID:    run.ID,
		Data:        data,
	})
	eventType := "user.message.completed"
	switch strings.TrimSpace(message.Role) {
	case "assistant":
		eventType = "assistant.message.completed"
	case "tool":
		eventType = "tool.call.completed"
	}
	a.publishCodingSessionEvent(run, eventType, map[string]any{
		"message_id":       message.ID,
		"role":             message.Role,
		"message_type":     message.MessageType,
		"content":          message.Content,
		"sequence_no":      message.SequenceNo,
		"content_blocks":   json.RawMessage(message.ContentBlocks),
		"turn_segments":    json.RawMessage(message.TurnSegments),
		"tool_invocations": json.RawMessage(message.ToolInvocations),
	})
}

func (a *AgentRunActivities) publishRunStreamEvent(run *model.AgentRun, event workerpkg.ExecutionEvent) {
	if run == nil {
		return
	}

	sentAt := time.Now().UTC()
	codingEventType := codingSessionEventTypeFromExecutionEvent(event)
	codingPayload := map[string]any{
		"message_id":        strings.TrimSpace(event.MessageID),
		"parent_message_id": strings.TrimSpace(event.ParentMessageID),
		"result_message_id": strings.TrimSpace(event.ResultMessageID),
		"text":              event.Text,
		"content":           coalesceRaw(event.Content, event.Text),
		"tool_call_id":      event.ToolCallID,
		"tool_name":         event.ToolName,
		"tool_input":        event.ToolInput,
		"args_delta":        event.ArgsDelta,
		"args_text":         event.ArgsText,
		"activity_id":       event.ActivityID,
		"activity_type":     event.ActivityType,
		"encrypted_value":   event.EncryptedValue,
		"output_summary":    event.OutputSummary,
		"duration_ms":       event.DurationMs,
		"error":             event.Error,
	}
	if codingEventType != "" {
		a.persistCodingSessionStreamSnapshot(context.Background(), run, codingEventType, codingPayload, sentAt)
	}
	if a.wsPublisher == nil {
		return
	}

	payload, err := json.Marshal(model.AgentRunStreamEvent{
		SentAt:          sentAt,
		Type:            event.Type,
		RunID:           run.ID,
		MessageID:       event.MessageID,
		ParentMessageID: event.ParentMessageID,
		ResultMessageID: event.ResultMessageID,
		Text:            event.Text,
		Content:         event.Content,
		ToolCallID:      event.ToolCallID,
		ToolName:        event.ToolName,
		ToolInput:       event.ToolInput,
		ArgsDelta:       event.ArgsDelta,
		ArgsText:        event.ArgsText,
		ActivityID:      event.ActivityID,
		ActivityType:    event.ActivityType,
		EncryptedValue:  event.EncryptedValue,
		OutputSummary:   event.OutputSummary,
		DurationMs:      event.DurationMs,
		Error:           event.Error,
	})
	if err != nil {
		return
	}

	a.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "agent_run_stream",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
		ParentType:  "agent_run",
		ParentID:    run.ID,
		Data:        payload,
	})
	if codingEventType != "" {
		a.publishCodingSessionEvent(run, codingEventType, codingPayload)
	}
}

func shouldClearCodingSessionStreamSnapshot(role, messageType string) bool {
	return strings.TrimSpace(role) == "assistant" && strings.TrimSpace(messageType) == "assistant_turn"
}

func (a *AgentRunActivities) clearCodingSessionStreamSnapshot(ctx context.Context, run *model.AgentRun) error {
	if a == nil || a.sessionSnapshotRepo == nil || run == nil {
		return nil
	}
	return a.sessionSnapshotRepo.DeleteByRun(ctx, run.WorkspaceID, run.ID)
}

func (a *AgentRunActivities) loadCodingSessionStreamSnapshot(ctx context.Context, run *model.AgentRun) (*model.CodingSessionStreamSnapshot, error) {
	if a == nil || a.sessionSnapshotRepo == nil || run == nil {
		return nil, nil
	}
	record, err := a.sessionSnapshotRepo.GetByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil || record == nil {
		return nil, err
	}
	return model.DecodeCodingSessionStreamSnapshot(record.SnapshotPayload)
}

func (a *AgentRunActivities) persistCodingSessionStreamSnapshot(ctx context.Context, run *model.AgentRun, eventType string, payload map[string]any, timestamp time.Time) {
	if a == nil || a.sessionSnapshotRepo == nil || run == nil || strings.TrimSpace(eventType) == "" {
		return
	}

	record, err := a.sessionSnapshotRepo.GetByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		slog.WarnContext(ctx, "load coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"error", err,
		)
		return
	}

	var snapshot *model.CodingSessionStreamSnapshot
	if record != nil {
		snapshot, err = model.DecodeCodingSessionStreamSnapshot(record.SnapshotPayload)
		if err != nil {
			slog.WarnContext(ctx, "decode coding session stream snapshot failed",
				"run_id", run.ID,
				"workspace_id", run.WorkspaceID,
				"error", err,
			)
			record = nil
		}
	}

	snapshot = model.ApplyCodingSessionStreamEvent(snapshot, eventType, payload, timestamp)
	if snapshot == nil || snapshot.IsEmpty() {
		if record != nil {
			if err := a.sessionSnapshotRepo.DeleteByRun(ctx, run.WorkspaceID, run.ID); err != nil {
				slog.WarnContext(ctx, "delete empty coding session stream snapshot failed",
					"run_id", run.ID,
					"workspace_id", run.WorkspaceID,
					"error", err,
				)
			}
		}
		return
	}

	encoded, err := model.EncodeCodingSessionStreamSnapshot(snapshot)
	if err != nil {
		slog.WarnContext(ctx, "encode coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"error", err,
		)
		return
	}

	nextRecord := &model.CodingSessionStateSnapshot{
		WorkspaceID:     run.WorkspaceID,
		RunID:           run.ID,
		SchemaVersion:   model.CodingSessionStateSnapshotSchemaVersionV1,
		SnapshotPayload: encoded,
	}
	if record != nil {
		nextRecord.ID = record.ID
		nextRecord.CreatedAt = record.CreatedAt
	}
	if err := a.sessionSnapshotRepo.Upsert(ctx, nextRecord); err != nil {
		slog.WarnContext(ctx, "persist coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"error", err,
		)
	}
}

func (a *AgentRunActivities) publishCodingSessionEvent(run *model.AgentRun, eventType string, payload map[string]any) {
	if a.wsPublisher == nil || run == nil || strings.TrimSpace(eventType) == "" {
		return
	}
	envelope, _ := json.Marshal(model.CodingSessionEvent{
		ID:          fmt.Sprintf("%s:%d", run.ID, time.Now().UTC().UnixNano()),
		SessionID:   run.ID,
		RunID:       run.ID,
		SequenceNo:  int(time.Now().UTC().UnixMilli()),
		Timestamp:   time.Now().UTC(),
		Type:        eventType,
		RuntimeKind: run.RuntimeKind,
		Payload:     payload,
	})
	a.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "coding_session_event",
		EntityID:    fmt.Sprintf("%s:%d", run.ID, time.Now().UTC().UnixNano()),
		WorkspaceID: run.WorkspaceID,
		ParentType:  "coding_session",
		ParentID:    run.ID,
		Data:        envelope,
	})
}

func (a *AgentRunActivities) publishCodingSessionInteractionEvent(run *model.AgentRun, interaction *model.AgentRunInteraction) {
	if interaction == nil {
		return
	}
	eventType, payload := codingSessionInteractionEventPayload(interaction)
	a.publishCodingSessionEvent(run, eventType, payload)
}

func codingSessionInteractionEventPayload(interaction *model.AgentRunInteraction) (string, map[string]any) {
	if interaction == nil {
		return "", nil
	}

	eventType := "interaction.updated"
	switch strings.TrimSpace(interaction.Status) {
	case model.AgentRunInteractionStatusPending:
		eventType = "interaction.requested"
	case model.AgentRunInteractionStatusResolved:
		eventType = "interaction.resolved"
	case model.AgentRunInteractionStatusCancelled:
		eventType = "interaction.cancelled"
	}

	payload := map[string]any{
		"interaction_id":         interaction.ID,
		"interaction_kind":       interaction.InteractionKind,
		"status":                 interaction.Status,
		"request_schema_version": interaction.RequestSchemaVersion,
		"request_payload":        json.RawMessage(interaction.RequestPayload),
		"title":                  derefString(interaction.Title),
		"summary":                derefString(interaction.Summary),
		"request_id":             derefString(interaction.RequestID),
		"thread_id":              derefString(interaction.ThreadID),
		"turn_id":                derefString(interaction.TurnID),
		"item_id":                derefString(interaction.ItemID),
		"approval_id":            derefString(interaction.ApprovalID),
	}
	if interaction.AssistantMessageSequenceNo != nil {
		payload["assistant_message_sequence_no"] = *interaction.AssistantMessageSequenceNo
	}
	if interaction.ResponseSchemaVersion != nil && strings.TrimSpace(*interaction.ResponseSchemaVersion) != "" {
		payload["response_schema_version"] = strings.TrimSpace(*interaction.ResponseSchemaVersion)
	}
	if len(interaction.ResponsePayload) > 0 && string(interaction.ResponsePayload) != "null" {
		payload["response_payload"] = json.RawMessage(interaction.ResponsePayload)
	}
	if interaction.ResolvedAt != nil {
		payload["resolved_at"] = interaction.ResolvedAt.UTC()
	}
	if interaction.ResolvedBy != nil && strings.TrimSpace(*interaction.ResolvedBy) != "" {
		payload["resolved_by"] = strings.TrimSpace(*interaction.ResolvedBy)
	}

	return eventType, payload
}

func codingSessionEventTypeFromExecutionEvent(event workerpkg.ExecutionEvent) string {
	switch strings.TrimSpace(event.Type) {
	case "assistant_message_started":
		return "assistant.message.started"
	case "assistant_message_delta":
		return "assistant.message.delta"
	case "assistant_message_completed":
		return "assistant.message.completed"
	case "reasoning_message_started":
		return "reasoning.message.started"
	case "reasoning_message_delta":
		return "reasoning.message.delta"
	case "reasoning_message_completed":
		return "reasoning.message.completed"
	case "tool_call_started":
		return "tool.call.started"
	case "tool_call_args_delta":
		return "tool.call.args.delta"
	case "tool_call_result":
		return "tool.call.result"
	case "tool_call_finished":
		if strings.TrimSpace(event.Error) != "" {
			return "tool.call.failed"
		}
		return "tool.call.completed"
	case "activity_snapshot":
		return "activity.snapshot"
	case "activity_delta":
		return "activity.delta"
	case "plan_updated":
		return "plan.updated"
	default:
		return ""
	}
}

func latestExecutionApprovalRequest(execCtx *workerpkg.ExecutionContext) *model.ApprovalRequest {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return workerpkg.ExtractLatestHumanApprovalRequest(execCtx.LastExecutionResult.ToolInvocations)
}

func latestExecutionHumanInputRequest(execCtx *workerpkg.ExecutionContext) *workerpkg.UserInputRequest {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return workerpkg.ExtractLatestHumanInputRequest(execCtx.LastExecutionResult.ToolInvocations)
}

func latestExecutionCodexAuthState(execCtx *workerpkg.ExecutionContext) *model.CodexAuthState {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return execCtx.LastExecutionResult.CodexAuthState
}

func (a *AgentRunActivities) handleLiveCodexInteractivePause(
	ctx context.Context,
	state *resolvedRunState,
	execCtx *workerpkg.ExecutionContext,
	result *workerpkg.ExecutionResult,
	setHeartbeatStage func(string),
) (*workerpkg.LiveExecutionResumeSignal, error) {
	if state == nil || state.run == nil || execCtx == nil || result == nil {
		return nil, fmt.Errorf("live codex pause context is incomplete")
	}

	pauseReason, pauseStage, err := liveCodexPauseState(result)
	if err != nil {
		return nil, err
	}

	execCtx.LastExecutionResult = result
	assistantMessage, err := a.persistAssistantRunMessage(ctx, state, execCtx)
	if err != nil {
		return nil, err
	}
	afterSequenceNo, err := a.currentRunMessageSequence(ctx, state.run)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	state.run.Status = model.AgentRunStatusPaused
	state.run.PauseReason = pauseReason
	state.run.ExecutionStage = strPtr(pauseStage)
	state.run.LastHeartbeatAt = &now
	state.run.CompletedAt = nil
	if pauseReason == model.AgentRunPauseReasonHumanApproval {
		state.run.ApprovalState = "pending"
	} else if state.run.ApprovalState != "approved" {
		state.run.ApprovalState = "not_required"
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return nil, err
	}
	a.runRepo.Notify(ctx, state.run)
	if setHeartbeatStage != nil {
		setHeartbeatStage(pauseStage)
	}

	if assistantMessage != nil && assistantMessage.SequenceNo > afterSequenceNo {
		afterSequenceNo = assistantMessage.SequenceNo
	}
	signal, err := a.waitForLiveCodexResumeSignal(ctx, state.run, afterSequenceNo, pauseReason)
	if err != nil {
		return nil, err
	}
	return &workerpkg.LiveExecutionResumeSignal{
		Intent:          signal.Intent,
		Content:         signal.Content,
		ResponsePayload: copyRawJSON(signal.ResponsePayload),
		Acknowledge: func() error {
			if setHeartbeatStage != nil {
				setHeartbeatStage("codex_running")
			}
			now := time.Now()
			state.run.Status = model.AgentRunStatusRunning
			state.run.PauseReason = model.AgentRunPauseReasonNone
			state.run.ExecutionStage = strPtr(liveCodexResumeStage(signal, pauseReason))
			state.run.LastHeartbeatAt = &now
			state.run.CompletedAt = nil
			switch strings.TrimSpace(signal.Intent) {
			case model.AgentRunResumeIntentApprove:
				state.run.ApprovalState = "approved"
			case model.AgentRunResumeIntentRequestChanges:
				state.run.ApprovalState = "rejected"
			default:
				if pauseReason != model.AgentRunPauseReasonHumanApproval && state.run.ApprovalState != "pending" && state.run.ApprovalState != "rejected" {
					state.run.ApprovalState = "not_required"
				}
			}
			if err := a.runRepo.Update(ctx, state.run); err != nil {
				return err
			}
			a.runRepo.Notify(ctx, state.run)
			return nil
		},
	}, nil
}

func (a *AgentRunActivities) currentRunMessageSequence(ctx context.Context, run *model.AgentRun) (int, error) {
	if a == nil || a.runMessageRepo == nil || run == nil {
		return 0, nil
	}
	nextSequenceNo, err := a.runMessageRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return 0, err
	}
	if nextSequenceNo <= 1 {
		return 0, nil
	}
	return nextSequenceNo - 1, nil
}

func liveCodexPauseState(result *workerpkg.ExecutionResult) (pauseReason, stage string, err error) {
	if approvalRequest := latestHumanApprovalRequestFromResult(result); approvalRequest != nil {
		stage = "awaiting_approval"
		if strings.TrimSpace(approvalRequest.Phase) != "" {
			stage = strings.TrimSpace(approvalRequest.Phase)
		}
		return model.AgentRunPauseReasonHumanApproval, stage, nil
	}
	if latestHumanInputRequestFromResult(result) != nil {
		return model.AgentRunPauseReasonHumanInput, "awaiting_input", nil
	}
	return "", "", fmt.Errorf("execution result did not include a live human interaction request")
}

func liveCodexResumeStage(signal *workerpkg.LiveExecutionResumeSignal, pauseReason string) string {
	if signal == nil {
		return "resuming"
	}
	switch strings.TrimSpace(signal.Intent) {
	case model.AgentRunResumeIntentApprove:
		return "approved"
	case model.AgentRunResumeIntentRequestChanges:
		return "feedback_received"
	case model.AgentRunResumeIntentReply:
		if pauseReason == model.AgentRunPauseReasonHumanApproval {
			return "feedback_received"
		}
		return "input_received"
	default:
		return "resuming"
	}
}

func (a *AgentRunActivities) waitForLiveCodexResumeSignal(ctx context.Context, run *model.AgentRun, afterSequenceNo int, pauseReason string) (*workerpkg.LiveExecutionResumeSignal, error) {
	ticker := time.NewTicker(codexLivePausePollEvery)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			return nil, workerpkg.ErrRunCancelled
		}

		currentRun, err := a.runRepo.GetByIDAny(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		if currentRun == nil {
			return nil, fmt.Errorf("agent run %s was not found while waiting for live codex input", run.ID)
		}
		model.NormalizeAgentRunPauseState(currentRun)

		switch currentRun.Status {
		case model.AgentRunStatusCancelled:
			return nil, workerpkg.ErrRunCancelled
		case model.AgentRunStatusFailed:
			return nil, fmt.Errorf("agent run failed while waiting for live codex input")
		case model.AgentRunStatusCompleted:
			return nil, fmt.Errorf("agent run completed while waiting for live codex input")
		}

		if signal, err := a.latestResolvedLiveCodexInteractionSignal(ctx, currentRun, afterSequenceNo); err != nil {
			return nil, err
		} else if signal != nil {
			return signal, nil
		}

		if pauseReason == model.AgentRunPauseReasonHumanApproval {
			switch strings.TrimSpace(currentRun.ApprovalState) {
			case "approved":
				content, err := a.latestLiveCodexUserMessage(ctx, currentRun, afterSequenceNo)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(content) == "" {
					content = "approve"
				}
				return &workerpkg.LiveExecutionResumeSignal{
					Intent:  model.AgentRunResumeIntentApprove,
					Content: content,
				}, nil
			case "rejected":
				content, err := a.latestLiveCodexUserMessage(ctx, currentRun, afterSequenceNo)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(content) == "" {
					return nil, fmt.Errorf("approval feedback message is missing for live codex resume")
				}
				return &workerpkg.LiveExecutionResumeSignal{
					Intent:  model.AgentRunResumeIntentRequestChanges,
					Content: content,
				}, nil
			}
		} else {
			content, err := a.latestLiveCodexUserMessage(ctx, currentRun, afterSequenceNo)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(content) != "" {
				return &workerpkg.LiveExecutionResumeSignal{
					Intent:  model.AgentRunResumeIntentReply,
					Content: content,
				}, nil
			}
		}

		select {
		case <-ctx.Done():
			return nil, workerpkg.ErrRunCancelled
		case <-ticker.C:
		}
	}
}

func (a *AgentRunActivities) latestResolvedLiveCodexInteractionSignal(ctx context.Context, run *model.AgentRun, afterSequenceNo int) (*workerpkg.LiveExecutionResumeSignal, error) {
	if a == nil || a.interactionRepo == nil || run == nil {
		return nil, nil
	}

	interaction, err := a.interactionRepo.GetLatestResolvedByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil || interaction == nil {
		return nil, err
	}
	if len(interaction.ResponsePayload) == 0 {
		return nil, nil
	}
	if interaction.AssistantMessageSequenceNo != nil && *interaction.AssistantMessageSequenceNo < afterSequenceNo {
		return nil, nil
	}

	followupInput, err := a.latestLiveCodexUserMessage(ctx, run, afterSequenceNo)
	if err != nil {
		return nil, err
	}
	return &workerpkg.LiveExecutionResumeSignal{
		Intent:          liveCodexResumeIntentForInteraction(interaction),
		Content:         strings.TrimSpace(followupInput),
		ResponsePayload: copyRawJSON(interaction.ResponsePayload),
	}, nil
}

func liveCodexResumeIntentForInteraction(interaction *model.AgentRunInteraction) string {
	if interaction == nil {
		return model.AgentRunResumeIntentReply
	}

	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindRequestUserInput:
		return model.AgentRunResumeIntentReply
	case model.AgentRunInteractionKindPermissionsApproval:
		var payload struct {
			Permissions map[string]any `json:"permissions"`
		}
		if err := json.Unmarshal(interaction.ResponsePayload, &payload); err == nil && len(payload.Permissions) > 0 {
			return model.AgentRunResumeIntentApprove
		}
		return model.AgentRunResumeIntentRequestChanges
	case model.AgentRunInteractionKindCommandExecutionApproval, model.AgentRunInteractionKindFileChangeApproval:
		var payload struct {
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(interaction.ResponsePayload, &payload); err == nil {
			switch strings.TrimSpace(payload.Decision) {
			case "accept", "acceptForSession", "acceptWithExecpolicyAmendment", "applyNetworkPolicyAmendment":
				return model.AgentRunResumeIntentApprove
			}
		}
		return model.AgentRunResumeIntentRequestChanges
	default:
		return model.AgentRunResumeIntentRequestChanges
	}
}

func (a *AgentRunActivities) latestLiveCodexUserMessage(ctx context.Context, run *model.AgentRun, afterSequenceNo int) (string, error) {
	if a.runMessageRepo == nil || run == nil {
		return "", nil
	}
	messages, err := a.runMessageRepo.ListByRunAfterSequence(ctx, run.WorkspaceID, run.ID, afterSequenceNo)
	if err != nil {
		return "", err
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(messages[i].Role) != "user" {
			continue
		}
		content := strings.TrimSpace(messages[i].Content)
		if content != "" {
			return content, nil
		}
	}
	return "", nil
}

func (a *AgentRunActivities) captureTranscriptPlanningArtifacts(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext, assistantMessage *model.AgentRunMessage, _ planningRunInput) error {
	if state == nil || state.run == nil || execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	switch state.run.TargetType {
	case "epic":
		if state.epic == nil {
			return nil
		}
	case "story", "task":
		if state.task == nil {
			return nil
		}
	default:
		return nil
	}

	assistantSequenceNo := 0
	if assistantMessage != nil {
		assistantSequenceNo = assistantMessage.SequenceNo
	}
	previews := workerpkg.ExtractPublishedPreviews(execCtx.LastExecutionResult.ToolInvocations)
	for index, preview := range previews {
		if err := a.createRunArtifactWithMetadata(ctx, state.run, workerpkg.RunPreviewArtifactType, "json", map[string]any{
			"panel_key": preview.PanelKey,
			"title":     preview.Title,
			"format":    preview.Format,
			"content":   json.RawMessage(preview.Content),
			"replace":   preview.Replace,
		}, 999900+index, buildAssistantSequenceArtifactMetadata(assistantSequenceNo)); err != nil {
			return err
		}
	}

	return nil
}

func (a *AgentRunActivities) applyApprovedInteractivePreview(ctx context.Context, state *resolvedRunState, input *planningRunInput) (string, error) {
	if state == nil || state.run == nil || input == nil {
		return "", nil
	}
	if state.run.InvocationMode != model.InvocationModeInteractive {
		return "", nil
	}
	if a.artifactRepo == nil {
		return "", nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return "", err
	}
	approvedArtifact, approvedPreview, err := nextUnappliedApprovedPreview(artifacts)
	if err != nil || approvedArtifact == nil || approvedPreview == nil {
		return "", err
	}

	var appliedAction string
	switch strings.ToLower(strings.TrimSpace(approvedPreview.Phase)) {
	case "prd":
		if state.epic == nil || state.run.TargetType != "epic" {
			return "", fmt.Errorf("approved PRD preview requires an epic target")
		}
		if err := a.applyApprovedPRDPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "persist_prd"
	case "task_doc", "story_doc":
		if state.task == nil {
			return "", fmt.Errorf("approved task planning doc preview requires a task target")
		}
		if err := a.applyApprovedTaskDocPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "persist_task_doc"
	case "tasks", "stories":
		if state.epic == nil || state.run.TargetType != "epic" {
			return "", fmt.Errorf("approved task plan preview requires an epic target")
		}
		if err := a.applyApprovedTaskPlanPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "create_tasks"
	default:
		return "", fmt.Errorf("unsupported approved preview phase %q", approvedPreview.Phase)
	}

	if _, err := a.appendRunArtifact(ctx, state.run, model.AgentRunArtifactTypeApprovedPreviewApplied, "json", model.AppliedApprovedRunPreview{
		ApprovedArtifactID: approvedArtifact.ID,
		Phase:              strings.TrimSpace(approvedPreview.Phase),
		Action:             appliedAction,
		AppliedAt:          time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	return appliedAction, nil
}

func nextUnappliedApprovedPreview(artifacts []model.AgentRunArtifact) (*model.AgentRunArtifact, *model.ApprovedRunPreview, error) {
	applied := make(map[string]bool)
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeApprovedPreviewApplied || artifact.InlineContent == nil {
			continue
		}
		var marker model.AppliedApprovedRunPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &marker); err != nil {
			return nil, nil, fmt.Errorf("parse applied approved preview artifact: %w", err)
		}
		if strings.TrimSpace(marker.ApprovedArtifactID) != "" {
			applied[strings.TrimSpace(marker.ApprovedArtifactID)] = true
		}
	}

	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeApprovedPreview || artifact.InlineContent == nil {
			continue
		}
		if applied[artifact.ID] {
			continue
		}
		var preview model.ApprovedRunPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
			return nil, nil, fmt.Errorf("parse approved preview artifact: %w", err)
		}
		if approvedPreviewDebugEnabled() {
			slog.Info("selected approved preview for application",
				"artifact_id", artifact.ID,
				"sequence_no", artifact.SequenceNo,
				"phase", strings.TrimSpace(preview.Phase),
				"panel_key", strings.TrimSpace(preview.PanelKey),
				"format", strings.TrimSpace(preview.Format),
				"source_message_id", strings.TrimSpace(preview.SourceMessageID),
				"content_preview", previewDebugSnippet(preview.Content, 1600),
			)
		}
		return &artifact, &preview, nil
	}
	return nil, nil, nil
}

func decodeApprovedTaskPlanPreviewContent(raw json.RawMessage) (model.OrchestrationProposal, error) {
	var proposal model.OrchestrationProposal
	normalized, err := workerpkg.NormalizeTaskPlanPreviewContent(raw)
	if err != nil {
		if approvedPreviewDebugEnabled() {
			slog.Error("approved task plan preview normalization failed during apply",
				"raw_preview", previewDebugSnippet(raw, 1600),
				"error", err,
			)
		}
		return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}; legacy proposed_stories is still accepted")
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(normalized, &payload); err != nil {
		if approvedPreviewDebugEnabled() {
			slog.Error("approved task plan preview payload unmarshal failed during apply",
				"normalized_preview", previewDebugSnippet(normalized, 1600),
				"error", err,
			)
		}
		return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}; legacy proposed_stories is still accepted")
	}

	proposal.EpicID = decodeLooseJSONString(payload["epic_id"])
	proposal.Summary = decodeLooseJSONString(payload["summary"])
	proposal.SpecVersionID = decodeLooseJSONString(payload["spec_version_id"])
	proposal.OpenQuestions = decodeLooseJSONStringArray(payload["open_questions"])
	proposal.Risks = decodeLooseJSONStringArray(payload["risks"])
	if verticalCoverage, ok := decodeLooseVerticalCoverage(payload["vertical_coverage"]); ok {
		proposal.VerticalCoverage = verticalCoverage
	}

	var storyItems []json.RawMessage
	if err := json.Unmarshal(payload["proposed_stories"], &storyItems); err != nil {
		if err := json.Unmarshal(payload["proposed_tasks"], &storyItems); err != nil {
			if approvedPreviewDebugEnabled() {
				slog.Error("approved task plan preview proposed_tasks decode failed during apply",
					"normalized_preview", previewDebugSnippet(normalized, 1600),
					"error", err,
				)
			}
			return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}; legacy proposed_stories is still accepted")
		}
	}
	proposal.ProposedStories = make([]model.ProposedStory, 0, len(storyItems))
	for index, item := range storyItems {
		task, ok := decodeLooseApprovedProposedTask(item)
		if !ok {
			if approvedPreviewDebugEnabled() {
				slog.Error("approved task plan preview item decode failed during apply",
					"task_index", index,
					"task_preview", previewDebugSnippet(item, 1200),
				)
			}
			return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}; legacy proposed_stories is still accepted")
		}
		proposal.ProposedStories = append(proposal.ProposedStories, task)
	}
	if approvedPreviewDebugEnabled() {
		slog.Info("decoded approved task plan preview",
			"summary_preview", truncateString(strings.TrimSpace(proposal.Summary), 240),
			"task_count", len(proposal.ProposedStories),
		)
	}
	return proposal, nil
}

func decodeLooseApprovedProposedTask(raw json.RawMessage) (model.ProposedStory, bool) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return model.ProposedStory{}, false
	}

	task := model.ProposedStory{
		Ref:                decodeLooseJSONString(payload["ref"]),
		Name:               firstNonEmptyString(decodeLooseJSONString(payload["name"]), decodeLooseJSONString(payload["title"])),
		Description:        decodeLooseJSONString(payload["description"]),
		StoryType:          firstNonEmptyString(decodeLooseJSONString(payload["task_type"]), decodeLooseJSONString(payload["story_type"]), decodeLooseJSONString(payload["type"])),
		SliceType:          decodeLooseJSONString(payload["slice_type"]),
		AcceptanceCriteria: decodeLooseJSONStringArray(payload["acceptance_criteria"]),
		DependencyRefs:     decodeLooseJSONStringArray(payload["dependency_refs"]),
	}
	if estimate, ok := decodeLooseJSONInt(payload["estimate"]); ok {
		task.Estimate = &estimate
	}
	if priority := decodeLooseJSONString(payload["priority"]); priority != "" {
		task.Priority = &priority
	}
	if assignAgentID := decodeLooseJSONString(payload["assign_agent_id"]); assignAgentID != "" {
		task.AssignAgentID = &assignAgentID
	}
	if sourceRefs, ok := decodeLoosePlanningSourceRefs(payload["source_refs"]); ok {
		task.SourceRefs = sourceRefs
	}
	if brief, ok := decodeLooseTaskImplementationBrief(payload["implementation_brief"]); ok {
		task.ImplementationBrief = brief
	}
	return task, true
}

func decodeLooseJSONString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return ""
}

func decodeLooseJSONStringArray(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err == nil {
		filtered := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				filtered = append(filtered, item)
			}
		}
		return filtered
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return nil
		}
		return []string{single}
	}
	var mixed []any
	if err := json.Unmarshal(raw, &mixed); err == nil {
		filtered := make([]string, 0, len(mixed))
		for _, item := range mixed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text != "" {
				filtered = append(filtered, text)
			}
		}
		return filtered
	}
	return nil
}

func decodeLooseJSONInt(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var value int
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(text, "%d", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func decodeLoosePlanningSourceRefs(raw json.RawMessage) ([]model.PlanningSourceRef, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var refs []model.PlanningSourceRef
	if err := json.Unmarshal(raw, &refs); err == nil {
		return refs, true
	}
	return nil, false
}

func decodeLooseTaskImplementationBrief(raw json.RawMessage) (*model.StoryImplementationBrief, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false
	}

	brief := &model.StoryImplementationBrief{
		Approach:       decodeLooseJSONString(payload["approach"]),
		FilesToModify:  decodeLooseFileChanges(payload["files_to_modify"]),
		TestStrategy:   decodeLooseJSONText(payload["test_strategy"]),
		VerticalLayers: decodeLooseJSONStringArray(payload["vertical_layers"]),
		DependsOnFiles: decodeLooseJSONStringArray(payload["depends_on_files"]),
	}
	return brief, true
}

func decodeLooseFileChanges(raw json.RawMessage) []model.FileChange {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	changes := make([]model.FileChange, 0, len(items))
	for _, item := range items {
		var change model.FileChange
		if err := json.Unmarshal(item, &change); err != nil {
			continue
		}
		if strings.TrimSpace(change.Path) == "" {
			continue
		}
		changes = append(changes, change)
	}
	return changes
}

func decodeLooseJSONText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err == nil {
		filtered := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				filtered = append(filtered, item)
			}
		}
		return strings.Join(filtered, "\n")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err == nil {
		return compact.String()
	}
	return strings.TrimSpace(string(raw))
}

func decodeLooseVerticalCoverage(raw json.RawMessage) ([]model.VerticalCoverageEntry, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var entries []model.VerticalCoverageEntry
	if err := json.Unmarshal(raw, &entries); err == nil {
		return entries, true
	}
	return nil, false
}

func approvedPreviewDebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_PREVIEW_DEBUG"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func previewDebugSnippet(raw json.RawMessage, max int) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return ""
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err == nil {
		trimmed = compact.String()
	}
	if max > 0 && len(trimmed) > max {
		return trimmed[:max] + "...(truncated)"
	}
	return trimmed
}

func truncateString(value string, max int) string {
	value = strings.TrimSpace(value)
	if max > 0 && len(value) > max {
		return value[:max] + "...(truncated)"
	}
	return value
}

func decodeApprovedMarkdownPreviewContent(raw json.RawMessage, previewLabel string) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", fmt.Errorf("%s content is empty; publish a non-empty markdown draft before requesting approval", previewLabel)
	}

	var markdown string
	if err := json.Unmarshal(raw, &markdown); err != nil {
		return "", fmt.Errorf("%s content must be a markdown string", previewLabel)
	}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return "", fmt.Errorf("%s content is empty; publish a non-empty markdown draft before requesting approval", previewLabel)
	}
	return markdown, nil
}

func (a *AgentRunActivities) applyApprovedPRDPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved PRD preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatMarkdown {
		return fmt.Errorf("approved PRD preview must use format %q", workerpkg.PreviewFormatMarkdown)
	}

	markdown, err := decodeApprovedMarkdownPreviewContent(preview.Content, "approved PRD preview")
	if err != nil {
		return err
	}

	doc, err := a.ensureEpicSpecDocument(ctx, state, runActorID(state.run))
	if err != nil {
		return err
	}
	if a.commandExecutor != nil {
		if _, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: state.run.WorkspaceID,
			TargetType:  "document",
			TargetID:    doc.ID,
		}, "docs.write_document_content", mustJSON(map[string]any{
			"document_id": doc.ID,
			"content":     markdown,
		})); err != nil {
			return err
		}
		if _, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: state.run.WorkspaceID,
			ActorID:     runActorID(state.run),
			TargetType:  "epic",
			TargetID:    state.epic.ID,
		}, "pm.approve_epic_spec", json.RawMessage(`{}`)); err != nil {
			return err
		}
	} else {
		savedContent, err := a.docsContentRepo.Upsert(ctx, doc.ID, tiptap.MarkdownToJSON(markdown))
		if err != nil {
			return err
		}
		label := "Approved Spec"
		version, err := a.docsVersionRepo.Create(ctx, doc.ID, runActorID(state.run), savedContent.Content, savedContent.ContentText, &label, "manual", len(strings.Fields(savedContent.ContentText)))
		if err != nil {
			return err
		}
		state.epic.SpecDocumentID = &doc.ID
		state.epic.ApprovedSpecVersionID = &version.ID
		if err := a.epicRepo.Update(ctx, state.epic); err != nil {
			return err
		}
	}

	epicWithStats, err := a.epicRepo.GetByID(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	if epicWithStats == nil {
		return fmt.Errorf("epic not found after applying approved PRD preview")
	}
	state.epic = &epicWithStats.Epic
	input.SpecDocumentID = strings.TrimSpace(derefString(state.epic.SpecDocumentID))
	input.SpecVersionID = strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID))
	return nil
}

func (a *AgentRunActivities) applyApprovedTaskPlanPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved task plan preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatJSON {
		return fmt.Errorf("approved task plan preview must use format %q", workerpkg.PreviewFormatJSON)
	}
	if a.commandExecutor == nil {
		return fmt.Errorf("planner commands are not available")
	}

	proposal, err := decodeApprovedTaskPlanPreviewContent(preview.Content)
	if err != nil {
		return err
	}
	if proposal.EpicID == "" {
		proposal.EpicID = state.epic.ID
	}
	if proposal.SpecVersionID == "" {
		proposal.SpecVersionID = strings.TrimSpace(firstNonEmptyString(input.SpecVersionID, derefString(state.epic.ApprovedSpecVersionID)))
	}
	if err := validatePlanningProposalTasks(proposal.ProposedStories); err != nil {
		return err
	}

	output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
		WorkspaceID: state.run.WorkspaceID,
		ActorID:     runActorID(state.run),
		TargetType:  "epic",
		TargetID:    state.epic.ID,
	}, "pm.create_task_batch", mustJSON(map[string]any{
		"tasks": proposal.ProposedStories,
	}))
	if err != nil {
		return err
	}

	var result workerpkg.CreateTaskBatchResult
	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("parse created task batch: %w", err)
	}

	tasks, err := a.epicRepo.ListStories(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	state.epicTasks = tasks

	epicWithStats, err := a.epicRepo.GetByID(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	if epicWithStats == nil {
		return fmt.Errorf("epic not found after applying approved task plan")
	}
	state.epic = &epicWithStats.Epic
	state.epic.LastPlanningRunID = &state.run.ID
	if err := a.epicRepo.Update(ctx, state.epic); err != nil {
		return err
	}

	state.run.OutputSummary, _ = json.Marshal(planningRunSummary{
		Stage:               model.PlanningStagePlanStories,
		SpecDocumentID:      strings.TrimSpace(firstNonEmptyString(input.SpecDocumentID, derefString(state.epic.SpecDocumentID))),
		SpecVersionID:       proposal.SpecVersionID,
		PlanningMethodology: input.PlanningMethodology,
		Summary:             strings.TrimSpace(proposal.Summary),
		Risks:               append([]string(nil), proposal.Risks...),
		OpenQuestions:       append([]string(nil), proposal.OpenQuestions...),
		Proposal:            &proposal,
	})

	createdCount := len(result.Tasks)
	summaryText := fmt.Sprintf("Applied the approved task plan and created %d tasks.", createdCount)
	if _, err := a.createRunMessage(ctx, state.run, "assistant", "assistant_turn", summaryText, nil, nil, nil, nil); err != nil {
		return err
	}

	completedAt := time.Now()
	state.run.Status = model.AgentRunStatusCompleted
	state.run.PauseReason = model.AgentRunPauseReasonNone
	state.run.CompletedAt = &completedAt
	state.run.ExecutionStage = strPtr("completed")
	state.run.LastHeartbeatAt = &completedAt
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)
	return a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
}

func (a *AgentRunActivities) applyApprovedTaskDocPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved task planning document preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatMarkdown {
		return fmt.Errorf("approved task planning document preview must use format %q", workerpkg.PreviewFormatMarkdown)
	}
	if state.task == nil {
		return fmt.Errorf("approved task planning document preview requires a task target")
	}

	markdown, err := decodeApprovedMarkdownPreviewContent(preview.Content, "approved task planning document preview")
	if err != nil {
		return err
	}

	doc, err := a.ensureTaskPlanDocument(ctx, state, runActorID(state.run))
	if err != nil {
		return err
	}
	if a.commandExecutor != nil {
		if _, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: state.run.WorkspaceID,
			TargetType:  "document",
			TargetID:    doc.ID,
		}, "docs.write_document_content", mustJSON(map[string]any{
			"document_id": doc.ID,
			"content":     markdown,
		})); err != nil {
			return err
		}
		if _, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: state.run.WorkspaceID,
			ActorID:     runActorID(state.run),
			TargetType:  "task",
			TargetID:    state.task.ID,
		}, "docs.link_document_to_object", mustJSON(map[string]any{
			"document_id":        doc.ID,
			"linked_object_type": model.LinkedObjectTask,
			"linked_object_id":   state.task.ID,
			"link_context":       model.LinkContextCreatedFrom,
		})); err != nil {
			return err
		}
	} else {
		if _, err := a.docsContentRepo.Upsert(ctx, doc.ID, tiptap.MarkdownToJSON(markdown)); err != nil {
			return err
		}
		if err := a.ensureTaskPlanLink(ctx, state.run.WorkspaceID, doc.ID, state.task.ID, runActorID(state.run)); err != nil {
			return err
		}
	}

	if content, err := a.docsContentRepo.GetByDocumentID(ctx, doc.ID); err == nil && content != nil {
		label := "Approved Task Plan"
		_, _ = a.docsVersionRepo.Create(ctx, doc.ID, runActorID(state.run), content.Content, content.ContentText, &label, "manual", len(strings.Fields(content.ContentText)))
	}

	state.task.PlanDocumentID = &doc.ID
	if err := a.storyRepo.Update(ctx, state.task); err != nil {
		return err
	}
	input.PlanDocumentID = doc.ID

	state.run.OutputSummary, _ = json.Marshal(planningRunSummary{
		Stage:          model.PlanningStageStoryPlanDoc,
		PlanDocumentID: doc.ID,
		Summary:        strings.TrimSpace(preview.ApprovalSummary),
	})

	summaryText := "Persisted the approved task plan to Docs and linked it to the task."
	if _, err := a.createRunMessage(ctx, state.run, "assistant", "assistant_turn", summaryText, nil, nil, nil, nil); err != nil {
		return err
	}

	completedAt := time.Now()
	state.run.Status = model.AgentRunStatusCompleted
	state.run.PauseReason = model.AgentRunPauseReasonNone
	state.run.CompletedAt = &completedAt
	state.run.ExecutionStage = strPtr("completed")
	state.run.LastHeartbeatAt = &completedAt
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)
	return a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
}

func (a *AgentRunActivities) loadRunState(ctx context.Context, runID string) (*resolvedRunState, error) {
	run, err := a.runRepo.GetByIDAny(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}
	model.NormalizeAgentRunPauseState(run)

	agent, err := a.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}

	resolved := workerpkg.ResolveAgentProfile(agent, run.InvocationMode)
	state := &resolvedRunState{
		run:      run,
		agent:    agent,
		resolved: resolved,
	}

	if run.TargetType == "" {
		switch {
		case run.StoryID != nil:
			run.TargetType = "task"
			run.TargetID = *run.StoryID
		case run.ConversationID != nil:
			run.TargetType = "support_conversation"
			run.TargetID = *run.ConversationID
		}
	}

	if run.StoryID != nil {
		task, err := a.storyRepo.GetRawByID(ctx, *run.StoryID)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, fmt.Errorf("task not found")
		}
		state.task = task

		target, teamDefault, err := a.resolveDeliveryTarget(ctx, run.WorkspaceID, task)
		if err != nil {
			return nil, err
		}
		state.deliveryTarget = target
		state.teamDefault = teamDefault

		if target != nil && target.RepositoryID != nil && *target.RepositoryID != "" {
			repo, err := a.gitRepo.GetByID(ctx, run.WorkspaceID, *target.RepositoryID)
			if err != nil {
				return nil, err
			}
			state.repository = repo
		}
		if target != nil && target.IntegrationID != nil && *target.IntegrationID != "" {
			integration, err := a.gitIntRepo.GetByID(ctx, run.WorkspaceID, *target.IntegrationID)
			if err != nil {
				return nil, err
			}
			state.integration = integration
		}
		if state.integration != nil {
			token, err := a.mintAccessToken(ctx, state.integration)
			if err != nil {
				return nil, err
			}
			state.accessToken = token
		}
		if task.EpicID != nil && strings.TrimSpace(*task.EpicID) != "" {
			epicWithStats, err := a.epicRepo.GetByID(ctx, *task.EpicID)
			if err != nil {
				return nil, err
			}
			if epicWithStats != nil {
				state.epic = &epicWithStats.Epic
			}
		}
	}

	if run.ConversationID != nil {
		conversation, err := a.conversationRepo.GetByID(ctx, run.WorkspaceID, *run.ConversationID, "", model.RoleOwner)
		if err != nil {
			return nil, err
		}
		if conversation == nil {
			return nil, fmt.Errorf("conversation not found")
		}
		state.conversation = conversation
	}

	if run.TargetType == "epic" {
		epicWithStats, err := a.epicRepo.GetByID(ctx, run.TargetID)
		if err != nil {
			return nil, err
		}
		if epicWithStats == nil {
			return nil, fmt.Errorf("epic not found")
		}
		state.epic = &epicWithStats.Epic
		epicTasks, err := a.epicRepo.ListStories(ctx, run.TargetID)
		if err != nil {
			return nil, err
		}
		state.epicTasks = epicTasks
	}

	return state, nil
}

func (a *AgentRunActivities) resolveDeliveryTarget(ctx context.Context, workspaceID string, story *model.PMStory) (*model.StoryDeliveryTarget, *model.PMTeamRepoDefault, error) {
	target, err := a.deliveryRepo.GetByStory(ctx, workspaceID, story.ID)
	if err != nil {
		return nil, nil, err
	}
	if target != nil {
		var teamDefault *model.PMTeamRepoDefault
		if story.TeamID != nil && *story.TeamID != "" {
			teamDefault, err = a.settingsRepo.GetTeamRepoDefault(ctx, *story.TeamID)
			if err != nil {
				return nil, nil, err
			}
		}
		return target, teamDefault, nil
	}

	target = &model.StoryDeliveryTarget{
		WorkspaceID:   workspaceID,
		StoryID:       story.ID,
		DeliveryState: "unconfigured",
	}

	var teamDefault *model.PMTeamRepoDefault
	if story.TeamID != nil && *story.TeamID != "" {
		teamDefault, err = a.settingsRepo.GetTeamRepoDefault(ctx, *story.TeamID)
		if err != nil {
			return nil, nil, err
		}
		if teamDefault != nil {
			repo, err := a.gitRepo.GetByID(ctx, workspaceID, teamDefault.RepositoryID)
			if err != nil {
				return nil, nil, err
			}
			if repo != nil {
				target.RepositoryID = &repo.ID
				target.RepoFullName = &repo.FullName
				target.IntegrationID = &repo.IntegrationID
				baseBranch := teamDefault.BaseBranch
				if strings.TrimSpace(baseBranch) == "" {
					baseBranch = defaultString(repo.DefaultBranch, "main")
				}
				target.BaseBranch = &baseBranch
				target.DeliveryState = "ready"
			}
		}
	}

	if err := a.deliveryRepo.Save(ctx, target); err != nil {
		return nil, nil, err
	}
	return target, teamDefault, nil
}

func (a *AgentRunActivities) prepareTaskDelivery(ctx context.Context, state *resolvedRunState) error {
	target := state.deliveryTarget
	if target == nil {
		return fmt.Errorf("task delivery target is missing")
	}
	if state.repository == nil || state.integration == nil {
		if state.resolved.RequiresRepo {
			return fmt.Errorf("task has no delivery target configured")
		}
		return nil
	}

	if target.RepoFullName == nil {
		target.RepoFullName = &state.repository.FullName
	}
	if target.IntegrationID == nil {
		target.IntegrationID = &state.repository.IntegrationID
	}
	if target.BaseBranch == nil || strings.TrimSpace(*target.BaseBranch) == "" {
		baseBranch := defaultString(state.repository.DefaultBranch, "main")
		target.BaseBranch = &baseBranch
	}

	if state.resolved.RequiresRepo && (target.WorkingBranch == nil || strings.TrimSpace(*target.WorkingBranch) == "") {
		branchName := buildWorkingBranch(state.task, state.teamDefault)
		target.WorkingBranch = &branchName
	}

	if state.resolved.RequiresRepo && state.accessToken == "" {
		return fmt.Errorf("repository access token is not available")
	}

	if state.resolved.RequiresRepo && target.WorkingBranch != nil && *target.WorkingBranch != "" {
		if err := a.ensureRemoteBranch(ctx, state.integration, state.accessToken, state.repository.FullName, *target.BaseBranch, *target.WorkingBranch); err != nil {
			return err
		}
	}

	target.LastRunID = &state.run.ID
	if target.RepositoryID != nil {
		state.run.RepositoryID = target.RepositoryID
	}
	state.run.RepoFullName = target.RepoFullName
	state.run.BaseBranch = target.BaseBranch
	state.run.WorkingBranch = target.WorkingBranch
	state.run.DeliveryTargetID = &target.ID
	if state.run.TaskQueue == nil || *state.run.TaskQueue == "" {
		queue := state.resolved.Queue
		state.run.TaskQueue = &queue
		state.run.RunnerPool = &queue
	}
	target.DeliveryState = "in_progress"
	if err := a.deliveryRepo.Save(ctx, target); err != nil {
		return err
	}

	return a.upsertGitLink(ctx, state, "", nil)
}

func (a *AgentRunActivities) checkoutRunRef(ctx context.Context, workDir string, state *resolvedRunState) error {
	baseBranch := derefString(state.run.BaseBranch)
	workingBranch := derefString(state.run.WorkingBranch)
	if workingBranch != "" {
		if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "fetch", "origin", workingBranch); err == nil {
			if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", workingBranch, "origin/"+workingBranch); err == nil {
				return nil
			}
		}
	}
	if baseBranch == "" {
		baseBranch = defaultString(state.repository.DefaultBranch, "main")
	}
	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "fetch", "origin", baseBranch); err != nil {
		return fmt.Errorf("fetch base branch: %w", err)
	}
	if workingBranch != "" {
		if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", workingBranch, "origin/"+baseBranch); err != nil {
			return fmt.Errorf("checkout working branch: %w", err)
		}
		return nil
	}
	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", baseBranch, "origin/"+baseBranch); err != nil {
		return fmt.Errorf("checkout base branch: %w", err)
	}
	return nil
}

func (a *AgentRunActivities) ensureRemoteBranch(ctx context.Context, integration *model.GitIntegration, accessToken, repoFullName, baseBranch, workingBranch string) error {
	workDir, err := workerpkg.PrepareWorkspace(ctx, integration, repoFullName, accessToken)
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, "fetch", "origin", baseBranch); err != nil {
		return fmt.Errorf("fetch base branch: %w", err)
	}
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, "fetch", "origin", workingBranch); err == nil {
		return nil
	}
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, "checkout", "-B", workingBranch, "origin/"+baseBranch); err != nil {
		return fmt.Errorf("create working branch: %w", err)
	}
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, "push", "-u", "origin", workingBranch); err != nil {
		return fmt.Errorf("push working branch: %w", err)
	}
	return nil
}

func (a *AgentRunActivities) recordPush(ctx context.Context, state *resolvedRunState, branch, sha string) error {
	if state.deliveryTarget == nil {
		return nil
	}
	state.deliveryTarget.WorkingBranch = &branch
	state.deliveryTarget.LastCommitSHA = &sha
	state.deliveryTarget.LastRunID = &state.run.ID
	state.deliveryTarget.DeliveryState = "in_progress"
	now := time.Now()
	state.deliveryTarget.LastSyncedAt = &now
	state.run.WorkingBranch = &branch
	state.run.LastHeartbeatAt = &now
	if err := a.deliveryRepo.Save(ctx, state.deliveryTarget); err != nil {
		return err
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	return a.upsertGitLink(ctx, state, sha, nil)
}

func (a *AgentRunActivities) recordPR(ctx context.Context, state *resolvedRunState, metadata workerpkg.PRMetadata, title string) error {
	if state.deliveryTarget == nil {
		return nil
	}
	state.deliveryTarget.ActivePRNumber = &metadata.Number
	state.deliveryTarget.ActivePRTitle = &title
	state.deliveryTarget.ActivePRURL = &metadata.URL
	openStatus := "open"
	state.deliveryTarget.ActivePRStatus = &openStatus
	state.deliveryTarget.LastRunID = &state.run.ID
	state.deliveryTarget.DeliveryState = "pr_open"
	now := time.Now()
	state.deliveryTarget.LastSyncedAt = &now
	if err := a.deliveryRepo.Save(ctx, state.deliveryTarget); err != nil {
		return err
	}
	return a.upsertGitLink(ctx, state, "", &prUpdate{
		number: metadata.Number,
		title:  title,
		url:    metadata.URL,
		status: openStatus,
	})
}

type prUpdate struct {
	number int
	title  string
	url    string
	status string
}

func (a *AgentRunActivities) upsertGitLink(ctx context.Context, state *resolvedRunState, sha string, pr *prUpdate) error {
	if state.task == nil || state.deliveryTarget == nil || state.repository == nil || state.integration == nil {
		return nil
	}
	branch := derefString(state.deliveryTarget.WorkingBranch)
	if branch == "" {
		return nil
	}

	link, err := a.gitLinkRepo.GetByBranch(ctx, state.run.WorkspaceID, state.repository.FullName, branch)
	if err != nil {
		return err
	}
	if link == nil {
		link = &model.StoryGitLink{
			WorkspaceID:   state.run.WorkspaceID,
			StoryID:       state.task.ID,
			IntegrationID: state.integration.ID,
			RepositoryID:  state.deliveryTarget.RepositoryID,
			RunID:         &state.run.ID,
			Provider:      state.integration.Provider,
			Repo:          state.repository.FullName,
			Branch:        &branch,
		}
		if sha != "" {
			link.CommitSHA = &sha
		}
		if pr != nil {
			link.PRNumber = &pr.number
			link.PRTitle = &pr.title
			link.PRURL = &pr.url
			link.PRStatus = &pr.status
		}
		return a.gitLinkRepo.Create(ctx, link)
	}

	link.RepositoryID = state.deliveryTarget.RepositoryID
	link.RunID = &state.run.ID
	link.Provider = state.integration.Provider
	link.IntegrationID = state.integration.ID
	if sha != "" {
		link.CommitSHA = &sha
	}
	if pr != nil {
		link.PRNumber = &pr.number
		link.PRTitle = &pr.title
		link.PRURL = &pr.url
		link.PRStatus = &pr.status
	}
	return a.gitLinkRepo.Update(ctx, link)
}

func (a *AgentRunActivities) resolvePlanningRunInput(ctx context.Context, state *resolvedRunState) (planningRunInput, error) {
	var input planningRunInput
	if len(state.run.Input) > 0 {
		_ = json.Unmarshal(state.run.Input, &input)
	}

	input.Stage = strings.TrimSpace(input.Stage)
	input.AdditionalContext = strings.TrimSpace(input.AdditionalContext)
	input.PlanDocumentID = strings.TrimSpace(input.PlanDocumentID)
	input.SpecDocumentID = strings.TrimSpace(input.SpecDocumentID)
	input.SpecVersionID = strings.TrimSpace(input.SpecVersionID)
	input.PlanningMethodology = model.NormalizePlanningMethodology(strings.TrimSpace(input.PlanningMethodology))
	input.FlowOutputKind = strings.TrimSpace(input.FlowOutputKind)

	if err := a.normalizeEpicSpecState(ctx, state); err != nil {
		return planningRunInput{}, err
	}

	tools := effectiveToolSet(state.resolved, input.AllowedTools)
	if state.run.TargetType == "task" && state.task != nil && tools[workerpkg.ToolPublishTaskPlanDoc] {
		if input.Stage == "" {
			input.Stage = model.PlanningStageTaskPlanDoc
		}
		if input.PlanDocumentID == "" && state.task.PlanDocumentID != nil {
			input.PlanDocumentID = strings.TrimSpace(*state.task.PlanDocumentID)
		}
		if state.epic != nil {
			input.SpecDocumentID = strings.TrimSpace(derefString(state.epic.SpecDocumentID))
			input.SpecVersionID = strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID))
		}
		payload, _ := json.Marshal(input)
		state.run.Input = payload
		return input, nil
	}

	if state.run.TargetType != "epic" || state.epic == nil {
		return input, nil
	}

	input.SpecDocumentID = strings.TrimSpace(derefString(state.epic.SpecDocumentID))
	input.SpecVersionID = strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID))
	if input.PlanningMethodology == "" {
		input.PlanningMethodology = model.PlanningMethodologyStructuredV1
	}

	payload, _ := json.Marshal(input)
	state.run.Input = payload

	return input, nil
}

func (a *AgentRunActivities) normalizeEpicSpecState(ctx context.Context, state *resolvedRunState) error {
	if state == nil || state.epic == nil || a.epicRepo == nil {
		return nil
	}

	changed := false
	if docID := strings.TrimSpace(derefString(state.epic.SpecDocumentID)); docID != "" && a.docsDocRepo != nil {
		doc, err := a.docsDocRepo.GetByID(ctx, docID)
		if err != nil {
			return err
		}
		if doc == nil {
			state.epic.SpecDocumentID = nil
			state.epic.ApprovedSpecVersionID = nil
			changed = true
		}
	}

	if versionID := strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID)); versionID != "" && a.docsVersionRepo != nil {
		version, err := a.docsVersionRepo.GetByID(ctx, versionID)
		if err != nil {
			return err
		}
		if version == nil || strings.TrimSpace(derefString(state.epic.SpecDocumentID)) == "" || version.DocumentID != strings.TrimSpace(derefString(state.epic.SpecDocumentID)) {
			state.epic.ApprovedSpecVersionID = nil
			changed = true
		}
	}

	if !changed {
		return nil
	}
	return a.epicRepo.Update(ctx, state.epic)
}

func (a *AgentRunActivities) buildInitialInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	tools := effectiveToolSet(state.resolved, input.AllowedTools)
	if strings.TrimSpace(input.FlowOutputKind) != "" {
		return a.buildFlowOutputInstructions(ctx, state, input)
	}
	if state.run.TargetType == "task" && state.task != nil && tools[workerpkg.ToolPublishTaskPlanDoc] {
		return a.buildTaskPlannerInstructions(ctx, state, input)
	}
	if state.run.TargetType != "epic" || state.epic == nil {
		return runInputAdditionalContext(state.run.Input), nil
	}
	if !tools[workerpkg.ToolPublishPRDDraft] || (!tools[workerpkg.ToolPublishTaskPlan] && !tools[workerpkg.ToolPublishStoryPlan]) {
		return runInputAdditionalContext(state.run.Input), nil
	}
	return a.buildAgenticEpicPlannerInstructions(ctx, state, input)
}

func (a *AgentRunActivities) buildFlowOutputInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	switch strings.TrimSpace(input.FlowOutputKind) {
	case "pm.story_completion_followups":
		return a.buildTaskCompletionInstructions(state, input), nil
	case "crm.deal_review_actions":
		return a.buildCRMDealReviewInstructions(ctx, state, input)
	default:
		return runInputAdditionalContext(state.run.Input), nil
	}
}

func (a *AgentRunActivities) buildTaskCompletionInstructions(state *resolvedRunState, input planningRunInput) string {
	var sections []string
	sections = append(sections, "Review this completed task and return JSON only with the shape {\"summary\":\"...\",\"followups\":[{\"title\":\"...\",\"description\":\"...\",\"task_type\":\"chore\",\"priority\":\"medium\"}]}. Legacy story_type is still accepted.")
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
	if deal.Pipeline != nil && len(deal.Pipeline.Stages) > 0 {
		lines := make([]string, 0, len(deal.Pipeline.Stages))
		for _, stage := range deal.Pipeline.Stages {
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

func (a *AgentRunActivities) buildTaskPlannerInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if state.task == nil {
		return "", fmt.Errorf("task planner requires a task target")
	}

	var sections []string
	sections = append(sections, fmt.Sprintf("Run mode: %s", state.run.InvocationMode))
	if state.run.InvocationMode == model.InvocationModeInteractive {
		sections = append(sections, "The shared run drawer is available for live questions, draft previews, inline approvals, and change requests.")
		sections = append(sections, "Treat this as one transcript-driven planning run. Humans approve and request changes with normal chat replies in this same transcript.")
		sections = append(sections, "Only a clear explicit approval counts as approval. Requested changes, critique, concerns, or ambiguous replies mean the draft is not approved yet.")
	}
	sections = append(sections, "Choose the next step from the transcript, task details, parent epic context, linked docs, comments, code context, and tool results.")
	sections = append(sections, "Use this sequence unless the human explicitly redirects you: clarify scope if needed, draft or refine the task planning doc, publish it with publish_task_plan_doc, wait for inline approval, then stop. The platform will persist and link the approved preview to the canonical task planning doc automatically.")
	sections = append(sections, "Keep approvals soft and inline. When you need approval, call request_review_checkpoint with phase=\"task_doc\" and stop after the request.")
	sections = append(sections, "Treat request_review_checkpoint as the final action in that turn. Do not call more tools after it, and do not append extra approval-choice prose after requesting the checkpoint.")
	sections = append(sections, "Use publish_task_plan_doc for reviewable right-pane task planning documents.")
	sections = append(sections, "Treat parent epic details, the epic PRD, and epic-linked docs as background context only. Use them to understand constraints, inherited requirements, and non-goals, but do not copy them wholesale into the task planning document unless they directly affect this task's implementation.")
	sections = append(sections, "Ground the planning document primarily in the task description, task comments, task-linked docs, and the current codebase context. Keep the output focused on this task's implementation plan.")

	if strings.TrimSpace(input.PlanDocumentID) != "" {
		sections = append(sections, fmt.Sprintf("Canonical task planning document ID: %s", input.PlanDocumentID))
		content, err := a.docsContentRepo.GetByDocumentID(ctx, input.PlanDocumentID)
		if err != nil {
			return "", err
		}
		if content != nil && strings.TrimSpace(content.ContentText) != "" {
			sections = append(sections, "Current task planning draft already in Docs:\n"+truncatePlanningText(content.ContentText, 12000))
			sections = append(sections, "Resume from the existing planning doc draft instead of starting over unless the human explicitly wants a reset.")
		}
	} else {
		sections = append(sections, "No canonical task planning doc exists yet. Keep the draft in chat-backed preview artifacts until approval; the platform will create, persist, and link the approved artifact.")
	}

	if input.AdditionalContext != "" {
		sections = append(sections, "Operator notes:\n"+input.AdditionalContext)
	}

	sections = append(sections, fmt.Sprintf("Task: %s", state.task.Name))
	if state.task.Description != nil {
		if description := tiptap.RichTextToMarkdown(*state.task.Description); description != "" {
			sections = append(sections, "Task description:\n"+truncatePlanningText(description, 8000))
		}
	}
	if state.task.TeamID != nil && strings.TrimSpace(*state.task.TeamID) != "" {
		sections = append(sections, fmt.Sprintf("Task team ID: %s", strings.TrimSpace(*state.task.TeamID)))
	}

	if state.epic != nil {
		sections = append(sections, fmt.Sprintf("Parent epic: %s", state.epic.Name))
		if state.epic.Description != nil {
			if description := tiptap.RichTextToMarkdown(*state.epic.Description); description != "" {
				sections = append(sections, "Parent epic description:\n"+truncatePlanningText(description, 8000))
			}
		}
	}

	if input.SpecVersionID != "" {
		version, err := a.docsVersionRepo.GetByID(ctx, input.SpecVersionID)
		if err != nil {
			return "", err
		}
		if version != nil && strings.TrimSpace(version.ContentText) != "" {
			sections = append(sections, fmt.Sprintf("Approved epic PRD version ID: %s", version.ID))
			sections = append(sections, "Approved epic PRD snapshot:\n"+truncatePlanningText(version.ContentText, 16000))
		}
	} else if input.SpecDocumentID != "" {
		content, err := a.docsContentRepo.GetByDocumentID(ctx, input.SpecDocumentID)
		if err != nil {
			return "", err
		}
		if content != nil && strings.TrimSpace(content.ContentText) != "" {
			sections = append(sections, fmt.Sprintf("Parent epic PRD document ID: %s", input.SpecDocumentID))
			sections = append(sections, "Current epic PRD draft:\n"+truncatePlanningText(content.ContentText, 12000))
		}
	}

	storyLinkedDocs, err := a.renderObjectLinkedDocsContext(ctx, state.run.WorkspaceID, model.LinkedObjectStory, state.task.ID, input.PlanDocumentID)
	if err != nil {
		return "", err
	}
	if storyLinkedDocs != "" {
		sections = append(sections, "Other docs linked directly to this task:\n"+storyLinkedDocs)
	}

	if state.epic != nil {
		epicLinkedDocs, err := a.renderLinkedDocsContext(ctx, state.run.WorkspaceID, state.epic.ID, input.SpecDocumentID)
		if err != nil {
			return "", err
		}
		if epicLinkedDocs != "" {
			sections = append(sections, "Other docs linked to the parent epic:\n"+epicLinkedDocs)
		}
	}

	commentsContext, err := a.renderTaskCommentsContext(ctx, state.task.ID)
	if err != nil {
		return "", err
	}
	if commentsContext != "" {
		sections = append(sections, "Task comments:\n"+commentsContext)
	}

	repoContext, err := a.buildDraftSpecCodeContext(ctx, state, strings.Join([]string{
		state.task.Name,
		tiptap.RichTextToMarkdown(derefString(state.task.Description)),
		input.AdditionalContext,
	}, "\n\n"))
	if err != nil {
		return "", err
	}
	if repoContext != "" {
		sections = append(sections, "Current implementation context from the live repository:\n"+repoContext)
	}

	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildAgenticEpicPlannerInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	var sections []string

	sections = append(sections, fmt.Sprintf("Run mode: %s", state.run.InvocationMode))
	if state.run.InvocationMode == model.InvocationModeInteractive {
		sections = append(sections, "The shared run drawer is available for live questions, draft previews, inline approvals, and change requests.")
		sections = append(sections, "Treat this as one transcript-driven planning run. There is no hidden planner phase machine controlling the next step for you.")
		sections = append(sections, "Humans approve and request changes with normal chat replies in this same transcript. Do not tell them to use a separate approval workflow, button, or UI gate.")
		sections = append(sections, "Only a clear explicit approval counts as approval. Any requested change, concern, critique, follow-up question, or ambiguous reply means the current phase is not approved yet.")
	}
	sections = append(sections, "Choose the next step from the transcript, current epic state, linked docs, existing tasks, and tool results.")
	sections = append(sections, "Use this sequence unless the human explicitly redirects you: clarify scope if needed, draft/refine the PRD, publish it with publish_prd_draft, wait for inline PRD approval, let the platform persist the approved PRD artifact to the canonical epic doc, propose the implementation task plan, publish it with publish_task_plan, wait for inline task approval, then let the platform apply the approved task plan artifact and create tasks.")
	sections = append(sections, "Keep approvals soft and inline. When you need approval, call request_review_checkpoint with phase=\"prd\" or phase=\"tasks\" and stop after the request.")
	sections = append(sections, "Treat request_review_checkpoint as the final action in that turn. Do not call more tools after it in the same turn. Do not append extra approval-choice prose after requesting the checkpoint.")
	sections = append(sections, "After explicit PRD approval, continue automatically to task planning in the same run. Do not ask whether to proceed to tasks unless the human explicitly redirects scope.")
	sections = append(sections, "Use publish_prd_draft for PRD markdown previews and publish_task_plan for task plan JSON previews.")
	sections = append(sections, "Before approval, keep drafts in chat-backed preview artifacts only. After approval, the platform applies the approved artifact; do not replay approved PRDs or task plans through mutation tools.")

	var hasSpecContent bool
	if input.SpecDocumentID != "" {
		sections = append(sections, fmt.Sprintf("Existing canonical spec document ID: %s", input.SpecDocumentID))

		// Inject durable draft content when a spec doc exists but is not approved.
		if input.SpecVersionID == "" {
			if content, err := a.docsContentRepo.GetByDocumentID(ctx, input.SpecDocumentID); err == nil && content != nil && strings.TrimSpace(content.ContentText) != "" {
				hasSpecContent = true
				sections = append(sections, "IMPORTANT: A PRD draft already exists in the spec document but was never formally approved. Resume from the current draft instead of starting over. Present the draft, revise it if needed, and request PRD approval before any task planning.")
				sections = append(sections, "Current spec draft:\n"+truncatePlanningText(content.ContentText, 12000))
			}
		}
	}
	if input.SpecVersionID != "" {
		sections = append(sections, fmt.Sprintf("Approved spec version ID: %s", input.SpecVersionID))
		sections = append(sections, "IMPORTANT: A previously approved spec already exists. The PRD is LOCKED. Do not redraft, rewrite, or re-approve it. Use it as the read-only source of truth for task planning. If the human asks to revise the PRD, explain the spec is approved and suggest creating a follow-up epic instead, unless they insist.")
		sections = append(sections, "If the current facts show the PRD is already approved, treat persistence as complete and continue from that state. Do not replay the PRD through mutation tools.")
	}
	if state.epic != nil && state.epic.TeamID != nil && strings.TrimSpace(*state.epic.TeamID) != "" {
		sections = append(sections, fmt.Sprintf("Epic team ID: %s", strings.TrimSpace(*state.epic.TeamID)))
	} else {
		sections = append(sections, "This epic does not currently have a team. Before creating tasks, call list_workspace_teams and ask the human to choose the correct team inline in chat.")
	}
	if input.AdditionalContext != "" {
		sections = append(sections, "Operator notes:\n"+input.AdditionalContext)
	}

	linkedDocs, err := a.renderLinkedDocsContext(ctx, state.run.WorkspaceID, state.epic.ID, input.SpecDocumentID)
	if err != nil {
		return "", err
	}
	if linkedDocs != "" {
		sections = append(sections, "Other docs linked to this epic:\n"+linkedDocs)
	}

	linkedTickets, err := a.renderLinkedTicketsContext(ctx, state)
	if err != nil {
		return "", err
	}
	if linkedTickets != "" {
		sections = append(sections, "Support and customer context already linked to this epic:\n"+linkedTickets)
	}

	repoContext, err := a.buildDraftSpecCodeContext(ctx, state, strings.Join([]string{
		state.epic.Name,
		tiptap.RichTextToMarkdown(derefString(state.epic.Description)),
		linkedDocs,
		linkedTickets,
		input.AdditionalContext,
	}, "\n\n"))
	if err != nil {
		return "", err
	}
	if repoContext != "" {
		sections = append(sections, "Current implementation context from the planning repository:\n"+repoContext)
	}

	if len(state.epicTasks) > 0 {
		lines := make([]string, 0, len(state.epicTasks))
		for _, task := range state.epicTasks {
			lines = append(lines, fmt.Sprintf("- %s [%s]", task.Name, task.ID))
		}
		sections = append(sections, fmt.Sprintf("IMPORTANT: %d tasks already exist under this epic. Do NOT recreate them. Only create new tasks if the human explicitly requests additions.", len(state.epicTasks)))
		sections = append(sections, "Tasks already linked to this epic:\n"+strings.Join(lines, "\n"))
	}

	hasApprovedSpec := input.SpecVersionID != ""
	hasSpecDoc := input.SpecDocumentID != ""
	hasTasks := len(state.epicTasks) > 0
	sections = append(sections, formatInteractivePlanningFacts(input, hasSpecContent, len(state.epicTasks)))
	switch {
	case hasApprovedSpec && hasTasks:
		sections = append(sections, "Next-step guidance: the PRD is approved and tasks already exist. Do not redraft the PRD or recreate existing tasks. Enter clarification or extension mode, inspect current tasks if needed, and only add new tasks if the human explicitly asks for them.")
	case hasApprovedSpec && !hasTasks:
		sections = append(sections, "Next-step guidance: the PRD is approved and no tasks exist yet. Skip PRD drafting entirely and proceed directly to task planning from the approved spec and current codebase context.")
	case hasSpecDoc && hasSpecContent:
		sections = append(sections, "Next-step guidance: a draft PRD exists but it is not approved yet. Resume from the current draft, present or revise it, and request PRD approval before any task planning.")
	default:
		sections = append(sections, "Next-step guidance: no approved PRD exists yet. Follow the full loop from clarification through PRD drafting, preview, revision if needed, and approval.")
	}

	return strings.Join(sections, "\n\n"), nil
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

func (a *AgentRunActivities) preparePlanningRepository(ctx context.Context, state *resolvedRunState, input planningRunInput) error {
	if state.run.TargetType != "epic" || state.epic == nil {
		return nil
	}
	if state.epic.PlanningRepositoryID == nil || strings.TrimSpace(*state.epic.PlanningRepositoryID) == "" {
		return nil
	}

	repo, err := a.gitRepo.GetByID(ctx, state.run.WorkspaceID, *state.epic.PlanningRepositoryID)
	if err != nil {
		return err
	}
	if repo == nil {
		return fmt.Errorf("planning repository not found")
	}
	if repo.Archived || !repo.Selected {
		return fmt.Errorf("planning repository is not available")
	}

	integration, err := a.gitIntRepo.GetByID(ctx, state.run.WorkspaceID, repo.IntegrationID)
	if err != nil {
		return err
	}
	if integration == nil {
		return fmt.Errorf("planning repository integration not found")
	}

	token, err := a.mintAccessToken(ctx, integration)
	if err != nil {
		return fmt.Errorf("planning repository access token: %w", err)
	}

	state.repository = repo
	state.integration = integration
	state.accessToken = token
	state.run.RepositoryID = &repo.ID
	state.run.RepoFullName = &repo.FullName
	baseBranch := repo.DefaultBranch
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = "main"
	}
	state.run.BaseBranch = &baseBranch
	return nil
}

func (a *AgentRunActivities) finalizePlanningRun(ctx context.Context, state *resolvedRunState, input planningRunInput) error {
	if strings.TrimSpace(input.FlowOutputKind) != "" {
		return a.finalizeFlowOutputRun(ctx, state, input)
	}
	if state.run.TargetType != "epic" || state.epic == nil {
		return nil
	}
	return a.finalizeAgenticEpicPlannerRun(ctx, state)
}

func (a *AgentRunActivities) finalizeAgenticEpicPlannerRun(ctx context.Context, state *resolvedRunState) error {
	if state.epic == nil {
		return nil
	}
	state.epic.LastPlanningRunID = &state.run.ID
	return a.epicRepo.Update(ctx, state.epic)
}

func (a *AgentRunActivities) finalizeFlowOutputRun(ctx context.Context, state *resolvedRunState, input planningRunInput) error {
	switch strings.TrimSpace(input.FlowOutputKind) {
	case "pm.story_completion_followups":
		var assessment model.StoryCompletionAssessment
		if err := json.Unmarshal(state.run.OutputSummary, &assessment); err != nil {
			return fmt.Errorf("decode task completion assessment: %w", err)
		}
		if strings.TrimSpace(assessment.Summary) == "" {
			return fmt.Errorf("task completion assessment is missing a summary")
		}
		return nil
	case "crm.deal_review_actions":
		var plan model.CRMDealReviewActionPlan
		if err := json.Unmarshal(state.run.OutputSummary, &plan); err != nil {
			return fmt.Errorf("decode CRM deal review plan: %w", err)
		}
		if strings.TrimSpace(plan.Summary) == "" {
			return fmt.Errorf("CRM deal review plan is missing a summary")
		}
		return nil
	default:
		return nil
	}
}

func (a *AgentRunActivities) ensureEpicSpecDocument(ctx context.Context, state *resolvedRunState, actorID string) (*model.DocsDocument, error) {
	if state.epic == nil {
		return nil, fmt.Errorf("epic context is required")
	}
	if state.epic.SpecDocumentID != nil && strings.TrimSpace(*state.epic.SpecDocumentID) != "" {
		doc, err := a.docsDocRepo.GetByID(ctx, *state.epic.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			return doc, nil
		}
	}

	space, err := a.docsSpaceRepo.GetBySlug(ctx, state.run.WorkspaceID, productSpecsSpaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		space, err = a.docsSpaceRepo.Create(ctx, &model.DocsSpace{
			WorkspaceID: state.run.WorkspaceID,
			Name:        productSpecsSpaceName,
			Slug:        productSpecsSpaceSlug,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			IsSystem:    true,
			Position:    2,
			CreatedBy:   actorID,
		})
		if err != nil {
			return nil, fmt.Errorf("create product specs space: %w", err)
		}
	}

	teamID := state.epic.TeamID
	if teamID == nil {
		teamID = strPtr(actorID)
	}

	doc, err := a.docsDocRepo.Create(ctx, &model.DocsDocument{
		WorkspaceID: state.run.WorkspaceID,
		SpaceID:     space.ID,
		Title:       strings.TrimSpace(state.epic.Name) + " Product Spec",
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		OwnerID:     strPtr(actorID),
		TeamID:      teamID,
		TemplateKey: strPtr("product_spec"),
		Tags:        model.DocsStringArray{"product-spec", "epic"},
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, err
	}

	state.epic.SpecDocumentID = &doc.ID
	if err := a.epicRepo.Update(ctx, state.epic); err != nil {
		return nil, err
	}
	if err := a.ensureEpicSpecLink(ctx, state.run.WorkspaceID, doc.ID, state.epic.ID, actorID); err != nil {
		return nil, err
	}

	return doc, nil
}

func (a *AgentRunActivities) ensureTaskPlanDocument(ctx context.Context, state *resolvedRunState, actorID string) (*model.DocsDocument, error) {
	if state.task == nil {
		return nil, fmt.Errorf("task not found")
	}
	if state.task.PlanDocumentID != nil && strings.TrimSpace(*state.task.PlanDocumentID) != "" {
		doc, err := a.docsDocRepo.GetByID(ctx, *state.task.PlanDocumentID)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			if err := a.ensureTaskPlanLink(ctx, state.run.WorkspaceID, doc.ID, state.task.ID, actorID); err != nil {
				return nil, err
			}
			return doc, nil
		}
	}

	space, err := a.docsSpaceRepo.GetBySlug(ctx, state.run.WorkspaceID, productSpecsSpaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		space, err = a.docsSpaceRepo.Create(ctx, &model.DocsSpace{
			WorkspaceID: state.run.WorkspaceID,
			Name:        productSpecsSpaceName,
			Slug:        productSpecsSpaceSlug,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			IsSystem:    true,
			Position:    2,
			CreatedBy:   actorID,
		})
		if err != nil {
			return nil, fmt.Errorf("create product specs space: %w", err)
		}
	}

	teamID := state.task.TeamID
	if teamID == nil && state.epic != nil {
		teamID = state.epic.TeamID
	}
	if teamID == nil {
		teamID = strPtr(actorID)
	}

	doc, err := a.docsDocRepo.Create(ctx, &model.DocsDocument{
		WorkspaceID: state.run.WorkspaceID,
		SpaceID:     space.ID,
		Title:       strings.TrimSpace(state.task.Name) + " Plan",
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		OwnerID:     strPtr(actorID),
		TeamID:      teamID,
		TemplateKey: strPtr("task_plan"),
		Tags:        model.DocsStringArray{"task-plan", "task"},
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, err
	}

	state.task.PlanDocumentID = &doc.ID
	if err := a.storyRepo.Update(ctx, state.task); err != nil {
		return nil, err
	}
	if err := a.ensureTaskPlanLink(ctx, state.run.WorkspaceID, doc.ID, state.task.ID, actorID); err != nil {
		return nil, err
	}
	return doc, nil
}

func (a *AgentRunActivities) ensureEpicSpecLink(ctx context.Context, workspaceID, documentID, epicID, actorID string) error {
	links, err := a.docsLinkRepo.ListByObject(ctx, workspaceID, model.LinkedObjectEpic, epicID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link.DocumentID == documentID {
			return nil
		}
	}
	_, err = a.docsLinkRepo.Create(ctx, &model.DocsLink{
		WorkspaceID:      workspaceID,
		DocumentID:       documentID,
		LinkedObjectType: model.LinkedObjectEpic,
		LinkedObjectID:   epicID,
		LinkContext:      model.LinkContextCreatedFrom,
		CreatedBy:        actorID,
	})
	return err
}

func (a *AgentRunActivities) ensureTaskPlanLink(ctx context.Context, workspaceID, documentID, storyID, actorID string) error {
	links, err := a.docsLinkRepo.ListByObject(ctx, workspaceID, model.LinkedObjectStory, storyID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link.DocumentID == documentID {
			return nil
		}
	}
	_, err = a.docsLinkRepo.Create(ctx, &model.DocsLink{
		WorkspaceID:      workspaceID,
		DocumentID:       documentID,
		LinkedObjectType: model.LinkedObjectStory,
		LinkedObjectID:   storyID,
		LinkContext:      model.LinkContextCreatedFrom,
		CreatedBy:        actorID,
	})
	return err
}

func (a *AgentRunActivities) renderLinkedDocsContext(ctx context.Context, workspaceID, epicID, excludeDocumentID string) (string, error) {
	return a.renderObjectLinkedDocsContext(ctx, workspaceID, model.LinkedObjectEpic, epicID, excludeDocumentID)
}

func (a *AgentRunActivities) renderObjectLinkedDocsContext(ctx context.Context, workspaceID, objectType, objectID, excludeDocumentID string) (string, error) {
	links, err := a.docsLinkRepo.ListByObject(ctx, workspaceID, objectType, objectID)
	if err != nil {
		return "", err
	}
	if len(links) == 0 {
		return "", nil
	}

	seen := make(map[string]bool, len(links))
	entries := make([]string, 0, len(links))
	for _, link := range links {
		if link.DocumentID == excludeDocumentID || seen[link.DocumentID] {
			continue
		}
		seen[link.DocumentID] = true

		doc, err := a.docsDocRepo.GetByID(ctx, link.DocumentID)
		if err != nil {
			return "", err
		}
		if doc == nil {
			continue
		}

		content, err := a.docsContentRepo.GetByDocumentID(ctx, doc.ID)
		if err != nil {
			return "", err
		}
		body := "(no content yet)"
		if content != nil && strings.TrimSpace(content.ContentText) != "" {
			body = truncatePlanningText(content.ContentText, 3000)
		}

		entries = append(entries, fmt.Sprintf("- %s [%s]\n%s", doc.Title, doc.ID, body))
		if len(entries) >= 5 {
			break
		}
	}

	return strings.Join(entries, "\n\n"), nil
}

func (a *AgentRunActivities) renderTaskCommentsContext(ctx context.Context, storyID string) (string, error) {
	if strings.TrimSpace(storyID) == "" || a.commentRepo == nil {
		return "", nil
	}
	comments, err := a.commentRepo.List(ctx, "task", storyID)
	if err != nil {
		return "", err
	}
	if len(comments) == 0 {
		return "", nil
	}

	entries := make([]string, 0, len(comments))
	for idx, entry := range comments {
		authorName := strings.TrimSpace(entry.Author.FullName)
		if authorName == "" {
			authorName = entry.Comment.AuthorID
		}
		line := fmt.Sprintf("- %s: %s", authorName, truncatePlanningText(entry.Comment.Body, 320))
		if len(entry.Replies) > 0 {
			replyLines := make([]string, 0, len(entry.Replies))
			for _, reply := range entry.Replies {
				replyAuthor := strings.TrimSpace(reply.Author.FullName)
				if replyAuthor == "" {
					replyAuthor = reply.Comment.AuthorID
				}
				replyLines = append(replyLines, fmt.Sprintf("  - %s: %s", replyAuthor, truncatePlanningText(reply.Comment.Body, 220)))
			}
			line += "\nReplies:\n" + strings.Join(replyLines, "\n")
		}
		entries = append(entries, line)
		if idx >= 5 {
			break
		}
	}
	return strings.Join(entries, "\n\n"), nil
}

func (a *AgentRunActivities) renderLinkedTicketsContext(ctx context.Context, state *resolvedRunState) (string, error) {
	if len(state.epicTasks) == 0 {
		return "", nil
	}

	taskIDs := make([]string, 0, len(state.epicTasks))
	taskNames := make(map[string]string, len(state.epicTasks))
	for _, task := range state.epicTasks {
		taskIDs = append(taskIDs, task.ID)
		taskNames[task.ID] = task.Name
	}

	tickets, err := a.conversationRepo.ListByLinkedStoryIDs(ctx, state.run.WorkspaceID, taskIDs)
	if err != nil {
		return "", err
	}
	if len(tickets) == 0 {
		return "", nil
	}

	entries := make([]string, 0, len(tickets))
	for idx, ticket := range tickets {
		taskName := ""
		if ticket.LinkedTaskID != nil {
			taskName = taskNames[*ticket.LinkedTaskID]
		}

		header := fmt.Sprintf("- Ticket #%d: %s [status=%s priority=%s]", ticket.DisplayID, ticket.Subject, ticket.Status, ticket.Priority)
		if taskName != "" {
			header += fmt.Sprintf(" linked_task=%q", taskName)
		}
		if ticket.CustomerEmail != nil && strings.TrimSpace(*ticket.CustomerEmail) != "" {
			header += fmt.Sprintf(" customer=%s", *ticket.CustomerEmail)
		}

		entry := header
		messages, err := a.messageRepo.ListByConversation(ctx, state.run.WorkspaceID, ticket.ID, true)
		if err != nil {
			return "", err
		}
		if len(messages) > 0 {
			start := len(messages) - 2
			if start < 0 {
				start = 0
			}
			lines := make([]string, 0, len(messages)-start)
			for _, message := range messages[start:] {
				scope := "public"
				if message.IsInternal {
					scope = "internal"
				}
				lines = append(lines, fmt.Sprintf("  - [%s/%s] %s", message.SenderType, scope, truncatePlanningText(message.Content, 280)))
			}
			entry += "\nRecent messages:\n" + strings.Join(lines, "\n")
		}

		entries = append(entries, entry)
		if idx >= 4 {
			break
		}
	}

	return strings.Join(entries, "\n\n"), nil
}

func (a *AgentRunActivities) buildPlanningCodeContext(ctx context.Context, state *resolvedRunState, specText string) (string, error) {
	if state.repository == nil || state.integration == nil {
		return "", nil
	}

	workDir, err := workerpkg.PrepareWorkspace(ctx, state.integration, state.repository.FullName, state.accessToken)
	if err != nil {
		return "", fmt.Errorf("prepare planning repository workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	treeEntries, err := collectPlanningTree(workDir, 3, 80)
	if err != nil {
		return "", err
	}

	manifestCandidates := []string{
		"package.json", "pnpm-workspace.yaml", "turbo.json", "tsconfig.json",
		"go.mod", "go.work", "Cargo.toml", "pyproject.toml", "requirements.txt",
		"Dockerfile", "docker-compose.yml", "docker-compose.yaml",
		"README.md", "WORKFLOW.md",
	}
	manifestSnippets := collectStaticFileSnippets(workDir, manifestCandidates, 6, 1200)

	relevantFiles, err := selectRelevantPlanningFiles(workDir, specText)
	if err != nil {
		return "", err
	}
	fileSnippets := collectStaticFileSnippets(workDir, relevantFiles, 8, 1400)

	var sections []string
	sections = append(sections, fmt.Sprintf("Repository root: %s", state.repository.FullName))
	if len(treeEntries) > 0 {
		sections = append(sections, "Repository structure:\n"+strings.Join(treeEntries, "\n"))
	}
	if len(manifestSnippets) > 0 {
		sections = append(sections, "Key manifests and architecture anchors:\n"+strings.Join(manifestSnippets, "\n\n"))
	}
	if len(fileSnippets) > 0 {
		sections = append(sections, "Relevant implementation files:\n"+strings.Join(fileSnippets, "\n\n"))
	} else {
		sections = append(sections, "Relevant implementation files: no confident file matches were found from the approved spec. Treat uncertain areas as risks or open questions.")
	}

	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildDraftSpecCodeContext(ctx context.Context, state *resolvedRunState, seedText string) (string, error) {
	if state.repository == nil || state.integration == nil {
		return "", nil
	}

	workDir, err := workerpkg.PrepareWorkspace(ctx, state.integration, state.repository.FullName, state.accessToken)
	if err != nil {
		return "", fmt.Errorf("prepare draft-spec repository workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	treeEntries, err := collectPlanningTree(workDir, 2, 50)
	if err != nil {
		return "", err
	}

	manifestCandidates := []string{
		"package.json", "go.mod", "go.work", "Cargo.toml", "pyproject.toml",
		"README.md", "WORKFLOW.md",
	}
	manifestSnippets := collectStaticFileSnippets(workDir, manifestCandidates, 4, 900)

	relevantFiles, err := selectRelevantPlanningFiles(workDir, seedText)
	if err != nil {
		return "", err
	}
	fileLimit := 5
	if len(relevantFiles) > fileLimit {
		relevantFiles = relevantFiles[:fileLimit]
	}
	fileSnippets := collectStaticFileSnippets(workDir, relevantFiles, fileLimit, 900)

	var sections []string
	sections = append(sections, fmt.Sprintf("Repository root: %s", state.repository.FullName))
	if len(treeEntries) > 0 {
		sections = append(sections, "High-level repository structure:\n"+strings.Join(treeEntries, "\n"))
	}
	if len(manifestSnippets) > 0 {
		sections = append(sections, "Key product and platform anchors:\n"+strings.Join(manifestSnippets, "\n\n"))
	}
	if len(fileSnippets) > 0 {
		sections = append(sections, "Likely relevant product surface files:\n"+strings.Join(fileSnippets, "\n\n"))
	} else {
		sections = append(sections, "Likely relevant product surface files: no confident matches were found. Treat repo-specific assumptions as risks or open questions.")
	}

	return strings.Join(sections, "\n\n"), nil
}

func collectPlanningTree(root string, maxDepth, maxEntries int) ([]string, error) {
	type item struct {
		path  string
		depth int
	}
	var items []item
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if shouldSkipPlanningPath(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth >= maxDepth && d.IsDir() {
			items = append(items, item{path: rel + "/", depth: depth})
			return filepath.SkipDir
		}
		if len(items) >= maxEntries {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		label := rel
		if d.IsDir() {
			label += "/"
		}
		items = append(items, item{path: label, depth: depth})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].path < items[j].path })
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprintf("%s%s", strings.Repeat("  ", item.depth), item.path))
	}
	return out, nil
}

func collectStaticFileSnippets(root string, candidates []string, limit, maxChars int) []string {
	results := make([]string, 0, limit)
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		fullPath := filepath.Join(root, candidate)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		results = append(results, fmt.Sprintf("[%s]\n%s", candidate, truncatePlanningText(string(content), maxChars)))
		if len(results) >= limit {
			break
		}
	}
	return results
}

func selectRelevantPlanningFiles(root, specText string) ([]string, error) {
	keywords := planningKeywords(specText)
	type candidate struct {
		path  string
		score int
	}
	candidates := make([]candidate, 0, 64)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || shouldSkipPlanningPath(rel) || !looksLikePlanningSourceFile(rel) {
			return nil
		}

		score := planningFileBaseScore(rel)
		lowerRel := strings.ToLower(rel)
		for _, keyword := range keywords {
			if strings.Contains(lowerRel, keyword) {
				score += 8
			}
		}
		if score <= 0 {
			return nil
		}
		candidates = append(candidates, candidate{path: rel, score: score})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].path < candidates[j].path
		}
		return candidates[i].score > candidates[j].score
	})

	limit := 8
	if len(candidates) < limit {
		limit = len(candidates)
	}
	out := make([]string, 0, limit)
	seen := make(map[string]bool, limit)
	for _, item := range candidates {
		if seen[item.path] {
			continue
		}
		seen[item.path] = true
		out = append(out, item.path)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func planningKeywords(specText string) []string {
	normalized := strings.ToLower(specText)
	replacer := strings.NewReplacer(
		"\n", " ", "\t", " ", ",", " ", ".", " ", ":", " ", ";", " ", "(", " ", ")", " ",
		"{", " ", "}", " ", "[", " ", "]", " ", "/", " ", "\\", " ", "-", " ", "_", " ",
	)
	normalized = replacer.Replace(normalized)
	words := strings.Fields(normalized)
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "that": true, "this": true, "from": true,
		"into": true, "will": true, "story": true, "stories": true, "spec": true, "product": true,
		"epic": true, "user": true, "users": true, "should": true, "have": true, "must": true,
		"plan": true, "planning": true, "acceptance": true, "criteria": true, "when": true, "then": true,
		"given": true, "goal": true, "goals": true, "risk": true, "risks": true,
	}
	freq := make(map[string]int)
	for _, word := range words {
		if len(word) < 4 || stop[word] {
			continue
		}
		freq[word]++
	}
	type pair struct {
		word  string
		count int
	}
	pairs := make([]pair, 0, len(freq))
	for word, count := range freq {
		pairs = append(pairs, pair{word: word, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].word < pairs[j].word
		}
		return pairs[i].count > pairs[j].count
	})
	limit := 12
	if len(pairs) < limit {
		limit = len(pairs)
	}
	keywords := make([]string, 0, limit)
	for idx := 0; idx < limit; idx++ {
		keywords = append(keywords, pairs[idx].word)
	}
	return keywords
}

func shouldSkipPlanningPath(rel string) bool {
	rel = filepath.ToSlash(strings.ToLower(rel))
	skipParts := []string{
		".git/", "node_modules/", "vendor/", "dist/", "build/", ".next/", ".turbo/", ".cache/",
		"coverage/", "tmp/", "temp/", "bin/", "public/", "assets/", "storybook-static/",
	}
	for _, part := range skipParts {
		if strings.Contains(rel, part) {
			return true
		}
	}
	return false
}

func looksLikePlanningSourceFile(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".json", ".yaml", ".yml", ".md", ".py", ".rb", ".java", ".kt", ".rs":
		return true
	default:
		return false
	}
}

func planningFileBaseScore(rel string) int {
	lower := filepath.ToSlash(strings.ToLower(rel))
	score := 0
	switch {
	case strings.Contains(lower, "router"), strings.Contains(lower, "routes/"):
		score += 12
	case strings.Contains(lower, "handler"), strings.Contains(lower, "controller"):
		score += 11
	case strings.Contains(lower, "service"):
		score += 10
	case strings.Contains(lower, "model"), strings.Contains(lower, "schema"), strings.Contains(lower, "entity"):
		score += 9
	case strings.Contains(lower, "repository"), strings.Contains(lower, "store"):
		score += 8
	case strings.Contains(lower, "test"):
		score += 6
	case strings.Contains(lower, "component"), strings.Contains(lower, "page"):
		score += 7
	}
	if strings.HasSuffix(lower, "package.json") || strings.HasSuffix(lower, "go.mod") || strings.HasSuffix(lower, "cargo.toml") || strings.HasSuffix(lower, "pyproject.toml") || strings.HasSuffix(lower, "readme.md") {
		score += 14
	}
	return score
}

func runActorID(run *model.AgentRun) string {
	if run.TriggeredByUserID != nil && strings.TrimSpace(*run.TriggeredByUserID) != "" {
		return *run.TriggeredByUserID
	}
	return run.AgentID
}

func truncatePlanningText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "\n... (truncated)"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// coalesceRaw returns the first non-empty string without trimming whitespace.
// Use this instead of firstNonEmptyString when the value may contain meaningful
// leading/trailing whitespace (e.g. LLM streaming token deltas like " found").
func coalesceRaw(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func validatePlanningProposalTasks(stories []model.ProposedStory) error {
	return model.NormalizeProposedStories(stories)
}

func buildDraftSpecClarifications(draft model.ProductSpecDraft) []model.SpecClarificationItem {
	items := make([]model.SpecClarificationItem, 0, len(draft.OpenQuestions)+len(draft.Assumptions))
	for idx, question := range draft.OpenQuestions {
		question = strings.TrimSpace(question)
		if question == "" {
			continue
		}
		items = append(items, model.SpecClarificationItem{
			ID:          fmt.Sprintf("open_question_%d", idx+1),
			Kind:        model.SpecClarificationKindOpenQuestion,
			Prompt:      question,
			Disposition: model.SpecClarificationDispositionPending,
		})
	}
	for idx, assumption := range draft.Assumptions {
		assumption = strings.TrimSpace(assumption)
		if assumption == "" {
			continue
		}
		items = append(items, model.SpecClarificationItem{
			ID:          fmt.Sprintf("assumption_%d", idx+1),
			Kind:        model.SpecClarificationKindAssumption,
			Prompt:      assumption,
			Disposition: model.SpecClarificationDispositionPending,
		})
	}
	return items
}

func renderSpecClarificationsContext(items []model.SpecClarificationItem) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		switch item.Kind {
		case model.SpecClarificationKindOpenQuestion:
			lines = append(lines, fmt.Sprintf("- %s -> %s", item.Prompt, item.Response))
		case model.SpecClarificationKindAssumption:
			if item.Disposition == model.SpecClarificationDispositionAccepted {
				lines = append(lines, fmt.Sprintf("- %s -> accepted", item.Prompt))
			} else if item.Disposition == model.SpecClarificationDispositionRejected {
				lines = append(lines, fmt.Sprintf("- %s -> rejected: %s", item.Prompt, item.Response))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func renderProductSpecMarkdown(specMarkdown string, sources []model.PlanningResearchSource) string {
	specMarkdown = strings.TrimSpace(specMarkdown)
	sources = normalizePlanningResearchSources(sources)
	if len(sources) == 0 {
		return specMarkdown
	}

	lines := []string{specMarkdown, "## Research Sources"}
	for _, source := range sources {
		parts := []string{fmt.Sprintf("[%s](%s)", source.Title, source.URL)}
		note := strings.TrimSpace(source.Note)
		if note != "" {
			parts = append(parts, note)
		}
		publishedAt := strings.TrimSpace(source.PublishedAt)
		if publishedAt != "" {
			parts = append(parts, "published "+publishedAt)
		}
		lines = append(lines, "- "+strings.Join(parts, " - "))
	}

	return strings.TrimSpace(strings.Join(lines, "\n\n"))
}

func normalizePlanningResearchSources(sources []model.PlanningResearchSource) []model.PlanningResearchSource {
	normalized := make([]model.PlanningResearchSource, 0, len(sources))
	seen := make(map[string]bool, len(sources))
	for _, source := range sources {
		title := strings.TrimSpace(source.Title)
		sourceURL := strings.TrimSpace(source.URL)
		if title == "" || sourceURL == "" {
			continue
		}
		key := strings.ToLower(sourceURL)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, model.PlanningResearchSource{
			Title:       title,
			URL:         sourceURL,
			Note:        strings.TrimSpace(source.Note),
			PublishedAt: strings.TrimSpace(source.PublishedAt),
		})
	}
	return normalized
}

func (a *AgentRunActivities) createRunArtifact(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any, sequenceNo int) error {
	return a.createRunArtifactWithMetadata(ctx, run, artifactType, format, payload, sequenceNo, nil)
}

func (a *AgentRunActivities) createRunArtifactWithMetadata(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any, sequenceNo int, metadata json.RawMessage) error {
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal artifact %s: %w", artifactType, err)
	}
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: strPtr(string(content)),
		Metadata:      metadata,
		SequenceNo:    sequenceNo,
	}
	return a.artifactRepo.Create(ctx, artifact)
}

func (a *AgentRunActivities) appendRunArtifact(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any) (*model.AgentRunArtifact, error) {
	return a.appendRunArtifactWithMetadata(ctx, run, artifactType, format, payload, nil)
}

func (a *AgentRunActivities) appendRunArtifactWithMetadata(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any, metadata json.RawMessage) (*model.AgentRunArtifact, error) {
	content, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal artifact %s: %w", artifactType, err)
	}
	sequenceNo, err := a.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: strPtr(string(content)),
		Metadata:      metadata,
		SequenceNo:    sequenceNo,
	}
	if err := a.artifactRepo.Create(ctx, artifact); err != nil {
		return nil, err
	}
	return artifact, nil
}

func lastAssistantSequenceNoUpTo(messages []model.AgentRunMessage, maxSequenceNo int) int {
	last := 0
	for _, message := range messages {
		if message.SequenceNo > maxSequenceNo {
			break
		}
		if strings.TrimSpace(message.Role) == "assistant" {
			last = message.SequenceNo
		}
	}
	return last
}

func (a *AgentRunActivities) mintAccessToken(ctx context.Context, integration *model.GitIntegration) (string, error) {
	if integration == nil {
		return "", nil
	}
	if integration.CredentialMode == "github_app" && integration.InstallationID != nil && *integration.InstallationID != "" {
		if a.githubApp == nil {
			return "", fmt.Errorf("github app credentials are not configured")
		}
		return a.githubApp.MintInstallationToken(ctx, *integration.InstallationID)
	}
	if integration.AccessToken != "" {
		return integration.AccessToken, nil
	}
	return "", fmt.Errorf("git integration has no usable credentials")
}

func (a *AgentRunActivities) serviceBridge() *workerpkg.ServiceBridge {
	return &workerpkg.ServiceBridge{
		ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("internal commands are not available")
			}
			return a.commandExecutor.Execute(ctx, meta, name, input)
		},
		AddComment: func(ctx context.Context, workspaceID, storyID, agentID, content string) error {
			comment := &model.PMComment{
				EntityType: "task",
				EntityID:   storyID,
				AuthorID:   agentID,
				Body:       content,
			}
			return a.commentRepo.Create(ctx, comment)
		},
		UpdateTaskState: func(ctx context.Context, workspaceID, storyID, stateID string) error {
			if a.commandExecutor != nil {
				_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					WorkspaceID: workspaceID,
					TargetType:  "task",
					TargetID:    storyID,
				}, "pm.update_task_state", mustJSON(map[string]any{
					"task_id":  storyID,
					"state_id": stateID,
				}))
				return err
			}
			// TODO(flow-platform): remove direct fallback once all native runs are command-backed.
			task, err := a.storyRepo.GetRawByID(ctx, storyID)
			if err != nil {
				return err
			}
			if task == nil {
				return fmt.Errorf("task not found")
			}
			task.WorkflowStateID = stateID
			return a.storyRepo.Update(ctx, task)
		},
		ListChecklist: func(ctx context.Context, workspaceID, storyID string) ([]model.PMChecklistItem, error) {
			return a.checklistRepo.List(ctx, storyID)
		},
		CreateTaskBatch: func(ctx context.Context, workspaceID, epicID, actorID string, stories []model.ProposedStory) (workerpkg.CreateTaskBatchResult, error) {
			if a.commandExecutor != nil {
				output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					WorkspaceID: workspaceID,
					ActorID:     actorID,
					TargetType:  "epic",
					TargetID:    epicID,
				}, "pm.create_task_batch", mustJSON(map[string]any{
					"tasks": stories,
				}))
				if err != nil {
					return workerpkg.CreateTaskBatchResult{}, err
				}
				var result workerpkg.CreateTaskBatchResult
				if err := json.Unmarshal(output, &result); err != nil {
					return workerpkg.CreateTaskBatchResult{}, err
				}
				return result, nil
			}
			return workerpkg.CreateTaskBatchResult{}, fmt.Errorf("planner commands are not available")
		},
		AssignTaskAgent: func(ctx context.Context, workspaceID, actorID, storyID, agentID string) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("planner commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "task",
				TargetID:    storyID,
			}, "pm.assign_task_agent", mustJSON(map[string]any{
				"task_id":  storyID,
				"agent_id": agentID,
			}))
			return err
		},
		SetTaskDependencies: func(ctx context.Context, workspaceID, actorID string, dependencies []workerpkg.TaskDependencyLink) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("planner commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "epic",
			}, "pm.set_task_dependencies", mustJSON(map[string]any{
				"dependencies": dependencies,
			}))
			return err
		},
		ListEpicTasks: func(ctx context.Context, workspaceID, epicID string) ([]workerpkg.EpicTaskSummary, error) {
			tasks, err := a.epicRepo.ListStories(ctx, epicID)
			if err != nil {
				return nil, err
			}
			summaries := make([]workerpkg.EpicTaskSummary, 0, len(tasks))
			for _, s := range tasks {
				if s.WorkspaceID != workspaceID {
					continue
				}
				status := "not_started"
				if s.Completed {
					status = "done"
				} else if s.Started {
					status = "in_progress"
				}
				summaries = append(summaries, workerpkg.EpicTaskSummary{
					ID:              s.ID,
					Name:            s.Name,
					TaskType:        s.StoryType,
					Status:          status,
					Estimate:        s.Estimate,
					Priority:        s.Priority,
					AssignedAgentID: s.AssignedAgentID,
				})
			}
			return summaries, nil
		},
		ListWorkspaceTeams: func(ctx context.Context, workspaceID string) ([]workerpkg.WorkspaceTeamSummary, error) {
			if a.settingsRepo == nil {
				return nil, fmt.Errorf("workspace settings are not available")
			}
			teams, err := a.settingsRepo.ListTeams(ctx, workspaceID)
			if err != nil {
				return nil, err
			}
			result := make([]workerpkg.WorkspaceTeamSummary, 0, len(teams))
			for _, team := range teams {
				result = append(result, workerpkg.WorkspaceTeamSummary{
					ID:              team.ID,
					Name:            team.Name,
					Handle:          team.Handle,
					TeamType:        team.TeamType,
					DefaultTaskType: team.DefaultStoryType,
				})
			}
			return result, nil
		},
		ApproveEpicSpec: func(ctx context.Context, workspaceID, epicID, actorID string, versionID *string) (*model.ApprovedSpecSummary, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("planner commands are not available")
			}
			input := mustJSON(map[string]any{})
			if versionID != nil && strings.TrimSpace(*versionID) != "" {
				input = mustJSON(map[string]any{"version_id": strings.TrimSpace(*versionID)})
			}
			output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "epic",
				TargetID:    epicID,
			}, "pm.approve_epic_spec", input)
			if err != nil {
				return nil, err
			}
			var summary model.ApprovedSpecSummary
			if err := json.Unmarshal(output, &summary); err != nil {
				return nil, err
			}
			return &summary, nil
		},
		ListConversationMessages: func(ctx context.Context, workspaceID, conversationID string) ([]model.SupportMessage, error) {
			return a.messageRepo.ListByConversation(ctx, workspaceID, conversationID, true)
		},
		UpdateConversationStatus: func(ctx context.Context, workspaceID, conversationID, status string) error {
			conversation, err := a.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
			if err != nil {
				return err
			}
			if conversation == nil {
				return fmt.Errorf("conversation not found")
			}
			conversation.Status = status
			if err := a.conversationRepo.Update(ctx, conversation); err != nil {
				return err
			}
			if a.wsPublisher != nil {
				a.wsPublisher.Publish(websocket.Event{
					Action:      "updated",
					Entity:      "support_conversation",
					EntityID:    conversationID,
					WorkspaceID: workspaceID,
				})
			}
			return nil
		},

		// CRM
		ListDeals: func(ctx context.Context, workspaceID string, limit int) ([]model.CRMDeal, error) {
			deals, _, err := a.crmDealRepo.List(ctx, workspaceID, model.CRMDealListFilters{}, model.PMPagination{Page: 1, PerPage: limit})
			return deals, err
		},
		GetDeal: func(ctx context.Context, id string) (*model.CRMDeal, error) {
			return a.crmDealRepo.GetByID(ctx, id)
		},
		UpdateDealStage: func(ctx context.Context, dealID, stageID string) error {
			if a.commandExecutor != nil {
				_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					TargetType: "crm_deal",
					TargetID:   dealID,
				}, "crm.update_deal_stage", mustJSON(map[string]any{
					"deal_id":  dealID,
					"stage_id": stageID,
				}))
				return err
			}
			// TODO(flow-platform): remove direct fallback once all native runs are command-backed.
			deal, err := a.crmDealRepo.GetByID(ctx, dealID)
			if err != nil {
				return err
			}
			if deal == nil {
				return fmt.Errorf("deal not found")
			}
			deal.StageID = stageID
			return a.crmDealRepo.Update(ctx, deal)
		},
		AddDealNote: func(ctx context.Context, workspaceID, dealID, agentID, content string) error {
			if a.commandExecutor != nil {
				_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					WorkspaceID: workspaceID,
					AgentID:     agentID,
					TargetType:  "crm_deal",
					TargetID:    dealID,
				}, "crm.add_deal_note", mustJSON(map[string]any{
					"deal_id": dealID,
					"content": content,
				}))
				return err
			}
			// TODO(flow-platform): remove direct fallback once all native runs are command-backed.
			now := time.Now()
			activity := &model.CRMActivity{
				WorkspaceID:  workspaceID,
				ActivityType: "note",
				DealID:       &dealID,
				Body:         &content,
				OccurredAt:   now,
			}
			return a.crmActivityRepo.Create(ctx, activity)
		},
		ListContacts: func(ctx context.Context, workspaceID string, limit int) ([]model.CRMContact, error) {
			contacts, _, err := a.crmContactRepo.List(ctx, workspaceID, model.CRMContactListFilters{}, model.PMPagination{Page: 1, PerPage: limit})
			return contacts, err
		},
		ListBuyerSignals: func(ctx context.Context, workspaceID string, dealID *string, limit int) ([]model.CRMBuyerSignal, error) {
			filters := model.CRMBuyerSignalListFilters{DealID: dealID}
			signals, _, err := a.crmSignalRepo.ListSignals(ctx, workspaceID, filters, model.PMPagination{Page: 1, PerPage: limit})
			return signals, err
		},

		// Docs
		GetDocument: func(ctx context.Context, id string) (*model.DocsDocument, error) {
			return a.docsDocRepo.GetByID(ctx, id)
		},
		ListDocuments: func(ctx context.Context, workspaceID string, spaceID *string) ([]model.DocsDocument, error) {
			published := "published"
			return a.docsDocRepo.List(ctx, workspaceID, spaceID, nil, &published, nil, "", false)
		},
		SearchDocuments: func(ctx context.Context, workspaceID, query string, limit int) ([]workerpkg.DocsSearchHit, error) {
			results, err := a.docsSearchRepo.Search(ctx, workspaceID, query, nil, nil, limit)
			if err != nil {
				return nil, err
			}
			hits := make([]workerpkg.DocsSearchHit, len(results))
			for i, r := range results {
				hits[i] = workerpkg.DocsSearchHit{
					ID:    r.ID,
					Title: r.Title,
				}
			}
			return hits, nil
		},
		EnsureEpicSpecDoc: func(ctx context.Context, workspaceID, epicID, actorID string) (*model.DocsDocument, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("document commands are not available")
			}
			output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "epic",
				TargetID:    epicID,
			}, "docs.ensure_spec_doc", json.RawMessage(`{}`))
			if err != nil {
				return nil, err
			}
			var response struct {
				DocumentID string `json:"document_id"`
			}
			if err := json.Unmarshal(output, &response); err != nil {
				return nil, err
			}
			if strings.TrimSpace(response.DocumentID) == "" {
				return nil, fmt.Errorf("ensure spec doc returned no document_id")
			}
			return a.docsDocRepo.GetByID(ctx, response.DocumentID)
		},
		EnsureTaskPlanDoc: func(ctx context.Context, workspaceID, storyID, actorID string) (*model.DocsDocument, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("document commands are not available")
			}
			output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "task",
				TargetID:    storyID,
			}, "docs.ensure_task_plan_doc", json.RawMessage(`{}`))
			if err != nil {
				return nil, err
			}
			var response struct {
				DocumentID string `json:"document_id"`
			}
			if err := json.Unmarshal(output, &response); err != nil {
				return nil, err
			}
			if strings.TrimSpace(response.DocumentID) == "" {
				return nil, fmt.Errorf("ensure task plan doc returned no document_id")
			}
			return a.docsDocRepo.GetByID(ctx, response.DocumentID)
		},
		GetDocumentContent: func(ctx context.Context, documentID string) (string, error) {
			content, err := a.docsContentRepo.GetByDocumentID(ctx, documentID)
			if err != nil {
				return "", err
			}
			if content == nil {
				return "", nil
			}
			return content.ContentText, nil
		},
		WriteDocumentContent: func(ctx context.Context, workspaceID, documentID string, content json.RawMessage) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("document commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				TargetType:  "document",
				TargetID:    documentID,
			}, "docs.write_document_content", mustJSON(map[string]any{
				"document_id": documentID,
				"content":     json.RawMessage(content),
			}))
			return err
		},
		LinkDocumentToObject: func(ctx context.Context, workspaceID, documentID, linkedObjectType, linkedObjectID, linkContext, actorID string) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("document commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  linkedObjectType,
				TargetID:    linkedObjectID,
			}, "docs.link_document_to_object", mustJSON(map[string]any{
				"document_id":        documentID,
				"linked_object_type": linkedObjectType,
				"linked_object_id":   linkedObjectID,
				"link_context":       linkContext,
			}))
			return err
		},
	}
}

func (a *AgentRunActivities) pushVisitorConversationRefresh(ctx context.Context, workspaceID string, conversation *model.SupportConversation) {
	if conversation == nil || conversation.AnonymousID == nil || strings.TrimSpace(*conversation.AnonymousID) == "" || a.wsPublisher == nil {
		return
	}
	conversations, err := a.conversationRepo.ListByAnonymousID(ctx, workspaceID, *conversation.AnonymousID)
	if err != nil {
		return
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	listJSON, err := json.Marshal(map[string]any{"conversations": conversations})
	if err != nil {
		return
	}
	a.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_visitor_conversations",
		EntityID:    *conversation.AnonymousID,
		WorkspaceID: workspaceID,
		Data:        listJSON,
	})
}

func (a *AgentRunActivities) failRun(ctx context.Context, state *resolvedRunState, errMsg string) error {
	now := time.Now()
	state.run.Status = "failed"
	state.run.PauseReason = model.AgentRunPauseReasonNone
	state.run.CompletedAt = &now
	state.run.ErrorMessage = &errMsg
	state.run.ExecutionStage = strPtr("failed")
	state.run.LastHeartbeatAt = &now
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)
	if state.deliveryTarget != nil {
		state.deliveryTarget.DeliveryState = "failed"
		state.deliveryTarget.LastRunID = &state.run.ID
		state.deliveryTarget.LastSyncedAt = &now
		_ = a.deliveryRepo.Save(ctx, state.deliveryTarget)
	}
	if agent, err := a.agentRepo.GetByID(ctx, state.run.WorkspaceID, state.run.AgentID); err == nil && agent != nil {
		agent.Status = "error"
		agent.ActiveStoryID = nil
		_ = a.agentRepo.Update(ctx, agent)
	}
	return nil
}

func (a *AgentRunActivities) isRunExplicitlyCancelled(ctx context.Context, runID string) (bool, error) {
	if a == nil || a.runRepo == nil || strings.TrimSpace(runID) == "" {
		return false, nil
	}
	currentRun, err := a.runRepo.GetByIDAny(ctx, runID)
	if err != nil {
		return false, err
	}
	if currentRun == nil {
		return false, nil
	}
	return currentRun.Status == model.AgentRunStatusCancelled, nil
}

func (a *AgentRunActivities) MarkRunFailedActivity(ctx context.Context, runID, errMsg string) error {
	if a == nil || a.runRepo == nil {
		return nil
	}

	run, err := a.runRepo.GetByIDAny(ctx, runID)
	if err != nil || run == nil {
		return err
	}
	switch run.Status {
	case model.AgentRunStatusCompleted, model.AgentRunStatusFailed, model.AgentRunStatusCancelled:
		return nil
	}

	now := time.Now()
	run.Status = model.AgentRunStatusFailed
	run.PauseReason = model.AgentRunPauseReasonNone
	run.CompletedAt = &now
	run.ErrorMessage = strPtr(strings.TrimSpace(errMsg))
	run.ExecutionStage = strPtr("failed")
	run.LastHeartbeatAt = &now
	if err := a.runRepo.Update(ctx, run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, run)

	if a.agentRepo != nil {
		if agent, err := a.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID); err == nil && agent != nil {
			agent.Status = "error"
			agent.ActiveStoryID = nil
			_ = a.agentRepo.Update(ctx, agent)
		}
	}
	return nil
}

func ensureRunNotTerminal(run *model.AgentRun) error {
	if run == nil {
		return nil
	}
	switch run.Status {
	case "failed", "completed", "cancelled", "paused":
		return temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("agent run %s is already in terminal state %q", run.ID, run.Status),
			"AgentRunTerminalState",
			nil,
		)
	default:
		return nil
	}
}

func nonRetryableRunError(err error) error {
	if err == nil {
		return nil
	}
	return temporal.NewNonRetryableApplicationError(err.Error(), "AgentRunFailed", err)
}

func (a *AgentRunActivities) markAgentIdle(ctx context.Context, workspaceID, agentID string, tokens int) error {
	agent, err := a.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return err
	}
	agent.Status = "idle"
	agent.ActiveStoryID = nil
	agent.TokensUsedThisMonth += tokens
	return a.agentRepo.Update(ctx, agent)
}

func (a *AgentRunActivities) runGitInDir(ctx context.Context, dir string, integration *model.GitIntegration, accessToken string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmdArgs := append(gitAuthArgs(integration, accessToken), args...)
	cmd := exec.CommandContext(cmdCtx, "git", cmdArgs...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, string(output))
	}
	return string(output), nil
}

func gitAuthArgs(integration *model.GitIntegration, accessToken string) []string {
	if integration == nil || accessToken == "" {
		return nil
	}
	switch integration.Provider {
	case "github":
		auth := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + accessToken))
		return []string{"-c", "http.extraheader=Authorization: Basic " + auth}
	case "gitlab":
		auth := base64.StdEncoding.EncodeToString([]byte("oauth2:" + accessToken))
		return []string{"-c", "http.extraheader=Authorization: Basic " + auth}
	default:
		return nil
	}
}

func buildWorkingBranch(task *model.PMStory, teamDefault *model.PMTeamRepoDefault) string {
	template := "{display_id}-{slug}"
	if teamDefault != nil && strings.TrimSpace(teamDefault.BranchTemplate) != "" {
		template = teamDefault.BranchTemplate
	}
	replacements := map[string]string{
		"{display_id}": fmt.Sprintf("%d", task.DisplayID),
		"{slug}":       slugifyBranchToken(task.Name),
	}
	for placeholder, value := range replacements {
		template = strings.ReplaceAll(template, placeholder, value)
	}
	template = strings.ToLower(strings.TrimSpace(template))
	template = strings.Trim(template, "/-")
	if template == "" {
		return fmt.Sprintf("%d-%s", task.DisplayID, slugifyBranchToken(task.Name))
	}
	return template
}

func slugifyBranchToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = branchTokenSanitizer.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "task"
	}
	return value
}

func isEmptySummary(summary json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(summary))
	return trimmed == "" || trimmed == "{}" || trimmed == "null"
}

func runInputAdditionalContext(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var payload struct {
		AdditionalContext string `json:"additional_context"`
	}
	if err := json.Unmarshal(input, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.AdditionalContext)
}

func buildDurableRunFacts(state *resolvedRunState, input planningRunInput) map[string]string {
	facts := map[string]string{}
	if state == nil || state.run == nil {
		return facts
	}

	collectStructIDFacts(facts, "run", state.run)
	collectStructIDFacts(facts, "agent", state.agent)
	collectStructIDFacts(facts, "task", state.task)
	collectStructIDFacts(facts, "story", state.task) // backward-compat alias for "task"
	collectStructIDFacts(facts, "epic", state.epic)
	collectStructIDFacts(facts, "conversation", state.conversation)
	collectStructIDFacts(facts, "delivery_target", state.deliveryTarget)
	collectStructIDFacts(facts, "repository", state.repository)
	collectStructIDFacts(facts, "integration", state.integration)
	collectStructIDFacts(facts, "team_default", state.teamDefault)
	collectStructIDFacts(facts, "planning_input", input)
	collectJSONIDFacts(facts, "", state.run.Input)

	setFact(facts, "workspace_id", state.run.WorkspaceID)
	setFact(facts, "run_id", state.run.ID)
	setFact(facts, "agent_id", state.run.AgentID)
	setFact(facts, "target_type", state.run.TargetType)
	setFact(facts, "target_id", state.run.TargetID)
	setFact(facts, "task_id", firstNonEmptyString(derefString(state.run.StoryID), structID(state.task)))
	setFact(facts, "story_id", firstNonEmptyString(derefString(state.run.StoryID), structID(state.task)))
	setFact(facts, "conversation_id", firstNonEmptyString(derefString(state.run.ConversationID), structID(state.conversation)))
	setFact(facts, "epic_id", firstNonEmptyString(structID(state.epic), derefString(epicIDOfTask(state.task))))
	setFact(facts, "plan_document_id", firstNonEmptyString(input.PlanDocumentID, derefString(planDocumentIDOfTask(state.task))))
	setFact(facts, "spec_document_id", firstNonEmptyString(input.SpecDocumentID, derefString(specDocumentIDOfEpic(state.epic))))
	setFact(facts, "spec_version_id", input.SpecVersionID)
	setFact(facts, "approved_spec_version_id", derefString(approvedSpecVersionIDOfEpic(state.epic)))
	setFact(facts, "repository_id", firstNonEmptyString(derefString(state.run.RepositoryID), repositoryIDOfDeliveryTarget(state.deliveryTarget), structID(state.repository)))
	setFact(facts, "delivery_target_id", firstNonEmptyString(derefString(state.run.DeliveryTargetID), structID(state.deliveryTarget)))
	setFact(facts, "repo_full_name", repoFullName(state))
	setFact(facts, "base_branch", derefString(state.run.BaseBranch))
	setFact(facts, "working_branch", derefString(state.run.WorkingBranch))

	targetType := sanitizeFactKey(state.run.TargetType)
	if targetType != "" && strings.TrimSpace(state.run.TargetID) != "" {
		setFact(facts, targetType+"_id", state.run.TargetID)
	}

	return facts
}

func collectStructIDFacts(facts map[string]string, prefix string, value any) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}

	rt := rv.Type()
	for idx := 0; idx < rv.NumField(); idx++ {
		field := rt.Field(idx)
		if field.PkgPath != "" {
			continue
		}
		name := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}

		key := ""
		switch {
		case name == "id":
			key = prefix + "_id"
		case strings.HasSuffix(name, "_id"), strings.HasSuffix(name, "_ids"):
			key = prefix + "_" + name
		default:
			continue
		}

		if list, ok := reflectedStringSlice(rv.Field(idx)); ok && len(list) > 0 {
			setFact(facts, key, strings.Join(list, ", "))
			continue
		}
		if value, ok := reflectedString(rv.Field(idx)); ok {
			setFact(facts, key, value)
		}
	}
}

func collectJSONIDFacts(facts map[string]string, prefix string, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	collectJSONIDFactsValue(facts, prefix, payload)
}

func collectJSONIDFactsValue(facts map[string]string, prefix string, value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			key = sanitizeFactKey(key)
			if key == "" {
				continue
			}
			path := key
			if prefix != "" {
				path = prefix + "_" + key
			}
			if strings.HasSuffix(key, "_id") {
				if stringValue, ok := child.(string); ok {
					setFact(facts, path, stringValue)
				}
			} else if strings.HasSuffix(key, "_ids") {
				if list := jsonStringSlice(child); len(list) > 0 {
					setFact(facts, path, strings.Join(list, ", "))
				}
			}
			collectJSONIDFactsValue(facts, path, child)
		}
	case []any:
		for _, child := range typed {
			collectJSONIDFactsValue(facts, prefix, child)
		}
	}
}

func jsonFieldName(field reflect.StructField) string {
	tag := strings.TrimSpace(field.Tag.Get("json"))
	if tag == "" {
		return sanitizeFactKey(field.Name)
	}
	name := strings.TrimSpace(strings.Split(tag, ",")[0])
	if name == "" {
		return sanitizeFactKey(field.Name)
	}
	return name
}

func reflectedString(value reflect.Value) (string, bool) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.String {
		return "", false
	}
	text := strings.TrimSpace(value.String())
	if text == "" {
		return "", false
	}
	return text, true
}

func reflectedStringSlice(value reflect.Value) ([]string, bool) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Slice {
		return nil, false
	}
	items := make([]string, 0, value.Len())
	for idx := 0; idx < value.Len(); idx++ {
		item, ok := reflectedString(value.Index(idx))
		if ok {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil, false
	}
	return items, true
}

func jsonStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if ok && strings.TrimSpace(text) != "" {
			result = append(result, strings.TrimSpace(text))
		}
	}
	return result
}

func sanitizeFactKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func setFact(facts map[string]string, key, value string) {
	key = sanitizeFactKey(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	facts[key] = value
}

func structID(value any) string {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return ""
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return ""
	}
	field := rv.FieldByName("ID")
	if !field.IsValid() {
		return ""
	}
	id, ok := reflectedString(field)
	if !ok {
		return ""
	}
	return id
}

func epicIDOfTask(story *model.PMStory) *string {
	if story == nil {
		return nil
	}
	return story.EpicID
}

func planDocumentIDOfTask(story *model.PMStory) *string {
	if story == nil {
		return nil
	}
	return story.PlanDocumentID
}

func specDocumentIDOfEpic(epic *model.PMEpic) *string {
	if epic == nil {
		return nil
	}
	return epic.SpecDocumentID
}

func approvedSpecVersionIDOfEpic(epic *model.PMEpic) *string {
	if epic == nil {
		return nil
	}
	return epic.ApprovedSpecVersionID
}

func repositoryIDOfDeliveryTarget(target *model.StoryDeliveryTarget) string {
	if target == nil {
		return ""
	}
	return derefString(target.RepositoryID)
}

func repoFullName(state *resolvedRunState) string {
	if state.run != nil && state.run.RepoFullName != nil && *state.run.RepoFullName != "" {
		return *state.run.RepoFullName
	}
	if state.deliveryTarget != nil && state.deliveryTarget.RepoFullName != nil {
		return *state.deliveryTarget.RepoFullName
	}
	if state.repository != nil {
		return state.repository.FullName
	}
	return ""
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func strPtr(value string) *string {
	return &value
}

func resolvedAllowedToolSet(resolved workerpkg.ResolvedProfile) map[string]bool {
	set := make(map[string]bool, len(resolved.Tools))
	for _, toolName := range resolved.Tools {
		set[toolName] = true
	}
	return set
}

func effectiveToolSet(resolved workerpkg.ResolvedProfile, explicitAllowed []string) map[string]bool {
	if len(explicitAllowed) > 0 {
		return stringSliceToSet(explicitAllowed)
	}
	return resolvedAllowedToolSet(resolved)
}

func stringSliceToSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			set[item] = true
		}
	}
	return set
}

func mustJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
