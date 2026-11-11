package service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/widgetorigin"
	"gorm.io/gorm"
)

func setupWidgetHelpCenterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS docs_helpcenter_configs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			subdomain TEXT NOT NULL DEFAULT '',
			custom_domain TEXT,
			custom_domain_status TEXT,
			public_url_mode TEXT NOT NULL DEFAULT 'hosted_subdomain',
			reverse_proxy_host TEXT,
			reverse_proxy_base_path TEXT,
			brand_name TEXT NOT NULL DEFAULT '',
			brand_logo_url TEXT,
			favicon_url TEXT,
			is_published BOOLEAN NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL DEFAULT '',
			preset_key TEXT,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	return db
}

func TestHelpCenterWidgetOrigins(t *testing.T) {
	verified, pending := model.HelpcenterDomainVerified, "pending"
	tests := []struct {
		name   string
		cfg    *model.DocsHelpcenterConfig
		hosted string
		want   []string
	}{
		{name: "no help center", cfg: nil, hosted: "helpin.center", want: nil},
		{
			name:   "hosted subdomain",
			cfg:    &model.DocsHelpcenterConfig{Subdomain: "Acme-Demo"},
			hosted: "helpin.center",
			want:   []string{"https://acme-demo.helpin.center"},
		},
		{
			name:   "no hosted domain configured",
			cfg:    &model.DocsHelpcenterConfig{Subdomain: "acme"},
			hosted: "",
			want:   nil,
		},
		{
			name: "live custom domain is added",
			cfg: &model.DocsHelpcenterConfig{
				Subdomain: "acme", CustomDomain: ptr("help.acme.com"), CustomDomainStatus: &verified,
			},
			hosted: "helpin.center",
			want:   []string{"https://acme.helpin.center", "https://help.acme.com"},
		},
		{
			name: "unverified custom domain is ignored",
			cfg: &model.DocsHelpcenterConfig{
				Subdomain: "acme", CustomDomain: ptr("help.acme.com"), CustomDomainStatus: &pending,
			},
			hosted: "helpin.center",
			want:   []string{"https://acme.helpin.center"},
		},
		{
			name: "reverse proxy host only in reverse proxy mode",
			cfg: &model.DocsHelpcenterConfig{
				Subdomain: "acme", PublicURLMode: model.HelpcenterPublicURLModeReverseProxy,
				ReverseProxyHost: ptr("https://www.acme.com"),
			},
			hosted: "helpin.center",
			want:   []string{"https://acme.helpin.center", "https://www.acme.com"},
		},
		{
			name: "reverse proxy host ignored in hosted mode",
			cfg: &model.DocsHelpcenterConfig{
				Subdomain: "acme", PublicURLMode: model.HelpcenterPublicURLModeHostedSubdomain,
				ReverseProxyHost: ptr("www.acme.com"),
			},
			hosted: "helpin.center",
			want:   []string{"https://acme.helpin.center"},
		},
		{
			name:   "host with a path is rejected",
			cfg:    &model.DocsHelpcenterConfig{Subdomain: "acme/evil"},
			hosted: "helpin.center",
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := helpCenterWidgetOrigins(tt.cfg, tt.hosted); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("helpCenterWidgetOrigins() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthorizeWidgetOriginAllowsOwnHelpCenter(t *testing.T) {
	db := setupWidgetHelpCenterTestDB(t)
	for _, stmt := range []string{
		// The allowlist names only the customer's website, as the seed does.
		`INSERT INTO support_widget_installations (id, workspace_id, widget_key, secret_key, allowed_origins, active)
			VALUES ('inst-a', 'ws-a', 'key-a', 'secret', '{"https://acme.com"}', true),
			       ('inst-b', 'ws-b', 'key-b', 'secret', '{}', true)`,
		`INSERT INTO docs_helpcenter_configs (id, workspace_id, subdomain) VALUES ('hc-a', 'ws-a', 'acme'), ('hc-b', 'ws-b', 'other')`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	svc := &SupportInboxService{
		installationRepo:   repository.NewSupportInboxInstallationRepository(db),
		sessionRepo:        repository.NewSupportInboxSessionRepository(db),
		docsHelpcenterRepo: repository.NewDocsHelpcenterRepository(db),
	}
	svc.SetHelpcenterHostedDomain("Helpin.Center")

	tests := []struct {
		name, origin, key string
		allowed           bool
	}{
		{"own help center", "https://acme.helpin.center", "key-a", true},
		{"own help center with empty allowlist", "https://other.helpin.center", "key-b", true},
		{"allowlisted website still works", "https://acme.com", "key-a", true},
		{"another workspace's help center", "https://other.helpin.center", "key-a", false},
		{"http scheme is not the help center", "http://acme.helpin.center", "key-a", false},
		{"lookalike host", "https://acme.helpin.center.evil.example", "key-a", false},
		{"missing origin", "", "key-a", false},
		{"opaque origin", "null", "key-a", false},
		{"unrelated site with empty allowlist", "https://acme.com", "key-b", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.AuthorizeWidgetOrigin(context.Background(), tt.origin, widgetorigin.Reference{WidgetKey: tt.key})
			if (err == nil) != tt.allowed {
				t.Fatalf("AuthorizeWidgetOrigin(%q) error = %v, want allowed=%v", tt.origin, err, tt.allowed)
			}
		})
	}
}

func TestAuthorizeWidgetOriginWithoutHostedDomainKeepsAllowlistOnly(t *testing.T) {
	db := setupWidgetHelpCenterTestDB(t)
	for _, stmt := range []string{
		`INSERT INTO support_widget_installations (id, workspace_id, widget_key, secret_key, allowed_origins, active) VALUES ('inst-a', 'ws-a', 'key-a', 'secret', '{}', true)`,
		`INSERT INTO docs_helpcenter_configs (id, workspace_id, subdomain) VALUES ('hc-a', 'ws-a', 'acme')`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	svc := &SupportInboxService{
		installationRepo:   repository.NewSupportInboxInstallationRepository(db),
		sessionRepo:        repository.NewSupportInboxSessionRepository(db),
		docsHelpcenterRepo: repository.NewDocsHelpcenterRepository(db),
	}
	if err := svc.AuthorizeWidgetOrigin(context.Background(), "https://acme.helpin.center", widgetorigin.Reference{WidgetKey: "key-a"}); err == nil {
		t.Fatal("hosted help center origin allowed although no hosted domain is configured")
	}
}

func TestApplyHelpCenterWidgetBranding(t *testing.T) {
	tests := []struct {
		name       string
		row        string
		settings   model.SupportInboxSettings
		wantName   string
		wantLogo   string
		wantAvatar string
	}{
		{
			name:     "published help center fills unset fields from the favicon",
			row:      `('hc', 'ws', 'acme', 'Acme', 'https://cdn/logo.png', 'https://cdn/fav.png', true)`,
			wantName: "Acme", wantLogo: "https://cdn/fav.png", wantAvatar: "https://cdn/fav.png",
		},
		{
			name:     "logo is the fallback mark",
			row:      `('hc', 'ws', 'acme', 'Acme', 'https://cdn/logo.png', NULL, true)`,
			wantName: "Acme", wantLogo: "https://cdn/logo.png", wantAvatar: "https://cdn/logo.png",
		},
		{
			name:     "saved widget values win",
			row:      `('hc', 'ws', 'acme', 'Acme', 'https://cdn/logo.png', 'https://cdn/fav.png', true)`,
			settings: model.SupportInboxSettings{WidgetName: "Acme Support", LogoURL: "https://own/logo.png"},
			wantName: "Acme Support", wantLogo: "https://own/logo.png", wantAvatar: "https://cdn/fav.png",
		},
		{
			name: "unpublished help center changes nothing",
			row:  `('hc', 'ws', 'acme', 'Acme', 'https://cdn/logo.png', 'https://cdn/fav.png', false)`,
		},
		{name: "no help center changes nothing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupWidgetHelpCenterTestDB(t)
			if tt.row != "" {
				if err := db.Exec(`INSERT INTO docs_helpcenter_configs (id, workspace_id, subdomain, brand_name, brand_logo_url, favicon_url, is_published) VALUES ` + tt.row).Error; err != nil {
					t.Fatalf("seed help center: %v", err)
				}
			}
			svc := &SupportInboxService{docsHelpcenterRepo: repository.NewDocsHelpcenterRepository(db)}
			settings := tt.settings
			svc.applyHelpCenterWidgetBranding(context.Background(), "ws", &settings)
			if settings.WidgetName != tt.wantName || settings.LogoURL != tt.wantLogo || settings.WidgetAvatarURL != tt.wantAvatar {
				t.Fatalf("branding = (%q, %q, %q), want (%q, %q, %q)",
					settings.WidgetName, settings.LogoURL, settings.WidgetAvatarURL, tt.wantName, tt.wantLogo, tt.wantAvatar)
			}
		})
	}
}

func TestBindDefaultSupportAgent(t *testing.T) {
	seed := func(t *testing.T, settings string, withAgent bool) (*SupportInboxService, *repository.SupportInboxInstallationRepository) {
		t.Helper()
		db := setupWidgetHelpCenterTestDB(t)
		if err := db.Exec(`INSERT INTO support_widget_installations (id, workspace_id, widget_key, secret_key, allowed_origins, settings, active) VALUES ('inst', 'ws', 'key', 'secret', '{}', ?, true)`, settings).Error; err != nil {
			t.Fatalf("seed installation: %v", err)
		}
		if withAgent {
			if err := db.Exec(`INSERT INTO agents (id, workspace_id, is_system, name, preset_key) VALUES
				('agent-echo', 'ws', true, 'Echo', 'support_agent'),
				('agent-other-ws', 'ws-2', true, 'Echo', 'support_agent'),
				('agent-custom', 'ws', false, 'Custom', 'support_agent')`).Error; err != nil {
				t.Fatalf("seed agents: %v", err)
			}
		}
		repo := repository.NewSupportInboxInstallationRepository(db)
		return &SupportInboxService{installationRepo: repo, agentRepo: repository.NewAgentRepository(db)}, repo
	}
	load := func(t *testing.T, repo *repository.SupportInboxInstallationRepository) model.SupportInboxSettings {
		t.Helper()
		inst, err := repo.GetByWorkspace(context.Background(), "ws")
		if err != nil || inst == nil {
			t.Fatalf("load installation: %v, %v", inst, err)
		}
		var settings model.SupportInboxSettings
		if err := json.Unmarshal([]byte(inst.Settings), &settings); err != nil {
			t.Fatalf("parse settings: %v", err)
		}
		return settings
	}

	t.Run("binds the system support agent and leaves AI off", func(t *testing.T) {
		svc, repo := seed(t, `{"welcome_message":"Hi"}`, true)
		if err := svc.BindDefaultSupportAgent(context.Background(), "ws"); err != nil {
			t.Fatalf("BindDefaultSupportAgent: %v", err)
		}
		got := load(t, repo)
		if got.AIAgentID == nil || *got.AIAgentID != "agent-echo" {
			t.Fatalf("ai_agent_id = %v, want agent-echo", got.AIAgentID)
		}
		if got.AIEnabled {
			t.Fatal("ai_enabled was turned on; it must stay an explicit choice")
		}
		if got.WelcomeMessage != "Hi" {
			t.Fatalf("welcome_message = %q, want it preserved", got.WelcomeMessage)
		}
	})

	t.Run("keeps an agent that is already chosen", func(t *testing.T) {
		svc, repo := seed(t, `{"ai_agent_id":"agent-custom"}`, true)
		if err := svc.BindDefaultSupportAgent(context.Background(), "ws"); err != nil {
			t.Fatalf("BindDefaultSupportAgent: %v", err)
		}
		if got := load(t, repo); got.AIAgentID == nil || *got.AIAgentID != "agent-custom" {
			t.Fatalf("ai_agent_id = %v, want agent-custom", got.AIAgentID)
		}
	})

	t.Run("does nothing before agents are seeded", func(t *testing.T) {
		svc, repo := seed(t, `{}`, false)
		if err := svc.BindDefaultSupportAgent(context.Background(), "ws"); err != nil {
			t.Fatalf("BindDefaultSupportAgent: %v", err)
		}
		if got := load(t, repo); got.AIAgentID != nil {
			t.Fatalf("ai_agent_id = %v, want nil", *got.AIAgentID)
		}
	})

	t.Run("workspace without an installation is a no-op", func(t *testing.T) {
		svc, _ := seed(t, `{}`, true)
		if err := svc.BindDefaultSupportAgent(context.Background(), "ws-missing"); err != nil {
			t.Fatalf("BindDefaultSupportAgent: %v", err)
		}
	})
}
