package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Connections reads bounded publication history, not private execution configuration.
func (h *CRMPlaybookHandler) Connections(w http.ResponseWriter, r *http.Request) {
	before, limit, ok := playbookCursor(w, r)
	if !ok {
		return
	}
	result, err := h.service.Connections(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), before, limit)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ReviewConnection inspects saved settings without writing or enabling anything.
func (h *CRMPlaybookHandler) ReviewConnection(w http.ResponseWriter, r *http.Request) {
	var req model.CRMPlaybookConnectionSelection
	if !decodePlaybookRequest(w, r, &req) {
		return
	}
	result, err := h.service.ReviewConnection(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PublishConnection saves a disabled, immutable setup; publication is not activation.
func (h *CRMPlaybookHandler) PublishConnection(w http.ResponseWriter, r *http.Request) {
	var req model.PublishCRMPlaybookConnectionRequest
	if !decodePlaybookRequest(w, r, &req) {
		return
	}
	result, err := h.service.PublishConnection(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

// Connection reads an exact historical receipt without exposing private instructions.
func (h *CRMPlaybookHandler) Connection(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Connection(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), chi.URLParam(r, "connection_id"))
	if err != nil {
		writePlaybookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
