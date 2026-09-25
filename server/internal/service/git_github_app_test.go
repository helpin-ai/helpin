package service

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const _testAppWebhookSecret = "app-webhook-secret"

func setupGitHubWebhookTestService(t *testing.T, appSecret string) (*GitService, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	mustExec(t, db, `CREATE TABLE git_integrations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT,
		organization_id TEXT,
		provider TEXT NOT NULL,
		display_name TEXT NOT NULL,
		credential_mode TEXT NOT NULL DEFAULT 'github_app',
		credential_id TEXT,
		account_login TEXT,
		base_url TEXT,
		installation_id TEXT,
		app_id TEXT,
		webhook_secret TEXT,
		access_token TEXT NOT NULL DEFAULT '',
		default_commit_author_name TEXT,
		default_commit_author_email TEXT,
		active BOOLEAN NOT NULL DEFAULT 1,
		deleted_at DATETIME,
		last_synced_at DATETIME,
		last_sync_error TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, organization_id, timezone) VALUES ('ws-1', 'Acme', 'acme', 'user-1', 'org-1', 'UTC')`)
	// Integrations get a random per-integration secret at creation; App
	// deliveries are never signed with it.
	mustExec(t, db, `INSERT INTO git_integrations (id, workspace_id, organization_id, provider, display_name, credential_mode, installation_id, webhook_secret, active, created_at, updated_at)
		VALUES ('gi-app', 'ws-1', 'org-1', 'github', 'GitHub', 'github_app', '777', 'per-integration-random', 1, datetime('now'), datetime('now'))`)
	mustExec(t, db, `INSERT INTO git_integrations (id, workspace_id, organization_id, provider, display_name, credential_mode, webhook_secret, active, created_at, updated_at)
		VALUES ('gi-legacy', 'ws-1', 'org-1', 'github', 'Legacy hook', 'repository_webhook', 'legacy-secret', 1, datetime('now'), datetime('now'))`)

	svc := &GitService{integrationRepo: repository.NewGitIntegrationRepository(db)}
	svc.SetGitHubAppSource(githubapp.StaticSource(githubapp.Credentials{
		AppID:         "1",
		Slug:          "helpin",
		PrivateKey:    "unused",
		WebhookSecret: appSecret,
	}))
	return svc, db
}

func TestResolveGitHubWebhookIntegrationUsesAppSecret(t *testing.T) {
	body := []byte(`{"action":"opened","installation":{"id":777}}`)
	tests := []struct {
		name      string
		appSecret string
		signature string
		wantID    string
		wantErr   error
	}{
		{name: "valid app signature", appSecret: _testAppWebhookSecret, signature: githubapp.SignWebhookBody(_testAppWebhookSecret, body), wantID: "gi-app"},
		{name: "per-integration secret is rejected", appSecret: _testAppWebhookSecret, signature: githubapp.SignWebhookBody("per-integration-random", body), wantErr: ErrGitHubWebhookSignature},
		{name: "wrong secret", appSecret: _testAppWebhookSecret, signature: githubapp.SignWebhookBody("other", body), wantErr: ErrGitHubWebhookSignature},
		{name: "missing signature", appSecret: _testAppWebhookSecret, signature: "", wantErr: ErrGitHubWebhookSignature},
		{name: "malformed signature", appSecret: _testAppWebhookSecret, signature: "sha1=abc", wantErr: ErrGitHubWebhookSignature},
		{name: "app secret not configured", appSecret: "", signature: githubapp.SignWebhookBody("", body), wantErr: ErrGitHubWebhookSignature},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := setupGitHubWebhookTestService(t, tt.appSecret)
			integration, err := svc.ResolveGitHubWebhookIntegration(context.Background(), "777", body, tt.signature)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got integration=%v err=%v", tt.wantErr, integration, err)
				}
				return
			}
			if err != nil || integration == nil || integration.ID != tt.wantID {
				t.Fatalf("expected integration %s, got %v err=%v", tt.wantID, integration, err)
			}
		})
	}
}

func TestResolveGitHubWebhookIntegrationUnknownInstallation(t *testing.T) {
	svc, _ := setupGitHubWebhookTestService(t, _testAppWebhookSecret)
	body := []byte(`{"installation":{"id":999}}`)
	_, err := svc.ResolveGitHubWebhookIntegration(context.Background(), "999", body, githubapp.SignWebhookBody(_testAppWebhookSecret, body))
	if !errors.Is(err, ErrGitHubWebhookUnknownInstallation) {
		t.Fatalf("expected unknown installation, got %v", err)
	}
}

func TestVerifyGitHubWebhookWithoutInstallation(t *testing.T) {
	body := []byte(`{"zen":"Keep it logically awesome.","repository":{"full_name":"acme/repo"}}`)
	tests := []struct {
		name        string
		workspaceID string
		signature   string
		wantErr     bool
	}{
		{name: "app-level delivery", signature: githubapp.SignWebhookBody(_testAppWebhookSecret, body)},
		{name: "legacy repository webhook", workspaceID: "ws-1", signature: githubapp.SignWebhookBody("legacy-secret", body)},
		{name: "app integration secret is not a repository webhook secret", workspaceID: "ws-1", signature: githubapp.SignWebhookBody("per-integration-random", body), wantErr: true},
		{name: "legacy secret without workspace", signature: githubapp.SignWebhookBody("legacy-secret", body), wantErr: true},
		{name: "unsigned", workspaceID: "ws-1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := setupGitHubWebhookTestService(t, _testAppWebhookSecret)
			err := svc.VerifyGitHubWebhookWithoutInstallation(context.Background(), tt.workspaceID, body, tt.signature)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestGitServiceReadsSlugFromSource(t *testing.T) {
	svc := &GitService{githubAppSlug: "static"}
	if got := svc.gitHubAppSlug(context.Background()); got != "static" {
		t.Fatalf("expected constructor slug without source, got %q", got)
	}
	svc.SetGitHubAppSource(githubapp.StaticSource(githubapp.Credentials{Slug: "from-source"}))
	if got := svc.gitHubAppSlug(context.Background()); got != "from-source" {
		t.Fatalf("expected source slug, got %q", got)
	}
}
