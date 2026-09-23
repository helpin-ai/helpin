package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Manifest flow errors, translated to HTTP statuses by the handler.
var (
	// ErrGitHubAppManifestUnavailable means the manifest flow is not offered
	// in this edition or deployment.
	ErrGitHubAppManifestUnavailable = errors.New("github app manifest flow is not available")
	// ErrGitHubAppAlreadyConfigured means an App is already configured from
	// the environment or the database.
	ErrGitHubAppAlreadyConfigured = errors.New("a github app is already configured")
	// ErrGitHubAppManifestInvalid wraps request validation failures.
	ErrGitHubAppManifestInvalid = errors.New("invalid github app manifest request")
)

const (
	_gitHubAppManifestStatePurpose = "github_app_manifest"
	_gitHubAppManifestStateTTL     = 30 * time.Minute
	_gitHubAppNameMaxLength        = 34
)

var _gitHubOrganizationLogin = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)

type gitHubAppWorkspaceLookup interface {
	GetByID(ctx context.Context, id string) (*model.Workspace, error)
	GetMemberRole(ctx context.Context, workspaceID, userID string) (string, error)
}

type gitHubAppManifestConverter interface {
	Convert(ctx context.Context, code string) (*githubapp.ManifestConversion, error)
}

type gitHubAppManifestState struct {
	Purpose     string `json:"purpose"`
	WorkspaceID string `json:"workspace_id"`
	ActorID     string `json:"actor_id"`
	// ReturnTo is the Helpin page to return to; absent in older tokens,
	// which return to settings.
	ReturnTo string `json:"return_to,omitempty"`
	jwt.RegisteredClaims
}

// gitHubAppManifest is the GitHub App manifest document. Permissions and
// events cover exactly what Helpin uses: repository contents (clone, push,
// branches, merges, releases, compare), pull requests, and check runs.
type gitHubAppManifest struct {
	Name               string                `json:"name"`
	URL                string                `json:"url"`
	HookAttributes     gitHubAppManifestHook `json:"hook_attributes"`
	RedirectURL        string                `json:"redirect_url"`
	SetupURL           string                `json:"setup_url"`
	SetupOnUpdate      bool                  `json:"setup_on_update"`
	Description        string                `json:"description"`
	Public             bool                  `json:"public"`
	DefaultPermissions map[string]string     `json:"default_permissions"`
	DefaultEvents      []string              `json:"default_events"`
}

type gitHubAppManifestHook struct {
	URL    string `json:"url"`
	Active bool   `json:"active"`
}

// CreateManifest returns the manifest and GitHub URL the browser must POST to.
// Route middleware restricts callers to workspace owners.
func (s *GitHubAppConfigService) CreateManifest(ctx context.Context, workspaceID, actorID string, req model.GitHubAppManifestRequest) (*model.GitHubAppManifestResponse, error) {
	if !s.opts.ManifestEnabled {
		return nil, ErrGitHubAppManifestUnavailable
	}
	_, source, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	if source != model.GitHubAppSourceNone {
		return nil, ErrGitHubAppAlreadyConfigured
	}
	if !s.manifestAvailable(source) {
		if reason := GitHubAppBaseURLBlockedReason(s.opts.AppBaseURL); reason != "" {
			return nil, fmt.Errorf("%w: %s", ErrGitHubAppManifestUnavailable, reason)
		}
		return nil, fmt.Errorf("%w: set APP_BASE_URL, JWT_SECRET and GIT_OAUTH_ENCRYPTION_KEY", ErrGitHubAppManifestUnavailable)
	}
	workspaceID = strings.TrimSpace(workspaceID)
	actorID = strings.TrimSpace(actorID)
	if workspaceID == "" || actorID == "" {
		return nil, fmt.Errorf("%w: workspace and user are required", ErrGitHubAppManifestInvalid)
	}
	organization := strings.TrimSpace(req.Organization)
	if organization != "" && !_gitHubOrganizationLogin.MatchString(organization) {
		return nil, fmt.Errorf("%w: organization must be a GitHub organization login", ErrGitHubAppManifestInvalid)
	}
	returnTo, err := NormalizeGitHubReturnTo(req.ReturnTo)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitHubAppManifestInvalid, err)
	}

	manifest, err := json.Marshal(s.buildManifest())
	if err != nil {
		return nil, fmt.Errorf("encode github app manifest: %w", err)
	}
	state, err := s.signManifestState(workspaceID, actorID, returnTo)
	if err != nil {
		return nil, err
	}
	postURL := "https://github.com/settings/apps/new"
	if organization != "" {
		postURL = "https://github.com/organizations/" + url.PathEscape(organization) + "/settings/apps/new"
	}
	postURL += "?state=" + url.QueryEscape(state)
	return &model.GitHubAppManifestResponse{Manifest: manifest, PostURL: postURL, State: state}, nil
}

// CompleteManifest exchanges the manifest code, stores the new App and
// returns the URL to redirect the browser to: the new App's GitHub install
// page (with Helpin install state) on success, otherwise the return_to page
// with the github=created|error result flag. An invalid state goes to the
// frontend /github/installed route with the error.
func (s *GitHubAppConfigService) CompleteManifest(ctx context.Context, code, stateToken string) string {
	claims, err := s.parseManifestState(stateToken)
	if err != nil {
		return gitHubInstalledURL(s.opts.AppBaseURL, gitHubResultQuery(GitHubResultError,
			"The GitHub App setup link is invalid or expired. Start again from Helpin."))
	}
	fail := func(message string) string {
		return s.manifestRedirect(ctx, claims, GitHubResultError, message)
	}
	if !s.opts.ManifestEnabled {
		return fail("Creating a GitHub App from Helpin is not available in this edition.")
	}
	if !s.isWorkspaceOwner(ctx, claims.WorkspaceID, claims.ActorID) {
		return fail("Only a workspace owner can create the GitHub App.")
	}
	if _, source, err := s.resolve(ctx); err != nil || source != model.GitHubAppSourceNone {
		return fail("A GitHub App is already configured for this Helpin instance. Delete the new App on GitHub if you do not need it.")
	}
	if strings.TrimSpace(code) == "" {
		return fail("GitHub did not return an App code.")
	}

	conversion, err := s.converter.Convert(ctx, code)
	if err != nil {
		slog.ErrorContext(ctx, "github app manifest conversion failed", "workspace_id", claims.WorkspaceID, "user_id", claims.ActorID, "error", err)
		return fail("GitHub could not complete the App setup. Try again.")
	}
	if err := s.storeConversion(ctx, conversion, claims.ActorID); err != nil {
		if errors.Is(err, repository.ErrGitHubAppCredentialExists) {
			return fail("A GitHub App is already configured for this Helpin instance. Delete the new App on GitHub if you do not need it.")
		}
		slog.ErrorContext(ctx, "store github app credentials failed", "workspace_id", claims.WorkspaceID, "user_id", claims.ActorID, "error", err)
		return fail("Helpin could not save the GitHub App credentials.")
	}
	slog.InfoContext(ctx, "github app created from manifest", "workspace_id", claims.WorkspaceID, "user_id", claims.ActorID, "app_id", conversion.ID, "slug", conversion.Slug)
	if s.installer != nil {
		installURL, err := s.installer.GitHubAppInstallRedirect(ctx, conversion.Slug, claims.WorkspaceID, claims.ActorID, claims.ReturnTo)
		if err == nil {
			return installURL
		}
		slog.ErrorContext(ctx, "github app install link failed after creation", "workspace_id", claims.WorkspaceID, "user_id", claims.ActorID, "slug", conversion.Slug, "error", err)
	}
	return s.manifestRedirect(ctx, claims, GitHubResultCreated, fmt.Sprintf("GitHub App %s created. Install it to connect repositories.", conversion.Slug))
}

func (s *GitHubAppConfigService) storeConversion(ctx context.Context, conversion *githubapp.ManifestConversion, actorID string) error {
	creds := trimGitHubAppCredentials(conversion.Credentials())
	privateKey, err := s.encryptField("private_key", creds.PrivateKey)
	if err != nil {
		return err
	}
	clientSecret, err := s.encryptField("client_secret", creds.ClientSecret)
	if err != nil {
		return err
	}
	webhookSecret, err := s.encryptField("webhook_secret", creds.WebhookSecret)
	if err != nil {
		return err
	}
	row := &model.GitHubAppCredential{
		AppID:                  creds.AppID,
		Slug:                   creds.Slug,
		Name:                   strings.TrimSpace(conversion.Name),
		ClientID:               trimPtr(&creds.ClientID),
		ClientSecretEncrypted:  clientSecret,
		PrivateKeyEncrypted:    derefString(privateKey),
		WebhookSecretEncrypted: webhookSecret,
		HTMLURL:                trimPtr(&creds.HTMLURL),
		OwnerLogin:             trimPtr(&conversion.OwnerLogin),
		OwnerType:              trimPtr(&conversion.OwnerType),
		CreatedBy:              trimPtr(&actorID),
	}
	err = s.store.Create(ctx, row, false)
	s.Invalidate()
	return err
}

func (s *GitHubAppConfigService) buildManifest() gitHubAppManifest {
	base := s.opts.AppBaseURL
	return gitHubAppManifest{
		Name:           gitHubAppManifestName(base, randomHex(2)),
		URL:            base,
		HookAttributes: gitHubAppManifestHook{URL: base + "/api/git/webhook", Active: true},
		RedirectURL:    base + "/api/github/app-manifest/callback",
		SetupURL:       base + "/api/git/github/callback",
		SetupOnUpdate:  true,
		Description:    "Connects repositories to Helpin for branches, pull requests and delivery status.",
		Public:         false,
		DefaultPermissions: map[string]string{
			"metadata":      "read",
			"contents":      "write",
			"pull_requests": "write",
			"checks":        "read",
		},
		DefaultEvents: []string{"push", "pull_request", "release", "check_suite"},
	}
}

// gitHubAppManifestName builds a name that fits GitHub's 34-character limit and
// is unlikely to collide, since App names are global on GitHub.
func gitHubAppManifestName(appBaseURL, suffix string) string {
	host := ""
	if parsed, err := url.Parse(appBaseURL); err == nil {
		host = parsed.Hostname()
	}
	name := "Helpin"
	if host != "" {
		budget := _gitHubAppNameMaxLength - len("Helpin () ") - len(suffix)
		if len(host) > budget {
			host = strings.TrimRight(host[:budget], ".-")
		}
		name += " (" + host + ")"
	}
	if suffix != "" {
		name += " " + suffix
	}
	return name
}

func (s *GitHubAppConfigService) signManifestState(workspaceID, actorID, returnTo string) (string, error) {
	now := s.now()
	claims := gitHubAppManifestState{
		Purpose:     _gitHubAppManifestStatePurpose,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ReturnTo:    gitHubReturnTo(returnTo),
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(_gitHubAppManifestStateTTL)),
			ID:        randomHex(16),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.opts.StateSecret))
	if err != nil {
		return "", fmt.Errorf("sign github app manifest state: %w", err)
	}
	return signed, nil
}

func (s *GitHubAppConfigService) parseManifestState(stateToken string) (*gitHubAppManifestState, error) {
	stateToken = strings.TrimSpace(stateToken)
	if stateToken == "" || s.opts.StateSecret == "" {
		return nil, fmt.Errorf("github app manifest state is missing")
	}
	claims := &gitHubAppManifestState{}
	token, err := jwt.ParseWithClaims(stateToken, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.opts.StateSecret), nil
	}, jwt.WithTimeFunc(s.now))
	if err != nil || token == nil || !token.Valid {
		return nil, fmt.Errorf("invalid github app manifest state")
	}
	if claims.Purpose != _gitHubAppManifestStatePurpose || claims.WorkspaceID == "" || claims.ActorID == "" {
		return nil, fmt.Errorf("invalid github app manifest state")
	}
	return claims, nil
}

func (s *GitHubAppConfigService) isWorkspaceOwner(ctx context.Context, workspaceID, userID string) bool {
	if s.workspaces == nil {
		return false
	}
	role, err := s.workspaces.GetMemberRole(ctx, workspaceID, userID)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(role), model.RoleOwner)
}

// manifestRedirect returns the return_to page of the state's workspace with
// the github result flag, or the frontend result route when the workspace
// cannot be resolved.
func (s *GitHubAppConfigService) manifestRedirect(ctx context.Context, claims *gitHubAppManifestState, status, message string) string {
	query := gitHubResultQuery(status, message)
	if claims != nil && claims.WorkspaceID != "" && s.workspaces != nil {
		if workspace, err := s.workspaces.GetByID(ctx, claims.WorkspaceID); err == nil && workspace != nil && workspace.Slug != "" {
			return withGitHubReturnQuery(gitHubReturnPageURL(s.opts.AppBaseURL, workspace.Slug, claims.ReturnTo), query)
		}
	}
	return gitHubInstalledURL(s.opts.AppBaseURL, query)
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}
