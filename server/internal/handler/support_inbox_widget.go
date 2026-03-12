package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportInboxWidgetHandler handles public widget HTTP endpoints (no JWT required).
type SupportInboxWidgetHandler struct {
	supportService *service.SupportInboxService
}

// NewSupportInboxWidgetHandler creates a new SupportInboxWidgetHandler.
func NewSupportInboxWidgetHandler(supportService *service.SupportInboxService) *SupportInboxWidgetHandler {
	return &SupportInboxWidgetHandler{supportService: supportService}
}

// GetConfig handles GET /api/widget/support/config?widget_key=...
func (h *SupportInboxWidgetHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	widgetKey := r.URL.Query().Get("widget_key")
	if widgetKey == "" {
		writeError(w, http.StatusBadRequest, "widget_key is required")
		return
	}

	config, err := h.supportService.GetPublicWidgetConfig(r.Context(), widgetKey)
	if err != nil || config == nil {
		writeError(w, http.StatusNotFound, "widget not found")
		return
	}

	writeJSON(w, http.StatusOK, config)
}

// CreateSession handles POST /api/widget/support/session.
func (h *SupportInboxWidgetHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req model.WidgetSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WidgetKey == "" {
		writeError(w, http.StatusBadRequest, "widget_key is required")
		return
	}

	session, err := h.supportService.CreateWidgetSession(r.Context(), req.WidgetKey, req.CustomerName, req.CustomerEmail)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"session_token": session.SessionToken,
		"expires_at":    session.ExpiresAt,
	})
}

// SendMessage handles POST /api/widget/support/messages.
func (h *SupportInboxWidgetHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	var req model.WidgetMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SessionToken == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, "session_token and content are required")
		return
	}

	msg, err := h.supportService.WidgetCreateMessage(r.Context(), req.SessionToken, req.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

// GetMessages handles GET /api/widget/support/messages?session_token=...
func (h *SupportInboxWidgetHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	sessionToken := r.URL.Query().Get("session_token")
	if sessionToken == "" {
		writeError(w, http.StatusBadRequest, "session_token is required")
		return
	}

	session, err := h.supportService.GetWidgetSession(r.Context(), sessionToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	conversationID := session.ConversationID
	if conversationID == nil {
		writeJSON(w, http.StatusOK, []model.SupportMessage{})
		return
	}

	messages, err := h.supportService.ListConversationMessages(r.Context(), session.WorkspaceID, *conversationID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []model.SupportMessage{}
	}
	writeJSON(w, http.StatusOK, messages)
}
