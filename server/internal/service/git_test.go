package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSignGitHubInstallState(t *testing.T) {
	svc := &GitService{stateSecret: "test-secret"}

	tokenString, err := svc.signGitHubInstallState("ws-123", "user-456")
	if err != nil {
		t.Fatalf("signGitHubInstallState returned error: %v", err)
	}
	if tokenString == "" {
		t.Fatal("expected a token string")
	}

	claims := &gitHubInstallState{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if !token.Valid {
		t.Fatal("expected token to be valid")
	}
	if claims.WorkspaceID != "ws-123" {
		t.Fatalf("expected workspace claim ws-123, got %q", claims.WorkspaceID)
	}
	if claims.ActorID != "user-456" {
		t.Fatalf("expected actor claim user-456, got %q", claims.ActorID)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("expected expiration claim to be set")
	}
}

func TestWithGitHubInstallStatus(t *testing.T) {
	result := withGitHubInstallStatus(
		"http://localhost:5173/w/demo/settings/system?tab=delivery",
		"connected",
		"Connected successfully.",
		map[string]string{"integration_id": "abc123"},
	)

	parsed, err := url.Parse(result)
	if err != nil {
		t.Fatalf("parse result URL: %v", err)
	}
	if parsed.Path != "/w/demo/settings/system" {
		t.Fatalf("unexpected path %q", parsed.Path)
	}
	if parsed.Query().Get("tab") != "delivery" {
		t.Fatalf("expected existing query param to be preserved, got %q", parsed.Query().Get("tab"))
	}
	if parsed.Query().Get("github_app") != "connected" {
		t.Fatalf("expected github_app=connected, got %q", parsed.Query().Get("github_app"))
	}
	if !strings.Contains(parsed.Query().Get("github_message"), "Connected successfully") {
		t.Fatalf("expected github_message to be populated, got %q", parsed.Query().Get("github_message"))
	}
	if parsed.Query().Get("integration_id") != "abc123" {
		t.Fatalf("expected integration_id=abc123, got %q", parsed.Query().Get("integration_id"))
	}
}

func TestGetGitHubInstallURLReturnsInstallActionWithoutExistingIntegration(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE git_integrations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		provider TEXT NOT NULL,
		display_name TEXT NOT NULL,
		credential_mode TEXT NOT NULL DEFAULT 'github_app',
		account_login TEXT,
		base_url TEXT,
		installation_id TEXT,
		app_id TEXT,
		webhook_secret TEXT,
		access_token TEXT NOT NULL DEFAULT '',
		active BOOLEAN NOT NULL DEFAULT 1,
		last_synced_at DATETIME,
		last_sync_error TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create git_integrations table: %v", err)
	}

	workspaceRepo := repository.NewWorkspaceRepository(db)
	if err := db.Create(&model.Workspace{
		ID:      "ws-123",
		Name:    "Demo Workspace",
		Slug:    "demo",
		OwnerID: "user-456",
	}).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	appClient, err := githubapp.NewClient("12345", generateTestPrivateKeyPEM(t))
	if err != nil {
		t.Fatalf("create github app client: %v", err)
	}

	svc := &GitService{
		integrationRepo: repository.NewGitIntegrationRepository(db),
		workspaceRepo:   workspaceRepo,
		githubApp:       appClient,
		githubAppSlug:   "helpin-test",
		stateSecret:     "test-secret",
	}

	installURL, action, err := svc.GetGitHubInstallURL(context.Background(), "ws-123", "user-456")
	if err != nil {
		t.Fatalf("GetGitHubInstallURL returned error: %v", err)
	}
	if action != "install" {
		t.Fatalf("expected install action, got %q", action)
	}

	parsed, err := url.Parse(installURL)
	if err != nil {
		t.Fatalf("parse install url: %v", err)
	}
	if parsed.Host != "github.com" {
		t.Fatalf("expected github.com host, got %q", parsed.Host)
	}
	if parsed.Path != "/apps/helpin-test/installations/new" {
		t.Fatalf("unexpected install path %q", parsed.Path)
	}
	if parsed.Query().Get("state") == "" {
		t.Fatal("expected state query param to be set")
	}
}

func generateTestPrivateKeyPEM(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
}
