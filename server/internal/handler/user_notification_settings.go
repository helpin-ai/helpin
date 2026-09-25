package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// UserNotificationSettingsHandler handles account-level notification settings endpoints.
type UserNotificationSettingsHandler struct {
	service *service.UserNotificationSettingsService
}

// NewUserNotificationSettingsHandler creates a new handler.
func NewUserNotificationSettingsHandler(svc *service.UserNotificationSettingsService) *UserNotificationSettingsHandler {
	return &UserNotificationSettingsHandler{service: svc}
}

// Get returns the current user's account-level notification settings.
func (h *UserNotificationSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	settings, err := h.service.Get(r.Context(), userID, r.URL.Query().Get("timezone"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get notification settings")
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// Update updates the current user's account-level notification settings.
func (h *UserNotificationSettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.UpdateUserNotificationSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	settings, err := h.service.Update(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update notification settings")
		return
	}

	writeJSON(w, http.StatusOK, settings)
}
