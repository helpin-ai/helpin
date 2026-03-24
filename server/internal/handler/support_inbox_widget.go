package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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

func widgetKeyFromRequest(r *http.Request) string {
	return r.URL.Query().Get("widget_key")
}

// GetWidgetTokens handles GET /api/internal/widget-tokens.
// Returns all active widget installations as tokens for the events-pipeline.
// Protected by INTERNAL_API_SECRET bearer token.
func (h *SupportInboxWidgetHandler) GetWidgetTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.supportService.ListWidgetTokens(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch widget tokens")
		return
	}
	writeJSON(w, http.StatusOK, model.WidgetTokensResponse{Tokens: tokens})
}

// GetConfig handles GET /api/widget/support/config?widget_key=...
func (h *SupportInboxWidgetHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	widgetKey := widgetKeyFromRequest(r)
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

// GetConfigByID handles GET /api/settings/website/{id}
func (h *SupportInboxWidgetHandler) GetConfigByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	config, err := h.supportService.GetPublicWidgetConfigByID(r.Context(), id)
	if err != nil || config == nil {
		writeError(w, http.StatusNotFound, "widget not found")
		return
	}

	writeJSON(w, http.StatusOK, config)
}

// CreateSession handles POST /api/widget/support/session (legacy HTTP endpoint).
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

	// Legacy HTTP path — anonymous_id defaults to empty, will be set by WS flow
	session, err := h.supportService.CreateWidgetSession(r.Context(), req.WidgetKey, "", req.CustomerName, req.CustomerEmail, nil, nil, nil, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"session_token": session.SessionToken,
		"expires_at":    session.ExpiresAt,
	})
}

// RevokeSession handles POST /widget/session/revoke (HTTP fallback for shutdown).
func (h *SupportInboxWidgetHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	var req model.WidgetSessionRevokeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SessionToken == "" {
		writeError(w, http.StatusBadRequest, "session_token is required")
		return
	}

	if err := h.supportService.RevokeWidgetSession(r.Context(), req.SessionToken); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

	msg, err := h.supportService.WidgetCreateMessage(r.Context(), req.SessionToken, req.Content, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

// TypingIndicator handles POST /widget/typing as an HTTP fallback when widget WS is unavailable.
func (h *SupportInboxWidgetHandler) TypingIndicator(w http.ResponseWriter, r *http.Request) {
	var req model.WidgetTypingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SessionToken == "" {
		writeError(w, http.StatusBadRequest, "session_token is required")
		return
	}

	if err := h.supportService.PublishWidgetTypingIndicator(r.Context(), req.SessionToken, req.IsTyping); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
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

// Identify handles POST /api/widget/identify (headless SDK path).
// When the widget is not open, the SDK sends identity data via HTTP instead of WebSocket.
func (h *SupportInboxWidgetHandler) Identify(w http.ResponseWriter, r *http.Request) {
	var req model.WidgetIdentifyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.APIKey == "" {
		writeError(w, http.StatusBadRequest, "api_key is required")
		return
	}
	if req.AnonymousID == "" {
		writeError(w, http.StatusBadRequest, "anonymous_id is required")
		return
	}
	source := req.Source
	if source == "" {
		source = "sdk_identify"
	}

	// Allow empty email — visitor skipped identification.
	if req.Email != "" {
		if err := h.supportService.IdentifyByAnonymousID(r.Context(), req.APIKey, req.AnonymousID, req.Email, req.Name, source); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// GetHelpCollections handles GET /api/widget/support/help/spaces/{spaceSlug}/collections?widget_key=...
func (h *SupportInboxWidgetHandler) GetHelpCollections(w http.ResponseWriter, r *http.Request) {
	widgetKey := widgetKeyFromRequest(r)
	if widgetKey == "" {
		writeError(w, http.StatusBadRequest, "widget_key is required")
		return
	}

	spaceSlug := chi.URLParam(r, "spaceSlug")
	if spaceSlug == "" {
		writeError(w, http.StatusBadRequest, "spaceSlug is required")
		return
	}

	collections, err := h.supportService.ListWidgetHelpCollections(r.Context(), widgetKey, spaceSlug)
	if err != nil {
		if err.Error() == "widget not found" || err.Error() == "space not found" {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if collections == nil {
		collections = []model.WidgetHelpCollection{}
	}
	writeJSON(w, http.StatusOK, collections)
}

// GetHelpArticles handles GET /api/widget/support/help/collections/{collectionSlug}/articles?widget_key=...
func (h *SupportInboxWidgetHandler) GetHelpArticles(w http.ResponseWriter, r *http.Request) {
	widgetKey := widgetKeyFromRequest(r)
	if widgetKey == "" {
		writeError(w, http.StatusBadRequest, "widget_key is required")
		return
	}

	collectionSlug := chi.URLParam(r, "collectionSlug")
	if collectionSlug == "" {
		writeError(w, http.StatusBadRequest, "collectionSlug is required")
		return
	}

	articles, err := h.supportService.ListWidgetHelpArticles(r.Context(), widgetKey, collectionSlug)
	if err != nil {
		if err.Error() == "widget not found" || err.Error() == "collection not found" {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if articles == nil {
		articles = []model.WidgetHelpArticleSummary{}
	}
	writeJSON(w, http.StatusOK, articles)
}

// GetHelpArticle handles GET /api/widget/support/help/articles/{articleSlug}?widget_key=...
func (h *SupportInboxWidgetHandler) GetHelpArticle(w http.ResponseWriter, r *http.Request) {
	widgetKey := widgetKeyFromRequest(r)
	if widgetKey == "" {
		writeError(w, http.StatusBadRequest, "widget_key is required")
		return
	}

	articleSlug := chi.URLParam(r, "articleSlug")
	if articleSlug == "" {
		writeError(w, http.StatusBadRequest, "articleSlug is required")
		return
	}

	article, err := h.supportService.GetWidgetHelpArticle(r.Context(), widgetKey, articleSlug)
	if err != nil {
		if err.Error() == "widget not found" || err.Error() == "article not found" {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, article)
}
