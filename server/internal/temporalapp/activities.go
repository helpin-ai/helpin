package temporalapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/helpin-ai/helpin/server/internal/agentskills"
	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/gitlab"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

const (
	productSpecsSpaceSlug   = "product-specs"
	productSpecsSpaceName   = "Product Specs"
	githubAPIRequestTimeout = 30 * time.Second
)

var codexLivePausePollEvery = time.Second
var errBranchSyncUnrelatedHistory = errors.New("working branch does not share history with base branch")

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

type NotificationEmitter interface {
	Emit(ctx context.Context, event model.NotificationEventInput) error
}

type ReleaseFactsProvider interface {
	GetReleaseContext(ctx context.Context, workspaceID string, req model.GetReleaseContextRequest) (*model.ReleaseContextResult, error)
	FindTasksForGitChanges(ctx context.Context, workspaceID string, req model.FindTasksForGitChangesRequest) (*model.FindTasksForGitChangesResult, error)
	GetTaskContext(ctx context.Context, workspaceID string, req model.GetTaskContextRequest) (*model.GetTaskContextResult, error)
}

type CommandBarPlanAdvancer interface {
	AdvanceCommandBarPlanAfterRun(ctx context.Context, completedRunID string) (*model.AgentRun, error)
	StartReadyCommandBarPlanSteps(ctx context.Context, input CommandBarPlanWorkflowInput) (*CommandBarPlanProgress, error)
}

type gitLabTokenRefresher interface {
	RefreshToken(ctx context.Context, refreshToken string) (*gitlab.TokenResponse, error)
}

// AgentRunActivities contains the Temporal activities that execute an agent run.
type AgentRunActivities struct {
	runRepo                    *repository.AgentRunRepository
	runMessageRepo             *repository.AgentRunMessageRepository
	agentRepo                  *repository.AgentRepository
	workspaceSkillRepo         *repository.WorkspaceSkillRepository
	skillPackageStore          agentskills.SkillPackageStore
	artifactRepo               *repository.AgentRunArtifactRepository
	interactionRepo            *repository.AgentRunInteractionRepository
	sessionSnapshotRepo        *repository.CodingSessionStateSnapshotRepository
	taskRepo                   *repository.PMTaskRepository
	taskLinkRepo               *repository.PMTaskLinkRepository
	epicRepo                   *repository.PMEpicRepository
	conversationRepo           *repository.SupportConversationRepository
	commentRepo                *repository.PMCommentRepository
	checklistRepo              *repository.PMChecklistItemRepository
	workflowRepo               *repository.PMWorkflowRepository
	messageRepo                *repository.SupportMessageRepository
	gitIntRepo                 *repository.GitIntegrationRepository
	gitRepo                    *repository.GitRepositoryRepository
	gitLinkRepo                *repository.TaskGitLinkRepository
	deliveryRepo               *repository.TaskDeliveryTargetRepository
	settingsRepo               *repository.SettingsRepository
	workspaceRepo              *repository.WorkspaceRepository
	docsSpaceRepo              *repository.DocsSpaceRepository
	docsCollectionRepo         *repository.DocsCollectionRepository
	docsDocRepo                *repository.DocsDocumentRepository
	docsDocumentKeyRepo        *repository.DocsDocumentKeyRepository
	docsContentRepo            *repository.DocsContentRepository
	docsBlockRepo              *repository.DocsBlockRepository
	docsAISectionCandidateRepo *repository.DocsAISectionCandidateRepository
	docsChangeProposalRepo     *repository.DocsChangeProposalRepository
	docsVersionRepo            *repository.DocsVersionRepository
	docsLinkRepo               *repository.DocsLinkRepository
	docsSearchRepo             *repository.DocsSearchRepository
	crmDealRepo                *repository.CRMDealRepository
	crmContactRepo             *repository.CRMContactRepository
	crmSignalRepo              *repository.CRMSignalRepository
	crmActivityRepo            *repository.CRMActivityRepository
	commandExecutor            InternalCommandExecutor
	notificationEmitter        NotificationEmitter
	releaseFacts               ReleaseFactsProvider
	wsPublisher                websocket.EventPublisher
	runtimes                   *workerpkg.RuntimeRegistry
	githubApp                  *githubapp.Client
	gitlabClient               gitLabTokenRefresher
	gitCredentialRepo          *repository.GitCredentialRepository
	gitOAuthEncryptionKey      []byte
	runEngine                  *RunEngine
	commandBarAdvancer         CommandBarPlanAdvancer
}

// NewAgentRunActivities creates the activity set used by shared Temporal workers.
func NewAgentRunActivities(
	runRepo *repository.AgentRunRepository,
	runMessageRepo *repository.AgentRunMessageRepository,
	agentRepo *repository.AgentRepository,
	workspaceSkillRepo *repository.WorkspaceSkillRepository,
	skillPackageStore agentskills.SkillPackageStore,
	artifactRepo *repository.AgentRunArtifactRepository,
	interactionRepo *repository.AgentRunInteractionRepository,
	sessionSnapshotRepo *repository.CodingSessionStateSnapshotRepository,
	taskRepo *repository.PMTaskRepository,
	taskLinkRepo *repository.PMTaskLinkRepository,
	epicRepo *repository.PMEpicRepository,
	conversationRepo *repository.SupportConversationRepository,
	commentRepo *repository.PMCommentRepository,
	checklistRepo *repository.PMChecklistItemRepository,
	workflowRepo *repository.PMWorkflowRepository,
	messageRepo *repository.SupportMessageRepository,
	gitIntRepo *repository.GitIntegrationRepository,
	gitRepo *repository.GitRepositoryRepository,
	gitLinkRepo *repository.TaskGitLinkRepository,
	deliveryRepo *repository.TaskDeliveryTargetRepository,
	settingsRepo *repository.SettingsRepository,
	workspaceRepo *repository.WorkspaceRepository,
	docsSpaceRepo *repository.DocsSpaceRepository,
	docsCollectionRepo *repository.DocsCollectionRepository,
	docsDocRepo *repository.DocsDocumentRepository,
	docsDocumentKeyRepo *repository.DocsDocumentKeyRepository,
	docsContentRepo *repository.DocsContentRepository,
	docsBlockRepo *repository.DocsBlockRepository,
	docsAISectionCandidateRepo *repository.DocsAISectionCandidateRepository,
	docsChangeProposalRepo *repository.DocsChangeProposalRepository,
	docsVersionRepo *repository.DocsVersionRepository,
	docsLinkRepo *repository.DocsLinkRepository,
	docsSearchRepo *repository.DocsSearchRepository,
	crmDealRepo *repository.CRMDealRepository,
	crmContactRepo *repository.CRMContactRepository,
	crmSignalRepo *repository.CRMSignalRepository,
	crmActivityRepo *repository.CRMActivityRepository,
	commandExecutor InternalCommandExecutor,
	notificationEmitter NotificationEmitter,
	releaseFacts ReleaseFactsProvider,
	wsPublisher websocket.EventPublisher,
	runtimes *workerpkg.RuntimeRegistry,
	githubApp *githubapp.Client,
	gitlabClient gitLabTokenRefresher,
	gitCredentialRepo *repository.GitCredentialRepository,
	gitOAuthEncryptionKey []byte,
	runEngine *RunEngine,
	commandBarAdvancer CommandBarPlanAdvancer,
) *AgentRunActivities {
	return &AgentRunActivities{
		runRepo:                    runRepo,
		runMessageRepo:             runMessageRepo,
		agentRepo:                  agentRepo,
		workspaceSkillRepo:         workspaceSkillRepo,
		skillPackageStore:          skillPackageStore,
		artifactRepo:               artifactRepo,
		interactionRepo:            interactionRepo,
		sessionSnapshotRepo:        sessionSnapshotRepo,
		taskRepo:                   taskRepo,
		taskLinkRepo:               taskLinkRepo,
		epicRepo:                   epicRepo,
		conversationRepo:           conversationRepo,
		commentRepo:                commentRepo,
		checklistRepo:              checklistRepo,
		workflowRepo:               workflowRepo,
		messageRepo:                messageRepo,
		gitIntRepo:                 gitIntRepo,
		gitRepo:                    gitRepo,
		gitLinkRepo:                gitLinkRepo,
		deliveryRepo:               deliveryRepo,
		settingsRepo:               settingsRepo,
		workspaceRepo:              workspaceRepo,
		docsSpaceRepo:              docsSpaceRepo,
		docsCollectionRepo:         docsCollectionRepo,
		docsDocRepo:                docsDocRepo,
		docsDocumentKeyRepo:        docsDocumentKeyRepo,
		docsContentRepo:            docsContentRepo,
		docsBlockRepo:              docsBlockRepo,
		docsAISectionCandidateRepo: docsAISectionCandidateRepo,
		docsChangeProposalRepo:     docsChangeProposalRepo,
		docsVersionRepo:            docsVersionRepo,
		docsLinkRepo:               docsLinkRepo,
		docsSearchRepo:             docsSearchRepo,
		crmDealRepo:                crmDealRepo,
		crmContactRepo:             crmContactRepo,
		crmSignalRepo:              crmSignalRepo,
		crmActivityRepo:            crmActivityRepo,
		commandExecutor:            commandExecutor,
		notificationEmitter:        notificationEmitter,
		releaseFacts:               releaseFacts,
		wsPublisher:                wsPublisher,
		runtimes:                   runtimes,
		githubApp:                  githubApp,
		gitlabClient:               gitlabClient,
		gitCredentialRepo:          gitCredentialRepo,
		gitOAuthEncryptionKey:      append([]byte(nil), gitOAuthEncryptionKey...),
		runEngine:                  runEngine,
		commandBarAdvancer:         commandBarAdvancer,
	}
}

type resolvedRunState struct {
	run                        *model.AgentRun
	agent                      *model.Agent
	runtimeSkillRefs           model.AgentSkillRefs
	runtimeSkillDefinitions    []workerpkg.SkillDefinition
	skillPolicy                workerpkg.SkillPolicy
	nativeSelectivePathEnabled bool
	task                       *model.PMTask
	workspaceKey               string
	epic                       *model.PMEpic
	epicTasks                  []model.PMTask
	conversation               *model.SupportConversation
	resolved                   workerpkg.ResolvedProfile // merged class+agent overrides — use this for decisions
	deliveryTarget             *model.TaskDeliveryTarget
	repository                 *model.GitRepository
	integration                *model.GitIntegration
	teamDefault                *model.PMTeamRepoDefault
	accessToken                string
	branchSync                 branchSyncState
}

type branchSyncState struct {
	Status        string
	BaseBranch    string
	WorkingBranch string
	ConflictFiles []string
	BackupBranch  string
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

func replayMessagesForExecution(state *resolvedRunState, messages []model.AgentRunMessage) []model.AgentRunMessage {
	if state == nil || !state.nativeSelectivePathEnabled {
		return messages
	}
	filtered := make([]model.AgentRunMessage, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.MessageType) == "policy_retry" {
			continue
		}
		filtered = append(filtered, message)
	}
	return filtered
}

func providerContinuationMode(continuation *workerpkg.ProviderContinuation) string {
	if continuation == nil {
		return "fresh"
	}
	if strings.TrimSpace(continuation.ResponseID) != "" {
		return "response_id"
	}
	if strings.TrimSpace(continuation.PreviousResponseID) != "" {
		return "previous_response_id"
	}
	return "fresh"
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

func (a *AgentRunActivities) AdvanceCommandBarPlanActivity(ctx context.Context, runID string) error {
	if a == nil || a.commandBarAdvancer == nil {
		return nil
	}
	_, err := a.commandBarAdvancer.AdvanceCommandBarPlanAfterRun(ctx, runID)
	return err
}

func (a *AgentRunActivities) StartReadyCommandBarPlanStepsActivity(ctx context.Context, input CommandBarPlanWorkflowInput) (*CommandBarPlanProgress, error) {
	if a == nil || a.commandBarAdvancer == nil {
		return &CommandBarPlanProgress{Terminal: true, Status: "not_configured"}, nil
	}
	return a.commandBarAdvancer.StartReadyCommandBarPlanSteps(ctx, input)
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
	runtimeKind := executionRuntimeKind(state)
	initialInstructions, err := a.buildInitialInstructions(ctx, state, planningInput)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	legacyInitialInstructions, phaseGuidance := splitNativePhaseGuidance(runtimeKind, state, initialInstructions)

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
	history, artifactContext, providerContinuation, repairInstruction, err := a.ensureRunConversation(ctx, state, legacyInitialInstructions, planningInput)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	slog.InfoContext(ctx, "agent run preparing workspace",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"repo", repoFullName(state),
	)
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
			if err := a.syncBaseIntoWorkingBranch(ctx, workDir, state); err != nil {
				if persistWorkspace {
					_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
				}
				_ = a.failRun(ctx, state, err.Error())
				return ExecuteRunResult{}, nonRetryableRunError(err)
			}
		} else {
			slog.InfoContext(ctx, "agent run reusing existing workspace checkout",
				"workspace_id", state.run.WorkspaceID,
				"run_id", state.run.ID,
				"repo", state.repository.FullName,
				"work_dir", workDir,
			)
		}
	}

	config := workerpkg.ParseWorkflowConfigForAgent(workDir, state.agent)
	if config == nil {
		config = workerpkg.DefaultWorkflowConfigForAgent(state.agent)
	}

	allowedTools := effectiveToolSet(state.resolved, planningInput.AllowedTools)
	activeSkillSelection := selectNativeActiveSkills(state, planningInput.Stage)
	state.skillPolicy = effectiveExecutionSkillPolicy(state, activeSkillSelection)

	execCtx := a.buildRuntimeExecutionContext(ctx, state, runtimeExecutionContextInput{
		workDir:                   workDir,
		runtimeKind:               runtimeKind,
		planningInput:             planningInput,
		legacyInitialInstructions: legacyInitialInstructions,
		phaseGuidance:             phaseGuidance,
		repairInstruction:         repairInstruction,
		allowedTools:              allowedTools,
		activeSkillSelection:      activeSkillSelection,
		config:                    config,
		artifactContext:           artifactContext,
		providerContinuation:      providerContinuation,
		history:                   history,
	})

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
		"preset_key", strings.TrimSpace(state.agent.EffectivePresetKey()),
		"target_type", strings.TrimSpace(state.run.TargetType),
		"native_selective_path_enabled", state.nativeSelectivePathEnabled,
		"continuation_mode", providerContinuationMode(providerContinuation),
		"runtime_skill_refs", runtimeSkillRefKeys(state.runtimeSkillRefs),
		"active_skill_refs", runtimeSkillRefKeys(activeSkillSelection.Refs),
		"active_policy_required_interactions", completionRequiredInteractionKinds(state.skillPolicy),
	)
	var repoSkillMask *workerpkg.RepoSkillMask
	if runtimeKind == "codex" || runtimeKind == "opencode" {
		repoSkillMask, err = workerpkg.MaskRepoSkillRoots(workDir, state.run.ID)
		if err != nil {
			if persistWorkspace {
				_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
			}
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
		if repoSkillMask != nil {
			defer func() {
				if restoreErr := repoSkillMask.Restore(); restoreErr != nil && !errors.Is(restoreErr, os.ErrNotExist) {
					slog.WarnContext(ctx, "failed to restore masked repo skill roots",
						"error", restoreErr,
						"run_id", state.run.ID,
						"runtime_kind", runtimeKind)
				}
			}()
		}
	}
	if runtimeKind == "codex" || runtimeKind == "opencode" {
		if err := a.stageRuntimeSkills(ctx, state, execCtx, runtimeKind); err != nil {
			if persistWorkspace {
				_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
			}
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
	}

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
		a.salvageFailedRuntimeStateFromSnapshotStore(bgCtx, state)
		_ = a.failRun(bgCtx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	slog.InfoContext(ctx, "agent run runtime execution completed",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"runtime_kind", runtimeKind,
	)
	if runtimeKind == "codex" {
		if err := a.pushCodexLocalCommit(ctx, workDir, state, execCtx); err != nil {
			if persistWorkspace {
				_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
			}
			a.salvageFailedRuntimeStateFromSnapshotStore(ctx, state)
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
	}
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
	reviewRequest := latestExecutionReviewCheckpointRequest(execCtx)
	humanInputRequest := latestExecutionHumanInputRequest(execCtx)
	authRequest := latestExecutionCodexAuthState(execCtx)
	if approvalRequest == nil && reviewRequest == nil && humanInputRequest == nil && authRequest == nil {
		synthesizedReview, synthesizedInput, err := a.synthesizeCompletionInteractionFallback(ctx, state, assistantMessage)
		if err != nil {
			if persistWorkspace {
				_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
			}
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
		if synthesizedReview != nil {
			reviewRequest = synthesizedReview
		}
		if synthesizedInput != nil {
			humanInputRequest = synthesizedInput
		}
	}
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

	waitForApproval, waitForInput, waitForAuth := resolveExecutionWaitState(state.run, humanInputRequest, approvalRequest, reviewRequest, authRequest)
	continueExecution := false
	if state.run.InvocationMode == model.InvocationModeInteractive && humanInputRequest != nil {
	}
	if !waitForApproval && !waitForInput && !waitForAuth && !continueExecution {
		if err := a.enforceCompletionInteractionPolicy(ctx, state, assistantMessage); err != nil {
			retried, retryErr := a.retryInvalidCompletionTurn(ctx, state, assistantMessage, err)
			if retryErr != nil {
				if persistWorkspace {
					_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
				}
				_ = a.failRun(ctx, state, retryErr.Error())
				return ExecuteRunResult{}, nonRetryableRunError(retryErr)
			}
			if retried {
				now := time.Now()
				state.run.Status = model.AgentRunStatusRunning
				state.run.PauseReason = model.AgentRunPauseReasonNone
				state.run.CompletedAt = nil
				state.run.ExecutionStage = strPtr("continuing")
				state.run.LastHeartbeatAt = &now
				if err := a.runRepo.Update(ctx, state.run); err != nil {
					return ExecuteRunResult{}, err
				}
				a.runRepo.Notify(ctx, state.run)
				return ExecuteRunResult{ContinueExecution: true}, nil
			}
			if persistWorkspace {
				_ = workerpkg.CleanupWorkspaceForRun(state.run.ID)
			}
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
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

func (a *AgentRunActivities) stageRuntimeSkills(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext, runtimeKind string) error {
	if a == nil || state == nil || state.run == nil || state.agent == nil || execCtx == nil {
		return nil
	}
	if len(agentskills.EffectiveRuntimeRefs(state.agent)) == 0 {
		return nil
	}
	stageRoot := workerpkg.RuntimeSkillRootPathForRun(state.run.ID, runtimeKind)
	resolution, err := agentskills.StageInto(
		ctx,
		state.run.WorkspaceID,
		state.agent,
		state.resolved.Tools,
		a.workspaceSkillRepo,
		a.skillPackageStore,
		stageRoot,
	)
	if err != nil {
		return fmt.Errorf("stage runtime skills: %w", err)
	}
	execCtx.StagedRuntimeSkillRoot = stageRoot
	a.persistRuntimeSkillManifest(ctx, state.run, runtimeKind, stageRoot, resolution)
	return nil
}

func (a *AgentRunActivities) persistRuntimeSkillManifest(ctx context.Context, run *model.AgentRun, runtimeKind, stageRoot string, resolution agentskills.Resolution) {
	if a == nil || a.artifactRepo == nil || run == nil {
		return
	}
	type skillManifestEntry struct {
		Key        string  `json:"key"`
		SourceKind string  `json:"source_kind"`
		VersionKey *string `json:"version_key,omitempty"`
		SkillID    *string `json:"skill_id,omitempty"`
	}
	entries := make([]skillManifestEntry, 0, len(resolution.Refs))
	for idx, ref := range resolution.Refs {
		definition := resolution.Definitions[idx]
		entries = append(entries, skillManifestEntry{
			Key:        definition.Key,
			SourceKind: definition.SourceKind,
			VersionKey: ref.VersionKey,
			SkillID:    ref.SkillID,
		})
	}
	payload, err := json.Marshal(map[string]any{
		"runtime_kind": runtimeKind,
		"staged_root":  stageRoot,
		"skills":       entries,
	})
	if err != nil {
		slog.WarnContext(ctx, "failed to marshal runtime skill manifest", "error", err, "run_id", run.ID)
		return
	}
	seqNo, err := a.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		slog.WarnContext(ctx, "failed to allocate runtime skill manifest sequence", "error", err, "run_id", run.ID)
		return
	}
	content := string(payload)
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  "runtime_skill_manifest",
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &content,
		Metadata:      json.RawMessage("{}"),
		SequenceNo:    seqNo,
	}
	if err := a.artifactRepo.Create(ctx, artifact); err != nil {
		slog.WarnContext(ctx, "failed to persist runtime skill manifest", "error", err, "run_id", run.ID)
	}
}

func resolveExecutionWaitState(run *model.AgentRun, humanInputRequest *workerpkg.UserInputRequest, approvalRequest *model.ApprovalRequest, reviewRequest *model.ReviewCheckpointRequest, authRequest *model.CodexAuthState) (waitForApproval bool, waitForInput bool, waitForAuth bool) {
	if run == nil {
		return false, false, false
	}

	if approvalRequest != nil || reviewRequest != nil {
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

func normalizeApprovalPreviewPanelKey(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	switch key {
	case workerpkg.ToolPublishPRDDraft:
		return "prd_draft"
	case workerpkg.ToolPublishTaskPlan:
		return "task_plan"
	case workerpkg.ToolPublishTaskPlanDoc:
		return "task_plan_doc"
	default:
		return key
	}
}

func isCanonicalApprovalPreviewPanelKey(value string) bool {
	switch normalizeApprovalPreviewPanelKey(value) {
	case "prd_draft", "task_plan", "task_plan_doc":
		return true
	default:
		return false
	}
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
	if reviewRequest := latestHumanReviewCheckpointRequestFromResult(result); reviewRequest != nil {
		stage = "awaiting_review"
		if strings.TrimSpace(reviewRequest.Phase) != "" {
			stage = strings.TrimSpace(reviewRequest.Phase)
		}
		return model.AgentRunPauseReasonHumanApproval, stage, nil
	}
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
	if a == nil || a.artifactRepo == nil || state == nil || state.run == nil || execCtx == nil || execCtx.LastExecutionResult == nil {
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
	skillRefs := agentskills.EffectiveRuntimeRefs(agent)
	skillResolution, err := agentskills.Resolve(ctx, run.WorkspaceID, skillRefs, a.workspaceSkillRepo)
	if err != nil {
		return nil, err
	}
	agent.Skills = skillResolution.Refs
	if err := agentskills.ValidateRuntimeAndTools(agent.RuntimeKind, resolved.Tools, skillResolution.Definitions); err != nil {
		return nil, err
	}
	agent.ResolvedSkillInstructions = agentskills.CompileInstructions(skillResolution.Definitions)

	if run.TargetType == "" {
		switch {
		case run.TaskID != nil:
			run.TargetType = "task"
			run.TargetID = *run.TaskID
		case run.ConversationID != nil:
			run.TargetType = "support_conversation"
			run.TargetID = *run.ConversationID
		}
	}

	nativeSelectivePathEnabled := resolveNativeSelectivePlannerPathEnabled(run, agent)
	state := &resolvedRunState{
		run:                        run,
		agent:                      agent,
		runtimeSkillRefs:           skillResolution.Refs,
		runtimeSkillDefinitions:    skillResolution.Definitions,
		skillPolicy:                agentskills.AggregatePolicy(skillResolution.Definitions),
		nativeSelectivePathEnabled: nativeSelectivePathEnabled,
		resolved:                   resolved,
	}
	slog.InfoContext(ctx, "resolved run state",
		"workspace_id", run.WorkspaceID,
		"run_id", run.ID,
		"agent_id", run.AgentID,
		"runtime_kind", executionRuntimeKind(state),
		"preset_key", strings.TrimSpace(agent.EffectivePresetKey()),
		"target_type", strings.TrimSpace(run.TargetType),
		"native_selective_path_enabled", nativeSelectivePathEnabled,
		"runtime_skill_refs", runtimeSkillRefKeys(skillResolution.Refs),
	)

	if run.TaskID != nil {
		task, err := a.taskRepo.GetRawByID(ctx, *run.TaskID)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, fmt.Errorf("task not found")
		}
		state.task = task

		if a.workspaceRepo != nil {
			ws, wsErr := a.workspaceRepo.GetByID(ctx, run.WorkspaceID)
			if wsErr == nil && ws != nil {
				state.workspaceKey = ws.WorkspaceKey
			}
		}

		target, teamDefault, err := a.resolveDeliveryTarget(ctx, run.WorkspaceID, task)
		if err != nil {
			return nil, err
		}
		state.deliveryTarget = target
		state.teamDefault = teamDefault

		if target != nil && target.RepositoryID != nil && *target.RepositoryID != "" {
			repo, err := a.gitRepo.GetByIDAny(ctx, *target.RepositoryID)
			if err != nil {
				return nil, err
			}
			if repo != nil {
				if repo.WorkspaceID != run.WorkspaceID {
					return nil, fmt.Errorf("delivery target repository does not belong to this workspace")
				}
				if repo.DeletedAt != nil || !repo.Active {
					return nil, fmt.Errorf("delivery target repository is inactive")
				}
			}
			state.repository = repo
		}
		if target != nil && target.IntegrationID != nil && *target.IntegrationID != "" {
			integration, err := a.gitIntRepo.GetByIDAny(ctx, *target.IntegrationID)
			if err != nil {
				return nil, err
			}
			if integration != nil && !integration.Active {
				return nil, fmt.Errorf("delivery target integration is inactive")
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
		epicTasks, err := a.epicRepo.ListTasks(ctx, run.TargetID)
		if err != nil {
			return nil, err
		}
		state.epicTasks = epicTasks
	}

	if err := a.hydrateRunRepositoryTarget(ctx, state); err != nil {
		return nil, err
	}

	return state, nil
}

func (a *AgentRunActivities) hydrateRunRepositoryTarget(ctx context.Context, state *resolvedRunState) error {
	if a == nil || state == nil || state.run == nil {
		return nil
	}
	if state.repository != nil && state.integration != nil && strings.TrimSpace(state.accessToken) != "" {
		return nil
	}

	targetType := strings.TrimSpace(state.run.TargetType)
	repoID := strings.TrimSpace(derefString(state.run.RepositoryID))
	if targetType == "repository" && repoID == "" {
		repoID = strings.TrimSpace(state.run.TargetID)
	}
	if targetType != "repository" {
		switch targetType {
		case "task", "story", "epic":
			return nil
		}
		if repoID == "" || state.repository != nil {
			return nil
		}
	}
	if repoID == "" {
		return fmt.Errorf("repository target id is required")
	}
	if a.gitRepo == nil {
		return fmt.Errorf("git repository repository is not configured")
	}
	if a.gitIntRepo == nil {
		return fmt.Errorf("git integration repository is not configured")
	}

	repo, err := a.gitRepo.GetByIDAny(ctx, repoID)
	if err != nil {
		return err
	}
	if repo == nil {
		return fmt.Errorf("repository target not found")
	}
	if repo.WorkspaceID != state.run.WorkspaceID {
		return fmt.Errorf("repository target does not belong to this workspace")
	}
	if repo.DeletedAt != nil || !repo.Active || repo.Archived || !repo.Selected {
		return fmt.Errorf("repository target is not available")
	}

	integration, err := a.gitIntRepo.GetByIDAny(ctx, repo.IntegrationID)
	if err != nil {
		return err
	}
	if integration == nil || !integration.Active {
		return fmt.Errorf("repository target integration not found")
	}

	token, err := a.mintAccessToken(ctx, integration)
	if err != nil {
		return fmt.Errorf("repository target access token: %w", err)
	}

	state.repository = repo
	state.integration = integration
	state.accessToken = token
	state.run.RepositoryID = &repo.ID
	state.run.RepoFullName = &repo.FullName
	if strings.TrimSpace(derefString(state.run.BaseBranch)) == "" {
		baseBranch := strings.TrimSpace(repo.DefaultBranch)
		if baseBranch == "" {
			baseBranch = "main"
		}
		state.run.BaseBranch = &baseBranch
	}
	return nil
}

func (a *AgentRunActivities) resolveDeliveryTarget(ctx context.Context, workspaceID string, task *model.PMTask) (*model.TaskDeliveryTarget, *model.PMTeamRepoDefault, error) {
	target, err := a.deliveryRepo.GetByTask(ctx, workspaceID, task.ID)
	if err != nil {
		return nil, nil, err
	}
	if target != nil {
		var teamDefault *model.PMTeamRepoDefault
		if task.TeamID != nil && *task.TeamID != "" {
			teamDefault, err = a.settingsRepo.GetTeamRepoDefault(ctx, *task.TeamID)
			if err != nil {
				return nil, nil, err
			}
		}
		return target, teamDefault, nil
	}

	target = &model.TaskDeliveryTarget{
		WorkspaceID:   workspaceID,
		TaskID:        task.ID,
		DeliveryState: "unconfigured",
	}

	var teamDefault *model.PMTeamRepoDefault
	if task.TeamID != nil && *task.TeamID != "" {
		teamDefault, err = a.settingsRepo.GetTeamRepoDefault(ctx, *task.TeamID)
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

func (a *AgentRunActivities) preparePlanningRepository(ctx context.Context, state *resolvedRunState, input planningRunInput) error {
	if state.run.TargetType != "epic" || state.epic == nil {
		return nil
	}
	if state.epic.PlanningRepositoryID == nil || strings.TrimSpace(*state.epic.PlanningRepositoryID) == "" {
		return nil
	}

	repo, err := a.gitRepo.GetByIDAny(ctx, *state.epic.PlanningRepositoryID)
	if err != nil {
		return err
	}
	if repo == nil {
		return fmt.Errorf("planning repository not found")
	}
	if repo.WorkspaceID != state.run.WorkspaceID {
		return fmt.Errorf("planning repository does not belong to this workspace")
	}
	if repo.DeletedAt != nil || !repo.Active || repo.Archived || !repo.Selected {
		return fmt.Errorf("planning repository is not available")
	}

	integration, err := a.gitIntRepo.GetByIDAny(ctx, repo.IntegrationID)
	if err != nil {
		return err
	}
	if integration == nil || !integration.Active {
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
	case "pm.task_completion_followups":
		var assessment model.TaskCompletionAssessment
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

func runActorID(run *model.AgentRun) string {
	if run.TriggeredByUserID != nil && strings.TrimSpace(*run.TriggeredByUserID) != "" {
		return *run.TriggeredByUserID
	}
	return run.AgentID
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

func validatePlanningProposalTasks(stories []model.ProposedTask) error {
	return model.NormalizeProposedTasks(stories)
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
	if integration.Provider == "gitlab" && integration.CredentialID != nil && strings.TrimSpace(*integration.CredentialID) != "" {
		if a.gitCredentialRepo == nil || len(a.gitOAuthEncryptionKey) != 32 {
			return "", fmt.Errorf("gitlab oauth credentials are not configured")
		}
		credential, err := a.gitCredentialRepo.GetByID(ctx, *integration.CredentialID)
		if err != nil {
			return "", err
		}
		if credential == nil || credential.AccessTokenEncrypted == nil || strings.TrimSpace(*credential.AccessTokenEncrypted) == "" {
			return "", fmt.Errorf("gitlab credential is not available")
		}
		if credential.ExpiresAt != nil && time.Until(*credential.ExpiresAt) < 2*time.Minute && credential.RefreshTokenEncrypted != nil && a.gitlabClient != nil {
			refreshToken, err := appcrypto.DecryptString(*credential.RefreshTokenEncrypted, a.gitOAuthEncryptionKey)
			if err == nil && strings.TrimSpace(refreshToken) != "" {
				if refreshed, refreshErr := a.gitlabClient.RefreshToken(ctx, refreshToken); refreshErr == nil && strings.TrimSpace(refreshed.AccessToken) != "" {
					if updateErr := a.updateGitLabCredentialToken(ctx, credential, refreshed); updateErr != nil {
						slog.WarnContext(ctx, "gitlab oauth token update failed", "credential_id", credential.ID, "error", updateErr)
					}
				} else if refreshErr != nil {
					slog.WarnContext(ctx, "gitlab oauth token refresh failed", "credential_id", credential.ID, "error", refreshErr)
				}
			}
			credential, err = a.gitCredentialRepo.GetByID(ctx, credential.ID)
			if err != nil {
				return "", err
			}
			if credential == nil || credential.AccessTokenEncrypted == nil || strings.TrimSpace(*credential.AccessTokenEncrypted) == "" {
				return "", fmt.Errorf("gitlab credential is not available")
			}
		}
		return appcrypto.DecryptString(*credential.AccessTokenEncrypted, a.gitOAuthEncryptionKey)
	}
	if integration.AccessToken != "" {
		return integration.AccessToken, nil
	}
	return "", fmt.Errorf("git integration has no usable credentials")
}

func (a *AgentRunActivities) updateGitLabCredentialToken(ctx context.Context, credential *model.GitCredential, token *gitlab.TokenResponse) error {
	if credential == nil || token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return nil
	}
	accessToken, err := appcrypto.EncryptString(token.AccessToken, a.gitOAuthEncryptionKey)
	if err != nil {
		return err
	}
	credential.AccessTokenEncrypted = &accessToken
	if strings.TrimSpace(token.RefreshToken) != "" {
		refreshToken, err := appcrypto.EncryptString(token.RefreshToken, a.gitOAuthEncryptionKey)
		if err != nil {
			return err
		}
		credential.RefreshTokenEncrypted = &refreshToken
	}
	if token.ExpiresIn > 0 {
		expiresAt := time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second)
		credential.ExpiresAt = &expiresAt
	}
	return a.gitCredentialRepo.Update(ctx, credential)
}

func (a *AgentRunActivities) serviceBridge() *workerpkg.ServiceBridge {
	return &workerpkg.ServiceBridge{
		ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("internal commands are not available")
			}
			return a.commandExecutor.Execute(ctx, meta, name, input)
		},
		AddComment: func(ctx context.Context, workspaceID, taskID, agentID, content string) error {
			comment := &model.PMComment{
				EntityType: "task",
				EntityID:   taskID,
				AuthorID:   agentID,
				Body:       content,
			}
			return a.commentRepo.Create(ctx, comment)
		},
		UpdateTaskState: func(ctx context.Context, workspaceID, taskID, stateID string) error {
			if a.commandExecutor != nil {
				_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					WorkspaceID: workspaceID,
					TargetType:  "task",
					TargetID:    taskID,
				}, "pm.update_task_state", mustJSON(map[string]any{
					"task_id":  taskID,
					"state_id": stateID,
				}))
				return err
			}
			// TODO(flow-platform): remove direct fallback once all native runs are command-backed.
			task, err := a.taskRepo.GetRawByID(ctx, taskID)
			if err != nil {
				return err
			}
			if task == nil {
				return fmt.Errorf("task not found")
			}
			task.WorkflowStateID = stateID
			return a.taskRepo.Update(ctx, task)
		},
		ListChecklist: func(ctx context.Context, workspaceID, taskID string) ([]model.PMChecklistItem, error) {
			return a.checklistRepo.List(ctx, taskID)
		},
		CreateTaskBatch: func(ctx context.Context, workspaceID, epicID, actorID string, tasks []model.ProposedTask) (workerpkg.CreateTaskBatchResult, error) {
			if a.commandExecutor != nil {
				output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					WorkspaceID: workspaceID,
					ActorID:     actorID,
					TargetType:  "epic",
					TargetID:    epicID,
				}, "pm.create_task_batch", mustJSON(map[string]any{
					"tasks": tasks,
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
		AssignTaskAgent: func(ctx context.Context, workspaceID, actorID, taskID, agentID string) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("planner commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "task",
				TargetID:    taskID,
			}, "pm.assign_task_agent", mustJSON(map[string]any{
				"task_id":  taskID,
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
			tasks, err := a.epicRepo.ListTasks(ctx, epicID)
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
					TaskType:        s.TaskType,
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
		ListTeamWorkflows: func(ctx context.Context, workspaceID string, teamID *string) ([]workerpkg.TeamWorkflowSummary, error) {
			if a.settingsRepo == nil || a.workflowRepo == nil {
				return nil, fmt.Errorf("team workflows are not available")
			}
			teams, err := a.settingsRepo.ListTeams(ctx, workspaceID)
			if err != nil {
				return nil, err
			}

			type teamRecord struct {
				ID   string
				Name string
			}
			selected := make([]teamRecord, 0, len(teams))
			if teamID != nil && strings.TrimSpace(*teamID) != "" {
				needle := strings.TrimSpace(*teamID)
				for _, team := range teams {
					if strings.TrimSpace(team.ID) == needle {
						selected = append(selected, teamRecord{ID: team.ID, Name: team.Name})
						break
					}
				}
				if len(selected) == 0 {
					return nil, fmt.Errorf("team not found")
				}
			} else {
				for _, team := range teams {
					selected = append(selected, teamRecord{ID: team.ID, Name: team.Name})
				}
			}

			defaultWorkflow, err := a.workflowRepo.GetDefaultWorkflow(ctx, workspaceID)
			if err != nil {
				return nil, err
			}

			result := make([]workerpkg.TeamWorkflowSummary, 0, len(selected))
			for _, team := range selected {
				workflow, err := a.workflowRepo.GetByTeamID(ctx, workspaceID, team.ID)
				if err != nil {
					return nil, err
				}
				usesTeamWorkflow := true
				if workflow == nil {
					workflow = defaultWorkflow
					usesTeamWorkflow = false
				}
				if workflow == nil {
					continue
				}
				stages := make([]workerpkg.WorkflowStageSummary, 0, len(workflow.States))
				for _, state := range workflow.States {
					stages = append(stages, workerpkg.WorkflowStageSummary{
						ID:        state.ID,
						Name:      state.Name,
						StateType: state.StateType,
						Position:  state.Position,
						IsDefault: state.IsDefault || (workflow.Workflow.DefaultStateID != nil && *workflow.Workflow.DefaultStateID == state.ID),
					})
				}
				result = append(result, workerpkg.TeamWorkflowSummary{
					TeamID:           team.ID,
					TeamName:         team.Name,
					WorkflowID:       workflow.Workflow.ID,
					WorkflowName:     workflow.Workflow.Name,
					DefaultStateID:   workflow.Workflow.DefaultStateID,
					UsesTeamWorkflow: usesTeamWorkflow,
					Stages:           stages,
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
		GetDocumentKey: func(ctx context.Context, workspaceID, keyType, key string) (*model.DocsDocumentKey, error) {
			if a.docsDocumentKeyRepo == nil {
				return nil, fmt.Errorf("docs document key repository is not available")
			}
			return a.docsDocumentKeyRepo.GetByKey(ctx, workspaceID, keyType, key)
		},
		ListCollections: func(ctx context.Context, workspaceID string, spaceID *string) ([]model.DocsCollection, error) {
			if spaceID != nil && strings.TrimSpace(*spaceID) != "" {
				return a.docsCollectionRepo.ListByWorkspaceAndSpace(ctx, workspaceID, strings.TrimSpace(*spaceID))
			}
			return a.docsCollectionRepo.ListByWorkspace(ctx, workspaceID)
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
		CreateDocument: func(ctx context.Context, workspaceID, userID string, req model.CreateDocsDocumentRequest, content json.RawMessage) (*model.DocsDocument, error) {
			if strings.TrimSpace(req.SpaceID) == "" {
				return nil, fmt.Errorf("space_id is required")
			}
			if strings.TrimSpace(req.Title) == "" {
				return nil, fmt.Errorf("title is required")
			}
			space, err := a.docsSpaceRepo.GetByID(ctx, req.SpaceID)
			if err != nil {
				return nil, err
			}
			if space == nil {
				return nil, fmt.Errorf("space not found")
			}
			if space.WorkspaceID != workspaceID {
				return nil, fmt.Errorf("space does not belong to this workspace")
			}
			collectionID := req.CollectionID
			if collectionID != nil && strings.TrimSpace(*collectionID) == "" {
				collectionID = nil
			}
			teamID := space.TeamID
			if teamID != nil && strings.TrimSpace(*teamID) == "" {
				teamID = nil
			}
			nextPos, err := a.docsDocRepo.NextPosition(ctx, req.SpaceID, collectionID)
			if err != nil {
				return nil, err
			}
			doc, err := a.docsDocRepo.Create(ctx, &model.DocsDocument{
				WorkspaceID:  workspaceID,
				SpaceID:      req.SpaceID,
				CollectionID: collectionID,
				Title:        req.Title,
				Status:       model.DocStatusDraft,
				Visibility:   model.SpaceVisibilityWorkspaceWide,
				OwnerID:      req.OwnerID,
				TeamID:       teamID,
				TemplateKey:  req.TemplateKey,
				Icon:         req.Icon,
				Tags:         model.DocsStringArray(req.Tags),
				Position:     nextPos,
				CreatedBy:    userID,
			})
			if err != nil {
				return nil, err
			}
			if len(strings.TrimSpace(string(content))) > 0 && strings.TrimSpace(string(content)) != "null" {
				if _, err := a.docsContentRepo.Upsert(ctx, doc.ID, content); err != nil {
					return nil, err
				}
			}
			if a.wsPublisher != nil {
				a.wsPublisher.Publish(websocket.Event{
					Action:      "created",
					Entity:      "docs_document",
					EntityID:    doc.ID,
					WorkspaceID: workspaceID,
					ActorID:     userID,
					ParentType:  "docs_space",
					ParentID:    doc.SpaceID,
				})
			}
			return doc, nil
		},
		CreateTask: func(ctx context.Context, workspaceID, actorID string, req workerpkg.CreateTaskToolRequest) (*workerpkg.CreateTaskToolResult, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("task creation is not available")
			}
			payload, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "workspace",
				TargetID:    workspaceID,
			}, "pm.create_task", mustJSON(map[string]any{
				"name":             req.Name,
				"description":      req.Description,
				"task_type":        req.TaskType,
				"estimate":         req.Estimate,
				"priority":         req.Priority,
				"epic_id":          req.EpicID,
				"team_id":          req.TeamID,
				"workflow_id":      req.WorkflowID,
				"state_id":         req.StateID,
				"owner_member_ids": req.OwnerMemberIDs,
				"label_ids":        req.LabelIDs,
				"deadline":         req.Deadline,
			}))
			if err != nil {
				return nil, err
			}
			var result workerpkg.CreateTaskToolResult
			if err := json.Unmarshal(payload, &result); err != nil {
				return nil, err
			}
			return &result, nil
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
		EnsureTaskPlanDoc: func(ctx context.Context, workspaceID, taskID, actorID string) (*model.DocsDocument, error) {
			if a.commandExecutor == nil {
				return nil, fmt.Errorf("document commands are not available")
			}
			output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "task",
				TargetID:    taskID,
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
		ListDocumentBlocks: func(ctx context.Context, documentID string) ([]model.DocsBlock, error) {
			if a.docsBlockRepo == nil {
				return []model.DocsBlock{}, nil
			}
			return a.docsBlockRepo.ListByDocument(ctx, documentID, false)
		},
		PublishAISectionCandidate: func(ctx context.Context, workspaceID, documentID, blockID, agentID, runID string, currentContent, candidateContent json.RawMessage, candidateText string, sourceRefs model.JSONB, prompt *string, promptHash *string, modelName *string) (*model.DocsAISectionCandidate, error) {
			if a.docsAISectionCandidateRepo == nil {
				return nil, fmt.Errorf("AI section candidate repository is not available")
			}
			doc, err := a.docsDocRepo.GetByID(ctx, documentID)
			if err != nil {
				return nil, err
			}
			if doc == nil || doc.WorkspaceID != workspaceID {
				return nil, fmt.Errorf("document not found")
			}
			candidate := &model.DocsAISectionCandidate{
				WorkspaceID:      workspaceID,
				DocumentID:       documentID,
				BlockID:          blockID,
				AgentRunID:       strPtr(runID),
				Status:           model.DocsAISectionCandidateStatusReady,
				CurrentContent:   currentContent,
				CandidateContent: candidateContent,
				CandidateText:    candidateText,
				SourceRefs:       sourceRefs,
				Prompt:           prompt,
				PromptHash:       promptHash,
				Model:            modelName,
				CreatedBy:        agentID,
			}
			return a.docsAISectionCandidateRepo.Create(ctx, candidate)
		},
		PublishDocumentChangeProposal: func(ctx context.Context, workspaceID string, req model.CreateDocsChangeProposalRequest) (*model.DocsChangeProposal, error) {
			if a.docsChangeProposalRepo == nil {
				return nil, fmt.Errorf("docs change proposal repository is not available")
			}
			doc, err := a.docsDocRepo.GetByID(ctx, strings.TrimSpace(req.DocumentID))
			if err != nil {
				return nil, err
			}
			if doc == nil || doc.WorkspaceID != workspaceID {
				return nil, fmt.Errorf("document not found")
			}
			sources := req.Sources
			if len(sources) == 0 || strings.TrimSpace(string(sources)) == "" || strings.TrimSpace(string(sources)) == "null" {
				sources = json.RawMessage(`[]`)
			} else {
				normalizedSources, err := json.Marshal(model.NormalizeDocsChangeProposalSources(sources))
				if err != nil {
					return nil, fmt.Errorf("marshal proposal sources: %w", err)
				}
				sources = normalizedSources
			}
			proposal := &model.DocsChangeProposal{
				WorkspaceID:     workspaceID,
				DocumentID:      strings.TrimSpace(req.DocumentID),
				BlockID:         trimStringPtr(req.BlockID),
				AgentID:         trimStringPtr(req.AgentID),
				AgentRunID:      trimStringPtr(req.AgentRunID),
				Scope:           strings.TrimSpace(req.Scope),
				Status:          model.DocsChangeProposalStatusPending,
				Revision:        req.Revision,
				Summary:         strings.TrimSpace(req.Summary),
				ContentMarkdown: strings.TrimSpace(req.ContentMarkdown),
				Content:         append(json.RawMessage(nil), req.Content...),
				Sources:         append(json.RawMessage(nil), sources...),
				CreatedBy:       strings.TrimSpace(req.CreatedBy),
			}
			return a.docsChangeProposalRepo.Create(ctx, proposal)
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
		UpsertDocumentKey: func(ctx context.Context, record *model.DocsDocumentKey) error {
			if a.docsDocumentKeyRepo == nil {
				return fmt.Errorf("docs document key repository is not available")
			}
			return a.docsDocumentKeyRepo.Upsert(ctx, record)
		},

		// Release facts
		GetReleaseContext: func(ctx context.Context, workspaceID string, req model.GetReleaseContextRequest) (*model.ReleaseContextResult, error) {
			if a.releaseFacts == nil {
				return nil, fmt.Errorf("release facts are not available")
			}
			return a.releaseFacts.GetReleaseContext(ctx, workspaceID, req)
		},
		FindTasksForGitChanges: func(ctx context.Context, workspaceID string, req model.FindTasksForGitChangesRequest) (*model.FindTasksForGitChangesResult, error) {
			if a.releaseFacts == nil {
				return nil, fmt.Errorf("release facts are not available")
			}
			return a.releaseFacts.FindTasksForGitChanges(ctx, workspaceID, req)
		},
		GetTaskContext: func(ctx context.Context, workspaceID string, req model.GetTaskContextRequest) (*model.GetTaskContextResult, error) {
			if a.releaseFacts == nil {
				return nil, fmt.Errorf("release facts are not available")
			}
			return a.releaseFacts.GetTaskContext(ctx, workspaceID, req)
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

func trimStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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
		agent.ActiveTaskID = nil
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
			agent.ActiveTaskID = nil
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
	agent.ActiveTaskID = nil
	agent.TokensUsedThisMonth += tokens
	return a.agentRepo.Update(ctx, agent)
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
