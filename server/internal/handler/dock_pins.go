package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// ListPins returns personal pinned resources independently of roster pagination.
func (h *DockChatHandler) ListPins(w http.ResponseWriter, r *http.Request) {
	pins, err := h.dockChatService.ListPins(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()))
	if err != nil {
		slog.ErrorContext(r.Context(), "list dock pins", "error", err, "workspace_id", getWorkspaceID(r), "user_id", middleware.GetUserID(r.Context()))
		writeError(w, 500, "Unable to load pinned items")
		return
	}
	writeJSON(w, http.StatusOK, pins)
}

// SetPin updates only the authenticated user's pin preference.
func (h *DockChatHandler) SetPin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "resourceID")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, 400, "Invalid resource ID")
		return
	}
	var req struct {
		Pinned *bool `json:"pinned"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Pinned == nil {
		writeError(w, 400, "pinned is required")
		return
	}
	err := h.dockChatService.SetPin(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "kind"), id, *req.Pinned)
	if errors.Is(err, service.ErrDockPinKind) {
		writeError(w, 400, err.Error())
		return
	}
	if errors.Is(err, service.ErrDockChatNotFound) || errors.Is(err, service.ErrDockRunNotFound) {
		writeError(w, 404, "Item unavailable")
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "update dock pin", "error", err, "workspace_id", getWorkspaceID(r), "user_id", middleware.GetUserID(r.Context()))
		writeError(w, 500, "Unable to update pin")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"pinned": *req.Pinned})
}
