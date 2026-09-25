package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Errors returned by ClaimGitHubInstallation, translated to HTTP statuses by
// the handler.
var (
	// ErrGitHubInstallClaimUnavailable means installations can only be linked
	// through Helpin's signed install link in this edition.
	ErrGitHubInstallClaimUnavailable = errors.New("start the GitHub installation from Helpin to connect it")
	// ErrGitHubInstallClaimForbidden means the actor is not an organization owner or admin.
	ErrGitHubInstallClaimForbidden = errors.New("only organization owners or admins can connect git integrations")
	// ErrGitHubInstallClaimInvalid wraps malformed installation or workspace IDs.
	ErrGitHubInstallClaimInvalid = errors.New("invalid github installation request")
	// ErrGitHubInstallAppMissing means no GitHub App is configured.
	ErrGitHubInstallAppMissing = errors.New("the github app is not configured on this server")
	// ErrGitHubInstallationNotFound means GitHub has no such installation of this App.
	ErrGitHubInstallationNotFound = errors.New("this installation does not belong to the GitHub App configured for Helpin")
	// ErrGitHubInstallationClaimed means another Helpin organization already uses the installation.
	ErrGitHubInstallationClaimed = errors.New("this GitHub installation is already connected to another Helpin organization")
)

// SetGitHubInstallClaimEnabled allows ClaimGitHubInstallation. Enable it only
// for a private, single-tenant App (Community), where every installation
// belongs to the App owner's account; a public App could otherwise be claimed
// by any Helpin organization that learns an installation ID.
func (s *GitService) SetGitHubInstallClaimEnabled(enabled bool) *GitService {
	s.installClaimEnabled = enabled
	return s
}

// gitHubAppSourceReporter is implemented by credential sources that know
// whether the App was created from Helpin (stored) or pinned in env.
type gitHubAppSourceReporter interface {
	Source(ctx context.Context) string
}

// gitHubAppCreatedInHelpin reports whether the active App was created through
// the manifest flow. Those Apps are private, so every installation belongs to
// the owner's account. An env-pinned App may be public, so its installations
// must start from Helpin's signed link instead of being claimed.
func (s *GitService) gitHubAppCreatedInHelpin(ctx context.Context) bool {
	reporter, ok := s.githubAppSource.(gitHubAppSourceReporter)
	return ok && reporter.Source(ctx) == model.GitHubAppSourceDatabase
}

// GitHubAppInstallRedirect returns the GitHub install page for the App with
// install state bound to the workspace's Helpin organization, workspace, actor
// and returnTo. The manifest flow uses it to send the browser straight from
// App creation to installation.
func (s *GitService) GitHubAppInstallRedirect(ctx context.Context, slug, workspaceID, actorID, returnTo string) (string, error) {
	if s.workspaceRepo == nil {
		return "", fmt.Errorf("workspace repository is not configured")
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return "", fmt.Errorf("get workspace %q: %w", workspaceID, err)
	}
	if workspace == nil || workspace.OrganizationID == nil || strings.TrimSpace(*workspace.OrganizationID) == "" {
		return "", fmt.Errorf("workspace %q has no organization", workspaceID)
	}
	return s.gitHubAppInstallURL(slug, strings.TrimSpace(*workspace.OrganizationID), workspace.ID, actorID, returnTo)
}

// GitHubInstallCallbackRedirect finishes GET /api/git/github/callback. It
// never fails: installs with valid Helpin state complete here, and installs
// without (or with expired) state go to the frontend claim route.
func (s *GitService) GitHubInstallCallbackRedirect(ctx context.Context, stateToken, installationID, setupAction string) string {
	installationID = strings.TrimSpace(installationID)
	setupAction = strings.ToLower(strings.TrimSpace(setupAction))
	if strings.TrimSpace(stateToken) != "" {
		redirectURL, err := s.CompleteGitHubInstall(ctx, stateToken, installationID)
		if err == nil {
			return redirectURL
		}
		slog.WarnContext(ctx, "github install callback state rejected", "installation_id", installationID, "error", err)
	}
	if installationID == "" {
		message := "GitHub did not return an installation. Start the installation again from Helpin."
		if setupAction == "request" {
			message = "GitHub sent your installation request to the account owner. Connect GitHub again after they approve it."
		}
		return gitHubInstalledURL(s.appBaseURL, gitHubResultQuery(GitHubResultError, message))
	}
	query := url.Values{}
	query.Set("installation_id", installationID)
	if setupAction == "install" || setupAction == "update" {
		query.Set("setup_action", setupAction)
	}
	return gitHubInstalledURL(s.appBaseURL, query)
}

// ClaimGitHubInstallation links an installation that reached Helpin without
// state (installed or updated from GitHub) to organizationID. The caller must
// be an organization owner or admin; the installation must belong to the
// configured App and must not be linked to another organization. Claiming an
// installation the organization already has refreshes it.
func (s *GitService) ClaimGitHubInstallation(ctx context.Context, organizationID, workspaceID, actorID, installationID string) (*model.GitHubInstallationClaimResponse, error) {
	organizationID = strings.TrimSpace(organizationID)
	installationID = strings.TrimSpace(installationID)
	if !s.installClaimEnabled || !s.gitHubAppCreatedInHelpin(ctx) {
		return nil, ErrGitHubInstallClaimUnavailable
	}
	if !isGitHubInstallationID(installationID) {
		return nil, fmt.Errorf("%w: installation_id must be numeric", ErrGitHubInstallClaimInvalid)
	}
	if !s.isOrgAdminOrOwner(ctx, organizationID, actorID) {
		return nil, ErrGitHubInstallClaimForbidden
	}
	if !s.hasGitHubApp(ctx) {
		return nil, ErrGitHubInstallAppMissing
	}
	workspace, _, err := s.resolveReturnWorkspace(ctx, organizationID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitHubInstallClaimInvalid, err)
	}

	installation, err := s.githubApp.GetInstallation(ctx, installationID)
	if err != nil {
		if errors.Is(err, githubapp.ErrInstallationNotFound) {
			return nil, ErrGitHubInstallationNotFound
		}
		return nil, fmt.Errorf("verify github installation %s: %w", installationID, err)
	}
	appID := strings.TrimSpace(s.currentGitHubApp(ctx).AppID)
	if appID != "" && installation.AppID != 0 && githubapp.InstallationIDString(installation.AppID) != appID {
		return nil, ErrGitHubInstallationNotFound
	}

	existing, err := s.integrationRepo.GetByInstallationID(ctx, "github", installationID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.OrganizationID != nil && strings.TrimSpace(*existing.OrganizationID) != organizationID {
		return nil, ErrGitHubInstallationClaimed
	}
	integration, err := s.upsertGitHubIntegration(ctx, organizationID, workspace, installationID, installation, actorID)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "github installation claimed", "organization_id", organizationID, "user_id", actorID,
		"installation_id", installationID, "integration_id", integration.ID, "created", existing == nil)
	return &model.GitHubInstallationClaimResponse{
		IntegrationID: integration.ID,
		AccountLogin:  strings.TrimSpace(installation.AccountLogin),
		AccountType:   strings.TrimSpace(installation.AccountType),
		Created:       existing == nil,
	}, nil
}

func (s *GitService) gitHubAppInstallURL(slug, organizationID, workspaceID, actorID, returnTo string) (string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", fmt.Errorf("github app slug is not known")
	}
	state, err := s.signGitHubInstallState(organizationID, workspaceID, actorID, returnTo)
	if err != nil {
		return "", err
	}
	installURL := url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   "/apps/" + slug + "/installations/new",
	}
	query := installURL.Query()
	query.Set("state", state)
	installURL.RawQuery = query.Encode()
	return installURL.String(), nil
}

// gitHubInstallReturnURL is the page an install returns to: the return_to page
// of the workspace, or the frontend result route when there is none.
func (s *GitService) gitHubInstallReturnURL(workspace *model.Workspace, returnTo string) string {
	if workspace == nil || strings.TrimSpace(workspace.Slug) == "" {
		return gitHubInstalledURL(s.appBaseURL, nil)
	}
	return gitHubReturnPageURL(s.appBaseURL, workspace.Slug, returnTo)
}

func isGitHubInstallationID(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
