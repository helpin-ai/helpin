package handler

import (
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PushDeviceHandler handles mobile push notification device registration endpoints.
type PushDeviceHandler struct {
	service *service.PushDeviceService
}

// NewPushDeviceHandler creates a new handler.
func NewPushDeviceHandler(svc *service.PushDeviceService) *PushDeviceHandler {
	return &PushDeviceHandler{service: svc}
}

// Register registers (or re-registers) a push device for the current user.
func (h *PushDeviceHandler) Register(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.RegisterPushDeviceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	device, err := h.service.Register(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, device)
}

// Unregister removes a push device registration for the current user.
func (h *PushDeviceHandler) Unregister(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.UnregisterPushDeviceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.Unregister(r.Context(), userID, req.Token); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
