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

func (h *DocsHandler) SetAIRewriteService(svc *service.DocsAIRewriteService) { h.aiRewriteSvc = svc }

// RewriteSelection previews a rewrite of supplied text; it never writes a document.
func (h *DocsHandler) RewriteSelection(w http.ResponseWriter, r *http.Request) {
	if h.aiRewriteSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "Document AI is unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	var req model.DocsAIRewriteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid rewrite request")
		return
	}
	workspaceID := middleware.GetWorkspaceID(r.Context())
	result, err := h.aiRewriteSvc.Rewrite(r.Context(), workspaceID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDocsRewriteInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, context.DeadlineExceeded):
			writeError(w, http.StatusGatewayTimeout, "AI took too long. Your text is unchanged. Try again.")
		default:
			slog.ErrorContext(r.Context(), "document selection rewrite failed", "workspace_id", workspaceID, "error", err)
			writeBillingAwareError(w, http.StatusInternalServerError, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}
