package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// AdminEmailQueueHandler serves admin endpoints for the email fallback queue.
type AdminEmailQueueHandler struct {
	emailFallbackService *service.EmailFallbackService
}

// NewAdminEmailQueueHandler creates a new AdminEmailQueueHandler.
func NewAdminEmailQueueHandler(emailFallbackService *service.EmailFallbackService) *AdminEmailQueueHandler {
	return &AdminEmailQueueHandler{emailFallbackService: emailFallbackService}
}

// List handles GET /api/admin/email-queue.
func (h *AdminEmailQueueHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.emailFallbackService == nil {
		writeJSON(w, http.StatusOK, map[string]any{"entries": []any{}, "total": 0})
		return
	}

	result, err := h.emailFallbackService.ListQueue(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list email queue")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
