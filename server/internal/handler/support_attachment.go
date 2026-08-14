package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/requestmeta"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportAttachmentHandler handles support attachment HTTP endpoints.
type SupportAttachmentHandler struct {
	attachmentService *service.SupportAttachmentService
	inboxService      *service.SupportInboxService
}

// NewSupportAttachmentHandler creates a new SupportAttachmentHandler.
func NewSupportAttachmentHandler(
	attachmentService *service.SupportAttachmentService,
	inboxService *service.SupportInboxService,
) *SupportAttachmentHandler {
	return &SupportAttachmentHandler{
		attachmentService: attachmentService,
		inboxService:      inboxService,
	}
}

// Create handles POST /support/inbox/conversations/{convId}/attachments (authenticated).
func (h *SupportAttachmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())
	convID := chi.URLParam(r, "convId")

	var req model.CreateSupportAttachmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.attachmentService.Create(r.Context(), req, workspaceID, convID, "user", &userID, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// ConfirmUpload handles PATCH /support/inbox/attachments/{attachmentId}/confirm (authenticated).
func (h *SupportAttachmentHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "attachmentId")
	userID := middleware.GetUserID(r.Context())
	if err := h.attachmentService.ConfirmUpload(r.Context(), id, "user", &userID, nil); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "upload confirmed"})
}

// Delete handles DELETE /support/inbox/attachments/{attachmentId} (authenticated).
func (h *SupportAttachmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "attachmentId")
	userID := middleware.GetUserID(r.Context())
	if err := h.attachmentService.Delete(r.Context(), id, "user", &userID, nil); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "attachment deleted"})
}

// WidgetCreate handles POST /widget/support/attachments (session token auth).
func (h *SupportAttachmentHandler) WidgetCreate(w http.ResponseWriter, r *http.Request) {
	sessionToken := r.Header.Get("X-Session-Token")
	if sessionToken == "" {
		writeError(w, http.StatusUnauthorized, "session token required")
		return
	}

	ctx := r.Context()
	if clientIP, ok := requestmeta.ExtractClientIP(r); ok {
		ctx = requestmeta.WithClientIP(ctx, clientIP)
	}

	session, err := h.inboxService.GetWidgetSession(ctx, sessionToken)
	if err != nil || session == nil {
		writeError(w, http.StatusUnauthorized, "invalid session")
		return
	}

	var req model.CreateSupportAttachmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	conversationID := ""
	if session.ConversationID != nil {
		conversationID = *session.ConversationID
	}
	resp, err := h.attachmentService.Create(ctx, req, session.WorkspaceID, conversationID, "customer", nil, &session.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// WidgetConfirmUpload handles PATCH /widget/support/attachments/{attachmentId}/confirm (session token auth).
func (h *SupportAttachmentHandler) WidgetConfirmUpload(w http.ResponseWriter, r *http.Request) {
	sessionToken := r.Header.Get("X-Session-Token")
	if sessionToken == "" {
		writeError(w, http.StatusUnauthorized, "session token required")
		return
	}

	ctx := r.Context()
	if clientIP, ok := requestmeta.ExtractClientIP(r); ok {
		ctx = requestmeta.WithClientIP(ctx, clientIP)
	}

	session, err := h.inboxService.GetWidgetSession(ctx, sessionToken)
	if err != nil || session == nil {
		writeError(w, http.StatusUnauthorized, "invalid session")
		return
	}
	id := chi.URLParam(r, "attachmentId")
	if err := h.attachmentService.ConfirmUpload(ctx, id, "customer", nil, &session.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "upload confirmed"})
}
