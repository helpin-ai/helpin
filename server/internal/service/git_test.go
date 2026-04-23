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
		"http://localhost:5173/w/demo/settings/delivery?tab=delivery",
		"connected",
		"Connected successfully.",
		map[string]string{"integration_id": "abc123"},
	)

	parsed, err := url.Parse(result)
	if err != nil {
		t.Fatalf("parse result URL: %v", err)
	}
	if parsed.Path != "/w/demo/settings/delivery" {
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
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS git_integrations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		organization_id TEXT NOT NULL,
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
		deleted_at DATETIME,
		last_synced_at DATETIME,
		last_sync_error TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create git_integrations table: %v", err)
	}

	workspaceRepo := repository.NewWorkspaceRepository(db)
	if err := db.Create(&model.Organization{
		ID:      "org-123",
		Name:    "Demo Org",
		Slug:    "demo-org",
		OwnerID: "user-456",
	}).Error; err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if err := db.Create(&model.Workspace{
		ID:      "ws-123",
		Name:    "Demo Workspace",
		Slug:    "demo",
		OwnerID: "user-456",
		OrganizationID: func() *string {
			value := "org-123"
			return &value
		}(),
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

	installURL, action, integrationID, err := svc.GetGitHubInstallURL(context.Background(), "ws-123", "user-456", false)
	if err != nil {
		t.Fatalf("GetGitHubInstallURL returned error: %v", err)
	}
	if action != "install" {
		t.Fatalf("expected install action, got %q", action)
	}
	if integrationID != nil {
		t.Fatalf("expected nil integration id, got %v", *integrationID)
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

func TestGetGitHubInstallURLReturnsPickReposForExistingOrgIntegration(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE git_integrations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		organization_id TEXT NOT NULL,
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
		deleted_at DATETIME,
		last_synced_at DATETIME,
		last_sync_error TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create git_integrations table: %v", err)
	}

	orgID := "org-123"
	if err := db.Create(&model.Organization{
		ID:      orgID,
		Name:    "Demo Org",
		Slug:    "demo-org",
		OwnerID: "user-456",
	}).Error; err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if err := db.Create(&model.Workspace{
		ID:             "ws-123",
		Name:           "Demo Workspace",
		Slug:           "demo",
		OwnerID:        "user-456",
		OrganizationID: &orgID,
	}).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := db.Create(&model.GitIntegration{
		ID:             "gi-123",
		WorkspaceID:    "ws-123",
		OrganizationID: &orgID,
		Provider:       "github",
		DisplayName:    "GitHub Demo",
		CredentialMode: "github_app",
		AccountLogin: func() *string {
			value := "demo-org"
			return &value
		}(),
		InstallationID: func() *string {
			value := "12345"
			return &value
		}(),
		Active: true,
	}).Error; err != nil {
		t.Fatalf("create integration: %v", err)
	}

	svc := &GitService{
		integrationRepo: repository.NewGitIntegrationRepository(db),
		workspaceRepo:   repository.NewWorkspaceRepository(db),
		githubApp: func() *githubapp.Client {
			client, err := githubapp.NewClient("12345", generateTestPrivateKeyPEM(t))
			if err != nil {
				t.Fatalf("create github app client: %v", err)
			}
			return client
		}(),
		githubAppSlug:   "helpin-test",
		stateSecret:     "test-secret",
	}

	installURL, action, integrationID, err := svc.GetGitHubInstallURL(context.Background(), "ws-123", "user-456", false)
	if err != nil {
		t.Fatalf("GetGitHubInstallURL returned error: %v", err)
	}
	if installURL != "https://github.com/organizations/demo-org/settings/installations/12345" {
		t.Fatalf("expected manage install url, got %q", installURL)
	}
	if action != "pick_repos" {
		t.Fatalf("expected pick_repos action, got %q", action)
	}
	if integrationID == nil || *integrationID != "gi-123" {
		t.Fatalf("expected integration id gi-123, got %v", integrationID)
	}
}

func TestGetGitHubInstallURLForceInstallBypassesExistingOrgIntegration(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE git_integrations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		organization_id TEXT NOT NULL,
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
		deleted_at DATETIME,
		last_synced_at DATETIME,
		last_sync_error TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create git_integrations table: %v", err)
	}

	orgID := "org-123"
	if err := db.Create(&model.Organization{
		ID:      orgID,
		Name:    "Demo Org",
		Slug:    "demo-org",
		OwnerID: "user-456",
	}).Error; err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if err := db.Create(&model.Workspace{
		ID:             "ws-123",
		Name:           "Demo Workspace",
		Slug:           "demo",
		OwnerID:        "user-456",
		OrganizationID: &orgID,
	}).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := db.Create(&model.GitIntegration{
		ID:             "gi-123",
		WorkspaceID:    "ws-123",
		OrganizationID: &orgID,
		Provider:       "github",
		DisplayName:    "GitHub Demo",
		CredentialMode: "github_app",
		InstallationID: func() *string {
			value := "12345"
			return &value
		}(),
		Active: true,
	}).Error; err != nil {
		t.Fatalf("create integration: %v", err)
	}

	appClient, err := githubapp.NewClient("12345", generateTestPrivateKeyPEM(t))
	if err != nil {
		t.Fatalf("create github app client: %v", err)
	}

	svc := &GitService{
		integrationRepo: repository.NewGitIntegrationRepository(db),
		workspaceRepo:   repository.NewWorkspaceRepository(db),
		githubApp:       appClient,
		githubAppSlug:   "helpin-test",
		stateSecret:     "test-secret",
	}

	installURL, action, integrationID, err := svc.GetGitHubInstallURL(context.Background(), "ws-123", "user-456", true)
	if err != nil {
		t.Fatalf("GetGitHubInstallURL returned error: %v", err)
	}
	if action != "install" {
		t.Fatalf("expected install action, got %q", action)
	}
	if integrationID != nil {
		t.Fatalf("expected nil integration id, got %v", integrationID)
	}
	parsed, err := url.Parse(installURL)
	if err != nil {
		t.Fatalf("parse install url: %v", err)
	}
	if parsed.Path != "/apps/helpin-test/installations/new" {
		t.Fatalf("unexpected install path %q", parsed.Path)
	}
	if parsed.Query().Get("state") == "" {
		t.Fatal("expected state query param to be set")
	}
}

func TestListIntegrationsReturnsOrgVisibleIntegrations(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE git_integrations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		organization_id TEXT,
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
		deleted_at DATETIME,
		last_synced_at DATETIME,
		last_sync_error TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create git_integrations table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS git_repositories (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		integration_id TEXT NOT NULL,
		provider TEXT NOT NULL,
		external_id TEXT NOT NULL,
		full_name TEXT NOT NULL,
		default_branch TEXT NOT NULL,
		permissions TEXT,
		private BOOLEAN NOT NULL DEFAULT 0,
		archived BOOLEAN NOT NULL DEFAULT 0,
		selected BOOLEAN NOT NULL DEFAULT 1,
		active BOOLEAN NOT NULL DEFAULT 1,
		deleted_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create git_repositories table: %v", err)
	}

	mustExec := func(query string, args ...interface{}) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}

	if err := db.Create(&model.Organization{
		ID:      "org-1",
		Name:    "Demo Org",
		Slug:    "demo-org",
		OwnerID: "user-1",
	}).Error; err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if err := db.Create(&model.Workspace{
		ID:             "ws-1",
		Name:           "Workspace One",
		Slug:           "workspace-one",
		OwnerID:        "user-1",
		OrganizationID: func() *string { value := "org-1"; return &value }(),
	}).Error; err != nil {
		t.Fatalf("create workspace one: %v", err)
	}
	if err := db.Create(&model.Workspace{
		ID:             "ws-2",
		Name:           "Workspace Two",
		Slug:           "workspace-two",
		OwnerID:        "user-1",
		OrganizationID: func() *string { value := "org-1"; return &value }(),
	}).Error; err != nil {
		t.Fatalf("create workspace two: %v", err)
	}

	mustExec(`INSERT INTO git_integrations (
		id, workspace_id, organization_id, provider, display_name, credential_mode, active, created_at, updated_at
	) VALUES
		('gi-self', 'ws-1', 'org-1', 'github', 'Workspace One', 'github_app', 1, datetime('now'), datetime('now')),
		('gi-shared', 'ws-2', 'org-1', 'github', 'Shared Org Install', 'github_app', 1, datetime('now'), datetime('now')),
		('gi-other', 'ws-2', 'org-1', 'github', 'Sibling Only', 'github_app', 1, datetime('now'), datetime('now'))`)
	mustExec(`INSERT INTO git_repositories (
		id, workspace_id, integration_id, provider, external_id, full_name, default_branch, active, created_at, updated_at
	) VALUES
		('repo-1', 'ws-1', 'gi-shared', 'github', '100', 'acme/shared', 'main', 1, datetime('now'), datetime('now')),
		('repo-2', 'ws-2', 'gi-other', 'github', '200', 'acme/other', 'main', 1, datetime('now'), datetime('now'))`)

	svc := &GitService{
		integrationRepo: repository.NewGitIntegrationRepository(db),
	}

	integrations, err := svc.ListIntegrations(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("ListIntegrations returned error: %v", err)
	}
	if len(integrations) != 3 {
		t.Fatalf("expected 3 integrations, got %d", len(integrations))
	}

	gotIDs := map[string]bool{}
	for _, integration := range integrations {
		gotIDs[integration.ID] = true
	}
	if !gotIDs["gi-self"] {
		t.Fatal("expected workspace-owned integration to be listed")
	}
	if !gotIDs["gi-shared"] {
		t.Fatal("expected shared integration with workspace repo claims to be listed")
	}
	if !gotIDs["gi-other"] {
		t.Fatal("expected sibling org integration to be listed")
	}
}

func TestListAvailableReposRejectsActorWithoutAdminRoleAnywhereInOrg(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE git_integrations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		organization_id TEXT NOT NULL,
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
		deleted_at DATETIME,
		last_synced_at DATETIME,
		last_sync_error TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create git_integrations table: %v", err)
	}

	orgID := "org-123"
	if err := db.Create(&model.Organization{
		ID:      orgID,
		Name:    "Demo Org",
		Slug:    "demo-org",
		OwnerID: "org-owner",
	}).Error; err != nil {
		t.Fatalf("create organization: %v", err)
	}
	seedUser(t, db, "viewer-1", "viewer@example.com", "Viewer", "hash")
	seedWorkspace(t, db, "ws-1", "Workspace One", "workspace-one", "org-owner")
	seedWorkspaceMember(t, db, "wm-viewer-1", "ws-1", "viewer-1", "viewer@example.com", "Viewer", model.RoleMember)
	mustExec := func(query string, args ...interface{}) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	mustExec(`UPDATE workspaces SET organization_id = ? WHERE id = ?`, orgID, "ws-1")
	mustExec(`INSERT INTO organization_members (id, organization_id, user_id, role, created_at, updated_at)
		VALUES (?, ?, ?, 'member', datetime('now'), datetime('now'))`, "om-viewer-1", orgID, "viewer-1")
	mustExec(`INSERT INTO git_integrations (
		id, workspace_id, organization_id, provider, display_name, credential_mode, installation_id, access_token, active, created_at, updated_at
	) VALUES (?, ?, ?, 'github', 'GitHub Demo', 'github_app', '12345', '', 1, datetime('now'), datetime('now'))`, "gi-1", "ws-1", orgID)

	svc := &GitService{
		integrationRepo: repository.NewGitIntegrationRepository(db),
		workspaceRepo:   repository.NewWorkspaceRepository(db),
		orgRepo:         repository.NewOrganizationRepository(db),
	}

	_, err := svc.ListAvailableRepos(context.Background(), "ws-1", "gi-1", "viewer-1")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "forbidden") {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestListAvailableReposAllowsAdminOnSiblingWorkspaceInOrg(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE git_integrations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		organization_id TEXT NOT NULL,
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
		deleted_at DATETIME,
		last_synced_at DATETIME,
		last_sync_error TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create git_integrations table: %v", err)
	}

	orgID := "org-456"
	if err := db.Create(&model.Organization{
		ID:      orgID,
		Name:    "Demo Org",
		Slug:    "demo-org-2",
		OwnerID: "org-owner",
	}).Error; err != nil {
		t.Fatalf("create organization: %v", err)
	}
	seedUser(t, db, "user-1", "user1@example.com", "User One", "hash")
	seedWorkspace(t, db, "ws-a", "Workspace A", "workspace-a", "org-owner")
	seedWorkspace(t, db, "ws-b", "Workspace B", "workspace-b", "org-owner")
	seedWorkspaceMember(t, db, "wm-a", "ws-a", "user-1", "user1@example.com", "User One", model.RoleMember)
	seedWorkspaceMember(t, db, "wm-b", "ws-b", "user-1", "user1@example.com", "User One", model.RoleAdmin)
	mustExec := func(query string, args ...interface{}) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	mustExec(`UPDATE workspaces SET organization_id = ? WHERE id IN (?, ?)`, orgID, "ws-a", "ws-b")
	mustExec(`INSERT INTO organization_members (id, organization_id, user_id, role, created_at, updated_at)
		VALUES (?, ?, ?, 'member', datetime('now'), datetime('now'))`, "om-user-1", orgID, "user-1")
	mustExec(`INSERT INTO git_integrations (
		id, workspace_id, organization_id, provider, display_name, credential_mode, installation_id, access_token, active, created_at, updated_at
	) VALUES (?, ?, ?, 'github', 'GitHub Demo', 'github_app', '12345', '', 1, datetime('now'), datetime('now'))`, "gi-1", "ws-a", orgID)

	svc := &GitService{
		integrationRepo: repository.NewGitIntegrationRepository(db),
		workspaceRepo:   repository.NewWorkspaceRepository(db),
		orgRepo:         repository.NewOrganizationRepository(db),
	}

	_, err := svc.ListAvailableRepos(context.Background(), "ws-a", "gi-1", "user-1")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "github app credentials are not configured") {
		t.Fatalf("expected auth to pass and github app config error to surface, got %v", err)
	}
}

func TestIntegrationHasWebhookClaimsUsesIntegrationScope(t *testing.T) {
	db := newTestDB(t)

	if err := db.Exec(`INSERT INTO git_repositories (
		id, workspace_id, integration_id, provider, external_id, full_name, default_branch,
		permissions, private, archived, selected, active, created_at, updated_at
	) VALUES (?, ?, ?, 'github', '101', 'acme/repo', 'main', '{}', 1, 0, 1, 1, datetime('now'), datetime('now'))`,
		"repo-1", "ws-1", "gi-1").Error; err != nil {
		t.Fatalf("insert repository: %v", err)
	}

	svc := &GitService{
		repoRepo: repository.NewGitRepositoryRepository(db),
	}

	hasClaims, err := svc.IntegrationHasWebhookClaims(context.Background(), "gi-1")
	if err != nil {
		t.Fatalf("IntegrationHasWebhookClaims returned error: %v", err)
	}
	if !hasClaims {
		t.Fatal("expected webhook claims for integration gi-1")
	}

	hasClaims, err = svc.IntegrationHasWebhookClaims(context.Background(), "gi-2")
	if err != nil {
		t.Fatalf("IntegrationHasWebhookClaims returned error: %v", err)
	}
	if hasClaims {
		t.Fatal("expected no webhook claims for integration gi-2")
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
