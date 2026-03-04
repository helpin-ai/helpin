package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// PMAttachmentHandler handles PM attachment HTTP endpoints.
type PMAttachmentHandler struct {
	attachmentService *service.PMAttachmentService
}

// NewPMAttachmentHandler creates a new PMAttachmentHandler.
func NewPMAttachmentHandler(attachmentService *service.PMAttachmentService) *PMAttachmentHandler {
	return &PMAttachmentHandler{attachmentService: attachmentService}
}

// Create handles POST /api/pm/attachments — initiates an upload and returns a presigned URL.
func (h *PMAttachmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())

	var req model.CreateAttachmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.attachmentService.Create(r.Context(), req, workspaceID, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// ConfirmUpload handles PATCH /api/pm/attachments/{id}/confirm.
func (h *PMAttachmentHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.attachmentService.ConfirmUpload(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "upload confirmed"})
}

// List handles GET /api/pm/attachments?entity_type=...&entity_id=...
func (h *PMAttachmentHandler) List(w http.ResponseWriter, r *http.Request) {
	entityType := r.URL.Query().Get("entity_type")
	entityID := r.URL.Query().Get("entity_id")
	if entityType == "" || entityID == "" {
		writeError(w, http.StatusBadRequest, "entity_type and entity_id are required")
		return
	}

	attachments, err := h.attachmentService.List(r.Context(), entityType, entityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if attachments == nil {
		attachments = []model.AttachmentResponse{}
	}
	writeJSON(w, http.StatusOK, attachments)
}

// Delete handles DELETE /api/pm/attachments/{id}.
func (h *PMAttachmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	if err := h.attachmentService.Delete(r.Context(), id, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "attachment deleted"})
}
