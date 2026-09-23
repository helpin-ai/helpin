package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CapabilityHandler serves capability statuses and the test email action.
type CapabilityHandler struct {
	service *service.CapabilityService
}

// NewCapabilityHandler creates a CapabilityHandler.
func NewCapabilityHandler(capabilities *service.CapabilityService) *CapabilityHandler {
	return &CapabilityHandler{service: capabilities}
}

// Workspace returns the capabilities visible to a workspace member.
// GET /api/workspaces/{id}/capabilities
func (h *CapabilityHandler) Workspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	response, err := h.service.Workspace(r.Context(), workspaceID)
	if err != nil {
		slog.ErrorContext(r.Context(), "read workspace capabilities failed", "error", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "failed to read capabilities")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// Instance returns coarse instance capabilities for operator tooling. It is
// mounted behind the internal API secret.
// GET /api/instance/capabilities
func (h *CapabilityHandler) Instance(w http.ResponseWriter, r *http.Request) {
	response, err := h.service.Instance(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "read instance capabilities failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read capabilities")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// SendTestEmail sends a test email to the requesting user's own address.
// POST /api/workspaces/{id}/email/test
func (h *CapabilityHandler) SendTestEmail(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	result, err := h.service.SendTestEmail(r.Context(), workspaceID, userID)
	switch {
	case errors.Is(err, service.ErrTestEmailRateLimited):
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, model.TestEmailResult{Error: err.Error()})
	case errors.Is(err, service.ErrTestEmailNoRecipient):
		writeJSON(w, http.StatusUnprocessableEntity, model.TestEmailResult{Error: err.Error()})
	case err != nil:
		slog.ErrorContext(r.Context(), "send test email failed", "error", err, "workspace_id", workspaceID)
		writeJSON(w, http.StatusInternalServerError, model.TestEmailResult{Error: "failed to send test email"})
	default:
		writeJSON(w, http.StatusOK, result)
	}
}
