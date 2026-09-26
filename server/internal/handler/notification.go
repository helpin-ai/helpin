package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// NotificationHandler handles notification HTTP endpoints.
type NotificationHandler struct {
	notifService    *service.NotificationService
	followerService *service.FollowerService
}

// NewNotificationHandler creates a new notification handler.
func NewNotificationHandler(notifService *service.NotificationService, followerService *service.FollowerService) *NotificationHandler {
	return &NotificationHandler{
		notifService:    notifService,
		followerService: followerService,
	}
}

// List returns paginated notifications for the current user.
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := getWorkspaceID(r)

	status := r.URL.Query().Get("status")
	filter := r.URL.Query().Get("filter")
	limitStr := r.URL.Query().Get("limit")
	cursorStr := r.URL.Query().Get("cursor")

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	var cursor *time.Time
	if cursorStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, cursorStr); err == nil {
			cursor = &t
		}
	}

	result, err := h.notifService.List(r.Context(), userID, workspaceID, status, filter, limit, cursor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetUnreadCount returns the unread notification count.
func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := getWorkspaceID(r)

	count, err := h.notifService.UnreadCount(r.Context(), userID, workspaceID, r.URL.Query().Get("timezone"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get unread count")
		return
	}

	writeJSON(w, http.StatusOK, model.UnreadCountResponse{Count: count})
}

// Update updates a notification (mark read/unread/archived, snooze).
func (h *NotificationHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	notifID := chi.URLParam(r, "notifId")

	var req model.UpdateNotificationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.notifService.Update(r.Context(), notifID, userID, req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update notification")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MarkAllRead marks all notifications as read.
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := getWorkspaceID(r)

	if err := h.notifService.MarkAllAsRead(r.Context(), userID, workspaceID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark all as read")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ArchiveAllRead archives all read notifications.
func (h *NotificationHandler) ArchiveAllRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := getWorkspaceID(r)

	if err := h.notifService.ArchiveAllRead(r.Context(), userID, workspaceID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive all read")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Delete deletes a notification.
func (h *NotificationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	notifID := chi.URLParam(r, "notifId")

	if err := h.notifService.Delete(r.Context(), notifID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete notification")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetPreferences returns the current user's notification preferences.
func (h *NotificationHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := getWorkspaceID(r)

	prefs, err := h.notifService.GetPreferences(r.Context(), userID, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get preferences")
		return
	}

	writeJSON(w, http.StatusOK, prefs)
}

// UpdatePreferences updates the current user's notification preferences.
func (h *NotificationHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := getWorkspaceID(r)

	var req model.UpdateNotificationPreferenceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.notifService.UpdatePreferences(r.Context(), userID, workspaceID, req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update preferences")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListFollowers returns followers of an entity.
func (h *NotificationHandler) ListFollowers(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityId")

	followers, err := h.followerService.ListFollowers(r.Context(), entityType, entityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list followers")
		return
	}

	writeJSON(w, http.StatusOK, followers)
}

// Follow follows an entity.
func (h *NotificationHandler) Follow(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := getWorkspaceID(r)
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityId")

	if err := h.followerService.Follow(r.Context(), userID, entityType, entityID, workspaceID, "manual"); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to follow entity")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Unfollow unfollows an entity.
func (h *NotificationHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityId")

	if err := h.followerService.Unfollow(r.Context(), userID, entityType, entityID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unfollow entity")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListFollowing returns all entities the current user follows.
func (h *NotificationHandler) ListFollowing(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	workspaceID := getWorkspaceID(r)

	following, err := h.followerService.ListUserFollowing(r.Context(), userID, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list following")
		return
	}

	writeJSON(w, http.StatusOK, following)
}

// IsFollowing checks if the current user follows an entity.
func (h *NotificationHandler) IsFollowing(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityId")

	following, err := h.followerService.IsFollowing(r.Context(), userID, entityType, entityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check following")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"following": following})
}
