package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// GitHub webhook verification errors. The handler maps
// ErrGitHubWebhookSignature to 401 and ErrGitHubWebhookUnknownInstallation to
// an ignored delivery.
var (
	ErrGitHubWebhookSignature           = errors.New("invalid github webhook signature")
	ErrGitHubWebhookUnknownInstallation = errors.New("no workspace matches github installation")
)

// SetGitHubAppSource makes the service read the App slug and webhook secret
// from source instead of the values fixed at construction.
func (s *GitService) SetGitHubAppSource(source githubapp.CredentialSource) *GitService {
	s.githubAppSource = source
	return s
}

// currentGitHubApp returns the active App credentials. Without a source it
// falls back to the slug passed to NewGitService (no webhook secret).
func (s *GitService) currentGitHubApp(ctx context.Context) githubapp.Credentials {
	if s.githubAppSource == nil {
		return githubapp.Credentials{Slug: s.githubAppSlug}
	}
	creds, err := s.githubAppSource.Current(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "resolve github app credentials", "error", err)
		return githubapp.Credentials{}
	}
	return creds
}

func (s *GitService) gitHubAppSlug(ctx context.Context) string {
	return strings.TrimSpace(s.currentGitHubApp(ctx).Slug)
}

// hasGitHubApp reports whether GitHub App API calls can currently succeed.
func (s *GitService) hasGitHubApp(ctx context.Context) bool {
	if s.githubApp == nil {
		return false
	}
	if configurable, ok := s.githubApp.(interface{ Configured(context.Context) bool }); ok {
		return configurable.Configured(ctx)
	}
	return true
}

// ResolveGitHubWebhookIntegration verifies a GitHub App webhook delivery and
// resolves the installation's integration. Deliveries that carry an
// installation are sent by the App and signed with the App's webhook secret
// (one per App), never with a per-integration secret, so the signature is
// checked before any lookup. Unsigned or wrongly signed deliveries fail with
// ErrGitHubWebhookSignature.
func (s *GitService) ResolveGitHubWebhookIntegration(ctx context.Context, installationID string, body []byte, signature string) (*model.GitIntegration, error) {
	installationID = strings.TrimSpace(installationID)
	if installationID == "" {
		return nil, fmt.Errorf("installation_id is required")
	}
	secret := s.currentGitHubApp(ctx).WebhookSecret
	if secret == "" {
		slog.WarnContext(ctx, "github app webhook rejected: webhook secret is not configured", "installation_id", installationID)
		return nil, fmt.Errorf("%w: github app webhook secret is not configured", ErrGitHubWebhookSignature)
	}
	if !githubapp.VerifyWebhookSignature(secret, body, signature) {
		return nil, ErrGitHubWebhookSignature
	}

	integration, err := s.integrationRepo.GetByInstallationID(ctx, "github", installationID)
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, fmt.Errorf("%w %s", ErrGitHubWebhookUnknownInstallation, installationID)
	}
	return integration, nil
}

// VerifyGitHubWebhookWithoutInstallation verifies a GitHub delivery that has
// no installation: an App-level event signed with the App secret, or a legacy
// repository webhook (?workspace_id=...) signed with the secret of one of that
// workspace's non-App GitHub integrations.
func (s *GitService) VerifyGitHubWebhookWithoutInstallation(ctx context.Context, workspaceID string, body []byte, signature string) error {
	if githubapp.VerifyWebhookSignature(s.currentGitHubApp(ctx).WebhookSecret, body, signature) {
		return nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" || s.integrationRepo == nil {
		return ErrGitHubWebhookSignature
	}
	integrations, err := s.integrationRepo.List(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, integration := range integrations {
		if integration.Provider != "github" || strings.EqualFold(strings.TrimSpace(integration.CredentialMode), "github_app") {
			continue
		}
		if integration.WebhookSecret == nil {
			continue
		}
		if githubapp.VerifyWebhookSignature(*integration.WebhookSecret, body, signature) {
			return nil
		}
	}
	return ErrGitHubWebhookSignature
}
