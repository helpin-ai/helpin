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

// GitHubAppConfigService resolves the instance GitHub App from the
// environment or the database and runs the App manifest flow. It implements
// githubapp.CredentialSource.
type GitHubAppConfigService struct {
	store      gitHubAppCredentialStore
	workspaces gitHubAppWorkspaceLookup
	converter  gitHubAppManifestConverter
	opts       GitHubAppConfigOptions
	now        func() time.Time

	mu       sync.Mutex
	cached   githubapp.Credentials
	cachedAt time.Time
	hasCache bool
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

// Status describes the active App without secrets.
func (s *GitHubAppConfigService) Status(ctx context.Context) (*model.GitHubAppStatusResponse, error) {
	creds, source, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return &model.GitHubAppStatusResponse{
		Configured:        creds.Usable(),
		Source:            source,
		Slug:              creds.Slug,
		InstallURL:        creds.InstallURL(),
		WebhookConfigured: creds.WebhookSecret != "",
		ManifestAvailable: s.manifestAvailable(source),
	}, nil
}

// Invalidate drops cached stored credentials.
func (s *GitHubAppConfigService) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasCache = false
	s.cached = githubapp.Credentials{}
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
		s.opts.AppBaseURL != ""
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
