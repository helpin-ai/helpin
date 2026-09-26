package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// _gitHubAppCacheTTL bounds how long another process (API replica, Temporal
// worker) may keep serving stale App credentials after a write. Writes in the
// same process invalidate immediately.
const _gitHubAppCacheTTL = 30 * time.Second

type gitHubAppCredentialStore interface {
	Get(ctx context.Context) (*model.GitHubAppCredential, error)
	Create(ctx context.Context, credential *model.GitHubAppCredential, replace bool) error
}

// GitHubAppConfigOptions configures GitHubAppConfigService.
type GitHubAppConfigOptions struct {
	// Env holds the GITHUB_APP_* values. When AppID and PrivateKey are both
	// set the environment is authoritative and stored credentials are ignored.
	Env githubapp.Credentials
	// EncryptionKey is the 32-byte GIT_OAUTH_ENCRYPTION_KEY.
	EncryptionKey []byte
	// AppBaseURL is the public URL that serves the UI and /api.
	AppBaseURL string
	// StateSecret signs manifest state tokens.
	StateSecret string
	// ManifestEnabled allows creating the App from the UI (Community only).
	ManifestEnabled bool
	// CacheTTL overrides _gitHubAppCacheTTL; tests only.
	CacheTTL time.Duration
}

// _gitHubAppOwnerRetry bounds how often a failed GET /app lookup is retried
// for an environment-configured App.
const _gitHubAppOwnerRetry = 5 * time.Minute

// gitHubAppInstallRedirector sends the browser from App creation straight to
// the App's install page with signed Helpin install state.
type gitHubAppInstallRedirector interface {
	GitHubAppInstallRedirect(ctx context.Context, slug, workspaceID, actorID, returnTo string) (string, error)
}

// gitHubAppLookup reads the authenticated App (GET /app) to learn its owner.
type gitHubAppLookup interface {
	GetApp(ctx context.Context) (*githubapp.App, error)
}

// gitHubAppOwner identifies the GitHub account that owns the App.
type gitHubAppOwner struct {
	Login string
	Type  string
}

// GitHubAppConfigService resolves the instance GitHub App from the
// environment or the database and runs the App manifest flow. It implements
// githubapp.CredentialSource.
type GitHubAppConfigService struct {
	store      gitHubAppCredentialStore
	workspaces gitHubAppWorkspaceLookup
	converter  gitHubAppManifestConverter
	installer  gitHubAppInstallRedirector
	appLookup  gitHubAppLookup
	opts       GitHubAppConfigOptions
	now        func() time.Time

	mu          sync.Mutex
	cached      githubapp.Credentials
	cachedOwner gitHubAppOwner
	cachedAt    time.Time
	hasCache    bool

	envOwnerMu      sync.Mutex
	envOwner        gitHubAppOwner
	envOwnerAt      time.Time
	envOwnerChecked bool
}

var _ githubapp.CredentialSource = (*GitHubAppConfigService)(nil)

// NewGitHubAppConfigService returns the App credential source. store,
// workspaces and converter may be nil when only environment credentials are
// used (for example in one-off commands).
func NewGitHubAppConfigService(
	store gitHubAppCredentialStore,
	workspaces gitHubAppWorkspaceLookup,
	converter gitHubAppManifestConverter,
	opts GitHubAppConfigOptions,
) *GitHubAppConfigService {
	opts.Env = trimGitHubAppCredentials(opts.Env)
	opts.AppBaseURL = strings.TrimRight(strings.TrimSpace(opts.AppBaseURL), "/")
	opts.StateSecret = strings.TrimSpace(opts.StateSecret)
	opts.EncryptionKey = append([]byte(nil), opts.EncryptionKey...)
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = _gitHubAppCacheTTL
	}
	if !opts.Env.Usable() && (opts.Env.AppID != "" || opts.Env.PrivateKey != "" || opts.Env.Slug != "") {
		slog.Warn("GITHUB_APP_* is partially set; GITHUB_APP_ID and GITHUB_APP_PRIVATE_KEY are both required, using stored GitHub App credentials instead")
	}
	return &GitHubAppConfigService{
		store:      store,
		workspaces: workspaces,
		converter:  converter,
		opts:       opts,
		now:        time.Now,
	}
}

// Current returns the active App credentials: the environment when it is
// fully set, otherwise the stored App, otherwise zero Credentials.
func (s *GitHubAppConfigService) Current(ctx context.Context) (githubapp.Credentials, error) {
	creds, _, err := s.resolve(ctx)
	return creds, err
}

// SetInstallRedirector lets CompleteManifest continue to the App's install
// page. Without one, it returns to Helpin with a "created" result.
func (s *GitHubAppConfigService) SetInstallRedirector(installer gitHubAppInstallRedirector) *GitHubAppConfigService {
	s.installer = installer
	return s
}

// SetAppLookup lets Status report the owner of an environment-configured App.
func (s *GitHubAppConfigService) SetAppLookup(lookup gitHubAppLookup) *GitHubAppConfigService {
	s.appLookup = lookup
	return s
}

// Status describes the active App without secrets.
func (s *GitHubAppConfigService) Status(ctx context.Context) (*model.GitHubAppStatusResponse, error) {
	creds, source, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	status := &model.GitHubAppStatusResponse{
		Configured:        creds.Usable(),
		Source:            source,
		Slug:              creds.Slug,
		InstallURL:        creds.InstallURL(),
		WebhookConfigured: creds.WebhookSecret != "",
		ManifestAvailable: s.manifestAvailable(source),
	}
	switch source {
	case model.GitHubAppSourceDatabase:
		owner := s.storedOwner()
		status.OwnerLogin, status.OwnerType = owner.Login, owner.Type
		status.Private = true
	case model.GitHubAppSourceEnv:
		owner := s.environmentOwner(ctx)
		status.OwnerLogin, status.OwnerType = owner.Login, owner.Type
	case model.GitHubAppSourceNone:
		if s.opts.ManifestEnabled {
			status.ManifestBlockedReason = GitHubAppBaseURLBlockedReason(s.opts.AppBaseURL)
		}
	}
	return status, nil
}

// Invalidate drops cached stored credentials.
func (s *GitHubAppConfigService) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasCache = false
	s.cached = githubapp.Credentials{}
	s.cachedOwner = gitHubAppOwner{}
}

func (s *GitHubAppConfigService) storedOwner() gitHubAppOwner {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cachedOwner
}

// environmentOwner fetches GET /app once for an environment-configured App.
// Failures are retried after _gitHubAppOwnerRetry and report no owner.
func (s *GitHubAppConfigService) environmentOwner(ctx context.Context) gitHubAppOwner {
	if s.appLookup == nil {
		return gitHubAppOwner{}
	}
	s.envOwnerMu.Lock()
	defer s.envOwnerMu.Unlock()
	if s.envOwnerChecked && (s.envOwner.Login != "" || s.now().Sub(s.envOwnerAt) < _gitHubAppOwnerRetry) {
		return s.envOwner
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	app, err := s.appLookup.GetApp(lookupCtx)
	s.envOwnerChecked = true
	s.envOwnerAt = s.now()
	if err != nil || app == nil {
		slog.WarnContext(ctx, "github app owner lookup failed", "error", err)
		s.envOwner = gitHubAppOwner{}
		return s.envOwner
	}
	s.envOwner = gitHubAppOwner{Login: strings.TrimSpace(app.OwnerLogin), Type: strings.TrimSpace(app.OwnerType)}
	return s.envOwner
}

// Source reports where the active App comes from: env, database or none.
func (s *GitHubAppConfigService) Source(ctx context.Context) string {
	_, source, err := s.resolve(ctx)
	if err != nil {
		return model.GitHubAppSourceNone
	}
	return source
}

func (s *GitHubAppConfigService) resolve(ctx context.Context) (githubapp.Credentials, string, error) {
	if s.opts.Env.Usable() {
		return s.opts.Env, model.GitHubAppSourceEnv, nil
	}
	creds, err := s.stored(ctx)
	if err != nil {
		return githubapp.Credentials{}, model.GitHubAppSourceNone, err
	}
	if !creds.Usable() {
		return githubapp.Credentials{}, model.GitHubAppSourceNone, nil
	}
	return creds, model.GitHubAppSourceDatabase, nil
}

func (s *GitHubAppConfigService) stored(ctx context.Context) (githubapp.Credentials, error) {
	if s.store == nil {
		return githubapp.Credentials{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hasCache && s.now().Sub(s.cachedAt) < s.opts.CacheTTL {
		return s.cached, nil
	}
	row, err := s.store.Get(ctx)
	if err != nil {
		return githubapp.Credentials{}, err
	}
	creds, err := s.decrypt(row)
	if err != nil {
		return githubapp.Credentials{}, err
	}
	s.cached = creds
	s.cachedOwner = gitHubAppOwner{}
	if row != nil {
		s.cachedOwner = gitHubAppOwner{Login: strings.TrimSpace(derefString(row.OwnerLogin)), Type: strings.TrimSpace(derefString(row.OwnerType))}
	}
	s.cachedAt = s.now()
	s.hasCache = true
	return creds, nil
}

func (s *GitHubAppConfigService) decrypt(row *model.GitHubAppCredential) (githubapp.Credentials, error) {
	if row == nil {
		return githubapp.Credentials{}, nil
	}
	if len(s.opts.EncryptionKey) != 32 {
		return githubapp.Credentials{}, fmt.Errorf("GIT_OAUTH_ENCRYPTION_KEY is required to read the stored GitHub App")
	}
	privateKey, err := s.decryptField("private_key", &row.PrivateKeyEncrypted)
	if err != nil {
		return githubapp.Credentials{}, err
	}
	clientSecret, err := s.decryptField("client_secret", row.ClientSecretEncrypted)
	if err != nil {
		return githubapp.Credentials{}, err
	}
	webhookSecret, err := s.decryptField("webhook_secret", row.WebhookSecretEncrypted)
	if err != nil {
		return githubapp.Credentials{}, err
	}
	return trimGitHubAppCredentials(githubapp.Credentials{
		AppID:         row.AppID,
		Slug:          row.Slug,
		ClientID:      derefString(row.ClientID),
		ClientSecret:  clientSecret,
		PrivateKey:    privateKey,
		WebhookSecret: webhookSecret,
		HTMLURL:       derefString(row.HTMLURL),
	}), nil
}

func (s *GitHubAppConfigService) encryptField(field, value string) (*string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	ciphertext, err := appcrypto.EncryptStringWithAAD(value, s.opts.EncryptionKey, gitHubAppFieldAAD(field))
	if err != nil {
		return nil, fmt.Errorf("encrypt github app %s: %w", field, err)
	}
	return &ciphertext, nil
}

func (s *GitHubAppConfigService) decryptField(field string, ciphertext *string) (string, error) {
	if ciphertext == nil || strings.TrimSpace(*ciphertext) == "" {
		return "", nil
	}
	plaintext, err := appcrypto.DecryptStringWithAAD(*ciphertext, s.opts.EncryptionKey, gitHubAppFieldAAD(field))
	if err != nil {
		return "", fmt.Errorf("decrypt github app %s: %w", field, err)
	}
	return plaintext, nil
}

func (s *GitHubAppConfigService) manifestAvailable(source string) bool {
	return s.opts.ManifestEnabled &&
		source == model.GitHubAppSourceNone &&
		s.store != nil &&
		s.converter != nil &&
		len(s.opts.EncryptionKey) == 32 &&
		s.opts.StateSecret != "" &&
		s.opts.AppBaseURL != "" &&
		GitHubAppBaseURLBlockedReason(s.opts.AppBaseURL) == ""
}

func gitHubAppFieldAAD(field string) []byte {
	return []byte("github_app_credentials:" + field)
}

func trimGitHubAppCredentials(c githubapp.Credentials) githubapp.Credentials {
	return githubapp.Credentials{
		AppID:         strings.TrimSpace(c.AppID),
		Slug:          strings.TrimSpace(c.Slug),
		ClientID:      strings.TrimSpace(c.ClientID),
		ClientSecret:  strings.TrimSpace(c.ClientSecret),
		PrivateKey:    strings.TrimSpace(c.PrivateKey),
		WebhookSecret: strings.TrimSpace(c.WebhookSecret),
		HTMLURL:       strings.TrimSpace(c.HTMLURL),
	}
}
