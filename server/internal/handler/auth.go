package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// AuthHandler handles authentication HTTP requests.
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
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
