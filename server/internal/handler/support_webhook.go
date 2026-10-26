package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportWebhookHandler manages workspace webhook endpoints.
type SupportWebhookHandler struct {
	webhooks *service.WebhookService
}

func NewSupportWebhookHandler(webhooks *service.WebhookService) *SupportWebhookHandler {
	return &SupportWebhookHandler{webhooks: webhooks}
}

// webhookIDs reads the path IDs and answers 404 for malformed ones.
func webhookIDs(w http.ResponseWriter, r *http.Request, names ...string) ([]string, bool) {
	ids := make([]string, 0, len(names))
	for _, name := range names {
		id := chi.URLParam(r, name)
		if !isUUID(id) {
			writeError(w, http.StatusNotFound, "webhook not found")
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func writeWebhookError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrWebhookInvalid):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrWebhookInvalid.Error()+": "))
	case errors.Is(err, service.ErrWebhookSecretsUnavailable):
		writeError(w, http.StatusServiceUnavailable, "Webhooks need WEBHOOK_ENCRYPTION_KEY or CRM_ENCRYPTION_KEY on the server.")
	case errors.Is(err, repository.ErrWebhookEndpointNotFound), errors.Is(err, repository.ErrWebhookDeliveryNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "unable to update webhooks")
	}
}

// EventTypes handles GET /api/support/webhooks/event-types.
func (h *SupportWebhookHandler) EventTypes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, model.WebhookEventTypes)
}

// List handles GET /api/support/webhooks.
func (h *SupportWebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	endpoints, err := h.webhooks.ListEndpoints(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, endpoints)
}

// Create handles POST /api/support/webhooks. The secret is returned once.
func (h *SupportWebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.WebhookEndpointRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	created, err := h.webhooks.CreateEndpoint(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), req)
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// Update handles PUT /api/support/webhooks/{webhookId}.
func (h *SupportWebhookHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.WebhookEndpointRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ids, ok := webhookIDs(w, r, "webhookId")
	if !ok {
		return
	}
	endpoint, err := h.webhooks.UpdateEndpoint(r.Context(), getWorkspaceID(r), ids[0], req)
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, endpoint)
}

// Delete handles DELETE /api/support/webhooks/{webhookId}.
func (h *SupportWebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ids, ok := webhookIDs(w, r, "webhookId")
	if !ok {
		return
	}
	if err := h.webhooks.DeleteEndpoint(r.Context(), getWorkspaceID(r), ids[0]); err != nil {
		writeWebhookError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RotateSecret handles POST /api/support/webhooks/{webhookId}/rotate-secret.
func (h *SupportWebhookHandler) RotateSecret(w http.ResponseWriter, r *http.Request) {
	ids, ok := webhookIDs(w, r, "webhookId")
	if !ok {
		return
	}
	rotated, err := h.webhooks.RotateSecret(r.Context(), getWorkspaceID(r), ids[0])
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rotated)
}

// Test handles POST /api/support/webhooks/{webhookId}/test.
func (h *SupportWebhookHandler) Test(w http.ResponseWriter, r *http.Request) {
	ids, ok := webhookIDs(w, r, "webhookId")
	if !ok {
		return
	}
	delivery, err := h.webhooks.SendTest(r.Context(), getWorkspaceID(r), ids[0])
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, delivery)
}

// Deliveries handles GET /api/support/webhooks/{webhookId}/deliveries.
func (h *SupportWebhookHandler) Deliveries(w http.ResponseWriter, r *http.Request) {
	ids, ok := webhookIDs(w, r, "webhookId")
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	deliveries, err := h.webhooks.ListDeliveries(r.Context(), getWorkspaceID(r), ids[0], limit)
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, deliveries)
}

// Redeliver handles POST /api/support/webhooks/{webhookId}/deliveries/{deliveryId}/redeliver.
func (h *SupportWebhookHandler) Redeliver(w http.ResponseWriter, r *http.Request) {
	ids, ok := webhookIDs(w, r, "webhookId", "deliveryId")
	if !ok {
		return
	}
	delivery, err := h.webhooks.Redeliver(r.Context(), getWorkspaceID(r), ids[0], ids[1])
	if err != nil {
		writeWebhookError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, delivery)
}
