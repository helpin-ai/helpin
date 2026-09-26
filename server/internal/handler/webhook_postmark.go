package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type postmarkEmailProcessor interface {
	AcceptInboundEmail(context.Context, model.PostmarkInboundPayload, string) error
	ProcessOpenEvent(context.Context, model.PostmarkOpenPayload, string) error
	ProcessDeliveryEvent(context.Context, model.PostmarkDeliveryPayload, string) error
	ProcessBounceEvent(context.Context, model.PostmarkBouncePayload, string) error
	ProcessSpamComplaintEvent(context.Context, model.PostmarkSpamComplaintPayload, string) error
}

// PostmarkInboundHandler handles Postmark inbound webhooks.
type PostmarkInboundHandler struct {
	emailFallbackService postmarkEmailProcessor
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
	h := &PostmarkInboundHandler{webhookSecrets: normalizedSecrets}
	if emailFallbackService != nil {
		h.emailFallbackService = emailFallbackService
	}
	return h
}

// PostmarkInbound handles POST /api/webhooks/postmark/inbound.
func (h *PostmarkInboundHandler) PostmarkInbound(w http.ResponseWriter, r *http.Request) {
	if h == nil || !h.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid inbound email payload")
		return
	}
	defer r.Body.Close()

	var payload model.PostmarkInboundPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		// Never acknowledge a payload that could not be durably accepted.
		writeError(w, http.StatusBadRequest, "invalid inbound email payload")
		return
	}
	slog.InfoContext(r.Context(), "postmark inbound webhook received",
		"message_id", strings.TrimSpace(payload.MessageID),
		"message_stream", strings.TrimSpace(payload.MessageStream),
		"mailbox_hash", strings.TrimSpace(payload.MailboxHash),
		"original_recipient", strings.TrimSpace(payload.OriginalRecipient),
	)

	if h.emailFallbackService == nil {
		writeError(w, http.StatusServiceUnavailable, "inbound email processing temporarily unavailable")
		return
	}
	if err := h.emailFallbackService.AcceptInboundEmail(r.Context(), payload, string(body)); err != nil {
		slog.ErrorContext(r.Context(), "inbound receipt persistence failed", "message_id", payload.MessageID, "error", err)
		writeError(w, http.StatusServiceUnavailable, "inbound email processing temporarily unavailable")
		return
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

// PostmarkDelivery handles POST /api/webhooks/postmark/delivery.
func (h *PostmarkInboundHandler) PostmarkDelivery(w http.ResponseWriter, r *http.Request) {
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

	var payload model.PostmarkDeliveryPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	slog.InfoContext(r.Context(), "postmark delivery webhook received",
		"message_id", strings.TrimSpace(payload.MessageID),
		"message_stream", strings.TrimSpace(payload.MessageStream),
	)

	if h.emailFallbackService != nil {
		if err := h.emailFallbackService.ProcessDeliveryEvent(r.Context(), payload, string(body)); err != nil {
			slog.Warn("postmark delivery processing failed", "error", err, "message_id", payload.MessageID)
		} else {
			slog.InfoContext(r.Context(), "postmark delivery webhook processed",
				"message_id", strings.TrimSpace(payload.MessageID),
				"message_stream", strings.TrimSpace(payload.MessageStream),
			)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// PostmarkBounce handles POST /api/webhooks/postmark/bounce.
func (h *PostmarkInboundHandler) PostmarkBounce(w http.ResponseWriter, r *http.Request) {
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

	var payload model.PostmarkBouncePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	slog.InfoContext(r.Context(), "postmark bounce webhook received",
		"message_id", strings.TrimSpace(payload.MessageID),
		"message_stream", strings.TrimSpace(payload.MessageStream),
		"type", strings.TrimSpace(payload.Type),
	)

	if h.emailFallbackService != nil {
		if err := h.emailFallbackService.ProcessBounceEvent(r.Context(), payload, string(body)); err != nil {
			slog.Warn("postmark bounce processing failed", "error", err, "message_id", payload.MessageID)
		} else {
			slog.InfoContext(r.Context(), "postmark bounce webhook processed",
				"message_id", strings.TrimSpace(payload.MessageID),
				"message_stream", strings.TrimSpace(payload.MessageStream),
				"type", strings.TrimSpace(payload.Type),
			)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// PostmarkSpamComplaint handles POST /api/webhooks/postmark/spam-complaint.
func (h *PostmarkInboundHandler) PostmarkSpamComplaint(w http.ResponseWriter, r *http.Request) {
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

	var payload model.PostmarkSpamComplaintPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	slog.InfoContext(r.Context(), "postmark spam complaint webhook received",
		"message_id", strings.TrimSpace(payload.MessageID),
		"message_stream", strings.TrimSpace(payload.MessageStream),
		"type", strings.TrimSpace(payload.Type),
	)

	if h.emailFallbackService != nil {
		if err := h.emailFallbackService.ProcessSpamComplaintEvent(r.Context(), payload, string(body)); err != nil {
			slog.Warn("postmark spam complaint processing failed", "error", err, "message_id", payload.MessageID)
		} else {
			slog.InfoContext(r.Context(), "postmark spam complaint webhook processed",
				"message_id", strings.TrimSpace(payload.MessageID),
				"message_stream", strings.TrimSpace(payload.MessageStream),
				"type", strings.TrimSpace(payload.Type),
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
