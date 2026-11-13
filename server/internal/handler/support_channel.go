package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// _maxChannelWebhookBody caps provider webhook bodies; Telegram updates are a
// few kilobytes.
const _maxChannelWebhookBody = 1 << 20

// SupportChannelHandler manages connected channels and receives their
// webhooks.
type SupportChannelHandler struct {
	channels *service.SupportChannelService
}

// NewSupportChannelHandler creates the handler.
func NewSupportChannelHandler(channels *service.SupportChannelService) *SupportChannelHandler {
	return &SupportChannelHandler{channels: channels}
}

// List handles GET /api/support/channels.
func (h *SupportChannelHandler) List(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.channels.List(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, accounts)
}

// Create handles POST /api/support/channels.
func (h *SupportChannelHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.ConnectSupportChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	account, err := h.channels.Connect(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), req)
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

// Update handles PATCH /api/support/channels/{channelId}.
func (h *SupportChannelHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := channelID(w, r)
	if !ok {
		return
	}
	var req model.UpdateSupportChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	account, err := h.channels.Update(r.Context(), getWorkspaceID(r), id, req)
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

// Reconnect handles POST /api/support/channels/{channelId}/reconnect.
func (h *SupportChannelHandler) Reconnect(w http.ResponseWriter, r *http.Request) {
	id, ok := channelID(w, r)
	if !ok {
		return
	}
	var req model.ReconnectSupportChannelRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	account, err := h.channels.Reconnect(r.Context(), getWorkspaceID(r), id, req)
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

// Disconnect handles DELETE /api/support/channels/{channelId}.
func (h *SupportChannelHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	id, ok := channelID(w, r)
	if !ok {
		return
	}
	if err := h.channels.Disconnect(r.Context(), getWorkspaceID(r), id); err != nil {
		writeChannelError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Webhook handles POST /api/webhooks/channels/{provider}/{accountId}. It
// answers 200 only after the events are stored, so the provider retries
// anything that was not.
func (h *SupportChannelHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, _maxChannelWebhookBody))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
		return
	}
	err = h.channels.AcceptWebhook(r.Context(), chi.URLParam(r, "provider"), chi.URLParam(r, "accountId"), r.Header, body)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case errors.Is(err, service.ErrChannelWebhookUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, service.ErrChannelWebhookInvalid):
		writeError(w, http.StatusBadRequest, "invalid payload")
	default:
		slog.ErrorContext(r.Context(), "accept channel webhook", "error", err, "provider", chi.URLParam(r, "provider"))
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
	}
}

func channelID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "channelId")
	if !isUUID(id) {
		writeError(w, http.StatusNotFound, "channel not found")
		return "", false
	}
	return id, true
}

func writeChannelError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrChannelInvalid):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrChannelInvalid.Error()+": "))
	case errors.Is(err, service.ErrChannelTokenRejected):
		writeError(w, http.StatusBadRequest, "Telegram rejected this bot token. Copy it again from @BotFather.")
	case errors.Is(err, service.ErrChannelAlreadyConnected):
		writeError(w, http.StatusConflict, "This bot is already connected to a workspace. Disconnect it there first.")
	case errors.Is(err, service.ErrChannelEncryptionUnavailable):
		writeError(w, http.StatusServiceUnavailable, "Channels need CHANNEL_ENCRYPTION_KEY or CRM_ENCRYPTION_KEY on the server.")
	case errors.Is(err, service.ErrChannelPublicURLUnavailable):
		writeError(w, http.StatusServiceUnavailable, "Channels need PUBLIC_API_BASE_URL set to the server's public HTTPS address.")
	case errors.Is(err, service.ErrChannelProviderUnavailable):
		slog.WarnContext(r.Context(), "channel provider unavailable", "error", err)
		writeError(w, http.StatusBadGateway, "Telegram could not be reached. Try again in a moment.")
	case errors.Is(err, repository.ErrSupportChannelAccountNotFound):
		writeError(w, http.StatusNotFound, "channel not found")
	default:
		slog.ErrorContext(r.Context(), "support channel request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to update channels")
	}
}
