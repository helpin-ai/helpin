package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestOIDCCallbackHappyPath(t *testing.T) {
	for name, emailInIDToken := range map[string]bool{"email only from userinfo": false, "email in id token": true} {
		t.Run(name, func(t *testing.T) {
			f := newOIDCHandlerFixture(t)
			f.provider.emailInIDToken = emailInIDToken
			cookies, state := f.start("/workspaces/personal")

			rec := f.callback(cookies, url.Values{"state": {state}, "code": {fakeOIDCCode}})

			if rec.Code != http.StatusFound || rec.Header().Get("Location") != "http://localhost:5173/workspaces/personal" {
				t.Fatalf("callback = %d -> %q, want 302 to the requested app path; body = %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
			}
			var access, refresh bool
			cleared := map[string]bool{}
			for _, c := range rec.Result().Cookies() {
				switch {
				case c.Name == accessTokenCookieName && c.Value != "":
					access = true
				case c.Name == refreshTokenCookieName && c.Value != "":
					refresh = true
				case c.MaxAge < 0:
					cleared[c.Name] = true
				}
			}
			if !access || !refresh {
				t.Fatal("callback did not set the session cookies")
			}
			for _, cookieName := range oidcCookieNames {
				if !cleared[cookieName] {
					t.Fatalf("cookie %s was not cleared after use", cookieName)
				}
			}
			var user model.User
			if err := f.db.Where("email = ?", "sso@example.com").First(&user).Error; err != nil {
				t.Fatalf("load user: %v", err)
			}
			if user.FullName != "Sso Person" || user.EmailVerifiedAt == nil {
				t.Fatalf("user = %+v, want name from claims and verified email", user)
			}
			var link model.UserOIDCIdentity
			if err := f.db.Where("issuer = ? AND subject = ?", f.provider.srv.URL, "authelia-user-1").First(&link).Error; err != nil || link.UserID != user.ID {
				t.Fatalf("identity link = %+v (err %v), want it to point at user %s", link, err, user.ID)
			}
			if emailInIDToken && f.provider.userinfoHit != 0 {
				t.Fatal("userinfo was called although the id token carried the email")
			}
			if !emailInIDToken && f.provider.userinfoHit != 1 {
				t.Fatalf("userinfo calls = %d, want 1", f.provider.userinfoHit)
			}
		})
	}
}

func TestOIDCCallbackAcceptsEmailVerifiedAsString(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	f.provider.emailInIDToken, f.provider.verifiedAsString = true, true
	cookies, state := f.start("")

	rec := f.callback(cookies, url.Values{"state": {state}, "code": {fakeOIDCCode}})

	if rec.Header().Get("Location") != "http://localhost:5173/workspaces" || f.userCount() != 1 {
		t.Fatalf("callback = %q with %d users, want a successful sign-in", rec.Header().Get("Location"), f.userCount())
	}
}

func TestOIDCCallbackRejectsBadState(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	cookies, _ := f.start("")

	f.wantLoginError(f.callback(cookies, url.Values{"state": {"forged"}, "code": {fakeOIDCCode}}), "invalid_state")
	f.wantLoginError(f.callback(cookies, url.Values{"code": {fakeOIDCCode}}), "invalid_state")
	f.wantLoginError(f.callback(nil, url.Values{"state": {"forged"}, "code": {fakeOIDCCode}}), "invalid_state")
	if f.provider.tokenCalls != 0 {
		t.Fatalf("token endpoint called %d times for a rejected state", f.provider.tokenCalls)
	}
}

func TestOIDCCallbackRequiresPKCEVerifierCookie(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	cookies, state := f.start("")

	f.wantLoginError(f.callback(withoutCookie(cookies, oidcVerifierCookie), url.Values{"state": {state}, "code": {fakeOIDCCode}}), "invalid_state")
	if f.provider.tokenCalls != 0 {
		t.Fatalf("token endpoint called %d times without a PKCE verifier", f.provider.tokenCalls)
	}
}

func TestOIDCCallbackSendsPKCEVerifierToTokenEndpoint(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	cookies, state := f.start("")
	// The fake token endpoint rejects a code_verifier that does not hash to the
	// challenge, as a provider enforcing PKCE does. Happy path covers the match.
	f.provider.set(func(p *fakeOIDCProvider) { p.challenge = "not-the-challenge" })

	f.wantLoginError(f.callback(cookies, url.Values{"state": {state}, "code": {fakeOIDCCode}}), "exchange_failed")
	// oauth2 retries once with the other client-auth style when it cannot tell
	// which the provider wants, so more than one call is fine.
	if f.provider.tokenCalls == 0 {
		t.Fatal("token endpoint was never called")
	}
}

func TestOIDCCallbackRejectsBadNonce(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	cookies, state := f.start("")
	f.provider.set(func(p *fakeOIDCProvider) { p.nonce = "replayed-nonce" })

	f.wantLoginError(f.callback(cookies, url.Values{"state": {state}, "code": {fakeOIDCCode}}), "invalid_nonce")
}

func TestOIDCCallbackRejectsWrongAudience(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	cookies, state := f.start("")
	f.provider.set(func(p *fakeOIDCProvider) { p.audience = "another-client" })

	f.wantLoginError(f.callback(cookies, url.Values{"state": {state}, "code": {fakeOIDCCode}}), "invalid_token")
}

func TestOIDCCallbackRefusesEmailOfUnlinkedAccount(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	if err := f.db.Create(&model.User{Email: "sso@example.com", PasswordHash: "x", FullName: "Existing"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	cookies, state := f.start("")

	rec := f.callback(cookies, url.Values{"state": {state}, "code": {fakeOIDCCode}})

	if rec.Header().Get("Location") != "http://localhost:5173/login?error=email_in_use" {
		t.Fatalf("callback redirected to %q, want email_in_use", rec.Header().Get("Location"))
	}
	var links int64
	if err := f.db.Model(&model.UserOIDCIdentity{}).Count(&links).Error; err != nil || links != 0 {
		t.Fatalf("identity rows = %d (err %v), want 0", links, err)
	}
}

func TestOIDCCallbackRefusesUnverifiedEmail(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	f.provider.emailVerified = false
	cookies, state := f.start("")

	f.wantLoginError(f.callback(cookies, url.Values{"state": {state}, "code": {fakeOIDCCode}}), "signin_failed")
}

func TestOIDCCallbackProviderError(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	cookies, state := f.start("")

	f.wantLoginError(f.callback(cookies, url.Values{"state": {state}, "error": {"access_denied"}}), "provider_error")
}

func TestOIDCStartIgnoresUnsafeRedirectPath(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	cookies, state := f.start("//evil.example/path")
	for _, c := range cookies {
		if c.Name == oidcRedirectCookie {
			t.Fatalf("unsafe redirect path was stored in cookie %+v", c)
		}
	}

	rec := f.callback(cookies, url.Values{"state": {state}, "code": {fakeOIDCCode}})
	if rec.Header().Get("Location") != "http://localhost:5173/workspaces" {
		t.Fatalf("callback redirected to %q, want the default app path", rec.Header().Get("Location"))
	}
}

func TestOIDCDiscoveryIsLazyAndRetriedAfterFailure(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	f.provider.set(func(p *fakeOIDCProvider) { p.down = true })

	// Configuring the handler never touched the provider, so the server booted.
	rec := httptest.NewRecorder()
	f.handler.OIDCStart(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/start", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "http://localhost:5173/login?error=provider_unavailable" {
		t.Fatalf("start while provider is down = %d -> %q, want provider_unavailable redirect", rec.Code, rec.Header().Get("Location"))
	}

	f.provider.set(func(p *fakeOIDCProvider) { p.down = false })
	cookies, state := f.start("")
	rec = f.callback(cookies, url.Values{"state": {state}, "code": {fakeOIDCCode}})
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "http://localhost:5173/workspaces" {
		t.Fatalf("sign-in after the provider recovered = %d -> %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestOIDCRoutesReturn404WhenNotConfigured(t *testing.T) {
	h := NewAuthHandler(nil)
	for name, handle := range map[string]http.HandlerFunc{"start": h.OIDCStart, "callback": h.OIDCCallback} {
		rec := httptest.NewRecorder()
		handle(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/"+name, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", name, rec.Code)
		}
	}
}

func TestAuthConfigReportsOIDCAndLocalLogin(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	read := func(h *AuthHandler) map[string]any {
		rec := httptest.NewRecorder()
		h.GetConfig(rec, httptest.NewRequest(http.MethodGet, "/api/auth/config", nil))
		var cfg map[string]any
		decodeJSONResponse(t, rec, &cfg)
		return cfg
	}

	cfg := read(f.handler)
	if cfg["oidc_login_enabled"] != true || cfg["oidc_provider_name"] != "Authelia" || cfg["local_login_enabled"] != true {
		t.Fatalf("config = %v, want oidc enabled as Authelia with local login on", cfg)
	}
	f.handler.SetLocalLoginDisabled(true)
	cfg = read(f.handler)
	if cfg["local_login_enabled"] != false || cfg["google_login_enabled"] != false {
		t.Fatalf("config = %v, want local login off and google untouched", cfg)
	}
	plain := read(NewAuthHandler(f.handler.authService))
	if plain["oidc_login_enabled"] != false || plain["local_login_enabled"] != true {
		t.Fatalf("default config = %v, want oidc off and local login on", plain)
	}
}

func TestLocalLoginDisabledRejectsLocalEndpoints(t *testing.T) {
	h := NewAuthHandler(nil)
	h.SetLocalLoginDisabled(true)
	passkey := NewPasskeyHandler(nil)
	passkey.SetLocalLoginDisabled(true)
	invite := NewInviteHandler(nil)
	invite.SetLocalLoginDisabled(true)

	endpoints := map[string]http.HandlerFunc{
		"signin":                 h.Signin,
		"signup":                 h.Signup,
		"forgot-password":        h.ForgotPassword,
		"reset-password":         h.ResetPassword,
		"2fa verify-signin":      h.Verify2FASignin,
		"passkey authentication": passkey.AuthenticationOptions,
		"passkey authenticate":   passkey.Authenticate,
		"invitation signup":      invite.AcceptWithSignup,
	}
	for name, handle := range endpoints {
		rec := httptest.NewRecorder()
		handle(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`))))
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "local sign-in is disabled") {
			t.Fatalf("%s = %d %s, want 403 local sign-in is disabled", name, rec.Code, rec.Body.String())
		}
	}
}
