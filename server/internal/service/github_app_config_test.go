package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const _testGitHubAppStateSecret = "manifest-state-secret"

var _testGitHubAppKey = []byte("0123456789abcdef0123456789abcdef")

type fakeManifestConverter struct {
	conversion *githubapp.ManifestConversion
	err        error
	codes      []string
}

func (f *fakeManifestConverter) Convert(_ context.Context, code string) (*githubapp.ManifestConversion, error) {
	f.codes = append(f.codes, code)
	return f.conversion, f.err
}

func setupGitHubAppTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t)
	mustExec(t, db, `CREATE TABLE github_app_credentials (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		singleton BOOLEAN NOT NULL DEFAULT 1 UNIQUE,
		app_id TEXT NOT NULL,
		slug TEXT NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		client_id TEXT,
		client_secret_encrypted TEXT,
		private_key_encrypted TEXT NOT NULL,
		webhook_secret_encrypted TEXT,
		html_url TEXT,
		owner_login TEXT,
		owner_type TEXT,
		created_by TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	seedWorkspace(t, db, "ws-1", "Acme", "acme", "user-owner")
	seedWorkspaceMember(t, db, "wm-owner", "ws-1", "user-owner", "owner@example.com", "Owner", model.RoleOwner)
	seedWorkspaceMember(t, db, "wm-admin", "ws-1", "user-admin", "admin@example.com", "Admin", model.RoleAdmin)
	return db
}

func newTestGitHubAppConfig(db *gorm.DB, converter gitHubAppManifestConverter, env githubapp.Credentials, manifestEnabled bool) *GitHubAppConfigService {
	return NewGitHubAppConfigService(
		repository.NewGitHubAppCredentialRepository(db),
		repository.NewWorkspaceRepository(db),
		converter,
		GitHubAppConfigOptions{
			Env:             env,
			EncryptionKey:   _testGitHubAppKey,
			AppBaseURL:      "https://helpin.example.com/",
			StateSecret:     _testGitHubAppStateSecret,
			ManifestEnabled: manifestEnabled,
		},
	)
}

func testManifestConversion(t *testing.T) *githubapp.ManifestConversion {
	t.Helper()
	return &githubapp.ManifestConversion{
		ID:            4242,
		Slug:          "helpin-acme",
		Name:          "Helpin (helpin.example.com)",
		ClientID:      "Iv1.client",
		ClientSecret:  "client-secret-value",
		WebhookSecret: "webhook-secret-value",
		PEM:           generateTestPrivateKeyPEM(t),
		HTMLURL:       "https://github.com/apps/helpin-acme",
		OwnerLogin:    "acme",
		OwnerType:     "Organization",
	}
}

func TestGitHubAppConfigEnvTakesPrecedence(t *testing.T) {
	db := setupGitHubAppTestDB(t)
	converter := &fakeManifestConverter{conversion: testManifestConversion(t)}
	stored := newTestGitHubAppConfig(db, converter, githubapp.Credentials{}, true)
	if err := stored.storeConversion(context.Background(), converter.conversion, "user-owner"); err != nil {
		t.Fatalf("store conversion: %v", err)
	}

	env := githubapp.Credentials{AppID: "1", Slug: "env-app", PrivateKey: "pem", WebhookSecret: "env-secret"}
	svc := newTestGitHubAppConfig(db, converter, env, true)
	creds, err := svc.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if creds.AppID != "1" || creds.WebhookSecret != "env-secret" {
		t.Fatalf("expected env credentials, got app_id=%q", creds.AppID)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Source != model.GitHubAppSourceEnv || !status.Configured || status.ManifestAvailable {
		t.Fatalf("unexpected status %+v", status)
	}
	if status.InstallURL != "https://github.com/apps/env-app/installations/new" {
		t.Fatalf("unexpected install url %q", status.InstallURL)
	}
}

func TestGitHubAppConfigPartialEnvFallsBackToDatabase(t *testing.T) {
	db := setupGitHubAppTestDB(t)
	conversion := testManifestConversion(t)
	svc := newTestGitHubAppConfig(db, &fakeManifestConverter{conversion: conversion}, githubapp.Credentials{Slug: "only-slug"}, true)
	if err := svc.storeConversion(context.Background(), conversion, "user-owner"); err != nil {
		t.Fatalf("store conversion: %v", err)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Source != model.GitHubAppSourceDatabase || status.Slug != "helpin-acme" || !status.WebhookConfigured {
		t.Fatalf("unexpected status %+v", status)
	}
}

func TestGitHubAppConfigStoresSecretsEncrypted(t *testing.T) {
	db := setupGitHubAppTestDB(t)
	conversion := testManifestConversion(t)
	svc := newTestGitHubAppConfig(db, &fakeManifestConverter{conversion: conversion}, githubapp.Credentials{}, true)
	if err := svc.storeConversion(context.Background(), conversion, "user-owner"); err != nil {
		t.Fatalf("store conversion: %v", err)
	}

	var row model.GitHubAppCredential
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	for name, value := range map[string]string{
		"private_key":    row.PrivateKeyEncrypted,
		"client_secret":  derefString(row.ClientSecretEncrypted),
		"webhook_secret": derefString(row.WebhookSecretEncrypted),
	} {
		if value == "" || strings.Contains(value, "secret-value") || strings.Contains(value, "PRIVATE KEY") {
			t.Fatalf("%s is not encrypted at rest: %q", name, value)
		}
	}

	creds, err := svc.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if creds.AppID != "4242" || creds.WebhookSecret != "webhook-secret-value" || creds.ClientSecret != "client-secret-value" || creds.PrivateKey != strings.TrimSpace(conversion.PEM) {
		t.Fatalf("decrypted credentials do not round-trip: %+v", creds.AppID)
	}

	wrongKey := NewGitHubAppConfigService(repository.NewGitHubAppCredentialRepository(db), nil, nil, GitHubAppConfigOptions{
		EncryptionKey: []byte("ffffffffffffffffffffffffffffffff"),
	})
	if _, err := wrongKey.Current(context.Background()); err == nil {
		t.Fatal("expected decryption with another key to fail")
	}
}

func TestGitHubAppConfigCacheInvalidatedOnWrite(t *testing.T) {
	db := setupGitHubAppTestDB(t)
	conversion := testManifestConversion(t)
	svc := newTestGitHubAppConfig(db, &fakeManifestConverter{conversion: conversion}, githubapp.Credentials{}, true)
	svc.opts.CacheTTL = time.Hour

	creds, err := svc.Current(context.Background())
	if err != nil || creds.Usable() {
		t.Fatalf("expected no app before creation, got usable=%v err=%v", creds.Usable(), err)
	}
	if err := svc.storeConversion(context.Background(), conversion, "user-owner"); err != nil {
		t.Fatalf("store conversion: %v", err)
	}
	creds, err = svc.Current(context.Background())
	if err != nil || !creds.Usable() {
		t.Fatalf("expected new app without restart, got usable=%v err=%v", creds.Usable(), err)
	}

	client := githubapp.NewClientWithSource(svc)
	if !client.Configured(context.Background()) {
		t.Fatal("expected client backed by the source to be configured")
	}
}

func TestGitHubAppCreateManifest(t *testing.T) {
	db := setupGitHubAppTestDB(t)
	svc := newTestGitHubAppConfig(db, &fakeManifestConverter{}, githubapp.Credentials{}, true)

	resp, err := svc.CreateManifest(context.Background(), "ws-1", "user-owner", model.GitHubAppManifestRequest{})
	if err != nil {
		t.Fatalf("CreateManifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(resp.Manifest, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	name, _ := manifest["name"].(string)
	if !strings.HasPrefix(name, "Helpin (helpin.example.com)") || len(name) > 34 {
		t.Fatalf("unexpected app name %q", name)
	}
	if manifest["url"] != "https://helpin.example.com" || manifest["public"] != false {
		t.Fatalf("unexpected url/public: %v %v", manifest["url"], manifest["public"])
	}
	if manifest["redirect_url"] != "https://helpin.example.com/api/github/app-manifest/callback" {
		t.Fatalf("unexpected redirect_url %v", manifest["redirect_url"])
	}
	if manifest["setup_url"] != "https://helpin.example.com/api/git/github/callback" {
		t.Fatalf("unexpected setup_url %v", manifest["setup_url"])
	}
	hook, _ := manifest["hook_attributes"].(map[string]any)
	if hook["url"] != "https://helpin.example.com/api/git/webhook" {
		t.Fatalf("unexpected hook url %v", hook["url"])
	}
	permissions, _ := manifest["default_permissions"].(map[string]any)
	if permissions["contents"] != "write" || permissions["pull_requests"] != "write" || permissions["checks"] != "read" || permissions["metadata"] != "read" {
		t.Fatalf("unexpected permissions %v", permissions)
	}

	postURL, err := url.Parse(resp.PostURL)
	if err != nil {
		t.Fatalf("parse post url: %v", err)
	}
	if postURL.Host != "github.com" || postURL.Path != "/settings/apps/new" || postURL.Query().Get("state") != resp.State {
		t.Fatalf("unexpected post url %q", resp.PostURL)
	}
	claims, err := svc.parseManifestState(resp.State)
	if err != nil || claims.WorkspaceID != "ws-1" || claims.ActorID != "user-owner" {
		t.Fatalf("state not bound to workspace/user: %+v err=%v", claims, err)
	}

	orgResp, err := svc.CreateManifest(context.Background(), "ws-1", "user-owner", model.GitHubAppManifestRequest{Organization: "acme-inc"})
	if err != nil {
		t.Fatalf("CreateManifest org: %v", err)
	}
	if !strings.HasPrefix(orgResp.PostURL, "https://github.com/organizations/acme-inc/settings/apps/new?state=") {
		t.Fatalf("unexpected org post url %q", orgResp.PostURL)
	}

	if _, err := svc.CreateManifest(context.Background(), "ws-1", "user-owner", model.GitHubAppManifestRequest{Organization: "bad/org"}); !errors.Is(err, ErrGitHubAppManifestInvalid) {
		t.Fatalf("expected invalid organization error, got %v", err)
	}
}

func TestGitHubAppCreateManifestUnavailable(t *testing.T) {
	db := setupGitHubAppTestDB(t)
	disabled := newTestGitHubAppConfig(db, &fakeManifestConverter{}, githubapp.Credentials{}, false)
	if _, err := disabled.CreateManifest(context.Background(), "ws-1", "user-owner", model.GitHubAppManifestRequest{}); !errors.Is(err, ErrGitHubAppManifestUnavailable) {
		t.Fatalf("expected unavailable in disabled edition, got %v", err)
	}
	status, err := disabled.Status(context.Background())
	if err != nil || status.ManifestAvailable {
		t.Fatalf("expected manifest_available=false, got %+v err=%v", status, err)
	}

	env := newTestGitHubAppConfig(db, &fakeManifestConverter{}, githubapp.Credentials{AppID: "1", PrivateKey: "pem"}, true)
	if _, err := env.CreateManifest(context.Background(), "ws-1", "user-owner", model.GitHubAppManifestRequest{}); !errors.Is(err, ErrGitHubAppAlreadyConfigured) {
		t.Fatalf("expected already configured with env app, got %v", err)
	}
}

func TestGitHubAppCompleteManifest(t *testing.T) {
	db := setupGitHubAppTestDB(t)
	converter := &fakeManifestConverter{conversion: testManifestConversion(t)}
	svc := newTestGitHubAppConfig(db, converter, githubapp.Credentials{}, true)
	resp, err := svc.CreateManifest(context.Background(), "ws-1", "user-owner", model.GitHubAppManifestRequest{})
	if err != nil {
		t.Fatalf("CreateManifest: %v", err)
	}

	redirect := svc.CompleteManifest(context.Background(), "code-1", resp.State)
	assertManifestRedirect(t, redirect, "/w/acme/settings/git-connections", "created")
	if len(converter.codes) != 1 || converter.codes[0] != "code-1" {
		t.Fatalf("expected code exchange, got %v", converter.codes)
	}
	creds, err := svc.Current(context.Background())
	if err != nil || creds.Slug != "helpin-acme" {
		t.Fatalf("expected stored app to be active, got %q err=%v", creds.Slug, err)
	}

	// A second App must not overwrite the first.
	second := svc.CompleteManifest(context.Background(), "code-2", resp.State)
	assertManifestRedirect(t, second, "/w/acme/settings/git-connections", "error")
	if len(converter.codes) != 1 {
		t.Fatalf("expected no exchange when an app exists, got %v", converter.codes)
	}
}

func TestGitHubAppCompleteManifestRejectsBadStateAndNonOwner(t *testing.T) {
	db := setupGitHubAppTestDB(t)
	converter := &fakeManifestConverter{conversion: testManifestConversion(t)}
	svc := newTestGitHubAppConfig(db, converter, githubapp.Credentials{}, true)

	assertManifestRedirect(t, svc.CompleteManifest(context.Background(), "code", "not-a-token"), "/github/installed", "error")

	adminState, err := svc.signManifestState("ws-1", "user-admin", "")
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}
	assertManifestRedirect(t, svc.CompleteManifest(context.Background(), "code", adminState), "/w/acme/settings/git-connections", "error")

	ownerState, err := svc.signManifestState("ws-1", "user-owner", "")
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}
	svc.now = func() time.Time { return time.Now().Add(2 * _gitHubAppManifestStateTTL) }
	assertManifestRedirect(t, svc.CompleteManifest(context.Background(), "code", ownerState), "/github/installed", "error")
	if len(converter.codes) != 0 {
		t.Fatalf("expected no code exchange, got %v", converter.codes)
	}
}

func TestGitHubAppManifestName(t *testing.T) {
	name := gitHubAppManifestName("https://a-very-long-subdomain-name.helpin.example.com", "abcd")
	if len(name) > _gitHubAppNameMaxLength || !strings.HasSuffix(name, " abcd") || !strings.HasPrefix(name, "Helpin (") {
		t.Fatalf("unexpected name %q (%d chars)", name, len(name))
	}
}

func assertManifestRedirect(t *testing.T, redirect, wantPath, wantStatus string) {
	t.Helper()
	parsed, err := url.Parse(redirect)
	if err != nil {
		t.Fatalf("parse redirect %q: %v", redirect, err)
	}
	if parsed.Host != "helpin.example.com" || parsed.Path != wantPath {
		t.Fatalf("unexpected redirect target %q", redirect)
	}
	if got := parsed.Query().Get("github"); got != wantStatus {
		t.Fatalf("expected github=%s, got %q (%s)", wantStatus, got, parsed.Query().Get("github_message"))
	}
}
