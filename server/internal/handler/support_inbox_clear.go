package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// supportInboxClearRecorder records an inbox clear and returns its stats.
type supportInboxClearRecorder interface {
	Record(ctx context.Context, workspaceID, userID string, req model.RecordSupportInboxClearRequest) (*model.SupportInboxClearStats, error)
}

// SupportInboxClearHandler serves the inbox-zero celebration endpoint.
type SupportInboxClearHandler struct {
	clears supportInboxClearRecorder
}

// NewSupportInboxClearHandler creates a SupportInboxClearHandler.
func NewSupportInboxClearHandler(clears supportInboxClearRecorder) *SupportInboxClearHandler {
	return &SupportInboxClearHandler{clears: clears}
}

// RecordClear handles POST /api/support/inbox-clears.
func (h *SupportInboxClearHandler) RecordClear(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())

	var req model.RecordSupportInboxClearRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	stats, err := h.clears.Record(r.Context(), workspaceID, userID, req)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, stats)
	case errors.Is(err, service.ErrSupportInboxClearInvalid):
		// Validation errors name only the request field at fault.
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrSupportInboxClearNotFound):
		writeError(w, http.StatusNotFound, "conversation not found")
	case errors.Is(err, service.ErrSupportInboxClearStillWaiting):
		writeError(w, http.StatusConflict, "conversation is not resolved")
	default:
		slog.ErrorContext(r.Context(), "record support inbox clear failed",
			"error", err, "workspace_id", workspaceID, "user_id", userID, "conversation_id", req.ConversationID)
		writeError(w, http.StatusInternalServerError, "could not record the inbox clear")
	}
}
