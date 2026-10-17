package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type oidcHandlerFixture struct {
	t        *testing.T
	db       *gorm.DB
	provider *fakeOIDCProvider
	handler  *AuthHandler
}

func newOIDCHandlerFixture(t *testing.T) *oidcHandlerFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:oidc-handler-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, full_name TEXT NOT NULL,
			email_verified_at DATETIME, google_subject TEXT UNIQUE, avatar_url TEXT, avatar_style TEXT,
			avatar_seed TEXT, avatar_background_mode TEXT, avatar_background_color TEXT, default_workspace_id TEXT,
			totp_secret_encrypted TEXT, totp_verified BOOLEAN NOT NULL DEFAULT 0, recovery_codes_encrypted TEXT,
			is_platform_admin BOOLEAN NOT NULL DEFAULT 0, is_server_admin BOOLEAN NOT NULL DEFAULT 0,
			signup_verification_pending BOOLEAN NOT NULL DEFAULT 0, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE user_oidc_identities (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id TEXT NOT NULL, issuer TEXT NOT NULL, subject TEXT NOT NULL, created_at DATETIME,
			UNIQUE(issuer, subject)
		)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	authService := service.NewAuthService(repository.NewUserRepository(db), nil, nil, nil, nil,
		auth.NewJWTManager("test-secret"), nil, nil, "http://localhost:5173", []byte(testAuthEncryptionKey))
	authService.SetOIDCIdentityRepository(repository.NewUserOIDCIdentityRepository(db))

	provider := newFakeOIDCProvider(t)
	h := NewAuthHandler(authService)
	h.SetOIDC(OIDCConfig{
		IssuerURL: provider.srv.URL, ClientID: fakeOIDCClientID, ClientSecret: fakeOIDCClientSecret,
		RedirectURL: fakeOIDCRedirectURL, ProviderName: "Authelia",
	})
	return &oidcHandlerFixture{t: t, db: db, provider: provider, handler: h}
}

// start runs OIDCStart, checks the authorization request, and points the fake
// provider at the request's nonce and PKCE challenge.
func (f *oidcHandlerFixture) start(redirect string) (cookies []*http.Cookie, state string) {
	f.t.Helper()
	target := "/api/auth/oidc/start"
	if redirect != "" {
		target += "?redirect=" + url.QueryEscape(redirect)
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	f.handler.OIDCStart(rec, req)
	if rec.Code != http.StatusFound {
		f.t.Fatalf("start status = %d, body = %s", rec.Code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		f.t.Fatalf("parse start redirect: %v", err)
	}
	if !strings.HasPrefix(loc.String(), f.provider.srv.URL+"/authorize?") {
		f.t.Fatalf("start redirected to %q, want the provider's authorization endpoint", loc)
	}
	q := loc.Query()
	for key, want := range map[string]string{
		"response_type": "code", "client_id": fakeOIDCClientID, "redirect_uri": fakeOIDCRedirectURL,
		"code_challenge_method": "S256", "scope": "openid email profile",
	} {
		if got := q.Get(key); got != want {
			f.t.Fatalf("authorize %s = %q, want %q", key, got, want)
		}
	}
	if q.Get("state") == "" || q.Get("nonce") == "" || q.Get("code_challenge") == "" {
		f.t.Fatalf("authorize request lacks state, nonce or code_challenge: %s", loc.RawQuery)
	}
	f.provider.set(func(p *fakeOIDCProvider) { p.nonce, p.challenge = q.Get("nonce"), q.Get("code_challenge") })

	cookies = rec.Result().Cookies()
	if len(cookies) < 3 {
		f.t.Fatalf("start set %d cookies, want state, nonce and verifier", len(cookies))
	}
	for _, c := range cookies {
		if c.Path != oidcCallbackPath || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.MaxAge <= 0 {
			f.t.Fatalf("cookie %s attributes = %+v, want HttpOnly Secure Lax scoped to the callback", c.Name, c)
		}
	}
	return cookies, q.Get("state")
}

func (f *oidcHandlerFixture) callback(cookies []*http.Cookie, query url.Values) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?"+query.Encode(), nil)
	for _, c := range cookies {
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
	rec := httptest.NewRecorder()
	f.handler.OIDCCallback(rec, req)
	return rec
}

func withoutCookie(cookies []*http.Cookie, name string) []*http.Cookie {
	var kept []*http.Cookie
	for _, c := range cookies {
		if c.Name != name {
			kept = append(kept, c)
		}
	}
	return kept
}

func (f *oidcHandlerFixture) userCount() int64 {
	f.t.Helper()
	var n int64
	if err := f.db.Model(&model.User{}).Count(&n).Error; err != nil {
		f.t.Fatalf("count users: %v", err)
	}
	return n
}

// wantLoginError asserts the callback bounced to /login?error=<reason> without
// a session and without creating a user.
func (f *oidcHandlerFixture) wantLoginError(rec *httptest.ResponseRecorder, reason string) {
	f.t.Helper()
	if rec.Code != http.StatusFound {
		f.t.Fatalf("callback status = %d, want 302; body = %s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Header().Get("Location"), "http://localhost:5173/login?error="+reason; got != want {
		f.t.Fatalf("callback redirected to %q, want %q", got, want)
	}
	for _, c := range rec.Result().Cookies() {
		if (c.Name == accessTokenCookieName || c.Name == refreshTokenCookieName) && c.MaxAge > 0 {
			f.t.Fatalf("failed sign-in set auth cookie %s", c.Name)
		}
	}
	if f.userCount() != 0 {
		f.t.Fatal("failed sign-in created a user")
	}
}
