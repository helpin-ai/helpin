package handler

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PostmarkInboundHandler handles Postmark inbound webhooks.
type PostmarkInboundHandler struct {
	emailFallbackService *service.EmailFallbackService
	webhookSecret        string
}

// NewPostmarkInboundHandler creates a new PostmarkInboundHandler.
func NewPostmarkInboundHandler(emailFallbackService *service.EmailFallbackService, webhookSecret string) *PostmarkInboundHandler {
	return &PostmarkInboundHandler{
		emailFallbackService: emailFallbackService,
		webhookSecret:        strings.TrimSpace(webhookSecret),
	}
}

// PostmarkInbound handles POST /api/webhooks/postmark/inbound.
func (h *PostmarkInboundHandler) PostmarkInbound(w http.ResponseWriter, r *http.Request) {
	if h == nil || !h.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var payload model.PostmarkInboundPayload
	if err := decodeJSON(r, &payload); err != nil {
		// Postmark retries non-200 responses aggressively; malformed payloads are best-effort ignored.
		w.WriteHeader(http.StatusOK)
		return
	}

	if h.emailFallbackService != nil {
		if err := h.emailFallbackService.ProcessInboundEmail(r.Context(), payload); err != nil {
			slog.Warn("postmark inbound processing failed", "error", err, "mailbox_hash", payload.MailboxHash)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// PostmarkOpen handles POST /api/webhooks/postmark/open.
func (h *PostmarkInboundHandler) PostmarkOpen(w http.ResponseWriter, r *http.Request) {
	if h == nil || !h.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var payload model.PostmarkOpenPayload
	if err := decodeJSON(r, &payload); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	if h.emailFallbackService != nil {
		if err := h.emailFallbackService.ProcessOpenEvent(r.Context(), payload); err != nil {
			slog.Warn("postmark open processing failed", "error", err, "message_id", payload.MessageID)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *PostmarkInboundHandler) authorized(r *http.Request) bool {
	if h == nil || h.webhookSecret == "" {
		return false
	}
	_, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(h.webhookSecret)) == 1
}
