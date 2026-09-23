package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// InstanceHandler serves self-hosted server administration (Community): the
// signup policy, server admins and application email settings. Every route
// requires a server admin.
type InstanceHandler struct {
	instance *service.InstanceService
	email    *service.AppEmailConfigService
}

// NewInstanceHandler creates an InstanceHandler.
func NewInstanceHandler(instance *service.InstanceService, email *service.AppEmailConfigService) *InstanceHandler {
	return &InstanceHandler{instance: instance, email: email}
}

// RequireServerAdmin rejects requests from accounts that are not server admins.
func (h *InstanceHandler) RequireServerAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		admin, err := h.instance.IsServerAdmin(r.Context(), middleware.GetUserID(r.Context()))
		if err != nil {
			slog.ErrorContext(r.Context(), "check server admin failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to check server admin")
			return
		}
		if !admin {
			writeErrorCode(w, http.StatusForbidden, "only server admins can manage this server", "server_admin_required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetSignupPolicy returns the signup policy.
// GET /api/instance/signup-policy
func (h *InstanceHandler) GetSignupPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := h.instance.SignupPolicy(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "read signup policy failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read signup policy")
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

// UpdateSignupPolicy changes the signup policy.
// PUT /api/instance/signup-policy
func (h *InstanceHandler) UpdateSignupPolicy(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateSignupPolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	policy, err := h.instance.UpdateSignupPolicy(r.Context(), middleware.GetUserID(r.Context()), req)
	if err != nil {
		h.writeInstanceError(w, r, err, "failed to update signup policy")
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

// ListAdmins returns the server admins.
// GET /api/instance/admins
func (h *InstanceHandler) ListAdmins(w http.ResponseWriter, r *http.Request) {
	admins, err := h.instance.ListAdmins(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list server admins failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list server admins")
		return
	}
	writeJSON(w, http.StatusOK, admins)
}

// GrantAdmin makes an existing account a server admin.
// POST /api/instance/admins
func (h *InstanceHandler) GrantAdmin(w http.ResponseWriter, r *http.Request) {
	var req model.GrantServerAdminRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	admins, err := h.instance.GrantAdmin(r.Context(), middleware.GetUserID(r.Context()), req.Email)
	if err != nil {
		h.writeInstanceError(w, r, err, "failed to add server admin")
		return
	}
	writeJSON(w, http.StatusOK, admins)
}

// RevokeAdmin removes server admin from an account.
// DELETE /api/instance/admins/{userID}
func (h *InstanceHandler) RevokeAdmin(w http.ResponseWriter, r *http.Request) {
	admins, err := h.instance.RevokeAdmin(r.Context(), middleware.GetUserID(r.Context()), chi.URLParam(r, "userID"))
	if err != nil {
		h.writeInstanceError(w, r, err, "failed to remove server admin")
		return
	}
	writeJSON(w, http.StatusOK, admins)
}

// GetEmailSettings returns application email settings without secrets.
// GET /api/instance/email
func (h *InstanceHandler) GetEmailSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.email.Settings(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "read email settings failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read email settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// UpdateEmailSettings saves application SMTP settings.
// PUT /api/instance/email
func (h *InstanceHandler) UpdateEmailSettings(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateAppEmailSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	settings, err := h.email.Save(r.Context(), middleware.GetUserID(r.Context()), req)
	if err != nil {
		h.writeInstanceError(w, r, err, "failed to save email settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// ClearEmailSettings removes the saved SMTP settings.
// DELETE /api/instance/email
func (h *InstanceHandler) ClearEmailSettings(w http.ResponseWriter, r *http.Request) {
	if err := h.email.Clear(r.Context(), middleware.GetUserID(r.Context())); err != nil {
		h.writeInstanceError(w, r, err, "failed to clear email settings")
		return
	}
	h.GetEmailSettings(w, r)
}

func (h *InstanceHandler) writeInstanceError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrNotServerAdmin):
		writeErrorCode(w, http.StatusForbidden, err.Error(), "server_admin_required")
	case errors.Is(err, service.ErrInvalidSignupPolicy):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrInvalidSignupPolicy.Error()+": "))
	case errors.Is(err, service.ErrInvalidAppEmailSettings):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrInvalidAppEmailSettings.Error()+": "))
	case errors.Is(err, service.ErrServerAdminUserNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrLastServerAdmin), errors.Is(err, service.ErrServerAdminFromEnv), errors.Is(err, service.ErrAppEmailSetByEnv):
		writeError(w, http.StatusConflict, err.Error())
	default:
		slog.ErrorContext(r.Context(), fallback, "error", err)
		writeError(w, http.StatusInternalServerError, fallback)
	}
}
