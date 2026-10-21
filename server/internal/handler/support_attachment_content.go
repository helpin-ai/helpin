package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// Content streams a private support attachment after checking conversation access.
func (h *SupportAttachmentHandler) Content(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	id := chi.URLParam(r, "attachmentId")
	content, err := h.inboxService.OpenAttachmentContent(ctx, getWorkspaceID(r), chi.URLParam(r, "convId"), id)
	if errors.Is(err, service.ErrSupportAttachmentNotFound) {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if err != nil {
		slog.ErrorContext(ctx, "open support attachment", "attachment_id", id, "workspace_id", getWorkspaceID(r), "error", err)
		writeError(w, http.StatusBadGateway, "could not load attachment")
		return
	}
	defer func() {
		if err := content.Body.Close(); err != nil {
			slog.WarnContext(ctx, "close support attachment", "attachment_id", id, "error", err)
		}
	}()
	writeSupportAttachmentContent(w, r, content)
}

func writeSupportAttachmentContent(w http.ResponseWriter, r *http.Request, content *service.SupportAttachmentContent) {
	contentType := content.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": content.FileName}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	if content.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(content.Size, 10))
	}
	if _, err := io.Copy(w, content.Body); err != nil {
		slog.WarnContext(r.Context(), "stream support attachment", "attachment_id", chi.URLParam(r, "attachmentId"), "error", err)
	}
}
