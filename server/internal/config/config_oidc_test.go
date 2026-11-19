package config

import (
	"reflect"
	"strings"
	"testing"
)

func setOIDCEnv(t *testing.T) {
	t.Helper()
	setRequiredConfigEnv(t)
	t.Setenv("OIDC_ISSUER_URL", "https://auth.example.com")
	t.Setenv("OIDC_CLIENT_ID", "helpin")
	t.Setenv("OIDC_CLIENT_SECRET", "secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://helpin.example.com/api/auth/oidc/callback")
}

func TestLoadOIDCDefaults(t *testing.T) {
	setOIDCEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.OIDCConfigured() {
		t.Fatal("OIDCConfigured() = false with all four settings present")
	}
	if cfg.OIDCProviderName != "Single sign-on" {
		t.Fatalf("OIDCProviderName = %q, want the default", cfg.OIDCProviderName)
	}
	if want := []string{"openid", "email", "profile"}; !reflect.DeepEqual(cfg.OIDCScopes, want) {
		t.Fatalf("OIDCScopes = %v, want %v", cfg.OIDCScopes, want)
	}
	if cfg.AuthLocalLoginDisabled {
		t.Fatal("local login must stay enabled unless AUTH_LOCAL_LOGIN_DISABLED is set")
	}
}

func TestLoadOIDCCustomNameAndScopes(t *testing.T) {
	setOIDCEnv(t)
	t.Setenv("OIDC_PROVIDER_NAME", "Authelia")
	t.Setenv("OIDC_SCOPES", "openid, email groups")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.OIDCProviderName != "Authelia" {
		t.Fatalf("OIDCProviderName = %q", cfg.OIDCProviderName)
	}
	if want := []string{"openid", "email", "groups"}; !reflect.DeepEqual(cfg.OIDCScopes, want) {
		t.Fatalf("OIDCScopes = %v, want %v", cfg.OIDCScopes, want)
	}
}

func TestLoadRejectsOIDCRedirectURLThatIsNotTheCallbackRoute(t *testing.T) {
	for _, redirect := range []string{
		"https://helpin.example.com/login",
		"https://helpin.example.com/api/auth/google/callback",
		"https://helpin.example.com/api/auth/oidc/callback/",
		"/api/auth/oidc/callback",
		"ftp://helpin.example.com/api/auth/oidc/callback",
	} {
		setOIDCEnv(t)
		t.Setenv("OIDC_REDIRECT_URL", redirect)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "OIDC_REDIRECT_URL must be") {
			t.Fatalf("Load() with redirect %q error = %v, want an OIDC_REDIRECT_URL error", redirect, err)
		}
	}
}

func TestLoadLocalLoginDisabledRequiresFullOIDCConfig(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("AUTH_LOCAL_LOGIN_DISABLED", "true")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AUTH_LOCAL_LOGIN_DISABLED requires OIDC_ISSUER_URL") {
		t.Fatalf("Load() error = %v, want a refusal that names the missing OIDC settings", err)
	}

	setOIDCEnv(t)
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("AUTH_LOCAL_LOGIN_DISABLED", "true")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted a local-login lockout with an incomplete OIDC config")
	}

	t.Setenv("OIDC_CLIENT_SECRET", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with full OIDC config: %v", err)
	}
	if !cfg.AuthLocalLoginDisabled {
		t.Fatal("AuthLocalLoginDisabled = false, want true")
	}
}
