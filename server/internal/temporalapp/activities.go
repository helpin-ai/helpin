package temporalapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
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
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

var branchTokenSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

const (
	productSpecsSpaceSlug = "product-specs"
	productSpecsSpaceName = "Product Specs"
)

type planningRunInput struct {
	Stage                     string `json:"stage,omitempty"`
	AdditionalContext         string `json:"additional_context,omitempty"`
	SpecDocumentID            string `json:"spec_document_id,omitempty"`
	SpecVersionID             string `json:"spec_version_id,omitempty"`
	PlanningMethodology       string `json:"planning_methodology,omitempty"`
	PlanningWebSearchEnabled  bool   `json:"planning_web_search_enabled,omitempty"`
	PlanningWebSearchProvider string `json:"planning_web_search_provider,omitempty"`
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

// AgentRunActivities contains the Temporal activities that execute an agent run.
type AgentRunActivities struct {
	runRepo         *repository.AgentRunRepository
	agentRepo       *repository.AgentRepository
	artifactRepo    *repository.AgentRunArtifactRepository
	storyRepo       *repository.PMStoryRepository
	storyLinkRepo   *repository.PMStoryLinkRepository
	epicRepo        *repository.PMEpicRepository
	conversationRepo *repository.SupportConversationRepository
	commentRepo      *repository.PMCommentRepository
	checklistRepo    *repository.PMChecklistItemRepository
	messageRepo      *repository.SupportMessageRepository
	gitIntRepo      *repository.GitIntegrationRepository
	gitRepo         *repository.GitRepositoryRepository
	gitLinkRepo     *repository.StoryGitLinkRepository
	deliveryRepo    *repository.StoryDeliveryTargetRepository
	settingsRepo    *repository.SettingsRepository
	docsSpaceRepo   *repository.DocsSpaceRepository
	docsDocRepo     *repository.DocsDocumentRepository
	docsContentRepo *repository.DocsContentRepository
	docsVersionRepo *repository.DocsVersionRepository
	docsLinkRepo    *repository.DocsLinkRepository
	runtimes        *workerpkg.RuntimeRegistry
	githubApp       *githubapp.Client
}

// NewAgentRunActivities creates the activity set used by shared Temporal workers.
func NewAgentRunActivities(
	runRepo *repository.AgentRunRepository,
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
	runtimes *workerpkg.RuntimeRegistry,
	githubApp *githubapp.Client,
) *AgentRunActivities {
	return &AgentRunActivities{
		runRepo:         runRepo,
		agentRepo:       agentRepo,
		artifactRepo:    artifactRepo,
		storyRepo:       storyRepo,
		storyLinkRepo:   storyLinkRepo,
		epicRepo:        epicRepo,
		conversationRepo: conversationRepo,
		commentRepo:      commentRepo,
		checklistRepo:    checklistRepo,
		messageRepo:      messageRepo,
		gitIntRepo:      gitIntRepo,
		gitRepo:         gitRepo,
		gitLinkRepo:     gitLinkRepo,
		deliveryRepo:    deliveryRepo,
		settingsRepo:    settingsRepo,
		docsSpaceRepo:   docsSpaceRepo,
		docsDocRepo:     docsDocRepo,
		docsContentRepo: docsContentRepo,
		docsVersionRepo: docsVersionRepo,
		docsLinkRepo:    docsLinkRepo,
		runtimes:        runtimes,
		githubApp:       githubApp,
	}
}

type resolvedRunState struct {
	run            *model.AgentRun
	agent          *model.Agent
	story          *model.PMStory
	epic           *model.PMEpic
	epicStories    []model.PMStory
	conversation   *model.SupportConversation
	profile        model.RuntimeProfile
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
	state.run.StartedAt = &now
	state.run.ExecutionStage = strPtr("starting")
	state.run.LastHeartbeatAt = &now
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return ExecuteRunResult{}, err
	}

	workDir, err := workerpkg.PrepareWorkspace(ctx, state.integration, repoFullName(state), state.accessToken)
	if err != nil {
		_ = a.failRun(ctx, state, fmt.Sprintf("prepare workspace: %v", err))
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}
	defer os.RemoveAll(workDir)

	if state.repository != nil {
		if err := a.checkoutRunRef(ctx, workDir, state); err != nil {
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, nonRetryableRunError(err)
		}
	}

	config := workerpkg.ParseWorkflowConfig(workDir)
	if config == nil {
		config = workerpkg.DefaultWorkflowConfig()
	}

	allowedTools := workerAllowedToolSet(state.profile)
	if input := planningInput; input.Stage != model.PlanningStageDraftSpec || !input.PlanningWebSearchEnabled {
		delete(allowedTools, "web_search")
	}

	bridge := a.serviceBridge()
	execCtx := &workerpkg.ExecutionContext{
		Context:                   ctx,
		WorkDir:                   workDir,
		WorkspaceID:               state.run.WorkspaceID,
		AgentID:                   state.run.AgentID,
		RunID:                     state.run.ID,
		TargetType:                state.run.TargetType,
		TargetID:                  state.run.TargetID,
		Agent:                     state.agent,
		Story:                     state.story,
		Epic:                      state.epic,
		EpicStories:               state.epicStories,
		Conversation:              state.conversation,
		GitIntegration:            state.integration,
		GitAccessToken:            state.accessToken,
		Repo:                      repoFullName(state),
		BaseBranch:                derefString(state.run.BaseBranch),
		WorkingBranch:             derefString(state.run.WorkingBranch),
		InitialInstructions:       initialInstructions,
		PlanningStage:             planningInput.Stage,
		PlanningMethodology:       planningInput.PlanningMethodology,
		PlanningSpecDocumentID:    planningInput.SpecDocumentID,
		PlanningSpecVersionID:     planningInput.SpecVersionID,
		PlanningWebSearchEnabled:  planningInput.PlanningWebSearchEnabled,
		PlanningWebSearchProvider: planningInput.PlanningWebSearchProvider,
		Config:                    config,
		RuntimeProfile:            state.profile,
		AllowedTools:              allowedTools,
		Services:                  bridge,
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

	if err := a.finalizePlanningRun(ctx, state, planningInput); err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, nonRetryableRunError(err)
	}

	if state.story != nil && execCtx.WorkingBranch != "" {
		state.run.WorkingBranch = &execCtx.WorkingBranch
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return ExecuteRunResult{}, err
	}

	waitForApproval := state.run.ApprovalState == "pending"
	completedAt := time.Now()
	if waitForApproval {
		state.run.Status = "awaiting_approval"
	} else {
		state.run.Status = "completed"
		state.run.CompletedAt = &completedAt
	}
	state.run.ExecutionStage = strPtr("completed")
	state.run.LastHeartbeatAt = &completedAt
	if isEmptySummary(state.run.OutputSummary) {
		state.run.OutputSummary = json.RawMessage(`{"status":"success"}`)
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return ExecuteRunResult{}, err
	}
	a.runRepo.Notify(ctx, state.run)

	if err := a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed); err != nil {
		return ExecuteRunResult{}, err
	}

	return ExecuteRunResult{WaitForApproval: waitForApproval}, nil
}

func (a *AgentRunActivities) loadRunState(ctx context.Context, runID string) (*resolvedRunState, error) {
	run, err := a.runRepo.GetByIDAny(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}

	agent, err := a.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}

	state := &resolvedRunState{
		run:     run,
		agent:   agent,
		profile: workerpkg.GetRuntimeProfile(agent.CapabilityProfile),
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
		if state.profile.RequiresRepo {
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

	if state.profile.RequiresRepo && (target.WorkingBranch == nil || strings.TrimSpace(*target.WorkingBranch) == "") {
		branchName := buildWorkingBranch(state.story, state.teamDefault)
		target.WorkingBranch = &branchName
	}

	if state.profile.RequiresRepo && state.accessToken == "" {
		return fmt.Errorf("repository access token is not available")
	}

	if state.profile.RequiresRepo && target.WorkingBranch != nil && *target.WorkingBranch != "" {
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
		queue := QueueForProfile(state.profile.Name)
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
	input.PlanningWebSearchProvider = model.NormalizePlanningWebSearchProvider(strings.TrimSpace(input.PlanningWebSearchProvider))

	if state.run.TargetType != "epic" || state.epic == nil {
		return input, nil
	}

	if input.Stage == "" {
		input.Stage = model.PlanningStageDraftSpec
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
	if input.PlanningMethodology == "" || input.PlanningWebSearchProvider == "" {
		settings, err := a.settingsRepo.GetWorkspaceSettings(ctx, state.run.WorkspaceID)
		if err != nil {
			return planningRunInput{}, err
		}
		if settings != nil {
			if input.PlanningMethodology == "" {
				input.PlanningMethodology = model.NormalizePlanningMethodology(settings.PlanningMethodology)
			}
			if !input.PlanningWebSearchEnabled {
				input.PlanningWebSearchEnabled = settings.PlanningWebSearchEnabled
			}
			if input.PlanningWebSearchProvider == "" {
				input.PlanningWebSearchProvider = model.NormalizePlanningWebSearchProvider(settings.PlanningWebSearchProvider)
			}
		}
	}
	if input.PlanningMethodology == "" {
		input.PlanningMethodology = model.PlanningMethodologyStructuredV1
	}
	if input.PlanningWebSearchProvider == "" {
		input.PlanningWebSearchProvider = model.PlanningWebSearchProviderBrave
	}

	payload, _ := json.Marshal(input)
	state.run.Input = payload

	return input, nil
}

func (a *AgentRunActivities) buildInitialInstructions(ctx context.Context, state *resolvedRunState, input planningRunInput) (string, error) {
	if state.run.TargetType != "epic" || state.epic == nil {
		return runInputAdditionalContext(state.run.Input), nil
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

	if input.PlanningWebSearchEnabled {
		sections = append(sections, fmt.Sprintf("External research is enabled for this run through the %s provider. Use the web_search tool when market context, standards, competitors, or external evidence would improve the spec. Return any sources separately in the JSON sources field instead of embedding a Research Sources section in spec_markdown.", input.PlanningWebSearchProvider))
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

func (a *AgentRunActivities) preparePlanningRepository(ctx context.Context, state *resolvedRunState, input planningRunInput) error {
	if state.run.TargetType != "epic" || state.epic == nil {
		return nil
	}
	if input.Stage != model.PlanningStageDraftSpec && input.Stage != model.PlanningStagePlanStories {
		return nil
	}
	if state.epic.PlanningRepositoryID == nil || strings.TrimSpace(*state.epic.PlanningRepositoryID) == "" {
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
	if state.run.TargetType != "epic" || state.epic == nil {
		return nil
	}

	switch input.Stage {
	case model.PlanningStageDraftSpec:
		return a.finalizeDraftSpecRun(ctx, state, input)
	case model.PlanningStagePlanStories:
		return a.finalizePlanStoriesRun(ctx, state, input)
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
	content := markdownToDocsJSON(renderedMarkdown)
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
	if len(clarifications) > 0 {
		state.epic.PlanningState = model.EpicPlanningStateAwaitingClarification
	} else {
		state.epic.PlanningState = model.EpicPlanningStateAwaitingSpecApproval
	}
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
	state.epic.PlanningState = model.EpicPlanningStateAwaitingPlanApproval
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

func markdownToDocsJSON(markdown string) json.RawMessage {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	nodes := make([]map[string]interface{}, 0, len(lines))
	paragraphLines := make([]string, 0)
	bulletLines := make([]string, 0)

	flushParagraph := func() {
		if len(paragraphLines) == 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(paragraphLines, " "))
		paragraphLines = paragraphLines[:0]
		if text == "" {
			return
		}
		nodes = append(nodes, map[string]interface{}{
			"type": "paragraph",
			"content": []map[string]interface{}{
				{"type": "text", "text": text},
			},
		})
	}

	flushBullets := func() {
		if len(bulletLines) == 0 {
			return
		}
		items := make([]map[string]interface{}, 0, len(bulletLines))
		for _, item := range bulletLines {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			items = append(items, map[string]interface{}{
				"type": "listItem",
				"content": []map[string]interface{}{
					{
						"type": "paragraph",
						"content": []map[string]interface{}{
							{"type": "text", "text": item},
						},
					},
				},
			})
		}
		bulletLines = bulletLines[:0]
		if len(items) == 0 {
			return
		}
		nodes = append(nodes, map[string]interface{}{
			"type":    "bulletList",
			"content": items,
		})
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			flushParagraph()
			flushBullets()
			continue
		}

		level, headingText, ok := parseMarkdownHeading(line)
		if ok {
			flushParagraph()
			flushBullets()
			nodes = append(nodes, map[string]interface{}{
				"type": "heading",
				"attrs": map[string]interface{}{
					"level": level,
				},
				"content": []map[string]interface{}{
					{"type": "text", "text": headingText},
				},
			})
			continue
		}

		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			flushParagraph()
			bulletLines = append(bulletLines, strings.TrimSpace(line[2:]))
			continue
		}

		flushBullets()
		paragraphLines = append(paragraphLines, line)
	}

	flushParagraph()
	flushBullets()

	if len(nodes) == 0 {
		nodes = append(nodes, map[string]interface{}{
			"type": "paragraph",
		})
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"type":    "doc",
		"content": nodes,
	})
	return payload
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

func parseMarkdownHeading(line string) (int, string, bool) {
	if !strings.HasPrefix(line, "#") {
		return 0, "", false
	}

	level := 0
	for level < len(line) && line[level] == '#' && level < 6 {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}

	text := strings.TrimSpace(line[level+1:])
	if text == "" {
		return 0, "", false
	}
	return level, text, true
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
			return a.conversationRepo.Update(ctx, conversation)
		},
	}
}

func (a *AgentRunActivities) failRun(ctx context.Context, state *resolvedRunState, errMsg string) error {
	now := time.Now()
	state.run.Status = "failed"
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

func ensureRunNotTerminal(run *model.AgentRun) error {
	if run == nil {
		return nil
	}
	switch run.Status {
	case "failed", "completed", "cancelled", "awaiting_approval":
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

func workerAllowedToolSet(profile model.RuntimeProfile) map[string]bool {
	set := make(map[string]bool, len(profile.AllowedTools))
	for _, toolName := range profile.AllowedTools {
		set[toolName] = true
	}
	return set
}
