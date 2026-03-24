package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AdminWebhookEventHandler serves admin endpoints for webhook event inspection.
type AdminWebhookEventHandler struct {
	webhookRepo *repository.SupportEmailWebhookEventRepository
}

// NewAdminWebhookEventHandler creates a new AdminWebhookEventHandler.
func NewAdminWebhookEventHandler(webhookRepo *repository.SupportEmailWebhookEventRepository) *AdminWebhookEventHandler {
	return &AdminWebhookEventHandler{webhookRepo: webhookRepo}
}

// List handles GET /api/admin/webhook-events.
func (h *AdminWebhookEventHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	eventType := r.URL.Query().Get("event_type")
	provider := r.URL.Query().Get("provider")

	result, err := h.webhookRepo.ListPaginated(r.Context(), page, perPage, eventType, provider)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list webhook events")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetByID handles GET /api/admin/webhook-events/{id}.
func (h *AdminWebhookEventHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}

	event, err := h.webhookRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get webhook event")
		return
	}
	if event == nil {
		writeError(w, http.StatusNotFound, "webhook event not found")
		return
	}

	writeJSON(w, http.StatusOK, event)
}
