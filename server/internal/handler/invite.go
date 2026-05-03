package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// InviteHandler handles invitation HTTP requests.
type InviteHandler struct {
	service *service.InviteService
}

// NewInviteHandler creates a new InviteHandler.
func NewInviteHandler(svc *service.InviteService) *InviteHandler {
	return &InviteHandler{service: svc}
}

// Send handles POST /api/invitations — create a new invitation.
func (h *InviteHandler) Send(w http.ResponseWriter, r *http.Request) {
	var req model.CreateInvitationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and email are required")
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}

	userID := middleware.GetUserID(r.Context())
	result, err := h.service.CreateInvitation(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// List handles GET /api/invitations?workspace_id=x — list invitations.
func (h *InviteHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	userID := middleware.GetUserID(r.Context())
	result, err := h.service.ListInvitations(r.Context(), workspaceID, userID)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Accept handles POST /api/invitations/accept — accept an invitation.
func (h *InviteHandler) Accept(w http.ResponseWriter, r *http.Request) {
	var req model.AcceptInvitationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if err := h.service.AcceptInvitation(r.Context(), req.Token, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "invitation accepted"})
}

// Resend handles POST /api/invitations/{id}/resend — resend an invitation.
func (h *InviteHandler) Resend(w http.ResponseWriter, r *http.Request) {
	invitationID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	if err := h.service.ResendInvitation(r.Context(), invitationID, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "invitation resent"})
}

// Revoke handles DELETE /api/invitations/{id} — revoke an invitation.
func (h *InviteHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	invitationID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	if err := h.service.RevokeInvitation(r.Context(), invitationID, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "invitation revoked"})
}

// AcceptWithSignup handles POST /api/invitations/accept-with-signup — public, register + accept.
func (h *InviteHandler) AcceptWithSignup(w http.ResponseWriter, r *http.Request) {
	var req model.AcceptInvitationWithSignupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || req.Password == "" || req.FullName == "" {
		writeError(w, http.StatusBadRequest, "token, password, and full_name are required")
		return
	}

	result, err := h.service.AcceptInvitationWithSignup(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	setAuthCookies(w, r, result.AccessToken, result.RefreshToken)
	writeJSON(w, http.StatusCreated, result)
}

// GetInfo handles GET /api/invitations/info?token=x — public, get invite details.
func (h *InviteHandler) GetInfo(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	result, err := h.service.GetInviteInfo(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
