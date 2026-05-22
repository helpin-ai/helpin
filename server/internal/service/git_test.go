package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestSignGitHubInstallState(t *testing.T) {
	svc := &GitService{stateSecret: "test-secret"}

	tokenString, err := svc.signGitHubInstallState("org-123", "ws-123", "user-456")
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
	if claims.OrganizationID != "org-123" {
		t.Fatalf("expected organization claim org-123, got %q", claims.OrganizationID)
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
		credential_id TEXT,
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
	if _, err := repository.NewOrganizationRepository(db).AddMember(context.Background(), "org-123", "user-456", model.RoleOwner); err != nil {
		t.Fatalf("create org member: %v", err)
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
		orgRepo:         repository.NewOrganizationRepository(db),
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
		credential_id TEXT,
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
	if _, err := repository.NewOrganizationRepository(db).AddMember(context.Background(), orgID, "user-456", model.RoleOwner); err != nil {
		t.Fatalf("create org member: %v", err)
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
		WorkspaceID:    strPtr("ws-123"),
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
		orgRepo:         repository.NewOrganizationRepository(db),
		githubApp: func() *githubapp.Client {
			client, err := githubapp.NewClient("12345", generateTestPrivateKeyPEM(t))
			if err != nil {
				t.Fatalf("create github app client: %v", err)
			}
			return client
		}(),
		githubAppSlug: "helpin-test",
		stateSecret:   "test-secret",
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
		credential_id TEXT,
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
	if _, err := repository.NewOrganizationRepository(db).AddMember(context.Background(), orgID, "user-456", model.RoleOwner); err != nil {
		t.Fatalf("create org member: %v", err)
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
		WorkspaceID:    strPtr("ws-123"),
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
		orgRepo:         repository.NewOrganizationRepository(db),
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
		credential_id TEXT,
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
		base_url TEXT,
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
		credential_id TEXT,
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
		credential_id TEXT,
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

func TestResolveGitLabWebhookRepositoriesChecksAllCandidateIntegrations(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS git_integrations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT,
		organization_id TEXT,
		provider TEXT NOT NULL,
		display_name TEXT NOT NULL,
		credential_mode TEXT NOT NULL DEFAULT 'oauth_user',
		credential_id TEXT,
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
	mustExec := func(query string, args ...interface{}) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	mustExec(`INSERT INTO git_integrations (
		id, workspace_id, organization_id, provider, display_name, credential_mode, webhook_secret, access_token, active, created_at, updated_at
	) VALUES
		('gl-1', 'ws-1', 'org-1', 'gitlab', 'GitLab One', 'oauth_user', 'secret-one', '', 1, datetime('now'), datetime('now')),
		('gl-2', 'ws-2', 'org-2', 'gitlab', 'GitLab Two', 'oauth_user', 'secret-two', '', 1, datetime('now'), datetime('now'))`)
	mustExec(`INSERT INTO git_repositories (
		id, workspace_id, integration_id, provider, external_id, full_name, default_branch,
		permissions, private, archived, selected, active, created_at, updated_at
	) VALUES
		('repo-1', 'ws-1', 'gl-1', 'gitlab', '101', 'acme/shared', 'main', CAST('{}' AS BLOB), 1, 0, 1, 1, datetime('now'), datetime('now')),
		('repo-2', 'ws-2', 'gl-2', 'gitlab', '101', 'acme/shared', 'main', CAST('{}' AS BLOB), 1, 0, 1, 1, datetime('now'), datetime('now'))`)

	svc := &GitService{
		integrationRepo: repository.NewGitIntegrationRepository(db),
		repoRepo:        repository.NewGitRepositoryRepository(db),
	}

	integration, repos, err := svc.ResolveGitLabWebhookRepositories(context.Background(), "101", "secret-two")
	if err != nil {
		t.Fatalf("ResolveGitLabWebhookRepositories returned error: %v", err)
	}
	if integration == nil || integration.ID != "gl-2" {
		t.Fatalf("expected matching integration gl-2, got %#v", integration)
	}
	if len(repos) != 1 || repos[0].ID != "repo-2" {
		t.Fatalf("expected repo-2 only, got %#v", repos)
	}

	_, _, err = svc.ResolveGitLabWebhookRepositories(context.Background(), "101", "wrong-secret")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "invalid gitlab webhook token") {
		t.Fatalf("expected invalid token error, got %v", err)
	}
}

func TestHandleInstallationLifecycleEventRemovesAllWorkspaceClaims(t *testing.T) {
	db := newTestDB(t)
	mustExec := func(query string, args ...interface{}) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	mustExec(`INSERT INTO git_repositories (
		id, workspace_id, integration_id, provider, external_id, full_name, default_branch,
		permissions, private, archived, selected, active, created_at, updated_at
	) VALUES
		('repo-a', 'ws-a', 'gi-1', 'github', '101', 'acme/shared', 'main', CAST('{}' AS BLOB), 1, 0, 1, 1, datetime('now'), datetime('now')),
		('repo-b', 'ws-b', 'gi-1', 'github', '101', 'acme/shared', 'main', CAST('{}' AS BLOB), 1, 0, 1, 1, datetime('now'), datetime('now')),
		('repo-c', 'ws-c', 'gi-1', 'github', '202', 'acme/other', 'main', CAST('{}' AS BLOB), 1, 0, 1, 1, datetime('now'), datetime('now')),
		('repo-d', 'ws-d', 'gi-2', 'github', '101', 'acme/shared', 'main', CAST('{}' AS BLOB), 1, 0, 1, 1, datetime('now'), datetime('now'))`)

	svc := &GitService{repoRepo: repository.NewGitRepositoryRepository(db)}
	integration := &model.GitIntegration{ID: "gi-1", Active: true}
	if err := svc.HandleInstallationLifecycleEvent(context.Background(), integration, "installation_repositories", "removed", []string{"101"}); err != nil {
		t.Fatalf("HandleInstallationLifecycleEvent returned error: %v", err)
	}

	var removedCount int64
	if err := db.Table("git_repositories").
		Where("integration_id = ? AND external_id = ? AND active = ? AND deleted_at IS NULL", "gi-1", "101", true).
		Count(&removedCount).Error; err != nil {
		t.Fatalf("count removed repos: %v", err)
	}
	if removedCount != 0 {
		t.Fatalf("expected all gi-1/101 claims removed, got %d active", removedCount)
	}

	var retainedCount int64
	if err := db.Table("git_repositories").
		Where("id IN (?, ?) AND active = ? AND deleted_at IS NULL", "repo-c", "repo-d", true).
		Count(&retainedCount).Error; err != nil {
		t.Fatalf("count retained repos: %v", err)
	}
	if retainedCount != 2 {
		t.Fatalf("expected unrelated repo claims retained, got %d active", retainedCount)
	}
}

type fakeGitHubAppClient struct {
	mergeCalls     []fakeGitHubMergeCall
	mergeErr       error
	pullRequests   map[int]*githubapp.PullRequest
	pullRequestErr error
}

type fakeGitHubMergeCall struct {
	InstallationID string
	Owner          string
	Repo           string
	Base           string
	Head           string
	CommitMessage  string
}

func (f *fakeGitHubAppClient) ListInstallationRepositories(context.Context, string) ([]githubapp.Repository, error) {
	return nil, nil
}

func (f *fakeGitHubAppClient) ListRepositoryBranches(context.Context, string, string, string) ([]githubapp.Branch, error) {
	return nil, nil
}

func (f *fakeGitHubAppClient) GetReleaseByTag(context.Context, string, string, string, string) (*githubapp.Release, error) {
	return nil, nil
}

func (f *fakeGitHubAppClient) ListReleases(context.Context, string, string, string, githubapp.ListReleasesOptions) ([]githubapp.Release, error) {
	return nil, nil
}

func (f *fakeGitHubAppClient) GetInstallation(context.Context, string) (*githubapp.Installation, error) {
	return nil, nil
}

func (f *fakeGitHubAppClient) GetPullRequest(_ context.Context, _ string, _ string, _ string, number int) (*githubapp.PullRequest, error) {
	if f.pullRequestErr != nil {
		return nil, f.pullRequestErr
	}
	if f.pullRequests != nil {
		if pr, ok := f.pullRequests[number]; ok {
			return pr, nil
		}
	}
	return &githubapp.PullRequest{Number: number, State: "open"}, nil
}

func (f *fakeGitHubAppClient) MergeBranch(_ context.Context, installationID, owner, repo, base, head, commitMessage string) error {
	f.mergeCalls = append(f.mergeCalls, fakeGitHubMergeCall{
		InstallationID: installationID,
		Owner:          owner,
		Repo:           repo,
		Base:           base,
		Head:           head,
		CommitMessage:  commitMessage,
	})
	return f.mergeErr
}

func ensureGitDeliveryStatusTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS git_integrations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
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
			active BOOLEAN NOT NULL DEFAULT 1,
			deleted_at DATETIME,
			last_synced_at DATETIME,
			last_sync_error TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS pm_team_repo_defaults (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			repository_id TEXT NOT NULL,
			base_branch TEXT NOT NULL DEFAULT 'main',
			branch_template TEXT NOT NULL DEFAULT '{task_key}-{slug}',
			auto_sync_states BOOLEAN NOT NULL DEFAULT 1,
			review_state_id TEXT,
			done_state_id TEXT,
			closed_state_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS automation_rules (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			team_id TEXT,
			workflow_id TEXT,
			trigger_type TEXT NOT NULL,
			trigger_config TEXT NOT NULL DEFAULT '{}',
			action_type TEXT NOT NULL,
			action_config TEXT NOT NULL DEFAULT '{}',
			position INTEGER NOT NULL DEFAULT 0,
			stop_on_match BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS task_delivery_targets (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			task_id TEXT NOT NULL UNIQUE,
			repository_id TEXT,
			repo_full_name TEXT,
			integration_id TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_state TEXT NOT NULL DEFAULT 'unconfigured',
			active_pr_number INTEGER,
			active_pr_title TEXT,
			active_pr_url TEXT,
			active_pr_status TEXT,
			last_commit_sha TEXT,
			last_run_id TEXT,
			last_synced_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS task_git_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			integration_id TEXT NOT NULL,
			repository_id TEXT,
			run_id TEXT,
			provider TEXT NOT NULL,
			base_url TEXT,
			repo TEXT NOT NULL,
			branch TEXT,
			pr_number INTEGER,
			pr_title TEXT,
			pr_url TEXT,
			pr_status TEXT,
			commit_sha TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create git delivery status table: %v", err)
		}
	}
}

func seedGitDeliveryStatusFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	ensureGitDeliveryStatusTables(t, db)
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO git_integrations (
		id, workspace_id, provider, display_name, credential_mode, installation_id, access_token, active, created_at, updated_at
	) VALUES (?, ?, 'github', 'GitHub', 'github_app', ?, '', 1, ?, ?)`,
		"gi-1", "ws-1", "inst-1", now, now)
	mustExec(t, db, `INSERT INTO git_repositories (
		id, workspace_id, integration_id, provider, external_id, full_name, default_branch, active, created_at, updated_at
	) VALUES (?, ?, ?, 'github', '101', 'acme/api', 'main', 1, ?, ?)`,
		"repo-1", "ws-1", "gi-1", now, now)
	mustExec(t, db, `INSERT INTO pm_team_repo_defaults (
		id, team_id, repository_id, base_branch, branch_template, auto_sync_states, review_state_id, done_state_id, closed_state_id, created_at, updated_at
	) VALUES (?, ?, ?, 'main', '{task_key}-{slug}', 1, ?, ?, ?, ?, ?)`,
		"trd-1", "team-1", "repo-1", "state-review", "state-done", "state-closed", now, now)
	mustExec(t, db, `INSERT INTO pm_tasks (
		id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, team_id, priority, severity, position, created_at, updated_at
	) VALUES (?, ?, ?, ?, 'feature', ?, ?, ?, 'none', 'none', 0, ?, ?)`,
		"task-1", "ws-1", 31, "Fix merge status", "wf-1", "state-review", "team-1", now, now)
	mustExec(t, db, `INSERT INTO task_delivery_targets (
		id, workspace_id, task_id, repository_id, repo_full_name, integration_id, base_branch, working_branch, delivery_state,
		active_pr_number, active_pr_title, active_pr_url, active_pr_status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, 'main', ?, 'pr_open', 42, 'Fix merge status', 'https://github.test/acme/api/pull/42', 'open', ?, ?)`,
		"target-1", "ws-1", "task-1", "repo-1", "acme/api", "gi-1", "hel-31-fix-merge-status", now, now)
	mustExec(t, db, `INSERT INTO task_git_links (
		id, workspace_id, task_id, integration_id, repository_id, provider, repo, branch, pr_number, pr_title, pr_url, pr_status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, 'github', ?, ?, 42, 'Fix merge status', 'https://github.test/acme/api/pull/42', 'open', ?, ?)`,
		"link-1", "ws-1", "task-1", "gi-1", "repo-1", "acme/api", "hel-31-fix-merge-status", now, now)
	mustExec(t, db, `INSERT INTO task_git_links (
		id, workspace_id, task_id, integration_id, repository_id, provider, repo, branch, pr_number, pr_title, pr_url, pr_status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, 'github', ?, ?, 7, 'Old PR', 'https://github.test/acme/api/pull/7', 'open', ?, ?)`,
		"link-old", "ws-1", "task-1", "gi-1", "repo-1", "acme/api", "old-branch", now, now)
}

func newGitDeliveryStatusService(db *gorm.DB, app gitHubAppClient) *GitService {
	return &GitService{
		integrationRepo: repository.NewGitIntegrationRepository(db),
		repoRepo:        repository.NewGitRepositoryRepository(db),
		linkRepo:        repository.NewTaskGitLinkRepository(db),
		deliveryRepo:    repository.NewTaskDeliveryTargetRepository(db),
		settingsRepo:    repository.NewSettingsRepository(db),
		taskRepo:        repository.NewPMTaskRepository(db),
		githubApp:       app,
	}
}

func TestUpdateDeliveryStatusAfterMergeUpdatesTargetLinkAndTaskState(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)

	svc := newGitDeliveryStatusService(db, nil)
	if err := svc.UpdateDeliveryStatusAfterMerge(context.Background(), "ws-1", "task-1", "merged"); err != nil {
		t.Fatalf("UpdateDeliveryStatusAfterMerge returned error: %v", err)
	}

	target, err := repository.NewTaskDeliveryTargetRepository(db).GetByTask(context.Background(), "ws-1", "task-1")
	if err != nil {
		t.Fatalf("load target: %v", err)
	}
	if target == nil || target.ActivePRStatus == nil || *target.ActivePRStatus != "merged" {
		t.Fatalf("active PR status = %#v, want merged", target)
	}
	if target.DeliveryState != "merged" {
		t.Fatalf("delivery state = %q, want merged", target.DeliveryState)
	}
	if target.LastSyncedAt == nil {
		t.Fatal("expected last_synced_at to be set")
	}

	var currentStatus string
	if err := db.Raw(`SELECT pr_status FROM task_git_links WHERE id = ?`, "link-1").Scan(&currentStatus).Error; err != nil {
		t.Fatalf("load current link status: %v", err)
	}
	if currentStatus != "merged" {
		t.Fatalf("current link pr_status = %q, want merged", currentStatus)
	}
	var oldStatus string
	if err := db.Raw(`SELECT pr_status FROM task_git_links WHERE id = ?`, "link-old").Scan(&oldStatus).Error; err != nil {
		t.Fatalf("load old link status: %v", err)
	}
	if oldStatus != "open" {
		t.Fatalf("old link pr_status = %q, want open", oldStatus)
	}

	task, err := repository.NewPMTaskRepository(db).GetRawByID(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.WorkflowStateID != "state-done" {
		t.Fatalf("workflow_state_id = %q, want state-done", task.WorkflowStateID)
	}
}

func TestProcessWebhookPRClosedUpdatesTargetLinkDeliveryTargetAndTaskState(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)

	svc := newGitDeliveryStatusService(db, nil)
	if err := svc.ProcessWebhookPR(context.Background(), "ws-1", "acme/api", "closed", 42, "Fix merge status", "https://github.test/acme/api/pull/42", "closed", "hel-31-fix-merge-status", "main"); err != nil {
		t.Fatalf("ProcessWebhookPR returned error: %v", err)
	}

	target, err := repository.NewTaskDeliveryTargetRepository(db).GetByTask(context.Background(), "ws-1", "task-1")
	if err != nil {
		t.Fatalf("load target: %v", err)
	}
	if target.ActivePRStatus == nil || *target.ActivePRStatus != "closed" {
		t.Fatalf("active PR status = %#v, want closed", target.ActivePRStatus)
	}
	if target.DeliveryState != "closed" {
		t.Fatalf("delivery state = %q, want closed", target.DeliveryState)
	}

	var linkStatus string
	if err := db.Raw(`SELECT pr_status FROM task_git_links WHERE id = ?`, "link-1").Scan(&linkStatus).Error; err != nil {
		t.Fatalf("load link status: %v", err)
	}
	if linkStatus != "closed" {
		t.Fatalf("link pr_status = %q, want closed", linkStatus)
	}

	task, err := repository.NewPMTaskRepository(db).GetRawByID(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.WorkflowStateID != "state-closed" {
		t.Fatalf("workflow_state_id = %q, want state-closed", task.WorkflowStateID)
	}
}

func TestProcessWebhookPRClosedWithoutClosedStateDoesNotTransitionTask(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)
	mustExec(t, db, `UPDATE pm_team_repo_defaults SET closed_state_id = NULL WHERE id = ?`, "trd-1")

	svc := newGitDeliveryStatusService(db, nil)
	if err := svc.ProcessWebhookPR(context.Background(), "ws-1", "acme/api", "closed", 42, "Fix merge status", "https://github.test/acme/api/pull/42", "closed", "hel-31-fix-merge-status", "main"); err != nil {
		t.Fatalf("ProcessWebhookPR returned error: %v", err)
	}

	task, err := repository.NewPMTaskRepository(db).GetRawByID(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.WorkflowStateID != "state-review" {
		t.Fatalf("workflow_state_id = %q, want unchanged state-review", task.WorkflowStateID)
	}
}

func TestProcessWebhookPRMergedUpdatesTargetLinkAndTaskState(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)

	svc := newGitDeliveryStatusService(db, nil)
	if err := svc.ProcessWebhookPR(context.Background(), "ws-1", "acme/api", "closed", 42, "Fix merge status", "https://github.test/acme/api/pull/42", "merged", "hel-31-fix-merge-status", "main"); err != nil {
		t.Fatalf("ProcessWebhookPR returned error: %v", err)
	}

	assertMergedDeliveryStatus(t, db)
	task, err := repository.NewPMTaskRepository(db).GetRawByID(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.WorkflowStateID != "state-done" {
		t.Fatalf("workflow_state_id = %q, want state-done", task.WorkflowStateID)
	}
}

func TestReconcileOpenPullRequestStatusesUpdatesMergedPR(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)

	app := &fakeGitHubAppClient{pullRequests: map[int]*githubapp.PullRequest{
		42: {
			Number:  42,
			Title:   "Fix merge status",
			HTMLURL: "https://github.test/acme/api/pull/42",
			State:   "closed",
			Merged:  true,
		},
		7: {
			Number:  7,
			Title:   "Old PR",
			HTMLURL: "https://github.test/acme/api/pull/7",
			State:   "open",
		},
	}}
	svc := newGitDeliveryStatusService(db, app)

	dryRun, err := svc.ReconcileOpenPullRequestStatuses(context.Background(), 100, true)
	if err != nil {
		t.Fatalf("dry-run reconcile returned error: %v", err)
	}
	if dryRun.Checked != 2 || dryRun.Updated != 1 || dryRun.Merged != 1 || !dryRun.DryRun {
		t.Fatalf("dry-run result = %#v, want checked=2 updated=1 merged=1 dry_run=true", dryRun)
	}
	var dryRunStatus string
	if err := db.Raw(`SELECT pr_status FROM task_git_links WHERE id = ?`, "link-1").Scan(&dryRunStatus).Error; err != nil {
		t.Fatalf("load dry-run status: %v", err)
	}
	if dryRunStatus != "open" {
		t.Fatalf("dry-run changed status to %q, want open", dryRunStatus)
	}

	result, err := svc.ReconcileOpenPullRequestStatuses(context.Background(), 100, false)
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if result.Checked != 2 || result.Updated != 1 || result.Merged != 1 || result.StillOpen != 1 {
		t.Fatalf("result = %#v, want checked=2 updated=1 merged=1 still_open=1", result)
	}

	target, err := repository.NewTaskDeliveryTargetRepository(db).GetByTask(context.Background(), "ws-1", "task-1")
	if err != nil {
		t.Fatalf("load target: %v", err)
	}
	if target == nil || target.ActivePRStatus == nil || *target.ActivePRStatus != "merged" {
		t.Fatalf("active PR status = %#v, want merged", target)
	}
	if target.DeliveryState != "merged" {
		t.Fatalf("delivery state = %q, want merged", target.DeliveryState)
	}
	var currentStatus string
	if err := db.Raw(`SELECT pr_status FROM task_git_links WHERE id = ?`, "link-1").Scan(&currentStatus).Error; err != nil {
		t.Fatalf("load current link status: %v", err)
	}
	if currentStatus != "merged" {
		t.Fatalf("current link status = %q, want merged", currentStatus)
	}
	var oldStatus string
	if err := db.Raw(`SELECT pr_status FROM task_git_links WHERE id = ?`, "link-old").Scan(&oldStatus).Error; err != nil {
		t.Fatalf("load old link status: %v", err)
	}
	if oldStatus != "open" {
		t.Fatalf("old link status = %q, want open", oldStatus)
	}
}

func TestDeliveryStateForPRStatusClosed(t *testing.T) {
	if got := deliveryStateForPRStatus("closed"); got != "closed" {
		t.Fatalf("deliveryStateForPRStatus(%q) = %q, want closed", "closed", got)
	}
}

type fakeAutomationHealthObserver struct {
	failures []string
}

func (o *fakeAutomationHealthObserver) ObserveSuccess(ctx context.Context, workspaceID, catalogID, scopeType, scopeID string, metrics model.JSONB) error {
	return nil
}

func (o *fakeAutomationHealthObserver) ObserveFailure(ctx context.Context, workspaceID, catalogID, scopeType, scopeID, message string, metrics model.JSONB) error {
	if ruleID, ok := metrics["rule_id"].(string); ok {
		o.failures = append(o.failures, ruleID)
	}
	return nil
}

func TestProcessWebhookPRClosedFiresAutomationTrigger(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO automation_rules (
		id, workspace_id, name, enabled, trigger_type, trigger_config, action_type, action_config, position, created_at, updated_at
	) VALUES (?, ?, ?, 1, ?, ?, ?, ?, 0, ?, ?)`,
		"rule-pr-closed", "ws-1", "Closed PR rule", model.TriggerGitHubPRClosed, []byte(`{"base_branch":"main"}`), model.ActionRunAgent, []byte(`{}`), now, now)

	observer := &fakeAutomationHealthObserver{}
	engine := NewAutomationRuleEngine(
		repository.NewAutomationRuleRepository(db),
		repository.NewPMTaskRepository(db),
		repository.NewPMWorkflowRepository(db),
		repository.NewTaskDeliveryTargetRepository(db),
		nil,
		nil,
		nil,
		nil,
	).SetHealthObserver(observer)
	svc := newGitDeliveryStatusService(db, nil).SetRuleEngine(engine)

	if err := svc.ProcessWebhookPR(context.Background(), "ws-1", "acme/api", "closed", 42, "Fix merge status", "https://github.test/acme/api/pull/42", "closed", "hel-31-fix-merge-status", "main"); err != nil {
		t.Fatalf("ProcessWebhookPR returned error: %v", err)
	}
	if len(observer.failures) != 1 || observer.failures[0] != "rule-pr-closed" {
		t.Fatalf("closed PR automation trigger failures = %#v, want rule-pr-closed", observer.failures)
	}
}

func TestMergeBranchThenUpdateDeliveryStatusUsesGitHubMergeResult(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)
	app := &fakeGitHubAppClient{}
	svc := newGitDeliveryStatusService(db, app)

	if err := svc.MergeBranch(context.Background(), "ws-1", "task-1", "main"); err != nil {
		t.Fatalf("MergeBranch returned error: %v", err)
	}
	if err := svc.UpdateDeliveryStatusAfterMerge(context.Background(), "ws-1", "task-1", "merged"); err != nil {
		t.Fatalf("UpdateDeliveryStatusAfterMerge returned error: %v", err)
	}
	if len(app.mergeCalls) != 1 {
		t.Fatalf("merge calls = %d, want 1", len(app.mergeCalls))
	}
	call := app.mergeCalls[0]
	if call.InstallationID != "inst-1" || call.Owner != "acme" || call.Repo != "api" || call.Base != "main" || call.Head != "hel-31-fix-merge-status" {
		t.Fatalf("unexpected merge call: %#v", call)
	}

	target, err := repository.NewTaskDeliveryTargetRepository(db).GetByTask(context.Background(), "ws-1", "task-1")
	if err != nil {
		t.Fatalf("load target: %v", err)
	}
	if target.ActivePRStatus == nil || *target.ActivePRStatus != "merged" || target.DeliveryState != "merged" {
		t.Fatalf("target after direct merge = %#v, want merged status/state", target)
	}
}

func assertMergedDeliveryStatus(t *testing.T, db *gorm.DB) {
	t.Helper()
	var targetStatus, deliveryState, linkStatus string
	row := db.Raw(`SELECT active_pr_status, delivery_state FROM task_delivery_targets WHERE id = ?`, "target-1").Row()
	if err := row.Scan(&targetStatus, &deliveryState); err != nil {
		t.Fatalf("scan delivery target: %v", err)
	}
	if targetStatus != "merged" || deliveryState != "merged" {
		t.Fatalf("target status/state = %q/%q, want merged/merged", targetStatus, deliveryState)
	}
	if err := db.Raw(`SELECT pr_status FROM task_git_links WHERE id = ?`, "link-1").Scan(&linkStatus).Error; err != nil {
		t.Fatalf("scan task git link: %v", err)
	}
	if linkStatus != "merged" {
		t.Fatalf("link pr_status = %q, want merged", linkStatus)
	}
}

func formatMergeCalls(calls []fakeGitHubMergeCall) string {
	return fmt.Sprintf("%#v", calls)
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
