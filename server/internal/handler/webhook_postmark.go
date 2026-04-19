package handler

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PostmarkInboundHandler handles Postmark inbound webhooks.
type PostmarkInboundHandler struct {
	emailFallbackService *service.EmailFallbackService
	webhookSecrets       []string
}

// NewPostmarkInboundHandler creates a new PostmarkInboundHandler.
func NewPostmarkInboundHandler(emailFallbackService *service.EmailFallbackService, webhookSecrets ...string) *PostmarkInboundHandler {
	normalizedSecrets := make([]string, 0, len(webhookSecrets))
	for _, secret := range webhookSecrets {
		if trimmed := strings.TrimSpace(secret); trimmed != "" {
			normalizedSecrets = append(normalizedSecrets, trimmed)
		}
	}
	return &PostmarkInboundHandler{
		emailFallbackService: emailFallbackService,
		webhookSecrets:       normalizedSecrets,
	}
}

// PostmarkInbound handles POST /api/webhooks/postmark/inbound.
func (h *PostmarkInboundHandler) PostmarkInbound(w http.ResponseWriter, r *http.Request) {
	if h == nil || !h.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	defer r.Body.Close()

	var payload model.PostmarkInboundPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		// Postmark retries non-200 responses aggressively; malformed payloads are best-effort ignored.
		w.WriteHeader(http.StatusOK)
		return
	}
	slog.InfoContext(r.Context(), "postmark inbound webhook received",
		"message_id", strings.TrimSpace(payload.MessageID),
		"message_stream", strings.TrimSpace(payload.MessageStream),
		"mailbox_hash", strings.TrimSpace(payload.MailboxHash),
		"original_recipient", strings.TrimSpace(payload.OriginalRecipient),
	)

	if h.emailFallbackService != nil {
		if err := h.emailFallbackService.ProcessInboundEmail(r.Context(), payload, string(body)); err != nil {
			slog.Warn("postmark inbound processing failed", "error", err, "mailbox_hash", payload.MailboxHash)
		} else {
			slog.InfoContext(r.Context(), "postmark inbound webhook processed",
				"message_id", strings.TrimSpace(payload.MessageID),
				"message_stream", strings.TrimSpace(payload.MessageStream),
			)
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

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	defer r.Body.Close()

	var payload model.PostmarkOpenPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	slog.InfoContext(r.Context(), "postmark open webhook received",
		"message_id", strings.TrimSpace(payload.MessageID),
		"message_stream", strings.TrimSpace(payload.MessageStream),
		"first_open", payload.FirstOpen,
	)

	if h.emailFallbackService != nil {
		if err := h.emailFallbackService.ProcessOpenEvent(r.Context(), payload, string(body)); err != nil {
			slog.Warn("postmark open processing failed", "error", err, "message_id", payload.MessageID)
		} else {
			slog.InfoContext(r.Context(), "postmark open webhook processed",
				"message_id", strings.TrimSpace(payload.MessageID),
				"message_stream", strings.TrimSpace(payload.MessageStream),
				"first_open", payload.FirstOpen,
			)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *PostmarkInboundHandler) authorized(r *http.Request) bool {
	if h == nil || len(h.webhookSecrets) == 0 {
		return false
	}
	_, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	for _, secret := range h.webhookSecrets {
		if subtle.ConstantTimeCompare([]byte(password), []byte(secret)) == 1 {
			return true
		}
	}
	return false
}
