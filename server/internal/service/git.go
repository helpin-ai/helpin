package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

var ErrTaskDeliveryTargetRequired = errors.New("story has no delivery target configured")

// GitService contains git integration and story delivery business logic.
type GitService struct {
	integrationRepo *repository.GitIntegrationRepository
	repoRepo        *repository.GitRepositoryRepository
	linkRepo        *repository.TaskGitLinkRepository
	deliveryRepo    *repository.TaskDeliveryTargetRepository
	settingsRepo    *repository.SettingsRepository
	workspaceRepo   *repository.WorkspaceRepository
	taskRepo        *repository.PMTaskRepository
	activitySvc     *PMActivityService
	wsPublisher     *websocket.Publisher
	ruleEngine      *AutomationRuleEngine
	githubApp       *githubapp.Client
	appBaseURL      string
	githubAppSlug   string
	stateSecret     string
}

// NewGitService creates a new GitService.
func NewGitService(
	integrationRepo *repository.GitIntegrationRepository,
	repoRepo *repository.GitRepositoryRepository,
	linkRepo *repository.TaskGitLinkRepository,
	deliveryRepo *repository.TaskDeliveryTargetRepository,
	settingsRepo *repository.SettingsRepository,
	workspaceRepo *repository.WorkspaceRepository,
	taskRepo *repository.PMTaskRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
	githubApp *githubapp.Client,
	appBaseURL string,
	githubAppSlug string,
	stateSecret string,
) *GitService {
	return &GitService{
		integrationRepo: integrationRepo,
		repoRepo:        repoRepo,
		linkRepo:        linkRepo,
		deliveryRepo:    deliveryRepo,
		settingsRepo:    settingsRepo,
		workspaceRepo:   workspaceRepo,
		taskRepo:        taskRepo,
		activitySvc:     activitySvc,
		wsPublisher:     wsPublisher,
		githubApp:       githubApp,
		appBaseURL:      strings.TrimRight(strings.TrimSpace(appBaseURL), "/"),
		githubAppSlug:   strings.TrimSpace(githubAppSlug),
		stateSecret:     strings.TrimSpace(stateSecret),
	}
}

// SetRuleEngine sets the automation rule engine used for webhook-derived triggers.
func (s *GitService) SetRuleEngine(engine *AutomationRuleEngine) *GitService {
	s.ruleEngine = engine
	return s
}

// ListIntegrations returns all git integrations for a workspace.
func (s *GitService) ListIntegrations(ctx context.Context, workspaceID string) ([]model.GitIntegration, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.integrationRepo.List(ctx, workspaceID)
}

// DeleteIntegration removes a git integration and its synced repositories.
func (s *GitService) DeleteIntegration(ctx context.Context, workspaceID, integrationID, actorID string) error {
	if workspaceID == "" || integrationID == "" {
		return fmt.Errorf("workspace_id and integration_id are required")
	}
	existing, err := s.integrationRepo.GetByID(ctx, workspaceID, integrationID)
	if err != nil {
		return fmt.Errorf("get integration: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("integration not found")
	}
	if err := s.repoRepo.DeleteByIntegration(ctx, workspaceID, integrationID); err != nil {
		return fmt.Errorf("delete integration repositories: %w", err)
	}
	if err := s.integrationRepo.Delete(ctx, workspaceID, integrationID); err != nil {
		return err
	}
	if s.activitySvc != nil && actorID != "" {
		_ = s.activitySvc.Log(ctx, workspaceID, "git_integration", existing.ID, &actorID, "deleted", nil, nil, &existing.DisplayName, nil)
	}
	s.publishSimpleEvent("deleted", "git_integration", existing.ID, workspaceID, actorID)
	return nil
}

// CreateIntegration creates a new git integration.
func (s *GitService) CreateIntegration(ctx context.Context, req model.CreateGitIntegrationRequest, actorID string) (*model.GitIntegration, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.DisplayName) == "" {
		return nil, fmt.Errorf("workspace_id and display_name are required")
	}
	if req.Provider != "github" && req.Provider != "gitlab" {
		return nil, fmt.Errorf("provider must be 'github' or 'gitlab'")
	}

	credentialMode := "github_app"
	if req.CredentialMode != nil && strings.TrimSpace(*req.CredentialMode) != "" {
		credentialMode = strings.TrimSpace(*req.CredentialMode)
	}
	if credentialMode == "github_app" && (req.Provider != "github" || req.InstallationID == nil || *req.InstallationID == "") {
		return nil, fmt.Errorf("github_app integrations require installation_id")
	}

	webhookSecret := req.WebhookSecret
	if webhookSecret == nil || strings.TrimSpace(*webhookSecret) == "" {
		secret := generateWebhookSecret()
		webhookSecret = &secret
	}

	integration := &model.GitIntegration{
		WorkspaceID:    req.WorkspaceID,
		Provider:       req.Provider,
		DisplayName:    strings.TrimSpace(req.DisplayName),
		CredentialMode: credentialMode,
		AccountLogin:   trimPtr(req.AccountLogin),
		BaseURL:        req.BaseURL,
		InstallationID: trimPtr(req.InstallationID),
		AppID:          trimPtr(req.AppID),
		WebhookSecret:  webhookSecret,
		AccessToken:    req.AccessToken,
		Active:         true,
	}

	if err := s.integrationRepo.Create(ctx, integration); err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, integration.WorkspaceID, "git_integration", integration.ID, &actorID, "created", nil, nil, &integration.DisplayName, nil)
	s.publishSimpleEvent("created", "git_integration", integration.ID, integration.WorkspaceID, actorID)

	if integration.Provider == "github" && integration.InstallationID != nil && s.githubApp != nil {
		if _, err := s.SyncRepositories(ctx, integration.WorkspaceID, integration.ID, actorID); err != nil {
			integration.LastSyncError = strPtr(err.Error())
			_ = s.integrationRepo.Update(ctx, integration)
		}
	}

	return integration, nil
}

// SyncRepositories refreshes the workspace repo catalog from a git provider installation.
func (s *GitService) SyncRepositories(ctx context.Context, workspaceID, integrationID, actorID string) ([]model.GitRepository, error) {
	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, integrationID)
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, fmt.Errorf("git integration not found")
	}
	if integration.Provider != "github" {
		return nil, fmt.Errorf("repository sync is only implemented for github")
	}
	if integration.InstallationID == nil || *integration.InstallationID == "" {
		return nil, fmt.Errorf("integration has no installation_id")
	}
	if s.githubApp == nil {
		return nil, fmt.Errorf("github app credentials are not configured")
	}

	repos, err := s.githubApp.ListInstallationRepositories(ctx, *integration.InstallationID)
	if err != nil {
		integration.LastSyncError = strPtr(err.Error())
		_ = s.integrationRepo.Update(ctx, integration)
		return nil, err
	}

	now := time.Now()
	upserts := make([]model.GitRepository, 0, len(repos))
	for _, repo := range repos {
		permissions, _ := json.Marshal(repo.Permissions)
		upserts = append(upserts, model.GitRepository{
			WorkspaceID:   workspaceID,
			IntegrationID: integrationID,
			Provider:      integration.Provider,
			ExternalID:    githubapp.InstallationIDString(repo.ID),
			FullName:      repo.FullName,
			DefaultBranch: defaultBranch(repo.DefaultBranch),
			Permissions:   permissions,
			Private:       repo.Private,
			Selected:      true,
		})
	}

	if err := s.repoRepo.UpsertMany(ctx, workspaceID, integrationID, upserts); err != nil {
		return nil, err
	}

	integration.LastSyncedAt = &now
	integration.LastSyncError = nil
	if err := s.integrationRepo.Update(ctx, integration); err != nil {
		return nil, err
	}

	s.publishSimpleEvent("updated", "git_integration", integration.ID, workspaceID, actorID)
	return s.repoRepo.List(ctx, workspaceID)
}

// ListRepositories returns the synced repository catalog for a workspace.
func (s *GitService) ListRepositories(ctx context.Context, workspaceID string) ([]model.GitRepository, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.repoRepo.List(ctx, workspaceID)
}

// ListRepositoryCatalog returns the full synced repository catalog for admin workflows.
func (s *GitService) ListRepositoryCatalog(ctx context.Context, workspaceID string) ([]model.GitRepository, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.repoRepo.ListAll(ctx, workspaceID)
}

// GetRepositoryByID returns a synced repository record for a workspace.
func (s *GitService) GetRepositoryByID(ctx context.Context, workspaceID, repoID string) (*model.GitRepository, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(repoID) == "" {
		return nil, fmt.Errorf("repository_id is required")
	}
	return s.repoRepo.GetByID(ctx, workspaceID, repoID)
}

// UpdateRepositorySelection updates whether a synced repository is available for story delivery.
func (s *GitService) UpdateRepositorySelection(ctx context.Context, workspaceID, repoID string, selected bool, actorID string) (*model.GitRepository, error) {
	repo, err := s.repoRepo.GetByID(ctx, workspaceID, repoID)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("repository not found")
	}
	repo.Selected = selected
	if err := s.repoRepo.Update(ctx, repo); err != nil {
		return nil, err
	}

	if s.activitySvc != nil && actorID != "" {
		value := repo.FullName
		_ = s.activitySvc.Log(ctx, workspaceID, "git_repository", repo.ID, &actorID, "updated", strPtr("selected"), nil, &value, nil)
	}
	s.publishSimpleEvent("updated", "git_repository", repo.ID, workspaceID, actorID)
	return repo, nil
}

// GetGitHubInstallURL returns the install or manage URL for the configured GitHub App.
func (s *GitService) GetGitHubInstallURL(ctx context.Context, workspaceID, actorID string) (string, string, error) {
	if workspaceID == "" {
		return "", "", fmt.Errorf("workspace_id is required")
	}
	if s.githubApp == nil || s.githubAppSlug == "" {
		return "", "", fmt.Errorf("github app onboarding is not configured")
	}
	if s.workspaceRepo == nil {
		return "", "", fmt.Errorf("workspace repository is not configured")
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return "", "", err
	}
	if workspace == nil {
		return "", "", fmt.Errorf("workspace not found")
	}

	integrations, err := s.integrationRepo.List(ctx, workspaceID)
	if err != nil {
		return "", "", err
	}
	for _, integration := range integrations {
		if integration.Provider != "github" || integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
			continue
		}

		installation, installErr := s.githubApp.GetInstallation(ctx, *integration.InstallationID)
		if installErr == nil && strings.TrimSpace(installation.HTMLURL) != "" {
			return installation.HTMLURL, "manage", nil
		}
	}

	state, err := s.signGitHubInstallState(workspaceID, actorID)
	if err != nil {
		return "", "", err
	}

	installURL := url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   "/apps/" + s.githubAppSlug + "/installations/new",
	}
	query := installURL.Query()
	query.Set("state", state)
	installURL.RawQuery = query.Encode()
	return installURL.String(), "install", nil
}

// CompleteGitHubInstall creates or updates the workspace integration after GitHub redirects back.
func (s *GitService) CompleteGitHubInstall(ctx context.Context, stateToken, installationID string) (string, error) {
	state, workspace, redirectURL, err := s.resolveGitHubInstallState(ctx, stateToken)
	if err != nil {
		return "", err
	}
	if installationID == "" {
		return withGitHubInstallStatus(redirectURL, "error", "GitHub did not return an installation ID.", nil), nil
	}
	if s.githubApp == nil {
		return withGitHubInstallStatus(redirectURL, "error", "GitHub App credentials are not configured on the server.", nil), nil
	}

	installation, err := s.githubApp.GetInstallation(ctx, installationID)
	if err != nil {
		return withGitHubInstallStatus(redirectURL, "error", err.Error(), nil), nil
	}

	integration, err := s.upsertGitHubIntegration(ctx, workspace.ID, installationID, installation, state.ActorID)
	if err != nil {
		return withGitHubInstallStatus(redirectURL, "error", err.Error(), nil), nil
	}

	repos, syncErr := s.SyncRepositories(ctx, workspace.ID, integration.ID, state.ActorID)
	params := map[string]string{
		"integration_id": integration.ID,
	}
	if len(repos) > 0 {
		params["repo_count"] = fmt.Sprintf("%d", len(repos))
	}
	if syncErr != nil {
		return withGitHubInstallStatus(redirectURL, "connected", "GitHub App connected, but repository sync failed. Open the integration card for details.", params), nil
	}
	return withGitHubInstallStatus(redirectURL, "connected", fmt.Sprintf("GitHub App connected to %s.", defaultAccountLogin(integration.AccountLogin)), params), nil
}

// ResolveGitHubWebhookWorkspace verifies the webhook signature and resolves the target workspace by installation.
func (s *GitService) ResolveGitHubWebhookWorkspace(ctx context.Context, installationID string, body []byte, signature string) (string, error) {
	if strings.TrimSpace(installationID) == "" {
		return "", fmt.Errorf("installation_id is required")
	}

	integration, err := s.integrationRepo.GetByInstallationID(ctx, "github", installationID)
	if err != nil {
		return "", err
	}
	if integration == nil {
		return "", fmt.Errorf("no workspace matches github installation %s", installationID)
	}
	if integration.WebhookSecret != nil && *integration.WebhookSecret != "" {
		expected := signGitHubWebhook(*integration.WebhookSecret, body)
		if !hmac.Equal([]byte(expected), []byte(signature)) {
			return "", fmt.Errorf("invalid github webhook signature")
		}
	}
	return integration.WorkspaceID, nil
}

// GetTaskGitLinks returns git links for a story.
func (s *GitService) GetTaskGitLinks(ctx context.Context, workspaceID, storyID string) ([]model.TaskGitLink, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.linkRepo.ListByTask(ctx, workspaceID, storyID)
}

// GetTaskDeliveryTarget resolves or creates the current delivery target for a story.
func (s *GitService) GetTaskDeliveryTarget(ctx context.Context, workspaceID, storyID string) (*model.TaskDeliveryTarget, error) {
	target, err := s.deliveryRepo.GetByTask(ctx, workspaceID, storyID)
	if err != nil {
		return nil, err
	}
	if target != nil {
		return target, nil
	}

	story, err := s.taskRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return nil, err
	}
	if story == nil {
		return nil, fmt.Errorf("story not found")
	}

	target = &model.TaskDeliveryTarget{
		WorkspaceID:   workspaceID,
		TaskID:        storyID,
		DeliveryState: "unconfigured",
	}

	if story.TeamID != nil && *story.TeamID != "" {
		teamDefault, err := s.settingsRepo.GetTeamRepoDefault(ctx, *story.TeamID)
		if err != nil {
			return nil, err
		}
		if teamDefault != nil {
			repo, err := s.repoRepo.GetByID(ctx, workspaceID, teamDefault.RepositoryID)
			if err != nil {
				return nil, err
			}
			if repo != nil {
				target.RepositoryID = &repo.ID
				target.RepoFullName = &repo.FullName
				target.IntegrationID = &repo.IntegrationID
				baseBranch := teamDefault.BaseBranch
				if baseBranch == "" {
					baseBranch = defaultBranch(repo.DefaultBranch)
				}
				target.BaseBranch = &baseBranch
				target.DeliveryState = "ready"
			}
		}
	}

	if err := s.deliveryRepo.Save(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}

// UpdateTaskDeliveryTarget updates the selected delivery target for a story.
func (s *GitService) UpdateTaskDeliveryTarget(ctx context.Context, workspaceID, storyID string, req model.UpdateTaskDeliveryTargetRequest, actorID string) (*model.TaskDeliveryTarget, error) {
	target, err := s.GetTaskDeliveryTarget(ctx, workspaceID, storyID)
	if err != nil {
		return nil, err
	}

	if req.RepositoryID != nil && *req.RepositoryID != "" {
		repo, err := s.repoRepo.GetByID(ctx, workspaceID, *req.RepositoryID)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository not found")
		}
		target.RepositoryID = &repo.ID
		target.RepoFullName = &repo.FullName
		target.IntegrationID = &repo.IntegrationID
		if req.BaseBranch == nil || *req.BaseBranch == "" {
			base := defaultBranch(repo.DefaultBranch)
			req.BaseBranch = &base
		}
	}
	if req.BaseBranch != nil && *req.BaseBranch != "" {
		baseBranch := strings.TrimSpace(*req.BaseBranch)
		target.BaseBranch = &baseBranch
	}
	if req.WorkingBranch != nil && *req.WorkingBranch != "" {
		workingBranch := strings.TrimSpace(*req.WorkingBranch)
		target.WorkingBranch = &workingBranch
	}
	if target.RepositoryID != nil && target.BaseBranch != nil {
		target.DeliveryState = "ready"
	}

	if err := s.deliveryRepo.Save(ctx, target); err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "task", storyID, &actorID, "updated", strPtr("delivery_target"), nil, target.RepoFullName, nil)
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "task_delivery_target",
		EntityID:    target.ID,
		WorkspaceID: workspaceID,
		ParentType:  "task",
		ParentID:    storyID,
		ActorID:     actorID,
	})
	return target, nil
}

// ResolveTaskDeliveryTargetForRun returns a delivery target suitable for a given execution policy.
func (s *GitService) ResolveTaskDeliveryTargetForRun(ctx context.Context, workspaceID, storyID string, requiresRepo bool) (*model.TaskDeliveryTarget, error) {
	target, err := s.GetTaskDeliveryTarget(ctx, workspaceID, storyID)
	if err != nil {
		return nil, err
	}
	if requiresRepo && (target.RepositoryID == nil || target.RepoFullName == nil || target.BaseBranch == nil) {
		return nil, ErrTaskDeliveryTargetRequired
	}
	return target, nil
}

// CreateBranch retains backward compatibility by updating the delivery target and a git link.
func (s *GitService) CreateBranch(ctx context.Context, workspaceID, storyID string, req model.CreateBranchRequest, actorID string) (*model.TaskGitLink, error) {
	targetReq := model.UpdateTaskDeliveryTargetRequest{
		BaseBranch: nil,
	}
	if req.IntegrationID != "" {
		integration, err := s.integrationRepo.GetByID(ctx, workspaceID, req.IntegrationID)
		if err != nil {
			return nil, err
		}
		if integration == nil {
			return nil, fmt.Errorf("git integration not found")
		}
		repos, err := s.repoRepo.List(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		for _, repo := range repos {
			if repo.IntegrationID == integration.ID && repo.FullName == req.Repo {
				targetReq.RepositoryID = &repo.ID
				break
			}
		}
	}
	targetReq.WorkingBranch = &req.BranchName
	target, err := s.UpdateTaskDeliveryTarget(ctx, workspaceID, storyID, targetReq, actorID)
	if err != nil {
		return nil, err
	}
	link := &model.TaskGitLink{
		WorkspaceID:   workspaceID,
		TaskID:        storyID,
		IntegrationID: deref(target.IntegrationID),
		RepositoryID:  target.RepositoryID,
		Provider:      "github",
		Repo:          deref(target.RepoFullName),
		Branch:        target.WorkingBranch,
	}
	if err := s.linkRepo.Create(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

// MergeBranch merges the story's working branch into the target branch via GitHub API.
func (s *GitService) MergeBranch(ctx context.Context, workspaceID, storyID, targetBranch string) error {
	target, err := s.deliveryRepo.GetByTask(ctx, workspaceID, storyID)
	if err != nil {
		return fmt.Errorf("load delivery target: %w", err)
	}
	if target == nil || target.WorkingBranch == nil || *target.WorkingBranch == "" {
		return fmt.Errorf("story has no working branch")
	}
	if target.IntegrationID == nil || *target.IntegrationID == "" {
		return fmt.Errorf("story has no git integration")
	}
	if target.RepoFullName == nil || *target.RepoFullName == "" {
		return fmt.Errorf("story has no repository configured")
	}

	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, *target.IntegrationID)
	if err != nil {
		return fmt.Errorf("load git integration: %w", err)
	}
	if integration == nil || integration.InstallationID == nil {
		return fmt.Errorf("git integration not found or has no installation ID")
	}

	// Parse owner/repo from full name (e.g., "org/repo")
	parts := strings.SplitN(*target.RepoFullName, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo full name: %s", *target.RepoFullName)
	}
	owner, repo := parts[0], parts[1]

	commitMsg := fmt.Sprintf("Merge %s into %s", *target.WorkingBranch, targetBranch)
	return s.githubApp.MergeBranch(ctx, *integration.InstallationID, owner, repo, targetBranch, *target.WorkingBranch, commitMsg)
}

// ProcessWebhookPush handles a push event from a git provider.
func (s *GitService) ProcessWebhookPush(ctx context.Context, workspaceID, repo, branch, commitSHA string) error {
	link, err := s.linkRepo.GetByBranch(ctx, workspaceID, repo, branch)
	if err != nil {
		return err
	}
	if link == nil {
		if s.ruleEngine != nil {
			s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
				WorkspaceID:  workspaceID,
				TriggerType:  model.TriggerGitHubPush,
				RepoFullName: repo,
				Branch:       branch,
			}, nil)
		}
		return nil
	}

	link.CommitSHA = &commitSHA
	if err := s.linkRepo.Update(ctx, link); err != nil {
		return err
	}

	target, err := s.deliveryRepo.GetByTask(ctx, workspaceID, link.TaskID)
	if err == nil && target != nil {
		target.LastCommitSHA = &commitSHA
		now := time.Now()
		target.LastSyncedAt = &now
		if target.DeliveryState == "ready" {
			target.DeliveryState = "in_progress"
		}
		_ = s.deliveryRepo.Save(ctx, target)
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "task_git_link",
		EntityID:    link.ID,
		WorkspaceID: workspaceID,
		ParentType:  "task",
		ParentID:    link.TaskID,
	})

	if s.ruleEngine != nil {
		event := model.AutomationEvent{
			WorkspaceID:  workspaceID,
			TriggerType:  model.TriggerGitHubPush,
			RepoFullName: repo,
			Branch:       branch,
			TargetType:   "task",
			TargetID:     link.TaskID,
			TaskID:       link.TaskID,
			StoryID:      link.TaskID,
		}
		if task, taskErr := s.taskRepo.GetRawByID(ctx, link.TaskID); taskErr == nil && task != nil {
			event.StateID = task.WorkflowStateID
			if task.TeamID != nil {
				event.TeamID = *task.TeamID
			}
		}
		s.ruleEngine.EvaluateEvent(ctx, event, nil)
	}

	return nil
}

// ProcessWebhookPR handles a PR event from a git provider.
func (s *GitService) ProcessWebhookPR(ctx context.Context, workspaceID, repo, action string, prNumber int, prTitle, prURL, prStatus, branch, baseBranch string) error {
	link, err := s.linkRepo.GetByPR(ctx, workspaceID, repo, prNumber)
	if err != nil {
		return err
	}
	if link == nil && branch != "" {
		link, err = s.linkRepo.GetByBranch(ctx, workspaceID, repo, branch)
		if err != nil {
			return err
		}
	}
	var story *model.PMTask
	if link != nil {
		link.PRNumber = &prNumber
		link.PRTitle = &prTitle
		link.PRURL = &prURL
		link.PRStatus = &prStatus
		if err := s.linkRepo.Update(ctx, link); err != nil {
			return err
		}

		target, err := s.deliveryRepo.GetByTask(ctx, workspaceID, link.TaskID)
		if err == nil && target != nil {
			target.ActivePRNumber = &prNumber
			target.ActivePRTitle = &prTitle
			target.ActivePRURL = &prURL
			target.ActivePRStatus = &prStatus
			now := time.Now()
			target.LastSyncedAt = &now
			switch prStatus {
			case "open":
				target.DeliveryState = "pr_open"
			case "merged":
				target.DeliveryState = "merged"
			case "closed":
				target.DeliveryState = "closed"
			}
			_ = s.deliveryRepo.Save(ctx, target)
		}

		story, err = s.taskRepo.GetRawByID(ctx, link.TaskID)
		if err != nil {
			story = nil
		}
		if story != nil && story.TeamID != nil && *story.TeamID != "" {
			if teamDefault, cfgErr := s.settingsRepo.GetTeamRepoDefault(ctx, *story.TeamID); cfgErr == nil && teamDefault != nil && teamDefault.AutoSyncStates {
				switch prStatus {
				case "open":
					if teamDefault.ReviewStateID != nil {
						story.WorkflowStateID = *teamDefault.ReviewStateID
						_ = s.taskRepo.Update(ctx, story)
					}
				case "merged":
					if teamDefault.DoneStateID != nil {
						story.WorkflowStateID = *teamDefault.DoneStateID
						_ = s.taskRepo.Update(ctx, story)
					}
				}
			}
		}
	}

	if s.ruleEngine != nil {
		event := model.AutomationEvent{
			WorkspaceID:       workspaceID,
			RepoFullName:      repo,
			Branch:            branch,
			BaseBranch:        baseBranch,
			PullRequestNumber: prNumber,
		}
		if link != nil {
			event.TargetType = "task"
			event.TargetID = link.TaskID
			event.TaskID = link.TaskID
			event.StoryID = link.TaskID
		}
		if story != nil {
			event.StateID = story.WorkflowStateID
			if story.TeamID != nil {
				event.TeamID = *story.TeamID
			}
		}
		switch {
		case prStatus == "merged":
			event.TriggerType = model.TriggerGitHubPRMerged
			s.ruleEngine.EvaluateEvent(ctx, event, nil)
		case strings.TrimSpace(action) == "opened":
			event.TriggerType = model.TriggerGitHubPROpened
			s.ruleEngine.EvaluateEvent(ctx, event, nil)
		case strings.TrimSpace(action) == "review_requested":
			event.TriggerType = model.TriggerGitHubPRReviewReq
			s.ruleEngine.EvaluateEvent(ctx, event, nil)
		}
	}

	if link != nil {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "task_git_link",
			EntityID:    link.ID,
			WorkspaceID: workspaceID,
			ParentType:  "task",
			ParentID:    link.TaskID,
		})
	}
	return nil
}

// ProcessWebhookRelease handles a release event from GitHub.
func (s *GitService) ProcessWebhookRelease(ctx context.Context, workspaceID, repo, action, tagName, targetCommitish string) error {
	if strings.TrimSpace(action) != "published" || s.ruleEngine == nil {
		return nil
	}

	event := model.AutomationEvent{
		WorkspaceID:  workspaceID,
		TriggerType:  model.TriggerGitHubReleasePub,
		RepoFullName: repo,
		Branch:       targetCommitish,
		TagName:      tagName,
	}
	if strings.TrimSpace(targetCommitish) != "" {
		link, err := s.linkRepo.GetByBranch(ctx, workspaceID, repo, targetCommitish)
		if err != nil {
			return err
		}
		if link != nil {
			event.TargetType = "task"
			event.TargetID = link.TaskID
			event.TaskID = link.TaskID
			event.StoryID = link.TaskID
			if task, taskErr := s.taskRepo.GetRawByID(ctx, link.TaskID); taskErr == nil && task != nil {
				event.StateID = task.WorkflowStateID
				if task.TeamID != nil {
					event.TeamID = *task.TeamID
				}
			}
		}
	}
	s.ruleEngine.EvaluateEvent(ctx, event, nil)
	return nil
}

// ProcessWebhookCheckSuite handles a check_suite event from GitHub.
func (s *GitService) ProcessWebhookCheckSuite(ctx context.Context, workspaceID, repo, action, branch, conclusion string) error {
	if strings.TrimSpace(action) != "completed" || s.ruleEngine == nil {
		return nil
	}

	event := model.AutomationEvent{
		WorkspaceID:  workspaceID,
		TriggerType:  model.TriggerGitHubCheckSuite,
		RepoFullName: repo,
		Branch:       branch,
		Conclusion:   conclusion,
	}
	if strings.TrimSpace(branch) != "" {
		link, err := s.linkRepo.GetByBranch(ctx, workspaceID, repo, branch)
		if err != nil {
			return err
		}
		if link != nil {
			event.TargetType = "task"
			event.TargetID = link.TaskID
			event.TaskID = link.TaskID
			event.StoryID = link.TaskID
			if task, taskErr := s.taskRepo.GetRawByID(ctx, link.TaskID); taskErr == nil && task != nil {
				event.StateID = task.WorkflowStateID
				if task.TeamID != nil {
					event.TeamID = *task.TeamID
				}
			}
		}
	}
	s.ruleEngine.EvaluateEvent(ctx, event, nil)
	return nil
}

type gitHubInstallState struct {
	WorkspaceID string `json:"workspace_id"`
	ActorID     string `json:"actor_id,omitempty"`
	jwt.RegisteredClaims
}

func (s *GitService) signGitHubInstallState(workspaceID, actorID string) (string, error) {
	if s.stateSecret == "" {
		return "", fmt.Errorf("github app state secret is not configured")
	}
	if workspaceID == "" {
		return "", fmt.Errorf("workspace_id is required")
	}

	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	claims := gitHubInstallState{
		WorkspaceID: workspaceID,
		ActorID:     strings.TrimSpace(actorID),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        hex.EncodeToString(buf),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.stateSecret))
}

func (s *GitService) resolveGitHubInstallState(ctx context.Context, stateToken string) (*gitHubInstallState, *model.Workspace, string, error) {
	if s.workspaceRepo == nil {
		return nil, nil, "", fmt.Errorf("workspace repository is not configured")
	}
	stateToken = strings.TrimSpace(stateToken)
	if stateToken == "" {
		return nil, nil, "", fmt.Errorf("github app callback state is missing")
	}
	claims := &gitHubInstallState{}
	token, err := jwt.ParseWithClaims(stateToken, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.stateSecret), nil
	})
	if err != nil || token == nil || !token.Valid {
		return nil, nil, "", fmt.Errorf("invalid github app callback state")
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, claims.WorkspaceID)
	if err != nil {
		return nil, nil, "", err
	}
	if workspace == nil {
		return nil, nil, "", fmt.Errorf("workspace not found")
	}
	return claims, workspace, s.workspaceSettingsURL(workspace.Slug), nil
}

func (s *GitService) upsertGitHubIntegration(
	ctx context.Context,
	workspaceID string,
	installationID string,
	installation *githubapp.Installation,
	actorID string,
) (*model.GitIntegration, error) {
	accountLogin := strings.TrimSpace(installation.AccountLogin)
	appID := githubapp.InstallationIDString(installation.AppID)
	displayName := "GitHub App"
	if accountLogin != "" {
		displayName = "GitHub " + accountLogin
	}

	existing, err := s.integrationRepo.GetByInstallationID(ctx, "github", installationID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("github installation %s is already connected to another workspace", installationID)
	}
	if existing == nil {
		secret := generateWebhookSecret()
		integration := &model.GitIntegration{
			WorkspaceID:    workspaceID,
			Provider:       "github",
			DisplayName:    displayName,
			CredentialMode: "github_app",
			AccountLogin:   trimPtr(&accountLogin),
			InstallationID: strPtr(installationID),
			AppID:          trimPtr(&appID),
			WebhookSecret:  &secret,
			Active:         true,
		}
		if err := s.integrationRepo.Create(ctx, integration); err != nil {
			return nil, err
		}
		if s.activitySvc != nil && actorID != "" {
			_ = s.activitySvc.Log(ctx, workspaceID, "git_integration", integration.ID, &actorID, "created", nil, nil, &integration.DisplayName, nil)
		}
		s.publishSimpleEvent("created", "git_integration", integration.ID, workspaceID, actorID)
		return integration, nil
	}

	existing.DisplayName = displayName
	existing.CredentialMode = "github_app"
	existing.AccountLogin = trimPtr(&accountLogin)
	existing.InstallationID = strPtr(installationID)
	existing.AppID = trimPtr(&appID)
	existing.Active = true
	if existing.WebhookSecret == nil || strings.TrimSpace(*existing.WebhookSecret) == "" {
		secret := generateWebhookSecret()
		existing.WebhookSecret = &secret
	}
	if err := s.integrationRepo.Update(ctx, existing); err != nil {
		return nil, err
	}
	if s.activitySvc != nil && actorID != "" {
		_ = s.activitySvc.Log(ctx, workspaceID, "git_integration", existing.ID, &actorID, "updated", nil, nil, &existing.DisplayName, nil)
	}
	s.publishSimpleEvent("updated", "git_integration", existing.ID, workspaceID, actorID)
	return existing, nil
}

func (s *GitService) workspaceSettingsURL(workspaceSlug string) string {
	base := strings.TrimRight(s.appBaseURL, "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	return fmt.Sprintf("%s/w/%s/settings/delivery", base, workspaceSlug)
}

func withGitHubInstallStatus(baseURL, status, message string, params map[string]string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	query := parsed.Query()
	query.Set("github_app", status)
	if strings.TrimSpace(message) != "" {
		query.Set("github_message", message)
	}
	for key, value := range params {
		if strings.TrimSpace(value) == "" {
			continue
		}
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (s *GitService) publishSimpleEvent(action, entity, entityID, workspaceID, actorID string) {
	if s.wsPublisher == nil {
		return
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      action,
		Entity:      entity,
		EntityID:    entityID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})
}

func generateWebhookSecret() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func signGitHubWebhook(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func trimPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func defaultAccountLogin(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "GitHub"
	}
	return *value
}

func defaultBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "main"
	}
	return branch
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// QueueForAgent returns the shared Temporal queue for an agent capability profile.
func QueueForAgent(agent *model.Agent) string {
	if agent == nil {
		return temporalapp.QueueAutomation
	}
	return temporalapp.QueueForRuntime(agent.RuntimeKind, resolveInvocationMode(agent))
}
