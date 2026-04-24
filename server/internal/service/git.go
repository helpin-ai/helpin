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
	"sort"
	"strconv"
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
	orgRepo         *repository.OrganizationRepository
	taskRepo        *repository.PMTaskRepository
	activitySvc     *PMActivityService
	wsPublisher     *websocket.Publisher
	ruleEngine      *AutomationRuleEngine
	githubApp       gitHubAppClient
	appBaseURL      string
	githubAppSlug   string
	stateSecret     string
}

type gitHubAppClient interface {
	ListInstallationRepositories(ctx context.Context, installationID string) ([]githubapp.Repository, error)
	ListRepositoryBranches(ctx context.Context, installationID, owner, repo string) ([]githubapp.Branch, error)
	GetInstallation(ctx context.Context, installationID string) (*githubapp.Installation, error)
	MergeBranch(ctx context.Context, installationID, owner, repo, base, head, commitMessage string) error
}

// NewGitService creates a new GitService.
func NewGitService(
	integrationRepo *repository.GitIntegrationRepository,
	repoRepo *repository.GitRepositoryRepository,
	linkRepo *repository.TaskGitLinkRepository,
	deliveryRepo *repository.TaskDeliveryTargetRepository,
	settingsRepo *repository.SettingsRepository,
	workspaceRepo *repository.WorkspaceRepository,
	orgRepo *repository.OrganizationRepository,
	taskRepo *repository.PMTaskRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
	githubApp *githubapp.Client,
	appBaseURL string,
	githubAppSlug string,
	stateSecret string,
) *GitService {
	var app gitHubAppClient
	if githubApp != nil {
		app = githubApp
	}
	return &GitService{
		integrationRepo: integrationRepo,
		repoRepo:        repoRepo,
		linkRepo:        linkRepo,
		deliveryRepo:    deliveryRepo,
		settingsRepo:    settingsRepo,
		workspaceRepo:   workspaceRepo,
		orgRepo:         orgRepo,
		taskRepo:        taskRepo,
		activitySvc:     activitySvc,
		wsPublisher:     wsPublisher,
		githubApp:       app,
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

func (s *GitService) GetIntegrationDetail(ctx context.Context, workspaceID, integrationID string) (*model.GitIntegrationDetail, error) {
	if workspaceID == "" || integrationID == "" {
		return nil, fmt.Errorf("workspace_id and integration_id are required")
	}
	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, integrationID)
	if err != nil {
		return nil, fmt.Errorf("get integration: %w", err)
	}
	if integration == nil {
		return nil, fmt.Errorf("integration not found")
	}
	affected, err := s.repoRepo.ListAffectedWorkspacesByIntegration(ctx, integration.ID)
	if err != nil {
		return nil, err
	}
	return &model.GitIntegrationDetail{
		Integration:        *integration,
		AffectedWorkspaces: affected,
	}, nil
}

// DeleteIntegration soft-deletes an org-scoped git integration and its repo claims.
func (s *GitService) DeleteIntegration(ctx context.Context, workspaceID, integrationID, actorID string) error {
	if workspaceID == "" || integrationID == "" {
		return fmt.Errorf("workspace_id and integration_id are required")
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if workspace == nil || workspace.OrganizationID == nil || strings.TrimSpace(*workspace.OrganizationID) == "" {
		return fmt.Errorf("workspace organization is required")
	}
	if !s.isOrgOwner(ctx, strings.TrimSpace(*workspace.OrganizationID), actorID) {
		return fmt.Errorf("only organization owners can uninstall git integrations")
	}

	existing, err := s.integrationRepo.GetByID(ctx, workspaceID, integrationID)
	if err != nil {
		return fmt.Errorf("get integration: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("integration not found")
	}
	if err := s.repoRepo.SoftDeleteByIntegration(ctx, integrationID); err != nil {
		return fmt.Errorf("delete integration repositories: %w", err)
	}
	if err := s.integrationRepo.SoftDeleteByID(ctx, integrationID); err != nil {
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
	workspace, err := s.workspaceRepo.GetByID(ctx, req.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if workspace == nil {
		return nil, fmt.Errorf("workspace not found")
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
		OrganizationID: workspace.OrganizationID,
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

	if s.activitySvc != nil && actorID != "" {
		_ = s.activitySvc.Log(ctx, integration.WorkspaceID, "git_integration", integration.ID, &actorID, "created", nil, nil, &integration.DisplayName, nil)
	}
	s.publishSimpleEvent("created", "git_integration", integration.ID, integration.WorkspaceID, actorID)
	return integration, nil
}

// SyncRepositories refreshes metadata for repo claims already wired to this workspace.
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
	if !integration.Active {
		return nil, fmt.Errorf("git integration is inactive")
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

	now := time.Now().UTC()
	remoteByExternalID := make(map[string]githubapp.Repository, len(repos))
	for _, repo := range repos {
		remoteByExternalID[githubapp.InstallationIDString(repo.ID)] = repo
	}
	claims, err := s.repoRepo.ListByWorkspaceAndIntegration(ctx, workspaceID, integrationID)
	if err != nil {
		return nil, err
	}
	for i := range claims {
		remote, ok := remoteByExternalID[claims[i].ExternalID]
		if !ok {
			if err := s.repoRepo.SoftDeleteByID(ctx, claims[i].ID); err != nil {
				return nil, err
			}
			continue
		}
		permissions, _ := json.Marshal(remote.Permissions)
		claims[i].FullName = remote.FullName
		claims[i].DefaultBranch = defaultBranch(remote.DefaultBranch)
		claims[i].Permissions = permissions
		claims[i].Private = remote.Private
		claims[i].Archived = remote.Archived
		claims[i].Active = true
		claims[i].DeletedAt = nil
		claims[i].UpdatedAt = now
		if err := s.repoRepo.Update(ctx, &claims[i]); err != nil {
			return nil, err
		}
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

// GetRepositoryByFullName returns a synced repository record for a workspace.
func (s *GitService) GetRepositoryByFullName(ctx context.Context, workspaceID, repoFullName string) (*model.GitRepository, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(repoFullName) == "" {
		return nil, fmt.Errorf("repo_full_name is required")
	}
	return s.repoRepo.GetByFullName(ctx, workspaceID, repoFullName)
}

// ResolveReleaseKind classifies a release relative to the previous published release.
func (s *GitService) ResolveReleaseKind(ctx context.Context, workspaceID, repoFullName, tagName string) (string, error) {
	if s == nil {
		return "unknown", fmt.Errorf("git service is not configured")
	}
	repo, err := s.GetRepositoryByFullName(ctx, workspaceID, repoFullName)
	if err != nil {
		return "unknown", err
	}
	if repo == nil {
		return "unknown", fmt.Errorf("repository not found")
	}
	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, repo.IntegrationID)
	if err != nil {
		return "unknown", err
	}
	if integration == nil {
		return "unknown", fmt.Errorf("git integration not found")
	}
	if integration.Provider != "github" {
		return "unknown", fmt.Errorf("release facts are not implemented for %s", integration.Provider)
	}
	if integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
		return "unknown", fmt.Errorf("integration has no installation_id")
	}
	if s.githubApp == nil {
		return "unknown", fmt.Errorf("github app is not configured")
	}
	owner, repoName, err := splitRepositoryFullName(repo.FullName)
	if err != nil {
		return "unknown", err
	}
	current, err := s.githubApp.GetReleaseByTag(ctx, strings.TrimSpace(*integration.InstallationID), owner, repoName, tagName)
	if err != nil {
		return "unknown", err
	}
	if current == nil {
		return "unknown", nil
	}
	releases, err := s.githubApp.ListReleases(ctx, strings.TrimSpace(*integration.InstallationID), owner, repoName, githubapp.ListReleasesOptions{
		IncludeDrafts:      false,
		IncludePrereleases: true,
		PerPage:            100,
		MaxPages:           5,
	})
	if err != nil {
		return "unknown", err
	}
	return classifyReleaseKind(current, selectPreviousRelease(releases, current)), nil
}

// ListRepositoryBranches returns available branches for a synced repository.
func (s *GitService) ListRepositoryBranches(ctx context.Context, workspaceID, repoID string) ([]model.GitBranch, error) {
	repo, err := s.GetRepositoryByID(ctx, workspaceID, repoID)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("repository not found")
	}

	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, repo.IntegrationID)
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, fmt.Errorf("git integration not found")
	}

	switch integration.Provider {
	case "github":
		if s.githubApp == nil {
			return nil, fmt.Errorf("github app credentials are not configured")
		}
		parts := strings.SplitN(strings.TrimSpace(repo.FullName), "/", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("repository full name is invalid")
		}
		if integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
			return nil, fmt.Errorf("integration has no installation_id")
		}
		branches, err := s.githubApp.ListRepositoryBranches(ctx, *integration.InstallationID, parts[0], parts[1])
		if err != nil {
			return nil, err
		}
		items := make([]model.GitBranch, 0, len(branches))
		defaultBranch := strings.TrimSpace(repo.DefaultBranch)
		for _, branch := range branches {
			items = append(items, model.GitBranch{
				Name:      branch.Name,
				IsDefault: strings.EqualFold(strings.TrimSpace(branch.Name), defaultBranch),
			})
		}
		return items, nil
	default:
		return nil, fmt.Errorf("branch listing is not implemented for %s", integration.Provider)
	}
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

// GetGitHubInstallURL returns either a fresh install URL or the repo-picker action
// for an existing installation visible to this organization.
func (s *GitService) GetGitHubInstallURL(ctx context.Context, workspaceID, actorID string, forceInstall bool) (string, string, *string, error) {
	if workspaceID == "" {
		return "", "", nil, fmt.Errorf("workspace_id is required")
	}
	if s.githubApp == nil || s.githubAppSlug == "" {
		return "", "", nil, fmt.Errorf("github app onboarding is not configured")
	}
	if s.workspaceRepo == nil {
		return "", "", nil, fmt.Errorf("workspace repository is not configured")
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return "", "", nil, err
	}
	if workspace == nil {
		return "", "", nil, fmt.Errorf("workspace not found")
	}
	if workspace.OrganizationID == nil || strings.TrimSpace(*workspace.OrganizationID) == "" {
		return "", "", nil, fmt.Errorf("workspace organization is required")
	}
	if !forceInstall {
		integration, err := s.integrationRepo.GetActiveByOrganization(ctx, strings.TrimSpace(*workspace.OrganizationID), "github")
		if err != nil {
			return "", "", nil, err
		}
		if integration != nil {
			manageURL := s.githubInstallationManageURL(integration)
			return manageURL, "pick_repos", &integration.ID, nil
		}
	}

	state, err := s.signGitHubInstallState(workspaceID, actorID)
	if err != nil {
		return "", "", nil, err
	}

	installURL := url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   "/apps/" + s.githubAppSlug + "/installations/new",
	}
	query := installURL.Query()
	query.Set("state", state)
	installURL.RawQuery = query.Encode()
	return installURL.String(), "install", nil, nil
}

func (s *GitService) githubInstallationManageURL(integration *model.GitIntegration) string {
	if integration == nil || integration.InstallationID == nil {
		return ""
	}
	installationID := strings.TrimSpace(*integration.InstallationID)
	if installationID == "" {
		return ""
	}
	if integration.AccountLogin != nil && strings.TrimSpace(*integration.AccountLogin) != "" {
		return fmt.Sprintf("https://github.com/organizations/%s/settings/installations/%s", url.PathEscape(strings.TrimSpace(*integration.AccountLogin)), url.PathEscape(installationID))
	}
	return fmt.Sprintf("https://github.com/settings/installations/%s", url.PathEscape(installationID))
}

// CompleteGitHubInstall creates or reuses the org integration after GitHub redirects back.
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

	integration, err := s.upsertGitHubIntegration(ctx, workspace, installationID, installation, state.ActorID)
	if err != nil {
		return withGitHubInstallStatus(redirectURL, "error", err.Error(), nil), nil
	}

	params := map[string]string{
		"integration_id": integration.ID,
	}
	return withGitHubInstallStatus(redirectURL, "connected", fmt.Sprintf("GitHub App connected to %s.", defaultAccountLogin(integration.AccountLogin)), params), nil
}

// ResolveGitHubWebhookIntegration verifies the webhook signature and resolves the installation.
func (s *GitService) ResolveGitHubWebhookIntegration(ctx context.Context, installationID string, body []byte, signature string) (*model.GitIntegration, error) {
	if strings.TrimSpace(installationID) == "" {
		return nil, fmt.Errorf("installation_id is required")
	}

	integration, err := s.integrationRepo.GetByInstallationID(ctx, "github", installationID)
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, fmt.Errorf("no workspace matches github installation %s", installationID)
	}
	if integration.WebhookSecret != nil && *integration.WebhookSecret != "" {
		expected := signGitHubWebhook(*integration.WebhookSecret, body)
		if !hmac.Equal([]byte(expected), []byte(signature)) {
			return nil, fmt.Errorf("invalid github webhook signature")
		}
	}
	return integration, nil
}

func (s *GitService) ListAvailableRepos(ctx context.Context, workspaceID, integrationID, actorID string) ([]model.GitAvailableRepo, error) {
	if err := s.assertCanEnumerateAvailableRepos(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, integrationID)
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, fmt.Errorf("integration not found")
	}
	if !integration.Active {
		return nil, fmt.Errorf("integration is inactive")
	}
	if integration.Provider != "github" {
		return nil, fmt.Errorf("available repos are only supported for github")
	}
	if integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
		return nil, fmt.Errorf("integration has no installation_id")
	}
	if s.githubApp == nil {
		return nil, fmt.Errorf("github app credentials are not configured")
	}

	repos, err := s.githubApp.ListInstallationRepositories(ctx, *integration.InstallationID)
	if err != nil {
		return nil, err
	}
	items := make([]model.GitAvailableRepo, 0, len(repos))
	for _, repo := range repos {
		externalID := githubapp.InstallationIDString(repo.ID)
		claimedBy, err := s.repoRepo.GetClaimedByByExternalID(ctx, integration.ID, externalID)
		if err != nil {
			return nil, err
		}
		items = append(items, model.GitAvailableRepo{
			ExternalID:    externalID,
			FullName:      repo.FullName,
			DefaultBranch: defaultBranch(repo.DefaultBranch),
			Private:       repo.Private,
			Archived:      repo.Archived,
			Permissions:   repo.Permissions,
			ClaimedBy:     claimedBy,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].FullName) < strings.ToLower(items[j].FullName)
	})
	return items, nil
}

func (s *GitService) WireRepositories(ctx context.Context, currentWorkspaceID, integrationID string, req model.WireGitRepositoriesRequest, actorID string) ([]model.GitRepository, *model.WireGitRepositoriesConflictResponse, error) {
	targetWorkspaceID := strings.TrimSpace(req.WorkspaceID)
	if targetWorkspaceID == "" {
		targetWorkspaceID = currentWorkspaceID
	}
	if targetWorkspaceID != currentWorkspaceID {
		return nil, nil, fmt.Errorf("workspace_id must match the current workspace")
	}
	if len(req.RepoIDs) == 0 {
		return nil, nil, fmt.Errorf("repo_ids are required")
	}

	integration, err := s.integrationRepo.GetByID(ctx, currentWorkspaceID, integrationID)
	if err != nil {
		return nil, nil, err
	}
	if integration == nil {
		return nil, nil, fmt.Errorf("integration not found")
	}
	if !integration.Active {
		return nil, nil, fmt.Errorf("integration is inactive")
	}
	if integration.Provider != "github" {
		return nil, nil, fmt.Errorf("repo wiring is only supported for github")
	}
	if integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
		return nil, nil, fmt.Errorf("integration has no installation_id")
	}
	if s.githubApp == nil {
		return nil, nil, fmt.Errorf("github app credentials are not configured")
	}

	available, err := s.githubApp.ListInstallationRepositories(ctx, *integration.InstallationID)
	if err != nil {
		return nil, nil, err
	}
	availableByID := make(map[string]githubapp.Repository, len(available))
	for _, repo := range available {
		availableByID[githubapp.InstallationIDString(repo.ID)] = repo
	}

	seen := make(map[string]struct{}, len(req.RepoIDs))
	requestedRepoIDs := make([]string, 0, len(req.RepoIDs))
	for _, repoID := range req.RepoIDs {
		repoID = strings.TrimSpace(repoID)
		if repoID == "" {
			continue
		}
		if _, ok := seen[repoID]; ok {
			continue
		}
		seen[repoID] = struct{}{}
		requestedRepoIDs = append(requestedRepoIDs, repoID)
	}
	if len(requestedRepoIDs) == 0 {
		return nil, nil, fmt.Errorf("repo_ids are required")
	}

	conflicts := make([]model.WireGitRepositoriesConflict, 0)
	for _, repoID := range requestedRepoIDs {
		repoID = strings.TrimSpace(repoID)
		if _, ok := availableByID[repoID]; !ok {
			return nil, nil, fmt.Errorf("repository %s is not available to this installation", repoID)
		}

		claimedBy, err := s.repoRepo.GetClaimedByByExternalID(ctx, integration.ID, repoID)
		if err != nil {
			return nil, nil, err
		}
		if claimedBy != nil && claimedBy.WorkspaceID != currentWorkspaceID {
			conflicts = append(conflicts, model.WireGitRepositoriesConflict{
				ExternalID:             repoID,
				ClaimedByWorkspaceID:   claimedBy.WorkspaceID,
				ClaimedByWorkspaceName: claimedBy.WorkspaceName,
			})
		}
	}

	if len(conflicts) > 0 {
		return nil, &model.WireGitRepositoriesConflictResponse{Conflicts: conflicts}, nil
	}

	upserted := make([]model.GitRepository, 0, len(requestedRepoIDs))
	for _, repoID := range requestedRepoIDs {
		remote := availableByID[repoID]
		permissions, _ := json.Marshal(remote.Permissions)
		repo := model.GitRepository{
			WorkspaceID:   currentWorkspaceID,
			IntegrationID: integration.ID,
			Provider:      integration.Provider,
			ExternalID:    repoID,
			FullName:      remote.FullName,
			DefaultBranch: defaultBranch(remote.DefaultBranch),
			Permissions:   permissions,
			Private:       remote.Private,
			Archived:      remote.Archived,
			Selected:      true,
			Active:        true,
			DeletedAt:     nil,
		}
		if err := s.repoRepo.UpsertRepository(ctx, &repo); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "already claimed by workspace") {
				claimedBy, claimErr := s.repoRepo.GetClaimedByByExternalID(ctx, integration.ID, repoID)
				if claimErr == nil && claimedBy != nil {
					return nil, &model.WireGitRepositoriesConflictResponse{
						Conflicts: []model.WireGitRepositoriesConflict{{
							ExternalID:             repoID,
							ClaimedByWorkspaceID:   claimedBy.WorkspaceID,
							ClaimedByWorkspaceName: claimedBy.WorkspaceName,
						}},
					}, nil
				}
			}
			return nil, nil, err
		}
		upserted = append(upserted, repo)
	}

	if s.activitySvc != nil && actorID != "" {
		for i := range upserted {
			value := upserted[i].FullName
			_ = s.activitySvc.Log(ctx, currentWorkspaceID, "git_repository", upserted[i].ID, &actorID, "created", nil, nil, &value, nil)
		}
	}
	return upserted, nil, nil
}

func (s *GitService) UnwireRepository(ctx context.Context, workspaceID, integrationID, repoID, actorID string) error {
	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, integrationID)
	if err != nil {
		return err
	}
	if integration == nil {
		return fmt.Errorf("integration not found")
	}
	repo, err := s.repoRepo.GetByID(ctx, workspaceID, repoID)
	if err != nil {
		return err
	}
	if repo == nil || repo.IntegrationID != integration.ID {
		return fmt.Errorf("repository not found")
	}
	if err := s.repoRepo.SoftDeleteByID(ctx, repo.ID); err != nil {
		return err
	}
	if s.activitySvc != nil && actorID != "" {
		value := repo.FullName
		_ = s.activitySvc.Log(ctx, workspaceID, "git_repository", repo.ID, &actorID, "deleted", nil, nil, &value, nil)
	}
	s.publishSimpleEvent("deleted", "git_repository", repo.ID, workspaceID, actorID)
	return nil
}

func (s *GitService) ResolveForAgentRun(ctx context.Context, workspaceID, repoID string) (*model.GitIntegration, *model.GitRepository, error) {
	repo, err := s.repoRepo.GetByIDAny(ctx, repoID)
	if err != nil {
		return nil, nil, err
	}
	if repo == nil {
		return nil, nil, fmt.Errorf("repository not found")
	}
	if repo.WorkspaceID != workspaceID {
		return nil, nil, fmt.Errorf("repository does not belong to this workspace")
	}
	if repo.DeletedAt != nil || !repo.Active {
		return nil, nil, fmt.Errorf("repository is inactive")
	}
	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, repo.IntegrationID)
	if err != nil {
		return nil, nil, err
	}
	if integration == nil {
		return nil, nil, fmt.Errorf("integration not found")
	}
	if !integration.Active {
		return nil, nil, fmt.Errorf("integration is inactive")
	}
	return integration, repo, nil
}

func (s *GitService) ResolveWebhookRepository(ctx context.Context, integrationID, externalID string) (*model.GitRepository, error) {
	if strings.TrimSpace(integrationID) == "" || strings.TrimSpace(externalID) == "" {
		return nil, fmt.Errorf("integration_id and external_id are required")
	}
	return s.repoRepo.GetActiveByExternalID(ctx, integrationID, externalID)
}

func (s *GitService) IntegrationHasWebhookClaims(ctx context.Context, integrationID string) (bool, error) {
	if strings.TrimSpace(integrationID) == "" {
		return false, fmt.Errorf("integration_id is required")
	}
	count, err := s.repoRepo.CountActiveByIntegration(ctx, integrationID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *GitService) HandleInstallationLifecycleEvent(ctx context.Context, integration *model.GitIntegration, event, action string, externalIDs []string) error {
	if integration == nil {
		return fmt.Errorf("integration is required")
	}
	now := time.Now().UTC()
	switch event {
	case "installation":
		switch strings.TrimSpace(action) {
		case "deleted":
			integration.Active = false
			integration.DeletedAt = &now
			if err := s.integrationRepo.Update(ctx, integration); err != nil {
				return err
			}
			return s.repoRepo.SoftDeleteByIntegration(ctx, integration.ID)
		case "suspend":
			integration.Active = false
			if err := s.integrationRepo.Update(ctx, integration); err != nil {
				return err
			}
		case "unsuspend":
			if integration.DeletedAt == nil {
				integration.Active = true
				if err := s.integrationRepo.Update(ctx, integration); err != nil {
					return err
				}
			}
		}
	case "installation_repositories":
		switch strings.TrimSpace(action) {
		case "removed":
			for _, externalID := range externalIDs {
				repo, err := s.repoRepo.GetActiveByExternalID(ctx, integration.ID, externalID)
				if err != nil {
					return err
				}
				if repo == nil {
					continue
				}
				if err := s.repoRepo.SoftDeleteByID(ctx, repo.ID); err != nil {
					return err
				}
			}
		case "added":
			for _, externalID := range externalIDs {
				repo, err := s.repoRepo.GetByExternalID(ctx, integration.ID, externalID)
				if err != nil {
					return err
				}
				if repo == nil || repo.DeletedAt == nil {
					continue
				}
				repo.Active = true
				repo.DeletedAt = nil
				repo.UpdatedAt = now
				if err := s.repoRepo.Update(ctx, repo); err != nil {
					return err
				}
			}
		}
	}
	return nil
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

	if s.activitySvc != nil && actorID != "" {
		_ = s.activitySvc.Log(ctx, workspaceID, "task", storyID, &actorID, "updated", strPtr("delivery_target"), nil, target.RepoFullName, nil)
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "task_delivery_target",
			EntityID:    target.ID,
			WorkspaceID: workspaceID,
			ParentType:  "task",
			ParentID:    storyID,
			ActorID:     actorID,
		})
	}
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

// ResolveTaskRunBranchValues returns the effective base and working branch values for a task run.
func (s *GitService) ResolveTaskRunBranchValues(ctx context.Context, workspaceID, taskID string) (string, string, error) {
	target, err := s.GetTaskDeliveryTarget(ctx, workspaceID, taskID)
	if err != nil {
		return "", "", err
	}
	if target == nil {
		return "", "", fmt.Errorf("task delivery target not found")
	}

	baseBranch := strings.TrimSpace(derefString(target.BaseBranch))
	workingBranch := strings.TrimSpace(derefString(target.WorkingBranch))
	if workingBranch != "" {
		return baseBranch, workingBranch, nil
	}

	task, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return "", "", err
	}
	if task == nil {
		return "", "", fmt.Errorf("task not found")
	}

	var teamDefault *model.PMTeamRepoDefault
	if task.TeamID != nil && strings.TrimSpace(*task.TeamID) != "" {
		teamDefault, err = s.settingsRepo.GetTeamRepoDefault(ctx, *task.TeamID)
		if err != nil {
			return "", "", err
		}
	}

	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return "", "", err
	}
	if workspace == nil {
		return "", "", fmt.Errorf("workspace not found")
	}

	return baseBranch, model.BuildTaskWorkingBranch(task, teamDefault, workspace.WorkspaceKey), nil
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

type deliveryStatusMetadata struct {
	LinkID   string
	PRNumber *int
	PRTitle  *string
	PRURL    *string
}

// UpdateDeliveryStatusAfterMerge reconciles task delivery state after a direct
// merge path succeeds, without waiting for a GitHub webhook.
func (s *GitService) UpdateDeliveryStatusAfterMerge(ctx context.Context, workspaceID, taskID, prStatus string) error {
	return s.updateDeliveryStatusForPR(ctx, workspaceID, taskID, prStatus, nil)
}

func (s *GitService) updateDeliveryStatusForPR(ctx context.Context, workspaceID, taskID, prStatus string, meta *deliveryStatusMetadata) error {
	prStatus = strings.TrimSpace(prStatus)
	if workspaceID == "" || taskID == "" || prStatus == "" {
		return fmt.Errorf("workspace_id, task_id, and pr_status are required")
	}
	if s.deliveryRepo == nil || s.linkRepo == nil {
		return fmt.Errorf("git delivery repositories are not configured")
	}

	target, err := s.deliveryRepo.GetByTask(ctx, workspaceID, taskID)
	if err != nil {
		return fmt.Errorf("load delivery target: %w", err)
	}

	now := time.Now()
	if target != nil {
		target.ActivePRStatus = &prStatus
		target.LastSyncedAt = &now
		if meta != nil {
			if meta.PRNumber != nil {
				target.ActivePRNumber = meta.PRNumber
			}
			if meta.PRTitle != nil {
				target.ActivePRTitle = meta.PRTitle
			}
			if meta.PRURL != nil {
				target.ActivePRURL = meta.PRURL
			}
		}
		if deliveryState := deliveryStateForPRStatus(prStatus); deliveryState != "" {
			target.DeliveryState = deliveryState
		}
		if err := s.deliveryRepo.Save(ctx, target); err != nil {
			return fmt.Errorf("save delivery target: %w", err)
		}
	}

	links, err := s.linkRepo.ListByTask(ctx, workspaceID, taskID)
	if err != nil {
		return fmt.Errorf("list task git links: %w", err)
	}
	for i := range links {
		updateLink := shouldUpdateGitLinkForDeliveryStatus(links[i], target)
		if meta != nil && meta.LinkID != "" && links[i].ID == meta.LinkID {
			updateLink = true
		}
		if !updateLink {
			continue
		}
		links[i].PRStatus = &prStatus
		if meta != nil {
			if meta.PRNumber != nil {
				links[i].PRNumber = meta.PRNumber
			}
			if meta.PRTitle != nil {
				links[i].PRTitle = meta.PRTitle
			}
			if meta.PRURL != nil {
				links[i].PRURL = meta.PRURL
			}
		}
		if err := s.linkRepo.Update(ctx, &links[i]); err != nil {
			return fmt.Errorf("update task git link: %w", err)
		}
		s.publishTaskGitLinkUpdated(workspaceID, links[i].ID, taskID)
	}

	if prStatus == "merged" && s.taskRepo != nil && s.settingsRepo != nil {
		story, err := s.taskRepo.GetRawByID(ctx, taskID)
		if err != nil {
			return fmt.Errorf("load task: %w", err)
		}
		if story != nil && story.TeamID != nil && *story.TeamID != "" {
			teamDefault, cfgErr := s.settingsRepo.GetTeamRepoDefault(ctx, *story.TeamID)
			if cfgErr != nil {
				return fmt.Errorf("load team repo default: %w", cfgErr)
			}
			if teamDefault != nil && teamDefault.AutoSyncStates && teamDefault.DoneStateID != nil {
				story.WorkflowStateID = *teamDefault.DoneStateID
				if err := s.taskRepo.Update(ctx, story); err != nil {
					return fmt.Errorf("update task workflow state: %w", err)
				}
			}
		}
	}

	return nil
}

func deliveryStateForPRStatus(prStatus string) string {
	switch prStatus {
	case "open":
		return "pr_open"
	case "merged":
		return "merged"
	case "closed":
		return "closed"
	default:
		return ""
	}
}

func shouldUpdateGitLinkForDeliveryStatus(link model.TaskGitLink, target *model.TaskDeliveryTarget) bool {
	if target == nil {
		return true
	}
	if target.RepoFullName != nil && *target.RepoFullName != "" && link.Repo != *target.RepoFullName {
		return false
	}
	if target.WorkingBranch != nil && *target.WorkingBranch != "" {
		return link.Branch != nil && *link.Branch == *target.WorkingBranch
	}
	return true
}

func (s *GitService) publishTaskGitLinkUpdated(workspaceID, linkID, taskID string) {
	if s.wsPublisher == nil || linkID == "" {
		return
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "task_git_link",
		EntityID:    linkID,
		WorkspaceID: workspaceID,
		ParentType:  "task",
		ParentID:    taskID,
	})
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
		err = s.updateDeliveryStatusForPR(ctx, workspaceID, link.TaskID, prStatus, &deliveryStatusMetadata{
			LinkID:   link.ID,
			PRNumber: &prNumber,
			PRTitle:  &prTitle,
			PRURL:    &prURL,
		})
		if err != nil {
			return err
		}
		story, err = s.taskRepo.GetRawByID(ctx, link.TaskID)
		if err != nil {
			story = nil
		}
		if prStatus == "open" && story != nil && story.TeamID != nil && *story.TeamID != "" {
			if teamDefault, cfgErr := s.settingsRepo.GetTeamRepoDefault(ctx, *story.TeamID); cfgErr == nil && teamDefault != nil && teamDefault.AutoSyncStates && teamDefault.ReviewStateID != nil {
				story.WorkflowStateID = *teamDefault.ReviewStateID
				_ = s.taskRepo.Update(ctx, story)
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

	return nil
}

func classifyWebhookReleaseKind(tagName string, isPrerelease bool) string {
	if isPrerelease {
		return "prerelease"
	}
	version := strings.TrimSpace(tagName)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	if version == "" {
		return "unknown"
	}
	if idx := strings.Index(version, "+"); idx >= 0 {
		version = version[:idx]
	}
	if idx := strings.Index(version, "-"); idx >= 0 {
		return "prerelease"
	}
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "unknown"
	}
	major, errMajor := strconv.Atoi(parts[0])
	minor, errMinor := strconv.Atoi(parts[1])
	patch, errPatch := strconv.Atoi(parts[2])
	if errMajor != nil || errMinor != nil || errPatch != nil {
		return "unknown"
	}
	switch {
	case patch > 0:
		return "patch"
	case minor > 0:
		return "minor"
	case major > 0:
		return "major"
	default:
		return "unknown"
	}
}

// ProcessWebhookRelease handles a release event from GitHub.
func (s *GitService) ProcessWebhookRelease(ctx context.Context, workspaceID, repo, action, tagName, targetCommitish, releaseName, releaseURL string, publishedAt *time.Time, isPrerelease bool) error {
	if strings.TrimSpace(action) != "published" || s.ruleEngine == nil {
		return nil
	}

	repository, err := s.repoRepo.GetByFullName(ctx, workspaceID, repo)
	if err != nil {
		return err
	}

	event := model.AutomationEvent{
		WorkspaceID:     workspaceID,
		TriggerType:     model.TriggerGitHubReleasePub,
		RepoFullName:    repo,
		Branch:          targetCommitish,
		TagName:         tagName,
		TargetCommitish: targetCommitish,
		ReleaseName:     releaseName,
		ReleaseURL:      releaseURL,
		PublishedAt:     publishedAt,
		IsPrerelease:    isPrerelease,
		ReleaseKind:     classifyWebhookReleaseKind(tagName, isPrerelease),
	}
	if repository != nil {
		event.RepositoryID = repository.ID
		event.TargetType = "repository"
		event.TargetID = repository.ID
	}
	if strings.TrimSpace(targetCommitish) != "" {
		link, err := s.linkRepo.GetByBranch(ctx, workspaceID, repo, targetCommitish)
		if err != nil {
			return err
		}
		if link != nil {
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
	workspace *model.Workspace,
	installationID string,
	installation *githubapp.Installation,
	actorID string,
) (*model.GitIntegration, error) {
	if workspace == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	if workspace.OrganizationID == nil || strings.TrimSpace(*workspace.OrganizationID) == "" {
		return nil, fmt.Errorf("workspace organization is required")
	}
	accountLogin := strings.TrimSpace(installation.AccountLogin)
	appID := githubapp.InstallationIDString(installation.AppID)
	displayName := "GitHub App"
	if accountLogin != "" {
		displayName = "GitHub " + accountLogin
	}
	orgID := strings.TrimSpace(*workspace.OrganizationID)

	existing, err := s.integrationRepo.GetByInstallationID(ctx, "github", installationID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.OrganizationID != nil && strings.TrimSpace(*existing.OrganizationID) != orgID {
		return nil, fmt.Errorf("github installation %s is already connected to another organization", installationID)
	}
	if existing == nil {
		secret := generateWebhookSecret()
		integration := &model.GitIntegration{
			WorkspaceID:    workspace.ID,
			OrganizationID: workspace.OrganizationID,
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
			_ = s.activitySvc.Log(ctx, workspace.ID, "git_integration", integration.ID, &actorID, "created", nil, nil, &integration.DisplayName, nil)
		}
		s.publishSimpleEvent("created", "git_integration", integration.ID, workspace.ID, actorID)
		return integration, nil
	}

	existing.DisplayName = displayName
	if existing.WorkspaceID == "" {
		existing.WorkspaceID = workspace.ID
	}
	existing.OrganizationID = workspace.OrganizationID
	existing.CredentialMode = "github_app"
	existing.AccountLogin = trimPtr(&accountLogin)
	existing.InstallationID = strPtr(installationID)
	existing.AppID = trimPtr(&appID)
	existing.Active = true
	existing.DeletedAt = nil
	if existing.WebhookSecret == nil || strings.TrimSpace(*existing.WebhookSecret) == "" {
		secret := generateWebhookSecret()
		existing.WebhookSecret = &secret
	}
	if err := s.integrationRepo.Update(ctx, existing); err != nil {
		return nil, err
	}
	if s.activitySvc != nil && actorID != "" {
		_ = s.activitySvc.Log(ctx, workspace.ID, "git_integration", existing.ID, &actorID, "updated", nil, nil, &existing.DisplayName, nil)
	}
	s.publishSimpleEvent("updated", "git_integration", existing.ID, workspace.ID, actorID)
	return existing, nil
}

func (s *GitService) isOrgOwner(ctx context.Context, organizationID, userID string) bool {
	if s.orgRepo == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(userID) == "" {
		return false
	}
	role, err := s.orgRepo.GetMemberRole(ctx, organizationID, userID)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(role), "owner")
}

func (s *GitService) assertCanEnumerateAvailableRepos(ctx context.Context, workspaceID, actorID string) error {
	if s.workspaceRepo == nil {
		return fmt.Errorf("workspace repository is not configured")
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if workspace == nil {
		return fmt.Errorf("workspace not found")
	}
	if workspace.OrganizationID == nil || strings.TrimSpace(*workspace.OrganizationID) == "" {
		return fmt.Errorf("workspace organization is required")
	}

	orgID := strings.TrimSpace(*workspace.OrganizationID)
	if s.isOrgOwner(ctx, orgID, actorID) {
		return nil
	}

	visibleWorkspaces, err := s.workspaceRepo.List(ctx, actorID, orgID)
	if err != nil {
		return err
	}
	for _, candidate := range visibleWorkspaces {
		role := strings.ToLower(strings.TrimSpace(candidate.Role))
		if role == model.RoleOwner || role == model.RoleAdmin {
			return nil
		}
	}
	return fmt.Errorf("forbidden")
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
