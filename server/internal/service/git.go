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
	"log/slog"
	"net/mail"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	agentruntime "github.com/helpin-ai/agent-runtime-go"
	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/gitlab"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

var ErrTaskDeliveryTargetRequired = errors.New("story has no delivery target configured")
var ErrEpicDeliveryTargetRequired = errors.New("epic has no delivery target configured")

// GitService contains git integration and story delivery business logic.
type GitService struct {
	integrationRepo  *repository.GitIntegrationRepository
	credentialRepo   *repository.GitCredentialRepository
	repoRepo         *repository.GitRepositoryRepository
	linkRepo         *repository.TaskGitLinkRepository
	deliveryRepo     *repository.TaskDeliveryTargetRepository
	epicDeliveryRepo *repository.EpicDeliveryTargetRepository
	settingsRepo     *repository.SettingsRepository
	workspaceRepo    *repository.WorkspaceRepository
	orgRepo          *repository.OrganizationRepository
	taskRepo         *repository.PMTaskRepository
	epicRepo         *repository.PMEpicRepository
	activitySvc      *PMActivityService
	wsPublisher      *websocket.Publisher
	ruleEngine       *AutomationRuleEngine
	githubApp        gitHubAppClient
	gitlabFactory    gitLabClientFactory
	encryptionKey    []byte
	availableRepos   *availableReposCache
	appBaseURL       string
	githubAppSlug    string
	stateSecret      string
}

func (s *GitService) SetEpicDeliveryDependencies(deliveryRepo *repository.EpicDeliveryTargetRepository, epicRepo *repository.PMEpicRepository) *GitService {
	s.epicDeliveryRepo = deliveryRepo
	s.epicRepo = epicRepo
	return s
}

type gitHubAppClient interface {
	MintInstallationToken(ctx context.Context, installationID string) (string, error)
	ListInstallationRepositories(ctx context.Context, installationID string) ([]githubapp.Repository, error)
	ListRepositoryBranches(ctx context.Context, installationID, owner, repo string) ([]githubapp.Branch, error)
	GetReleaseByTag(ctx context.Context, installationID, owner, repo, tag string) (*githubapp.Release, error)
	ListReleases(ctx context.Context, installationID, owner, repo string, opts githubapp.ListReleasesOptions) ([]githubapp.Release, error)
	GetInstallation(ctx context.Context, installationID string) (*githubapp.Installation, error)
	GetPullRequest(ctx context.Context, installationID, owner, repo string, number int) (*githubapp.PullRequest, error)
	EnsureBranch(ctx context.Context, installationID, owner, repo, branch, base string) error
	EnsurePullRequest(ctx context.Context, installationID, owner, repo string, input githubapp.EnsurePullRequestInput) (*githubapp.PullRequest, error)
	MergeBranch(ctx context.Context, installationID, owner, repo, base, head, commitMessage string) error
}

// gitLabClient is the API surface backed by gitlab.Client. Constructed per-credential
// using the credential's base URL.
type gitLabClient interface {
	WebBaseURL() string
	CurrentUser(ctx context.Context, accessToken string) (*gitlab.User, error)
	ListProjects(ctx context.Context, accessToken, search string) ([]gitlab.Project, error)
	GetProject(ctx context.Context, accessToken string, projectID int64) (*gitlab.Project, error)
	ListBranches(ctx context.Context, accessToken string, projectID int64) ([]gitlab.Branch, error)
	ListMergeRequests(ctx context.Context, accessToken string, projectID int64, sourceBranch, targetBranch string) ([]gitlab.MergeRequest, error)
	CreateMergeRequest(ctx context.Context, accessToken string, projectID int64, sourceBranch, targetBranch, title, description string) (*gitlab.MergeRequest, error)
	UpsertProjectWebhook(ctx context.Context, accessToken string, projectID int64, hookURL, secret string) (*gitlab.ProjectWebhook, error)
}

// newGitLabClient builds a per-call client for the credential's GitLab instance.
// The result is a *gitlab.Client; the indirection through gitLabClientFactory lets
// tests inject a fake.
type gitLabClientFactory func(baseURL string) gitLabClient

func defaultGitLabClientFactory(baseURL string) gitLabClient {
	return gitlab.NewClient(baseURL)
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
		availableRepos:  newAvailableReposCache(),
	}
}

// SetGitLabDependencies wires the GitLab credential store and token encryption key.
// GitLab API calls construct a per-request client from each credential's base URL.
func (s *GitService) SetGitLabDependencies(credentialRepo *repository.GitCredentialRepository, encryptionKey []byte) *GitService {
	s.credentialRepo = credentialRepo
	if s.gitlabFactory == nil {
		s.gitlabFactory = defaultGitLabClientFactory
	}
	s.encryptionKey = append([]byte(nil), encryptionKey...)
	return s
}

// SetGitLabClientFactory overrides the GitLab client factory (used by tests).
func (s *GitService) SetGitLabClientFactory(factory gitLabClientFactory) *GitService {
	if factory != nil {
		s.gitlabFactory = factory
	}
	return s
}

func (s *GitService) gitLabClientFor(baseURL string) gitLabClient {
	factory := s.gitlabFactory
	if factory == nil {
		factory = defaultGitLabClientFactory
	}
	return factory(baseURL)
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

// ListOrganizationIntegrations returns active git integrations owned by an organization.
func (s *GitService) ListOrganizationIntegrations(ctx context.Context, organizationID, actorID string) ([]model.GitIntegration, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	if !s.isOrgMember(ctx, organizationID, actorID) {
		return nil, fmt.Errorf("not a member of this organization")
	}
	return s.integrationRepo.ListByOrganization(ctx, strings.TrimSpace(organizationID))
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

func (s *GitService) GetOrganizationIntegrationDetail(ctx context.Context, organizationID, integrationID, actorID string) (*model.GitIntegrationDetail, error) {
	if strings.TrimSpace(organizationID) == "" || strings.TrimSpace(integrationID) == "" {
		return nil, fmt.Errorf("organization_id and integration_id are required")
	}
	if !s.isOrgMember(ctx, organizationID, actorID) {
		return nil, fmt.Errorf("not a member of this organization")
	}
	integration, err := s.integrationRepo.GetByIDForOrganization(ctx, strings.TrimSpace(organizationID), strings.TrimSpace(integrationID))
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
	return &model.GitIntegrationDetail{Integration: *integration, AffectedWorkspaces: affected}, nil
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
	return s.DeleteOrganizationIntegration(ctx, strings.TrimSpace(*workspace.OrganizationID), integrationID, actorID, workspaceID)
}

func (s *GitService) DeleteOrganizationIntegration(ctx context.Context, organizationID, integrationID, actorID, eventWorkspaceID string) error {
	if strings.TrimSpace(organizationID) == "" || strings.TrimSpace(integrationID) == "" {
		return fmt.Errorf("organization_id and integration_id are required")
	}
	if !s.isOrgAdminOrOwner(ctx, organizationID, actorID) {
		return fmt.Errorf("only organization owners or admins can uninstall git integrations")
	}
	existing, err := s.integrationRepo.GetByIDForOrganization(ctx, strings.TrimSpace(organizationID), strings.TrimSpace(integrationID))
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
	s.availableRepos.InvalidateIntegration(integrationID)
	publishWorkspaceID := strings.TrimSpace(eventWorkspaceID)
	if publishWorkspaceID == "" {
		publishWorkspaceID = workspaceIDForIntegration(existing)
	}
	if s.activitySvc != nil && actorID != "" {
		_ = s.activitySvc.Log(ctx, publishWorkspaceID, "git_integration", existing.ID, &actorID, "deleted", nil, nil, &existing.DisplayName, nil)
	}
	s.publishSimpleEvent("deleted", "git_integration", existing.ID, publishWorkspaceID, actorID)
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
	defaultCommitAuthorEmail, err := normalizeOptionalCommitAuthorEmail(req.DefaultCommitAuthorEmail)
	if err != nil {
		return nil, err
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
		WorkspaceID:              strPtr(req.WorkspaceID),
		OrganizationID:           workspace.OrganizationID,
		Provider:                 req.Provider,
		DisplayName:              strings.TrimSpace(req.DisplayName),
		CredentialMode:           credentialMode,
		AccountLogin:             trimPtr(req.AccountLogin),
		BaseURL:                  req.BaseURL,
		InstallationID:           trimPtr(req.InstallationID),
		AppID:                    trimPtr(req.AppID),
		WebhookSecret:            webhookSecret,
		AccessToken:              req.AccessToken,
		DefaultCommitAuthorName:  trimPtr(req.DefaultCommitAuthorName),
		DefaultCommitAuthorEmail: trimPtr(defaultCommitAuthorEmail),
		Active:                   true,
	}

	if err := s.integrationRepo.Create(ctx, integration); err != nil {
		return nil, err
	}

	if s.activitySvc != nil && actorID != "" {
		_ = s.activitySvc.Log(ctx, req.WorkspaceID, "git_integration", integration.ID, &actorID, "created", nil, nil, &integration.DisplayName, nil)
	}
	s.publishSimpleEvent("created", "git_integration", integration.ID, req.WorkspaceID, actorID)
	return integration, nil
}

func (s *GitService) CreateOrganizationIntegration(ctx context.Context, organizationID, returnWorkspaceID string, req model.CreateGitIntegrationRequest, actorID string) (*model.GitIntegration, error) {
	if strings.TrimSpace(organizationID) == "" || strings.TrimSpace(req.DisplayName) == "" {
		return nil, fmt.Errorf("organization_id and display_name are required")
	}
	if !s.isOrgAdminOrOwner(ctx, organizationID, actorID) {
		return nil, fmt.Errorf("only organization owners or admins can connect git integrations")
	}
	var workspaceID *string
	if strings.TrimSpace(returnWorkspaceID) != "" {
		workspace, err := s.workspaceRepo.GetByID(ctx, strings.TrimSpace(returnWorkspaceID))
		if err != nil {
			return nil, err
		}
		if workspace == nil || workspace.OrganizationID == nil || strings.TrimSpace(*workspace.OrganizationID) != strings.TrimSpace(organizationID) {
			return nil, fmt.Errorf("return workspace does not belong to this organization")
		}
		workspaceID = strPtr(strings.TrimSpace(returnWorkspaceID))
	}
	if req.Provider != "github" && req.Provider != "gitlab" {
		return nil, fmt.Errorf("provider must be 'github' or 'gitlab'")
	}
	defaultCommitAuthorEmail, err := normalizeOptionalCommitAuthorEmail(req.DefaultCommitAuthorEmail)
	if err != nil {
		return nil, err
	}
	credentialMode := "github_app"
	if req.CredentialMode != nil && strings.TrimSpace(*req.CredentialMode) != "" {
		credentialMode = strings.TrimSpace(*req.CredentialMode)
	}
	if credentialMode == "github_app" && (req.Provider != "github" || req.InstallationID == nil || strings.TrimSpace(*req.InstallationID) == "") {
		return nil, fmt.Errorf("github_app integrations require installation_id")
	}
	webhookSecret := req.WebhookSecret
	if webhookSecret == nil || strings.TrimSpace(*webhookSecret) == "" {
		secret := generateWebhookSecret()
		webhookSecret = &secret
	}
	integration := &model.GitIntegration{
		WorkspaceID:              workspaceID,
		OrganizationID:           strPtr(strings.TrimSpace(organizationID)),
		Provider:                 req.Provider,
		DisplayName:              strings.TrimSpace(req.DisplayName),
		CredentialMode:           credentialMode,
		AccountLogin:             trimPtr(req.AccountLogin),
		BaseURL:                  req.BaseURL,
		InstallationID:           trimPtr(req.InstallationID),
		AppID:                    trimPtr(req.AppID),
		WebhookSecret:            webhookSecret,
		AccessToken:              req.AccessToken,
		DefaultCommitAuthorName:  trimPtr(req.DefaultCommitAuthorName),
		DefaultCommitAuthorEmail: trimPtr(defaultCommitAuthorEmail),
		Active:                   true,
	}
	if err := s.integrationRepo.Create(ctx, integration); err != nil {
		return nil, err
	}
	s.publishSimpleEvent("created", "git_integration", integration.ID, workspaceIDForIntegration(integration), actorID)
	return integration, nil
}

func (s *GitService) UpdateOrganizationIntegration(ctx context.Context, organizationID, integrationID string, req model.UpdateGitIntegrationRequest, actorID string) (*model.GitIntegration, error) {
	organizationID = strings.TrimSpace(organizationID)
	integrationID = strings.TrimSpace(integrationID)
	if organizationID == "" || integrationID == "" {
		return nil, fmt.Errorf("organization_id and integration_id are required")
	}
	if !s.isOrgAdminOrOwner(ctx, organizationID, actorID) {
		return nil, fmt.Errorf("only organization owners or admins can update git integrations")
	}
	integration, err := s.integrationRepo.GetByIDForOrganization(ctx, organizationID, integrationID)
	if err != nil {
		return nil, fmt.Errorf("get integration: %w", err)
	}
	if integration == nil {
		return nil, fmt.Errorf("integration not found")
	}
	if req.DefaultCommitAuthorName != nil {
		integration.DefaultCommitAuthorName = trimPtr(req.DefaultCommitAuthorName)
	}
	if req.DefaultCommitAuthorEmail != nil {
		email, err := normalizeCommitAuthorEmail(*req.DefaultCommitAuthorEmail)
		if err != nil {
			return nil, err
		}
		integration.DefaultCommitAuthorEmail = trimPtr(&email)
	}
	if err := s.integrationRepo.Update(ctx, integration); err != nil {
		return nil, err
	}
	s.publishSimpleEvent("updated", "git_integration", integration.ID, workspaceIDForIntegration(integration), actorID)
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
	if !integration.Active {
		return nil, fmt.Errorf("git integration is inactive")
	}
	remoteByExternalID := map[string]model.GitAvailableRepo{}
	switch integration.Provider {
	case "github":
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
		for _, repo := range repos {
			externalID := githubapp.InstallationIDString(repo.ID)
			remoteByExternalID[externalID] = model.GitAvailableRepo{
				ExternalID:    externalID,
				FullName:      repo.FullName,
				DefaultBranch: repo.DefaultBranch,
				Permissions:   repo.Permissions,
				Private:       repo.Private,
				Archived:      repo.Archived,
			}
		}
	case "gitlab":
		repos, err := s.ListAvailableRepos(ctx, workspaceID, integrationID, actorID, ListAvailableReposOptions{NoCache: true})
		if err != nil {
			integration.LastSyncError = strPtr(err.Error())
			_ = s.integrationRepo.Update(ctx, integration)
			return nil, err
		}
		for _, repo := range repos {
			remoteByExternalID[repo.ExternalID] = repo
		}
	default:
		return nil, fmt.Errorf("repository sync is not implemented for %s", integration.Provider)
	}
	now := time.Now().UTC()
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

func (s *GitService) SyncOrganizationRepositories(ctx context.Context, organizationID, integrationID, actorID string) ([]model.GitRepository, error) {
	if strings.TrimSpace(organizationID) == "" || strings.TrimSpace(integrationID) == "" {
		return nil, fmt.Errorf("organization_id and integration_id are required")
	}
	if !s.isOrgAdminOrOwner(ctx, organizationID, actorID) {
		return nil, fmt.Errorf("only organization owners or admins can sync git integrations")
	}
	integration, err := s.integrationRepo.GetByIDForOrganization(ctx, strings.TrimSpace(organizationID), strings.TrimSpace(integrationID))
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, fmt.Errorf("git integration not found")
	}
	affected, err := s.repoRepo.ListAffectedWorkspacesByIntegration(ctx, integration.ID)
	if err != nil {
		return nil, err
	}
	all := make([]model.GitRepository, 0)
	for _, workspace := range affected {
		repos, err := s.SyncRepositories(ctx, workspace.WorkspaceID, integration.ID, actorID)
		if err != nil {
			return nil, err
		}
		all = append(all, repos...)
	}
	now := time.Now().UTC()
	integration.LastSyncedAt = &now
	integration.LastSyncError = nil
	if err := s.integrationRepo.Update(ctx, integration); err != nil {
		return nil, err
	}
	return all, nil
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

// GetEnabledRepositoryByID returns a repository that is available for PM delivery and agents.
func (s *GitService) GetEnabledRepositoryByID(ctx context.Context, workspaceID, repoID string) (*model.GitRepository, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(repoID) == "" {
		return nil, fmt.Errorf("repository_id is required")
	}
	return s.repoRepo.GetEnabledByID(ctx, workspaceID, repoID)
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
	case "gitlab":
		projectID, err := strconv.ParseInt(repo.ExternalID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("gitlab project id is invalid")
		}
		token, err := s.gitlabAccessToken(ctx, integration)
		if err != nil {
			return nil, err
		}
		branches, err := s.gitLabClientFor(derefString(integration.BaseURL)).ListBranches(ctx, token, projectID)
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
	return s.GetGitHubInstallURLForOrganization(ctx, strings.TrimSpace(*workspace.OrganizationID), workspaceID, actorID, forceInstall)
}

func (s *GitService) GetGitHubInstallURLForOrganization(ctx context.Context, organizationID, returnWorkspaceID, actorID string, forceInstall bool) (string, string, *string, error) {
	if strings.TrimSpace(organizationID) == "" {
		return "", "", nil, fmt.Errorf("organization_id is required")
	}
	if s.githubApp == nil || s.githubAppSlug == "" {
		return "", "", nil, fmt.Errorf("github app onboarding is not configured")
	}
	if !s.isOrgAdminOrOwner(ctx, organizationID, actorID) {
		return "", "", nil, fmt.Errorf("only organization owners or admins can connect git integrations")
	}
	if !forceInstall {
		integration, err := s.integrationRepo.GetActiveByOrganization(ctx, strings.TrimSpace(organizationID), "github")
		if err != nil {
			return "", "", nil, err
		}
		if integration != nil {
			manageURL := s.githubInstallationManageURL(integration)
			return manageURL, "pick_repos", &integration.ID, nil
		}
	}

	state, err := s.signGitHubInstallState(organizationID, returnWorkspaceID, actorID)
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

	integration, err := s.upsertGitHubIntegration(ctx, strings.TrimSpace(state.OrganizationID), workspace, installationID, installation, state.ActorID)
	if err != nil {
		return withGitHubInstallStatus(redirectURL, "error", err.Error(), nil), nil
	}

	params := map[string]string{
		"integration_id": integration.ID,
	}
	return withGitHubInstallStatus(redirectURL, "connected", fmt.Sprintf("GitHub App connected to %s.", defaultAccountLogin(integration.AccountLogin)), params), nil
}

// ConnectGitLabWithToken stores an org-scoped GitLab access token (PAT, Group, or
// Project Access Token), verifies it by calling /user, and upserts the integration.
// Returns the resulting integration. There is no OAuth flow.
func (s *GitService) ConnectGitLabWithToken(
	ctx context.Context,
	organizationID, returnWorkspaceID, actorID string,
	req model.GitLabConnectTokenRequest,
) (*model.GitIntegration, error) {
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	if s.credentialRepo == nil || len(s.encryptionKey) != 32 {
		return nil, fmt.Errorf("gitlab credential storage is not configured")
	}
	if !s.isOrgAdminOrOwner(ctx, organizationID, actorID) {
		return nil, fmt.Errorf("only organization owners or admins can connect git integrations")
	}

	baseURL, err := validateGitLabBaseURL(req.BaseURL)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}
	authType, err := normalizeGitLabTokenAuthType(req.AuthType)
	if err != nil {
		return nil, err
	}

	workspace, _, err := s.resolveReturnWorkspace(ctx, organizationID, returnWorkspaceID)
	if err != nil {
		return nil, err
	}

	client := s.gitLabClientFor(baseURL)
	user, err := client.CurrentUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("verify gitlab token: %w", err)
	}
	if user == nil || user.ID == 0 {
		return nil, fmt.Errorf("gitlab token did not return a user")
	}

	credential, err := s.upsertGitLabTokenCredential(ctx, organizationID, actorID, baseURL, authType, token, req.Label, user)
	if err != nil {
		return nil, err
	}
	integration, err := s.upsertGitLabIntegration(ctx, organizationID, workspace, credential, actorID)
	if err != nil {
		return nil, err
	}
	if req.DefaultCommitAuthorName != nil || req.DefaultCommitAuthorEmail != nil {
		integration, err = s.UpdateOrganizationIntegration(ctx, organizationID, integration.ID, model.UpdateGitIntegrationRequest{
			DefaultCommitAuthorName:  req.DefaultCommitAuthorName,
			DefaultCommitAuthorEmail: req.DefaultCommitAuthorEmail,
		}, actorID)
		if err != nil {
			return nil, err
		}
	}
	return integration, nil
}

func validateGitLabBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "https://gitlab.com", nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("base_url must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("base_url must include a host")
	}
	// Strip path/query/fragment — GitLab API is mounted at /api/v4 from root.
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func normalizeGitLabTokenAuthType(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "personal_token":
		return "personal_token", nil
	case "group_token":
		return "group_token", nil
	case "project_token":
		return "project_token", nil
	default:
		return "", fmt.Errorf("auth_type must be personal_token, group_token, or project_token")
	}
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

// ListAvailableReposOptions tunes a single ListAvailableRepos call.
type ListAvailableReposOptions struct {
	Search  string // optional GitLab search term
	NoCache bool   // bypass the in-memory cache (manual Refresh)
}

func (s *GitService) ListAvailableRepos(ctx context.Context, workspaceID, integrationID, actorID string, opts ListAvailableReposOptions) ([]model.GitAvailableRepo, error) {
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
	search := strings.TrimSpace(opts.Search)
	if integration.Provider == "gitlab" {
		if opts.NoCache {
			s.availableRepos.InvalidateIntegration(integration.ID)
		} else if cached, ok := s.availableRepos.Get(integration.ID, search); ok {
			return cached, nil
		}
		token, err := s.gitlabAccessToken(ctx, integration)
		if err != nil {
			return nil, err
		}
		projects, err := s.gitLabClientFor(derefString(integration.BaseURL)).ListProjects(ctx, token, search)
		if err != nil {
			return nil, err
		}
		items := make([]model.GitAvailableRepo, 0, len(projects))
		for _, project := range projects {
			externalID := strconv.FormatInt(project.ID, 10)
			claimedBy, err := s.repoRepo.GetWorkspaceClaimByExternalID(ctx, workspaceID, integration.ID, externalID)
			if err != nil {
				return nil, err
			}
			items = append(items, model.GitAvailableRepo{
				ExternalID:    externalID,
				FullName:      project.PathWithNamespace,
				DefaultBranch: defaultBranch(project.DefaultBranch),
				Private:       project.Visibility != "public",
				Archived:      project.Archived,
				Permissions:   gitlabProjectPermissions(project),
				ClaimedBy:     claimedBy,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			return strings.ToLower(items[i].FullName) < strings.ToLower(items[j].FullName)
		})
		s.availableRepos.Set(integration.ID, search, items)
		return items, nil
	}
	if integration.Provider != "github" {
		return nil, fmt.Errorf("available repos are not supported for %s", integration.Provider)
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
		claimedBy, err := s.repoRepo.GetWorkspaceClaimByExternalID(ctx, workspaceID, integration.ID, externalID)
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
	availableByID := map[string]model.GitAvailableRepo{}
	switch integration.Provider {
	case "github":
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
		for _, repo := range available {
			externalID := githubapp.InstallationIDString(repo.ID)
			availableByID[externalID] = model.GitAvailableRepo{
				ExternalID:    externalID,
				FullName:      repo.FullName,
				DefaultBranch: defaultBranch(repo.DefaultBranch),
				Permissions:   repo.Permissions,
				Private:       repo.Private,
				Archived:      repo.Archived,
			}
		}
	case "gitlab":
		token, err := s.gitlabAccessToken(ctx, integration)
		if err != nil {
			return nil, nil, err
		}
		client := s.gitLabClientFor(derefString(integration.BaseURL))
		for _, repoID := range req.RepoIDs {
			repoID = strings.TrimSpace(repoID)
			if repoID == "" {
				continue
			}
			if _, ok := availableByID[repoID]; ok {
				continue
			}
			projectID, parseErr := strconv.ParseInt(repoID, 10, 64)
			if parseErr != nil {
				return nil, nil, fmt.Errorf("gitlab project id %q is invalid", repoID)
			}
			project, fetchErr := client.GetProject(ctx, token, projectID)
			if fetchErr != nil {
				return nil, nil, fmt.Errorf("gitlab project %s is not accessible: %w", repoID, fetchErr)
			}
			availableByID[repoID] = model.GitAvailableRepo{
				ExternalID:    repoID,
				FullName:      project.PathWithNamespace,
				DefaultBranch: defaultBranch(project.DefaultBranch),
				Private:       project.Visibility != "public",
				Archived:      project.Archived,
				Permissions:   gitlabProjectPermissions(*project),
			}
		}
	default:
		return nil, nil, fmt.Errorf("repo wiring is not supported for %s", integration.Provider)
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

	for _, repoID := range requestedRepoIDs {
		repoID = strings.TrimSpace(repoID)
		if _, ok := availableByID[repoID]; !ok {
			return nil, nil, fmt.Errorf("repository %s is not available to this installation", repoID)
		}
	}

	upserted := make([]model.GitRepository, 0, len(requestedRepoIDs))
	for _, repoID := range requestedRepoIDs {
		remote := availableByID[repoID]
		permissions, _ := json.Marshal(remote.Permissions)
		repo := model.GitRepository{
			WorkspaceID:   currentWorkspaceID,
			IntegrationID: integration.ID,
			Provider:      integration.Provider,
			BaseURL:       integration.BaseURL,
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
			return nil, nil, err
		}
		if integration.Provider == "gitlab" {
			s.tryInstallGitLabWebhook(ctx, integration, repoID, remote.Permissions)
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

func (s *GitService) ResolveWebhookRepositories(ctx context.Context, integrationID, externalID string) ([]model.GitRepository, error) {
	if strings.TrimSpace(integrationID) == "" || strings.TrimSpace(externalID) == "" {
		return nil, fmt.Errorf("integration_id and external_id are required")
	}
	return s.repoRepo.ListActiveByExternalID(ctx, strings.TrimSpace(integrationID), strings.TrimSpace(externalID))
}

func (s *GitService) ResolveGitLabWebhookRepository(ctx context.Context, externalID string, token string) (*model.GitIntegration, *model.GitRepository, error) {
	integration, repos, err := s.ResolveGitLabWebhookRepositories(ctx, externalID, token)
	if err != nil {
		return nil, nil, err
	}
	if len(repos) == 0 {
		return nil, nil, nil
	}
	return integration, &repos[0], nil
}

func (s *GitService) ResolveGitLabWebhookRepositories(ctx context.Context, externalID string, token string) (*model.GitIntegration, []model.GitRepository, error) {
	if strings.TrimSpace(externalID) == "" {
		return nil, nil, fmt.Errorf("project id is required")
	}
	repos, err := s.repoRepo.ListActiveByProviderExternalID(ctx, "gitlab", strings.TrimSpace(externalID))
	if err != nil {
		return nil, nil, err
	}
	if len(repos) == 0 {
		return nil, nil, nil
	}
	integrationIDs := make([]string, 0)
	seenIntegrationIDs := make(map[string]bool)
	for _, repo := range repos {
		if !seenIntegrationIDs[repo.IntegrationID] {
			seenIntegrationIDs[repo.IntegrationID] = true
			integrationIDs = append(integrationIDs, repo.IntegrationID)
		}
	}
	sort.Strings(integrationIDs)

	reposByIntegration := make(map[string][]model.GitRepository, len(integrationIDs))
	for _, repo := range repos {
		reposByIntegration[repo.IntegrationID] = append(reposByIntegration[repo.IntegrationID], repo)
	}

	var firstIntegration *model.GitIntegration
	matchedRepos := make([]model.GitRepository, 0, len(repos))
	sawTokenProtectedCandidate := false
	for _, integrationID := range integrationIDs {
		integration, err := s.integrationRepo.GetByIDAny(ctx, integrationID)
		if err != nil {
			return nil, nil, err
		}
		if integration == nil || !integration.Active {
			continue
		}
		if integration.WebhookSecret != nil && strings.TrimSpace(*integration.WebhookSecret) != "" {
			sawTokenProtectedCandidate = true
			if strings.TrimSpace(token) != strings.TrimSpace(*integration.WebhookSecret) {
				continue
			}
		}
		if firstIntegration == nil {
			firstIntegration = integration
		}
		matchedRepos = append(matchedRepos, reposByIntegration[integration.ID]...)
	}
	if len(matchedRepos) == 0 {
		if sawTokenProtectedCandidate {
			return nil, nil, fmt.Errorf("invalid gitlab webhook token")
		}
		return nil, nil, nil
	}
	return firstIntegration, matchedRepos, nil
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
				repos, err := s.repoRepo.ListActiveByExternalID(ctx, integration.ID, externalID)
				if err != nil {
					return err
				}
				for _, repo := range repos {
					if err := s.repoRepo.SoftDeleteByID(ctx, repo.ID); err != nil {
						return err
					}
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
		TargetSource:  model.TaskDeliveryTargetSourceManual,
	}

	if story.TeamID != nil && *story.TeamID != "" {
		teamDefault, err := s.settingsRepo.GetTeamRepoDefault(ctx, *story.TeamID)
		if err != nil {
			return nil, err
		}
		if teamDefault != nil {
			repo, err := s.repoRepo.GetEnabledByID(ctx, workspaceID, teamDefault.RepositoryID)
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
				target.TargetSource = model.TaskDeliveryTargetSourceTeamDefault
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
	if req.ClearTarget {
		target.RepositoryID = nil
		target.RepoFullName = nil
		target.IntegrationID = nil
		target.BaseBranch = nil
		target.WorkingBranch = nil
		target.DeliveryState = "unconfigured"
		target.TargetSource = model.TaskDeliveryTargetSourceManual
		target.SourceEpicID = nil
		if err := s.deliveryRepo.Save(ctx, target); err != nil {
			return nil, err
		}
		s.publishTaskDeliveryTargetUpdated(ctx, workspaceID, storyID, actorID, target)
		return target, nil
	}

	if req.RepositoryID != nil && *req.RepositoryID != "" {
		repo, err := s.repoRepo.GetEnabledByID(ctx, workspaceID, *req.RepositoryID)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository is not available for PM delivery")
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
	if target.RepositoryID != nil && strings.TrimSpace(*target.RepositoryID) != "" {
		repo, err := s.repoRepo.GetEnabledByID(ctx, workspaceID, *target.RepositoryID)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository is not available for PM delivery")
		}
		target.RepositoryID = &repo.ID
		target.RepoFullName = &repo.FullName
		target.IntegrationID = &repo.IntegrationID
	}
	if target.RepositoryID != nil && target.BaseBranch != nil {
		target.DeliveryState = "ready"
	}
	target.TargetSource = model.TaskDeliveryTargetSourceManual
	target.SourceEpicID = nil

	if err := s.deliveryRepo.Save(ctx, target); err != nil {
		return nil, err
	}

	s.publishTaskDeliveryTargetUpdated(ctx, workspaceID, storyID, actorID, target)
	return target, nil
}

func (s *GitService) publishTaskDeliveryTargetUpdated(ctx context.Context, workspaceID, taskID, actorID string, target *model.TaskDeliveryTarget) {
	if s.activitySvc != nil && actorID != "" {
		_ = s.activitySvc.Log(ctx, workspaceID, "task", taskID, &actorID, "updated", strPtr("delivery_target"), nil, target.RepoFullName, nil)
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task_delivery_target", EntityID: target.ID, WorkspaceID: workspaceID, ParentType: "task", ParentID: taskID, ActorID: actorID})
	}
}

// GetEpicDeliveryTarget resolves or creates the current integration branch for an epic.
func (s *GitService) GetEpicDeliveryTarget(ctx context.Context, workspaceID, epicID string) (*model.EpicDeliveryTarget, error) {
	if s.epicDeliveryRepo == nil || s.epicRepo == nil {
		return nil, fmt.Errorf("epic delivery target is not configured")
	}
	target, err := s.epicDeliveryRepo.GetByEpic(ctx, workspaceID, epicID)
	if err != nil {
		return nil, err
	}
	if target != nil {
		return target, nil
	}

	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, err
	}
	if epicWithStats == nil || epicWithStats.Epic.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if workspace == nil {
		return nil, fmt.Errorf("workspace not found")
	}

	target = &model.EpicDeliveryTarget{
		WorkspaceID:   workspaceID,
		EpicID:        epicID,
		DeliveryState: "unconfigured",
	}
	if err := s.applyDefaultEpicDeliveryTarget(ctx, target, epic, workspace.WorkspaceKey); err != nil {
		return nil, err
	}
	if err := s.epicDeliveryRepo.Save(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}

// UpdateEpicDeliveryTarget updates the selected delivery target for an epic.
func (s *GitService) UpdateEpicDeliveryTarget(ctx context.Context, workspaceID, epicID string, req model.UpdateEpicDeliveryTargetRequest, actorID string) (*model.EpicDeliveryTarget, error) {
	target, err := s.GetEpicDeliveryTarget(ctx, workspaceID, epicID)
	if err != nil {
		return nil, err
	}
	if req.ClearTarget {
		target.RepositoryID = nil
		target.RepoFullName = nil
		target.IntegrationID = nil
		target.BaseBranch = nil
		target.EpicBranch = nil
		target.DeliveryState = "unconfigured"
		if err := s.epicDeliveryRepo.Save(ctx, target); err != nil {
			return nil, err
		}
		s.publishEpicDeliveryTargetUpdated(workspaceID, epicID, actorID, target)
		return target, nil
	}
	if req.RepositoryID != nil && strings.TrimSpace(*req.RepositoryID) != "" {
		repo, err := s.repoRepo.GetEnabledByID(ctx, workspaceID, strings.TrimSpace(*req.RepositoryID))
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository is not available for PM delivery")
		}
		target.RepositoryID = &repo.ID
		target.RepoFullName = &repo.FullName
		target.IntegrationID = &repo.IntegrationID
		if req.BaseBranch == nil || strings.TrimSpace(*req.BaseBranch) == "" {
			base := defaultBranch(repo.DefaultBranch)
			req.BaseBranch = &base
		}
	}
	if req.BaseBranch != nil && strings.TrimSpace(*req.BaseBranch) != "" {
		baseBranch := strings.TrimSpace(*req.BaseBranch)
		target.BaseBranch = &baseBranch
	}
	if req.EpicBranch != nil && strings.TrimSpace(*req.EpicBranch) != "" {
		epicBranch := strings.TrimSpace(*req.EpicBranch)
		target.EpicBranch = &epicBranch
	}
	if target.RepositoryID != nil && strings.TrimSpace(*target.RepositoryID) != "" {
		repo, err := s.repoRepo.GetEnabledByID(ctx, workspaceID, *target.RepositoryID)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository is not available for PM delivery")
		}
		target.RepositoryID = &repo.ID
		target.RepoFullName = &repo.FullName
		target.IntegrationID = &repo.IntegrationID
	}
	if target.RepositoryID != nil && target.BaseBranch != nil && target.EpicBranch != nil {
		target.DeliveryState = "ready"
	}
	if err := s.epicDeliveryRepo.Save(ctx, target); err != nil {
		return nil, err
	}
	if s.activitySvc != nil && actorID != "" {
		_ = s.activitySvc.Log(ctx, workspaceID, "epic", epicID, &actorID, "updated", strPtr("delivery_target"), nil, target.RepoFullName, nil)
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "epic_delivery_target",
			EntityID:    target.ID,
			WorkspaceID: workspaceID,
			ParentType:  "epic",
			ParentID:    epicID,
			ActorID:     actorID,
		})
	}
	return target, nil
}

func (s *GitService) applyDefaultEpicDeliveryTarget(ctx context.Context, target *model.EpicDeliveryTarget, epic *model.PMEpic, workspaceKey string) error {
	var teamDefault *model.PMTeamRepoDefault
	if epic.TeamID != nil && strings.TrimSpace(*epic.TeamID) != "" {
		var err error
		teamDefault, err = s.settingsRepo.GetTeamRepoDefault(ctx, *epic.TeamID)
		if err != nil {
			return err
		}
	}

	var repo *model.GitRepository
	if epic.PlanningRepositoryID != nil && strings.TrimSpace(*epic.PlanningRepositoryID) != "" {
		var err error
		repo, err = s.repoRepo.GetEnabledByID(ctx, target.WorkspaceID, strings.TrimSpace(*epic.PlanningRepositoryID))
		if err != nil {
			return err
		}
		if repo == nil {
			return fmt.Errorf("planning repository is not available for PM delivery")
		}
	} else if teamDefault != nil && strings.TrimSpace(teamDefault.RepositoryID) != "" {
		var err error
		repo, err = s.repoRepo.GetEnabledByID(ctx, target.WorkspaceID, teamDefault.RepositoryID)
		if err != nil {
			return err
		}
	}
	if repo == nil {
		return nil
	}

	baseBranch := defaultBranch(repo.DefaultBranch)
	if teamDefault != nil && teamDefault.RepositoryID == repo.ID && strings.TrimSpace(teamDefault.BaseBranch) != "" {
		baseBranch = strings.TrimSpace(teamDefault.BaseBranch)
	}
	epicBranch := model.BuildEpicWorkingBranch(epic, workspaceKey)
	target.RepositoryID = &repo.ID
	target.RepoFullName = &repo.FullName
	target.IntegrationID = &repo.IntegrationID
	target.BaseBranch = &baseBranch
	target.EpicBranch = &epicBranch
	target.DeliveryState = "ready"
	return nil
}

func (s *GitService) EnsureEpicBranch(ctx context.Context, workspaceID, epicID, actorID, runID string) (*model.EpicDeliveryTarget, error) {
	target, err := s.GetEpicDeliveryTarget(ctx, workspaceID, epicID)
	if err != nil {
		return nil, err
	}
	if target.RepositoryID == nil || target.RepoFullName == nil || target.IntegrationID == nil || target.BaseBranch == nil || target.EpicBranch == nil {
		return nil, ErrEpicDeliveryTargetRequired
	}
	if s.githubApp == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	integration, owner, repo, err := s.githubIntegrationAndRepoParts(ctx, workspaceID, *target.IntegrationID, *target.RepoFullName)
	if err != nil {
		return nil, err
	}
	if integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
		return nil, fmt.Errorf("git integration has no installation ID")
	}
	if err := s.githubApp.EnsureBranch(ctx, *integration.InstallationID, owner, repo, strings.TrimSpace(*target.EpicBranch), strings.TrimSpace(*target.BaseBranch)); err != nil {
		return nil, err
	}
	now := time.Now()
	target.DeliveryState = "in_progress"
	target.LastSyncedAt = &now
	if strings.TrimSpace(runID) != "" {
		target.LastRunID = &runID
	}
	if err := s.epicDeliveryRepo.Save(ctx, target); err != nil {
		return nil, err
	}
	s.publishEpicDeliveryTargetUpdated(workspaceID, epicID, actorID, target)
	return target, nil
}

func (s *GitService) PrepareTaskForEpicBranch(ctx context.Context, workspaceID, taskID, epicID, actorID string) (*model.TaskDeliveryTarget, error) {
	target, _, err := s.SyncTaskDeliveryTargetToEpic(ctx, workspaceID, taskID, epicID, actorID, true)
	return target, err
}

func (s *GitService) SyncTaskDeliveryTargetToEpic(ctx context.Context, workspaceID, taskID, epicID, actorID string, force bool) (*model.TaskDeliveryTarget, bool, error) {
	epicTarget, err := s.GetEpicDeliveryTarget(ctx, workspaceID, epicID)
	if err != nil {
		return nil, false, err
	}
	if epicTarget.RepositoryID == nil || epicTarget.EpicBranch == nil {
		return nil, false, ErrEpicDeliveryTargetRequired
	}
	taskTarget, err := s.GetTaskDeliveryTarget(ctx, workspaceID, taskID)
	if err != nil {
		return nil, false, err
	}
	source := model.NormalizeTaskDeliveryTargetSource(taskTarget.TargetSource)
	hasConfiguredTarget := taskTarget.RepositoryID != nil && taskTarget.BaseBranch != nil && strings.TrimSpace(*taskTarget.BaseBranch) != ""
	if !force && source == model.TaskDeliveryTargetSourceManual && hasConfiguredTarget && taskTarget.DeliveryState != "unconfigured" {
		return taskTarget, false, nil
	}
	taskTarget.RepositoryID = epicTarget.RepositoryID
	taskTarget.RepoFullName = epicTarget.RepoFullName
	taskTarget.IntegrationID = epicTarget.IntegrationID
	epicBranch := strings.TrimSpace(*epicTarget.EpicBranch)
	taskTarget.BaseBranch = &epicBranch
	taskTarget.TargetSource = model.TaskDeliveryTargetSourceEpic
	taskTarget.SourceEpicID = &epicID
	if taskTarget.WorkingBranch == nil || strings.TrimSpace(*taskTarget.WorkingBranch) == "" {
		_, working, err := s.ResolveTaskRunBranchValues(ctx, workspaceID, taskID)
		if err != nil {
			return nil, false, err
		}
		taskTarget.WorkingBranch = &working
	}
	taskTarget.DeliveryState = "ready"
	if err := s.deliveryRepo.Save(ctx, taskTarget); err != nil {
		return nil, false, err
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "task_delivery_target",
			EntityID:    taskTarget.ID,
			WorkspaceID: workspaceID,
			ParentType:  "task",
			ParentID:    taskID,
			ActorID:     actorID,
		})
	}
	return taskTarget, true, nil
}

func (s *GitService) UseTaskEpicDeliveryTarget(ctx context.Context, workspaceID, taskID, actorID string) (*model.TaskDeliveryTarget, error) {
	task, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil || task.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("task not found")
	}
	if task.EpicID == nil || strings.TrimSpace(*task.EpicID) == "" {
		return nil, fmt.Errorf("task has no epic")
	}
	target, _, err := s.SyncTaskDeliveryTargetToEpic(ctx, workspaceID, taskID, strings.TrimSpace(*task.EpicID), actorID, true)
	return target, err
}

func (s *GitService) MergeTaskBranchIntoEpic(ctx context.Context, workspaceID, taskID, epicID, runID string) (*model.EpicDeliveryTarget, error) {
	epicTarget, err := s.GetEpicDeliveryTarget(ctx, workspaceID, epicID)
	if err != nil {
		return nil, err
	}
	if epicTarget.EpicBranch == nil || strings.TrimSpace(*epicTarget.EpicBranch) == "" {
		return nil, ErrEpicDeliveryTargetRequired
	}
	if err := s.MergeBranch(ctx, workspaceID, taskID, strings.TrimSpace(*epicTarget.EpicBranch)); err != nil {
		return nil, err
	}
	if err := s.UpdateDeliveryStatusAfterMerge(ctx, workspaceID, taskID, "merged"); err != nil {
		return nil, err
	}
	now := time.Now()
	epicTarget.DeliveryState = "integrating"
	epicTarget.LastSyncedAt = &now
	if strings.TrimSpace(runID) != "" {
		epicTarget.LastRunID = &runID
	}
	if err := s.epicDeliveryRepo.Save(ctx, epicTarget); err != nil {
		return nil, err
	}
	return epicTarget, nil
}

func (s *GitService) OpenEpicFinalPullRequest(ctx context.Context, workspaceID, epicID, runID string) (*model.EpicDeliveryTarget, error) {
	target, err := s.GetEpicDeliveryTarget(ctx, workspaceID, epicID)
	if err != nil {
		return nil, err
	}
	if target.IntegrationID == nil || target.RepoFullName == nil || target.BaseBranch == nil || target.EpicBranch == nil {
		return nil, ErrEpicDeliveryTargetRequired
	}
	if s.githubApp == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	integration, owner, repo, err := s.githubIntegrationAndRepoParts(ctx, workspaceID, *target.IntegrationID, *target.RepoFullName)
	if err != nil {
		return nil, err
	}
	if integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
		return nil, fmt.Errorf("git integration has no installation ID")
	}
	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, err
	}
	title := fmt.Sprintf("Merge epic %s", strings.TrimSpace(*target.EpicBranch))
	if epicWithStats != nil && strings.TrimSpace(epicWithStats.Epic.Name) != "" {
		title = "Merge epic: " + strings.TrimSpace(epicWithStats.Epic.Name)
	}
	pr, err := s.githubApp.EnsurePullRequest(ctx, *integration.InstallationID, owner, repo, githubapp.EnsurePullRequestInput{
		Head:  strings.TrimSpace(*target.EpicBranch),
		Base:  strings.TrimSpace(*target.BaseBranch),
		Title: title,
		Body:  "Created by Helpin after all epic task branches were merged into the epic integration branch.",
	})
	if err != nil {
		return nil, err
	}
	if pr != nil {
		target.FinalPRNumber = &pr.Number
		target.FinalPRTitle = &pr.Title
		target.FinalPRURL = &pr.HTMLURL
		status := statusForGitHubPullRequest(pr)
		target.FinalPRStatus = &status
	}
	now := time.Now()
	target.DeliveryState = "pr_open"
	target.LastSyncedAt = &now
	if strings.TrimSpace(runID) != "" {
		target.LastRunID = &runID
	}
	if err := s.epicDeliveryRepo.Save(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}

func (s *GitService) githubIntegrationAndRepoParts(ctx context.Context, workspaceID, integrationID, repoFullName string) (*model.GitIntegration, string, string, error) {
	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, strings.TrimSpace(integrationID))
	if err != nil {
		return nil, "", "", fmt.Errorf("load git integration: %w", err)
	}
	if integration == nil || strings.TrimSpace(integration.Provider) != "github" {
		return nil, "", "", fmt.Errorf("epic branch orchestration currently requires a GitHub integration")
	}
	owner, repo, ok := splitRepoFullName(strings.TrimSpace(repoFullName))
	if !ok {
		return nil, "", "", fmt.Errorf("invalid repo full name: %s", repoFullName)
	}
	return integration, owner, repo, nil
}

func (s *GitService) publishEpicDeliveryTargetUpdated(workspaceID, epicID, actorID string, target *model.EpicDeliveryTarget) {
	if s == nil || s.wsPublisher == nil || target == nil {
		return
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "epic_delivery_target",
		EntityID:    target.ID,
		WorkspaceID: workspaceID,
		ParentType:  "epic",
		ParentID:    epicID,
		ActorID:     actorID,
	})
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
	if target.RepositoryID != nil && strings.TrimSpace(*target.RepositoryID) != "" {
		repo, err := s.repoRepo.GetEnabledByID(ctx, workspaceID, *target.RepositoryID)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository is not available for PM delivery")
		}
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

func (s *GitService) ResolveAgentRuntimeRepositorySpec(ctx context.Context, workspaceID string, target agentruntime.TargetRef, runID string) (*agentruntime.RepositoryWorkspaceSpec, error) {
	if s == nil {
		return nil, fmt.Errorf("git service is not configured")
	}
	target.Type = strings.TrimSpace(target.Type)
	target.ID = strings.TrimSpace(target.ID)
	if target.Type == "" || target.ID == "" {
		return nil, fmt.Errorf("target.type and target.id are required")
	}

	var (
		repo          *model.GitRepository
		baseBranch    string
		workingBranch string
		deliveryMeta  map[string]interface{}
		err           error
	)

	switch target.Type {
	case "repository":
		if strings.TrimSpace(workspaceID) == "" {
			return nil, fmt.Errorf("workspace_id metadata is required for repository targets")
		}
		repoFullName := strings.TrimSpace(firstStringValue(target.Metadata, "repo_full_name"))
		if strings.TrimSpace(target.ID) != "" {
			repo, err = s.repoRepo.GetEnabledByID(ctx, workspaceID, target.ID)
			if err != nil {
				return nil, err
			}
		}
		if repo == nil && repoFullName == "" && strings.Contains(strings.TrimSpace(target.ID), "/") {
			repoFullName = strings.TrimSpace(target.ID)
		}
		if repo == nil && repoFullName != "" {
			repo, err = s.repoRepo.GetByFullName(ctx, workspaceID, repoFullName)
			if err != nil {
				return nil, err
			}
			if repo != nil && (repo.Archived || !repo.Selected || !repo.Active || repo.DeletedAt != nil) {
				repo = nil
			}
		}
		if repo == nil {
			return nil, fmt.Errorf("repository is not available")
		}
		baseBranch = defaultBranch(repo.DefaultBranch)
		if requestedBase := strings.TrimSpace(firstStringValue(target.Metadata, "base_branch")); requestedBase != "" {
			baseBranch = requestedBase
		}
		workingBranch = strings.TrimSpace(firstStringValue(target.Metadata, "work_branch", "working_branch"))
	case "task", "story":
		if strings.TrimSpace(workspaceID) == "" {
			return nil, fmt.Errorf("workspace_id metadata is required for task repository targets")
		}
		deliveryTarget, err := s.ResolveTaskDeliveryTargetForRun(ctx, workspaceID, target.ID, true)
		if err != nil {
			return nil, err
		}
		if deliveryTarget == nil || deliveryTarget.RepositoryID == nil {
			return nil, ErrTaskDeliveryTargetRequired
		}
		repo, err = s.repoRepo.GetEnabledByID(ctx, workspaceID, strings.TrimSpace(*deliveryTarget.RepositoryID))
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository is not available")
		}
		baseBranch, workingBranch, err = s.ResolveTaskRunBranchValues(ctx, workspaceID, target.ID)
		if err != nil {
			return nil, err
		}
		deliveryMeta = map[string]interface{}{
			"delivery_target_id": deliveryTarget.ID,
			"delivery_state":     deliveryTarget.DeliveryState,
			"target_source":      deliveryTarget.TargetSource,
		}
		if deliveryTarget.ActivePRNumber != nil {
			deliveryMeta["active_pr_number"] = *deliveryTarget.ActivePRNumber
		}
	case "epic":
		if strings.TrimSpace(workspaceID) == "" {
			return nil, fmt.Errorf("workspace_id metadata is required for epic repository targets")
		}
		deliveryTarget, err := s.GetEpicDeliveryTarget(ctx, workspaceID, target.ID)
		if err != nil {
			return nil, err
		}
		if deliveryTarget == nil || deliveryTarget.RepositoryID == nil || deliveryTarget.BaseBranch == nil || deliveryTarget.EpicBranch == nil {
			return nil, ErrEpicDeliveryTargetRequired
		}
		repo, err = s.repoRepo.GetEnabledByID(ctx, workspaceID, strings.TrimSpace(*deliveryTarget.RepositoryID))
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository is not available")
		}
		baseBranch = strings.TrimSpace(*deliveryTarget.BaseBranch)
		workingBranch = strings.TrimSpace(*deliveryTarget.EpicBranch)
		deliveryMeta = map[string]interface{}{
			"delivery_target_id": deliveryTarget.ID,
			"delivery_state":     deliveryTarget.DeliveryState,
		}
		if deliveryTarget.FinalPRNumber != nil {
			deliveryMeta["final_pr_number"] = *deliveryTarget.FinalPRNumber
		}
	default:
		return nil, fmt.Errorf("target type %q does not resolve to a repository workspace", target.Type)
	}

	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = defaultBranch(repo.DefaultBranch)
	}
	if strings.TrimSpace(workingBranch) == "" {
		workingBranch = strings.TrimSpace(firstStringValue(target.Metadata, "work_branch", "working_branch"))
	}
	if strings.TrimSpace(workingBranch) == "" {
		workingBranch = fmt.Sprintf("agent-runtime/%s", strings.TrimSpace(runID))
	}

	integration, err := s.integrationRepo.GetByID(ctx, repo.WorkspaceID, repo.IntegrationID)
	if err != nil {
		return nil, err
	}
	if integration == nil || !integration.Active {
		return nil, fmt.Errorf("git integration is not available")
	}
	auth, err := s.agentRuntimeRepositoryAuth(ctx, integration)
	if err != nil {
		return nil, err
	}

	meta := map[string]interface{}{
		"workspace_id":   repo.WorkspaceID,
		"repository_id":  repo.ID,
		"integration_id": repo.IntegrationID,
		"repo_full_name": repo.FullName,
		"target_type":    target.Type,
		"target_id":      target.ID,
	}
	for key, value := range deliveryMeta {
		meta[key] = value
	}

	return &agentruntime.RepositoryWorkspaceSpec{
		Provider:       strings.TrimSpace(repo.Provider),
		CloneURL:       agentRuntimeCloneURL(repo, integration),
		Auth:           auth,
		BaseBranch:     strings.TrimSpace(baseBranch),
		WorkBranch:     strings.TrimSpace(workingBranch),
		FinalizePolicy: agentruntime.RepositoryFinalizePushBranch,
		CommitIdentity: &agentruntime.GitIdentity{
			Name:  strings.TrimSpace(agentRuntimeHostFirstNonEmpty(agentRuntimeStringPtr(integration.DefaultCommitAuthorName), "Helpin Agent")),
			Email: strings.TrimSpace(agentRuntimeHostFirstNonEmpty(agentRuntimeStringPtr(integration.DefaultCommitAuthorEmail), "agents@helpin.ai")),
		},
		Metadata: meta,
	}, nil
}

func (s *GitService) agentRuntimeRepositoryAuth(ctx context.Context, integration *model.GitIntegration) (*agentruntime.RepositoryAuth, error) {
	if integration == nil {
		return nil, fmt.Errorf("git integration is required")
	}
	switch strings.TrimSpace(strings.ToLower(integration.Provider)) {
	case "github":
		token := strings.TrimSpace(integration.AccessToken)
		if strings.EqualFold(strings.TrimSpace(integration.CredentialMode), "github_app") {
			if integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
				return nil, fmt.Errorf("github installation id is required")
			}
			if s.githubApp == nil {
				return nil, fmt.Errorf("github app client is not configured")
			}
			minted, err := s.githubApp.MintInstallationToken(ctx, strings.TrimSpace(*integration.InstallationID))
			if err != nil {
				return nil, err
			}
			token = minted
		}
		if token == "" {
			return nil, fmt.Errorf("github repository token is unavailable")
		}
		return &agentruntime.RepositoryAuth{Type: "github", Token: token}, nil
	case "gitlab":
		token, err := s.gitlabAccessToken(ctx, integration)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(token) == "" {
			return nil, fmt.Errorf("gitlab repository token is unavailable")
		}
		return &agentruntime.RepositoryAuth{Type: "gitlab", Token: token}, nil
	default:
		return nil, fmt.Errorf("unsupported git provider %q", integration.Provider)
	}
}

func agentRuntimeCloneURL(repo *model.GitRepository, integration *model.GitIntegration) string {
	if repo == nil {
		return ""
	}
	fullName := strings.Trim(strings.TrimSpace(repo.FullName), "/")
	switch strings.TrimSpace(strings.ToLower(repo.Provider)) {
	case "gitlab":
		base := model.ResolveGitLabWebBaseURL(repo.BaseURL)
		if integration != nil && strings.TrimSpace(agentRuntimeStringPtr(integration.BaseURL)) != "" {
			base = model.ResolveGitLabWebBaseURL(integration.BaseURL)
		}
		return strings.TrimRight(base, "/") + "/" + fullName + ".git"
	default:
		base := model.ResolveGitHubWebBaseURL(repo.BaseURL)
		if integration != nil && strings.TrimSpace(agentRuntimeStringPtr(integration.BaseURL)) != "" {
			base = model.ResolveGitHubWebBaseURL(integration.BaseURL)
		}
		return strings.TrimRight(base, "/") + "/" + fullName + ".git"
	}
}

func firstStringValue(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if values == nil {
			return ""
		}
		if value, ok := values[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func agentRuntimeStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func agentRuntimeHostFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
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
	provider := "github"
	var baseURL *string
	if target.IntegrationID != nil && strings.TrimSpace(*target.IntegrationID) != "" {
		if integration, err := s.integrationRepo.GetByID(ctx, workspaceID, *target.IntegrationID); err == nil && integration != nil && strings.TrimSpace(integration.Provider) != "" {
			provider = strings.TrimSpace(integration.Provider)
			baseURL = integration.BaseURL
		}
	}
	link := &model.TaskGitLink{
		WorkspaceID:   workspaceID,
		TaskID:        storyID,
		IntegrationID: deref(target.IntegrationID),
		RepositoryID:  target.RepositoryID,
		Provider:      provider,
		BaseURL:       baseURL,
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

type GitPRReconcileResult struct {
	Checked   int                         `json:"checked"`
	Updated   int                         `json:"updated"`
	Merged    int                         `json:"merged"`
	Closed    int                         `json:"closed"`
	StillOpen int                         `json:"still_open"`
	Skipped   int                         `json:"skipped"`
	Failed    int                         `json:"failed"`
	DryRun    bool                        `json:"dry_run"`
	Items     []GitPRReconcileResultItem  `json:"items"`
	Errors    []GitPRReconcileResultError `json:"errors"`
}

type GitPRReconcileResultItem struct {
	LinkID      string `json:"link_id"`
	WorkspaceID string `json:"workspace_id"`
	TaskID      string `json:"task_id"`
	Repo        string `json:"repo"`
	PRNumber    int    `json:"pr_number"`
	OldStatus   string `json:"old_status"`
	NewStatus   string `json:"new_status"`
	PRURL       string `json:"pr_url,omitempty"`
}

type GitPRReconcileResultError struct {
	LinkID   string `json:"link_id,omitempty"`
	Repo     string `json:"repo,omitempty"`
	PRNumber int    `json:"pr_number,omitempty"`
	Error    string `json:"error"`
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

	if err := s.syncTaskWorkflowForPRStatus(ctx, taskID, prStatus); err != nil {
		return err
	}

	return nil
}

func (s *GitService) syncTaskWorkflowForPRStatus(ctx context.Context, taskID, prStatus string) error {
	if s.taskRepo == nil || s.settingsRepo == nil {
		return nil
	}
	story, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("load task: %w", err)
	}
	if story == nil || story.TeamID == nil || *story.TeamID == "" {
		return nil
	}
	teamDefault, err := s.settingsRepo.GetTeamRepoDefault(ctx, *story.TeamID)
	if err != nil {
		return fmt.Errorf("load team repo default: %w", err)
	}
	if teamDefault == nil || !teamDefault.AutoSyncStates {
		return nil
	}

	var nextStateID *string
	switch prStatus {
	case "open":
		nextStateID = teamDefault.ReviewStateID
	case "merged":
		nextStateID = teamDefault.DoneStateID
	case "closed":
		nextStateID = teamDefault.ClosedStateID
	}
	if nextStateID == nil || *nextStateID == "" || story.WorkflowStateID == *nextStateID {
		return nil
	}
	story.WorkflowStateID = *nextStateID
	if err := s.taskRepo.Update(ctx, story); err != nil {
		return fmt.Errorf("update task workflow state: %w", err)
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

func (s *GitService) providerForWebhookEvent(ctx context.Context, workspaceID, repo string, link *model.TaskGitLink) string {
	if link != nil && strings.TrimSpace(link.Provider) != "" {
		return strings.TrimSpace(link.Provider)
	}
	if s.repoRepo != nil {
		if repository, err := s.repoRepo.GetByFullName(ctx, workspaceID, repo); err == nil && repository != nil && strings.TrimSpace(repository.Provider) != "" {
			return strings.TrimSpace(repository.Provider)
		}
	}
	return "github"
}

func (s *GitService) gitLinkByBranch(ctx context.Context, workspaceID, provider, repo, branch string) (*model.TaskGitLink, error) {
	if strings.TrimSpace(provider) != "" {
		return s.linkRepo.GetByProviderBranch(ctx, workspaceID, strings.TrimSpace(provider), repo, branch)
	}
	return s.linkRepo.GetByBranch(ctx, workspaceID, repo, branch)
}

func (s *GitService) gitLinkByPR(ctx context.Context, workspaceID, provider, repo string, prNumber int) (*model.TaskGitLink, error) {
	if strings.TrimSpace(provider) != "" {
		return s.linkRepo.GetByProviderPR(ctx, workspaceID, strings.TrimSpace(provider), repo, prNumber)
	}
	return s.linkRepo.GetByPR(ctx, workspaceID, repo, prNumber)
}

func (s *GitService) gitRepositoryByFullName(ctx context.Context, workspaceID, provider, repo string) (*model.GitRepository, error) {
	if strings.TrimSpace(provider) != "" {
		return s.repoRepo.GetByProviderFullName(ctx, workspaceID, strings.TrimSpace(provider), repo)
	}
	return s.repoRepo.GetByFullName(ctx, workspaceID, repo)
}

func repositoryProvider(repository *model.GitRepository) string {
	if repository != nil && strings.TrimSpace(repository.Provider) != "" {
		return strings.TrimSpace(repository.Provider)
	}
	return "github"
}

func pushTriggerForProvider(provider string) string {
	if provider == "gitlab" {
		return model.TriggerGitLabPush
	}
	return model.TriggerGitHubPush
}

func prOpenedTriggerForProvider(provider string) string {
	if provider == "gitlab" {
		return model.TriggerGitLabMROpened
	}
	return model.TriggerGitHubPROpened
}

func prMergedTriggerForProvider(provider string) string {
	if provider == "gitlab" {
		return model.TriggerGitLabMRMerged
	}
	return model.TriggerGitHubPRMerged
}

func prClosedTriggerForProvider(provider string) string {
	if provider == "gitlab" {
		return model.TriggerGitLabMRClosed
	}
	return model.TriggerGitHubPRClosed
}

func releaseTriggerForProvider(provider string) string {
	if provider == "gitlab" {
		return model.TriggerGitLabReleasePub
	}
	return model.TriggerGitHubReleasePub
}

func pipelineTriggerForProvider(provider string) string {
	if provider == "gitlab" {
		return model.TriggerGitLabPipeline
	}
	return model.TriggerGitHubCheckSuite
}

// ReconcileOpenPullRequestStatuses repairs missed pull request lifecycle webhooks
// by comparing persisted open PR links with GitHub's current source of truth.
func (s *GitService) ReconcileOpenPullRequestStatuses(ctx context.Context, limit int, dryRun bool) (*GitPRReconcileResult, error) {
	if s.githubApp == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	if s.linkRepo == nil || s.integrationRepo == nil {
		return nil, fmt.Errorf("git repositories are not configured")
	}
	links, err := s.linkRepo.ListOpenPullRequests(ctx, limit)
	if err != nil {
		return nil, err
	}

	result := &GitPRReconcileResult{DryRun: dryRun}
	for _, link := range links {
		if link.PRNumber == nil {
			result.Skipped++
			continue
		}
		result.Checked++
		item := GitPRReconcileResultItem{
			LinkID:      link.ID,
			WorkspaceID: link.WorkspaceID,
			TaskID:      link.TaskID,
			Repo:        link.Repo,
			PRNumber:    *link.PRNumber,
			OldStatus:   strings.TrimSpace(derefString(link.PRStatus)),
		}
		if item.OldStatus == "" {
			item.OldStatus = "open"
		}

		integration, err := s.integrationRepo.GetByIDAny(ctx, link.IntegrationID)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, reconcileError(link, err))
			continue
		}
		if integration == nil || !integration.Active || integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
			result.Skipped++
			result.Errors = append(result.Errors, reconcileError(link, fmt.Errorf("active github integration with installation id not found")))
			continue
		}
		owner, repo, ok := splitRepoFullName(link.Repo)
		if !ok {
			result.Skipped++
			result.Errors = append(result.Errors, reconcileError(link, fmt.Errorf("invalid repo full name")))
			continue
		}

		pr, err := s.githubApp.GetPullRequest(ctx, *integration.InstallationID, owner, repo, *link.PRNumber)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, reconcileError(link, err))
			continue
		}
		newStatus := statusForGitHubPullRequest(pr)
		item.NewStatus = newStatus
		if pr != nil {
			item.PRURL = strings.TrimSpace(pr.HTMLURL)
		}
		result.Items = append(result.Items, item)

		if newStatus == "open" {
			result.StillOpen++
			continue
		}
		if newStatus == "merged" {
			result.Merged++
		}
		if newStatus == "closed" {
			result.Closed++
		}
		result.Updated++
		if dryRun {
			continue
		}

		prTitle := ""
		prURL := ""
		if pr != nil {
			prTitle = strings.TrimSpace(pr.Title)
			prURL = strings.TrimSpace(pr.HTMLURL)
		}
		prNumber := *link.PRNumber
		if err := s.updateDeliveryStatusForPR(ctx, link.WorkspaceID, link.TaskID, newStatus, &deliveryStatusMetadata{
			LinkID:   link.ID,
			PRNumber: &prNumber,
			PRTitle:  &prTitle,
			PRURL:    &prURL,
		}); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, reconcileError(link, err))
			slog.ErrorContext(ctx, "git pr reconcile update failed", "error", err, "link_id", link.ID, "workspace_id", link.WorkspaceID, "repo", link.Repo, "pr_number", prNumber)
			continue
		}
	}
	return result, nil
}

func statusForGitHubPullRequest(pr *githubapp.PullRequest) string {
	if pr == nil {
		return "open"
	}
	if pr.Merged || pr.MergedAt != nil {
		return "merged"
	}
	if strings.EqualFold(strings.TrimSpace(pr.State), "closed") {
		return "closed"
	}
	return "open"
}

func splitRepoFullName(repoFullName string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(repoFullName), "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func reconcileError(link model.TaskGitLink, err error) GitPRReconcileResultError {
	item := GitPRReconcileResultError{
		LinkID: link.ID,
		Repo:   link.Repo,
		Error:  err.Error(),
	}
	if link.PRNumber != nil {
		item.PRNumber = *link.PRNumber
	}
	return item
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
	return s.processWebhookPush(ctx, workspaceID, "", repo, branch, commitSHA)
}

// ProcessWebhookPushForProvider handles a provider-scoped push event.
func (s *GitService) ProcessWebhookPushForProvider(ctx context.Context, workspaceID, provider, repo, branch, commitSHA string) error {
	return s.processWebhookPush(ctx, workspaceID, provider, repo, branch, commitSHA)
}

func (s *GitService) processWebhookPush(ctx context.Context, workspaceID, provider, repo, branch, commitSHA string) error {
	link, err := s.gitLinkByBranch(ctx, workspaceID, provider, repo, branch)
	if err != nil {
		return err
	}
	resolvedProvider := strings.TrimSpace(provider)
	if resolvedProvider == "" {
		resolvedProvider = s.providerForWebhookEvent(ctx, workspaceID, repo, link)
	}
	pushTrigger := pushTriggerForProvider(resolvedProvider)
	if link == nil {
		if s.ruleEngine != nil {
			s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
				WorkspaceID:  workspaceID,
				TriggerType:  pushTrigger,
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
			TriggerType:  pushTrigger,
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
	return s.processWebhookPR(ctx, workspaceID, "", repo, action, prNumber, prTitle, prURL, prStatus, branch, baseBranch)
}

// ProcessWebhookPRForProvider handles a provider-scoped PR/MR event.
func (s *GitService) ProcessWebhookPRForProvider(ctx context.Context, workspaceID, provider, repo, action string, prNumber int, prTitle, prURL, prStatus, branch, baseBranch string) error {
	return s.processWebhookPR(ctx, workspaceID, provider, repo, action, prNumber, prTitle, prURL, prStatus, branch, baseBranch)
}

func (s *GitService) processWebhookPR(ctx context.Context, workspaceID, provider, repo, action string, prNumber int, prTitle, prURL, prStatus, branch, baseBranch string) error {
	link, err := s.gitLinkByPR(ctx, workspaceID, provider, repo, prNumber)
	if err != nil {
		return err
	}
	if link == nil && branch != "" {
		link, err = s.gitLinkByBranch(ctx, workspaceID, provider, repo, branch)
		if err != nil {
			return err
		}
	}
	resolvedProvider := strings.TrimSpace(provider)
	if resolvedProvider == "" {
		resolvedProvider = s.providerForWebhookEvent(ctx, workspaceID, repo, link)
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
		if prStatus == "closed" && story != nil && story.TeamID != nil && *story.TeamID != "" {
			if teamDefault, cfgErr := s.settingsRepo.GetTeamRepoDefault(ctx, *story.TeamID); cfgErr == nil && teamDefault != nil && teamDefault.AutoSyncStates && teamDefault.ClosedStateID != nil {
				story.WorkflowStateID = *teamDefault.ClosedStateID
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
			event.TriggerType = prMergedTriggerForProvider(resolvedProvider)
			s.ruleEngine.EvaluateEvent(ctx, event, nil)
		case prStatus == "closed":
			event.TriggerType = prClosedTriggerForProvider(resolvedProvider)
			s.ruleEngine.EvaluateEvent(ctx, event, nil)
		case strings.TrimSpace(action) == "opened":
			event.TriggerType = prOpenedTriggerForProvider(resolvedProvider)
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
	return s.processWebhookRelease(ctx, workspaceID, "", repo, action, tagName, targetCommitish, releaseName, releaseURL, publishedAt, isPrerelease)
}

// ProcessWebhookReleaseForProvider handles a provider-scoped release event.
func (s *GitService) ProcessWebhookReleaseForProvider(ctx context.Context, workspaceID, provider, repo, action, tagName, targetCommitish, releaseName, releaseURL string, publishedAt *time.Time, isPrerelease bool) error {
	return s.processWebhookRelease(ctx, workspaceID, provider, repo, action, tagName, targetCommitish, releaseName, releaseURL, publishedAt, isPrerelease)
}

func (s *GitService) processWebhookRelease(ctx context.Context, workspaceID, provider, repo, action, tagName, targetCommitish, releaseName, releaseURL string, publishedAt *time.Time, isPrerelease bool) error {
	if strings.TrimSpace(action) != "published" || s.ruleEngine == nil {
		return nil
	}

	repository, err := s.gitRepositoryByFullName(ctx, workspaceID, provider, repo)
	if err != nil {
		return err
	}
	resolvedProvider := strings.TrimSpace(provider)
	if resolvedProvider == "" {
		resolvedProvider = repositoryProvider(repository)
	}

	event := model.AutomationEvent{
		WorkspaceID:     workspaceID,
		TriggerType:     releaseTriggerForProvider(resolvedProvider),
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
		link, err := s.gitLinkByBranch(ctx, workspaceID, provider, repo, targetCommitish)
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
	return s.processWebhookCheckSuite(ctx, workspaceID, "", repo, action, branch, conclusion)
}

// ProcessWebhookCheckSuiteForProvider handles a provider-scoped pipeline/check event.
func (s *GitService) ProcessWebhookCheckSuiteForProvider(ctx context.Context, workspaceID, provider, repo, action, branch, conclusion string) error {
	return s.processWebhookCheckSuite(ctx, workspaceID, provider, repo, action, branch, conclusion)
}

func (s *GitService) processWebhookCheckSuite(ctx context.Context, workspaceID, provider, repo, action, branch, conclusion string) error {
	if strings.TrimSpace(action) != "completed" || s.ruleEngine == nil {
		return nil
	}
	resolvedProvider := strings.TrimSpace(provider)
	if resolvedProvider == "" {
		resolvedProvider = s.providerForWebhookEvent(ctx, workspaceID, repo, nil)
	}

	event := model.AutomationEvent{
		WorkspaceID:  workspaceID,
		TriggerType:  pipelineTriggerForProvider(resolvedProvider),
		RepoFullName: repo,
		Branch:       branch,
		Conclusion:   conclusion,
	}
	if strings.TrimSpace(branch) != "" {
		link, err := s.gitLinkByBranch(ctx, workspaceID, provider, repo, branch)
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
	OrganizationID string `json:"organization_id"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	ActorID        string `json:"actor_id,omitempty"`
	jwt.RegisteredClaims
}

func (s *GitService) signGitHubInstallState(organizationID, workspaceID, actorID string) (string, error) {
	if s.stateSecret == "" {
		return "", fmt.Errorf("github app state secret is not configured")
	}
	if strings.TrimSpace(organizationID) == "" {
		return "", fmt.Errorf("organization_id is required")
	}

	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	claims := gitHubInstallState{
		OrganizationID: strings.TrimSpace(organizationID),
		WorkspaceID:    strings.TrimSpace(workspaceID),
		ActorID:        strings.TrimSpace(actorID),
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
	if strings.TrimSpace(claims.OrganizationID) == "" {
		return nil, nil, "", fmt.Errorf("github app callback organization is missing")
	}
	workspace, redirectURL, err := s.resolveReturnWorkspace(ctx, claims.OrganizationID, claims.WorkspaceID)
	if err != nil {
		return nil, nil, "", err
	}
	return claims, workspace, redirectURL, nil
}

func (s *GitService) upsertGitLabTokenCredential(
	ctx context.Context,
	organizationID, actorID, baseURL, authType, token, label string,
	user *gitlab.User,
) (*model.GitCredential, error) {
	if user == nil || user.ID == 0 {
		return nil, fmt.Errorf("gitlab user is required")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("gitlab token is required")
	}
	encryptedToken, err := appcrypto.EncryptString(token, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt gitlab token: %w", err)
	}
	externalUserID := strconv.FormatInt(user.ID, 10)
	accountLogin := strings.TrimSpace(user.Username)
	displayName := strings.TrimSpace(label)
	if displayName == "" {
		displayName = "GitLab"
		if accountLogin != "" {
			displayName = "GitLab " + accountLogin
		}
	}
	credential := &model.GitCredential{
		OrganizationID:       organizationID,
		Provider:             "gitlab",
		BaseURL:              baseURL,
		AuthType:             authType,
		ExternalUserID:       &externalUserID,
		AccountLogin:         trimPtr(&accountLogin),
		DisplayName:          displayName,
		AccessTokenEncrypted: &encryptedToken,
		Status:               "active",
		ConnectedBy:          trimPtr(&actorID),
	}
	return s.credentialRepo.UpsertOAuthUser(ctx, credential)
}

func (s *GitService) upsertGitLabIntegration(ctx context.Context, organizationID string, workspace *model.Workspace, credential *model.GitCredential, actorID string) (*model.GitIntegration, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	if credential == nil || strings.TrimSpace(credential.ID) == "" {
		return nil, fmt.Errorf("gitlab credential is required")
	}
	var workspaceID *string
	if workspace != nil && strings.TrimSpace(workspace.ID) != "" {
		workspaceID = strPtr(workspace.ID)
	}
	accountLogin := strings.TrimSpace(derefString(credential.AccountLogin))
	displayName := credential.DisplayName
	if strings.TrimSpace(displayName) == "" {
		displayName = "GitLab"
	}
	existing, err := s.integrationRepo.GetActiveByCredential(ctx, credential.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		secret := generateWebhookSecret()
		integration := &model.GitIntegration{
			WorkspaceID:    workspaceID,
			OrganizationID: strPtr(strings.TrimSpace(organizationID)),
			Provider:       "gitlab",
			DisplayName:    displayName,
			CredentialMode: "gitlab_oauth",
			CredentialID:   &credential.ID,
			AccountLogin:   trimPtr(&accountLogin),
			BaseURL:        strPtr(credential.BaseURL),
			WebhookSecret:  &secret,
			Active:         true,
		}
		if err := s.integrationRepo.Create(ctx, integration); err != nil {
			return nil, err
		}
		s.publishSimpleEvent("created", "git_integration", integration.ID, workspaceIDForIntegration(integration), actorID)
		return integration, nil
	}
	existing.DisplayName = displayName
	existing.WorkspaceID = workspaceID
	existing.OrganizationID = strPtr(strings.TrimSpace(organizationID))
	existing.CredentialMode = "gitlab_oauth"
	existing.CredentialID = &credential.ID
	existing.AccountLogin = trimPtr(&accountLogin)
	existing.BaseURL = strPtr(credential.BaseURL)
	existing.Active = true
	existing.DeletedAt = nil
	if existing.WebhookSecret == nil || strings.TrimSpace(*existing.WebhookSecret) == "" {
		secret := generateWebhookSecret()
		existing.WebhookSecret = &secret
	}
	if err := s.integrationRepo.Update(ctx, existing); err != nil {
		return nil, err
	}
	s.publishSimpleEvent("updated", "git_integration", existing.ID, workspaceIDForIntegration(existing), actorID)
	return existing, nil
}

func (s *GitService) gitlabAccessToken(ctx context.Context, integration *model.GitIntegration) (string, error) {
	if integration == nil || integration.CredentialID == nil || strings.TrimSpace(*integration.CredentialID) == "" {
		return "", fmt.Errorf("gitlab integration has no credential")
	}
	if s.credentialRepo == nil || len(s.encryptionKey) != 32 {
		return "", fmt.Errorf("gitlab credential storage is not configured")
	}
	credential, err := s.credentialRepo.GetByID(ctx, *integration.CredentialID)
	if err != nil {
		return "", err
	}
	if credential == nil || credential.AccessTokenEncrypted == nil || strings.TrimSpace(*credential.AccessTokenEncrypted) == "" {
		return "", fmt.Errorf("gitlab credential is not available")
	}
	return appcrypto.DecryptString(*credential.AccessTokenEncrypted, s.encryptionKey)
}

func (s *GitService) tryInstallGitLabWebhook(ctx context.Context, integration *model.GitIntegration, externalID string, permissions map[string]bool) {
	if integration == nil || !permissions["manage_webhooks"] || integration.WebhookSecret == nil || strings.TrimSpace(*integration.WebhookSecret) == "" {
		return
	}
	projectID, err := strconv.ParseInt(strings.TrimSpace(externalID), 10, 64)
	if err != nil {
		return
	}
	token, err := s.gitlabAccessToken(ctx, integration)
	if err != nil {
		return
	}
	hookURL := strings.TrimRight(s.appBaseURL, "/") + "/api/git/webhook"
	client := s.gitLabClientFor(derefString(integration.BaseURL))
	if _, err := client.UpsertProjectWebhook(ctx, token, projectID, hookURL, *integration.WebhookSecret); err != nil {
		slog.WarnContext(ctx, "gitlab webhook auto-install failed", "integration_id", integration.ID, "project_id", externalID, "error", err)
	}
}

func (s *GitService) upsertGitHubIntegration(
	ctx context.Context,
	organizationID string,
	workspace *model.Workspace,
	installationID string,
	installation *githubapp.Installation,
	actorID string,
) (*model.GitIntegration, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	accountLogin := strings.TrimSpace(installation.AccountLogin)
	appID := githubapp.InstallationIDString(installation.AppID)
	displayName := "GitHub App"
	if accountLogin != "" {
		displayName = "GitHub " + accountLogin
	}
	orgID := strings.TrimSpace(organizationID)
	var workspaceID *string
	if workspace != nil && strings.TrimSpace(workspace.ID) != "" {
		workspaceID = strPtr(workspace.ID)
	}

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
			WorkspaceID:    workspaceID,
			OrganizationID: strPtr(orgID),
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
			_ = s.activitySvc.Log(ctx, workspaceIDForIntegration(integration), "git_integration", integration.ID, &actorID, "created", nil, nil, &integration.DisplayName, nil)
		}
		s.publishSimpleEvent("created", "git_integration", integration.ID, workspaceIDForIntegration(integration), actorID)
		return integration, nil
	}

	existing.DisplayName = displayName
	if existing.WorkspaceID == nil {
		existing.WorkspaceID = workspaceID
	}
	existing.OrganizationID = strPtr(orgID)
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
		_ = s.activitySvc.Log(ctx, workspaceIDForIntegration(existing), "git_integration", existing.ID, &actorID, "updated", nil, nil, &existing.DisplayName, nil)
	}
	s.publishSimpleEvent("updated", "git_integration", existing.ID, workspaceIDForIntegration(existing), actorID)
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

func (s *GitService) isOrgAdminOrOwner(ctx context.Context, organizationID, userID string) bool {
	if s.orgRepo == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(userID) == "" {
		return false
	}
	role, err := s.orgRepo.GetMemberRole(ctx, strings.TrimSpace(organizationID), strings.TrimSpace(userID))
	if err != nil {
		return false
	}
	role = strings.ToLower(strings.TrimSpace(role))
	return role == model.RoleOwner || role == model.RoleAdmin
}

func (s *GitService) isOrgMember(ctx context.Context, organizationID, userID string) bool {
	if s.orgRepo == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(userID) == "" {
		return false
	}
	role, err := s.orgRepo.GetMemberRole(ctx, strings.TrimSpace(organizationID), strings.TrimSpace(userID))
	return err == nil && strings.TrimSpace(role) != ""
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

func (s *GitService) organizationSettingsURL(workspaceSlug string) string {
	base := strings.TrimRight(s.appBaseURL, "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	if strings.TrimSpace(workspaceSlug) == "" {
		return base
	}
	return fmt.Sprintf("%s/w/%s/settings/git-connections", base, workspaceSlug)
}

func (s *GitService) resolveReturnWorkspace(ctx context.Context, organizationID, workspaceID string) (*model.Workspace, string, error) {
	if s.workspaceRepo == nil {
		return nil, s.organizationSettingsURL(""), nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, s.organizationSettingsURL(""), nil
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, "", err
	}
	if workspace == nil {
		return nil, "", fmt.Errorf("return workspace not found")
	}
	if workspace.OrganizationID == nil || strings.TrimSpace(*workspace.OrganizationID) != strings.TrimSpace(organizationID) {
		return nil, "", fmt.Errorf("return workspace does not belong to this organization")
	}
	return workspace, s.organizationSettingsURL(workspace.Slug), nil
}

func workspaceIDForIntegration(integration *model.GitIntegration) string {
	if integration == nil || integration.WorkspaceID == nil {
		return ""
	}
	return strings.TrimSpace(*integration.WorkspaceID)
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

func normalizeOptionalCommitAuthorEmail(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized, err := normalizeCommitAuthorEmail(*value)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func normalizeCommitAuthorEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || strings.TrimSpace(parsed.Address) == "" {
		return "", fmt.Errorf("default_commit_author_email must be a valid email address")
	}
	return strings.TrimSpace(parsed.Address), nil
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

func gitlabProjectPermissions(project gitlab.Project) map[string]bool {
	level := 0
	if project.Permissions.ProjectAccess != nil && project.Permissions.ProjectAccess.AccessLevel > level {
		level = project.Permissions.ProjectAccess.AccessLevel
	}
	if project.Permissions.GroupAccess != nil && project.Permissions.GroupAccess.AccessLevel > level {
		level = project.Permissions.GroupAccess.AccessLevel
	}
	return map[string]bool{
		"read":            level >= 10,
		"push":            level >= 30,
		"merge_request":   level >= 30,
		"manage_webhooks": level >= 40,
	}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
