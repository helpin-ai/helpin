package handler

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// ListInbound lists the operator recovery queue without full payloads.
func (h *AdminEmailQueueHandler) ListInbound(w http.ResponseWriter, r *http.Request) {
	if !h.inboundAvailable(w) {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	status := r.URL.Query().Get("status")
	if status != "" && status != "attention" && status != "recovered" && status != "processing" && status != "all" {
		writeError(w, 400, "Invalid queue filter.")
		return
	}
	result, err := h.emailFallbackService.ListAdminInboundEmails(r.Context(), status, r.URL.Query().Get("q"), page)
	if err != nil {
		writeInboundAdminError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetInbound returns one receipt for review.
func (h *AdminEmailQueueHandler) GetInbound(w http.ResponseWriter, r *http.Request) {
	if !h.inboundAvailable(w) {
		return
	}
	result, err := h.emailFallbackService.GetAdminInboundEmail(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeInboundAdminError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// InboundConversations offers explicit, validated destinations for the retained sender.
func (h *AdminEmailQueueHandler) InboundConversations(w http.ResponseWriter, r *http.Request) {
	if !h.inboundAvailable(w) {
		return
	}
	result, err := h.emailFallbackService.FindAdminInboundConversations(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("q"))
	if err != nil {
		writeInboundAdminError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// RecoverInbound queues a retry; acceptance is not a delivery confirmation.
func (h *AdminEmailQueueHandler) RecoverInbound(w http.ResponseWriter, r *http.Request) {
	if !h.inboundAvailable(w) {
		return
	}
	var req model.RecoverInboundEmailRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "Invalid recovery request.")
		return
	}
	err := h.emailFallbackService.RecoverAdminInboundEmail(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()), strings.TrimSpace(req.ConversationID))
	if err != nil {
		writeInboundAdminError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
}

// DownloadInboundAttachment serves attachments as downloads through the MFA-protected admin route.
func (h *AdminEmailQueueHandler) DownloadInboundAttachment(w http.ResponseWriter, r *http.Request) {
	if !h.inboundAvailable(w) {
		return
	}
	data, name, err := h.emailFallbackService.DownloadAdminInboundAttachment(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "attachmentID"))
	if err != nil {
		writeInboundAdminError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	if _, err := w.Write(data); err != nil {
		slog.ErrorContext(r.Context(), "admin attachment download failed", "job_id", chi.URLParam(r, "id"), "error", err)
	}
}

func (h *AdminEmailQueueHandler) inboundAvailable(w http.ResponseWriter) bool {
	w.Header().Set("Cache-Control", "no-store")
	if h.emailFallbackService == nil {
		writeError(w, http.StatusServiceUnavailable, "Incoming email recovery is unavailable.")
		return false
	}
	return true
}

func writeInboundAdminError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrInboundRecoveryNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrInboundRecoveryConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrInboundRecoveryDestination):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.ErrorContext(r.Context(), "admin inbound email operation failed", "job_id", chi.URLParam(r, "id"), "error", err)
		writeError(w, http.StatusInternalServerError, "Could not process this email. Try again.")
	}
}
