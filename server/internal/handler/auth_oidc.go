package handler

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/helpin-ai/helpin/server/internal/service"
)

const (
	oidcCallbackPath      = "/api/auth/oidc/callback"
	oidcStateCookie       = "helpin_oidc_state"
	oidcNonceCookie       = "helpin_oidc_nonce"
	oidcVerifierCookie    = "helpin_oidc_verifier"
	oidcRedirectCookie    = "helpin_oidc_redirect"
	oidcCookieTTL         = 10 * time.Minute
	oidcDiscoveryTimeout  = 10 * time.Second
	localLoginDisabledMsg = "local sign-in is disabled"
)

var oidcCookieNames = []string{oidcStateCookie, oidcNonceCookie, oidcVerifierCookie, oidcRedirectCookie}

// OIDCConfig configures generic OpenID Connect sign-in.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	ProviderName string
	Scopes       []string
	// HTTPClient is used for discovery, token and userinfo requests; nil uses
	// the default client.
	HTTPClient *http.Client
}

// oidcClient discovers the provider on first use, so the server starts while
// the identity provider is down, and caches it once discovery succeeds.
type oidcClient struct {
	cfg       OIDCConfig
	mu        sync.Mutex // guards provider only; never held across network I/O
	provider  *oidc.Provider
	discovery singleflight.Group
}

func (c *oidcClient) clientContext(ctx context.Context) context.Context {
	if c.cfg.HTTPClient != nil {
		return oidc.ClientContext(ctx, c.cfg.HTTPClient)
	}
	return ctx
}

// discover returns the cached provider, or discovers it once however many
// requests arrive meanwhile. A waiter gives up when its own request ends, so a
// hung provider never pins requests behind a lock. Failures are not cached.
func (c *oidcClient) discover(ctx context.Context) (*oidc.Provider, error) {
	c.mu.Lock()
	provider := c.provider
	c.mu.Unlock()
	if provider != nil {
		return provider, nil
	}
	result := c.discovery.DoChan("discover", func() (any, error) {
		// Detached from the first caller's request, so cancelling that request
		// does not fail the callers sharing this discovery.
		dctx, cancel := context.WithTimeout(context.Background(), oidcDiscoveryTimeout)
		defer cancel()
		found, err := oidc.NewProvider(c.clientContext(dctx), c.cfg.IssuerURL)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.provider = found
		c.mu.Unlock()
		return found, nil
	})
	select {
	case res := <-result:
		if res.Err != nil {
			return nil, res.Err
		}
		return res.Val.(*oidc.Provider), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *oidcClient) oauthConfig(provider *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		RedirectURL:  c.cfg.RedirectURL,
		Scopes:       c.cfg.Scopes,
		Endpoint:     provider.Endpoint(),
	}
}

// SetOIDC enables OpenID Connect sign-in. Without it the OIDC routes return 404.
func (h *AuthHandler) SetOIDC(cfg OIDCConfig) {
	cfg.IssuerURL = strings.TrimSpace(cfg.IssuerURL)
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	cfg.RedirectURL = strings.TrimSpace(cfg.RedirectURL)
	cfg.ProviderName = strings.TrimSpace(cfg.ProviderName)
	if cfg.IssuerURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		h.oidc = nil
		return
	}
	if cfg.ProviderName == "" {
		cfg.ProviderName = "Single sign-on"
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	}
	h.oidc = &oidcClient{cfg: cfg}
}

func (h *AuthHandler) oidcProviderName() string {
	if h.oidc == nil {
		return ""
	}
	return h.oidc.cfg.ProviderName
}

// SetLocalLoginDisabled turns off password signin and signup, password reset and two-factor signin.
func (h *AuthHandler) SetLocalLoginDisabled(disabled bool) {
	h.localLoginDisabled = disabled
}

// rejectLocalLoginDisabled writes a 403 and reports true when local sign-in is off.
func (h *AuthHandler) rejectLocalLoginDisabled(w http.ResponseWriter) bool {
	if !h.localLoginDisabled {
		return false
	}
	writeErrorCode(w, http.StatusForbidden, localLoginDisabledMsg, "local_login_disabled")
	return true
}

// OIDCStart redirects the browser to the identity provider's authorization endpoint.
func (h *AuthHandler) OIDCStart(w http.ResponseWriter, r *http.Request) {
	if h.oidc == nil {
		writeError(w, http.StatusNotFound, "oidc sign-in is not configured")
		return
	}
	provider, err := h.oidc.discover(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "oidc provider discovery failed", "error", err)
		http.Redirect(w, r, h.authFailureRedirect("provider_unavailable"), http.StatusFound)
		return
	}
	state, err := randomState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start oidc sign-in")
		return
	}
	nonce, err := randomState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start oidc sign-in")
		return
	}
	verifier := oauth2.GenerateVerifier()

	setOIDCCookie(w, r, oidcStateCookie, state)
	setOIDCCookie(w, r, oidcNonceCookie, nonce)
	setOIDCCookie(w, r, oidcVerifierCookie, verifier)
	if redirectPath := sanitizeRedirectPath(r.URL.Query().Get("redirect")); redirectPath != "" {
		setOIDCCookie(w, r, oidcRedirectCookie, base64.RawURLEncoding.EncodeToString([]byte(redirectPath)))
	}
	requestHost := r.Host
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		requestHost = forwarded
	}
	if redirectHost := redirectURLHost(h.oidc.cfg.RedirectURL); redirectHost != "" && !strings.EqualFold(redirectHost, requestHost) {
		// The state cookies are scoped to this host, so the callback must come
		// back to it; a different redirect host breaks sign-in.
		slog.WarnContext(r.Context(), "oidc sign-in started on a different host than OIDC_REDIRECT_URL",
			"request_host", requestHost, "redirect_url_host", redirectHost)
	}
	authURL := h.oidc.oauthConfig(provider).AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oidc.Nonce(nonce))
	http.Redirect(w, r, authURL, http.StatusFound)
}

// OIDCCallback completes the authorization code flow and signs the user in.
func (h *AuthHandler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if h.oidc == nil {
		writeError(w, http.StatusNotFound, "oidc sign-in is not configured")
		return
	}
	cookies := readOIDCCookies(r)
	clearOIDCCookies(w, r)
	fail := func(reason string) {
		http.Redirect(w, r, h.authFailureRedirect(reason), http.StatusFound)
	}

	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if state == "" || cookies.state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookies.state)) != 1 {
		fail("invalid_state")
		return
	}
	if cookies.nonce == "" || cookies.verifier == "" {
		fail("invalid_state")
		return
	}
	if providerErr := r.URL.Query().Get("error"); providerErr != "" {
		slog.InfoContext(r.Context(), "oidc provider returned an error", "error", providerErr, "description", r.URL.Query().Get("error_description"))
		fail("provider_error")
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		fail("missing_code")
		return
	}

	provider, err := h.oidc.discover(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "oidc provider discovery failed", "error", err)
		fail("provider_unavailable")
		return
	}
	ctx := h.oidc.clientContext(r.Context())
	token, err := h.oidc.oauthConfig(provider).Exchange(ctx, code, oauth2.VerifierOption(cookies.verifier))
	if err != nil {
		slog.ErrorContext(r.Context(), "oidc token exchange failed", "error", err)
		fail("exchange_failed")
		return
	}
	identity, reason, err := h.oidcIdentity(ctx, provider, token, cookies.nonce)
	if err != nil {
		slog.ErrorContext(r.Context(), "oidc identity verification failed", "reason", reason, "error", err)
		fail(reason)
		return
	}

	resp, err := h.authService.SignInWithOIDC(r.Context(), identity)
	switch {
	case err == nil:
	case isSignupRestricted(err):
		slog.InfoContext(r.Context(), "oidc signup rejected by signup policy", "error", err)
		fail("signup_restricted")
		return
	case errors.Is(err, service.ErrOIDCEmailInUse):
		fail("email_in_use")
		return
	default:
		slog.ErrorContext(r.Context(), "oidc signin failed", "error", err)
		fail("signin_failed")
		return
	}
	setAuthCookies(w, r, resp.AccessToken, resp.RefreshToken)
	http.Redirect(w, r, h.appRedirect(oidcRedirectPath(cookies.redirect)), http.StatusFound)
}

// oidcIdentity verifies the ID token (issuer, audience, expiry, signature and
// nonce) and returns the normalized identity. Claims the ID token lacks, such
// as the email on providers that only serve it from userinfo, come from the
// userinfo endpoint. On error the returned string is the login error reason.
func (h *AuthHandler) oidcIdentity(ctx context.Context, provider *oidc.Provider, token *oauth2.Token, nonce string) (service.OIDCIdentity, string, error) {
	rawIDToken, _ := token.Extra("id_token").(string)
	if rawIDToken == "" {
		return service.OIDCIdentity{}, "invalid_token", errors.New("token response has no id_token")
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: h.oidc.cfg.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return service.OIDCIdentity{}, "invalid_token", fmt.Errorf("verify id_token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return service.OIDCIdentity{}, "invalid_nonce", errors.New("id_token nonce mismatch")
	}
	var claims struct {
		Email         string    `json:"email"`
		EmailVerified *oidcBool `json:"email_verified"`
		Name          string    `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return service.OIDCIdentity{}, "invalid_token", fmt.Errorf("decode id_token claims: %w", err)
	}
	identity := service.OIDCIdentity{
		Issuer:   idToken.Issuer,
		Subject:  idToken.Subject,
		Email:    claims.Email,
		FullName: claims.Name,
	}
	if claims.EmailVerified != nil {
		identity.EmailVerified = bool(*claims.EmailVerified)
	}
	if claims.Email == "" || claims.EmailVerified == nil {
		info, err := provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if err != nil {
			return service.OIDCIdentity{}, "userinfo_failed", fmt.Errorf("fetch userinfo: %w", err)
		}
		if info.Subject != idToken.Subject {
			return service.OIDCIdentity{}, "userinfo_failed", errors.New("userinfo subject does not match id_token subject")
		}
		var infoClaims struct {
			Name string `json:"name"`
		}
		if err := info.Claims(&infoClaims); err != nil {
			return service.OIDCIdentity{}, "userinfo_failed", fmt.Errorf("decode userinfo claims: %w", err)
		}
		if identity.Email == "" {
			identity.Email = info.Email
			identity.EmailVerified = info.EmailVerified
		} else if claims.EmailVerified == nil {
			// Userinfo can vouch only for the address it returns, so its
			// verified flag counts only when that is the ID token address.
			sameAddress := strings.EqualFold(strings.TrimSpace(info.Email), strings.TrimSpace(identity.Email))
			identity.EmailVerified = sameAddress && info.EmailVerified
		}
		if identity.FullName == "" {
			identity.FullName = infoClaims.Name
		}
	}
	return identity, "", nil
}

// oidcBool decodes a JSON boolean that some providers (for example Cognito)
// send as the string "true" or "false".
type oidcBool bool

func (b *oidcBool) UnmarshalJSON(data []byte) error {
	switch strings.Trim(string(data), `"`) {
	case "true":
		*b = true
	case "false", "null":
		*b = false
	default:
		return fmt.Errorf("invalid boolean %s", data)
	}
	return nil
}

func redirectURLHost(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Host
}

type oidcCookieValues struct {
	state, nonce, verifier, redirect string
}

func readOIDCCookies(r *http.Request) oidcCookieValues {
	value := func(name string) string {
		if cookie, err := r.Cookie(name); err == nil {
			return cookie.Value
		}
		return ""
	}
	return oidcCookieValues{
		state:    value(oidcStateCookie),
		nonce:    value(oidcNonceCookie),
		verifier: value(oidcVerifierCookie),
		redirect: value(oidcRedirectCookie),
	}
}

func oidcRedirectPath(cookieValue string) string {
	raw, err := base64.RawURLEncoding.DecodeString(cookieValue)
	if err != nil {
		return "/workspaces"
	}
	if path := sanitizeRedirectPath(string(raw)); path != "" {
		return path
	}
	return "/workspaces"
}

func setOIDCCookie(w http.ResponseWriter, r *http.Request, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     oidcCallbackPath,
		MaxAge:   int(oidcCookieTTL.Seconds()),
		Expires:  time.Now().Add(oidcCookieTTL),
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOIDCCookies(w http.ResponseWriter, r *http.Request) {
	for _, name := range oidcCookieNames {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     oidcCallbackPath,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   secureCookie(r),
			SameSite: http.SameSiteLaxMode,
		})
	}
}
