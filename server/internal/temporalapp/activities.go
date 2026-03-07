package temporalapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

var branchTokenSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// AgentRunActivities contains the Temporal activities that execute an agent run.
type AgentRunActivities struct {
	runRepo       *repository.AgentRunRepository
	agentRepo     *repository.AgentRepository
	artifactRepo  *repository.AgentRunArtifactRepository
	storyRepo     *repository.PMStoryRepository
	epicRepo      *repository.PMEpicRepository
	ticketRepo    *repository.SupportTicketRepository
	commentRepo   *repository.PMCommentRepository
	checklistRepo *repository.PMChecklistItemRepository
	messageRepo   *repository.SupportMessageRepository
	gitIntRepo    *repository.GitIntegrationRepository
	gitRepo       *repository.GitRepositoryRepository
	gitLinkRepo   *repository.StoryGitLinkRepository
	deliveryRepo  *repository.StoryDeliveryTargetRepository
	settingsRepo  *repository.SettingsRepository
	runtimes      *workerpkg.RuntimeRegistry
	githubApp     *githubapp.Client
}

// NewAgentRunActivities creates the activity set used by shared Temporal workers.
func NewAgentRunActivities(
	runRepo *repository.AgentRunRepository,
	agentRepo *repository.AgentRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
	storyRepo *repository.PMStoryRepository,
	epicRepo *repository.PMEpicRepository,
	ticketRepo *repository.SupportTicketRepository,
	commentRepo *repository.PMCommentRepository,
	checklistRepo *repository.PMChecklistItemRepository,
	messageRepo *repository.SupportMessageRepository,
	gitIntRepo *repository.GitIntegrationRepository,
	gitRepo *repository.GitRepositoryRepository,
	gitLinkRepo *repository.StoryGitLinkRepository,
	deliveryRepo *repository.StoryDeliveryTargetRepository,
	settingsRepo *repository.SettingsRepository,
	runtimes *workerpkg.RuntimeRegistry,
	githubApp *githubapp.Client,
) *AgentRunActivities {
	return &AgentRunActivities{
		runRepo:       runRepo,
		agentRepo:     agentRepo,
		artifactRepo:  artifactRepo,
		storyRepo:     storyRepo,
		epicRepo:      epicRepo,
		ticketRepo:    ticketRepo,
		commentRepo:   commentRepo,
		checklistRepo: checklistRepo,
		messageRepo:   messageRepo,
		gitIntRepo:    gitIntRepo,
		gitRepo:       gitRepo,
		gitLinkRepo:   gitLinkRepo,
		deliveryRepo:  deliveryRepo,
		settingsRepo:  settingsRepo,
		runtimes:      runtimes,
		githubApp:     githubApp,
	}
}

type resolvedRunState struct {
	run            *model.AgentRun
	agent          *model.Agent
	story          *model.PMStory
	epic           *model.PMEpic
	epicStories    []model.PMStory
	ticket         *model.SupportTicket
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
		return ExecuteRunResult{}, err
	}
	defer os.RemoveAll(workDir)

	if state.repository != nil {
		if err := a.checkoutRunRef(ctx, workDir, state); err != nil {
			_ = a.failRun(ctx, state, err.Error())
			return ExecuteRunResult{}, err
		}
	}

	config := workerpkg.ParseWorkflowConfig(workDir)
	if config == nil {
		config = workerpkg.DefaultWorkflowConfig()
	}

	bridge := a.serviceBridge()
	execCtx := &workerpkg.ExecutionContext{
		Context:             ctx,
		WorkDir:             workDir,
		WorkspaceID:         state.run.WorkspaceID,
		AgentID:             state.run.AgentID,
		RunID:               state.run.ID,
		TargetType:          state.run.TargetType,
		TargetID:            state.run.TargetID,
		Agent:               state.agent,
		Story:               state.story,
		Epic:                state.epic,
		EpicStories:         state.epicStories,
		Ticket:              state.ticket,
		GitIntegration:      state.integration,
		GitAccessToken:      state.accessToken,
		Repo:                repoFullName(state),
		BaseBranch:          derefString(state.run.BaseBranch),
		WorkingBranch:       derefString(state.run.WorkingBranch),
		InitialInstructions: runInputAdditionalContext(state.run.Input),
		Config:              config,
		RuntimeProfile:      state.profile,
		AllowedTools:        workerAllowedToolSet(state.profile),
		Services:            bridge,
		Heartbeat: func(stage string) error {
			now := time.Now()
			activity.RecordHeartbeat(ctx, stage)
			return a.runRepo.UpdateStage(ctx, state.run.WorkspaceID, state.run.ID, stage, &now)
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
	if state.ticket != nil {
		execCtx.TicketID = state.ticket.ID
	}

	runtimeKind := state.run.RuntimeKind
	if runtimeKind == "" {
		runtimeKind = state.agent.RuntimeKind
	}
	adapter, err := a.runtimes.Get(runtimeKind)
	if err != nil {
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, err
	}

	err = adapter.Execute(execCtx, state.run)
	if err != nil {
		if err == workerpkg.ErrRunCancelled || ctx.Err() != nil {
			_ = a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
			return ExecuteRunResult{}, nil
		}
		_ = a.failRun(ctx, state, err.Error())
		return ExecuteRunResult{}, err
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
		case run.TicketID != nil:
			run.TargetType = "support_ticket"
			run.TargetID = *run.TicketID
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

	if run.TicketID != nil {
		ticket, err := a.ticketRepo.GetByID(ctx, run.WorkspaceID, *run.TicketID)
		if err != nil {
			return nil, err
		}
		if ticket == nil {
			return nil, fmt.Errorf("ticket not found")
		}
		state.ticket = ticket
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
		ListTicketMessages: func(ctx context.Context, workspaceID, ticketID string) ([]model.SupportMessage, error) {
			return a.messageRepo.ListByTicket(ctx, workspaceID, ticketID, true)
		},
		UpdateTicketStatus: func(ctx context.Context, workspaceID, ticketID, status string) error {
			ticket, err := a.ticketRepo.GetByID(ctx, workspaceID, ticketID)
			if err != nil {
				return err
			}
			if ticket == nil {
				return fmt.Errorf("ticket not found")
			}
			ticket.Status = status
			return a.ticketRepo.Update(ctx, ticket)
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
