package temporalapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

type planningRunInput struct {
	Stage               string   `json:"stage,omitempty"`
	AdditionalContext   string   `json:"additional_context,omitempty"`
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
	runRepo          *repository.AgentRunRepository
	runMessageRepo   *repository.AgentRunMessageRepository
	agentRepo        *repository.AgentRepository
	artifactRepo     *repository.AgentRunArtifactRepository
	storyRepo        *repository.PMStoryRepository
	storyLinkRepo    *repository.PMStoryLinkRepository
	epicRepo         *repository.PMEpicRepository
	conversationRepo *repository.SupportConversationRepository
	commentRepo      *repository.PMCommentRepository
	checklistRepo    *repository.PMChecklistItemRepository
	messageRepo      *repository.SupportMessageRepository
	gitIntRepo       *repository.GitIntegrationRepository
	gitRepo          *repository.GitRepositoryRepository
	gitLinkRepo      *repository.StoryGitLinkRepository
	deliveryRepo     *repository.StoryDeliveryTargetRepository
	settingsRepo     *repository.SettingsRepository
	docsSpaceRepo    *repository.DocsSpaceRepository
	docsDocRepo      *repository.DocsDocumentRepository
	docsContentRepo  *repository.DocsContentRepository
	docsVersionRepo  *repository.DocsVersionRepository
	docsLinkRepo     *repository.DocsLinkRepository
	docsSearchRepo   *repository.DocsSearchRepository
	crmDealRepo      *repository.CRMDealRepository
	crmContactRepo   *repository.CRMContactRepository
	crmSignalRepo    *repository.CRMSignalRepository
	crmActivityRepo  *repository.CRMActivityRepository
	commandExecutor  InternalCommandExecutor
	wsPublisher      websocket.EventPublisher
	runtimes         *workerpkg.RuntimeRegistry
	githubApp        *githubapp.Client
	runEngine        *RunEngine
}

// NewAgentRunActivities creates the activity set used by shared Temporal workers.
func NewAgentRunActivities(
	runRepo *repository.AgentRunRepository,
	runMessageRepo *repository.AgentRunMessageRepository,
	agentRepo *repository.AgentRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
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
		runRepo:          runRepo,
		runMessageRepo:   runMessageRepo,
		agentRepo:        agentRepo,
		artifactRepo:     artifactRepo,
		storyRepo:        storyRepo,
		storyLinkRepo:    storyLinkRepo,
		epicRepo:         epicRepo,
		conversationRepo: conversationRepo,
		commentRepo:      commentRepo,
		checklistRepo:    checklistRepo,
		messageRepo:      messageRepo,
		gitIntRepo:       gitIntRepo,
		gitRepo:          gitRepo,
		gitLinkRepo:      gitLinkRepo,
		deliveryRepo:     deliveryRepo,
		settingsRepo:     settingsRepo,
		docsSpaceRepo:    docsSpaceRepo,
		docsDocRepo:      docsDocRepo,
		docsContentRepo:  docsContentRepo,
		docsVersionRepo:  docsVersionRepo,
		docsLinkRepo:     docsLinkRepo,
		docsSearchRepo:   docsSearchRepo,
		crmDealRepo:      crmDealRepo,
		crmContactRepo:   crmContactRepo,
		crmSignalRepo:    crmSignalRepo,
		crmActivityRepo:  crmActivityRepo,
		commandExecutor:  commandExecutor,
		wsPublisher:      wsPublisher,
		runtimes:         runtimes,
		githubApp:        githubApp,
		runEngine:        runEngine,
	}
}

type resolvedRunState struct {
	run            *model.AgentRun
	agent          *model.Agent
	story          *model.PMStory
	epic           *model.PMEpic
	epicStories    []model.PMStory
	conversation   *model.SupportConversation
	resolved       workerpkg.ResolvedProfile // merged class+agent overrides — use this for decisions
	deliveryTarget *model.StoryDeliveryTarget
	repository     *model.GitRepository
	integration    *model.GitIntegration
	teamDefault    *model.PMTeamRepoDefault
	accessToken    string
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

	if state.story != nil {
		if err := a.prepareStoryDelivery(ctx, state); err != nil {
			_ = a.failRun(ctx, state, err.Error())
			return err
		}
	}

	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)

	activity.RecordHeartbeat(ctx, "prepared")
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
	case "create_stories":
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
	workDir, err := workerpkg.PrepareWorkspace(ctx, state.integration, repoFullName(state), state.accessToken)
	if err != nil {
		_ = a.failRun(ctx, state, fmt.Sprintf("prepare workspace: %v", err))
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	defer os.RemoveAll(workDir)
	slog.InfoContext(ctx, "agent run workspace ready",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"repo", repoFullName(state),
	)

	if state.repository != nil {
		slog.InfoContext(ctx, "agent run checking out run ref",
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"repo", state.repository.FullName,
		)
		if err := a.checkoutRunRef(ctx, workDir, state); err != nil {
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
		slog.InfoContext(ctx, "agent run checked out run ref",
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"repo", state.repository.FullName,
		)
	}

	config := workerpkg.ParseWorkflowConfig(workDir)
	if config == nil {
		config = workerpkg.DefaultWorkflowConfig()
	}

	allowedTools := resolvedAllowedToolSet(state.resolved)
	if len(planningInput.AllowedTools) > 0 {
		allowedTools = stringSliceToSet(planningInput.AllowedTools)
	}

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
		Story:                  state.story,
		Epic:                   state.epic,
		EpicStories:            state.epicStories,
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
			activity.RecordHeartbeat(ctx, stage)
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
	if state.story != nil {
		execCtx.StoryID = state.story.ID
	}
	if state.conversation != nil {
		execCtx.ConversationID = state.conversation.ID
	}

	runtimeKind := state.run.RuntimeKind
	if runtimeKind == "" {
		runtimeKind = state.agent.RuntimeKind
	}
	adapter, err := a.runtimes.Get(runtimeKind)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	err = adapter.Execute(execCtx, state.run)
	if err != nil {
		if err == workerpkg.ErrRunCancelled || ctx.Err() != nil {
			bgCtx := context.Background()
			_ = a.markAgentIdle(bgCtx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
			return ExecuteRunResult{}, nil
		}
		bgCtx := context.Background()
		_ = a.failRun(bgCtx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if err := a.persistAssistantRunMessage(ctx, state, execCtx); err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if err := a.captureTranscriptPlanningArtifacts(ctx, state, execCtx, planningInput); err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	approvalRequest := latestExecutionApprovalRequest(execCtx)
	humanInputRequest := latestExecutionHumanInputRequest(execCtx)
	if err := a.finalizePlanningRun(ctx, state, planningInput); err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	if err := a.finalizeSupportConversationRun(ctx, state); err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	if state.story != nil && execCtx.WorkingBranch != "" {
		state.run.WorkingBranch = &execCtx.WorkingBranch
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return ExecuteRunResult{}, err
	}

	waitForApproval, waitForInput := resolveExecutionWaitState(state.run, humanInputRequest, approvalRequest)
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

	if waitForApproval || waitForInput {
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
		ContinueExecution: continueExecution && !waitForApproval && !waitForInput,
	}, nil
}

func resolveExecutionWaitState(run *model.AgentRun, humanInputRequest *workerpkg.HumanInputRequest, approvalRequest *model.ApprovalRequest) (waitForApproval bool, waitForInput bool) {
	if run == nil {
		return false, false
	}

	waitForApproval = run.ApprovalState == "pending"
	if run.InvocationMode != model.InvocationModeInteractive {
		return waitForApproval, false
	}
	if approvalRequest != nil {
		return true, false
	}
	if humanInputRequest != nil {
		return false, true
	}
	return waitForApproval, false
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
		created, err := a.createRunMessage(ctx, state.run, "user", "prompt", prompt, nil, nil, nil)
		if err != nil {
			return nil, nil, nil, err
		}
		messages = append(messages, *created)
	}

	history := make([]workerpkg.ExecutionMessage, 0, len(messages))
	for _, message := range messages {
		if !shouldIncludeRunMessageInExecutionHistory(message) {
			continue
		}
		history = append(history, runMessageToExecutionMessage(message))
	}
	return history, artifactContext, providerContinuation, nil
}

func (a *AgentRunActivities) buildInitialRunUserPrompt(ctx context.Context, state *resolvedRunState, artifactContext *workerpkg.ArtifactContext, planningInput planningRunInput, initialInstructions string) (string, error) {
	var checklist []model.PMChecklistItem
	if state.story != nil {
		items, err := a.checklistRepo.List(ctx, state.story.ID)
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
		state.story,
		state.epic,
		state.epicStories,
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
				Label:   "Linked epic spec document",
				Source:  "spec_document",
				Status:  "approved",
				Format:  "text",
				Content: strings.TrimSpace(content.ContentText),
			})
		}
	}
	if len(artifactContext.Entries) == 0 {
		return nil, nil
	}
	return artifactContext, nil
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
			status := "approved"
			if marker, ok := applied[artifact.ID]; ok && strings.TrimSpace(marker.Action) != "" {
				status = "approved_and_" + strings.TrimSpace(marker.Action)
			}
			latestApproved[strings.TrimSpace(preview.PanelKey)] = approvedPreviewContextEntry{
				Preview: preview,
				Status:  status,
			}
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
		if preview, ok := latestRunPreview[panelKey]; ok {
			content, err := renderArtifactContextContent(preview.Format, preview.Content)
			if err != nil {
				return nil, err
			}
			entries = append(entries, workerpkg.ArtifactContextEntry{
				Label:   fmt.Sprintf("Current preview for %s", panelKey),
				Source:  workerpkg.RunPreviewArtifactType,
				Status:  "draft",
				Format:  preview.Format,
				Content: content,
			})
		}
		if approved, ok := latestApproved[panelKey]; ok {
			content, err := renderArtifactContextContent(approved.Preview.Format, approved.Preview.Content)
			if err != nil {
				return nil, err
			}
			entries = append(entries, workerpkg.ArtifactContextEntry{
				Label:   fmt.Sprintf("Approved preview for %s", panelKey),
				Source:  model.AgentRunArtifactTypeApprovedPreview,
				Status:  approved.Status,
				Format:  approved.Preview.Format,
				Content: content,
			})
		}
	}
	entries = append(entries, otherEntries...)
	return entries, nil
}

type approvedPreviewContextEntry struct {
	Preview model.ApprovedRunPreview
	Status  string
}

func buildOtherArtifactContextEntry(artifact model.AgentRunArtifact) *workerpkg.ArtifactContextEntry {
	content := strings.TrimSpace(derefString(artifact.InlineContent))
	if content == "" {
		return nil
	}
	switch strings.TrimSpace(artifact.ArtifactType) {
	case "product_spec_draft":
		return &workerpkg.ArtifactContextEntry{
			Label:   "Structured product spec draft artifact",
			Source:  artifact.ArtifactType,
			Status:  "draft",
			Format:  artifact.Format,
			Content: content,
		}
	case "story_plan_proposal":
		return &workerpkg.ArtifactContextEntry{
			Label:   "Structured story plan proposal artifact",
			Source:  artifact.ArtifactType,
			Status:  "draft",
			Format:  artifact.Format,
			Content: content,
		}
	default:
		return nil
	}
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

func runMessageToExecutionMessage(message model.AgentRunMessage) workerpkg.ExecutionMessage {
	execMessage := workerpkg.ExecutionMessage{
		SequenceNo: message.SequenceNo,
		Role:       message.Role,
		Content:    message.Content,
	}
	if len(message.ContentBlocks) > 0 && string(message.ContentBlocks) != "null" {
		var blocks []workerpkg.ExecutionBlock
		if err := json.Unmarshal(message.ContentBlocks, &blocks); err == nil {
			execMessage.Blocks = blocks
		}
	}
	return execMessage
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
	_, err = a.createRunMessage(ctx, run, "assistant", "status", strings.TrimSpace(content), nil, nil, nil)
	return err
}

func (a *AgentRunActivities) persistAssistantRunMessage(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext) error {
	if a.runMessageRepo == nil || execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	result := execCtx.LastExecutionResult
	if strings.TrimSpace(result.AssistantText) == "" && len(result.AssistantBlocks) == 0 {
		return nil
	}

	var blocks json.RawMessage
	if len(result.AssistantBlocks) > 0 {
		payload, err := json.Marshal(result.AssistantBlocks)
		if err != nil {
			return fmt.Errorf("marshal assistant blocks: %w", err)
		}
		blocks = payload
	}
	var invocations json.RawMessage
	if len(result.ToolInvocations) > 0 {
		payload, err := json.Marshal(result.ToolInvocations)
		if err != nil {
			return fmt.Errorf("marshal tool invocations: %w", err)
		}
		invocations = payload
	}
	usagePayload, err := json.Marshal(map[string]int{
		"input_tokens":  result.Usage.InputTokens,
		"output_tokens": result.Usage.OutputTokens,
	})
	if err != nil {
		return fmt.Errorf("marshal usage: %w", err)
	}

	assistantMessage, err := a.createRunMessage(ctx, state.run, "assistant", "assistant_turn", strings.TrimSpace(result.AssistantText), blocks, invocations, usagePayload)
	if err != nil {
		return err
	}
	if err := a.persistProviderResponseCheckpoint(ctx, state, result, assistantMessage); err != nil {
		return err
	}

	for _, toolMessage := range finalRoundToolMessages(result.Messages) {
		var toolBlocks json.RawMessage
		if len(toolMessage.Blocks) > 0 {
			payload, err := json.Marshal(toolMessage.Blocks)
			if err != nil {
				return fmt.Errorf("marshal tool message blocks: %w", err)
			}
			toolBlocks = payload
		}
		if _, err := a.createRunMessage(ctx, state.run, "tool", "tool_result", strings.TrimSpace(toolMessage.Content), toolBlocks, nil, nil); err != nil {
			return err
		}
	}

	return nil
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

	_, err := a.appendRunArtifact(ctx, state.run, model.AgentRunArtifactTypeProviderResponseCheckpoint, "json", model.ProviderResponseCheckpoint{
		Provider:              provider,
		ResponseID:            strings.TrimSpace(result.ProviderContinuation.ResponseID),
		PreviousResponseID:    strings.TrimSpace(result.ProviderContinuation.PreviousResponseID),
		AssistantMessageSeqNo: assistantMessage.SequenceNo,
		RecordedAt:            time.Now().UTC(),
	})
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

func (a *AgentRunActivities) createRunMessage(ctx context.Context, run *model.AgentRun, role, messageType, content string, blocks, toolInvocations, tokenUsage json.RawMessage) (*model.AgentRunMessage, error) {
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
		ToolInvocations: toolInvocations,
		TokenUsage:      tokenUsage,
		SequenceNo:      sequenceNo,
	}
	if err := a.runMessageRepo.Create(ctx, message); err != nil {
		return nil, err
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
}

func (a *AgentRunActivities) publishRunStreamEvent(run *model.AgentRun, event workerpkg.ExecutionEvent) {
	if a.wsPublisher == nil || run == nil {
		return
	}

	payload, err := json.Marshal(model.AgentRunStreamEvent{
		SentAt:        time.Now().UTC(),
		Type:          event.Type,
		RunID:         run.ID,
		Text:          event.Text,
		ToolCallID:    event.ToolCallID,
		ToolName:      event.ToolName,
		ToolInput:     event.ToolInput,
		OutputSummary: event.OutputSummary,
		DurationMs:    event.DurationMs,
		Error:         event.Error,
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
}

func latestExecutionApprovalRequest(execCtx *workerpkg.ExecutionContext) *model.ApprovalRequest {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return workerpkg.ExtractLatestHumanApprovalRequest(execCtx.LastExecutionResult.ToolInvocations)
}

func latestExecutionHumanInputRequest(execCtx *workerpkg.ExecutionContext) *workerpkg.HumanInputRequest {
	if execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}
	return workerpkg.ExtractLatestHumanInputRequest(execCtx.LastExecutionResult.ToolInvocations)
}

func (a *AgentRunActivities) captureTranscriptPlanningArtifacts(ctx context.Context, state *resolvedRunState, execCtx *workerpkg.ExecutionContext, _ planningRunInput) error {
	if state == nil || state.run == nil || state.epic == nil || state.run.TargetType != "epic" || execCtx == nil || execCtx.LastExecutionResult == nil {
		return nil
	}

	previews := workerpkg.ExtractPublishedPreviews(execCtx.LastExecutionResult.ToolInvocations)
	for index, preview := range previews {
		if err := a.createRunArtifact(ctx, state.run, workerpkg.RunPreviewArtifactType, "json", map[string]any{
			"panel_key": preview.PanelKey,
			"title":     preview.Title,
			"format":    preview.Format,
			"content":   json.RawMessage(preview.Content),
			"replace":   preview.Replace,
		}, 999900+index); err != nil {
			return err
		}
	}

	return nil
}

func (a *AgentRunActivities) applyApprovedInteractivePreview(ctx context.Context, state *resolvedRunState, input *planningRunInput) (string, error) {
	if state == nil || state.run == nil || state.epic == nil || input == nil {
		return "", nil
	}
	if state.run.TargetType != "epic" || state.run.InvocationMode != model.InvocationModeInteractive || strings.TrimSpace(input.Stage) != "" {
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
		if err := a.applyApprovedPRDPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "persist_prd"
	case "stories":
		if err := a.applyApprovedStoryPlanPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "create_stories"
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

	for _, artifact := range artifacts {
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
		return &artifact, &preview, nil
	}
	return nil, nil, nil
}

func decodeApprovedStoryPlanPreviewContent(raw json.RawMessage) (model.OrchestrationProposal, error) {
	var proposal model.OrchestrationProposal
	if err := json.Unmarshal(raw, &proposal); err == nil {
		return proposal, nil
	}

	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return proposal, err
	}
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return proposal, fmt.Errorf("story plan preview content is empty")
	}
	if err := json.Unmarshal([]byte(encoded), &proposal); err != nil {
		return proposal, err
	}
	return proposal, nil
}

func (a *AgentRunActivities) applyApprovedPRDPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatMarkdown {
		return fmt.Errorf("approved PRD preview must use markdown format")
	}

	var markdown string
	if err := json.Unmarshal(preview.Content, &markdown); err != nil {
		return fmt.Errorf("parse approved PRD preview content: %w", err)
	}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return fmt.Errorf("approved PRD preview content is empty")
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

func (a *AgentRunActivities) applyApprovedStoryPlanPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatJSON {
		return fmt.Errorf("approved story plan preview must use json format")
	}
	if a.commandExecutor == nil {
		return fmt.Errorf("planner commands are not available")
	}

	proposal, err := decodeApprovedStoryPlanPreviewContent(preview.Content)
	if err != nil {
		return fmt.Errorf("parse approved story plan preview content: %w", err)
	}
	if proposal.EpicID == "" {
		proposal.EpicID = state.epic.ID
	}
	if proposal.SpecVersionID == "" {
		proposal.SpecVersionID = strings.TrimSpace(firstNonEmptyString(input.SpecVersionID, derefString(state.epic.ApprovedSpecVersionID)))
	}
	if err := validatePlanningProposalStories(proposal.ProposedStories); err != nil {
		return err
	}

	output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
		WorkspaceID: state.run.WorkspaceID,
		ActorID:     runActorID(state.run),
		TargetType:  "epic",
		TargetID:    state.epic.ID,
	}, "pm.create_story_batch", mustJSON(map[string]any{
		"stories": proposal.ProposedStories,
	}))
	if err != nil {
		return err
	}

	var result workerpkg.CreateStoryBatchResult
	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("parse created story batch: %w", err)
	}

	stories, err := a.epicRepo.ListStories(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	state.epicStories = stories

	epicWithStats, err := a.epicRepo.GetByID(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	if epicWithStats == nil {
		return fmt.Errorf("epic not found after applying approved story plan")
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

	createdCount := len(result.Stories)
	summaryText := fmt.Sprintf("Applied the approved story plan and created %d stories.", createdCount)
	if _, err := a.createRunMessage(ctx, state.run, "assistant", "assistant_turn", summaryText, nil, nil, nil); err != nil {
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
			run.TargetType = "story"
			run.TargetID = *run.StoryID
		case run.ConversationID != nil:
			run.TargetType = "support_conversation"
			run.TargetID = *run.ConversationID
		}
	}

	if run.StoryID != nil {
		story, err := a.storyRepo.GetRawByID(ctx, *run.StoryID)
		if err != nil {
			return nil, err
		}
		if story == nil {
			return nil, fmt.Errorf("story not found")
		}
		state.story = story

		target, teamDefault, err := a.resolveDeliveryTarget(ctx, run.WorkspaceID, story)
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
	}

	if run.ConversationID != nil {
		conversation, err := a.conversationRepo.GetByID(ctx, run.WorkspaceID, *run.ConversationID)
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
		epicStories, err := a.epicRepo.ListStories(ctx, run.TargetID)
		if err != nil {
			return nil, err
		}
		state.epicStories = epicStories
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

func (a *AgentRunActivities) prepareStoryDelivery(ctx context.Context, state *resolvedRunState) error {
	target := state.deliveryTarget
	if target == nil {
		return fmt.Errorf("story delivery target is missing")
	}
	if state.repository == nil || state.integration == nil {
		if state.resolved.RequiresRepo {
			return fmt.Errorf("story has no delivery target configured")
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
		branchName := buildWorkingBranch(state.story, state.teamDefault)
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
	if state.story == nil || state.deliveryTarget == nil || state.repository == nil || state.integration == nil {
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
			StoryID:       state.story.ID,
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
	input.SpecDocumentID = strings.TrimSpace(input.SpecDocumentID)
	input.SpecVersionID = strings.TrimSpace(input.SpecVersionID)
	input.PlanningMethodology = model.NormalizePlanningMethodology(strings.TrimSpace(input.PlanningMethodology))
	input.FlowOutputKind = strings.TrimSpace(input.FlowOutputKind)

	if state.run.TargetType != "epic" || state.epic == nil {
		return input, nil
	}

	if input.SpecDocumentID == "" && state.epic.SpecDocumentID != nil {
		input.SpecDocumentID = strings.TrimSpace(*state.epic.SpecDocumentID)
	}
	if input.Stage == model.PlanningStageDraftSpec && input.SpecDocumentID == "" {
		doc, err := a.ensureEpicSpecDocument(ctx, state, runActorID(state.run))
		if err != nil {
			return planningRunInput{}, err
		}
		input.SpecDocumentID = doc.ID
	}
	if input.SpecVersionID == "" && state.epic.ApprovedSpecVersionID != nil {
		input.SpecVersionID = strings.TrimSpace(*state.epic.ApprovedSpecVersionID)
	}
	if input.PlanningMethodology == "" {
		input.PlanningMethodology = model.PlanningMethodologyStructuredV1
	}

	payload, _ := json.Marshal(input)
	state.run.Input = payload

	return input, nil
}

func (a *AgentRunActivities) buildInitialInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if strings.TrimSpace(input.FlowOutputKind) != "" {
		return a.buildFlowOutputInstructions(ctx, state, input)
	}
	if state.run.TargetType != "epic" || state.epic == nil {
		return runInputAdditionalContext(state.run.Input), nil
	}
	if input.Stage == "" {
		return a.buildAgenticEpicPlannerInstructions(ctx, state, input)
	}

	switch input.Stage {
	case model.PlanningStageDraftSpec:
		return a.buildDraftSpecInstructions(ctx, state, input)
	case model.PlanningStagePlanStories:
		return a.buildStoryPlanInstructions(ctx, state, input)
	default:
		return runInputAdditionalContext(state.run.Input), nil
	}
}

func (a *AgentRunActivities) buildFlowOutputInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	switch strings.TrimSpace(input.FlowOutputKind) {
	case "pm.story_completion_followups":
		return a.buildStoryCompletionInstructions(state, input), nil
	case "crm.deal_review_actions":
		return a.buildCRMDealReviewInstructions(ctx, state, input)
	default:
		return runInputAdditionalContext(state.run.Input), nil
	}
}

func (a *AgentRunActivities) buildStoryCompletionInstructions(state *resolvedRunState, input planningRunInput) string {
	var sections []string
	sections = append(sections, "Review this completed story and return JSON only with the shape {\"summary\":\"...\",\"followups\":[{\"title\":\"...\",\"description\":\"...\",\"story_type\":\"chore\",\"priority\":\"medium\"}]}.")
	sections = append(sections, "Only propose internal PM/docs/support follow-up work. Do not publish customer-facing docs or website changes directly.")
	if state.story != nil {
		sections = append(sections, fmt.Sprintf("Story: %s", state.story.Name))
		if state.story.Description != nil && strings.TrimSpace(*state.story.Description) != "" {
			sections = append(sections, "Story description:\n"+truncatePlanningText(*state.story.Description, 8000))
		}
		if state.story.EpicID != nil && *state.story.EpicID != "" {
			sections = append(sections, fmt.Sprintf("Epic ID: %s", *state.story.EpicID))
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

func (a *AgentRunActivities) buildDraftSpecInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	var sections []string
	sections = append(sections, "Use Teampulse Docs as the canonical source of truth for this epic's product spec. Refresh the existing spec if one already exists instead of inventing a separate planning artifact.")

	if input.AdditionalContext != "" {
		sections = append(sections, "Operator notes:\n"+input.AdditionalContext)
	}

	var currentSpecText string
	if input.SpecDocumentID != "" {
		sections = append(sections, fmt.Sprintf("Canonical product spec document ID: %s", input.SpecDocumentID))

		content, err := a.docsContentRepo.GetByDocumentID(ctx, input.SpecDocumentID)
		if err != nil {
			return "", err
		}
		if content != nil && strings.TrimSpace(content.ContentText) != "" {
			currentSpecText = strings.TrimSpace(content.ContentText)
			sections = append(sections, "Current spec draft already in Docs:\n"+truncatePlanningText(content.ContentText, 12000))
		}
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
		sections = append(sections, "Support tickets already connected to stories in this epic:\n"+linkedTickets)
	}

	repoContext, err := a.buildDraftSpecCodeContext(ctx, state, strings.Join([]string{
		state.epic.Name,
		derefString(state.epic.Description),
		currentSpecText,
		linkedDocs,
		linkedTickets,
		input.AdditionalContext,
	}, "\n\n"))
	if err != nil {
		return "", err
	}
	if repoContext != "" {
		sections = append(sections, "Current implementation and product surface context from the live repository:\n"+repoContext)
	}

	if toolAllowedForPlanningRun(state.resolved, input.AllowedTools, "web_search_brave") {
		sections = append(sections, "If market context, standards, competitors, or external evidence would improve the spec, use the `web_search_brave` tool and return any sources separately in the JSON sources field instead of embedding a Research Sources section in spec_markdown.")
	}

	sections = append(sections, "Use this normalized section structure in the spec markdown:\n# Problem\n## User Impact\n## Source Context\n## Goals\n## Non-goals\n## Requirements\n## Scenarios\n## Constraints / Risks\n## Success Metrics\n## Proposed Story Areas\n## Assumptions\n## Open Questions")

	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildStoryPlanInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if input.SpecVersionID == "" {
		return "", fmt.Errorf("story planning requires an approved spec version")
	}
	if state.repository == nil || state.integration == nil {
		return "", fmt.Errorf("story planning requires a configured epic planning repository")
	}

	version, err := a.docsVersionRepo.GetByID(ctx, input.SpecVersionID)
	if err != nil {
		return "", err
	}
	if version == nil {
		return "", fmt.Errorf("approved spec version not found")
	}

	var sections []string
	sections = append(sections, "Create a dependency-aware story plan from the approved spec. Do not rewrite the spec; turn it into implementation-ready stories with stable refs, explicit dependencies, and clear acceptance criteria.")
	sections = append(sections, "Prefer vertical, user-visible slices that can be tested independently. Only introduce enabler stories when a vertical slice would be unsafe or misleading.")
	sections = append(sections, fmt.Sprintf("Approved spec version ID: %s", version.ID))
	if input.SpecDocumentID != "" {
		sections = append(sections, fmt.Sprintf("Canonical product spec document ID: %s", input.SpecDocumentID))
	}
	sections = append(sections, fmt.Sprintf("Planning repository: %s", state.repository.FullName))
	if strings.TrimSpace(version.ContentText) != "" {
		sections = append(sections, "Approved spec snapshot:\n"+truncatePlanningText(version.ContentText, 16000))
	}
	if clarifications := model.ParseSpecClarifications(state.epic.SpecClarifications); len(clarifications) > 0 {
		resolved := make([]model.SpecClarificationItem, 0, len(clarifications))
		for _, item := range clarifications {
			if model.SpecClarificationResolved(item) {
				resolved = append(resolved, item)
			}
		}
		if len(resolved) > 0 {
			sections = append(sections, "Resolved open questions and assumptions:\n"+renderSpecClarificationsContext(resolved))
		}
	}
	if input.AdditionalContext != "" {
		sections = append(sections, "Operator notes:\n"+input.AdditionalContext)
	}

	linkedDocs, err := a.renderLinkedDocsContext(ctx, state.run.WorkspaceID, state.epic.ID, input.SpecDocumentID)
	if err != nil {
		return "", err
	}
	if linkedDocs != "" {
		sections = append(sections, "Other linked docs that may affect decomposition:\n"+linkedDocs)
	}

	linkedTickets, err := a.renderLinkedTicketsContext(ctx, state)
	if err != nil {
		return "", err
	}
	if linkedTickets != "" {
		sections = append(sections, "Support tickets already connected to this epic's stories:\n"+linkedTickets)
	}

	codeContext, err := a.buildPlanningCodeContext(ctx, state, version.ContentText)
	if err != nil {
		return "", err
	}
	if codeContext != "" {
		sections = append(sections, "Current implementation context from the live repository:\n"+codeContext)
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
	sections = append(sections, "Choose the next step from the transcript, current epic state, linked docs, existing stories, and tool results.")
	sections = append(sections, "Use this sequence unless the human explicitly redirects you: clarify scope if needed, draft/refine the PRD, publish it with publish_preview using panel_key=\"prd_draft\", wait for inline PRD approval, persist the approved PRD to the canonical epic doc, propose the implementation story plan, publish it with publish_preview using panel_key=\"story_plan\", wait for inline story approval, then create stories.")
	sections = append(sections, "Keep approvals soft and inline. When you need approval, call request_human_approval with phase=\"prd\" or phase=\"stories\" and stop after the request.")
	sections = append(sections, "Use publish_preview for reviewable right-pane content. For PRDs, publish markdown. For story plans, publish JSON.")
	sections = append(sections, "Before PRD approval, keep the draft in chat only. After PRD approval, use ensure_epic_spec_doc, write_document_content, and approve_epic_spec to persist and record the approved PRD. After story approval, use create_story_batch and only use assign_story_agent or set_story_dependencies as follow-up correction tools.")

	var hasSpecContent bool
	if input.SpecDocumentID != "" {
		sections = append(sections, fmt.Sprintf("Existing canonical spec document ID: %s", input.SpecDocumentID))

		// Inject durable draft content when a spec doc exists but is not approved.
		if input.SpecVersionID == "" {
			if content, err := a.docsContentRepo.GetByDocumentID(ctx, input.SpecDocumentID); err == nil && content != nil && strings.TrimSpace(content.ContentText) != "" {
				hasSpecContent = true
				sections = append(sections, "IMPORTANT: A PRD draft already exists in the spec document but was never formally approved. Resume from the current draft instead of starting over. Present the draft, revise it if needed, and request PRD approval before any story planning.")
				sections = append(sections, "Current spec draft:\n"+truncatePlanningText(content.ContentText, 12000))
			}
		}
	}
	if input.SpecVersionID != "" {
		sections = append(sections, fmt.Sprintf("Approved spec version ID: %s", input.SpecVersionID))
		sections = append(sections, "IMPORTANT: A previously approved spec already exists. The PRD is LOCKED. Do not redraft, rewrite, or re-approve it. Use it as the read-only source of truth for story planning. If the human asks to revise the PRD, explain the spec is approved and suggest creating a follow-up epic instead, unless they insist.")
		sections = append(sections, "If ensure_epic_spec_doc or other tool output shows has_approved_spec=true, treat PRD persistence as already complete. Do NOT call ensure_epic_spec_doc, write_document_content, link_document_to_object, or approve_epic_spec again unless the human explicitly asks you to rewrite the canonical doc.")
	}
	if state.epic != nil && state.epic.TeamID != nil && strings.TrimSpace(*state.epic.TeamID) != "" {
		sections = append(sections, fmt.Sprintf("Epic team ID: %s", strings.TrimSpace(*state.epic.TeamID)))
	} else {
		sections = append(sections, "This epic does not currently have a team. Before creating stories, call list_workspace_teams and ask the human to choose the correct team inline in chat.")
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
		derefString(state.epic.Description),
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

	if len(state.epicStories) > 0 {
		lines := make([]string, 0, len(state.epicStories))
		for _, story := range state.epicStories {
			lines = append(lines, fmt.Sprintf("- %s [%s]", story.Name, story.ID))
		}
		sections = append(sections, fmt.Sprintf("IMPORTANT: %d stories already exist under this epic. Do NOT recreate them. Only create new stories if the human explicitly requests additions.", len(state.epicStories)))
		sections = append(sections, "Stories already linked to this epic:\n"+strings.Join(lines, "\n"))
	}

	hasApprovedSpec := input.SpecVersionID != ""
	hasSpecDoc := input.SpecDocumentID != ""
	hasStories := len(state.epicStories) > 0
	sections = append(sections, formatInteractivePlanningFacts(input, hasSpecContent, len(state.epicStories)))
	switch {
	case hasApprovedSpec && hasStories:
		sections = append(sections, "Next-step guidance: the PRD is approved and stories already exist. Do not redraft the PRD or recreate existing stories. Enter clarification or extension mode, inspect current stories if needed, and only add new stories if the human explicitly asks for them.")
	case hasApprovedSpec && !hasStories:
		sections = append(sections, "Next-step guidance: the PRD is approved and no stories exist yet. Skip PRD drafting entirely and proceed directly to story planning from the approved spec and current codebase context.")
	case hasSpecDoc && hasSpecContent:
		sections = append(sections, "Next-step guidance: a draft PRD exists but it is not approved yet. Resume from the current draft, present or revise it, and request PRD approval before any story planning.")
	default:
		sections = append(sections, "Next-step guidance: no approved PRD exists yet. Follow the full loop from clarification through PRD drafting, preview, revision if needed, and approval.")
	}

	return strings.Join(sections, "\n\n"), nil
}

func formatInteractivePlanningFacts(input planningRunInput, hasDraftSpec bool, storyCount int) string {
	facts := []string{
		fmt.Sprintf("- approved_spec_exists=%t", strings.TrimSpace(input.SpecVersionID) != ""),
		fmt.Sprintf("- draft_spec_exists=%t", hasDraftSpec),
		fmt.Sprintf("- existing_story_count=%d", storyCount),
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
	if input.Stage != "" && input.Stage != model.PlanningStageDraftSpec && input.Stage != model.PlanningStagePlanStories {
		return nil
	}
	if state.epic.PlanningRepositoryID == nil || strings.TrimSpace(*state.epic.PlanningRepositoryID) == "" {
		if input.Stage == "" {
			return nil
		}
		if input.Stage == model.PlanningStageDraftSpec {
			return fmt.Errorf("drafting a product spec requires an epic planning repository")
		}
		return fmt.Errorf("story planning requires an epic planning repository")
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

	switch input.Stage {
	case model.PlanningStageDraftSpec:
		return a.finalizeDraftSpecRun(ctx, state, input)
	case model.PlanningStagePlanStories:
		return a.finalizePlanStoriesRun(ctx, state, input)
	case "":
		return a.finalizeAgenticEpicPlannerRun(ctx, state)
	default:
		return nil
	}
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
			return fmt.Errorf("decode story completion assessment: %w", err)
		}
		if strings.TrimSpace(assessment.Summary) == "" {
			return fmt.Errorf("story completion assessment is missing a summary")
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

func (a *AgentRunActivities) finalizeDraftSpecRun(ctx context.Context, state *resolvedRunState, input planningRunInput) error {
	var draft model.ProductSpecDraft
	if err := json.Unmarshal(state.run.OutputSummary, &draft); err != nil {
		return fmt.Errorf("decode product spec draft: %w", err)
	}
	if strings.TrimSpace(draft.SpecMarkdown) == "" {
		return fmt.Errorf("product spec draft is missing spec_markdown")
	}

	doc, err := a.ensureEpicSpecDocument(ctx, state, runActorID(state.run))
	if err != nil {
		return err
	}

	renderedMarkdown := renderProductSpecMarkdown(draft.SpecMarkdown, draft.Sources)
	content := tiptap.MarkdownToJSON(renderedMarkdown)
	savedContent, err := a.docsContentRepo.Upsert(ctx, doc.ID, content)
	if err != nil {
		return err
	}

	label := "AI Draft"
	version, err := a.docsVersionRepo.Create(ctx, doc.ID, runActorID(state.run), savedContent.Content, savedContent.ContentText, &label, "manual", len(strings.Fields(savedContent.ContentText)))
	if err != nil {
		return err
	}

	title := strings.TrimSpace(draft.Title)
	if title == "" {
		title = strings.TrimSpace(state.epic.Name) + " Product Spec"
	}
	updates := map[string]interface{}{
		"title": title,
	}
	if summary := strings.TrimSpace(draft.Summary); summary != "" {
		updates["excerpt"] = summary
	} else {
		updates["excerpt"] = nil
	}
	if _, err := a.docsDocRepo.Update(ctx, doc.ID, updates); err != nil {
		return err
	}

	clarifications := buildDraftSpecClarifications(draft)
	state.epic.SpecDocumentID = &doc.ID
	state.epic.SpecClarifications = model.MarshalSpecClarifications(clarifications)
	state.epic.SpecClarifiedAt = nil
	state.epic.SpecClarifiedBy = nil
	state.epic.LastPlanningRunID = &state.run.ID
	if err := a.epicRepo.Update(ctx, state.epic); err != nil {
		return err
	}

	summary, _ := json.Marshal(planningRunSummary{
		Stage:               model.PlanningStageDraftSpec,
		SpecDocumentID:      doc.ID,
		SpecVersionID:       version.ID,
		PlanningMethodology: input.PlanningMethodology,
		Summary:             strings.TrimSpace(draft.Summary),
		Risks:               append([]string(nil), draft.Risks...),
		Assumptions:         append([]string(nil), draft.Assumptions...),
		OpenQuestions:       append([]string(nil), draft.OpenQuestions...),
		Clarifications:      clarifications,
	})
	state.run.OutputSummary = summary

	if len(draft.Sources) > 0 {
		if err := a.createRunArtifact(ctx, state.run, "external_research_sources", "json", draft.Sources, 999997); err != nil {
			return err
		}
	}

	return nil
}

func (a *AgentRunActivities) finalizePlanStoriesRun(ctx context.Context, state *resolvedRunState, input planningRunInput) error {
	var proposal model.OrchestrationProposal
	if err := json.Unmarshal(state.run.OutputSummary, &proposal); err != nil {
		return fmt.Errorf("decode story plan proposal: %w", err)
	}
	if len(proposal.ProposedStories) == 0 {
		return fmt.Errorf("story plan proposal did not include proposed stories")
	}
	if proposal.EpicID == "" {
		proposal.EpicID = state.epic.ID
	}
	if proposal.SpecVersionID == "" {
		proposal.SpecVersionID = input.SpecVersionID
	}
	if err := validatePlanningProposalStories(proposal.ProposedStories); err != nil {
		return err
	}

	state.epic.LastPlanningRunID = &state.run.ID
	if err := a.epicRepo.Update(ctx, state.epic); err != nil {
		return err
	}

	summary, _ := json.Marshal(planningRunSummary{
		Stage:               model.PlanningStagePlanStories,
		SpecDocumentID:      input.SpecDocumentID,
		SpecVersionID:       proposal.SpecVersionID,
		PlanningMethodology: input.PlanningMethodology,
		Summary:             strings.TrimSpace(proposal.Summary),
		Risks:               append([]string(nil), proposal.Risks...),
		OpenQuestions:       append([]string(nil), proposal.OpenQuestions...),
		Proposal:            &proposal,
	})
	state.run.OutputSummary = summary

	return nil
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

func (a *AgentRunActivities) renderLinkedDocsContext(ctx context.Context, workspaceID, epicID, excludeDocumentID string) (string, error) {
	links, err := a.docsLinkRepo.ListByObject(ctx, workspaceID, model.LinkedObjectEpic, epicID)
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

func (a *AgentRunActivities) renderLinkedTicketsContext(ctx context.Context, state *resolvedRunState) (string, error) {
	if len(state.epicStories) == 0 {
		return "", nil
	}

	storyIDs := make([]string, 0, len(state.epicStories))
	storyNames := make(map[string]string, len(state.epicStories))
	for _, story := range state.epicStories {
		storyIDs = append(storyIDs, story.ID)
		storyNames[story.ID] = story.Name
	}

	tickets, err := a.conversationRepo.ListByLinkedStoryIDs(ctx, state.run.WorkspaceID, storyIDs)
	if err != nil {
		return "", err
	}
	if len(tickets) == 0 {
		return "", nil
	}

	entries := make([]string, 0, len(tickets))
	for idx, ticket := range tickets {
		storyName := ""
		if ticket.LinkedStoryID != nil {
			storyName = storyNames[*ticket.LinkedStoryID]
		}

		header := fmt.Sprintf("- Ticket #%d: %s [status=%s priority=%s]", ticket.DisplayID, ticket.Subject, ticket.Status, ticket.Priority)
		if storyName != "" {
			header += fmt.Sprintf(" linked_story=%q", storyName)
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

func validatePlanningProposalStories(stories []model.ProposedStory) error {
	refToIdx := make(map[string]int, len(stories))
	for idx := range stories {
		stories[idx].Name = strings.TrimSpace(stories[idx].Name)
		if stories[idx].Name == "" {
			return fmt.Errorf("proposed story %d is missing a name", idx+1)
		}
		filteredCriteria := make([]string, 0, len(stories[idx].AcceptanceCriteria))
		for _, item := range stories[idx].AcceptanceCriteria {
			item = strings.TrimSpace(item)
			if item != "" {
				filteredCriteria = append(filteredCriteria, item)
			}
		}
		stories[idx].AcceptanceCriteria = filteredCriteria
		if len(filteredCriteria) == 0 {
			return fmt.Errorf("proposed story %d is missing acceptance criteria", idx+1)
		}
		stories[idx].Ref = strings.TrimSpace(stories[idx].Ref)
		if stories[idx].Ref == "" {
			stories[idx].Ref = fmt.Sprintf("story_%d", idx+1)
		}
		if prev, exists := refToIdx[stories[idx].Ref]; exists {
			return fmt.Errorf("story refs must be unique; stories %d and %d both use %q", prev+1, idx+1, stories[idx].Ref)
		}
		refToIdx[stories[idx].Ref] = idx
	}

	for idx, story := range stories {
		for _, depRef := range story.DependencyRefs {
			depRef = strings.TrimSpace(depRef)
			if _, ok := refToIdx[depRef]; !ok {
				return fmt.Errorf("story %d references unknown dependency ref %q", idx+1, depRef)
			}
			if depRef == story.Ref {
				return fmt.Errorf("story %d cannot depend on itself", idx+1)
			}
		}
	}

	visited := make(map[string]uint8, len(stories))
	var visit func(ref string) error
	visit = func(ref string) error {
		switch visited[ref] {
		case 1:
			return fmt.Errorf("circular dependency detected involving %q", ref)
		case 2:
			return nil
		}
		visited[ref] = 1
		story := stories[refToIdx[ref]]
		for _, depRef := range story.DependencyRefs {
			if err := visit(strings.TrimSpace(depRef)); err != nil {
				return err
			}
		}
		visited[ref] = 2
		return nil
	}

	for _, story := range stories {
		if err := visit(story.Ref); err != nil {
			return err
		}
	}
	return nil
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
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal artifact %s: %w", artifactType, err)
	}
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: strPtr(string(content)),
		Metadata:      json.RawMessage("{}"),
		SequenceNo:    sequenceNo,
	}
	return a.artifactRepo.Create(ctx, artifact)
}

func (a *AgentRunActivities) appendRunArtifact(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any) (*model.AgentRunArtifact, error) {
	content, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal artifact %s: %w", artifactType, err)
	}
	sequenceNo, err := a.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: strPtr(string(content)),
		Metadata:      json.RawMessage("{}"),
		SequenceNo:    sequenceNo,
	}
	if err := a.artifactRepo.Create(ctx, artifact); err != nil {
		return nil, err
	}
	return artifact, nil
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
		AddComment: func(ctx context.Context, workspaceID, storyID, agentID, content string) error {
			comment := &model.PMComment{
				EntityType: "story",
				EntityID:   storyID,
				AuthorID:   agentID,
				Body:       content,
			}
			return a.commentRepo.Create(ctx, comment)
		},
		UpdateStoryState: func(ctx context.Context, workspaceID, storyID, stateID string) error {
			if a.commandExecutor != nil {
				_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					WorkspaceID: workspaceID,
					TargetType:  "story",
					TargetID:    storyID,
				}, "pm.update_story_state", mustJSON(map[string]any{
					"story_id": storyID,
					"state_id": stateID,
				}))
				return err
			}
			// TODO(flow-platform): remove direct fallback once all native runs are command-backed.
			story, err := a.storyRepo.GetRawByID(ctx, storyID)
			if err != nil {
				return err
			}
			if story == nil {
				return fmt.Errorf("story not found")
			}
			story.WorkflowStateID = stateID
			return a.storyRepo.Update(ctx, story)
		},
		ListChecklist: func(ctx context.Context, workspaceID, storyID string) ([]model.PMChecklistItem, error) {
			return a.checklistRepo.List(ctx, storyID)
		},
		CreateStoryBatch: func(ctx context.Context, workspaceID, epicID, actorID string, stories []model.ProposedStory) (workerpkg.CreateStoryBatchResult, error) {
			if a.commandExecutor != nil {
				output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
					WorkspaceID: workspaceID,
					ActorID:     actorID,
					TargetType:  "epic",
					TargetID:    epicID,
				}, "pm.create_story_batch", mustJSON(map[string]any{
					"stories": stories,
				}))
				if err != nil {
					return workerpkg.CreateStoryBatchResult{}, err
				}
				var result workerpkg.CreateStoryBatchResult
				if err := json.Unmarshal(output, &result); err != nil {
					return workerpkg.CreateStoryBatchResult{}, err
				}
				return result, nil
			}
			return workerpkg.CreateStoryBatchResult{}, fmt.Errorf("planner commands are not available")
		},
		AssignStoryAgent: func(ctx context.Context, workspaceID, actorID, storyID, agentID string) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("planner commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "story",
				TargetID:    storyID,
			}, "pm.assign_story_agent", mustJSON(map[string]any{
				"story_id": storyID,
				"agent_id": agentID,
			}))
			return err
		},
		SetStoryDependencies: func(ctx context.Context, workspaceID, actorID string, dependencies []workerpkg.StoryDependencyLink) error {
			if a.commandExecutor == nil {
				return fmt.Errorf("planner commands are not available")
			}
			_, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
				WorkspaceID: workspaceID,
				ActorID:     actorID,
				TargetType:  "epic",
			}, "pm.set_story_dependencies", mustJSON(map[string]any{
				"dependencies": dependencies,
			}))
			return err
		},
		ListEpicStories: func(ctx context.Context, workspaceID, epicID string) ([]workerpkg.EpicStorySummary, error) {
			stories, err := a.epicRepo.ListStories(ctx, epicID)
			if err != nil {
				return nil, err
			}
			summaries := make([]workerpkg.EpicStorySummary, 0, len(stories))
			for _, s := range stories {
				if s.WorkspaceID != workspaceID {
					continue
				}
				status := "not_started"
				if s.Completed {
					status = "done"
				} else if s.Started {
					status = "in_progress"
				}
				summaries = append(summaries, workerpkg.EpicStorySummary{
					ID:              s.ID,
					Name:            s.Name,
					StoryType:       s.StoryType,
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
					ID:               team.ID,
					Name:             team.Name,
					Handle:           team.Handle,
					TeamType:         team.TeamType,
					DefaultStoryType: team.DefaultStoryType,
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
			conversation, err := a.conversationRepo.GetByID(ctx, workspaceID, conversationID)
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

func buildWorkingBranch(story *model.PMStory, teamDefault *model.PMTeamRepoDefault) string {
	template := "tp-{display_id}-{slug}"
	if teamDefault != nil && strings.TrimSpace(teamDefault.BranchTemplate) != "" {
		template = teamDefault.BranchTemplate
	}
	replacements := map[string]string{
		"{display_id}": fmt.Sprintf("%d", story.DisplayID),
		"{slug}":       slugifyBranchToken(story.Name),
	}
	for placeholder, value := range replacements {
		template = strings.ReplaceAll(template, placeholder, value)
	}
	template = strings.ToLower(strings.TrimSpace(template))
	template = strings.Trim(template, "/-")
	if template == "" {
		return fmt.Sprintf("tp-%d-%s", story.DisplayID, slugifyBranchToken(story.Name))
	}
	return template
}

func slugifyBranchToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = branchTokenSanitizer.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "story"
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

func toolAllowedForPlanningRun(resolved workerpkg.ResolvedProfile, explicitAllowed []string, toolName string) bool {
	if len(explicitAllowed) > 0 {
		for _, allowed := range explicitAllowed {
			if strings.TrimSpace(allowed) == toolName {
				return true
			}
		}
		return false
	}
	for _, allowed := range resolved.Tools {
		if strings.TrimSpace(allowed) == toolName {
			return true
		}
	}
	return false
}

func mustJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
