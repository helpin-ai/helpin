package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	apptotp "github.com/helpin-ai/helpin/server/internal/totp"
)

const testAuthEncryptionKey = "0123456789abcdef0123456789abcdef"

func TestGoogleMobileRedirectHelpers(t *testing.T) {
	h := NewAuthHandler(nil, GoogleOAuthConfig{
		AppBaseURL:       "https://app.helpin.ai",
		MobileAppBaseURL: "https://mobile.helpin.ai",
	})

	if got := googleOAuthClient("mobile_native"); got != "mobile_native" {
		t.Fatalf("googleOAuthClient(mobile_native) = %q", got)
	}
	if got := googleOAuthClient("attacker_redirect"); got != "" {
		t.Fatalf("googleOAuthClient accepted unknown client %q", got)
	}
	if got := googleNativeRedirect("code", "a+b/c"); got != "helpin://auth/google?code=a%2Bb%2Fc" {
		t.Fatalf("googleNativeRedirect = %q", got)
	}
	if got := h.googleAuthFailureRedirect("mobile_native", "invalid_state"); got != "helpin://auth/google?error=invalid_state" {
		t.Fatalf("native failure redirect = %q", got)
	}
	if got := h.googleAuthFailureRedirect("mobile_web", "invalid_state"); got != "https://mobile.helpin.ai/login?google_error=invalid_state" {
		t.Fatalf("mobile web failure redirect = %q", got)
	}
}

func TestAuthHandler_TwoFASetupToSigninFlow(t *testing.T) {
	h, userID := newAuthHandlerTestFixture(t)

	setupReq := newAuthedJSONRequest(t, http.MethodPost, "/api/auth/2fa/setup", userID, model.TwoFASetupRequest{
		Password: "strongpass1",
	})
	setupRec := httptest.NewRecorder()
	h.Setup2FA(setupRec, setupReq)

	if setupRec.Code != http.StatusOK {
		t.Fatalf("setup status = %d, want %d, body = %s", setupRec.Code, http.StatusOK, setupRec.Body.String())
	}

	var setupResp model.TwoFASetupResponse
	decodeJSONResponse(t, setupRec, &setupResp)
	if setupResp.ProvisioningURI == "" {
		t.Fatal("expected provisioning URI")
	}
	if len(setupResp.RecoveryCodes) != 10 {
		t.Fatalf("recovery code count = %d, want 10", len(setupResp.RecoveryCodes))
	}

	secret := provisioningSecret(t, setupResp.ProvisioningURI)
	code := generateCurrentTOTP(t, secret)

	verifyReq := newAuthedJSONRequest(t, http.MethodPost, "/api/auth/2fa/verify", userID, model.TwoFAVerifyRequest{
		TOTPCode: code,
	})
	verifyRec := httptest.NewRecorder()
	h.Verify2FA(verifyRec, verifyReq)

	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify setup status = %d, want %d, body = %s", verifyRec.Code, http.StatusOK, verifyRec.Body.String())
	}

	statusReq := newAuthedJSONRequest(t, http.MethodGet, "/api/auth/2fa/status", userID, nil)
	statusRec := httptest.NewRecorder()
	h.Get2FAStatus(statusRec, statusReq)

	if statusRec.Code != http.StatusOK {
		t.Fatalf("status status = %d, want %d, body = %s", statusRec.Code, http.StatusOK, statusRec.Body.String())
	}

	var statusResp model.TwoFAStatusResponse
	decodeJSONResponse(t, statusRec, &statusResp)
	if !statusResp.Enabled {
		t.Fatal("expected two-factor authentication to be enabled")
	}

	signinReq := newJSONRequest(t, http.MethodPost, "/api/auth/signin", model.SigninRequest{
		Email:    "alice@example.com",
		Password: "strongpass1",
	})
	signinRec := httptest.NewRecorder()
	h.Signin(signinRec, signinReq)

	if signinRec.Code != http.StatusOK {
		t.Fatalf("signin status = %d, want %d, body = %s", signinRec.Code, http.StatusOK, signinRec.Body.String())
	}

	var signinResp model.SigninResponse
	decodeJSONResponse(t, signinRec, &signinResp)
	if !signinResp.Requires2FA {
		t.Fatal("expected signin to require 2fa")
	}
	if signinResp.TwoFAToken == "" {
		t.Fatal("expected short-lived two_fa_token")
	}
	if signinResp.AccessToken != "" || signinResp.RefreshToken != "" || signinResp.User != nil {
		t.Fatal("did not expect full auth session before 2fa verification")
	}

	verifySigninReq := newJSONRequest(t, http.MethodPost, "/api/auth/2fa/verify-signin", model.TwoFASigninRequest{
		TwoFAToken: signinResp.TwoFAToken,
		TOTPCode:   generateCurrentTOTP(t, secret),
	})
	verifySigninRec := httptest.NewRecorder()
	h.Verify2FASignin(verifySigninRec, verifySigninReq)

	if verifySigninRec.Code != http.StatusOK {
		t.Fatalf("verify signin status = %d, want %d, body = %s", verifySigninRec.Code, http.StatusOK, verifySigninRec.Body.String())
	}

	var authResp model.AuthResponse
	decodeJSONResponse(t, verifySigninRec, &authResp)
	if authResp.AccessToken == "" || authResp.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens after 2fa verification")
	}
	if authResp.User.Email != "alice@example.com" {
		t.Fatalf("user email = %q, want %q", authResp.User.Email, "alice@example.com")
	}
	if !authResp.User.TwoFAEnabled {
		t.Fatal("expected authenticated user profile to report two_fa_enabled")
	}
}

func TestAuthHandler_DisableTwoFARestoresPasswordOnlySignin(t *testing.T) {
	h, userID := newAuthHandlerTestFixture(t)
	enableTwoFAForUser(t, h, userID)

	disableReq := newAuthedJSONRequest(t, http.MethodDelete, "/api/auth/2fa", userID, model.TwoFADisableRequest{
		Password: "strongpass1",
	})
	disableRec := httptest.NewRecorder()
	h.Disable2FA(disableRec, disableReq)

	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status = %d, want %d, body = %s", disableRec.Code, http.StatusOK, disableRec.Body.String())
	}

	statusReq := newAuthedJSONRequest(t, http.MethodGet, "/api/auth/2fa/status", userID, nil)
	statusRec := httptest.NewRecorder()
	h.Get2FAStatus(statusRec, statusReq)

	if statusRec.Code != http.StatusOK {
		t.Fatalf("status status = %d, want %d, body = %s", statusRec.Code, http.StatusOK, statusRec.Body.String())
	}

	var statusResp model.TwoFAStatusResponse
	decodeJSONResponse(t, statusRec, &statusResp)
	if statusResp.Enabled {
		t.Fatal("expected two-factor authentication to be disabled")
	}

	signinReq := newJSONRequest(t, http.MethodPost, "/api/auth/signin", model.SigninRequest{
		Email:    "alice@example.com",
		Password: "strongpass1",
	})
	signinRec := httptest.NewRecorder()
	h.Signin(signinRec, signinReq)

	if signinRec.Code != http.StatusOK {
		t.Fatalf("signin status = %d, want %d, body = %s", signinRec.Code, http.StatusOK, signinRec.Body.String())
	}

	var signinResp model.SigninResponse
	decodeJSONResponse(t, signinRec, &signinResp)
	if signinResp.Requires2FA {
		t.Fatal("did not expect 2fa challenge after disabling 2fa")
	}
	if signinResp.AccessToken == "" || signinResp.RefreshToken == "" {
		t.Fatal("expected direct auth tokens after disabling 2fa")
	}
	if signinResp.User == nil {
		t.Fatal("expected user profile after password-only signin")
	}
	if signinResp.User.TwoFAEnabled {
		t.Fatal("expected user profile to report two_fa_enabled = false")
	}
}

func newAuthHandlerTestFixture(t *testing.T) (*AuthHandler, string) {
	t.Helper()

	dbName := "file:handler-auth-test-" + time.Now().UTC().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		full_name TEXT NOT NULL,
		email_verified_at DATETIME,
		google_subject TEXT UNIQUE,
		avatar_url TEXT,
		avatar_style TEXT,
		avatar_seed TEXT,
		avatar_background_mode TEXT,
		avatar_background_color TEXT,
		default_workspace_id TEXT,
		totp_secret_encrypted TEXT,
		totp_verified BOOLEAN NOT NULL DEFAULT 0,
		recovery_codes_encrypted TEXT,
		is_platform_admin BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create users table: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")
	authService := service.NewAuthService(
		userRepo,
		nil,
		nil,
		nil,
		nil,
		jwtManager,
		nil,
		nil,
		"http://localhost:5173",
		[]byte(testAuthEncryptionKey),
	)

	resp, err := authService.Signup(context.Background(), model.SignupRequest{
		Email:    "alice@example.com",
		Password: "strongpass1",
		FullName: "Alice Example",
	})
	if err != nil {
		t.Fatalf("signup fixture user: %v", err)
	}

	return NewAuthHandler(authService), resp.User.ID
}

func enableTwoFAForUser(t *testing.T, h *AuthHandler, userID string) string {
	t.Helper()

	setupReq := newAuthedJSONRequest(t, http.MethodPost, "/api/auth/2fa/setup", userID, model.TwoFASetupRequest{
		Password: "strongpass1",
	})
	setupRec := httptest.NewRecorder()
	h.Setup2FA(setupRec, setupReq)
	if setupRec.Code != http.StatusOK {
		t.Fatalf("setup status = %d, want %d, body = %s", setupRec.Code, http.StatusOK, setupRec.Body.String())
	}

	var setupResp model.TwoFASetupResponse
	decodeJSONResponse(t, setupRec, &setupResp)
	secret := provisioningSecret(t, setupResp.ProvisioningURI)

	verifyReq := newAuthedJSONRequest(t, http.MethodPost, "/api/auth/2fa/verify", userID, model.TwoFAVerifyRequest{
		TOTPCode: generateCurrentTOTP(t, secret),
	})
	verifyRec := httptest.NewRecorder()
	h.Verify2FA(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want %d, body = %s", verifyRec.Code, http.StatusOK, verifyRec.Body.String())
	}

	return secret
}

func newJSONRequest(t *testing.T, method, target string, body any) *http.Request {
	t.Helper()

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, target, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func newAuthedJSONRequest(t *testing.T, method, target, userID string, body any) *http.Request {
	t.Helper()

	req := newJSONRequest(t, method, target, body)
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	return req
}

func decodeJSONResponse(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
}

func provisioningSecret(t *testing.T, provisioningURI string) string {
	t.Helper()

	parsed, err := url.Parse(provisioningURI)
	if err != nil {
		t.Fatalf("parse provisioning URI: %v", err)
	}
	secret := parsed.Query().Get("secret")
	if secret == "" {
		t.Fatal("expected secret in provisioning URI")
	}
	return secret
}

func generateCurrentTOTP(t *testing.T, secret string) string {
	t.Helper()

	code, err := apptotp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate current totp: %v", err)
	}
	return code
}
