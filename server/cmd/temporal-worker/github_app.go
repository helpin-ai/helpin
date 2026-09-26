package main

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// newGitHubAppConfigService resolves the instance GitHub App from GITHUB_APP_*
// or, when unset, from the App stored by the API's manifest flow.
func newGitHubAppConfigService(db *gorm.DB, cfg *config.Config) (*service.GitHubAppConfigService, error) {
	if _, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey); err != nil {
		return nil, fmt.Errorf("GITHUB_APP_PRIVATE_KEY: %w", err)
	}
	return service.NewGitHubAppConfigService(
		repository.NewGitHubAppCredentialRepository(db),
		nil,
		nil,
		service.GitHubAppConfigOptions{
			Env: githubapp.Credentials{
				AppID:         cfg.GitHubAppID,
				Slug:          cfg.GitHubAppSlug,
				ClientID:      cfg.GitHubAppClientID,
				ClientSecret:  cfg.GitHubAppClientSecret,
				PrivateKey:    cfg.GitHubAppPrivateKey,
				WebhookSecret: cfg.GitHubAppWebhookSecret,
			},
			EncryptionKey: resolveGitOAuthEncryptionKey(cfg),
			AppBaseURL:    cfg.AppBaseURL,
		},
	), nil
}
