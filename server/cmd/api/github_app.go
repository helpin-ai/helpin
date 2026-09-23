package main

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// newGitHubAppConfigService builds the instance GitHub App source: GITHUB_APP_*
// when set, otherwise the App stored by the manifest flow.
func newGitHubAppConfigService(db *gorm.DB, cfg *config.Config) (*service.GitHubAppConfigService, error) {
	// Validate an environment-pinned key at startup, as before.
	if _, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey); err != nil {
		return nil, fmt.Errorf("GITHUB_APP_PRIVATE_KEY: %w", err)
	}
	return service.NewGitHubAppConfigService(
		repository.NewGitHubAppCredentialRepository(db),
		repository.NewWorkspaceRepository(db),
		githubapp.NewManifestClient("", nil),
		service.GitHubAppConfigOptions{
			Env: githubapp.Credentials{
				AppID:         cfg.GitHubAppID,
				Slug:          cfg.GitHubAppSlug,
				ClientID:      cfg.GitHubAppClientID,
				ClientSecret:  cfg.GitHubAppClientSecret,
				PrivateKey:    cfg.GitHubAppPrivateKey,
				WebhookSecret: cfg.GitHubAppWebhookSecret,
			},
			EncryptionKey:   resolveGitOAuthEncryptionKey(cfg),
			AppBaseURL:      cfg.AppBaseURL,
			StateSecret:     cfg.JWTSecret,
			ManifestEnabled: gitHubAppManifestEnabled,
		},
	), nil
}
