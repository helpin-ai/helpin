package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// AuthHandler handles authentication HTTP requests.
type AuthHandler struct {
	publicWidgetURL  string
	publicSDKURL     string
	authService      *service.AuthService
	googleOAuth      *oauth2.Config
	appBaseURL       string
	mobileAppBaseURL string
}

type GoogleOAuthConfig struct {
	ClientID         string
	ClientSecret     string
	RedirectURL      string
	AppBaseURL       string
	MobileAppBaseURL string
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService *service.AuthService, googleConfig ...GoogleOAuthConfig) *AuthHandler {
	h := &AuthHandler{authService: authService}
	if len(googleConfig) > 0 {
		cfg := googleConfig[0]
		h.appBaseURL = strings.TrimRight(strings.TrimSpace(cfg.AppBaseURL), "/")
		h.mobileAppBaseURL = strings.TrimRight(strings.TrimSpace(cfg.MobileAppBaseURL), "/")
		if strings.TrimSpace(cfg.ClientID) != "" && strings.TrimSpace(cfg.ClientSecret) != "" && strings.TrimSpace(cfg.RedirectURL) != "" {
			h.googleOAuth = &oauth2.Config{
				ClientID:     strings.TrimSpace(cfg.ClientID),
				ClientSecret: strings.TrimSpace(cfg.ClientSecret),
				RedirectURL:  strings.TrimSpace(cfg.RedirectURL),
				Scopes:       []string{"openid", "email", "profile"},
				Endpoint:     google.Endpoint,
			}
		}
	}
	return h
}

// Signup handles POST /api/auth/signup.
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req model.SignupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authService.Signup(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	setAuthCookies(w, r, resp.AccessToken, resp.RefreshToken)
	writeJSON(w, http.StatusCreated, resp)
}

// VerifyEmail handles POST /api/auth/verify-email.
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	profile, err := h.authService.VerifyEmail(r.Context(), req.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// ResendVerificationEmail handles POST /api/auth/resend-verification.
func (h *AuthHandler) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if err := h.authService.ResendEmailVerification(r.Context(), userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "verification email sent"})
}

// GoogleStart redirects the browser to Google's OAuth consent screen.
func (h *AuthHandler) GoogleStart(w http.ResponseWriter, r *http.Request) {
	if h.googleOAuth == nil {
		writeError(w, http.StatusNotFound, "google sign-in is not configured")
		return
	}
	state, err := randomState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start google sign-in")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "helpin_google_oauth_state",
		Value:    state,
		Path:     "/api/auth/google/callback",
		MaxAge:   600,
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
	if client := googleOAuthClient(r.URL.Query().Get("client")); client != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "helpin_google_oauth_client",
			Value:    client,
			Path:     "/api/auth/google/callback",
			MaxAge:   600,
			Expires:  time.Now().Add(10 * time.Minute),
			HttpOnly: true,
			Secure:   secureCookie(r),
			SameSite: http.SameSiteLaxMode,
		})
	}
	if redirectPath := sanitizeGoogleRedirectPath(r.URL.Query().Get("redirect")); redirectPath != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "helpin_google_oauth_redirect",
			Value:    base64.RawURLEncoding.EncodeToString([]byte(redirectPath)),
			Path:     "/api/auth/google/callback",
			MaxAge:   600,
			Expires:  time.Now().Add(10 * time.Minute),
			HttpOnly: true,
			Secure:   secureCookie(r),
			SameSite: http.SameSiteLaxMode,
		})
	}
	http.Redirect(w, r, h.googleOAuth.AuthCodeURL(state, oauth2.AccessTypeOnline), http.StatusFound)
}

// GoogleCallback handles the Google OAuth redirect.
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h.googleOAuth == nil {
		writeError(w, http.StatusNotFound, "google sign-in is not configured")
		return
	}
	client := googleOAuthClientFromCookie(r)
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	cookie, err := r.Cookie("helpin_google_oauth_state")
	if err != nil || state == "" || cookie.Value != state {
		clearGoogleOAuthCookies(w, r)
		http.Redirect(w, r, h.googleAuthFailureRedirect(client, "invalid_state"), http.StatusFound)
		return
	}
	redirectPath := googleRedirectPathFromCookie(r)
	clearGoogleOAuthCookies(w, r)
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		http.Redirect(w, r, h.googleAuthFailureRedirect(client, "missing_code"), http.StatusFound)
		return
	}
	token, err := h.googleOAuth.Exchange(r.Context(), code)
	if err != nil {
		slog.ErrorContext(r.Context(), "google oauth exchange failed", "error", err)
		http.Redirect(w, r, h.googleAuthFailureRedirect(client, "exchange_failed"), http.StatusFound)
		return
	}
	identity, err := googleIdentityFromToken(r, token)
	if err != nil {
		slog.ErrorContext(r.Context(), "google userinfo failed", "error", err)
		http.Redirect(w, r, h.googleAuthFailureRedirect(client, "userinfo_failed"), http.StatusFound)
		return
	}
	resp, err := h.authService.SignInWithGoogle(r.Context(), identity)
	if err != nil {
		slog.ErrorContext(r.Context(), "google auth signin failed", "error", err)
		http.Redirect(w, r, h.googleAuthFailureRedirect(client, "signin_failed"), http.StatusFound)
		return
	}
	if client == "mobile_native" {
		handoffCode, err := h.authService.CreateOAuthMobileHandoff(r.Context(), resp.User.ID)
		if err != nil {
			slog.ErrorContext(r.Context(), "google mobile handoff creation failed", "error", err, "user_id", resp.User.ID)
			http.Redirect(w, r, h.googleAuthFailureRedirect(client, "handoff_failed"), http.StatusFound)
			return
		}
		http.Redirect(w, r, googleNativeRedirect("code", handoffCode), http.StatusFound)
		return
	}
	setAuthCookies(w, r, resp.AccessToken, resp.RefreshToken)
	if client == "mobile_web" {
		http.Redirect(w, r, h.mobileAppRedirect("/login?google=success"), http.StatusFound)
		return
	}
	http.Redirect(w, r, h.appRedirect(redirectPath), http.StatusFound)
}

// GoogleMobileExchange turns a single-use native deep-link code into the
// normal JSON auth response consumed by the mobile session store.
func (h *AuthHandler) GoogleMobileExchange(w http.ResponseWriter, r *http.Request) {
	var req model.OAuthMobileExchangeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.authService.ExchangeOAuthMobileHandoff(r.Context(), req.Code)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	setAuthCookies(w, r, resp.AccessToken, resp.RefreshToken)
	writeJSON(w, http.StatusOK, resp)
}

// Signin handles POST /api/auth/signin.
func (h *AuthHandler) Signin(w http.ResponseWriter, r *http.Request) {
	var req model.SigninRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authService.Signin(r.Context(), req)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	setAuthCookies(w, r, resp.AccessToken, resp.RefreshToken)
	writeJSON(w, http.StatusOK, resp)
}

// DemoSignin handles POST /api/auth/demo. It signs the visitor in as the shared
// read-only demo viewer without a password. Disabled unless DEMO_VIEWER_EMAIL is set.
func (h *AuthHandler) DemoSignin(w http.ResponseWriter, r *http.Request) {
	var req model.DemoSigninRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	resp, err := h.authService.DemoSignin(r.Context(), req)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	setAuthCookies(w, r, resp.AccessToken, resp.RefreshToken)
	writeJSON(w, http.StatusOK, resp)
}

// ForgotPassword handles POST /api/auth/forgot-password.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req model.ForgotPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	slog.InfoContext(r.Context(), "forgot password submit received",
		"host", r.Host,
		"origin", r.Header.Get("Origin"),
		"referer", r.Referer(),
		"email_present", strings.TrimSpace(req.Email) != "",
	)

	if err := h.authService.ForgotPassword(r.Context(), req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "If an account with that email exists, a password reset link has been sent"})
}

// ResetPassword handles POST /api/auth/reset-password.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req model.ResetPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	slog.InfoContext(r.Context(), "password reset submit received",
		"host", r.Host,
		"origin", r.Header.Get("Origin"),
		"referer", r.Referer(),
		"token_len", len(strings.TrimSpace(req.Token)),
	)

	if err := h.authService.ResetPassword(r.Context(), req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "password reset"})
}

// Me handles GET /api/auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	profile, err := h.authService.GetProfile(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if claims := middleware.ClaimsFrom(r.Context()); claims != nil {
		profile.MFASatisfiedInToken = claims.MFASatisfied
	}

	writeJSON(w, http.StatusOK, profile)
}

// UpdateProfile handles PUT /api/auth/me.
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.UpdateProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	profile, err := h.authService.UpdateProfile(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// ChangePassword handles PUT /api/auth/change-password.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.ChangePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.authService.ChangePassword(r.Context(), userID, req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "password updated"})
}

// UploadAvatar handles POST /api/auth/me/avatar.
func (h *AuthHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 2MB)")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing avatar file")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/webp" && contentType != "image/svg+xml" {
		writeError(w, http.StatusBadRequest, "only PNG, JPEG, WebP, and SVG images are allowed")
		return
	}

	profile, err := h.authService.UploadAvatar(r.Context(), userID, file, header.Size, contentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// DeleteAvatar handles DELETE /api/auth/me/avatar.
func (h *AuthHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	profile, err := h.authService.DeleteAvatar(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// RefreshToken handles POST /api/auth/refresh.
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshTokenRequest
	if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		if cookie, err := r.Cookie(refreshTokenCookieName); err == nil {
			refreshToken = strings.TrimSpace(cookie.Value)
		}
	}

	resp, err := h.authService.RefreshToken(r.Context(), refreshToken)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	setAuthCookies(w, r, resp.AccessToken, resp.RefreshToken)
	writeJSON(w, http.StatusOK, resp)
}

// Signout handles POST /api/auth/signout.
func (h *AuthHandler) Signout(w http.ResponseWriter, r *http.Request) {
	clearAuthCookies(w, r)
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "signed out"})
}

func randomState() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func clearGoogleOAuthCookies(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "helpin_google_oauth_state",
		Value:    "",
		Path:     "/api/auth/google/callback",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "helpin_google_oauth_redirect",
		Value:    "",
		Path:     "/api/auth/google/callback",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "helpin_google_oauth_client",
		Value:    "",
		Path:     "/api/auth/google/callback",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func googleOAuthClient(raw string) string {
	switch strings.TrimSpace(raw) {
	case "mobile_native", "mobile_web":
		return strings.TrimSpace(raw)
	default:
		return ""
	}
}

func googleOAuthClientFromCookie(r *http.Request) string {
	cookie, err := r.Cookie("helpin_google_oauth_client")
	if err != nil {
		return ""
	}
	return googleOAuthClient(cookie.Value)
}

func googleRedirectPathFromCookie(r *http.Request) string {
	cookie, err := r.Cookie("helpin_google_oauth_redirect")
	if err != nil {
		return "/workspaces"
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return "/workspaces"
	}
	if path := sanitizeGoogleRedirectPath(string(raw)); path != "" {
		return path
	}
	return "/workspaces"
}

func sanitizeGoogleRedirectPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\r\n\t") {
		return ""
	}
	return path
}

func (h *AuthHandler) appRedirect(path string) string {
	base := h.appBaseURL
	if base == "" {
		base = "http://localhost:5173"
	}
	return strings.TrimRight(base, "/") + path
}

func (h *AuthHandler) mobileAppRedirect(path string) string {
	base := h.mobileAppBaseURL
	if base == "" {
		base = h.appBaseURL
	}
	if base == "" {
		base = "http://localhost:5176"
	}
	return strings.TrimRight(base, "/") + path
}

func googleNativeRedirect(key, value string) string {
	query := url.Values{}
	query.Set(key, value)
	return "helpin://auth/google?" + query.Encode()
}

func (h *AuthHandler) googleAuthFailureRedirect(client, reason string) string {
	switch client {
	case "mobile_native":
		return googleNativeRedirect("error", reason)
	case "mobile_web":
		return h.mobileAppRedirect("/login?google_error=" + url.QueryEscape(reason))
	default:
		return h.authFailureRedirect(reason)
	}
}

func (h *AuthHandler) authFailureRedirect(reason string) string {
	return h.appRedirect("/login?error=" + reason)
}

func googleIdentityFromToken(r *http.Request, token *oauth2.Token) (service.GoogleIdentity, error) {
	client := oauth2.NewClient(r.Context(), oauth2.StaticTokenSource(token))
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return service.GoogleIdentity{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return service.GoogleIdentity{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return service.GoogleIdentity{}, errors.New("google userinfo returned non-success status")
	}
	var payload struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return service.GoogleIdentity{}, err
	}
	return service.GoogleIdentity{
		Subject:       payload.Subject,
		Email:         payload.Email,
		EmailVerified: payload.EmailVerified,
		FullName:      payload.Name,
	}, nil
}

// Get2FAStatus handles GET /api/auth/2fa/status.
func (h *AuthHandler) Get2FAStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	resp, err := h.authService.Get2FAStatus(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Setup2FA handles POST /api/auth/2fa/setup.
func (h *AuthHandler) Setup2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.TwoFASetupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authService.Setup2FA(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Verify2FA handles POST /api/auth/2fa/verify.
func (h *AuthHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.TwoFAVerifyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authService.Verify2FASetupWithSession(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Verify2FASignin handles POST /api/auth/2fa/verify-signin.
func (h *AuthHandler) Verify2FASignin(w http.ResponseWriter, r *http.Request) {
	var req model.TwoFASigninRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authService.Verify2FASignin(r.Context(), req)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	setAuthCookies(w, r, resp.AccessToken, resp.RefreshToken)
	writeJSON(w, http.StatusOK, resp)
}

// StepUp2FA handles POST /api/auth/2fa/step-up.
func (h *AuthHandler) StepUp2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.TwoFAStepUpRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authService.StepUp2FA(r.Context(), userID, req)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		writeErrorCode(w, http.StatusUnauthorized, "invalid credentials", "invalid_credentials")
	case errors.Is(err, service.ErrTwoFAUnavailable):
		writeErrorCode(w, http.StatusServiceUnavailable, err.Error(), "two_factor_unavailable")
	case errors.Is(err, service.ErrDemoDisabled):
		writeErrorCode(w, http.StatusNotFound, err.Error(), "demo_disabled")
	case errors.Is(err, service.ErrBadRequest):
		writeErrorCode(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrBadRequest.Error()+": "), "bad_request")
	default:
		writeErrorCode(w, http.StatusUnauthorized, err.Error(), "auth_failed")
	}
}

// Disable2FA handles DELETE /api/auth/2fa.
func (h *AuthHandler) Disable2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.TwoFADisableRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.authService.Disable2FA(r.Context(), userID, req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "two-factor authentication disabled"})
}

// RegenerateRecoveryCodes handles POST /api/auth/2fa/regenerate-recovery-codes.
func (h *AuthHandler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.TwoFARegenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authService.RegenerateRecoveryCodes(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetConfig exposes non-secret authentication capabilities to prebuilt clients.
func (h *AuthHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"public_widget_url":           h.publicWidgetURL,
		"public_sdk_url":              h.publicSDKURL,
		"email_verification_required": h.authService.EmailVerificationRequired(),
		"app_email_configured":        h.authService.AppEmailConfigured(),
		"google_login_enabled":        h.googleOAuth != nil,
		"demo_enabled":                h.authService.DemoEnabled(),
		"demo_requires_email":         h.authService.DemoRequiresEmail(),
	})
}

func (h *AuthHandler) SetPublicWidgetURLs(widget, sdk string) {
	h.publicWidgetURL = widget
	h.publicSDKURL = sdk
}
