package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeInstallGitHubApp struct {
	gitHubAppClient
	installations map[string]*githubapp.Installation
}

func (f *fakeInstallGitHubApp) GetInstallation(_ context.Context, installationID string) (*githubapp.Installation, error) {
	if installation, ok := f.installations[installationID]; ok {
		return installation, nil
	}
	return nil, fmt.Errorf("%w: %s", githubapp.ErrInstallationNotFound, installationID)
}

type failingInstallRedirector struct{}

func (failingInstallRedirector) GitHubAppInstallRedirect(context.Context, string, string, string, string) (string, error) {
	return "", errors.New("sign failed")
}

type fakeAppLookup struct {
	app   *githubapp.App
	err   error
	calls int
}

func (f *fakeAppLookup) GetApp(context.Context) (*githubapp.App, error) {
	f.calls++
	return f.app, f.err
}

type gitHubFlowFixture struct {
	db        *gorm.DB
	git       *GitService
	apps      *GitHubAppConfigService
	converter *fakeManifestConverter
}

func createFlowGitIntegrationsTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `CREATE TABLE git_integrations (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
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
}

// setupGitHubFlow wires the manifest service to a GitService the way main
// does: ws-1 (slug acme) belongs to org-1; org-2 is another organization.
func setupGitHubFlow(t *testing.T) *gitHubFlowFixture {
	t.Helper()
	db := setupGitHubAppTestDB(t)
	createFlowGitIntegrationsTable(t, db)
	orgRepo := repository.NewOrganizationRepository(db)
	for _, org := range []model.Organization{
		{ID: "org-1", Name: "Acme", Slug: "acme-org", OwnerID: "user-owner"},
		{ID: "org-2", Name: "Other", Slug: "other-org", OwnerID: "user-other"},
	} {
		if err := db.Create(&org).Error; err != nil {
			t.Fatalf("create organization: %v", err)
		}
	}
	for _, member := range []struct{ org, user, role string }{
		{"org-1", "user-owner", model.RoleOwner},
		{"org-1", "user-admin", model.RoleAdmin},
		{"org-1", "user-member", model.RoleMember},
		{"org-2", "user-other", model.RoleOwner},
	} {
		if _, err := orgRepo.AddMember(context.Background(), member.org, member.user, member.role); err != nil {
			t.Fatalf("add org member: %v", err)
		}
	}
	mustExec(t, db, `UPDATE workspaces SET organization_id = 'org-1' WHERE id = 'ws-1'`)

	git := &GitService{
		integrationRepo: repository.NewGitIntegrationRepository(db),
		workspaceRepo:   repository.NewWorkspaceRepository(db),
		orgRepo:         orgRepo,
		githubApp: &fakeInstallGitHubApp{installations: map[string]*githubapp.Installation{
			"777": {ID: 777, AppID: 4242, AccountLogin: "acme", AccountType: "Organization"},
			"888": {ID: 888, AppID: 1, AccountLogin: "elsewhere", AccountType: "User"},
		}},
		appBaseURL:          "https://helpin.example.com",
		stateSecret:         _testGitHubAppStateSecret,
		installClaimEnabled: true,
	}
	converter := &fakeManifestConverter{conversion: testManifestConversion(t)}
	apps := newTestGitHubAppConfig(db, converter, githubapp.Credentials{}, true).SetInstallRedirector(git)
	git.SetGitHubAppSource(apps)
	return &gitHubFlowFixture{db: db, git: git, apps: apps, converter: converter}
}

func TestGitHubAppManifestToInstallRoundTripsReturnTo(t *testing.T) {
	tests := []struct {
		returnTo string
		want     string
		wantPath string
	}{
		{returnTo: "", want: GitHubReturnSettings, wantPath: "/w/acme/settings/git-connections"},
		{returnTo: "settings", want: GitHubReturnSettings, wantPath: "/w/acme/settings/git-connections"},
		{returnTo: "setup", want: GitHubReturnSetup, wantPath: "/w/acme/setup"},
		{returnTo: "system_status", want: GitHubReturnSystemStatus, wantPath: "/w/acme/settings/system-status"},
	}
	for _, tt := range tests {
		t.Run("return_to="+tt.returnTo, func(t *testing.T) {
			f := setupGitHubFlow(t)
			ctx := context.Background()

			resp, err := f.apps.CreateManifest(ctx, "ws-1", "user-owner", model.GitHubAppManifestRequest{Organization: "acme", ReturnTo: tt.returnTo})
			if err != nil {
				t.Fatalf("CreateManifest: %v", err)
			}
			if !strings.HasPrefix(resp.PostURL, "https://github.com/organizations/acme/settings/apps/new?state=") {
				t.Fatalf("expected organization post url, got %q", resp.PostURL)
			}
			manifestClaims, err := f.apps.parseManifestState(resp.State)
			if err != nil || manifestClaims.ReturnTo != tt.want {
				t.Fatalf("manifest state return_to = %+v err=%v, want %q", manifestClaims, err, tt.want)
			}

			redirect, err := url.Parse(f.apps.CompleteManifest(ctx, "code-1", resp.State))
			if err != nil {
				t.Fatalf("parse manifest redirect: %v", err)
			}
			if redirect.Host != "github.com" || redirect.Path != "/apps/helpin-acme/installations/new" {
				t.Fatalf("expected the new App's install page, got %q", redirect.String())
			}
			installState := redirect.Query().Get("state")
			claims, _, returnURL, err := f.git.resolveGitHubInstallState(ctx, installState)
			if err != nil {
				t.Fatalf("install state is not parseable: %v", err)
			}
			if claims.OrganizationID != "org-1" || claims.WorkspaceID != "ws-1" || claims.ActorID != "user-owner" || claims.ReturnTo != tt.want {
				t.Fatalf("unexpected install claims %+v", claims)
			}
			if !strings.HasSuffix(returnURL, tt.wantPath) {
				t.Fatalf("return url %q, want path %q", returnURL, tt.wantPath)
			}

			final, err := url.Parse(f.git.GitHubInstallCallbackRedirect(ctx, installState, "777", "install"))
			if err != nil {
				t.Fatalf("parse final redirect: %v", err)
			}
			if final.Host != "helpin.example.com" || final.Path != tt.wantPath || final.Query().Get("github") != GitHubResultConnected {
				t.Fatalf("unexpected final redirect %q", final.String())
			}
			integration, err := f.git.integrationRepo.GetByInstallationID(ctx, "github", "777")
			if err != nil || integration == nil || derefString(integration.OrganizationID) != "org-1" {
				t.Fatalf("expected org-1 integration, got %+v err=%v", integration, err)
			}
		})
	}
}

func TestGitHubAppCompleteManifestFallsBackWhenInstallLinkFails(t *testing.T) {
	f := setupGitHubFlow(t)
	f.apps.SetInstallRedirector(failingInstallRedirector{})
	resp, err := f.apps.CreateManifest(context.Background(), "ws-1", "user-owner", model.GitHubAppManifestRequest{ReturnTo: "setup"})
	if err != nil {
		t.Fatalf("CreateManifest: %v", err)
	}
	redirect := f.apps.CompleteManifest(context.Background(), "code-1", resp.State)
	assertManifestRedirect(t, redirect, "/w/acme/setup", GitHubResultCreated)
	parsed, _ := url.Parse(redirect)
	if !strings.Contains(parsed.Query().Get("github_message"), "Install it") {
		t.Fatalf("expected install hint, got %q", parsed.Query().Get("github_message"))
	}
}

func TestGitHubAppCompleteManifestInvalidStateUsesFrontendRoute(t *testing.T) {
	f := setupGitHubFlow(t)
	redirect := f.apps.CompleteManifest(context.Background(), "code", "expired-or-forged")
	assertManifestRedirect(t, redirect, "/github/installed", GitHubResultError)
	parsed, _ := url.Parse(redirect)
	if !strings.Contains(parsed.Query().Get("github_message"), "invalid or expired") {
		t.Fatalf("expected an explanation, got %q", parsed.Query().Get("github_message"))
	}
	if len(f.converter.codes) != 0 {
		t.Fatalf("expected no code exchange, got %v", f.converter.codes)
	}
}

func TestGitHubAppCreateManifestRejectsUnknownReturnTo(t *testing.T) {
	f := setupGitHubFlow(t)
	_, err := f.apps.CreateManifest(context.Background(), "ws-1", "user-owner", model.GitHubAppManifestRequest{ReturnTo: "elsewhere"})
	if !errors.Is(err, ErrGitHubAppManifestInvalid) {
		t.Fatalf("expected invalid return_to, got %v", err)
	}
}

func TestGitHubInstallCallbackWithoutValidState(t *testing.T) {
	f := setupGitHubFlow(t)
	tests := []struct {
		name, state, installationID, setupAction string
		wantQuery                                map[string]string
	}{
		{name: "installed from GitHub", installationID: "777", setupAction: "install",
			wantQuery: map[string]string{"installation_id": "777", "setup_action": "install", "github": ""}},
		{name: "updated from GitHub", installationID: "777", setupAction: "update",
			wantQuery: map[string]string{"installation_id": "777", "setup_action": "update"}},
		{name: "expired state", state: "not-a-token", installationID: "777", setupAction: "install",
			wantQuery: map[string]string{"installation_id": "777", "setup_action": "install"}},
		{name: "no installation", wantQuery: map[string]string{"github": GitHubResultError, "installation_id": ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redirect, err := url.Parse(f.git.GitHubInstallCallbackRedirect(context.Background(), tt.state, tt.installationID, tt.setupAction))
			if err != nil {
				t.Fatalf("parse redirect: %v", err)
			}
			if redirect.Host != "helpin.example.com" || redirect.Path != "/github/installed" {
				t.Fatalf("expected the frontend route, got %q", redirect.String())
			}
			for key, want := range tt.wantQuery {
				if got := redirect.Query().Get(key); got != want {
					t.Fatalf("%s = %q, want %q (%s)", key, got, want, redirect.String())
				}
			}
		})
	}

	request, _ := url.Parse(f.git.GitHubInstallCallbackRedirect(context.Background(), "", "", "request"))
	if !strings.Contains(request.Query().Get("github_message"), "request") {
		t.Fatalf("expected the install-request explanation, got %q", request.Query().Get("github_message"))
	}
}

func TestGitHubInstallStateWithoutReturnToReturnsToSettings(t *testing.T) {
	f := setupGitHubFlow(t)
	// A token signed before return_to existed.
	legacy, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"organization_id": "org-1",
		"workspace_id":    "ws-1",
		"actor_id":        "user-owner",
		"exp":             time.Now().Add(time.Minute).Unix(),
	}).SignedString([]byte(_testGitHubAppStateSecret))
	if err != nil {
		t.Fatalf("sign legacy state: %v", err)
	}
	redirect, _ := url.Parse(f.git.GitHubInstallCallbackRedirect(context.Background(), legacy, "777", "install"))
	if redirect.Path != "/w/acme/settings/git-connections" || redirect.Query().Get("github") != GitHubResultConnected {
		t.Fatalf("unexpected redirect %q", redirect.String())
	}
}

func TestGitHubInstallURLCarriesReturnTo(t *testing.T) {
	f := setupGitHubFlow(t)
	f.git.githubAppSlug = "helpin-acme"
	f.git.githubAppSource = nil
	installURL, action, _, err := f.git.GetGitHubInstallURLForOrganization(context.Background(), "org-1", "ws-1", "user-admin", false, GitHubReturnSystemStatus)
	if err != nil || action != "install" {
		t.Fatalf("GetGitHubInstallURLForOrganization: action=%q err=%v", action, err)
	}
	parsed, _ := url.Parse(installURL)
	claims, _, returnURL, err := f.git.resolveGitHubInstallState(context.Background(), parsed.Query().Get("state"))
	if err != nil || claims.ReturnTo != GitHubReturnSystemStatus || !strings.HasSuffix(returnURL, "/w/acme/settings/system-status") {
		t.Fatalf("unexpected state %+v return=%q err=%v", claims, returnURL, err)
	}
}

// setupClaimFlow stores a manifest-created App: claiming is only allowed for
// Apps created from Helpin, which are private to their owner account.
func setupClaimFlow(t *testing.T) *gitHubFlowFixture {
	t.Helper()
	f := setupGitHubFlow(t)
	if err := f.apps.storeConversion(context.Background(), f.converter.conversion, "user-owner"); err != nil {
		t.Fatalf("store app: %v", err)
	}
	return f
}

func TestClaimGitHubInstallation(t *testing.T) {
	ctx := context.Background()

	t.Run("refuses Apps pinned in env, which may be public", func(t *testing.T) {
		f := setupGitHubFlow(t)
		env := githubapp.Credentials{AppID: "4242", Slug: "helpin-env", PrivateKey: "pinned-key"}
		f.git.SetGitHubAppSource(newTestGitHubAppConfig(f.db, f.converter, env, true))
		if _, err := f.git.ClaimGitHubInstallation(ctx, "org-1", "ws-1", "user-owner", "777"); !errors.Is(err, ErrGitHubInstallClaimUnavailable) {
			t.Fatalf("expected claim to be unavailable for an env App, got %v", err)
		}
	})

	t.Run("links and refreshes idempotently", func(t *testing.T) {
		f := setupClaimFlow(t)
		first, err := f.git.ClaimGitHubInstallation(ctx, "org-1", "ws-1", "user-admin", "777")
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if !first.Created || first.AccountLogin != "acme" || first.AccountType != "Organization" || first.IntegrationID == "" {
			t.Fatalf("unexpected claim %+v", first)
		}
		again, err := f.git.ClaimGitHubInstallation(ctx, "org-1", "ws-1", "user-owner", "777")
		if err != nil {
			t.Fatalf("second claim: %v", err)
		}
		if again.Created || again.IntegrationID != first.IntegrationID {
			t.Fatalf("expected a refresh of %s, got %+v", first.IntegrationID, again)
		}
	})

	t.Run("refuses an installation linked to another organization", func(t *testing.T) {
		f := setupClaimFlow(t)
		if _, err := f.git.ClaimGitHubInstallation(ctx, "org-1", "ws-1", "user-owner", "777"); err != nil {
			t.Fatalf("claim: %v", err)
		}
		if _, err := f.git.ClaimGitHubInstallation(ctx, "org-2", "", "user-other", "777"); !errors.Is(err, ErrGitHubInstallationClaimed) {
			t.Fatalf("expected conflict, got %v", err)
		}
	})

	tests := []struct {
		name           string
		org, workspace string
		actor          string
		installationID string
		disable        bool
		want           error
	}{
		{name: "members cannot claim", org: "org-1", workspace: "ws-1", actor: "user-member", installationID: "777", want: ErrGitHubInstallClaimForbidden},
		{name: "outsiders cannot claim", org: "org-1", workspace: "ws-1", actor: "user-other", installationID: "777", want: ErrGitHubInstallClaimForbidden},
		{name: "unknown installation", org: "org-1", workspace: "ws-1", actor: "user-owner", installationID: "999", want: ErrGitHubInstallationNotFound},
		{name: "non-numeric installation", org: "org-1", workspace: "ws-1", actor: "user-owner", installationID: "../app", want: ErrGitHubInstallClaimInvalid},
		{name: "workspace from another organization", org: "org-2", workspace: "ws-1", actor: "user-other", installationID: "777", want: ErrGitHubInstallClaimInvalid},
		{name: "disabled for public apps", org: "org-1", workspace: "ws-1", actor: "user-owner", installationID: "777", disable: true, want: ErrGitHubInstallClaimUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setupClaimFlow(t)
			f.git.SetGitHubInstallClaimEnabled(!tt.disable)
			if _, err := f.git.ClaimGitHubInstallation(ctx, tt.org, tt.workspace, tt.actor, tt.installationID); !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			if integration, _ := f.git.integrationRepo.GetByInstallationID(ctx, "github", tt.installationID); integration != nil {
				t.Fatalf("expected no integration, got %+v", integration)
			}
		})
	}

	t.Run("installation of a different App", func(t *testing.T) {
		f := setupGitHubFlow(t)
		if err := f.apps.storeConversion(ctx, f.converter.conversion, "user-owner"); err != nil {
			t.Fatalf("store app: %v", err)
		}
		if _, err := f.git.ClaimGitHubInstallation(ctx, "org-1", "ws-1", "user-owner", "888"); !errors.Is(err, ErrGitHubInstallationNotFound) {
			t.Fatalf("expected not found for another App's installation, got %v", err)
		}
	})
}

func TestGitHubAppStatusReportsOwner(t *testing.T) {
	ctx := context.Background()

	t.Run("manifest-created App is private and owned by its creator account", func(t *testing.T) {
		f := setupGitHubFlow(t)
		if err := f.apps.storeConversion(ctx, f.converter.conversion, "user-owner"); err != nil {
			t.Fatalf("store app: %v", err)
		}
		status, err := f.apps.Status(ctx)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if status.OwnerLogin != "acme" || status.OwnerType != "Organization" || !status.Private || status.ManifestBlockedReason != "" {
			t.Fatalf("unexpected status %+v", status)
		}
	})

	t.Run("environment App owner comes from GET /app once", func(t *testing.T) {
		db := setupGitHubAppTestDB(t)
		lookup := &fakeAppLookup{app: &githubapp.App{OwnerLogin: "acme-env", OwnerType: "User"}}
		svc := newTestGitHubAppConfig(db, nil, githubapp.Credentials{AppID: "1", PrivateKey: "pem"}, false).SetAppLookup(lookup)
		for i := 0; i < 2; i++ {
			status, err := svc.Status(ctx)
			if err != nil {
				t.Fatalf("Status: %v", err)
			}
			if status.OwnerLogin != "acme-env" || status.OwnerType != "User" || status.Private {
				t.Fatalf("unexpected status %+v", status)
			}
		}
		if lookup.calls != 1 {
			t.Fatalf("expected one GET /app, got %d", lookup.calls)
		}
	})

	t.Run("environment App owner is empty when GitHub is unavailable", func(t *testing.T) {
		db := setupGitHubAppTestDB(t)
		svc := newTestGitHubAppConfig(db, nil, githubapp.Credentials{AppID: "1", PrivateKey: "pem"}, false).
			SetAppLookup(&fakeAppLookup{err: errors.New("offline")})
		status, err := svc.Status(ctx)
		if err != nil || status.OwnerLogin != "" || status.OwnerType != "" {
			t.Fatalf("unexpected status %+v err=%v", status, err)
		}
	})
}

func TestGitHubAppStatusManifestBlockedReason(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		blocked bool
	}{
		{name: "public https", baseURL: "https://helpin.example.com", blocked: false},
		{name: "public ip", baseURL: "https://8.8.8.8", blocked: false},
		{name: "http", baseURL: "http://helpin.example.com", blocked: true},
		{name: "localhost", baseURL: "https://localhost:8080", blocked: true},
		{name: "localhost subdomain", baseURL: "https://app.localhost", blocked: true},
		{name: "mdns", baseURL: "https://helpin.local", blocked: true},
		{name: "single label", baseURL: "https://helpin", blocked: true},
		{name: "private ip", baseURL: "https://192.168.1.20", blocked: true},
		{name: "private 10/8", baseURL: "https://10.0.0.5:8443", blocked: true},
		{name: "loopback", baseURL: "https://127.0.0.1", blocked: true},
		{name: "link local", baseURL: "https://169.254.10.1", blocked: true},
		{name: "ipv6 loopback", baseURL: "https://[::1]", blocked: true},
		{name: "empty", baseURL: "", blocked: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := GitHubAppBaseURLBlockedReason(tt.baseURL)
			if (reason != "") != tt.blocked {
				t.Fatalf("GitHubAppBaseURLBlockedReason(%q) = %q, blocked=%v", tt.baseURL, reason, tt.blocked)
			}
			if tt.blocked && tt.baseURL != "" && !strings.Contains(reason, "(currently "+tt.baseURL+")") {
				t.Fatalf("reason does not name the current value: %q", reason)
			}
		})
	}

	db := setupGitHubAppTestDB(t)
	svc := NewGitHubAppConfigService(repository.NewGitHubAppCredentialRepository(db), repository.NewWorkspaceRepository(db),
		&fakeManifestConverter{}, GitHubAppConfigOptions{
			EncryptionKey:   _testGitHubAppKey,
			AppBaseURL:      "http://localhost:8080",
			StateSecret:     _testGitHubAppStateSecret,
			ManifestEnabled: true,
		})
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.ManifestAvailable || !strings.Contains(status.ManifestBlockedReason, "APP_BASE_URL") {
		t.Fatalf("expected a blocked manifest, got %+v", status)
	}
	if _, err := svc.CreateManifest(context.Background(), "ws-1", "user-owner", model.GitHubAppManifestRequest{}); !errors.Is(err, ErrGitHubAppManifestUnavailable) ||
		!strings.Contains(err.Error(), "public https") {
		t.Fatalf("expected the reachability error, got %v", err)
	}
}
